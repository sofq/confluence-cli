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

// blogpostsCmd is the hand-written parent command for blog post operations.
// mergeCommand(rootCmd, blogpostsCmd) is called from cmd/root.go init() (Phase 7).
var blogpostsCmd = &cobra.Command{
	Use:   "blogposts",
	Short: "Confluence blog post operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema blogposts` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema blogposts` to list operations", cmd.CommandPath())
	},
}

// fetchBlogpostVersion fetches the current version number of a blog post.
// Uses GET /blogposts/{id} (v2 path; BaseURL already includes /wiki/api/v2).
func fetchBlogpostVersion(ctx context.Context, c *client.Client, id string) (int, int) {
	body, code := c.Fetch(ctx, "GET", fmt.Sprintf("/blogposts/%s", url.PathEscape(id)), nil)
	if code != cferrors.ExitOK {
		return 0, code
	}
	var post struct {
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &post); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   "failed to parse blog post version: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return 0, cferrors.ExitError
	}
	return post.Version.Number, cferrors.ExitOK
}

// blogpostUpdateBody is the request body for PUT /blogposts/{id}.
type blogpostUpdateBody struct {
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

// doBlogpostUpdate sends a PUT /blogposts/{id} request with the given parameters.
// Returns the exit code. On success, writes the response body to c.Stdout.
func doBlogpostUpdate(ctx context.Context, c *client.Client, id, title, storageValue string, versionNumber int) int {
	var reqBody blogpostUpdateBody
	reqBody.ID = id
	reqBody.Status = "current"
	reqBody.Title = title
	reqBody.Body.Representation = "storage"
	reqBody.Body.Value = storageValue
	reqBody.Version.Number = versionNumber
	encoded, _ := json.Marshal(reqBody)
	respBody, code := c.Fetch(ctx, "PUT", fmt.Sprintf("/blogposts/%s", url.PathEscape(id)), bytes.NewReader(encoded))
	if code != cferrors.ExitOK {
		return code
	}
	return c.WriteOutput(respBody)
}

// ---------------------------------------------------------------------------
// Subcommands
// ---------------------------------------------------------------------------

// blogposts_workflow_get_by_id retrieves a blog post by ID, always injecting body-format=storage.
var blogposts_workflow_get_by_id = &cobra.Command{
	Use:   "get-blog-post-by-id",
	Short: "Get blog post by ID with storage body",
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
		path := fmt.Sprintf("/blogposts/%s", url.PathEscape(id))
		code := c.Do(cmd.Context(), "GET", path, q, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// blogposts_workflow_create creates a blog post with friendly flags, building the JSON body internally.
var blogposts_workflow_create = &cobra.Command{
	Use:   "create-blog-post",
	Short: "Create a blog post with storage format body",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		spaceID, _ := cmd.Flags().GetString("space-id")
		title, _ := cmd.Flags().GetString("title")
		bodyVal, _ := cmd.Flags().GetString("body")

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

		// Build request body (no parent-id for blog posts).
		type createBody struct {
			SpaceID string `json:"spaceId"`
			Title   string `json:"title"`
			Body    struct {
				Representation string `json:"representation"`
				Value          string `json:"value"`
			} `json:"body"`
		}
		var reqBody createBody
		reqBody.SpaceID = spaceID
		reqBody.Title = title
		reqBody.Body.Representation = "storage"
		reqBody.Body.Value = bodyVal
		encoded, _ := json.Marshal(reqBody)
		respBody, code := c.Fetch(cmd.Context(), "POST", "/blogposts", bytes.NewReader(encoded))
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: ec}
		}
		return nil
	},
}

// blogposts_workflow_update updates a blog post with automatic version increment and single 409 retry.
var blogposts_workflow_update = &cobra.Command{
	Use:   "update-blog-post",
	Short: "Update a blog post with automatic version increment",
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
		currentVersion, code := fetchBlogpostVersion(cmd.Context(), c, id)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		// First attempt.
		code = doBlogpostUpdate(cmd.Context(), c, id, title, bodyVal, currentVersion+1)
		if code == cferrors.ExitConflict {
			// Single retry: re-fetch version and try once more.
			currentVersion, code = fetchBlogpostVersion(cmd.Context(), c, id)
			if code != cferrors.ExitOK {
				return &cferrors.AlreadyWrittenError{Code: code}
			}
			code = doBlogpostUpdate(cmd.Context(), c, id, title, bodyVal, currentVersion+1)
		}
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// blogposts_workflow_delete soft-deletes a blog post (moves to trash) via HTTP DELETE.
var blogposts_workflow_delete = &cobra.Command{
	Use:   "delete-blog-post",
	Short: "Delete a blog post (moves to trash)",
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
		path := fmt.Sprintf("/blogposts/%s", url.PathEscape(id))
		code := c.Do(cmd.Context(), "DELETE", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// blogposts_workflow_list lists blog posts, optionally filtered by space-id, with auto-pagination.
var blogposts_workflow_list = &cobra.Command{
	Use:   "get-blog-posts",
	Short: "List blog posts in a space",
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
		code := c.Do(cmd.Context(), "GET", "/blogposts", q, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

func init() {
	// get-blog-post-by-id flags
	blogposts_workflow_get_by_id.Flags().String("id", "", "Blog post ID (required)")
	blogposts_workflow_get_by_id.Flags().String("body-format", "storage", "Body format (default: storage)")

	// create-blog-post flags
	blogposts_workflow_create.Flags().String("space-id", "", "Space ID to create blog post in (required)")
	blogposts_workflow_create.Flags().String("title", "", "Blog post title (required)")
	blogposts_workflow_create.Flags().String("body", "", "Blog post body in storage format XML (required)")

	// update-blog-post flags
	blogposts_workflow_update.Flags().String("id", "", "Blog post ID to update (required)")
	blogposts_workflow_update.Flags().String("title", "", "Blog post title (required)")
	blogposts_workflow_update.Flags().String("body", "", "Blog post body in storage format XML (required)")

	// delete-blog-post flags
	blogposts_workflow_delete.Flags().String("id", "", "Blog post ID to delete (required)")

	// get-blog-posts (list) flags
	blogposts_workflow_list.Flags().String("space-id", "", "Filter blog posts by space ID")

	// Register all subcommands on blogpostsCmd.
	blogpostsCmd.AddCommand(blogposts_workflow_get_by_id)
	blogpostsCmd.AddCommand(blogposts_workflow_create)
	blogpostsCmd.AddCommand(blogposts_workflow_update)
	blogpostsCmd.AddCommand(blogposts_workflow_delete)
	blogpostsCmd.AddCommand(blogposts_workflow_list)
}
