# Phase 16: Schema + Gendocs - Research

**Researched:** 2026-03-28
**Domain:** Schema registration, documentation generation (Go, Cobra, text/template)
**Confidence:** HIGH

## Summary

Phase 16 adds hand-written schema registrations for all new commands (diff, workflow, export, preset, templates) and builds a standalone `gendocs` binary that generates VitePress-compatible Markdown and sidebar JSON from the Cobra command tree. Both patterns are directly ported from the jr (jira-cli-v2) reference implementation with minor adaptations for cf's module path, exit codes, and CLI name.

The schema registration pattern is mechanically straightforward: create individual `*_schema.go` files returning `[]generated.SchemaOp` slices, then update `schema_cmd.go` to aggregate them alongside `generated.AllSchemaOps()`. The gendocs binary is a self-contained `cmd/gendocs/main.go` that walks the Cobra command tree, enriches verb info from the schema lookup, and renders Markdown via `text/template`. Both patterns are proven in production in jr and require zero new dependencies.

**Primary recommendation:** Port jr's patterns directly. The schema files are pure data declarations (no logic to debug). The gendocs binary is a direct adaptation with cf-specific exit codes, module path, and `--output` flag instead of jr's positional argument.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Individual `*_schema.go` files per command group, each exporting a `func XxxSchemaOps() []generated.SchemaOp` function -- mirrors jr's pattern
- **D-02:** Schema files to create: `cmd/diff_schema.go` (1 op), `cmd/workflow_schema.go` (6 ops), `cmd/export_schema.go` (1 op), `cmd/preset_schema.go` (1 op), `cmd/templates_schema.go` (2 ops: show, create)
- **D-03:** Each SchemaOp includes: Resource, Verb, Method, Path, Summary, HasBody, Flags (matching command init() flags)
- **D-04:** `schema_cmd.go` updated to aggregate: call `generated.AllSchemaOps()` + append results from each `*SchemaOps()` function
- **D-05:** Standalone binary at `cmd/gendocs/main.go` -- not a Cobra subcommand. Invoked via `go run cmd/gendocs/main.go --output website/`
- **D-06:** Generates per-resource Markdown files by walking the Cobra command tree
- **D-07:** Generates `sidebar.json` -- VitePress sidebar configuration
- **D-08:** Generates error-codes table from `internal/errors` exit code constants
- **D-09:** `--output` flag specifies target directory (default: `website/commands/`). Creates directory if it doesn't exist
- **D-10:** Mirrors jr's `cmd/gendocs/main.go` structure: flagInfo, verbInfo, resourcePage, sidebarEntry types + Markdown templates

### Claude's Discretion
- Exact Markdown template formatting within gendocs (heading structure, flag table format)
- Whether to include hidden/deprecated commands in docs output
- Test approach for schema files (unit tests vs integration tests vs both)
- Whether sidebar.json groups by category or uses flat alphabetical list
- Error codes table format and placement

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SCHM-01 | All new commands (diff, workflow, export, preset) registered in `cf schema` output | Schema registration pattern from jr; exact flags extracted from each command's init() |
| SCHM-02 | Schema ops aggregated in `schema_cmd.go` for agent discoverability | jr's aggregation pattern in schema_cmd.go; also batch.go opMap needs updating |
| DOCS-05 | `gendocs` binary generates VitePress sidebar JSON and per-command docs from Cobra tree | jr's gendocs/main.go is the complete reference; adapt module paths, exit codes, output flag |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `text/template` | stdlib | Markdown template rendering in gendocs | Go stdlib, zero dependencies, same as jr |
| `encoding/json` | stdlib | sidebar.json generation, schema data marshaling | Go stdlib |
| `github.com/spf13/cobra` | v1.10.2 | Command tree walking in gendocs, flag extraction | Already in go.mod, same as jr |
| `github.com/spf13/pflag` | v1.0.9 | Flag metadata extraction (annotations, types) | Already in go.mod (indirect via cobra) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `path/filepath` | stdlib | Output directory path construction | File I/O in gendocs |
| `sort` | stdlib | Stable alphabetical page ordering | Page and sidebar sorting |
| `os` | stdlib | Directory creation, file writing | gendocs file output |

**No new dependencies required.** Everything uses Go stdlib + existing cobra/pflag.

## Architecture Patterns

### Schema Registration File Structure
```
cmd/
  diff_schema.go          # DiffSchemaOps() -> 1 op
  workflow_schema.go      # WorkflowSchemaOps() -> 6 ops
  export_schema.go        # ExportSchemaOps() -> 1 op
  preset_schema.go        # PresetSchemaOps() -> 1 op
  templates_schema.go     # TemplatesSchemaOps() -> 2 ops
  schema_cmd.go           # Updated: aggregates generated + hand-written ops
cmd/gendocs/
  main.go                 # Standalone binary: flagInfo, verbInfo, resourcePage, templates
```

### Pattern 1: Hand-Written Schema Registration
**What:** Each `*_schema.go` file exports a function returning `[]generated.SchemaOp` with operation metadata matching the command's actual flags.
**When to use:** For every hand-written command that needs to appear in `cf schema` and `cf batch`.

```go
// Source: jr cmd/diff_schema.go (adapted for cf)
package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

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
                {Name: "id", Required: true, Type: "string", Description: "page ID to compare versions", In: "custom"},
                {Name: "since", Required: false, Type: "string", Description: "filter changes since duration (e.g. 2h, 1d) or ISO date", In: "custom"},
                {Name: "from", Required: false, Type: "integer", Description: "start version number for explicit comparison", In: "custom"},
                {Name: "to", Required: false, Type: "integer", Description: "end version number for explicit comparison", In: "custom"},
            },
        },
    }
}
```

### Pattern 2: Schema Aggregation in schema_cmd.go
**What:** Update the `allOps` construction to include hand-written ops alongside generated ones.
**When to use:** Once in `schema_cmd.go` -- the single aggregation point.

```go
// Source: jr cmd/schema_cmd.go lines 29-34 (adapted for cf)
allOps := generated.AllSchemaOps()
allOps = append(allOps, DiffSchemaOps()...)
allOps = append(allOps, WorkflowSchemaOps()...)
allOps = append(allOps, ExportSchemaOps()...)
allOps = append(allOps, PresetSchemaOps()...)
allOps = append(allOps, TemplatesSchemaOps()...)
```

### Pattern 3: Gendocs Schema Lookup
**What:** Build a `map[schemaKey]generated.SchemaOp` from all ops for enriching Cobra command info with HTTP method/path.
**When to use:** In gendocs `buildSchemaLookup()`.

```go
// Source: jr cmd/gendocs/main.go lines 70-84 (adapted for cf)
func buildSchemaLookup() map[schemaKey]generated.SchemaOp {
    m := make(map[schemaKey]generated.SchemaOp)
    all := append(generated.AllSchemaOps(), DiffSchemaOps()...)
    all = append(all, WorkflowSchemaOps()...)
    all = append(all, ExportSchemaOps()...)
    all = append(all, PresetSchemaOps()...)
    all = append(all, TemplatesSchemaOps()...)
    for _, op := range all {
        m[schemaKey{op.Resource, op.Verb}] = op
    }
    return m
}
```

### Pattern 4: Gendocs --output Flag
**What:** cf's gendocs uses a `--output` flag (D-09) rather than jr's positional argument.
**When to use:** In gendocs main().

```go
// Adaptation from jr: positional arg -> flag-based
func main() {
    outDir := flag.String("output", "website/commands/", "output directory for generated docs")
    flag.Parse()
    // ...
    if err := run(*outDir); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

### Anti-Patterns to Avoid
- **Duplicating flag definitions:** Schema flags MUST match the command's actual init() flags exactly. Do not copy-paste and diverge -- if a flag name, type, or description changes in the command, the schema must be updated in lockstep.
- **Including watch in schema scope:** The watch command is NOT listed in D-02. Only the five specified command groups need schema files. Watch can be added later if needed.
- **Hardcoding ExitTimeout:** CONTEXT.md mentions ExitTimeout but it does not exist in cf's `internal/errors/errors.go`. The actual constants are ExitOK(0), ExitError(1), ExitAuth(2), ExitNotFound(3), ExitValidation(4), ExitRateLimit(5), ExitConflict(6), ExitServer(7). Use only these.
- **Forgetting batch.go:** The `batch.go` opMap currently only uses `generated.AllSchemaOps()`. When schema_cmd.go is updated, batch.go MUST also be updated to include hand-written ops, or batch operations for new commands will fail silently.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cobra flag extraction | Custom flag parser | `pflag.Flag` + `cobra.BashCompOneRequiredFlag` annotation | Cobra already tracks required flags via annotations; jr's extractFlags() uses this |
| Markdown rendering | String concatenation | `text/template` with FuncMap | Handles escaping, conditionals, loops cleanly; proven in jr |
| JSON sidebar | Manual string building | `json.MarshalIndent` | Correct escaping, formatting guaranteed |
| Directory creation | Manual checks | `os.MkdirAll` in writeFile helper | Handles nested paths, idempotent |

**Key insight:** The entire gendocs binary is a template-rendering pipeline. Every piece of jr's gendocs is directly portable -- the data model (flagInfo, verbInfo, resourcePage, sidebarEntry, exitCodeRow), the Cobra walking logic, the template rendering, and the file I/O helpers.

## Common Pitfalls

### Pitfall 1: Schema Flag Type Mismatch
**What goes wrong:** Schema declares a flag as "string" but the command uses `Int` or `Bool`, causing agents to send wrong types.
**Why it happens:** Copy-paste from jr schemas without adapting to cf's actual flag types.
**How to avoid:** Cross-reference every schema flag against the command's init() block. For cf: diff's `--from` and `--to` are `Int` (type "integer"), export's `--tree` is `Bool` (type "boolean"), export's `--depth` is `Int` (type "integer").
**Warning signs:** `cf schema diff diff` shows "string" for `--from`/`--to` instead of "integer".

### Pitfall 2: Missing batch.go Update
**What goes wrong:** `cf batch` cannot resolve new commands (diff, workflow move, etc.) because `batch.go` only queries `generated.AllSchemaOps()`.
**Why it happens:** The aggregation pattern is updated in `schema_cmd.go` but `batch.go` is forgotten.
**How to avoid:** Update `batch.go` line 152 to match the same aggregation pattern as `schema_cmd.go`. Both files must include the same hand-written ops.
**Warning signs:** `cf batch '[{"command":"diff diff","args":{"id":"123"}}]'` returns "unknown command" error.

### Pitfall 3: Resource/Verb Key Mismatch
**What goes wrong:** Schema op's Resource/Verb don't match how batch.go or schema_cmd.go look them up, causing silent failures.
**Why it happens:** Using inconsistent naming (e.g., "workflow_move" vs "workflow move" vs "move").
**How to avoid:** Follow jr's convention exactly: Resource is the parent command name (e.g., "workflow"), Verb is the subcommand name (e.g., "move"). For leaf commands (diff, export), Resource and Verb use the command name.
**Warning signs:** `cf schema workflow move` returns "operation not found".

### Pitfall 4: Gendocs Module Import Path
**What goes wrong:** `cmd/gendocs/main.go` fails to compile because it imports jr's module path instead of cf's.
**Why it happens:** Direct copy-paste from jr without updating import paths.
**How to avoid:** All imports must use `github.com/sofq/confluence-cli/cmd`, `github.com/sofq/confluence-cli/cmd/generated`, `github.com/sofq/confluence-cli/internal/errors`.
**Warning signs:** `go run cmd/gendocs/main.go` compilation error.

### Pitfall 5: Stale Generated Pages
**What goes wrong:** Old command pages linger after a resource is removed or renamed.
**Why it happens:** Gendocs appends new pages but doesn't clean old ones.
**How to avoid:** Port jr's stale-page cleanup logic (gendocs lines 384-398): before writing, scan the commands directory and remove `.md` files not in the current page set.
**Warning signs:** `website/commands/` contains orphaned `.md` files after regeneration.

### Pitfall 6: ExitTimeout Does Not Exist
**What goes wrong:** Gendocs error-codes table references a non-existent `ExitTimeout` constant.
**Why it happens:** CONTEXT.md D-08 mentions ExitTimeout, but cf's errors.go only has 8 constants (ExitOK through ExitServer). There is no ExitTimeout -- timeouts are reported as ExitError.
**How to avoid:** Build the error codes table from the actual constants in `internal/errors/errors.go`: ExitOK(0), ExitError(1), ExitAuth(2), ExitNotFound(3), ExitValidation(4), ExitRateLimit(5), ExitConflict(6), ExitServer(7).
**Warning signs:** Compilation failure referencing `cferrors.ExitTimeout`.

### Pitfall 7: Workflow Subcommand Has No Method/Path for Some Ops
**What goes wrong:** Some workflow subcommands use v1 API endpoints while the schema Path field expects v2-style paths.
**Why it happens:** cf's workflow commands hit different API versions (v1 for move/copy/restrict/archive, v2 for publish/comment).
**How to avoid:** Use the actual endpoint paths: move uses `/wiki/rest/api/content/{id}/move/append/{targetId}` (v1), publish uses `/pages/{id}` (v2), etc. Schema consumers (agents) use Path for documentation only, not invocation.
**Warning signs:** Path field shows incorrect or empty values for workflow operations.

## Code Examples

### Complete Workflow Schema (6 ops)
```go
// Source: derived from cmd/workflow.go init() flags
package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

func WorkflowSchemaOps() []generated.SchemaOp {
    return []generated.SchemaOp{
        {
            Resource: "workflow",
            Verb:     "move",
            Method:   "PUT",
            Path:     "/wiki/rest/api/content/{id}/move/append/{targetId}",
            Summary:  "Move a page to a different parent",
            HasBody:  false,
            Flags: []generated.SchemaFlag{
                {Name: "id", Required: true, Type: "string", Description: "page ID to move", In: "custom"},
                {Name: "target-id", Required: true, Type: "string", Description: "target parent page ID", In: "custom"},
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
                {Name: "id", Required: true, Type: "string", Description: "page ID to copy", In: "custom"},
                {Name: "target-id", Required: true, Type: "string", Description: "target parent page ID", In: "custom"},
                {Name: "title", Required: false, Type: "string", Description: "title for the copied page", In: "custom"},
                {Name: "copy-attachments", Required: false, Type: "boolean", Description: "include attachments in copy", In: "custom"},
                {Name: "copy-labels", Required: false, Type: "boolean", Description: "include labels in copy", In: "custom"},
                {Name: "copy-permissions", Required: false, Type: "boolean", Description: "include permissions in copy", In: "custom"},
                {Name: "no-wait", Required: false, Type: "boolean", Description: "return immediately without polling", In: "custom"},
                {Name: "timeout", Required: false, Type: "string", Description: "timeout for async operation (e.g. 30s, 2m)", In: "custom"},
            },
        },
        // ... publish, comment, restrict, archive follow same pattern
    }
}
```

### Schema Aggregation Update for schema_cmd.go
```go
// Source: jr cmd/schema_cmd.go lines 29-34
// Current cf code (line 30):
//   allOps := generated.AllSchemaOps()
// Updated to:
allOps := generated.AllSchemaOps()
allOps = append(allOps, DiffSchemaOps()...)
allOps = append(allOps, WorkflowSchemaOps()...)
allOps = append(allOps, ExportSchemaOps()...)
allOps = append(allOps, PresetSchemaOps()...)
allOps = append(allOps, TemplatesSchemaOps()...)
```

### Batch.go Update (Critical)
```go
// Source: cmd/batch.go line 151-157
// Current:
//   allOps := generated.AllSchemaOps()
// Must become:
allOps := generated.AllSchemaOps()
allOps = append(allOps, DiffSchemaOps()...)
allOps = append(allOps, WorkflowSchemaOps()...)
allOps = append(allOps, ExportSchemaOps()...)
allOps = append(allOps, PresetSchemaOps()...)
allOps = append(allOps, TemplatesSchemaOps()...)
opMap := make(map[string]generated.SchemaOp, len(allOps))
```

### Gendocs Error Codes Table (cf-specific)
```go
// Source: adapted from jr gendocs, using cf's actual exit code constants
var exitCodeNames = map[int]string{
    cferrors.ExitOK:         "OK",
    cferrors.ExitError:      "Error",
    cferrors.ExitAuth:       "Auth",
    cferrors.ExitNotFound:   "NotFound",
    cferrors.ExitValidation: "Validation",
    cferrors.ExitRateLimit:  "RateLimit",
    cferrors.ExitConflict:   "Conflict",
    cferrors.ExitServer:     "Server",
}
```

### Gendocs Command Tree Walking
```go
// Source: jr cmd/gendocs/main.go lines 129-173
// Directly portable -- walks root.Commands(), filters hidden/help/completion,
// groups leaf commands as SingleVerb pages, subcommands as multi-verb pages.
func walkCommands(root *cobra.Command, schema map[schemaKey]generated.SchemaOp) []resourcePage {
    // Same logic as jr, no changes needed except CLI name in templates
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| All schema ops in generated code | Generated + hand-written ops aggregated | jr v1.0 | Hand-written commands are discoverable via schema and batch |
| No docs generation | Standalone gendocs binary from Cobra tree | jr v1.0 | Automated, always-current VitePress command reference |
| Positional args for gendocs | `--output` flag (D-09 decision) | cf Phase 16 | More explicit invocation: `go run cmd/gendocs/main.go --output website/` |

**Notable cf vs jr differences:**
- cf has no `ExitTimeout` constant (jr added `ExitRateLimit` in its exit codes; cf has it but not timeout)
- cf's workflow commands are content-lifecycle ops (move/copy/publish/comment/restrict/archive) vs jr's issue-lifecycle ops (transition/assign/comment/move/create/link/log-work/sprint)
- cf uses `--output` flag, jr uses positional argument
- cf has no `pretty` package dependency (uses `json.Indent` from stdlib instead of `tidwall/pretty`)
- cf's template commands use "templates" (plural) as the resource name, not "template" (singular like jr)

## Open Questions

1. **Should watch also get a schema file?**
   - What we know: watch command exists in cf but is NOT listed in D-02 schema files to create
   - What's unclear: Whether this was intentional exclusion or oversight
   - Recommendation: Follow D-02 strictly -- do not create watch_schema.go in this phase. Can be added in a future phase if needed.

2. **Gendocs output directory structure**
   - What we know: D-09 says `--output` default is `website/commands/`. jr writes to `{outdir}/commands/`, `{outdir}/guide/`, `{outdir}/.vitepress/`
   - What's unclear: Whether cf should use the same subdirectory structure
   - Recommendation: Mirror jr's structure: `{outdir}/commands/` for per-resource pages + index, `{outdir}/guide/` for error-codes, `{outdir}/.vitepress/` for sidebar-commands.json. The `--output` flag points to the root website directory, not the commands subdirectory.

3. **Templates resource name: "templates" vs "template"**
   - What we know: cf uses "templates" (plural) as the Cobra command name, jr uses "template" (singular)
   - What's unclear: N/A -- this is clear
   - Recommendation: Schema Resource field should be "templates" (matching cf's actual command name) to ensure `cf schema templates` works correctly.

## Sources

### Primary (HIGH confidence)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/workflow_schema.go` -- Hand-written schema ops pattern (8 ops across 2 functions)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/diff_schema.go` -- Diff schema ops pattern (1 op)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/template_schema.go` -- Template schema ops pattern (4 ops)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/schema_cmd.go` -- Schema aggregation pattern (lines 29-34)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/gendocs/main.go` -- Complete gendocs reference (480 lines)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/schema_cmd.go` -- Current cf schema command (no hand-written ops yet)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/batch.go` -- Batch opMap construction (line 151-157, needs update)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/generated/schema_data.go` -- SchemaOp, SchemaFlag types, AllSchemaOps(), AllResources()
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/internal/errors/errors.go` -- Exit code constants (lines 18-27)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/diff.go` -- Diff command flags (lines 307-311)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/workflow.go` -- Workflow command flags (lines 557-598)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/export.go` -- Export command flags (lines 214-219)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/preset.go` -- Preset command (lines 73-75)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/templates.go` -- Templates command flags (lines 243-250)
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/cmd/root.go` -- RootCommand() export, command registration

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new dependencies, all stdlib + existing cobra/pflag
- Architecture: HIGH -- direct port from proven jr patterns with trivial adaptations
- Pitfalls: HIGH -- all discovered from comparing jr reference with cf codebase differences (exit codes, batch.go, module paths)

**Research date:** 2026-03-28
**Valid until:** Indefinite (patterns are stable, no external API dependencies)
