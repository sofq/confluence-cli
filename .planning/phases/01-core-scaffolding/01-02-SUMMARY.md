---
phase: 01-core-scaffolding
plan: 02
subsystem: api
tags: [http-client, confluence, cursor-pagination, caching, jq, cobra]

# Dependency graph
requires:
  - phase: 01-01
    provides: "errors, config, cache, jq packages with types used by client"
provides:
  - "Client struct with Do(), Fetch(), WriteOutput(), ApplyAuth(), VerboseLog()"
  - "NewContext(), FromContext() for Cobra command integration"
  - "QueryFromFlags() for Cobra flag-to-URL-query mapping"
  - "Confluence cursor-based pagination via detectCursorPagination() and doCursorPagination()"
affects: [cmd, 02-pages, 03-content, 04-spaces, 05-search, all-cobra-commands]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "context key type pattern (unexported struct) for safe context storage"
    - "cursor-based pagination following _links.next until empty"
    - "pagination envelope reconstruction: merge results[], strip _links.next"
    - "cache-check-before-fetch pattern in doWithPagination"
    - "VerboseLog to stderr only; all responses to stdout"

key-files:
  created:
    - internal/client/client.go
  modified: []

key-decisions:
  - "Used encoding/json indent for pretty-print instead of tidwall/pretty — avoids external dependency; functionally equivalent for Phase 1"
  - "oauth2 auth type removed from ApplyAuth — basic + bearer only per INFRA-05, Phase 4 deferred"
  - "Phase 4 fields (AuditLogger, Policy, Operation, Profile) omitted from Client struct — clean separation of concerns"

patterns-established:
  - "All HTTP responses written as JSON to stdout; errors as structured JSON to stderr"
  - "Non-zero exit codes on HTTP errors via cferrors.ExitCode constants"
  - "Cursor pagination: detect _links + results envelope, follow _links.next, merge results[], clear next in final output"

requirements-completed: [INFRA-01, INFRA-05, INFRA-07, INFRA-08, INFRA-10, INFRA-11]

# Metrics
duration: 5min
completed: 2026-03-20
---

# Phase 01 Plan 02: HTTP Client with Confluence Cursor Pagination Summary

**net/http Client with cursor-paginated GET support following Confluence v2 _links.next pattern, auth (basic/bearer), caching, dry-run, verbose, JQ filtering, and JSON-only stdout contract**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-20T00:49:00Z
- **Completed:** 2026-03-20T00:53:47Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- `Client` struct with all required fields — no Phase 4 fields (AuditLogger, Policy, Operation, Profile)
- `Do()` with dry-run, field injection, pagination dispatch, and single-shot path
- `Fetch()` returning raw bytes without writing to stdout — for workflow/batch callers
- `WriteOutput()` with JQ filter, pretty-print, and trailing-newline normalization
- `ApplyAuth()` supporting basic and bearer (oauth2 removed per INFRA-05 decision)
- Confluence cursor pagination: `detectCursorPagination()`, `doWithPagination()`, `doCursorPagination()`
- Pagination follows `_links.next` chains, merges all `results[]` arrays, strips `_links.next` from final envelope
- Cache integration via `cache.Key()`, `cache.Get()`, `cache.Set()` in both `doOnce` and `doWithPagination`
- `NewContext()` / `FromContext()` / `QueryFromFlags()` for Cobra command wiring

## Task Commits

Each task was committed atomically:

1. **Task 1: HTTP client with cursor-based pagination** - `a50b7c9` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `internal/client/client.go` — Complete HTTP client (~310 LOC): Client struct, Do, Fetch, WriteOutput, ApplyAuth, VerboseLog, cursor pagination, context helpers

## Decisions Made
- **pretty-print without external dep:** Reference uses `tidwall/pretty` which is not in go.mod. Used `encoding/json.Indent` instead — same behavior for Phase 1, no dependency addition required. (Rule 3 auto-fix — blocking import avoided)
- **oauth2 removed from ApplyAuth:** Matches Phase 1 constraint in INFRA-05 and existing config.validAuthTypes decision from Plan 01.
- **Phase 4 fields excluded:** AuditLogger, Policy, Operation, Profile stripped from Client struct — clean phase boundary.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Replaced tidwall/pretty with encoding/json.Indent**
- **Found during:** Task 1 (creating client.go)
- **Issue:** Reference client.go imports `github.com/tidwall/pretty` which is absent from go.mod. Adding it would require `go get` and a new dependency not in the plan.
- **Fix:** Used `json.Indent` from stdlib in `WriteOutput()` — functionally equivalent for the Pretty flag behavior needed in Phase 1.
- **Files modified:** internal/client/client.go
- **Verification:** `go build ./internal/client/...` exits 0; no external dependency required.
- **Committed in:** a50b7c9 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Auto-fix avoids unnecessary dependency; no behavioral difference for Phase 1 usage.

## Issues Encountered
None — plan executed cleanly after the pretty-print substitution.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/client` is fully importable; `go build ./internal/...` exits 0
- All Cobra commands in Phase 2+ can use `client.FromContext(cmd.Context())` to get the configured client
- Cursor pagination is transparent to callers — they just set `client.Paginate = true`
- Cache, JQ, verbose, and dry-run modes all wired and ready

---
*Phase: 01-core-scaffolding*
*Completed: 2026-03-20*
