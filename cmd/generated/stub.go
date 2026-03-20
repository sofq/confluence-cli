// Package generated contains auto-generated Cobra commands produced by the
// code generator in gen/. During Phase 1 this package contains only stubs.
// Phase 2 will replace this file with generated command registrations.
package generated

import "github.com/spf13/cobra"

// SchemaFlag describes a single flag/parameter for a schema operation.
type SchemaFlag struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Description string `json:"description"`
	In          string `json:"in"`
}

// SchemaOp describes a single API operation for the schema command.
type SchemaOp struct {
	Resource string       `json:"resource"`
	Verb     string       `json:"verb"`
	Method   string       `json:"method"`
	Path     string       `json:"path"`
	Summary  string       `json:"summary"`
	HasBody  bool         `json:"has_body"`
	Flags    []SchemaFlag `json:"flags"`
}

// RegisterAll registers generated commands on root. Phase 2 fills this in.
func RegisterAll(root *cobra.Command) {}

// AllSchemaOps returns all generated schema operations. Phase 2 fills this in.
func AllSchemaOps() []SchemaOp { return nil }

// AllResources returns all generated resource names. Phase 2 fills this in.
func AllResources() []string { return nil }
