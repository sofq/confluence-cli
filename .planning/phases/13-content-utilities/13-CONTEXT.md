# Phase 13: Content Utilities - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Built-in presets and templates ship out of the box, users can manage templates (list, show, create from page), list presets, and export page content (single page or recursive tree as NDJSON). This phase wires the internal packages from Phase 12 into user-facing CLI commands.

</domain>

<decisions>
## Implementation Decisions

### Built-in template definitions (CONT-03)
- **D-01:** Built-in templates stored as an embedded Go `map[string]*Template` in `internal/template/template.go` source code — same pattern as built-in presets in `internal/preset/preset.go`
- **D-02:** 6 built-in templates: blank, meeting-notes, decision, runbook, retrospective, adr — bodies are Confluence storage format (XHTML) structural skeletons with headings, sections, and `{{.variable}}` placeholders
- **D-03:** Built-in templates merged with user templates in `templates list` output — same merge pattern as presets (built-in lowest priority, user overrides). Each entry includes name + source tag (builtin/user)

### Template list refactoring
- **D-04:** Refactor `template.List()` to return `[]templateEntry` (name, source) JSON like `preset.List()` — built-in map checked first, user dir overlays with higher priority. Minimal struct: name + source (no body in list output)

### Templates show command (CONT-04)
- **D-05:** `cf templates show <name>` outputs the full Template struct as JSON (title, body, space_id). For built-in templates, serialize from embedded map. For user templates, read the file. Same format either way
- **D-06:** Show output includes a `variables` array extracted by parsing the body for `{{.varName}}` patterns — agents can discover required vars without parsing XHTML

### Templates create from page (CONT-05)
- **D-07:** `cf templates create --from-page <id> --name <name>` fetches the page via v2 API, saves the storage-format body as-is into a template JSON file. User manually adds `{{.variable}}` placeholders later by editing
- **D-08:** Captures title (as title template string) and body only. SpaceID left empty so template is reusable across spaces
- **D-09:** Template file saved to user templates directory (`~/.config/cf/templates/<name>.json`). Requires `--name` flag to specify template name

### Export command (CONT-06, CONT-07)
- **D-10:** `cf export --id <pageId>` outputs page body in requested format. `--format` flag supports all three Confluence v2 body formats: storage (default), atlas_doc_format, view. Passed as `body-format` query param
- **D-11:** `cf export --id <pageId> --tree` recursively exports page tree as NDJSON using v2 child pages API. Depth-first traversal. Each line is a JSON object with id, title, parentId, depth, and body in requested format
- **D-12:** `--depth` flag controls recursion depth (0 = unlimited, default unlimited). Lets agents control tree scope
- **D-13:** Tree export handles partial failures with skip + stderr warning — inaccessible pages logged as APIError JSON to stderr, NDJSON stream continues. Agents see which pages failed

### Preset list command (CONT-01)
- **D-14:** `cf preset list` (singular parent, matches jr exactly). `presetCmd` parent with `presetListCmd` child registered to root
- **D-15:** Output from `preset.List()` flows through standard `--jq` and `--pretty` pipeline, consistent with all other commands
- **D-16:** Passes current profile's presets to `preset.List()` so output reflects all three tiers (builtin, user, profile) — shows what `--preset` would actually resolve to

### Claude's Discretion
- Exact XHTML content for each of the 6 built-in template bodies (structural skeletons appropriate to each template's purpose)
- JQ expression for built-in presets already defined in Phase 12 — no changes needed
- Internal helper functions, error message wording, test case selection
- Template variable extraction implementation details (regex vs template parse)
- NDJSON line field ordering and any additional metadata fields beyond id/title/parentId/depth/body

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### jr reference implementation (architecture mirror)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/preset.go` — Preset list command pattern (mirror for cf)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/template.go` — Template command pattern (mirror for cf)

### Existing cf packages (Phase 12 foundation)
- `internal/preset/preset.go` — Complete preset package: `Lookup()`, `List()`, built-in presets, three-tier resolution
- `internal/template/template.go` — Template package: `List()`, `Load()`, `Render()`, `Dir()`, Template/RenderedTemplate structs
- `internal/jsonutil/jsonutil.go` — `MarshalNoEscape()` for JSON output

### Existing cf commands
- `cmd/templates.go` — Existing `templates list` and `resolveTemplate()` helper (needs refactoring for built-in merge)
- `cmd/root.go` — `--preset` flag wiring (lines 62, 169-186, 263), profile loading

### API endpoints
- `cmd/generated/pages.go` — v2 pages API: get page (with body-format), get child pages (for tree walk)

### Phase 10 context (decisions carried forward)
- `.planning/phases/10-output-presets-and-templates/10-CONTEXT.md` — Original preset/template design decisions

### Phase 12 context (foundation packages)
- `.planning/phases/12-internal-utilities/12-CONTEXT.md` — Internal package decisions, built-in preset definitions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/preset/preset.go`: Complete — `Lookup()`, `List()`, 7 built-in presets, three-tier resolution all working
- `internal/template/template.go`: `List()`, `Load()`, `Render()` — needs extension for built-in templates and show
- `cmd/templates.go`: `templates list` command + `resolveTemplate()` helper — needs refactoring
- `internal/jq/jq.go`: `Apply()` — for --jq pipeline on preset list output
- `internal/errors/errors.go`: `APIError` struct — for export error handling
- `cmd/generated/pages.go`: v2 page get (with body-format) and child pages endpoints

### Established Patterns
- Built-in data as embedded Go maps (presets pattern) — reuse for templates
- Three-tier resolution: profile > user file > built-in — already in preset, apply to templates
- List commands return JSON arrays with source attribution
- All output through --jq/--pretty pipeline via root command flags
- APIError JSON to stderr for structured error reporting
- NDJSON output for streaming (established in watch command, Phase 11)

### Integration Points
- `cmd/root.go`: Register `presetCmd` and `exportCmd` as root subcommands
- `cmd/templates.go`: Add `show` and `create` subcommands to existing `templatesCmd`
- `internal/template/template.go`: Add built-in map, refactor `List()` return type, add `Show()` and variable extraction
- `cmd/generated/pages.go`: Use page get endpoint (body-format param) and child pages endpoint for export

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

*Phase: 13-content-utilities*
*Context gathered: 2026-03-28*
