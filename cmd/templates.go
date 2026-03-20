package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cferrors "github.com/sofq/confluence-cli/internal/errors"
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
		return fmt.Errorf("missing subcommand for %q; available: list", cmd.CommandPath())
	},
}

// templates_list lists available templates from the templates directory.
var templates_list = &cobra.Command{
	Use:   "list",
	Short: "List available content templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := cftemplate.List()
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "config_error",
				Message:   "failed to list templates: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}
		data, _ := json.Marshal(names)
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
}

// resolveTemplate loads a template by name, parses --var flags into a map,
// renders the template, and returns the rendered result. It writes JSON errors
// to stderr and returns an AlreadyWrittenError on failure.
func resolveTemplate(templateName string, varFlags []string) (*cftemplate.RenderedTemplate, error) {
	varMap := make(map[string]string)
	for _, v := range varFlags {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   fmt.Sprintf("invalid --var format %q: expected key=value", v),
			}
			apiErr.WriteJSON(os.Stderr)
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
		apiErr.WriteJSON(os.Stderr)
		return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	rendered, err := cftemplate.Render(tmpl, varMap)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   "template render failed: " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return nil, &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	return rendered, nil
}

func init() {
	templatesCmd.AddCommand(templates_list)
}
