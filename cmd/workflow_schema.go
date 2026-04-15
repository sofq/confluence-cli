package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

// WorkflowSchemaOps returns schema operations for the workflow subcommands.
func WorkflowSchemaOps() []generated.SchemaOp {
	return []generated.SchemaOp{
		{
			Resource: "workflow",
			Verb:     "move",
			Method:   "PUT",
			Path:     "/wiki/rest/api/content/{id}/move/append/{target-id}",
			Summary:  "Move a page to a different parent",
			HasBody:  false,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to move (required)", In: "path"},
				{Name: "target-id", Required: true, Type: "string", Description: "target parent page ID (required)", In: "path"},
			},
		},
		{
			Resource: "workflow",
			Verb:     "copy",
			Method:   "POST",
			Path:     "/wiki/rest/api/content/{id}/copy",
			Summary:  "Copy a page to a target parent",
			HasBody:  true,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to copy (required)", In: "path"},
				{Name: "target-id", Required: true, Type: "string", Description: "target parent page ID (required)", In: "custom"},
				{Name: "title", Required: false, Type: "string", Description: "title for the copied page", In: "custom"},
				{Name: "copy-attachments", Required: false, Type: "boolean", Description: "include attachments in copy", In: "custom"},
				{Name: "copy-labels", Required: false, Type: "boolean", Description: "include labels in copy", In: "custom"},
				{Name: "copy-permissions", Required: false, Type: "boolean", Description: "include permissions in copy", In: "custom"},
				{Name: "no-wait", Required: false, Type: "boolean", Description: "return immediately without polling", In: "custom"},
				{Name: "timeout", Required: false, Type: "string", Description: "timeout for async operation (e.g. 30s, 2m)", In: "custom"},
			},
		},
		{
			Resource: "workflow",
			Verb:     "publish",
			Method:   "PUT",
			Path:     "/pages/{id}",
			Summary:  "Publish a draft page",
			HasBody:  true,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to publish (required)", In: "path"},
			},
		},
		{
			Resource: "workflow",
			Verb:     "comment",
			Method:   "POST",
			Path:     "/pages/{id}/footer-comments",
			Summary:  "Add a plain-text comment to a page",
			HasBody:  true,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to comment on (required)", In: "path"},
				{Name: "body", Required: true, Type: "string", Description: "comment text (required)", In: "custom"},
			},
		},
		{
			Resource: "workflow",
			Verb:     "restrict",
			Method:   "GET",
			Path:     "/wiki/rest/api/content/{id}/restriction",
			Summary:  "View, add, or remove page restrictions",
			HasBody:  false,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to manage restrictions (required)", In: "path"},
				{Name: "add", Required: false, Type: "boolean", Description: "add a restriction", In: "custom"},
				{Name: "remove", Required: false, Type: "boolean", Description: "remove a restriction", In: "custom"},
				{Name: "operation", Required: false, Type: "string", Description: "restriction operation: read or update", In: "custom"},
				{Name: "user", Required: false, Type: "string", Description: "user account ID", In: "custom"},
				{Name: "group", Required: false, Type: "string", Description: "group name", In: "custom"},
			},
		},
		{
			Resource: "workflow",
			Verb:     "archive",
			Method:   "POST",
			Path:     "/wiki/rest/api/content/archive",
			Summary:  "Archive a page",
			HasBody:  true,
			Flags: []generated.SchemaFlag{
				{Name: "id", Required: true, Type: "string", Description: "page ID to archive (required)", In: "custom"},
				{Name: "no-wait", Required: false, Type: "boolean", Description: "return immediately without polling", In: "custom"},
				{Name: "timeout", Required: false, Type: "string", Description: "timeout for async operation (e.g. 30s, 2m)", In: "custom"},
			},
		},
	}
}
