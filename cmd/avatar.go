package cmd

import (
	"encoding/json"
	"strings"

	"github.com/sofq/confluence-cli/internal/avatar"
	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// avatarCmd is the parent command for user writing style profiling.
var avatarCmd = &cobra.Command{
	Use:   "avatar",
	Short: "User writing style profiling for AI agents",
}

// avatarAnalyzeCmd analyzes a Confluence user's writing style and outputs
// a JSON PersonaProfile to stdout.
var avatarAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze a Confluence user's writing style and output a JSON persona profile",
	RunE:  runAvatarAnalyze,
}

func runAvatarAnalyze(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	userFlag, _ := cmd.Flags().GetString("user")
	if strings.TrimSpace(userFlag) == "" {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   "--user must not be empty; provide a Confluence account ID",
		}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	pages, err := avatar.FetchUserPages(c, userFlag)
	if err != nil {
		// Classify error: 401/unauthorized/auth → ExitAuth, else ExitError.
		errStr := strings.ToLower(err.Error())
		exitCode := cferrors.ExitError
		errorType := "analysis_error"
		if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "auth") {
			exitCode = cferrors.ExitAuth
			errorType = "auth_failed"
		}
		apiErr := &cferrors.APIError{ErrorType: errorType, Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: exitCode}
	}

	profile := avatar.BuildProfile(userFlag, "", pages)

	out, err := json.Marshal(profile)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "analysis_error", Message: "failed to marshal profile: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

func init() {
	avatarAnalyzeCmd.Flags().String("user", "", "Confluence account ID to analyze (required)")
	avatarCmd.AddCommand(avatarAnalyzeCmd)
	// avatarCmd is registered into rootCmd in cmd/root.go init().
}
