package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jq"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	cftemplate "github.com/sofq/confluence-cli/internal/template"
	"github.com/spf13/cobra"
)

// templatesCmd is the parent command for template operations.
var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Content template operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		}
		return fmt.Errorf("missing subcommand for %q; available: list, show, create", cmd.CommandPath())
	},
}

// templates_list lists available templates from the templates directory.
var templates_list = &cobra.Command{
	Use:   "list",
	Short: "List available content templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := cftemplate.List()
		if err != nil {
			apiErr := &cferrors.APIError{ErrorType: "config_error", Message: "failed to list templates: " + err.Error()}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}
		// MarshalNoEscape on template entries cannot fail (basic field types).
		data, _ := jsonutil.MarshalNoEscape(entries)

		jqFilter, _ := cmd.Flags().GetString("jq")
		prettyFlag, _ := cmd.Flags().GetBool("pretty")
		if jqFilter != "" {
			filtered, jqErr := jq.Apply(data, jqFilter)
			if jqErr != nil {
				apiErr := &cferrors.APIError{ErrorType: "jq_error", Message: "jq: " + jqErr.Error()}
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

// templatesShowCmd shows a template's full definition including variables.
var templatesShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a template's full definition including variables",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		output, err := cftemplate.Show(name)
		if err != nil {
			apiErr := &cferrors.APIError{ErrorType: "not_found", Message: err.Error()}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitNotFound}
		}

		// MarshalNoEscape on template output cannot fail (basic field types).
		data, _ := jsonutil.MarshalNoEscape(output)

		jqFilter, _ := cmd.Flags().GetString("jq")
		prettyFlag, _ := cmd.Flags().GetBool("pretty")
		if jqFilter != "" {
			filtered, jqErr := jq.Apply(data, jqFilter)
			if jqErr != nil {
				apiErr := &cferrors.APIError{ErrorType: "jq_error", Message: "jq: " + jqErr.Error()}
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

// templatesCreateCmd creates a template from an existing Confluence page.
var templatesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a template from an existing page",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromPage, _ := cmd.Flags().GetString("from-page")
		name, _ := cmd.Flags().GetString("name")

		if strings.TrimSpace(name) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--name must not be empty"}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		if strings.TrimSpace(fromPage) == "" {
			apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--from-page must not be empty"}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}

		// Manual client construction for templates create (since "templates" is
		// in skipClientCommands and PersistentPreRunE does not inject a client).
		profileName, _ := cmd.Flags().GetString("profile")
		resolved, err := config.Resolve(config.DefaultPath(), profileName, &config.FlagOverrides{})
		if err != nil {
			apiErr := &cferrors.APIError{ErrorType: "config_error", Message: "failed to resolve config: " + err.Error()}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}
		c := &client.Client{
			BaseURL:    resolved.BaseURL,
			Auth:       resolved.Auth,
			HTTPClient: &http.Client{Timeout: 30 * time.Second},
			Stdout:     cmd.OutOrStdout(),
			Stderr:     os.Stderr,
		}

		// Fetch page with storage body format.
		path := fmt.Sprintf("/pages/%s?body-format=storage", url.PathEscape(fromPage))
		body, code := c.Fetch(cmd.Context(), "GET", path, nil)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		// Extract title and storage body.
		var page struct {
			Title string `json:"title"`
			Body  struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
		}
		if jsonErr := json.Unmarshal(body, &page); jsonErr != nil {
			apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse page response: " + jsonErr.Error()}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		tmpl := &cftemplate.Template{
			Title: page.Title,
			Body:  page.Body.Storage.Value,
			// SpaceID intentionally empty per D-08 (reusable across spaces)
		}

		if saveErr := cftemplate.Save(name, tmpl); saveErr != nil {
			apiErr := &cferrors.APIError{ErrorType: "config_error", Message: "failed to save template: " + saveErr.Error()}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}

		out, _ := jsonutil.MarshalNoEscape(map[string]string{
			"status":   "created",
			"template": name,
			"path":     filepath.Join(cftemplate.Dir(), name+".json"),
		})
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out)
		return nil
	},
}

// resolveTemplate loads a template by name, parses --var flags into a map,
// renders the template, and returns the rendered result. It writes JSON errors
// to w and returns an AlreadyWrittenError on failure.
func resolveTemplate(w io.Writer, templateName string, varFlags []string) (*cftemplate.RenderedTemplate, error) {
	if w == nil {
		w = os.Stderr
	}
	varMap := make(map[string]string)
	for _, v := range varFlags {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   fmt.Sprintf("invalid --var format %q: expected key=value", v),
			}
			apiErr.WriteJSON(w)
			return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		varMap[parts[0]] = parts[1]
	}

	tmpl, err := cftemplate.Load(templateName)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Message:   "template not found: " + err.Error(),
		}
		apiErr.WriteJSON(w)
		return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	rendered, err := cftemplate.Render(tmpl, varMap)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   "template render failed: " + err.Error(),
		}
		apiErr.WriteJSON(w)
		return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	return rendered, nil
}

func init() {
	templatesCreateCmd.Flags().String("from-page", "", "page ID to create template from (required)")
	templatesCreateCmd.Flags().String("name", "", "template name (required)")

	templatesCmd.AddCommand(templates_list)
	templatesCmd.AddCommand(templatesShowCmd)
	templatesCmd.AddCommand(templatesCreateCmd)
}
