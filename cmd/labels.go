package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// labelsCmd is the hand-written parent command for labels operations.
// mergeCommand(rootCmd, labelsCmd) is called from cmd/root.go init() (Plan 04).
var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "Confluence label operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema labels` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema labels` to list operations", cmd.CommandPath())
	},
}

var labels_list = &cobra.Command{
	Use:   "list",
	Short: "List labels on a page",
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
		path := fmt.Sprintf("/pages/%s/labels", url.PathEscape(pageID))
		code := c.Do(cmd.Context(), "GET", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// labelItem is the v1 label body format for POST /wiki/rest/api/content/{id}/label.
type labelItem struct {
	Prefix string `json:"prefix"`
	Name   string `json:"name"`
}

// fetchV1WithBody performs an HTTP request against a v1 URL (full absolute URL).
// It applies auth from c and writes error JSON to c.Stderr on failure.
// Used for POST and DELETE on /wiki/rest/api/content/{id}/label.
func fetchV1WithBody(cmd *cobra.Command, c *client.Client, method, fullURL string, body io.Reader) ([]byte, int) {
	req, err := http.NewRequestWithContext(cmd.Context(), method, fullURL, body)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to create request: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.ApplyAuth(req); err != nil {
		apiErr := &cferrors.APIError{ErrorType: "auth_error", Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitAuth
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "reading response body: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}

	if resp.StatusCode >= 400 {
		// Extract path from full URL for cleaner error reporting.
		errPath := fullURL
		if u, parseErr := url.Parse(fullURL); parseErr == nil {
			errPath = u.Path
		}
		apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(respBody)), method, errPath, resp)
		apiErr.WriteJSON(c.Stderr)
		return nil, apiErr.ExitCode()
	}

	// 204 No Content
	if len(respBody) == 0 || resp.StatusCode == http.StatusNoContent {
		respBody = []byte("{}")
	}

	return respBody, cferrors.ExitOK
}

var labels_add = &cobra.Command{
	Use:   "add",
	Short: "Add labels to a page (uses v1 API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		pageID, _ := cmd.Flags().GetString("page-id")
		labelNames, _ := cmd.Flags().GetStringSlice("label")
		if strings.TrimSpace(pageID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--page-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if len(labelNames) == 0 {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--label must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		var items []labelItem
		for _, n := range labelNames {
			if n != "" {
				items = append(items, labelItem{Prefix: "global", Name: n})
			}
		}
		if len(items) == 0 {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--label must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		encoded, _ := json.Marshal(items)

		// v1 API: c.BaseURL is "https://domain/wiki/api/v2", extract domain.
		domain := client.SearchV1Domain(c.BaseURL)
		fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/label", url.PathEscape(pageID))

		respBody, code := fetchV1WithBody(cmd, c, "POST", fullURL, bytes.NewReader(encoded))
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	},
}

var labels_remove = &cobra.Command{
	Use:   "remove",
	Short: "Remove a label from a page (uses v1 API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		pageID, _ := cmd.Flags().GetString("page-id")
		labelName, _ := cmd.Flags().GetString("label")
		if strings.TrimSpace(pageID) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--page-id must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(labelName) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--label must not be empty"}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		// v1 API: extract domain from c.BaseURL.
		domain := client.SearchV1Domain(c.BaseURL)
		fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/label?name=%s",
			url.PathEscape(pageID), url.QueryEscape(labelName))

		_, code := fetchV1WithBody(cmd, c, "DELETE", fullURL, nil)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		out, _ := json.Marshal(map[string]string{"status": "removed", "label": labelName})
		if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	},
}

func init() {
	labels_list.Flags().String("page-id", "", "Page ID to list labels for (required)")
	labels_add.Flags().String("page-id", "", "Page ID to add labels to (required)")
	labels_add.Flags().StringSlice("label", nil, "Label name to add (repeatable, e.g. --label foo --label bar)")
	labels_remove.Flags().String("page-id", "", "Page ID to remove label from (required)")
	labels_remove.Flags().String("label", "", "Label name to remove (required)")

	labelsCmd.AddCommand(labels_list)
	labelsCmd.AddCommand(labels_add)
	labelsCmd.AddCommand(labels_remove)
	// labelsCmd is registered via mergeCommand(rootCmd, labelsCmd) in cmd/root.go (Plan 04).
	// Do NOT call rootCmd.AddCommand here.
}
