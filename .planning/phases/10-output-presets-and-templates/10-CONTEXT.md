# Phase 10: Output Presets and Templates - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Two distinct features: (1) named output presets that apply saved JQ expressions via `--preset <name>`, and (2) a content template system that creates pages/blogposts from template files with variable substitution. Both are config/file-based features with no new API endpoints.

</domain>

<decisions>
## Implementation Decisions

### Preset storage
- Presets stored per-profile in config.json under a `presets` map
- Structure: `"presets": { "brief": ".results[] | {id, title}", "titles": ".results[].title" }`
- Different profiles can have different presets
- `--preset <name>` flag on root command (applies globally like --jq)
- Preset is resolved to a JQ expression, then passed to existing `jq.Apply()` — no new JQ logic needed

### Template location
- Templates stored in `~/.config/cf/templates/` (OS-appropriate config dir, same as config.json)
- Each template is a file (e.g., `meeting-notes.json`) containing a JSON structure with Go text/template syntax in the body field
- `cf templates list` reads the directory and lists available templates
- Template file format: `{"title": "{{.title}}", "body": "<p>Meeting on {{.date}}</p>", "space_id": "{{.space_id}}"}`

### Template variables
- Variables passed via repeated `--var key=value` flags
- Example: `cf pages create --template meeting-notes --var "date=2026-03-20" --var "attendees=Alice,Bob"`
- Variables are parsed into `map[string]string` and passed to Go `text/template.Execute()`
- Missing variables produce an error (no silent empty substitution)

### Claude's Discretion
- Whether presets command needs a `cf presets list` subcommand or just documentation
- Template file extension (.json, .tmpl, or both)
- Whether to add a `cf templates show <name>` command to preview a template
- Error messages for missing presets or templates

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### JQ filtering (existing)
- `internal/jq/jq.go` — Existing JQ filter implementation via gojq
- `cmd/root.go` — --jq flag handling in PersistentPreRunE (line 60) and Client construction (line 204)

### Config system
- `internal/config/config.go` — Profile struct where presets map will be added
- Config path resolution: DefaultPath() for OS-appropriate config directory

### Template security
- `.planning/research/PITFALLS.md` — SSTI prevention: use `map[string]string` as template data, not struct

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/jq/jq.go`: `Apply(input, filter)` — presets resolve to a JQ expression passed here
- `internal/config/config.go`: `Profile` struct — add `Presets map[string]string` field
- `config.DefaultPath()` — derive templates directory from same config root

### Established Patterns
- Global flags on root command (--jq, --cache, --dry-run) — --preset follows same pattern
- Config resolution: file → env → flag priority

### Integration Points
- `cmd/root.go PersistentPreRunE`: Resolve --preset to JQ expression before Client construction
- `cmd/root.go`: Register --preset flag alongside --jq
- Templates integrate with existing `cf pages create` and `cf blogposts create` via --template flag

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard config-based presets and file-based templates.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-output-presets-and-templates*
*Context gathered: 2026-03-20*
