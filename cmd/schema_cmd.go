package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sofq/confluence-cli/cmd/generated"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jq"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema [resource] [verb]",
	Short: "Discover available commands and flags (JSON output for agents)",
	Long: `Machine-readable command/flag discovery.

  cf schema --list          # list all resource names
  cf schema issue           # list operations for a resource
  cf schema issue get       # full schema for one operation`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		listFlag, _ := cmd.Flags().GetBool("list")
		compactFlag, _ := cmd.Flags().GetBool("compact")

		allOps := generated.AllSchemaOps()
		allOps = append(allOps, DiffSchemaOps()...)
		allOps = append(allOps, WorkflowSchemaOps()...)
		allOps = append(allOps, ExportSchemaOps()...)
		allOps = append(allOps, PresetSchemaOps()...)

		if compactFlag || (len(args) == 0 && !listFlag) {
			data, _ := jsonutil.MarshalNoEscape(compactSchema(allOps))
			return schemaOutput(cmd, data)
		}

		if listFlag {
			resources := generated.AllResources()
			seen := make(map[string]bool, len(resources))
			for _, r := range resources {
				seen[r] = true
			}
			for _, op := range allOps {
				if !seen[op.Resource] {
					resources = append(resources, op.Resource)
					seen[op.Resource] = true
				}
			}
			data, _ := jsonutil.MarshalNoEscape(resources)
			return schemaOutput(cmd, data)
		}

		resource := args[0]

		if len(args) == 1 {
			var matching []generated.SchemaOp
			for _, op := range allOps {
				if op.Resource == resource {
					matching = append(matching, op)
				}
			}
			if len(matching) == 0 {
				apiErr := &cferrors.APIError{
					ErrorType: "not_found",
					Message:   fmt.Sprintf("resource %q not found", resource),
				}
				apiErr.WriteJSON(os.Stderr)
				return &cferrors.AlreadyWrittenError{Code: cferrors.ExitNotFound}
			}
			data, _ := jsonutil.MarshalNoEscape(matching)
			return schemaOutput(cmd, data)
		}

		verb := args[1]
		for _, op := range allOps {
			if op.Resource == resource && op.Verb == verb {
				data, _ := jsonutil.MarshalNoEscape(op)
				return schemaOutput(cmd, data)
			}
		}
		apiErr := &cferrors.APIError{
			ErrorType: "not_found",
			Message:   fmt.Sprintf("operation %s %s not found", resource, verb),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitNotFound}
	},
}

// compactSchema builds a resource→verbs map from a list of schema ops.
func compactSchema(ops []generated.SchemaOp) map[string][]string {
	compact := make(map[string][]string)
	for _, op := range ops {
		compact[op.Resource] = append(compact[op.Resource], op.Verb)
	}
	return compact
}

// schemaOutput applies --jq and --pretty flags to schema JSON output.
func schemaOutput(cmd *cobra.Command, data []byte) error {
	jqFilter, _ := cmd.Flags().GetString("jq")
	prettyFlag, _ := cmd.Flags().GetBool("pretty")

	if jqFilter != "" {
		filtered, err := jq.Apply(data, jqFilter)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "jq_error",
				Message:   "jq: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		data = filtered
	}

	if prettyFlag {
		var out bytes.Buffer
		if err := json.Indent(&out, data, "", "  "); err == nil {
			data = out.Bytes()
		}
	}

	fmt.Fprintf(os.Stdout, "%s\n", strings.TrimRight(string(data), "\n"))
	return nil
}

func init() {
	schemaCmd.Flags().Bool("list", false, "list all resource names")
	schemaCmd.Flags().Bool("compact", false, "compact output: resource → verbs mapping only")
	rootCmd.AddCommand(schemaCmd)
}
