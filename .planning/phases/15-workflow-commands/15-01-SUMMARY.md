---
phase: 15-workflow-commands
plan: 01
subsystem: cli
tags: [cobra, workflow, v1-api, v2-api, long-task-polling, content-lifecycle]

requires:
  - phase: 12-internal-utilities
    provides: duration.Parse for --timeout flag parsing
  - phase: 03-workflow-commands
    provides: fetchV1WithBody, SearchV1Domain, createCommentBody, fetchPageVersion patterns
provides:
  - workflowCmd parent command with 6 subcommands (move, copy, publish, comment, restrict, archive)
  - pollLongTask helper for async v1 operation polling
  - v1 move/copy/restrict/archive + v2 publish/comment CLI wrappers
affects: [15-02-workflow-tests, release-infrastructure]

tech-stack:
  added: []
  patterns: [pollLongTask async polling with deadline+ticker, three-mode restrict command (view/add/remove)]

key-files:
  created: [cmd/workflow.go]
  modified: [cmd/root.go]

key-decisions:
  - "v1 move endpoint (PUT /content/{id}/move/append/{targetId}) over v2 PUT parentId -- reliable dedicated endpoint"
  - "v1 archive endpoint (POST /content/archive) -- no v2 equivalent exists"
  - "pollLongTask returns raw body on unmarshal failure -- graceful degradation for unexpected task response shapes"
  - "Removed unused io import from initial plan spec -- only bytes.NewReader and nil used for body params"

patterns-established:
  - "pollLongTask: deadline+ticker select loop for async v1 operations with configurable timeout"
  - "Three-mode subcommand: view (no flags) / add (--add) / remove (--remove) with mutual exclusivity validation"

requirements-completed: [WKFL-01, WKFL-02, WKFL-03, WKFL-04, WKFL-05, WKFL-06]

duration: 2min
completed: 2026-03-28
---

# Phase 15 Plan 01: Workflow Commands Summary

**Six workflow subcommands (move, copy, publish, comment, restrict, archive) in cmd/workflow.go with v1 async polling and v2 page update patterns**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-28T16:13:46Z
- **Completed:** 2026-03-28T16:16:13Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Implemented all six workflow subcommands covering content lifecycle operations (WKFL-01 through WKFL-06)
- Built pollLongTask helper for async v1 operations (copy and archive) with configurable timeout via duration.Parse
- Three-mode restrict command supports view/add/remove with user and group targets
- All subcommands registered and visible via `cf workflow --help`

## Task Commits

Each task was committed atomically:

1. **Task 1: Create cmd/workflow.go with parent command and all six subcommands** - `d0a4e23` (feat)
2. **Task 2: Register workflowCmd in cmd/root.go** - `948601d` (feat)

## Files Created/Modified
- `cmd/workflow.go` - Parent command + move/copy/publish/comment/restrict/archive subcommands + pollLongTask helper (598 lines)
- `cmd/root.go` - Added rootCmd.AddCommand(workflowCmd) registration for Phase 15

## Decisions Made
- Used v1 move endpoint (PUT /content/{id}/move/append/{targetId}) instead of v2 PUT with parentId -- the v1 endpoint is the dedicated, reliable move mechanism per research
- Used v1 archive endpoint (POST /content/archive) -- no v2 equivalent exists
- pollLongTask returns raw body on JSON unmarshal failure for graceful degradation
- Removed unused `io` import that was in the plan spec but not needed (only `nil` and `bytes.NewReader` used for body parameters)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed unused `io` import**
- **Found during:** Task 1 (compilation)
- **Issue:** Plan spec included `io` in imports but no direct usage in workflow.go (fetchV1WithBody accepts io.Reader but callers pass nil or bytes.NewReader)
- **Fix:** Removed `io` from import block
- **Files modified:** cmd/workflow.go
- **Verification:** `go build ./...` succeeds
- **Committed in:** d0a4e23 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Trivial unused import removal. No scope change.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All six workflow subcommands are implemented and registered
- Ready for Phase 15 Plan 02 (workflow tests)
- All flag validation patterns consistent with existing commands

## Self-Check: PASSED

- cmd/workflow.go: FOUND
- 15-01-SUMMARY.md: FOUND
- Commit d0a4e23: FOUND
- Commit 948601d: FOUND

---
*Phase: 15-workflow-commands*
*Completed: 2026-03-28*
