package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// pagesCmd is the hand-written parent command for pages operations.
// mergeCommand(rootCmd, pagesCmd) is called from cmd/root.go init() (Plan 04).
var pagesCmd = &cobra.Command{
	Use:   "pages",
	Short: "Confluence page operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema pages` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema pages` to list operations", cmd.CommandPath())
	},
}

// fetchPageVersion fetches the current version number of a page.
// Uses GET /pages/{id} (v2 path; BaseURL already includes /wiki/api/v2).
func fetchPageVersion(ctx context.Context, c *client.Client, id string) (int, int) {
	body, code := c.Fetch(ctx, "GET", fmt.Sprintf("/pages/%s", url.PathEscape(id)), nil)
	if code != cferrors.ExitOK {
		return 0, code
	}
	var page struct {
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   "failed to parse page version: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return 0, cferrors.ExitError
	}
	return page.Version.Number, cferrors.ExitOK
}

// pageUpdateBody is the request body for PUT /pages/{id}.
type pageUpdateBody struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Body   struct {
		Representation string `json:"representation"`
		Value          string `json:"value"`
	} `json:"body"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
}

// doPageUpdate sends a PUT /pages/{id} request with the given parameters.
// Returns the exit code. On success, writes the response body to c.Stdout.
func doPageUpdate(ctx context.Context, c *client.Client, id, title, storageValue string, versionNumber int) int {
	var reqBody pageUpdateBody
	reqBody.ID = id
	reqBody.Status = "current"
	reqBody.Title = title
	reqBody.Body.Representation = "storage"
	reqBody.Body.Value = storageValue
	reqBody.Version.Number = versionNumber
	encoded, _ := json.Marshal(reqBody)
	respBody, code := c.Fetch(ctx, "PUT", fmt.Sprintf("/pages/%s", url.PathEscape(id)), bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return code
	}
	return c.WriteOutput(respBody)
}

// ---------------------------------------------------------------------------
// Subcommands
// ---------------------------------------------------------------------------

// pages_workflow_get_by_id retrieves a page by ID, always injecting body-format=storage.
var pages_workflow_get_by_id = &cobra.Command{
	Use:   "get-by-id",
	Short: "Get page by ID with storage body",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		// Always inject body-format=storage unless the user explicitly overrides.
		q := url.Values{"body-format": []string{"storage"}}
		if cmd.Flags().Changed("body-format") {
			bf, _ := cmd.Flags().GetString("body-format")
			q.Set("body-format", bf)
		}
		path := fmt.Sprintf("/pages/%s", url.PathEscape(id))
		code := c.Do(cmd.Context(), "GET", path, q, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// pages_workflow_create creates a page with friendly flags, building the JSON body internally.
var pages_workflow_create = &cobra.Command{
	Use:   "create",
	Short: "Create a page with storage format body",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		spaceID, _ := cmd.Flags().GetString("space-id")
		title, _ := cmd.Flags().GetString("title")
		bodyVal, _ := cmd.Flags().GetString("body")
		parentID, _ := cmd.Flags().GetString("parent-id")
		templateName, _ := cmd.Flags().GetString("template")
		varFlags, _ := cmd.Flags().GetStringArray("var")

		// Template resolution (before validation so template can provide title/body/space-id).
		if templateName != "" {
			if bodyVal != "" {
				apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "cannot use --template and --body together"}
				apiErr.WriteJSON(c.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
			rendered, resolveErr := resolveTemplate(c.Stderr, templateName, varFlags)
			if resolveErr != nil {
				return resolveErr
			}
			if title == "" {
				title = rendered.Title
			}
			bodyVal = rendered.Body
			if spaceID == "" && rendered.SpaceID != "" {
				spaceID = rendered.SpaceID
			}
		}

		// Validate required flags.
		if strings.TrimSpace(spaceID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--space-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(title) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--title must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(bodyVal) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--body must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		// Build request body.
		type createBody struct {
			SpaceID  string `json:"spaceId"`
			Title    string `json:"title"`
			Body     struct {
				Representation string `json:"representation"`
				Value          string `json:"value"`
			} `json:"body"`
			ParentID string `json:"parentId,omitempty"`
		}
		var reqBody createBody
		reqBody.SpaceID = spaceID
		reqBody.Title = title
		reqBody.Body.Representation = "storage"
		reqBody.Body.Value = bodyVal
		if parentID != "" {
			reqBody.ParentID = parentID
		}
		encoded, _ := json.Marshal(reqBody)
		respBody, code := c.Fetch(cmd.Context(), "POST", "/pages", bytes.NewReader(encoded))
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	},
}

// pages_workflow_update updates a page with automatic version increment and single 409 retry.
var pages_workflow_update = &cobra.Command{
	Use:   "update",
	Short: "Update a page with automatic version increment",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		id, _ := cmd.Flags().GetString("id")
		title, _ := cmd.Flags().GetString("title")
		bodyVal, _ := cmd.Flags().GetString("body")

		// Validate required flags.
		for _, pair := range []struct{ name, val string }{
			{"--id", id},
			{"--title", title},
			{"--body", bodyVal},
		} {
			if strings.TrimSpace(pair.val) == "" {
				apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: pair.name + " must not be empty"}
				apiErr.WriteJSON(c.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
		}

		// Fetch current version.
		currentVersion, code := fetchPageVersion(cmd.Context(), c, id)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		// First attempt.
		code = doPageUpdate(cmd.Context(), c, id, title, bodyVal, currentVersion+1)
		if code == cferrors.ExitConflict {
			// Single retry: re-fetch version and try once more.
			currentVersion, code = fetchPageVersion(cmd.Context(), c, id)
			if code != cferrors.ExitOK {
				return &cferrors.AlreadyWrittenError{Code: code}
			}
			code = doPageUpdate(cmd.Context(), c, id, title, bodyVal, currentVersion+1)
		}
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// pages_workflow_delete soft-deletes a page (moves to trash) via HTTP DELETE.
var pages_workflow_delete = &cobra.Command{
	Use:   "delete",
	Short: "Delete a page (moves to trash)",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		path := fmt.Sprintf("/pages/%s", url.PathEscape(id))
		code := c.Do(cmd.Context(), "DELETE", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// pages_workflow_list lists pages, optionally filtered by space-id, with auto-pagination.
var pages_workflow_list = &cobra.Command{
	Use:   "get",
	Short: "List pages in a space",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		spaceID, _ := cmd.Flags().GetString("space-id")
		q := url.Values{}
		if spaceID != "" {
			q.Set("space-id", spaceID)
		}
		code := c.Do(cmd.Context(), "GET", "/pages", q, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

func init() {
	// get-by-id flags
	pages_workflow_get_by_id.Flags().String("id", "", "Page ID (required)")
	pages_workflow_get_by_id.Flags().String("body-format", "storage", "Body format (default: storage)")

	// create flags
	pages_workflow_create.Flags().String("space-id", "", "Space ID to create page in (required)")
	pages_workflow_create.Flags().String("title", "", "Page title (required)")
	pages_workflow_create.Flags().String("body", "", "Page body in storage format XML (required)")
	pages_workflow_create.Flags().String("parent-id", "", "Parent page ID (optional)")
	pages_workflow_create.Flags().String("template", "", "Content template name to use")
	pages_workflow_create.Flags().StringArray("var", nil, "Template variable in key=value format (repeatable)")

	// update flags
	pages_workflow_update.Flags().String("id", "", "Page ID to update (required)")
	pages_workflow_update.Flags().String("title", "", "Page title (required)")
	pages_workflow_update.Flags().String("body", "", "Page body in storage format XML (required)")

	// delete flags
	pages_workflow_delete.Flags().String("id", "", "Page ID to delete (required)")

	// list flags
	pages_workflow_list.Flags().String("space-id", "", "Filter pages by space ID")

	// Register all subcommands on pagesCmd.
	// NOTE: Do NOT call mergeCommand or rootCmd.AddCommand here — that wiring happens in Plan 04 (cmd/root.go).
	pagesCmd.AddCommand(pages_workflow_get_by_id)
	pagesCmd.AddCommand(pages_workflow_create)
	pagesCmd.AddCommand(pages_workflow_update)
	pagesCmd.AddCommand(pages_workflow_delete)
	pagesCmd.AddCommand(pages_workflow_list)
}
