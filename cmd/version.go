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
		out, err := jsonutil.MarshalNoEscape(map[string]string{"version": Version})
		if err != nil {
			return err
		}
		return schemaOutput(cmd, out)
	},
}
