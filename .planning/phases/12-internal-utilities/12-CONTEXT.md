# Phase 12: Internal Utilities - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Three pure-logic internal packages — `internal/jsonutil`, `internal/duration`, `internal/preset` — providing the foundation that all subsequent v1.2 CLI commands depend on. Also includes refactoring all existing code to adopt `jsonutil.MarshalNoEscape()` and wiring `preset.Lookup()` into `cmd/root.go`.

</domain>

<decisions>
## Implementation Decisions

### jsonutil package (UTIL-01)
- **D-01:** Create `internal/jsonutil/` package with `MarshalNoEscape(v any) ([]byte, error)` that serializes to JSON without HTML-escaping `&`, `<`, `>` characters
- **D-02:** Refactor ALL existing call sites (12+ locations) using inline `enc.SetEscapeHTML(false)` across `internal/client/client.go`, `cmd/root.go`, `cmd/watch.go`, `cmd/batch.go`, `internal/jq/jq.go`, `internal/errors/errors.go`, `cmd/schema_cmd.go` to use the new `jsonutil.MarshalNoEscape()` — full consolidation in one shot
- **D-03:** Remove the existing `marshalNoEscape()` function from `cmd/schema_cmd.go` and replace all its call sites with `jsonutil.MarshalNoEscape()`

### duration package (UTIL-02)
- **D-04:** Create `internal/duration/` package with `Parse(s string) (time.Duration, error)` returning Go's standard `time.Duration` type
- **D-05:** Use **calendar time conventions** (differs from jr's work-time): 1d = 24h, 1w = 7d = 168h
- **D-06:** Support four units only: `w` (weeks), `d` (days), `h` (hours), `m` (minutes) — no months
- **D-07:** Support compound expressions like `1d 3h` (same as jr)

### preset package (UTIL-03)
- **D-08:** Create `internal/preset/` package with `Lookup(name string, profilePresets map[string]string) (string, string, error)` returning (JQ expression, source, error) through three-tier resolution
- **D-09:** Three-tier resolution order: profile config (highest) > user preset file > built-in (lowest). Profile presets passed in from caller, not read internally
- **D-10:** Built-in presets defined as an embedded Go `map[string]string` in package source code — compiled into binary, no external files
- **D-11:** User preset file at `~/.config/cf/presets.json` (via `os.UserConfigDir()` with fallback), `map[string]string` JSON format
- **D-12:** Preset values are pure JQ expression strings — no struct with Fields like jr. Matches cf's existing `config.Profile.Presets map[string]string`
- **D-13:** `List(profilePresets map[string]string) ([]byte, error)` returns all available presets as JSON with source attribution (builtin/user/profile)
- **D-14:** 7 built-in presets: brief, titles, agent, tree, meta, search, diff — JQ expressions designed by Claude during planning based on Confluence v2 API response schemas
- **D-15:** Wire `preset.Lookup()` into `cmd/root.go` replacing the current inline `rawProfile.Presets[preset]` lookup, enabling three-tier resolution for all `--preset` usage

### Claude's Discretion
- Exact JQ expressions for the 7 built-in presets (designed from API response schemas)
- Internal helper functions and error message wording
- Test case selection and organization
- Whether `jsonutil` also exposes an `Encoder` helper or just the `MarshalNoEscape` function (based on call site needs)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### jr reference implementation (architecture mirror)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/jsonutil/jsonutil.go` — MarshalNoEscape pattern (adapt for cf)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/duration/duration.go` — Duration parsing pattern (adapt: calendar time, return time.Duration)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/preset/preset.go` — Preset resolution pattern (adapt: three-tier, pure JQ strings)

### Existing cf code (refactoring targets)
- `internal/client/client.go` — 4 SetEscapeHTML(false) call sites to refactor
- `cmd/root.go` — SetEscapeHTML(false) sites + preset resolution to replace (lines ~171-186)
- `cmd/schema_cmd.go` — marshalNoEscape() function to remove (lines 98-103), call sites to update
- `cmd/watch.go` — SetEscapeHTML(false) call site
- `cmd/batch.go` — SetEscapeHTML(false) call site
- `internal/jq/jq.go` — SetEscapeHTML(false) call site
- `internal/errors/errors.go` — SetEscapeHTML(false) call site
- `cmd/version.go` — uses marshalNoEscape from schema_cmd.go
- `cmd/configure.go` — uses marshalNoEscape from schema_cmd.go

### Config system
- `internal/config/config.go` — Profile.Presets field (existing, line 36)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/schema_cmd.go:marshalNoEscape()` — existing implementation to extract into `internal/jsonutil/`
- `internal/config/config.go:Profile.Presets` — existing profile-level preset map, becomes the top tier
- `internal/jq/jq.go:Apply()` — presets resolve to JQ expressions consumed by this function

### Established Patterns
- Internal packages follow `internal/{name}/{name}.go` + `internal/{name}/{name}_test.go` convention
- Functions are exported, package-level vars used for testability (e.g., `var userPresetsPath = func()`)
- `json.NewEncoder` + `SetEscapeHTML(false)` + `bytes.Buffer` pattern used throughout
- Config dir via `os.UserConfigDir()` with home dir fallback (matches jr)

### Integration Points
- `cmd/root.go PersistentPreRunE` — where preset resolution runs (replace inline lookup with preset.Lookup)
- All cmd files using `enc.SetEscapeHTML(false)` — refactored to import `internal/jsonutil`
- Phase 13 will add `cf preset list` command that calls `preset.List()`
- Phase 14 will use `duration.Parse()` for `--since` flag

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

*Phase: 12-internal-utilities*
*Context gathered: 2026-03-28*
