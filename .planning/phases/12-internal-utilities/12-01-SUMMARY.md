---
phase: 12-internal-utilities
plan: 01
subsystem: utilities
tags: [json, duration, parsing, stdlib]

# Dependency graph
requires: []
provides:
  - "internal/jsonutil package with MarshalNoEscape and NewEncoder (no HTML escaping)"
  - "internal/duration package with Parse returning time.Duration (calendar conventions)"
affects: [13-diff-command, 14-workflow-commands, 15-presets-templates, schema_cmd, errors]

# Tech tracking
tech-stack:
  added: []
  patterns: ["SetEscapeHTML(false) consolidated in jsonutil", "regex-based duration parsing with fullPattern validation"]

key-files:
  created:
    - internal/jsonutil/jsonutil.go
    - internal/jsonutil/jsonutil_test.go
    - internal/duration/duration.go
    - internal/duration/duration_test.go
  modified: []

key-decisions:
  - "Calendar time conventions for duration: 1d=24h, 1w=168h (not Jira work-time 1d=8h, 1w=40h)"
  - "White-box tests (same package) matching jr test pattern"
  - "NewEncoder added beyond jr pattern for streaming use cases (errors.go, watch.go)"

patterns-established:
  - "internal/{name}/{name}.go + {name}_test.go package structure"
  - "Table-driven tests with t.Run subtests for comprehensive coverage"
  - "Regex pair pattern: unitPattern for extraction + fullPattern for validation"

requirements-completed: [UTIL-01, UTIL-02]

# Metrics
duration: 2min
completed: 2026-03-28
---

# Phase 12 Plan 01: Internal Utilities Summary

**jsonutil package (MarshalNoEscape + NewEncoder) and duration package (Parse with calendar-time 1d=24h, 1w=168h) -- zero external dependencies, 25 tests**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-28T13:50:21Z
- **Completed:** 2026-03-28T13:52:29Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Created internal/jsonutil with MarshalNoEscape and NewEncoder consolidating the SetEscapeHTML(false) pattern
- Created internal/duration with Parse returning time.Duration using calendar conventions (1d=24h, 1w=168h)
- 25 tests total: 5 for jsonutil, 20 for duration (18 table-driven + 2 error message assertions)
- Zero new external dependencies -- all stdlib

## Task Commits

Each task was committed atomically (TDD: test then feat):

1. **Task 1: Create internal/jsonutil package** - `68a214c` (test) + `a9fb2fd` (feat)
2. **Task 2: Create internal/duration package** - `aa0afeb` (test) + `74856c7` (feat)

_Note: TDD tasks have two commits each (RED: failing test, GREEN: implementation)_

## Files Created/Modified
- `internal/jsonutil/jsonutil.go` - MarshalNoEscape and NewEncoder functions with SetEscapeHTML(false)
- `internal/jsonutil/jsonutil_test.go` - 5 tests: no-escape map, no trailing newline, error case, encoder no-escape, encoder script tag
- `internal/duration/duration.go` - Parse function converting human duration strings to time.Duration
- `internal/duration/duration_test.go` - 20 tests: all units, compounds, edge cases, error messages

## Decisions Made
- Used calendar time conventions (1d=24h, 1w=168h) instead of Jira work-time (1d=8h, 1w=40h) per plan decision D-06
- Added NewEncoder beyond jr's pattern to support streaming use cases (errors.go WriteJSON, watch.go)
- White-box testing (same package name) matching jr test conventions

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- jsonutil ready for adoption in schema_cmd.go, errors.go, and all future JSON output paths
- duration ready for --since flag in Phase 14 workflow commands
- Both packages have comprehensive test suites for regression safety

## Self-Check: PASSED

All 4 source files exist, all 4 commits verified in git log.

---
*Phase: 12-internal-utilities*
*Completed: 2026-03-28*
