---
phase: 15-workflow-commands
plan: 02
subsystem: testing
tags: [cobra, httptest, workflow, v1-api, v2-api, mock-server]

# Dependency graph
requires:
  - phase: 15-workflow-commands/15-01
    provides: "Workflow subcommands (move, copy, publish, comment, restrict, archive)"
provides:
  - "Comprehensive test suite for all six workflow subcommands"
  - "runWorkflowCommand test helper for workflow integration tests"
  - "resetWorkflowFlags helper preventing Cobra singleton flag contamination"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Workflow test helper with Cobra flag reset for all six subcommands"
    - "v1 and v2 mock server patterns in same test file"

key-files:
  created:
    - cmd/workflow_test.go
  modified: []

key-decisions:
  - "Reused setupTemplateEnv and dummyServer from existing test files (same package)"
  - "resetWorkflowFlags iterates workflow subcommands and re-registers all flags to match init()"
  - "v1 endpoints tested via httptest mux with /wiki/rest/api/ paths"
  - "v2 endpoints tested via httptest mux with /wiki/api/v2/ paths"

patterns-established:
  - "resetWorkflowFlags: centralized flag reset for nested subcommands under workflow parent"
  - "Mixed v1/v2 mock server: single mux handles both /wiki/rest/api/ and /wiki/api/v2/ paths"

requirements-completed: [WKFL-01, WKFL-02, WKFL-03, WKFL-04, WKFL-05, WKFL-06]

# Metrics
duration: 2min
completed: 2026-03-28
---

# Phase 15 Plan 02: Workflow Tests Summary

**21 tests covering validation, API integration, and edge cases for all six workflow subcommands (move, copy, publish, comment, restrict, archive)**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-28T16:18:32Z
- **Completed:** 2026-03-28T16:20:30Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- 13 validation tests verifying JSON error output on stderr for missing/invalid flags
- 8 API integration tests with mock HTTP servers confirming correct method, path, body, and output
- Test helper pair (runWorkflowCommand + resetWorkflowFlags) prevents Cobra singleton contamination
- Full cmd test suite passes with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Create cmd/workflow_test.go with test helper and validation tests** - `fc86286` (test)

## Files Created/Modified
- `cmd/workflow_test.go` - 663-line test file with runWorkflowCommand helper, resetWorkflowFlags helper, 13 validation tests, and 8 API integration tests

## Decisions Made
- Reused setupTemplateEnv and dummyServer from existing test infrastructure (same cmd_test package)
- Created resetWorkflowFlags as a centralized helper that iterates all workflow subcommands, matching the exact flag registrations in init()
- Used httptest.NewServer with http.NewServeMux for tests needing multiple route handlers (publish needs both GET and PUT on same path)
- For restrict view mode, tested that no --add/--remove flags triggers GET to /restriction path

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All workflow subcommands fully tested with both validation and integration coverage
- Phase 15 (workflow-commands) is complete -- both implementation and tests delivered
- Ready for phase transition

---
*Phase: 15-workflow-commands*
*Completed: 2026-03-28*
