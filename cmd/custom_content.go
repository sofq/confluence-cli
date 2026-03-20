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

// custom_contentCmd is the hand-written parent command for custom content operations.
// mergeCommand(rootCmd, custom_contentCmd) is called from cmd/root.go init() (Phase 9).
var custom_contentCmd = &cobra.Command{
	Use:   "custom-content",
	Short: "Confluence custom content operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema custom-content` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema custom-content` to list operations", cmd.CommandPath())
	},
}

// fetchCustomContentVersion fetches the current version number of a custom content item.
// Uses GET /custom-content/{id} (v2 path; BaseURL already includes /wiki/api/v2).
func fetchCustomContentVersion(ctx context.Context, c *client.Client, id string) (int, int) {
	body, code := c.Fetch(ctx, "GET", fmt.Sprintf("/custom-content/%s", url.PathEscape(id)), nil)
	if code != cferrors.ExitOK {
		return 0, code
	}
	var item struct {
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   "failed to parse custom content version: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return 0, cferrors.ExitError
	}
	return item.Version.Number, cferrors.ExitOK
}

// customContentUpdateBody is the request body for PUT /custom-content/{id}.
type customContentUpdateBody struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
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

// doCustomContentUpdate sends a PUT /custom-content/{id} request with the given parameters.
// Returns the exit code. On success, writes the response body to c.Stdout.
func doCustomContentUpdate(ctx context.Context, c *client.Client, id, title, storageValue string, versionNumber int) int {
	var reqBody customContentUpdateBody
	reqBody.ID = id
	reqBody.Status = "current"
	reqBody.Title = title
	reqBody.Body.Representation = "storage"
	reqBody.Body.Value = storageValue
	reqBody.Version.Number = versionNumber
	encoded, _ := json.Marshal(reqBody)
	respBody, code := c.Fetch(ctx, "PUT", fmt.Sprintf("/custom-content/%s", url.PathEscape(id)), bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return code
	}
	return c.WriteOutput(respBody)
}

// ---------------------------------------------------------------------------
// Subcommands
// ---------------------------------------------------------------------------

// custom_content_workflow_get_by_type lists custom content by type with required --type flag.
var custom_content_workflow_get_by_type = &cobra.Command{
	Use:   "get-custom-content-by-type",
	Short: "Get custom content by type",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		typeVal, _ := cmd.Flags().GetString("type")
		if strings.TrimSpace(typeVal) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--type must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		q := url.Values{"type": []string{typeVal}}
		spaceID, _ := cmd.Flags().GetString("space-id")
		if spaceID != "" {
			q.Set("space-id", spaceID)
		}
		code := c.Do(cmd.Context(), "GET", "/custom-content", q, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// custom_content_workflow_create creates custom content with required --type, --space-id, --title, --body.
var custom_content_workflow_create = &cobra.Command{
	Use:   "create-custom-content",
	Short: "Create custom content with storage format body",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		typeVal, _ := cmd.Flags().GetString("type")
		spaceID, _ := cmd.Flags().GetString("space-id")
		title, _ := cmd.Flags().GetString("title")
		bodyVal, _ := cmd.Flags().GetString("body")

		// Validate required flags.
		for _, pair := range []struct{ name, val string }{
			{"--type", typeVal},
			{"--space-id", spaceID},
			{"--title", title},
			{"--body", bodyVal},
		} {
			if strings.TrimSpace(pair.val) == "" {
				apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: pair.name + " must not be empty"}
				apiErr.WriteJSON(c.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
		}

		// Build request body.
		type createBody struct {
			Type    string `json:"type"`
			SpaceID string `json:"spaceId"`
			Title   string `json:"title"`
			Body    struct {
				Representation string `json:"representation"`
				Value          string `json:"value"`
			} `json:"body"`
		}
		var reqBody createBody
		reqBody.Type = typeVal
		reqBody.SpaceID = spaceID
		reqBody.Title = title
		reqBody.Body.Representation = "storage"
		reqBody.Body.Value = bodyVal
		encoded, _ := json.Marshal(reqBody)
		respBody, code := c.Fetch(cmd.Context(), "POST", "/custom-content", bytes.NewReader(encoded))
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	},
}

// custom_content_workflow_get_by_id retrieves custom content by ID, always injecting body-format=storage.
var custom_content_workflow_get_by_id = &cobra.Command{
	Use:   "get-custom-content-by-id",
	Short: "Get custom content by ID with storage body",
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
		path := fmt.Sprintf("/custom-content/%s", url.PathEscape(id))
		code := c.Do(cmd.Context(), "GET", path, q, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// custom_content_workflow_update updates custom content with automatic version increment and single 409 retry.
var custom_content_workflow_update = &cobra.Command{
	Use:   "update-custom-content",
	Short: "Update custom content with automatic version increment",
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
		currentVersion, code := fetchCustomContentVersion(cmd.Context(), c, id)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		// First attempt.
		code = doCustomContentUpdate(cmd.Context(), c, id, title, bodyVal, currentVersion+1)
		if code == cferrors.ExitConflict {
			// Single retry: re-fetch version and try once more.
			currentVersion, code = fetchCustomContentVersion(cmd.Context(), c, id)
			if code != cferrors.ExitOK {
				return &cferrors.AlreadyWrittenError{Code: code}
			}
			code = doCustomContentUpdate(cmd.Context(), c, id, title, bodyVal, currentVersion+1)
		}
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// custom_content_workflow_delete soft-deletes custom content via HTTP DELETE.
var custom_content_workflow_delete = &cobra.Command{
	Use:   "delete-custom-content",
	Short: "Delete custom content (moves to trash)",
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
		path := fmt.Sprintf("/custom-content/%s", url.PathEscape(id))
		code := c.Do(cmd.Context(), "DELETE", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

func init() {
	// get-custom-content-by-type flags
	custom_content_workflow_get_by_type.Flags().String("type", "", "Custom content type (required, e.g. ac:app:type)")
	custom_content_workflow_get_by_type.Flags().String("space-id", "", "Filter by space ID")

	// create-custom-content flags
	custom_content_workflow_create.Flags().String("type", "", "Custom content type (required, e.g. ac:app:type)")
	custom_content_workflow_create.Flags().String("space-id", "", "Space ID to create custom content in (required)")
	custom_content_workflow_create.Flags().String("title", "", "Custom content title (required)")
	custom_content_workflow_create.Flags().String("body", "", "Custom content body in storage format XML (required)")

	// get-custom-content-by-id flags
	custom_content_workflow_get_by_id.Flags().String("id", "", "Custom content ID (required)")
	custom_content_workflow_get_by_id.Flags().String("body-format", "storage", "Body format (default: storage)")

	// update-custom-content flags
	custom_content_workflow_update.Flags().String("id", "", "Custom content ID to update (required)")
	custom_content_workflow_update.Flags().String("title", "", "Custom content title (required)")
	custom_content_workflow_update.Flags().String("body", "", "Custom content body in storage format XML (required)")

	// delete-custom-content flags
	custom_content_workflow_delete.Flags().String("id", "", "Custom content ID to delete (required)")

	// Register all subcommands on custom_contentCmd.
	custom_contentCmd.AddCommand(custom_content_workflow_get_by_type)
	custom_contentCmd.AddCommand(custom_content_workflow_create)
	custom_contentCmd.AddCommand(custom_content_workflow_get_by_id)
	custom_contentCmd.AddCommand(custom_content_workflow_update)
	custom_contentCmd.AddCommand(custom_content_workflow_delete)
}
