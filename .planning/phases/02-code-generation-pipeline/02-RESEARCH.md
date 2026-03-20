# Phase 2: Code Generation Pipeline - Research

**Researched:** 2026-03-20
**Domain:** Go code generation from OpenAPI spec using libopenapi + text/template
**Confidence:** HIGH

## Summary

This phase ports the `gen/` code generator from the Jira CLI reference implementation (`/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/`) to the Confluence CLI. The reference is mature, tested, and directly applicable — the primary adaptation work is (1) path-based resource extraction for Confluence's flat URL structure (vs Jira's `/rest/api/3/<resource>` prefix), (2) adding libopenapi to the confluence-cli `go.mod`, and (3) replacing `cmd/generated/stub.go` with real generated code.

The Confluence v2 OpenAPI spec at `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` parses cleanly with libopenapi v0.34.3 (confirmed: `errs == nil`, 146 paths, 212 operations, 24 resource groups). The generator pipeline is: ParseSpec -> GroupOperations -> GenerateResource (24 files) + GenerateSchemaData + GenerateInit -> `cmd/generated/`.

**Primary recommendation:** Mirror gen/ from jira-cli-v2 exactly, with one critical adaptation: `ExtractResource` for Confluence paths uses the first path segment directly (paths start with `/{resource}/...`) rather than the Jira pattern (`/rest/api/3/{resource}/...`).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
(None — all implementation choices are at Claude's discretion for this infrastructure phase.)

### Claude's Discretion

All implementation choices are at Claude's discretion — pure infrastructure phase. Mirror the `gen/` directory from the reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/` (main.go, parser.go, grouper.go, generator.go). Adapt for:
- Confluence v2 spec URL: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
- Module path: `github.com/sofq/confluence-cli`
- Replace `cmd/generated/stub.go` stubs with real `RegisterAll`, `AllSchemaOps`, `AllResources` implementations
- Pin the spec locally to `spec/confluence-v2.json`

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CGEN-01 | CLI auto-generates Cobra commands from Confluence v2 OpenAPI spec | libopenapi v0.34.3 parses spec cleanly; `go run ./gen/...` invokes generator; templates produce valid Cobra commands |
| CGEN-02 | Generator groups operations by resource (pages, spaces, search, etc.) | Path first-segment extraction produces 24 resource groups covering all 212 ops; `GroupOperations` pattern from reference applies directly |
| CGEN-03 | Generated commands include all path/query/body parameters from spec | libopenapi Parameter model exposes Name/In/Required/Schema; `ParseSpec` merges path-level and operation-level params; array-type params need string flag workaround |
| CGEN-04 | Hand-written workflow commands can override generated commands via `mergeCommand` | `mergeCommand` already implemented in `cmd/root.go`; generator's `RegisterAll` registers generated commands first; hand-written commands replace via `mergeCommand` |
| CGEN-05 | Spec is pinned locally at `spec/confluence-v2.json` with known gaps documented | Spec downloaded to `spec/confluence-v2.json` (596KB); gaps identified: no attachment upload in v2, 1 deprecated op, 18 EAP/experimental ops, array-type query params |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pb33f/libopenapi` | v0.34.3 | Parse OpenAPI 3.0 spec at build time | Used in reference implementation; produces typed Go model; v0.34.3 confirmed working against Confluence spec |
| `text/template` | stdlib | Render Go source code from templates | No external dep; gofmt post-processing catches template errors |
| `go/format` | stdlib | Format generated Go source (`gofmt`) | Makes output canonical; catches template bugs (invalid Go) |

### Supporting (already in go.mod or implicit)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/spf13/cobra` | v1.10.2 | Generated command wiring | Already in go.mod; generated code imports it |
| `github.com/sofq/confluence-cli/internal/client` | local | `client.FromContext`, `client.QueryFromFlags` | Generated command RunE bodies call these |
| `github.com/sofq/confluence-cli/internal/errors` | local | `APIError`, `AlreadyWrittenError`, exit codes | Generated validation error paths |

### Installation

```bash
# From confluence-cli root — add libopenapi to main module
go get github.com/pb33f/libopenapi@v0.34.3
go mod tidy
```

libopenapi indirect dependencies pulled in automatically:
- `github.com/bahlo/generic-list-go`
- `github.com/buger/jsonparser`
- `github.com/pb33f/jsonpath`
- `github.com/pb33f/ordered-map/v2`
- `go.yaml.in/yaml/v4`

## Architecture Patterns

### Project Structure After Phase 2

```
confluence-cli/
├── spec/
│   └── confluence-v2.json         # pinned spec (596KB, downloaded once)
├── gen/
│   ├── main.go                    # run(specPath, outDir), main()
│   ├── parser.go                  # ParseSpec, Operation, Param types
│   ├── grouper.go                 # GroupOperations, ExtractResource, DeriveVerb
│   ├── generator.go               # GenerateResource, GenerateSchemaData, GenerateInit
│   ├── templates/
│   │   ├── resource.go.tmpl       # per-resource Cobra command file
│   │   ├── schema_data.go.tmpl    # AllSchemaOps(), AllResources(), type defs
│   │   └── init.go.tmpl           # RegisterAll()
│   ├── main_test.go               # TestRun, TestMainSuccess, TestMainError
│   ├── parser_test.go             # TestParseSpec_*, TestSchemaType*
│   ├── grouper_test.go            # TestGroupOperations, TestDeriveVerb, TestExtractResource
│   ├── generator_test.go          # TestBuildPathTemplate*, TestLoadTemplate*
│   └── conformance_test.go        # Golden-file: generated output matches spec
├── cmd/generated/
│   ├── init.go                    # GENERATED: RegisterAll(root *cobra.Command)
│   ├── schema_data.go             # GENERATED: types + AllSchemaOps() + AllResources()
│   ├── pages.go                   # GENERATED: 29 ops
│   ├── blogposts.go               # GENERATED: 24 ops
│   ├── spaces.go                  # GENERATED: 20 ops
│   ├── ... (21 more .go files)
│   └── stub.go                    # DELETED: replaced by schema_data.go
└── Makefile                       # generate: go run ./gen/...
```

### Pattern 1: Pipeline Architecture (Parse -> Group -> Generate)

**What:** Three-stage pipeline where each stage is independently testable.
**When to use:** Always — this is the only generation pattern.

```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/main.go
func run(specPath, outDir string) error {
    ops, err := ParseSpec(specPath)         // stage 1: OpenAPI -> []Operation
    groups := GroupOperations(ops)          // stage 2: []Operation -> map[resource][]Operation
    for _, resource := range resources {
        GenerateResource(resource, groups[resource], outDir)  // stage 3a: one .go per resource
    }
    GenerateSchemaData(groups, resources, outDir)  // stage 3b: schema_data.go
    GenerateInit(resources, outDir)               // stage 3c: init.go
}
```

### Pattern 2: ExtractResource for Confluence Paths

**What:** Confluence v2 paths start at `/{resource}/...` with no version prefix. The Jira reference uses `/rest/api/3/{resource}` extraction — this MUST be replaced.

**Confluence path structure (confirmed):**
```
/pages                           -> "pages"
/pages/{id}                      -> "pages"
/pages/{id}/footer-comments      -> "pages"
/spaces/{id}/role-assignments    -> "spaces"
/admin-key                       -> "admin-key"
/custom-content/{id}/attachments -> "custom-content"
```

**Adapted ExtractResource for Confluence:**
```go
// Confluence paths: /{resource}/... — use first segment directly.
func ExtractResource(path string) string {
    segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
    for _, s := range segments {
        if s != "" && !strings.HasPrefix(s, "{") {
            return s
        }
    }
    return path
}
```

This produces 24 resource groups from 146 paths. All hyphenated resources (`admin-key`, `custom-content`, etc.) are safely handled by the existing `toGoIdentifier` function (hyphens -> underscores).

### Pattern 3: Template Rendering with gofmt

**What:** `renderTemplate` executes `text/template` then calls `go/format.Source` on the result. If gofmt fails, the unformatted source is returned alongside the error — enabling debugging of template bugs.

**When to use:** Always wrap template execution with format.Source. This catches missing imports, syntax errors in template logic.

```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/generator.go
formatted, err := format.Source(buf.Bytes())
if err != nil {
    return buf.Bytes(), fmt.Errorf("formatting generated code for %q: %w", name, err)
}
```

### Pattern 4: mergeCommand Override Mechanism (ALREADY IMPLEMENTED)

**What:** `cmd/root.go` already has `mergeCommand` which allows hand-written commands to replace generated ones while inheriting generated subcommands.

**Registration order:**
```go
// cmd/root.go init()
generated.RegisterAll(rootCmd)         // 1. register all generated commands
mergeCommand(rootCmd, handWrittenCmd)  // 2. replace specific ones with hand-written
```

**How it works:** `mergeCommand` finds the generated command by name, copies its subcommands onto the hand-written command (skipping duplicates), removes the generated command, and adds the hand-written one.

### Pattern 5: Template Import Management

**What:** Generated resource files import packages that may not all be used (e.g., `io` is only used when `HasBody == true`). The reference uses blank identifier suppression.

```go
// In resource.go.tmpl — suppress unused import warnings
var (
    _ = fmt.Sprintf
    _ = io.Discard
    _ = url.PathEscape
    _ = os.Exit
    _ = strings.NewReader
    _ = jerrors.ExitOK   // adapt to: cferrors.ExitOK
)
```

The import alias in the confluence template must change: `jerrors` -> `cferrors` (matching confluence-cli's established alias pattern).

### Pattern 6: Verb Deduplication

**What:** When two operations in the same resource group produce the same CLI verb (from DeriveVerb), fall back to the full operationId in kebab-case.

**When to use:** Automatically applied by `deduplicateVerbs` — not a concern unless testing.

### Anti-Patterns to Avoid

- **Tag-based grouping:** Confluence tags have multi-word names with spaces ("Content Properties", "Space Permissions"). Path-based first-segment grouping is simpler, deterministic, and already proven in the reference.
- **Separate go.mod for gen/:** The gen/ package is `package main` within the main module. It imports libopenapi from the main module's go.mod. No separate module needed.
- **Checking `errs != nil` as fatal for Confluence spec:** Confirmed `errs == nil` for the Confluence spec with libopenapi v0.34.3. The reference pattern is correct and safe.
- **Modifying stub.go incrementally:** `cmd/generated/` is cleaned and recreated on each `make generate` run. The stub.go file is deleted by the generator's `os.RemoveAll(outDir)`. Phase 2 must commit the generated output.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OpenAPI spec parsing | Custom JSON/YAML parser | libopenapi v0.34.3 | Handles `$ref` resolution, path-level params, model building |
| Go code formatting | Template whitespace management | `go/format.Source` | Canonical output, catches syntax errors in templates |
| Verb collision detection | None needed | `deduplicateVerbs` from reference | Already solves the problem with full operationId fallback |
| Singular/plural normalization | Word library | `singularize` from reference | Simple heuristic + exceptions map is sufficient for API operationIds |

**Key insight:** The reference implementation's gen/ directory is complete and production-tested. The only custom work is adapting `ExtractResource` for Confluence path structure and updating import paths/module name.

## Common Pitfalls

### Pitfall 1: libopenapi `BuildV3Model` Returns Non-Fatal Warnings as `errs`

**What goes wrong:** In some libopenapi versions, `BuildV3Model` returns both a valid model AND non-nil `errs` (warnings about unresolved `$ref`s, etc.). Treating non-nil `errs` as fatal would abort generation on a valid spec.

**Why it happens:** libopenapi distinguishes parse errors from model-build warnings.

**How to avoid:** Verified: Confluence spec produces `errs == nil` with libopenapi v0.34.3. The reference pattern (`if errs != nil { return error }`) is safe. If a future spec version produces warnings, log them but only fail if `model == nil`.

**Warning signs:** Generator exits with "building model" error on a spec that appears valid.

### Pitfall 2: `stub.go` Type Conflict After Generation

**What goes wrong:** If `cmd/generated/stub.go` is not deleted before generating, the `SchemaOp`, `SchemaFlag` types in `stub.go` conflict with those in the generated `schema_data.go`.

**Why it happens:** `os.RemoveAll(outDir)` in `run()` deletes the entire `cmd/generated/` directory including `stub.go`. But if the generator is run into a different path, or `stub.go` exists in a location the generator doesn't clean, there will be duplicate type declarations.

**How to avoid:** The generator always cleans then recreates the output directory. After running `make generate`, `stub.go` is gone — replaced by `schema_data.go` which defines the same types with real implementations. Commit the generated files (including the deletion of stub.go).

**Warning signs:** `go build` error: "SchemaOp redeclared in this block".

### Pitfall 3: Array-Type Query Parameters Rendered as String Flags

**What goes wrong:** Many Confluence list endpoints accept array query parameters (e.g., `?id=1&id=2&id=3`). The generator's `schemaType` function falls back to `"string"` for array schemas. Generated flags will be `--id string` instead of something that supports multiple values.

**Why it happens:** The reference parser extracts only the first `s.Type[0]` and maps arrays to "string".

**How to avoid:** This is an accepted limitation for Phase 2. Document in SPEC_GAPS.md. Affected endpoints:
- `GET /attachments ?status[]` (array of string)
- `GET /blogposts ?id[]`, `?space-id[]`, `?status[]` (integers and strings)
- `GET /pages ?id[]`, `?space-id[]`, `?status[]`
- Many other list endpoints

Users can pass comma-separated or repeated values via `--body` or `cf raw` for array params until a future enhancement.

**Warning signs:** Array params silently accept only a single value.

### Pitfall 4: Confluence Paths Without Stable First Segment

**What goes wrong:** A path like `/pages/{id}/footer-comments` correctly groups under "pages" (first non-param segment). But if `ExtractResource` naively uses `segments[0]`, a future spec version with paths like `/{something}/...` would produce `{something}` as a resource name.

**Why it happens:** Template paths use `{param}` as placeholders. The Confluence v2 spec currently has NO path-level parameters in the spec root (all params are at operation level), so all first segments are concrete resource names.

**How to avoid:** `ExtractResource` should skip segments that start with `{`. Current Confluence spec has no such paths — but defensive coding is cheap.

### Pitfall 5: `runtime.Caller(0)` Template Path Resolution in Tests

**What goes wrong:** `loadTemplateDefault` uses `runtime.Caller(0)` to find the source file location and resolve `gen/templates/` relative to it. This works when running from the repo root but can fail in unusual test environments.

**Why it happens:** `runtime.Caller(0)` returns the source path baked in at compile time.

**How to avoid:** The reference includes a CWD fallback: if `runtime.Caller` path fails, try `gen/templates/` and `templates/` relative to CWD. Mirror this fallback exactly.

### Pitfall 6: `go run ./gen/...` Requires libopenapi in Main Module

**What goes wrong:** `go run ./gen/...` runs the gen package using the main module's `go.mod`. If libopenapi is not in `go.mod`, compilation fails.

**Why it happens:** gen/ is `package main` within `github.com/sofq/confluence-cli`, not a separate module.

**How to avoid:** Run `go get github.com/pb33f/libopenapi@v0.34.3 && go mod tidy` before implementing gen/.

## Code Examples

### parser.go — Adapted for Confluence

```go
// Source: adapted from /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/parser.go
// Key difference: same libopenapi API, different path structure handled in grouper.go
func ParseSpec(path string) ([]Operation, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading spec: %w", err)
    }
    doc, err := libopenapi.NewDocument(data)
    if err != nil {
        return nil, fmt.Errorf("parsing spec: %w", err)
    }
    model, errs := doc.BuildV3Model()
    if errs != nil {
        return nil, fmt.Errorf("building model: %v", errs)
    }
    if model.Model.Paths == nil {
        return nil, fmt.Errorf("no paths in spec")
    }
    // ... iterate paths, merge path-level + operation-level params ...
}
```

### grouper.go — ExtractResource for Confluence

```go
// Source: adapted from /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/grouper.go
// Confluence paths: /{resource}/... (no /rest/api/3/ prefix)
func ExtractResource(path string) string {
    segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
    // Use first concrete (non-param) segment as resource name.
    for _, s := range segments {
        if s != "" && !strings.HasPrefix(s, "{") {
            return s
        }
    }
    return path
}
```

### resource.go.tmpl — Import Alias Adaptation

```go
// In templates/resource.go.tmpl, change import alias from jerrors to cferrors:
import (
    "fmt"
    "io"
    "net/url"
    "os"
    "strings"

    "github.com/sofq/confluence-cli/internal/client"
    cferrors "github.com/sofq/confluence-cli/internal/errors"
    "github.com/spf13/cobra"
)

var (
    _ = fmt.Sprintf
    _ = io.Discard
    _ = url.PathEscape
    _ = os.Exit
    _ = strings.NewReader
    _ = cferrors.ExitOK
)
```

### main.go — Confluence-Specific Paths

```go
// Source: adapted from /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/main.go
func main() {
    specPath := filepath.Join("spec", "confluence-v2.json")  // not jira-v3.json
    outDir := filepath.Join("cmd", "generated")
    if err := run(specPath, outDir); err != nil {
        log.Println(err)
        exitFn(1)
    }
}
```

### conformance_test.go — Key Adaptation for Operation Count

```go
// Confluence-specific counts (use these in conformance assertions):
// Total operations: 212
// Total resource groups: 24
// Key resources: pages (29), blogposts (24), spaces (20), databases (15)

func TestConformance_OperationCount(t *testing.T) {
    specPath := filepath.Join("..", "spec", "confluence-v2.json")
    ops, err := ParseSpec(specPath)
    // ...
    if len(ops) < 200 {  // Confluence has 212 ops (vs Jira's 600+)
        t.Errorf("expected 200+ operations from Confluence spec, got %d", len(ops))
    }
    if len(groups) < 20 {  // Confluence has 24 resource groups
        t.Errorf("expected 20+ resource groups, got %d", len(groups))
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `cmd/generated/stub.go` (Phase 1 stubs) | Real generated files from spec | Phase 2 | `RegisterAll`, `AllSchemaOps`, `AllResources` go from no-ops to full implementations |
| Manual Cobra command authoring | Code generation from OpenAPI spec | Phase 2 | 212 API operations become CLI commands automatically |
| Jira-specific `/rest/api/3/` path extraction | Confluence flat `/{resource}/` extraction | Phase 2 | `ExtractResource` is the single critical adaptation |

**Deprecated/outdated:**
- `cmd/generated/stub.go`: Deleted by `os.RemoveAll` during `make generate`. No longer needed after Phase 2.

## Spec Gaps (for SPEC_GAPS.md)

These gaps must be documented in `spec/SPEC_GAPS.md` as part of CGEN-05:

### Gap 1: No Attachment Upload in v2 API
`POST /attachments` does not exist in the v2 spec. File upload remains v1-only:
`POST /wiki/rest/api/content/{id}/child/attachment`
Workaround: use `cf raw POST /rest/api/content/{id}/child/attachment --body @file`.

### Gap 2: Deprecated Operation
`GET /pages/{id}/children` (`getChildPages`) is marked deprecated. Generated but flagged.

### Gap 3: EAP / Experimental Operations (18 ops)
These 18 operations carry `x-experimental: true` and/or `EAP` tag — they may change without notice:
`createSpace`, `getAvailableSpacePermissions`, `getAvailableSpaceRoles`, `createSpaceRole`,
`getSpaceRolesById`, `updateSpaceRole`, `deleteSpaceRole`, `getSpaceRoleMode`,
`getSpaceRoleAssignments`, `setSpaceRoleAssignments`, `checkAccessByEmail`, `inviteByEmail`,
`getDataPolicyMetadata`, `getDataPolicySpaces`, `getForgeAppProperties`, `getForgeAppProperty`,
`putForgeAppProperty`, `deleteForgeAppProperty`.

### Gap 4: Array Query Parameters Rendered as String Flags
Many list endpoints accept array-valued query parameters (e.g., `?id=1&id=2`). The generator
renders these as `--flag string` (single value). Affected parameters include:
`status[]`, `id[]`, `space-id[]`, `label-id[]`, `prefix[]` across multiple resources.

## Open Questions

1. **`embeds` resource in spec but not in official Confluence documentation tags**
   - What we know: `embeds` appears as a path-first-segment resource with 12 operations in the spec. It is not listed in the `tags` array in the spec root.
   - What's unclear: Whether these are stable API endpoints or internal/undocumented.
   - Recommendation: Generate as-is. The spec is the source of truth. Document in SPEC_GAPS.md that `embeds` is undocumented in the API tag list.

2. **libopenapi `BuildV3Model` warnings in future spec versions**
   - What we know: Confluence spec v2.0.0 produces `errs == nil` with libopenapi v0.34.3.
   - What's unclear: Whether future spec pins or spec refreshes could produce warnings.
   - Recommendation: Mirror the reference's `if errs != nil { return error }` pattern. If generation ever fails with Confluence warnings, evaluate upgrading to `len(errs) == 0 || model == nil` check.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib) |
| Config file | none — `go test ./...` from project root |
| Quick run command | `go test ./gen/... -count=1` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CGEN-01 | `make generate` runs without error, produces valid Go | integration | `go run ./gen/... && go build ./...` | Wave 0 (gen/ files) |
| CGEN-02 | 24 resource groups extracted from Confluence spec | unit | `go test ./gen/... -run TestConformance_OperationCount` | Wave 0 |
| CGEN-03 | All path/query/body params parsed and rendered as flags | unit | `go test ./gen/... -run TestConformance_AllPathParamsHaveFlags` | Wave 0 |
| CGEN-04 | `mergeCommand` preserves generated subcommands | unit | `go test ./cmd/... -run TestMergeCommand` | exists (root_test.go) |
| CGEN-05 | Spec file present at `spec/confluence-v2.json` | smoke | `test -f spec/confluence-v2.json` | Wave 0 (download) |
| CGEN-01+02 | Generated output matches spec exactly (no stale files) | conformance | `go test ./gen/... -run TestConformance_GeneratedCodeMatchesSpec` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./gen/... -count=1`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `gen/main.go` — pipeline entry point
- [ ] `gen/parser.go` — `ParseSpec`, `Operation`, `Param` types
- [ ] `gen/grouper.go` — `GroupOperations`, `ExtractResource`, `DeriveVerb`
- [ ] `gen/generator.go` — `GenerateResource`, `GenerateSchemaData`, `GenerateInit`, templates
- [ ] `gen/templates/resource.go.tmpl` — per-resource Cobra command template
- [ ] `gen/templates/schema_data.go.tmpl` — schema types and data template
- [ ] `gen/templates/init.go.tmpl` — `RegisterAll` template
- [ ] `gen/main_test.go` — `TestRun*`, `TestMain*`
- [ ] `gen/parser_test.go` — `TestParseSpec_*`
- [ ] `gen/grouper_test.go` — `TestGroupOperations`, `TestExtractResource`, `TestDeriveVerb`
- [ ] `gen/generator_test.go` — `TestBuildPathTemplate*`, `TestLoadTemplate*`
- [ ] `gen/conformance_test.go` — `TestConformance_*`
- [ ] `spec/confluence-v2.json` — pinned spec (download from Atlassian CDN)
- [ ] `spec/SPEC_GAPS.md` — documents known gaps (attachment upload, deprecated, EAP ops, array params)

## Sources

### Primary (HIGH confidence)

- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/gen/` — full source read (main.go, parser.go, grouper.go, generator.go, all templates, all test files)
- Confluence CLI project: `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/` — go.mod, Makefile, cmd/root.go, cmd/generated/stub.go, cmd/schema_cmd.go
- Live spec verification: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` — fetched and parsed with Python to extract path structure, resource groups, op counts, and gaps
- libopenapi spike: `go run` test against Confluence spec with libopenapi v0.34.3 from jira-cli-v2 — confirmed `errs == nil`, 146 paths parsed

### Secondary (MEDIUM confidence)

- libopenapi v0.34.3 indirect deps: extracted from `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/go.mod` and `go.sum` — cross-verified with existing working build

### Tertiary (LOW confidence)

- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — libopenapi version pinned, verified working against Confluence spec
- Architecture: HIGH — reference implementation fully read and analyzed; path adaptation derived from spec analysis
- Pitfalls: HIGH — pitfalls derived from source code inspection and live spec testing; one pitfall (array params) from direct spec data analysis
- Spec gaps: HIGH — gaps derived from automated analysis of the live spec

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (Confluence spec URL is versioned; libopenapi is stable)
