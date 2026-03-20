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

	enc := json.NewEncoder(c.Stdout)
	enc.SetEscapeHTML(false)

	seen := make(map[string]string) // contentID -> modifiedAt

	// Initial poll immediately.
	pollAndEmit(ctx, cmd, c, cqlQuery, seen, enc)

	// If max-polls is set and we've done enough, emit shutdown and return.
	if maxPolls > 0 && maxPolls <= 1 {
		_ = enc.Encode(map[string]string{"type": "shutdown"})
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pollsDone := 1
	for {
		select {
		case <-ctx.Done():
			_ = enc.Encode(map[string]string{"type": "shutdown"})
			return nil
		case <-ticker.C:
			pollAndEmit(ctx, cmd, c, cqlQuery, seen, enc)
			pollsDone++
			if maxPolls > 0 && pollsDone >= maxPolls {
				_ = enc.Encode(map[string]string{"type": "shutdown"})
				return nil
			}
		}
	}
}

// pollAndEmit performs a single CQL search poll and emits NDJSON change events
// for any content that has been modified since last seen.
func pollAndEmit(ctx context.Context, cmd *cobra.Command, c *client.Client, cqlQuery string, seen map[string]string, enc *json.Encoder) {
	fullCQL := buildWatchCQL(cqlQuery, seen)
	domain := searchV1Domain(c.BaseURL)

	q := url.Values{}
	q.Set("cql", fullCQL)
	q.Set("limit", "25")
	q.Set("expand", "content.version,content.space")
	nextURL := domain + "/wiki/rest/api/search?" + q.Encode()

	pageCount := 0
	for nextURL != "" && pageCount < 5 {
		body, code := fetchV1(cmd, c, nextURL)
		if code != cferrors.ExitOK {
			// Check if shutdown caused the error.
			if ctx.Err() != nil {
				return
			}
			// Real error already written to stderr by fetchV1. Continue on next interval.
			return
		}

		var page watchSearchResponse
		if err := json.Unmarshal(body, &page); err != nil {
			apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse search response: " + err.Error()}
			apiErr.WriteJSON(c.Stderr)
			return
		}

		for _, result := range page.Results {
			contentID := result.Content.ID
			modifiedAt := result.Content.Version.When
			if modifiedAt == "" {
				modifiedAt = result.LastModified
			}

			// Dedup: skip if already seen with same or newer timestamp.
			if prev, ok := seen[contentID]; ok && prev >= modifiedAt {
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
				ModifiedAt:  modifiedAt,
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
	pruneThreshold := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	for id, ts := range seen {
		if ts < pruneThreshold {
			delete(seen, id)
		}
	}
}

// buildWatchCQL combines the user's CQL query with a lastModified date filter
// and ORDER BY clause.
func buildWatchCQL(userCQL string, seen map[string]string) string {
	var dateFilter string
	if len(seen) == 0 {
		// First poll: look back 1 day.
		dateFilter = time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	} else {
		// Subsequent polls: use today's date.
		dateFilter = time.Now().UTC().Format("2006-01-02")
	}
	return fmt.Sprintf("(%s) AND lastModified >= \"%s\" ORDER BY lastModified DESC", userCQL, dateFilter)
}

func init() {
	watchCmd.Flags().String("cql", "", "CQL query to watch (required)")
	watchCmd.Flags().Duration("interval", 60*time.Second, "polling interval (e.g. 30s, 2m)")
	watchCmd.Flags().Int("max-polls", 0, "maximum number of polls before exiting (0 = unlimited, for testing)")
	// Hide max-polls from help -- it's for testing only.
	watchCmd.Flags().MarkHidden("max-polls")
}
