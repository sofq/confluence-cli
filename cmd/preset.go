package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jq"
	preset_pkg "github.com/sofq/confluence-cli/internal/preset"
	"github.com/spf13/cobra"
)

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Manage output presets",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; available: list", cmd.CommandPath())
	},
}

var presetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available output presets",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve profile directly (no API client needed).
		profileName, _ := cmd.Flags().GetString("profile")
		resolved, err := config.Resolve(config.DefaultPath(), profileName, &config.FlagOverrides{})
		if err != nil {
			// Non-fatal: list built-in presets only if config fails.
			resolved = &config.ResolvedConfig{}
		}
		var rawProfile config.Profile
		if cfg, loadErr := config.LoadFrom(config.DefaultPath()); loadErr == nil {
			rawProfile = cfg.Profiles[resolved.ProfileName]
		}

		data, err := preset_pkg.List(rawProfile.Presets)
		if err != nil {
			apiErr := &cferrors.APIError{ErrorType: "config_error", Message: "failed to list presets: " + err.Error()}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		jqFilter, _ := cmd.Flags().GetString("jq")
		prettyFlag, _ := cmd.Flags().GetBool("pretty")
		if jqFilter != "" {
			filtered, err := jq.Apply(data, jqFilter)
			if err != nil {
				apiErr := &cferrors.APIError{ErrorType: "jq_error", Message: "jq: " + err.Error()}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
			data = filtered
		}
		if prettyFlag {
			var out bytes.Buffer
			if jsonErr := json.Indent(&out, data, "", "  "); jsonErr == nil {
				data = out.Bytes()
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.TrimRight(string(data), "\n"))
		return nil
	},
}

func init() {
	presetCmd.AddCommand(presetListCmd)
}
