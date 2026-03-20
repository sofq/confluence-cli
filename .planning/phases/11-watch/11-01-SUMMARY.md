---
phase: 11-watch
plan: 01
subsystem: cli
tags: [ndjson, polling, signal-handling, cql, watch]

requires:
  - phase: 04-search
    provides: searchV1Domain() and fetchV1() helpers for v1 API access
provides:
  - "cf watch command for CQL-based content change polling with NDJSON output"
  - "Signal-aware polling loop pattern with clean shutdown"
  - "Client-side timestamp deduplication for date-granularity CQL"
affects: []

tech-stack:
  added: []
  patterns:
    - "signal.NotifyContext for long-running command cancellation"
    - "time.NewTicker + select for interval-based polling loop"
    - "json.NewEncoder for NDJSON streaming to stdout"
    - "--max-polls hidden flag pattern for deterministic testing of polling commands"

key-files:
  created:
    - cmd/watch.go
    - cmd/watch_test.go
  modified:
    - cmd/root.go

key-decisions:
  - "Used --max-polls hidden flag for deterministic test control instead of signal-based test termination"
  - "Merged watchCmd registration into root.go during Task 1 since tests require it"
  - "Seen map pruning threshold set to 48 hours to balance memory vs dedup accuracy"

patterns-established:
  - "Long-running command pattern: signal.NotifyContext + ticker + select loop with shutdown event"
  - "Hidden --max-polls flag for testing polling commands without real timers"

requirements-completed: [WTCH-01, WTCH-02]

duration: 5min
completed: 2026-03-20
---

# Phase 11 Plan 01: Watch Command Summary

**CQL polling watch command with NDJSON change events, client-side timestamp dedup, and signal-aware shutdown**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-20T14:32:47Z
- **Completed:** 2026-03-20T14:37:22Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Watch command polls v1 CQL search API at configurable intervals, emitting NDJSON change events
- Client-side timestamp deduplication handles CQL's date-only lastModified granularity
- Clean shutdown on SIGINT/SIGTERM emits {"type":"shutdown"} and exits code 0
- API errors written to stderr as JSON, polling continues on next interval
- 7 unit tests covering poll/emit, dedup, HTTP errors, shutdown, CQL validation

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing watch tests** - `2a4419b` (test)
2. **Task 1 (GREEN): Watch command implementation** - `a30d65b` (feat)

_Note: Task 2 (root.go registration + full build verification) was completed as part of Task 1 since registration was required for tests to pass._

## Files Created/Modified
- `cmd/watch.go` - Watch command with polling loop, NDJSON emission, signal handling, dedup
- `cmd/watch_test.go` - 7 unit tests for all watch behaviors
- `cmd/root.go` - Added watchCmd registration (Phase 11 comment)

## Decisions Made
- Used --max-polls hidden flag for deterministic test control instead of signal-based termination
- Merged root.go registration into Task 1 commit since test infrastructure requires command availability
- Seen map prunes entries older than 48 hours to prevent unbounded memory growth

## Deviations from Plan

None - plan executed exactly as written. The root.go registration was pulled into Task 1 because tests required it, but all planned work was completed.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Watch command is fully functional and tested
- No new Go dependencies added (stdlib only, per project decision)

---
*Phase: 11-watch*
*Completed: 2026-03-20*

## Self-Check: PASSED
