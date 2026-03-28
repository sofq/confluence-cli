---
phase: 14-version-diff
plan: 02
subsystem: cli
tags: [cobra, diff, version-comparison, httptest, confluence-api-v2]

# Dependency graph
requires:
  - phase: 14-version-diff plan 01
    provides: internal/diff package with Compare(), ParseSince(), LineStats(), types
  - phase: 12-internal-utilities
    provides: internal/duration, internal/jsonutil packages
provides:
  - cf diff command with --id, --since, --from/--to flags
  - Version fetching with pagination from Confluence v2 API
  - Command registration in root.go
affects: [15-workflow-commands, 16-ci-cd]

# Tech tracking
tech-stack:
  added: []
  patterns: [pre-filter-before-fetch, cobra-flag-reset-in-tests]

key-files:
  created: [cmd/diff.go, cmd/diff_test.go]
  modified: [cmd/root.go]

key-decisions:
  - "Pre-filter versions by --since cutoff before fetching bodies (avoids unnecessary API calls for old versions)"
  - "Cobra flag reset in test helper to prevent global state contamination between sequential tests"
  - "Registered diffCmd in root.go during Task 1 (needed for test execution, merged Task 2 scope)"

patterns-established:
  - "Pre-filter pattern: filter API list results before fetching detail for each item"
  - "Test flag reset: reset subcommand flags + persistent root flags in test helper for cobra singleton"

requirements-completed: [DIFF-01, DIFF-02, DIFF-03]

# Metrics
duration: 9min
completed: 2026-03-28
---

# Phase 14 Plan 02: Diff Command Summary

**cf diff cobra command wiring API version fetching to internal/diff package with default, since, and from/to modes**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-28T15:38:20Z
- **Completed:** 2026-03-28T15:47:42Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created `cf diff` command with three modes: default (two most recent), --since (time-range filtered), --from/--to (explicit versions)
- Integrated version list fetching with cursor pagination and per-version body retrieval from Confluence v2 API
- Implemented dry-run, validation errors, body-unavailable handling per all 14 locked decisions
- Registered diffCmd in root.go as root subcommand
- 8 integration tests with httptest servers covering all modes and edge cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Create diff command (TDD RED)** - `744c837` (test)
2. **Task 1: Create diff command (TDD GREEN)** - `27ed1c5` (feat) -- includes Task 2 (root.go registration)

**Plan metadata:** (pending)

_Note: Task 2 was merged into Task 1's GREEN commit because test execution required the command to be registered._

## Files Created/Modified
- `cmd/diff.go` - Diff cobra command with runDiff, fetchVersionList, fetchVersionBody, fetchVersionBodies helpers
- `cmd/diff_test.go` - 8 integration tests: DefaultMode, SinceMode, FromToMode, MissingID, SinceWithFromTo, DryRun, EmptySinceRange, BodyUnavailable
- `cmd/root.go` - Added `rootCmd.AddCommand(diffCmd)` registration at line 301

## Decisions Made
- Pre-filter versions by --since cutoff before fetching bodies: avoids unnecessary API calls when most versions are outside the time range. The internal/diff.Compare() still receives already-filtered versions.
- Cobra flag reset in test helper: diffCmd subcommand flags and rootCmd persistent flags (--dry-run) must be reset between test executions because cobra retains parsed values on the global singleton.
- Merged Task 2 into Task 1: diffCmd registration in root.go was required for integration tests to find the diff subcommand.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Registered diffCmd in root.go during Task 1**
- **Found during:** Task 1 (TDD GREEN phase)
- **Issue:** Integration tests could not find the diff subcommand because it was not registered in root.go yet (Task 2 scope)
- **Fix:** Added `rootCmd.AddCommand(diffCmd)` to root.go init() as part of Task 1
- **Files modified:** cmd/root.go
- **Verification:** All tests pass, build succeeds
- **Committed in:** 27ed1c5

**2. [Rule 1 - Bug] Pre-filter versions by --since cutoff before fetching bodies**
- **Found during:** Task 1 (TDD GREEN phase, TestDiff_EmptySinceRange)
- **Issue:** Original implementation fetched all version bodies then let Compare() filter. This caused API errors when the body endpoint had no handler for out-of-range versions.
- **Fix:** Added pre-filtering in fetchSinceVersions() using diff.ParseSince() before calling fetchVersionBodies()
- **Files modified:** cmd/diff.go
- **Verification:** TestDiff_EmptySinceRange passes
- **Committed in:** 27ed1c5

**3. [Rule 1 - Bug] Cobra test isolation via flag reset**
- **Found during:** Task 1 (TDD GREEN phase, sequential test execution)
- **Issue:** Cobra retains parsed flag values on the global command singleton, causing test contamination. --since, --from, --dry-run values from earlier tests leaked into later tests.
- **Fix:** Added flag reset logic in runDiffCommand test helper: ResetFlags() on diff subcommand + Set("dry-run", "false") on rootCmd persistent flags
- **Files modified:** cmd/diff_test.go
- **Verification:** All 8 tests pass when run together
- **Committed in:** 27ed1c5

---

**Total deviations:** 3 auto-fixed (2 bugs, 1 blocking)
**Impact on plan:** All auto-fixes necessary for correctness and test execution. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 14 (version-diff) is complete: internal/diff package + cf diff command
- Ready for Phase 15 (workflow commands) and Phase 16 (CI/CD) which have no dependency on Phase 14
- cf diff produces structured JSON through the standard --jq/--preset/--pretty pipeline

---
*Phase: 14-version-diff*
*Completed: 2026-03-28*
