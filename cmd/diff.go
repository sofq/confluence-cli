package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/diff"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare page versions and show structured diff",
	Long: `Compares page versions and outputs structured JSON with version metadata and change statistics.

Supports three modes:
  Default: compares the two most recent versions
  --since: shows all changes within a time range (pairwise diffs)
  --from/--to: compares two explicit version numbers

Examples:
  cf diff --id 123456
  cf diff --id 123456 --since 2h
  cf diff --id 123456 --since 2026-01-01
  cf diff --id 123456 --from 3 --to 5`,
	RunE: runDiff,
}

// apiVersionEntry matches the Confluence v2 version list item shape.
type apiVersionEntry struct {
	Number    int    `json:"number"`
	AuthorID  string `json:"authorId"`
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message"`
}

// apiVersionList is the paginated response from GET /pages/{id}/versions.
type apiVersionList struct {
	Results []apiVersionEntry `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

func runDiff(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	if strings.TrimSpace(id) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	since, _ := cmd.Flags().GetString("since")
	from, _ := cmd.Flags().GetInt("from")
	to, _ := cmd.Flags().GetInt("to")

	// Mutual exclusivity: --since and --from/--to cannot be used together.
	if since != "" && (from != 0 || to != 0) {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "cannot use --since with --from/--to"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Dry-run mode: output request as JSON without executing API calls.
	if c.DryRun {
		dryOut := map[string]any{
			"method": "GET",
			"url":    c.BaseURL + fmt.Sprintf("/pages/%s/versions", url.PathEscape(id)),
			"note":   fmt.Sprintf("would fetch version diff for page %s", id),
		}
		out, _ := jsonutil.MarshalNoEscape(dryOut)
		if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	}

	ctx := cmd.Context()
	opts := diff.Options{Since: since, From: from, To: to, Now: time.Now()}

	var versions []diff.VersionInput

	switch {
	case from != 0 || to != 0:
		// --from/--to mode: fetch specific versions directly.
		versions, err = fetchFromToVersions(ctx, c, id, from, to)
	case since != "":
		// --since mode: fetch all versions, filter by time.
		versions, err = fetchSinceVersions(ctx, c, id, since)
	default:
		// Default mode: fetch two most recent versions.
		versions, err = fetchDefaultVersions(ctx, c, id)
	}
	if err != nil {
		return err
	}

	// Compare and MarshalNoEscape cannot fail here:
	// - Compare's mutual-exclusivity and since-validation are already checked above.
	// - MarshalNoEscape marshals a struct with only basic field types.
	result, _ := diff.Compare(id, versions, opts)
	out, _ := jsonutil.MarshalNoEscape(result)

	if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// fetchDefaultVersions fetches the two most recent versions and their bodies.
func fetchDefaultVersions(ctx context.Context, c *client.Client, pageID string) ([]diff.VersionInput, error) {
	entries, err := fetchVersionList(ctx, c, pageID, 2)
	if err != nil {
		return nil, err
	}

	// Sort ascending by version number (oldest first) for Compare.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Number < entries[j].Number
	})

	return fetchVersionBodies(ctx, c, pageID, entries)
}

// fetchSinceVersions fetches all versions (paginated) for time-range filtering.
// It pre-filters entries by the --since cutoff before fetching bodies to avoid
// unnecessary API calls for versions outside the time range.
func fetchSinceVersions(ctx context.Context, c *client.Client, pageID, since string) ([]diff.VersionInput, error) {
	entries, err := fetchVersionList(ctx, c, pageID, 50)
	if err != nil {
		return nil, err
	}

	// Pre-filter by --since cutoff to avoid fetching bodies for old versions.
	cutoff, err := diff.ParseSince(since, time.Now())
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	var filtered []apiVersionEntry
	for _, e := range entries {
		t, parseErr := time.Parse(time.RFC3339, e.CreatedAt)
		if parseErr != nil {
			continue
		}
		if !t.Before(cutoff) {
			filtered = append(filtered, e)
		}
	}

	// Sort ascending by version number (oldest first) for Compare.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Number < filtered[j].Number
	})

	return fetchVersionBodies(ctx, c, pageID, filtered)
}

// fetchFromToVersions fetches specific version numbers and their bodies.
func fetchFromToVersions(ctx context.Context, c *client.Client, pageID string, from, to int) ([]diff.VersionInput, error) {
	// If only --from is set, default --to to the latest version.
	if to == 0 {
		entries, err := fetchVersionList(ctx, c, pageID, 1)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			to = entries[0].Number
		}
	}
	// If only --to is set, default --from to 1.
	if from == 0 {
		from = 1
	}

	var versions []diff.VersionInput
	for _, num := range []int{from, to} {
		body, available, err := fetchVersionBody(ctx, c, pageID, num)
		if err != nil {
			return nil, err
		}
		versions = append(versions, diff.VersionInput{
			Meta: diff.VersionMeta{
				Number:    num,
				CreatedAt: "", // Not known in from/to mode without version list
			},
			Body:          body,
			BodyAvailable: available,
		})
	}

	return versions, nil
}

// fetchVersionList retrieves the version list for a page, handling cursor pagination.
func fetchVersionList(ctx context.Context, c *client.Client, pageID string, limit int) ([]apiVersionEntry, error) {
	var all []apiVersionEntry
	path := fmt.Sprintf("/pages/%s/versions?limit=%d&sort=-modified-date", url.PathEscape(pageID), limit)

	for path != "" {
		if ctx.Err() != nil {
			return all, ctx.Err()
		}

		body, code := c.Fetch(ctx, "GET", path, nil)
		if code != cferrors.ExitOK {
			return nil, &cferrors.AlreadyWrittenError{Code: code}
		}

		var page apiVersionList
		if err := json.Unmarshal(body, &page); err != nil {
			apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse versions response: " + err.Error()}
			apiErr.WriteJSON(c.Stderr)
			return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		all = append(all, page.Results...)

		// Follow pagination cursor.
		nextLink := page.Links.Next
		if nextLink == "" {
			break
		}
		// Strip /wiki/api/v2 prefix from _links.next (same as export.go).
		if idx := strings.Index(nextLink, "/pages/"); idx >= 0 {
			path = nextLink[idx:]
		} else {
			path = nextLink
		}
	}
	return all, nil
}

// fetchVersionBody retrieves the body content for a specific page version.
// Returns (bodyContent, bodyAvailable, error).
func fetchVersionBody(ctx context.Context, c *client.Client, pageID string, versionNum int) (string, bool, error) {
	path := fmt.Sprintf("/pages/%s?version=%d&body-format=storage", url.PathEscape(pageID), versionNum)
	body, code := c.Fetch(ctx, "GET", path, nil)
	if code != cferrors.ExitOK {
		return "", false, &cferrors.AlreadyWrittenError{Code: code}
	}

	var page struct {
		Body struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return "", false, fmt.Errorf("failed to parse page version %d: %w", versionNum, err)
	}

	// Per D-09: if body.storage.value is empty, body is not available.
	if page.Body.Storage.Value == "" {
		return "", false, nil
	}

	return page.Body.Storage.Value, true, nil
}

// fetchVersionBodies fetches body content for each version entry and builds VersionInput slice.
func fetchVersionBodies(ctx context.Context, c *client.Client, pageID string, entries []apiVersionEntry) ([]diff.VersionInput, error) {
	var versions []diff.VersionInput
	for _, e := range entries {
		body, available, err := fetchVersionBody(ctx, c, pageID, e.Number)
		if err != nil {
			return nil, err
		}
		versions = append(versions, diff.VersionInput{
			Meta: diff.VersionMeta{
				Number:    e.Number,
				AuthorID:  e.AuthorID,
				CreatedAt: e.CreatedAt,
				Message:   e.Message,
			},
			Body:          body,
			BodyAvailable: available,
		})
	}
	return versions, nil
}

func init() {
	diffCmd.Flags().String("id", "", "page ID to compare versions (required)")
	diffCmd.Flags().String("since", "", "filter changes since duration (e.g. 2h, 1d) or ISO date (e.g. 2026-01-01)")
	diffCmd.Flags().Int("from", 0, "start version number for explicit comparison")
	diffCmd.Flags().Int("to", 0, "end version number for explicit comparison")
}
