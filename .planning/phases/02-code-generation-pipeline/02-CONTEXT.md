# Phase 2: Code Generation Pipeline - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the `gen/` code generation pipeline that reads `spec/confluence-v2.json` (the pinned Confluence Cloud v2 OpenAPI spec) and produces `cmd/generated/*.go` — a complete, compilable Cobra command tree covering all API operations. The generator groups operations by resource tag, creates flags for all parameters, and supports `mergeCommand` override by hand-written wrappers.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

All implementation choices are at Claude's discretion — pure infrastructure phase. Mirror the `gen/` directory from the reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/` (main.go, parser.go, grouper.go, generator.go). Adapt for:
- Confluence v2 spec URL: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
- Module path: `github.com/sofq/confluence-cli`
- Replace `cmd/generated/stub.go` stubs with real `RegisterAll`, `AllSchemaOps`, `AllResources` implementations
- Pin the spec locally to `spec/confluence-v2.json`

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Reference `gen/` at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/` — complete code generator
- `cmd/generated/stub.go` — stub types (SchemaOp, SchemaFlag) already defined, ready to be replaced
- Phase 1 client and commands compile and test green

### Established Patterns
- `generated.RegisterAll(rootCmd)` already called in `cmd/root.go`
- `generated.AllSchemaOps()` already called in `cmd/schema_cmd.go`
- `libopenapi` for OpenAPI spec parsing (build-time only, not in CLI binary)

### Integration Points
- `cmd/root.go` init() calls `generated.RegisterAll(rootCmd)` — generated code must provide this
- `cmd/schema_cmd.go` calls `generated.AllSchemaOps()` — generated code must provide this
- Makefile already has `generate` target placeholder

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase. Mirror jr gen/ architecture exactly.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
