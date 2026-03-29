package cmd

import (
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version as JSON",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// MarshalNoEscape on a map[string]string cannot fail.
		out, _ := jsonutil.MarshalNoEscape(map[string]string{"version": Version})
		return schemaOutput(cmd, out)
	},
}
