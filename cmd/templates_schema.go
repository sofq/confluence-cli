package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

// TemplatesSchemaOps returns schema operations for the templates subcommands.
func TemplatesSchemaOps() []generated.SchemaOp {
	return []generated.SchemaOp{
		{
			Resource: "templates",
			Verb:     "show",
			Method:   "GET",
			Path:     "",
			Summary:  "Show a template's full definition including variables",
			HasBody:  false,
			Flags: []generated.SchemaFlag{
				{Name: "name", Required: true, Type: "string", Description: "template name (positional argument)", In: "custom"},
			},
		},
		{
			Resource: "templates",
			Verb:     "create",
			Method:   "POST",
			Path:     "/pages/{id}",
			Summary:  "Create a template from an existing page",
			HasBody:  false,
			Flags: []generated.SchemaFlag{
				{Name: "from-page", Required: true, Type: "string", Description: "page ID to create template from (required)", In: "custom"},
				{Name: "name", Required: true, Type: "string", Description: "template name (required)", In: "custom"},
			},
		},
	}
}
