package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

// ExportSchemaOps returns schema operations for the export command.
func ExportSchemaOps() []generated.SchemaOp {
	return []generated.SchemaOp{
		{
			Resource: "export",
			Verb:     "export",
			Method:   "GET",
			Path:     "/pages/{id}",
			Summary:  "Export page body in requested format",
			HasBody:  false,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to export (required)", In: "custom"},
				{Name: "format", Required: false, Type: "string", Description: "body format: storage, atlas_doc_format, view", In: "custom"},
				{Name: "tree", Required: false, Type: "boolean", Description: "recursively export page tree as NDJSON", In: "custom"},
				{Name: "depth", Required: false, Type: "integer", Description: "maximum tree depth (0 = unlimited)", In: "custom"},
			},
		},
	}
}
