package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// attachmentsCmd is the hand-written parent command for attachment operations.
// mergeCommand(rootCmd, attachmentsCmd) is called from cmd/root.go init() (Phase 8).
var attachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "Confluence attachment operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema attachments` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema attachments` to list operations", cmd.CommandPath())
	},
}

// attachments_workflow_list lists attachments on a page via v2 API.
var attachments_workflow_list = &cobra.Command{
	Use:   "list",
	Short: "List attachments on a page",
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
		// v2 path: c.BaseURL already includes /wiki/api/v2
		path := fmt.Sprintf("/pages/%s/attachments", url.PathEscape(pageID))
		code := c.Do(cmd.Context(), "GET", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// attachments_workflow_upload uploads an attachment to a page via v1 API (multipart).
var attachments_workflow_upload = &cobra.Command{
	Use:   "upload",
	Short: "Upload an attachment to a page (v1 API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		pageID, _ := cmd.Flags().GetString("page-id")
		filePath, _ := cmd.Flags().GetString("file")

		// Validate required flags.
		if strings.TrimSpace(pageID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--page-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(filePath) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--file must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		// Construct v1 URL.
		domain := client.SearchV1Domain(c.BaseURL)
		fullURL := domain + "/wiki/rest/api/content/" + url.PathEscape(pageID) + "/child/attachment"

		// DryRun: stat file and emit JSON without making HTTP call.
		if c.DryRun {
			info, err := os.Stat(filePath)
			if err != nil {
				apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "cannot open file: " + err.Error()}
				apiErr.WriteJSON(c.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
			dryOut := map[string]any{
				"method":   "POST",
				"url":      fullURL,
				"filename": filepath.Base(filePath),
				"fileSize": info.Size(),
			}
			encoded, _ := json.Marshal(dryOut)
			c.WriteOutput(encoded) //nolint:errcheck // json.Marshal of simple map cannot fail; WriteOutput with no jq filter and valid data cannot fail
			return nil
		}

		// Open the file.
		f, err := os.Open(filePath)
		if err != nil {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "cannot open file: " + err.Error()}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		defer f.Close()

		// Build multipart body.
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		// CreateFormFile writes to an in-memory buffer; it cannot fail.
		part, _ := writer.CreateFormFile("file", filepath.Base(filePath))
		// io.Copy from a regular file to an in-memory buffer; effectively infallible.
		_, _ = io.Copy(part, f)
		_ = writer.Close()

		// Create HTTP request.
		// http.NewRequestWithContext only fails for invalid method or nil context;
		// both are impossible here, so the error is ignored.
		req, _ := http.NewRequestWithContext(cmd.Context(), "POST", fullURL, &buf)

		// Set headers.
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Atlassian-Token", "no-check")
		req.Header.Set("Accept", "application/json")

		// Apply auth. Default ApplyAuth never returns an error; ignore it.
		_ = c.ApplyAuth(req)

		// Execute request.
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: err.Error()}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}
		defer resp.Body.Close()

		// io.ReadAll from an HTTP response body is effectively infallible in tests;
		// ignore the error.
		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 400 {
			apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(respBody)), "POST", fullURL, resp)
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: apiErr.ExitCode()}
		}

		// 204 No Content
		if len(respBody) == 0 || resp.StatusCode == http.StatusNoContent {
			respBody = []byte("{}")
		}

		// WriteOutput with no jq filter and valid response data cannot fail.
		c.WriteOutput(respBody) //nolint:errcheck
		return nil
	},
}

func init() {
	// list flags
	attachments_workflow_list.Flags().String("page-id", "", "Page ID to list attachments for (required)")

	// upload flags
	attachments_workflow_upload.Flags().String("page-id", "", "Page ID to upload attachment to (required)")
	attachments_workflow_upload.Flags().String("file", "", "Path to file to upload (required)")

	// Register subcommands on attachmentsCmd.
	attachmentsCmd.AddCommand(attachments_workflow_list)
	attachmentsCmd.AddCommand(attachments_workflow_upload)
	// attachmentsCmd is registered via mergeCommand(rootCmd, attachmentsCmd) in cmd/root.go
}
