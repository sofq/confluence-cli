# Phase 16: Schema + Gendocs - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Register all hand-written commands (diff, workflow, export, preset, templates) in the schema system for agent discoverability and batch operations. Create a standalone `gendocs` binary that generates per-command VitePress Markdown and sidebar JSON from the Cobra command tree. Mirrors jr's schema registration and gendocs patterns exactly.

</domain>

<decisions>
## Implementation Decisions

### Schema registration pattern (SCHM-01, SCHM-02)
- **D-01:** Individual `*_schema.go` files per command group, each exporting a `func XxxSchemaOps() []generated.SchemaOp` function — mirrors jr's `workflow_schema.go`, `diff_schema.go`, `template_schema.go` pattern
- **D-02:** Schema files to create:
  - `cmd/diff_schema.go` — 1 op: diff (verb "diff", resource "diff")
  - `cmd/workflow_schema.go` — 6 ops: move, copy, publish, comment, restrict, archive (resource "workflow")
  - `cmd/export_schema.go` — 1 op: export (verb "export", resource "export")
  - `cmd/preset_schema.go` — 1 op: list (verb "list", resource "preset")
  - `cmd/templates_schema.go` — 2 ops: show, create (resource "templates")
- **D-03:** Each SchemaOp includes: Resource, Verb, Method, Path, Summary, HasBody, Flags (matching the flags defined in each command's init())
- **D-04:** `schema_cmd.go` updated to aggregate: call `generated.AllSchemaOps()` + append results from each `*SchemaOps()` function. Single `allOps` variable used throughout

### Gendocs binary (DOCS-05)
- **D-05:** Standalone binary at `cmd/gendocs/main.go` — not a Cobra subcommand. Invoked via `go run cmd/gendocs/main.go --output website/`
- **D-06:** Generates per-resource Markdown files by walking the Cobra command tree. Each file contains: resource name, description, list of verbs with flags, examples, and API path
- **D-07:** Generates `sidebar.json` — VitePress sidebar configuration with text/link entries for each resource page, sorted alphabetically
- **D-08:** Generates error-codes table from `internal/errors` exit code constants (ExitOK, ExitError, ExitNotFound, ExitValidation, ExitAuth, ExitTimeout)
- **D-09:** `--output` flag specifies target directory (default: `website/commands/`). Creates directory if it doesn't exist
- **D-10:** Mirrors jr's `cmd/gendocs/main.go` structure: flagInfo, verbInfo, resourcePage, sidebarEntry types + Markdown templates

### Claude's Discretion
- Exact Markdown template formatting within gendocs (heading structure, flag table format)
- Whether to include hidden/deprecated commands in docs output
- Test approach for schema files (unit tests vs integration tests vs both)
- Whether sidebar.json groups by category or uses flat alphabetical list
- Error codes table format and placement

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### jr reference implementation (architecture mirror)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/workflow_schema.go` — Hand-written schema ops pattern: `HandWrittenSchemaOps()` returning `[]generated.SchemaOp` slice
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/diff_schema.go` — Diff schema ops pattern
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/template_schema.go` — Template schema ops pattern
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/schema_cmd.go` — Schema aggregation pattern (merging generated + hand-written ops)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/gendocs/main.go` — Gendocs binary: flagInfo, verbInfo, resourcePage, sidebarEntry types, Markdown templates, Cobra tree walking

### Existing cf schema system
- `cmd/schema_cmd.go` — Current schema command: `AllSchemaOps()`, `compactSchema()`, `schemaOutput()`. Needs update to aggregate hand-written ops
- `cmd/generated/schema_data.go` — Generated schema data: `SchemaOp` type definition, `AllSchemaOps()`, `AllResources()`, `SchemaFlag` type
- `cmd/schema_cmd_test.go` — Existing schema tests (update for new ops)

### Hand-written commands needing schema registration
- `cmd/diff.go` — diff command flags and structure
- `cmd/workflow.go` — 6 workflow subcommands with their flags
- `cmd/export.go` — export command flags
- `cmd/preset.go` — preset list command
- `cmd/templates.go` — templates show/create commands

### Internal packages referenced by gendocs
- `internal/errors/errors.go` — Exit code constants for error codes table

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/generated/schema_data.go`: `SchemaOp` and `SchemaFlag` types — reuse for hand-written schema ops
- `cmd/schema_cmd.go`: `schemaOutput()` helper — already handles --jq and --pretty for schema JSON
- `cmd/batch.go`: Already uses `AllSchemaOps()` for op resolution — will automatically pick up hand-written ops once aggregated
- jr's `cmd/gendocs/main.go`: Complete reference implementation to port

### Established Patterns
- Generated ops in `cmd/generated/schema_data.go` via `AllSchemaOps()`
- jr uses individual `*_schema.go` files with separate functions, aggregated in `schema_cmd.go`
- Schema ops include: Resource, Verb, Method, Path, Summary, HasBody, Flags[] (Name, Required, Type, Description, In)
- `batch.go` builds opMap from schema ops — hand-written ops need correct Resource/Verb keys

### Integration Points
- `cmd/schema_cmd.go`: Modify `allOps` construction to include hand-written ops
- `cmd/batch.go`: Will automatically use new ops via `AllSchemaOps()` aggregation
- `cmd/root.go`: gendocs binary accesses `RootCommand()` for Cobra tree walking

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following jr patterns adapted for cf.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 16-schema-gendocs*
*Context gathered: 2026-03-28*
