package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// spacesCmd is the hand-written parent command for spaces operations.
// It overrides the generated parent while preserving generated subcommands
// (wired by Plan 04 via mergeCommand).
var spacesCmd = &cobra.Command{
	Use:   "spaces",
	Short: "Confluence space operations",
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q; run `cf schema spaces` to list operations", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; run `cf schema spaces` to list operations", cmd.CommandPath())
	},
}

// resolveSpaceID transparently resolves a space key (e.g. "ENG") or numeric ID
// string to a numeric ID string. If keyOrID is already numeric it is returned
// as-is. Alpha keys are resolved via GET /spaces?keys=<KEY> which returns the
// first matching space's numeric ID.
func resolveSpaceID(ctx context.Context, c *client.Client, keyOrID string) (string, int) {
	// Numeric string: pass through unchanged (SPCE-03).
	if _, err := strconv.ParseInt(keyOrID, 10, 64); err == nil {
		return keyOrID, cferrors.ExitOK
	}

	// Alpha key: resolve via API.
	path := fmt.Sprintf("/spaces?keys=%s", url.QueryEscape(keyOrID))
	body, code := c.Fetch(ctx, "GET", path, nil)
	if code != cferrors.ExitOK {
		return "", code
	}

	var resp struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Results) == 0 {
		apiErr := &cferrors.APIError{
			ErrorType: "not_found",
			Message:   fmt.Sprintf("no space found with key %q", keyOrID),
		}
		apiErr.WriteJSON(os.Stderr)
		return "", cferrors.ExitNotFound
	}

	return resp.Results[0].ID, cferrors.ExitOK
}

// spaces_workflow_list lists all spaces with auto-pagination, or resolves a
// space key and returns that single space's details when --key is provided.
var spaces_workflow_list = &cobra.Command{
	Use:   "get",
	Short: "List spaces (or look up a specific space by key)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		key, _ := cmd.Flags().GetString("key")
		if key != "" {
			// Resolve space key to numeric ID, then return that single space.
			id, code := resolveSpaceID(cmd.Context(), c, key)
			if code != cferrors.ExitOK {
				return &cferrors.AlreadyWrittenError{Code: code}
			}
			path := fmt.Sprintf("/spaces/%s", url.PathEscape(id))
			code = c.Do(cmd.Context(), "GET", path, nil, nil)
			if code != 0 {
				return &cferrors.AlreadyWrittenError{Code: code}
			}
			return nil
		}
		// No key: list all spaces with auto-pagination.
		code := c.Do(cmd.Context(), "GET", "/spaces", nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

// spaces_workflow_get_by_id fetches a single space by numeric ID or alpha key.
// The --id flag accepts either form; alpha keys are resolved via resolveSpaceID.
var spaces_workflow_get_by_id = &cobra.Command{
	Use:   "get-by-id",
	Short: "Get space by ID or key",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.FromContext(cmd.Context())
		if err != nil {
			return err
		}
		idOrKey, _ := cmd.Flags().GetString("id")
		if strings.TrimSpace(idOrKey) == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "--id must not be empty",
			}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		id, code := resolveSpaceID(cmd.Context(), c, idOrKey)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		path := fmt.Sprintf("/spaces/%s", url.PathEscape(id))
		code = c.Do(cmd.Context(), "GET", path, nil, nil)
		if code != 0 {
			return &cferrors.AlreadyWrittenError{Code: code}
		}
		return nil
	},
}

func init() {
	// spaces_workflow_list flags.
	spaces_workflow_list.Flags().String("key", "", "Resolve space key to ID and return that space (e.g. ENG)")

	// spaces_workflow_get_by_id flags.
	spaces_workflow_get_by_id.Flags().String("id", "", "Space ID or key (required)")

	// Register workflow subcommands on spacesCmd.
	// Do NOT call mergeCommand or rootCmd.AddCommand here — Plan 04 handles wiring.
	spacesCmd.AddCommand(spaces_workflow_list)
	spacesCmd.AddCommand(spaces_workflow_get_by_id)
}
