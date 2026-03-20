package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/cmd/generated"
	"github.com/sofq/confluence-cli/internal/audit"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/oauth2"
	"github.com/sofq/confluence-cli/internal/policy"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags: -X github.com/sofq/confluence-cli/cmd.Version=<ver>
var Version = "dev"

// skipClientCommands are command names that do not require a configured client.
var skipClientCommands = map[string]bool{
	"configure":  true,
	"version":    true,
	"completion": true,
	"help":       true,
	"schema":     true,
	"templates":  true,
}

var rootCmd = &cobra.Command{
	Use:           "cf",
	Short:         "Agent-friendly Confluence CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip client injection for built-in / setup commands.
		name := cmd.Name()
		if skipClientCommands[name] {
			return nil
		}
		// Also skip for subcommands of skipped commands (e.g., completion bash)
		if cmd.Parent() != nil && skipClientCommands[cmd.Parent().Name()] {
			return nil
		}

		// Read flag overrides.
		baseURL, _ := cmd.Flags().GetString("base-url")
		authType, _ := cmd.Flags().GetString("auth-type")
		authUser, _ := cmd.Flags().GetString("auth-user")
		authToken, _ := cmd.Flags().GetString("auth-token")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		cloudID, _ := cmd.Flags().GetString("cloud-id")

		profileName, _ := cmd.Flags().GetString("profile")
		jqFilter, _ := cmd.Flags().GetString("jq")
		preset, _ := cmd.Flags().GetString("preset")
		pretty, _ := cmd.Flags().GetBool("pretty")
		noPaginate, _ := cmd.Flags().GetBool("no-paginate")
		verbose, _ := cmd.Flags().GetBool("verbose")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		fields, _ := cmd.Flags().GetString("fields")
		cacheTTL, _ := cmd.Flags().GetDuration("cache")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		auditFlag, _ := cmd.Flags().GetString("audit")

		flags := &config.FlagOverrides{
			BaseURL:      baseURL,
			AuthType:     authType,
			Username:     authUser,
			Token:        authToken,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			CloudID:      cloudID,
		}

		resolved, err := config.Resolve(config.DefaultPath(), profileName, flags)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "config_error",
				Status:    0,
				Message:   "failed to resolve config: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		if resolved.BaseURL == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "config_error",
				Status:    0,
				Message:   "base_url is not set; run `cf configure --base-url <url> --token <token>` or set CF_BASE_URL",
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		// OAuth2 token resolution: fetch/cache token, switch to bearer for downstream.
		if resolved.Auth.Type == "oauth2" || resolved.Auth.Type == "oauth2-3lo" {
			store := oauth2.NewFileStore(config.TokenDir(), resolved.ProfileName)
			var token *oauth2.Token
			var tokenErr error

			switch resolved.Auth.Type {
			case "oauth2":
				token, tokenErr = oauth2.ClientCredentials(
					resolved.Auth.ClientID,
					resolved.Auth.ClientSecret,
					resolved.Auth.Scopes,
					store,
				)
			case "oauth2-3lo":
				token, tokenErr = oauth2.ThreeLO(
					resolved.Auth.ClientID,
					resolved.Auth.ClientSecret,
					resolved.Auth.Scopes,
					resolved.Auth.CloudID,
					store,
				)
			}
			if tokenErr != nil {
				apiErr := &cferrors.APIError{
					ErrorType: "auth_error",
					Status:    0,
					Message:   "OAuth2 token fetch failed: " + tokenErr.Error(),
				}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitAuth}
			}

			// Switch to bearer for downstream Client.
			resolved.Auth.Type = "bearer"
			resolved.Auth.Token = token.AccessToken

			// Determine cloudID: from config, or discovered during 3LO flow.
			effectiveCloudID := resolved.Auth.CloudID
			if effectiveCloudID == "" && token.CloudID != "" {
				effectiveCloudID = token.CloudID
			}
			if effectiveCloudID == "" {
				apiErr := &cferrors.APIError{
					ErrorType: "config_error",
					Status:    0,
					Message:   "cloud_id is required for OAuth2; set via config, CF_AUTH_CLOUD_ID, or --cloud-id flag",
				}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}

			// Switch base URL to Atlassian API proxy.
			resolved.BaseURL = fmt.Sprintf(
				"https://api.atlassian.com/ex/confluence/%s/wiki/rest/api/v2",
				effectiveCloudID,
			)
		}

		// Load raw profile for governance fields (AllowedOperations, DeniedOperations, AuditLog).
		// If load fails, silently use zero Profile — governance fields are additive, not required.
		var rawProfile config.Profile
		if cfg, loadErr := config.LoadFrom(config.DefaultPath()); loadErr == nil {
			rawProfile = cfg.Profiles[resolved.ProfileName]
		}

		// Resolve --preset to JQ expression.
		if preset != "" {
			if jqFilter != "" {
				apiErr := &cferrors.APIError{
					ErrorType: "validation_error",
					Message:   "cannot use --preset and --jq together; choose one",
				}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
			expr, ok := rawProfile.Presets[preset]
			if !ok {
				apiErr := &cferrors.APIError{
					ErrorType: "config_error",
					Message:   fmt.Sprintf("preset %q not found in profile %q; available presets: %s", preset, resolved.ProfileName, availablePresets(rawProfile)),
				}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
			}
			jqFilter = expr
		}

		// Policy enforcement — build from profile config.
		pol, err := policy.NewFromConfig(rawProfile.AllowedOperations, rawProfile.DeniedOperations)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "config_error",
				Status:    0,
				Message:   "invalid policy config: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		// Audit logging — --audit flag takes precedence over profile audit_log.
		var auditLogger *audit.Logger
		auditPath := auditFlag
		if auditPath == "" {
			auditPath = rawProfile.AuditLog
		}
		if auditPath != "" {
			auditLogger, err = audit.NewLogger(auditPath)
			if err != nil {
				apiErr := &cferrors.APIError{
					ErrorType: "config_error",
					Status:    0,
					Message:   "cannot open audit log: " + err.Error(),
				}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
			}
		}

		c := &client.Client{
			BaseURL:     resolved.BaseURL,
			Auth:        resolved.Auth,
			HTTPClient:  &http.Client{Timeout: timeout},
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
			JQFilter:    jqFilter,
			Paginate:    !noPaginate,
			DryRun:      dryRun,
			Verbose:     verbose,
			Pretty:      pretty,
			Fields:      fields,
			CacheTTL:    cacheTTL,
			Policy:      pol,
			AuditLogger: auditLogger,
			Profile:     resolved.ProfileName,
		}

		cmd.SetContext(client.NewContext(cmd.Context(), c))
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if c, err := client.FromContext(cmd.Context()); err == nil {
			if c.AuditLogger != nil {
				c.AuditLogger.Close()
			}
		}
	},
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringP("profile", "p", "", "config profile to use")
	pf.String("base-url", "", "Confluence base URL (overrides config)")
	pf.String("auth-type", "", "auth type: basic, bearer, oauth2, oauth2-3lo (overrides config)")
	pf.String("auth-user", "", "username for basic auth (overrides config)")
	pf.String("auth-token", "", "API token or bearer token (overrides config)")
	pf.String("client-id", "", "OAuth2 client ID (overrides config)")
	pf.String("client-secret", "", "OAuth2 client secret (overrides config)")
	pf.String("cloud-id", "", "Atlassian Cloud site ID (overrides config)")
	pf.String("jq", "", "jq filter expression to apply to the response")
	pf.String("preset", "", "named output preset to apply (defined in profile config)")
	pf.Bool("pretty", false, "pretty-print JSON output")
	pf.Bool("no-paginate", false, "disable automatic pagination")
	pf.Bool("verbose", false, "log HTTP request/response details to stderr")
	pf.Bool("dry-run", false, "print the request as JSON without executing it")
	pf.String("fields", "", "comma-separated list of fields to return (GET only)")
	pf.Duration("cache", 0, "cache GET responses for this duration (e.g. 5m, 1h)")
	pf.Duration("timeout", 30*time.Second, "HTTP request timeout (e.g. 10s, 1m)")
	pf.String("audit", "", "path to NDJSON audit log file (overrides profile audit_log)")

	// Override --version template to output JSON.
	rootCmd.SetVersionTemplate(`{"version":"{{.Version}}"}` + "\n")

	// Register generated commands first, then replace version parent
	// with hand-written one while preserving generated subcommands.
	generated.RegisterAll(rootCmd)

	// Merge hand-written version with generated subcommands.
	mergeCommand(rootCmd, versionCmd)

	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(rawCmd)

	// Phase 3: workflow command overrides for primary resources
	mergeCommand(rootCmd, pagesCmd)    // replaces generated pages parent, preserves generated subcommands not overridden
	mergeCommand(rootCmd, spacesCmd)   // replaces generated spaces parent
	mergeCommand(rootCmd, commentsCmd) // replaces generated comments parent (use "comments" not "footer-comments")
	mergeCommand(rootCmd, labelsCmd)   // replaces generated labels parent
	rootCmd.AddCommand(searchCmd)      // no generated search command exists — add directly
	rootCmd.AddCommand(avatarCmd)      // Phase 5: user writing style profiling
	mergeCommand(rootCmd, blogpostsCmd) // Phase 7: blog post workflow overrides
	mergeCommand(rootCmd, attachmentsCmd)    // Phase 8: attachment workflow overrides
	mergeCommand(rootCmd, custom_contentCmd) // Phase 9: custom content workflow overrides
	rootCmd.AddCommand(templatesCmd)              // Phase 10: content template operations

	// Override cobra's default help output so that "cf" with no args and
	// "cf help <resource>" emit JSON errors to stderr instead of plain text
	// to stdout. This preserves the JSON-only stdout contract.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd {
			// Output a helpful JSON hint and exit 0 for explicit --help / help.
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(map[string]string{
				"hint":    "use `cf schema` to discover commands, or `cf schema <resource>` for operations on a resource",
				"version": Version,
			})
			fmt.Fprintf(os.Stdout, "%s", buf.String())
			return
		}
		// Write help text to stderr so stdout stays JSON-only.
		cmd.SetOut(os.Stderr)
		defaultHelp(cmd, args)
	})
}

// RootCommand returns the root cobra.Command for documentation generation.
func RootCommand() *cobra.Command {
	return rootCmd
}

// availablePresets returns a comma-separated list of preset names from a profile.
func availablePresets(p config.Profile) string {
	if len(p.Presets) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(p.Presets))
	for k := range p.Presets {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Execute runs the root command and returns an exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		if aw, ok := err.(*cferrors.AlreadyWrittenError); ok {
			return aw.Code
		}
		enc := json.NewEncoder(os.Stderr)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]string{
			"error_type": "command_error",
			"message":    err.Error(),
		})
		return cferrors.ExitError
	}
	return cferrors.ExitOK
}

// mergeCommand replaces a generated parent command on root with a hand-written
// one, while preserving any generated subcommands that are not already present
// on the hand-written command.
func mergeCommand(root *cobra.Command, handWritten *cobra.Command) {
	name := handWritten.Name()
	// Find existing generated command.
	for _, c := range root.Commands() {
		if c.Name() == name {
			// Copy generated subcommands to hand-written command.
			existingSubs := make(map[string]bool)
			for _, sub := range handWritten.Commands() {
				existingSubs[sub.Name()] = true
			}
			for _, sub := range c.Commands() {
				if !existingSubs[sub.Name()] {
					handWritten.AddCommand(sub)
				}
			}
			root.RemoveCommand(c)
			break
		}
	}
	root.AddCommand(handWritten)
}
