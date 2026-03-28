package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

// PresetSchemaOps returns schema operations for the preset command.
func PresetSchemaOps() []generated.SchemaOp {
	return []generated.SchemaOp{
		{
			Resource: "preset",
			Verb:     "list",
			Method:   "GET",
			Path:     "",
			Summary:  "List all available output presets",
			HasBody:  false,
			Flags:    []generated.SchemaFlag{},
		},
	}
}
