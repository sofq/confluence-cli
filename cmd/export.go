package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export page content in requested format",
	RunE:  runExport,
}

// exportEntry is one NDJSON line in tree export output.
type exportEntry struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	ParentID string          `json:"parentId"`
	Depth    int             `json:"depth"`
	Body     json.RawMessage `json:"body"`
}

// childInfo represents a child page from the children endpoint.
type childInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func runExport(cmd *cobra.Command, args []string) error {
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

	format, _ := cmd.Flags().GetString("format")
	tree, _ := cmd.Flags().GetBool("tree")

	if tree {
		return runTreeExport(cmd.Context(), c, id, format, cmd)
	}
	return runSingleExport(cmd.Context(), c, id, format)
}

// runSingleExport fetches a single page and outputs the body object.
func runSingleExport(ctx context.Context, c *client.Client, pageID, format string) error {
	path := fmt.Sprintf("/pages/%s?body-format=%s", url.PathEscape(pageID), url.QueryEscape(format))
	body, code := c.Fetch(ctx, "GET", path, nil)
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}

	// Extract the body field from the page response.
	var page struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse page response: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	if page.Body == nil {
		apiErr := &cferrors.APIError{ErrorType: "not_found", Message: "page has no body in format " + format}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitNotFound}
	}

	// Output the body object through WriteOutput (applies --jq/--pretty if set).
	if ec := c.WriteOutput(page.Body); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// runTreeExport performs recursive depth-first tree export as NDJSON.
func runTreeExport(ctx context.Context, c *client.Client, rootPageID, format string, cmd *cobra.Command) error {
	depth, _ := cmd.Flags().GetInt("depth")

	enc := jsonutil.NewEncoder(c.Stdout)

	// Export root page first.
	walkTree(ctx, c, rootPageID, "", 0, depth, format, enc)
	return nil
}

// walkTree recursively exports a page and its children as NDJSON.
func walkTree(ctx context.Context, c *client.Client, pageID, parentID string,
	currentDepth, maxDepth int, format string, enc *json.Encoder) {

	// Check context cancellation.
	if ctx.Err() != nil {
		return
	}

	// Fetch page with body.
	path := fmt.Sprintf("/pages/%s?body-format=%s", url.PathEscape(pageID), url.QueryEscape(format))
	body, code := c.Fetch(ctx, "GET", path, nil)
	if code != cferrors.ExitOK {
		// Partial failure: log to stderr, continue (D-13).
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   fmt.Sprintf("failed to fetch page %s: exit code %d", pageID, code),
		}
		apiErr.WriteJSON(c.Stderr)
		return
	}

	// Parse page response.
	var page struct {
		ID    string          `json:"id"`
		Title string          `json:"title"`
		Body  json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   fmt.Sprintf("failed to parse page %s: %s", pageID, err.Error()),
		}
		apiErr.WriteJSON(c.Stderr)
		return
	}

	// Emit NDJSON line for this page.
	entry := exportEntry{
		ID:       page.ID,
		Title:    page.Title,
		ParentID: parentID,
		Depth:    currentDepth,
		Body:     page.Body,
	}
	_ = enc.Encode(entry)

	// Check depth limit: 0 = unlimited, otherwise stop when currentDepth >= maxDepth.
	if maxDepth > 0 && currentDepth >= maxDepth {
		return
	}

	// Fetch children and recurse.
	children, err := fetchAllChildren(ctx, c, pageID)
	if err != nil {
		// Partial failure on children: log and continue.
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   fmt.Sprintf("failed to fetch children of page %s: %s", pageID, err.Error()),
		}
		apiErr.WriteJSON(c.Stderr)
		return
	}

	for _, child := range children {
		walkTree(ctx, c, child.ID, pageID, currentDepth+1, maxDepth, format, enc)
	}
}

// fetchAllChildren retrieves all child pages of a given page, handling cursor pagination.
// Returns only id and title since the children endpoint does NOT return body.
func fetchAllChildren(ctx context.Context, c *client.Client, pageID string) ([]childInfo, error) {
	var all []childInfo
	path := fmt.Sprintf("/pages/%s/children?limit=25", url.PathEscape(pageID))

	for path != "" {
		if ctx.Err() != nil {
			return all, ctx.Err()
		}

		body, code := c.Fetch(ctx, "GET", path, nil)
		if code != cferrors.ExitOK {
			return all, fmt.Errorf("fetch children failed with exit code %d", code)
		}

		var page struct {
			Results []childInfo `json:"results"`
			Links   struct {
				Next string `json:"next"`
			} `json:"_links"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return all, fmt.Errorf("parse children response: %w", err)
		}

		all = append(all, page.Results...)

		// Follow pagination cursor.
		nextLink := page.Links.Next
		if nextLink == "" {
			break
		}
		// The next link from v2 API is a relative path (e.g., /wiki/api/v2/pages/123/children?cursor=abc).
		// c.Fetch prepends BaseURL, so strip the /wiki/api/v2 prefix if present.
		if idx := strings.Index(nextLink, "/pages/"); idx >= 0 {
			path = nextLink[idx:]
		} else {
			path = nextLink
		}
	}
	return all, nil
}

func init() {
	exportCmd.Flags().String("id", "", "page ID to export (required)")
	exportCmd.Flags().String("format", "storage", "body format: storage, atlas_doc_format, view")
	exportCmd.Flags().Bool("tree", false, "recursively export page tree as NDJSON")
	exportCmd.Flags().Int("depth", 0, "maximum tree depth (0 = unlimited)")
}
