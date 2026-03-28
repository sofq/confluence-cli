package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch Confluence content for changes via CQL polling",
	RunE:  runWatch,
}

// watchSearchResponse is the v1 search API response envelope.
type watchSearchResponse struct {
	Results []watchSearchResult `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// watchSearchResult represents a single v1 search result with content metadata.
type watchSearchResult struct {
	Content struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Title string `json:"title"`
		Space struct {
			ID  json.Number `json:"id"`
			Key string      `json:"key"`
		} `json:"space"`
		Version struct {
			When string `json:"when"`
			By   struct {
				DisplayName string `json:"displayName"`
			} `json:"by"`
		} `json:"version"`
	} `json:"content"`
	LastModified string `json:"lastModified"`
}

// watchChangeEvent is the NDJSON change event emitted to stdout.
type watchChangeEvent struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Title       string `json:"title"`
	SpaceID     string `json:"spaceId"`
	Modifier    string `json:"modifier"`
	ModifiedAt  string `json:"modifiedAt"`
}

func runWatch(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	cqlQuery, _ := cmd.Flags().GetString("cql")
	if strings.TrimSpace(cqlQuery) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--cql must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	interval, _ := cmd.Flags().GetDuration("interval")
	maxPolls, _ := cmd.Flags().GetInt("max-polls")

	// Create signal-aware context from the command's existing context.
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	enc := jsonutil.NewEncoder(c.Stdout)

	seen := make(map[string]time.Time) // contentID -> modifiedAt (parsed)

	// Initial poll immediately.
	consecutiveErrors := 0
	if err := pollAndEmit(ctx, cmd, c, cqlQuery, seen, enc); err != nil {
		consecutiveErrors++
	} else {
		consecutiveErrors = 0
	}

	// If max-polls is set and we've done enough, emit shutdown and return.
	if maxPolls > 0 && maxPolls <= 1 {
		_ = enc.Encode(map[string]string{"type": "shutdown"})
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	const maxConsecutiveErrors = 5
	pollsDone := 1
	for {
		select {
		case <-ctx.Done():
			_ = enc.Encode(map[string]string{"type": "shutdown"})
			return nil
		case <-ticker.C:
			if err := pollAndEmit(ctx, cmd, c, cqlQuery, seen, enc); err != nil {
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					_ = enc.Encode(map[string]any{"type": "error", "message": fmt.Sprintf("stopping after %d consecutive poll failures", maxConsecutiveErrors)})
					return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
				}
			} else {
				consecutiveErrors = 0
			}
			pollsDone++
			if maxPolls > 0 && pollsDone >= maxPolls {
				_ = enc.Encode(map[string]string{"type": "shutdown"})
				return nil
			}
		}
	}
}

// pollAndEmit performs a single CQL search poll and emits NDJSON change events
// for any content that has been modified since last seen. Returns an error if
// the poll failed (errors are also written to stderr).
func pollAndEmit(ctx context.Context, cmd *cobra.Command, c *client.Client, cqlQuery string, seen map[string]time.Time, enc *json.Encoder) error {
	fullCQL := buildWatchCQL(cqlQuery, seen)
	domain := client.SearchV1Domain(c.BaseURL)

	q := url.Values{}
	q.Set("cql", fullCQL)
	q.Set("limit", "25")
	q.Set("expand", "content.version,content.space")
	nextURL := domain + "/wiki/rest/api/search?" + q.Encode()

	pageCount := 0
	for nextURL != "" && pageCount < 5 {
		body, code := fetchV1(cmd, c, nextURL)
		if code != cferrors.ExitOK {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("poll failed with exit code %d", code)
		}

		var page watchSearchResponse
		if err := json.Unmarshal(body, &page); err != nil {
			apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse search response: " + err.Error()}
			apiErr.WriteJSON(c.Stderr)
			return fmt.Errorf("parse error: %w", err)
		}

		for _, result := range page.Results {
			contentID := result.Content.ID
			modifiedAtStr := result.Content.Version.When
			if modifiedAtStr == "" {
				modifiedAtStr = result.LastModified
			}

			modifiedAt := parseTimestamp(modifiedAtStr)

			// Dedup: skip if already seen with same or newer timestamp.
			if prev, ok := seen[contentID]; ok && !modifiedAt.After(prev) {
				continue
			}
			seen[contentID] = modifiedAt

			event := watchChangeEvent{
				Type:        "change",
				ID:          contentID,
				ContentType: result.Content.Type,
				Title:       result.Content.Title,
				SpaceID:     result.Content.Space.ID.String(),
				Modifier:    result.Content.Version.By.DisplayName,
				ModifiedAt:  modifiedAtStr,
			}
			_ = enc.Encode(event)
		}

		// Follow pagination.
		nextLink := page.Links.Next
		if nextLink == "" {
			break
		}
		if strings.HasPrefix(nextLink, "http") {
			nextURL = nextLink
		} else {
			nextURL = domain + nextLink
		}
		pageCount++
	}

	// Prune seen entries older than 48 hours to prevent unbounded growth.
	pruneThreshold := time.Now().UTC().Add(-48 * time.Hour)
	for id, ts := range seen {
		if ts.Before(pruneThreshold) {
			delete(seen, id)
		}
	}

	return nil
}

// parseTimestamp parses a Confluence timestamp (RFC3339 or with milliseconds).
// Returns the zero time as fallback if parsing fails entirely, so that the
// dedup check in pollAndEmit treats the item as "never seen" rather than
// re-emitting it on every poll (which time.Now() would cause).
func parseTimestamp(s string) time.Time {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.999Z07:00",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// buildWatchCQL combines the user's CQL query with a lastModified date filter
// and ORDER BY clause. Always uses a 1-day lookback because CQL lastModified
// only supports date granularity (not time). The seen map handles dedup for
// content already emitted.
// Note: userCQL is wrapped in parentheses but not escaped — it is trusted
// user input from the --cql flag.
func buildWatchCQL(userCQL string, seen map[string]time.Time) string {
	// Always look back 1 day to avoid missing changes near midnight UTC.
	// The seen map (with proper timestamp comparison) prevents duplicate emissions.
	dateFilter := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	return fmt.Sprintf("(%s) AND lastModified >= \"%s\" ORDER BY lastModified DESC", userCQL, dateFilter)
}

func init() {
	watchCmd.Flags().String("cql", "", "CQL query to watch (required)")
	watchCmd.Flags().Duration("interval", 60*time.Second, "polling interval (e.g. 30s, 2m)")
	watchCmd.Flags().Int("max-polls", 0, "maximum number of polls before exiting (0 = unlimited, for testing)")
	// Hide max-polls from help -- it's for testing only.
	watchCmd.Flags().MarkHidden("max-polls")
}
