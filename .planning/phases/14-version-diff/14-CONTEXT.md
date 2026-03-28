# Phase 14: Version Diff - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

A `cf diff` command that compares page versions and outputs structured JSON with version metadata and change statistics. Supports time-range filtering (`--since`) and explicit version comparison (`--from`/`--to`). Uses the duration parser from Phase 12 and follows the same command patterns established across the codebase.

</domain>

<decisions>
## Implementation Decisions

### Diff output structure (DIFF-01)
- **D-01:** Output is always a `diffs` array for consistent shape — even when comparing just two most recent versions (single-element array). Agents can always expect the same JSON structure
- **D-02:** Each diff entry contains `from` (version metadata: number, authorId, createdAt, message), `to` (same fields), and `stats` (linesAdded, linesRemoved)
- **D-03:** No body content in diff output — agents use `pages get-by-id --version N` if they need the full body. Keeps diff output small and focused
- **D-04:** Line stats computed by splitting storage format on `\n` and running a simple line-level diff. No XHTML-aware parsing — treat as plain text. Fast, zero dependencies

### --since behavior (DIFF-02)
- **D-05:** `--since` supports both human durations (2h, 1d, 1w) via `duration.Parse()` and ISO date strings (2026-01-01, RFC3339) — mirror jr's `parseSince()` pattern from `internal/changelog/`
- **D-06:** When `--since` captures multiple versions (e.g. v3, v4, v5 all within the time range), output contains pairwise diffs between all adjacent versions (v3→v4, v4→v5). Agents see the full change timeline

### Explicit version comparison (DIFF-03)
- **D-07:** `--from` and `--to` flags specify version numbers for explicit comparison. Output is a single-element `diffs` array with the diff between those two versions

### Body retrieval strategy
- **D-08:** Use v2 `GET /pages/{id}` with `?version=N&body-format=storage` query params to retrieve historical version bodies for diff computation
- **D-09:** If v2 API doesn't return body content for historical versions (needs live API validation during research), fall back to metadata-only diff with `stats` field omitted and an informational note. Do NOT silently return zero stats
- **D-10:** No v1 API fallback for body retrieval — maintain v2-primary approach consistent with project constraints

### Edge case behavior
- **D-11:** Single version (no prior to diff): Return `diffs` array with single entry, `from: null`, `to` has version metadata, stats show all lines as added
- **D-12:** Empty `--since` range (no versions in time window): Return `{"pageId": "...", "since": "2h", "diffs": []}` — empty array, not an error
- **D-13:** `--from` equals `--to`: Return diff entry with zero stats — not an error
- **D-14:** Page not found or permission denied: Standard APIError JSON to stderr, same pattern as all other commands

### Claude's Discretion
- Internal package structure (likely `internal/diff/` mirroring jr's `internal/changelog/`)
- Exact diff algorithm implementation (Myers, patience, or simple LCS)
- Version metadata field names matching actual v2 API response shape
- Test case selection and organization
- Whether `--from`/`--to` are mutually exclusive with `--since`, or can combine

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### jr reference implementation (architecture mirror)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/diff.go` — Diff command pattern: flags, error handling, output pipeline
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/changelog/changelog.go` — Internal package pattern: `Parse()` function, `Options` struct, `parseSince()` for duration+ISO date parsing, `Result` struct

### Existing cf packages (Phase 12 foundation)
- `internal/duration/duration.go` — `Parse(s string) (time.Duration, error)` for `--since` flag
- `internal/jsonutil/jsonutil.go` — `MarshalNoEscape()` for JSON output

### Generated API endpoints
- `cmd/generated/pages.go` lines 854-921 — `pages get-versions` (list all versions for a page) and `pages get-version-details` (single version details)
- `cmd/generated/pages.go` line 146 — `pages get-by-id` with `--version` flag for retrieving historical version body

### Existing cf commands (pattern reference)
- `cmd/export.go` — Recent hand-written command: flag handling, API calls, error patterns, NDJSON output
- `cmd/root.go` — `--jq`, `--preset`, `--pretty` pipeline wiring

### Phase 12 context
- `.planning/phases/12-internal-utilities/12-CONTEXT.md` — Duration package decisions (calendar conventions, supported units)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/duration/duration.go`: `Parse()` — used for `--since` duration values
- `internal/jsonutil/jsonutil.go`: `MarshalNoEscape()` — for JSON output without HTML escaping
- `internal/errors/errors.go`: `APIError` struct + `WriteJSON()` — for structured error output
- `internal/client/client.go`: `FromContext()`, `Do()`, `Fetch()` — API call patterns
- `internal/preset/preset.go`: Built-in `"diff"` preset already defined

### Established Patterns
- Hand-written commands in `cmd/` follow: flag parsing → client creation → API call → marshal → WriteOutput
- jr's diff uses `internal/changelog.Parse(issueKey, body, opts)` → returning `*Result` — mirror with `internal/diff.Compare()` or similar
- All output through `--jq`/`--preset`/`--pretty` pipeline via root command
- APIError JSON to stderr for all errors

### Integration Points
- `cmd/root.go`: Register `diffCmd` as root subcommand
- `internal/duration`: Import for `--since` parsing
- `internal/jsonutil`: Import for `MarshalNoEscape()`
- `cmd/generated/pages.go`: Use `pages get-by-id` with version param for body retrieval, `pages get-versions` for version listing

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

*Phase: 14-version-diff*
*Context gathered: 2026-03-28*
