package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/duration"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// workflowCmd is the parent command for content lifecycle operations.
// Registered to root via rootCmd.AddCommand(workflowCmd) in cmd/root.go.
var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Content lifecycle operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; available: move, copy, publish, comment, restrict, archive", cmd.CommandPath())
	},
}

// ---------------------------------------------------------------------------
// Move (WKFL-01) -- v1 API
// PUT /wiki/rest/api/content/{id}/move/append/{targetId}
// ---------------------------------------------------------------------------

var workflow_move = &cobra.Command{
	Use:   "move",
	Short: "Move a page to a different parent",
	RunE:  runWorkflowMove,
}

func runWorkflowMove(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	targetID, _ := cmd.Flags().GetString("target-id")

	if strings.TrimSpace(id) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	if strings.TrimSpace(targetID) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--target-id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	domain := client.SearchV1Domain(c.BaseURL)
	fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/move/append/%s",
		url.PathEscape(id), url.PathEscape(targetID))

	respBody, code := fetchV1WithBody(cmd, c, "PUT", fullURL, nil)
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}
	if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Copy (WKFL-02) -- v1 API, async
// POST /wiki/rest/api/content/{id}/copy
// ---------------------------------------------------------------------------

// copyRequestBody is the v1 copy API request body.
type copyRequestBody struct {
	CopyAttachments    bool            `json:"copyAttachments"`
	CopyPermissions    bool            `json:"copyPermissions"`
	CopyLabels         bool            `json:"copyLabels"`
	CopyProperties     bool            `json:"copyProperties"`
	CopyCustomContents bool            `json:"copyCustomContents"`
	Destination        copyDestination `json:"destination"`
	PageTitle          string          `json:"pageTitle,omitempty"`
}

type copyDestination struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

var workflow_copy = &cobra.Command{
	Use:   "copy",
	Short: "Copy a page to a target parent",
	RunE:  runWorkflowCopy,
}

func runWorkflowCopy(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	targetID, _ := cmd.Flags().GetString("target-id")
	title, _ := cmd.Flags().GetString("title")
	copyAttachments, _ := cmd.Flags().GetBool("copy-attachments")
	copyLabels, _ := cmd.Flags().GetBool("copy-labels")
	copyPermissions, _ := cmd.Flags().GetBool("copy-permissions")
	noWait, _ := cmd.Flags().GetBool("no-wait")
	timeoutStr, _ := cmd.Flags().GetString("timeout")

	if strings.TrimSpace(id) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	if strings.TrimSpace(targetID) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--target-id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	reqBody := copyRequestBody{
		CopyAttachments:    copyAttachments,
		CopyPermissions:    copyPermissions,
		CopyLabels:         copyLabels,
		CopyProperties:     false,
		CopyCustomContents: false,
		Destination: copyDestination{
			Type:  "parent_page",
			Value: targetID,
		},
		PageTitle: title,
	}

	encoded, _ := json.Marshal(reqBody)

	domain := client.SearchV1Domain(c.BaseURL)
	fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/copy", url.PathEscape(id))

	respBody, code := fetchV1WithBody(cmd, c, "POST", fullURL, bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}

	if noWait {
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	}

	// Parse long task ID from response.
	var taskResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &taskResp); err != nil || strings.TrimSpace(taskResp.ID) == "" {
		// If no task ID found, return raw response (may already be complete).
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	}

	timeout, err := duration.Parse(timeoutStr)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "invalid --timeout: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	taskBody, taskCode := pollLongTask(cmd.Context(), cmd, c, taskResp.ID, timeout)
	if taskCode != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: taskCode}
	}
	if ec := c.WriteOutput(taskBody); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Publish (WKFL-03) -- v2 API
// GET /pages/{id} then PUT /pages/{id} with status "current"
// ---------------------------------------------------------------------------

var workflow_publish = &cobra.Command{
	Use:   "publish",
	Short: "Publish a draft page",
	RunE:  runWorkflowPublish,
}

func runWorkflowPublish(cmd *cobra.Command, args []string) error {
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

	ctx := cmd.Context()

	// Fetch current page to get title and version.
	body, code := c.Fetch(ctx, "GET", fmt.Sprintf("/pages/%s", url.PathEscape(id)), nil)
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}

	var page struct {
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse page response: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	// Build publish request body.
	var reqBody struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	}
	reqBody.ID = id
	reqBody.Status = "current"
	reqBody.Title = page.Title
	reqBody.Version.Number = page.Version.Number + 1

	encoded, _ := json.Marshal(reqBody)
	respBody, code := c.Fetch(ctx, "PUT", fmt.Sprintf("/pages/%s", url.PathEscape(id)), bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}
	if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Comment (WKFL-04) -- v2 API
// POST /footer-comments with <p>-wrapped body
// ---------------------------------------------------------------------------

var workflow_comment = &cobra.Command{
	Use:   "comment",
	Short: "Add a plain-text comment to a page",
	RunE:  runWorkflowComment,
}

func runWorkflowComment(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	bodyText, _ := cmd.Flags().GetString("body")

	if strings.TrimSpace(id) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	if strings.TrimSpace(bodyText) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--body must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Wrap plain text in storage format paragraph tags.
	storageBody := "<p>" + bodyText + "</p>"

	var reqBody createCommentBody
	reqBody.PageID = id
	reqBody.Body.Representation = "storage"
	reqBody.Body.Value = storageBody

	encoded, _ := json.Marshal(reqBody)
	respBody, code := c.Fetch(cmd.Context(), "POST", "/footer-comments", bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}
	if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Restrict (WKFL-05) -- v1 API
// GET/PUT/DELETE /wiki/rest/api/content/{id}/restriction...
// ---------------------------------------------------------------------------

var workflow_restrict = &cobra.Command{
	Use:   "restrict",
	Short: "View, add, or remove page restrictions",
	RunE:  runWorkflowRestrict,
}

func runWorkflowRestrict(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	addFlag, _ := cmd.Flags().GetBool("add")
	removeFlag, _ := cmd.Flags().GetBool("remove")
	operation, _ := cmd.Flags().GetString("operation")
	user, _ := cmd.Flags().GetString("user")
	group, _ := cmd.Flags().GetString("group")

	if strings.TrimSpace(id) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Validate mutual exclusivity.
	if addFlag && removeFlag {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--add and --remove are mutually exclusive"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	domain := client.SearchV1Domain(c.BaseURL)

	// View mode: no --add, no --remove.
	if !addFlag && !removeFlag {
		fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/restriction", url.PathEscape(id))
		respBody, code := fetchV1WithBody(cmd, c, "GET", fullURL, nil)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	}

	// Add or remove mode: require --operation and at least one of --user/--group.
	if strings.TrimSpace(operation) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--operation must not be empty when using --add or --remove"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	if operation != "read" && operation != "update" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--operation must be 'read' or 'update'"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}
	if strings.TrimSpace(user) == "" && strings.TrimSpace(group) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "at least one of --user or --group must be provided"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	method := http.MethodPut
	statusWord := "added"
	if removeFlag {
		method = http.MethodDelete
		statusWord = "removed"
	}

	// Apply restriction for user.
	if strings.TrimSpace(user) != "" {
		fullURL := domain + fmt.Sprintf(
			"/wiki/rest/api/content/%s/restriction/byOperation/%s/user?accountId=%s",
			url.PathEscape(id),
			url.PathEscape(operation),
			url.QueryEscape(user),
		)
		_, code := fetchV1WithBody(cmd, c, method, fullURL, nil)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		out, _ := json.Marshal(map[string]string{"status": statusWord, "operation": operation, "user": user})
		if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
	}

	// Apply restriction for group.
	if strings.TrimSpace(group) != "" {
		fullURL := domain + fmt.Sprintf(
			"/wiki/rest/api/content/%s/restriction/byOperation/%s/byGroupId/%s",
			url.PathEscape(id),
			url.PathEscape(operation),
			url.PathEscape(group),
		)
		_, code := fetchV1WithBody(cmd, c, method, fullURL, nil)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		out, _ := json.Marshal(map[string]string{"status": statusWord, "operation": operation, "group": group})
		if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Archive (WKFL-06) -- v1 API, async
// POST /wiki/rest/api/content/archive
// ---------------------------------------------------------------------------

// archiveRequest is the v1 archive API request body.
type archiveRequest struct {
	Pages []archivePage `json:"pages"`
}

type archivePage struct {
	ID string `json:"id"`
}

var workflow_archive = &cobra.Command{
	Use:   "archive",
	Short: "Archive a page",
	RunE:  runWorkflowArchive,
}

func runWorkflowArchive(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	noWait, _ := cmd.Flags().GetBool("no-wait")
	timeoutStr, _ := cmd.Flags().GetString("timeout")

	if strings.TrimSpace(id) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	reqBody := archiveRequest{
		Pages: []archivePage{{ID: id}},
	}
	encoded, _ := json.Marshal(reqBody)

	domain := client.SearchV1Domain(c.BaseURL)
	fullURL := domain + "/wiki/rest/api/content/archive"

	respBody, code := fetchV1WithBody(cmd, c, "POST", fullURL, bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}

	if noWait {
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	}

	// Parse long task ID from response.
	var taskResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &taskResp); err != nil || strings.TrimSpace(taskResp.ID) == "" {
		// If no task ID found, return raw response (may already be complete).
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	}

	timeout, err := duration.Parse(timeoutStr)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "invalid --timeout: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	taskBody, taskCode := pollLongTask(cmd.Context(), cmd, c, taskResp.ID, timeout)
	if taskCode != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: taskCode}
	}
	if ec := c.WriteOutput(taskBody); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

// ---------------------------------------------------------------------------
// pollLongTask -- polls a v1 long-running task until finished or timeout
// ---------------------------------------------------------------------------

func pollLongTask(ctx context.Context, cmd *cobra.Command, c *client.Client, taskID string, timeout time.Duration) ([]byte, int) {
	domain := client.SearchV1Domain(c.BaseURL)
	deadline := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			apiErr := &cferrors.APIError{ErrorType: "timeout_error", Message: fmt.Sprintf("operation timed out after %s", timeout)}
			apiErr.WriteJSON(c.Stderr)
			return nil, cferrors.ExitError
		case <-ctx.Done():
			return nil, cferrors.ExitError
		case <-ticker.C:
			taskURL := domain + fmt.Sprintf("/wiki/rest/api/longtask/%s", url.PathEscape(taskID))
			body, code := fetchV1WithBody(cmd, c, "GET", taskURL, nil)
			if code != cferrors.ExitOK {
				return nil, code
			}
			var task struct {
				Successful bool `json:"successful"`
				Finished   bool `json:"finished"`
			}
			if err := json.Unmarshal(body, &task); err != nil {
				return body, cferrors.ExitOK // return raw if unparseable
			}
			if task.Finished {
				if !task.Successful {
					apiErr := &cferrors.APIError{ErrorType: "api_error", Message: "long-running task failed"}
					apiErr.WriteJSON(c.Stderr)
					return nil, cferrors.ExitError
				}
				return body, cferrors.ExitOK
			}
		}
	}
}

// ---------------------------------------------------------------------------
// init -- register flags and wire subcommands to parent
// ---------------------------------------------------------------------------

func init() {
	// move flags
	workflow_move.Flags().String("id", "", "page ID to move (required)")
	workflow_move.Flags().String("target-id", "", "target parent page ID (required)")

	// copy flags
	workflow_copy.Flags().String("id", "", "page ID to copy (required)")
	workflow_copy.Flags().String("target-id", "", "target parent page ID (required)")
	workflow_copy.Flags().String("title", "", "title for the copied page")
	workflow_copy.Flags().Bool("copy-attachments", false, "include attachments in copy")
	workflow_copy.Flags().Bool("copy-labels", false, "include labels in copy")
	workflow_copy.Flags().Bool("copy-permissions", false, "include permissions in copy")
	workflow_copy.Flags().Bool("no-wait", false, "return immediately without polling")
	workflow_copy.Flags().String("timeout", "60s", "timeout for async operation (e.g. 30s, 2m)")

	// publish flags
	workflow_publish.Flags().String("id", "", "page ID to publish (required)")

	// comment flags
	workflow_comment.Flags().String("id", "", "page ID to comment on (required)")
	workflow_comment.Flags().String("body", "", "comment text (required)")

	// restrict flags
	workflow_restrict.Flags().String("id", "", "page ID to manage restrictions (required)")
	workflow_restrict.Flags().Bool("add", false, "add a restriction")
	workflow_restrict.Flags().Bool("remove", false, "remove a restriction")
	workflow_restrict.Flags().String("operation", "", "restriction operation: read or update")
	workflow_restrict.Flags().String("user", "", "user account ID")
	workflow_restrict.Flags().String("group", "", "group name")

	// archive flags
	workflow_archive.Flags().String("id", "", "page ID to archive (required)")
	workflow_archive.Flags().Bool("no-wait", false, "return immediately without polling")
	workflow_archive.Flags().String("timeout", "60s", "timeout for async operation (e.g. 30s, 2m)")

	workflowCmd.AddCommand(workflow_move)
	workflowCmd.AddCommand(workflow_copy)
	workflowCmd.AddCommand(workflow_publish)
	workflowCmd.AddCommand(workflow_comment)
	workflowCmd.AddCommand(workflow_restrict)
	workflowCmd.AddCommand(workflow_archive)
}
