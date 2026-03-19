# Phase 1: Core Scaffolding - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the foundational Go module, HTTP client, config/profile system, auth layer, and all infrastructure flags (JSON output, semantic exit codes, JQ filtering, pagination, caching, dry-run, verbose, raw API, schema discovery, version). This phase delivers no resource-specific commands — only the plumbing that every subsequent phase depends on.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

All implementation choices are at Claude's discretion — pure infrastructure phase. Mirror the `jr` (jira-cli-v2) reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` for all patterns: directory structure, client architecture, config resolution, error handling, and flag design.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` — full Go CLI with identical architecture
- `internal/client/client.go` (685 LOC) — HTTP client with auth, pagination, caching, JQ filtering
- `internal/config/config.go` — profile-based config resolution (flags > env > file > defaults)
- `internal/errors/errors.go` — structured JSON errors with semantic exit codes
- `internal/jq/jq.go` — gojq wrapper for in-process JQ filtering
- `internal/cache/cache.go` — GET response caching with TTL

### Established Patterns
- Cobra PersistentPreRunE for client injection via cmd.Context()
- `Do()` for generated commands (executes + writes output)
- `Fetch()` for workflow commands (executes + returns bytes)
- `AlreadyWrittenError` sentinel to prevent double-writing errors
- Config prefix: `CF_` (matching `JR_` pattern)

### Integration Points
- `main.go` → `cmd.Execute()` → `cmd/root.go` (entry point chain)
- `generated.RegisterAll(rootCmd)` (will be wired in Phase 2)
- Confluence v2 API base URL: `https://{domain}/wiki/api/v2`
- Cursor-based pagination (differs from Jira's offset-based)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase. Mirror jr architecture exactly.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
