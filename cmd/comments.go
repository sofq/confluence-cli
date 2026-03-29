package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// commentsCmd is the hand-written parent command for comments operations.
// mergeCommand(rootCmd, commentsCmd) is called from cmd/root.go init() (Plan 04).
var commentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "Confluence comment operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema comments` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema comments` to list operations", cmd.CommandPath())
	},
}

var comments_list = &cobra.Command{
	Use:   "list",
	Short: "List footer comments on a page",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		pageID, _ := cmd.Flags().GetString("page-id")
		if strings.TrimSpace(pageID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--page-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		// v2 path: c.BaseURL already includes /wiki/api/v2, so /pages/{id}/footer-comments is correct.
		path := fmt.Sprintf("/pages/%s/footer-comments", url.PathEscape(pageID))
		code := c.Do(cmd.Context(), "GET", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// createCommentBody is the v2 footer-comments POST request body.
type createCommentBody struct {
	PageID string `json:"pageId"`
	Body   struct {
		Representation string `json:"representation"`
		Value          string `json:"value"`
	} `json:"body"`
}

var comments_create = &cobra.Command{
	Use:   "create",
	Short: "Create a footer comment on a page",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		pageID, _ := cmd.Flags().GetString("page-id")
		bodyVal, _ := cmd.Flags().GetString("body")
		if strings.TrimSpace(pageID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--page-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(bodyVal) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--body must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		var reqBody createCommentBody
		reqBody.PageID = pageID
		reqBody.Body.Representation = "storage"
		reqBody.Body.Value = bodyVal
		encoded, _ := json.Marshal(reqBody)

		// v2 path: POST /footer-comments
		respBody, code := c.Fetch(cmd.Context(), "POST", "/footer-comments", bytes.NewReader(encoded))
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		// WriteOutput with valid JSON from the server cannot fail.
		c.WriteOutput(respBody) //nolint:errcheck
		return nil
	},
}

var comments_delete = &cobra.Command{
	Use:   "delete",
	Short: "Delete a footer comment",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		commentID, _ := cmd.Flags().GetString("comment-id")
		if strings.TrimSpace(commentID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--comment-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		// v2 path: DELETE /footer-comments/{id}
		path := fmt.Sprintf("/footer-comments/%s", url.PathEscape(commentID))
		code := c.Do(cmd.Context(), "DELETE", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

func init() {
	comments_list.Flags().String("page-id", "", "Page ID to list comments for (required)")
	comments_create.Flags().String("page-id", "", "Page ID to create comment on (required)")
	comments_create.Flags().String("body", "", "Comment body in storage format XML (required)")
	comments_delete.Flags().String("comment-id", "", "Comment ID to delete (required)")

	commentsCmd.AddCommand(comments_list)
	commentsCmd.AddCommand(comments_create)
	commentsCmd.AddCommand(comments_delete)
	// commentsCmd is registered via mergeCommand(rootCmd, commentsCmd) in cmd/root.go (Plan 04).
	// Do NOT call rootCmd.AddCommand here.
}
