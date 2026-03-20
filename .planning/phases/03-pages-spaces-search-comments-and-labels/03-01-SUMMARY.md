---
phase: 03-pages-spaces-search-comments-and-labels
plan: "01"
subsystem: api
tags: [confluence, pages, cobra, go, storage-format, version-increment]

# Dependency graph
requires:
  - phase: 02-code-generation-pipeline
    provides: cmd/generated/pages.go with generated page subcommands for mergeCommand to preserve

provides:
  - pagesCmd parent command (var pagesCmd *cobra.Command) for mergeCommand wiring in Plan 04
  - fetchPageVersion(ctx, c, id) helper — GET /pages/{id} returning current version.number
  - doPageUpdate(ctx, c, id, title, storageValue, versionNumber) helper — PUT /pages/{id} with storage body
  - pageUpdateBody struct for type-safe PUT request marshalling
  - Five page operation subcommands on pagesCmd: get-by-id, create, update, delete, get

affects:
  - 03-04-PLAN.md (root.go wiring via mergeCommand — must call mergeCommand(rootCmd, pagesCmd))

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Workflow command pattern: hand-written cmd/*.go files override generated subcommands via mergeCommand"
    - "body-format=storage injection: get-by-id always sets body-format=storage as default query param"
    - "Version auto-increment: fetchPageVersion then doPageUpdate, retry once on ExitConflict"
    - "Friendly flag pattern: create uses --space-id/--title/--body flags to build JSON body internally"

key-files:
  created:
    - cmd/pages.go
  modified: []

key-decisions:
  - "pages_workflow_list uses Use: 'get' (not 'list') to match generated subcommand name for mergeCommand override"
  - "pages_workflow_get_by_id defaults body-format=storage but allows explicit override via --body-format flag"
  - "doPageUpdate returns int (not error) — callers wrap non-zero return in AlreadyWrittenError"
  - "init() in pages.go does NOT call mergeCommand or rootCmd.AddCommand — Plan 04 handles wiring"

patterns-established:
  - "Workflow RunE pattern: client.FromContext -> validate flags -> build request -> c.Fetch/c.Do -> return AlreadyWrittenError on failure"
  - "Retry pattern: fetch version -> attempt update -> on ExitConflict re-fetch version -> retry once"

requirements-completed: [PAGE-01, PAGE-02, PAGE-03, PAGE-04, PAGE-05]

# Metrics
duration: 3min
completed: 2026-03-20
---

# Phase 03 Plan 01: Pages Workflow Commands Summary

**Hand-written cmd/pages.go with five Confluence page operations: get-by-id (storage body auto-inject), create (friendly flags), update (version auto-increment + 409 retry), delete, and list**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-20T03:07:15Z
- **Completed:** 2026-03-20T03:10:00Z
- **Tasks:** 2 (both tasks implemented in single file, committed as part of prior 03-03 session)
- **Files modified:** 1

## Accomplishments

- `pagesCmd` parent command defined and ready for `mergeCommand` wiring in Plan 04
- `fetchPageVersion` helper: GET /pages/{id}, unmarshals `version.number`, returns `(int, int)`
- `doPageUpdate` helper: PUT /pages/{id} with storage body struct, calls `c.WriteOutput` on success
- Five subcommands registered on `pagesCmd`: get-by-id, create, update, delete, get
- `pages_workflow_update` implements the version fetch → increment → retry-on-409 pattern

## Task Commits

Both tasks were implemented together in a single prior session commit:

1. **Task 1: Helper functions (pagesCmd, fetchPageVersion, doPageUpdate)** - `f427b78` (feat)
2. **Task 2: Five operation subcommands** - `f427b78` (feat)

**Plan metadata:** see final commit below

_Note: Both tasks were committed together in a prior session as part of commit f427b78 (feat(03-03))_

## Files Created/Modified

- `cmd/pages.go` — pagesCmd parent command, fetchPageVersion, doPageUpdate, pageUpdateBody, and five page operation subcommands with init() flag registration

## Decisions Made

- `pages_workflow_list` uses `Use: "get"` to match the generated subcommand name so `mergeCommand` will replace it correctly
- `get-by-id` defaults `body-format=storage` but respects an explicit `--body-format` flag override
- The `init()` function in `cmd/pages.go` does NOT wire to `rootCmd` — Plan 04 calls `mergeCommand(rootCmd, pagesCmd)`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `cmd/spaces_test.go` (pre-existing untracked file from another session) references `testAuth()` and `resolveSpaceIDExported` which are not yet defined. This causes `go vet ./cmd/...` to fail for the test package only. Logged as out-of-scope deferred item — `go build ./...` passes cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `cmd/pages.go` is complete and ready for Plan 04 to call `mergeCommand(rootCmd, pagesCmd)`
- `go build ./...` passes with no errors
- All five page operation subcommands compile and are registered on `pagesCmd`

---
*Phase: 03-pages-spaces-search-comments-and-labels*
*Completed: 2026-03-20*
