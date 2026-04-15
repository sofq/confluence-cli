package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

// DiffSchemaOps returns schema operations for the diff command.
func DiffSchemaOps() []generated.SchemaOp {
	return []generated.SchemaOp{
		{
			Resource: "diff",
			Verb:     "diff",
			Method:   "GET",
			Path:     "/pages/{id}/versions",
			Summary:  "Compare page versions and show structured diff",
			HasBody:  false,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to compare versions (required)", In: "path"},
				{Name: "since", Required: false, Type: "string", Description: "filter changes since duration (e.g. 2h, 1d) or ISO date (e.g. 2026-01-01)", In: "custom"},
				{Name: "from", Required: false, Type: "integer", Description: "start version number for explicit comparison", In: "custom"},
				{Name: "to", Required: false, Type: "integer", Description: "end version number for explicit comparison", In: "custom"},
			},
		},
	}
}
