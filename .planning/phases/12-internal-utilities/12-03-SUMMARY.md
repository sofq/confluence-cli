---
phase: 12-internal-utilities
plan: 03
subsystem: api
tags: [json, refactoring, dry, encoding]

# Dependency graph
requires:
  - phase: 12-01
    provides: internal/jsonutil package with MarshalNoEscape and NewEncoder
provides:
  - All SetEscapeHTML(false) call sites consolidated to jsonutil package
  - Zero inline JSON no-escape patterns remaining in codebase
affects: [any future cmd/ or internal/ code writing JSON output]

# Tech tracking
tech-stack:
  added: []
  patterns: [jsonutil.MarshalNoEscape for all non-streaming JSON, jsonutil.NewEncoder for streaming JSON]

key-files:
  created: []
  modified:
    - internal/client/client.go
    - internal/jq/jq.go
    - internal/errors/errors.go
    - cmd/schema_cmd.go
    - cmd/root.go
    - cmd/watch.go
    - cmd/batch.go
    - cmd/version.go
    - cmd/configure.go

key-decisions:
  - "Removed encoding/json import from errors.go since jsonutil fully replaces it"
  - "Removed bytes import from jq.go after deleting marshalNoHTMLEscape"
  - "Removed bytes and encoding/json imports from root.go after both sites converted"

patterns-established:
  - "jsonutil.MarshalNoEscape for all byte-returning JSON serialization without HTML escaping"
  - "jsonutil.NewEncoder for streaming JSON to io.Writer without HTML escaping"

requirements-completed: [UTIL-01]

# Metrics
duration: 5min
completed: 2026-03-28
---

# Phase 12 Plan 03: SetEscapeHTML Refactoring Summary

**Consolidated 12+ inline SetEscapeHTML(false) patterns across 9 files into jsonutil.MarshalNoEscape/NewEncoder calls**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-28T13:57:52Z
- **Completed:** 2026-03-28T14:02:44Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Replaced 5 inline SetEscapeHTML patterns in internal/client/client.go with jsonutil.MarshalNoEscape
- Deleted marshalNoHTMLEscape from internal/jq/jq.go and marshalNoEscape from cmd/schema_cmd.go
- Refactored all 6 cmd/ files to use jsonutil imports instead of inline encoding
- Zero SetEscapeHTML(false) calls remain outside internal/jsonutil/jsonutil.go
- All existing tests pass across the entire project

## Task Commits

Each task was committed atomically:

1. **Task 1: Refactor internal packages (client, jq, errors) to use jsonutil** - `4c8b214` (refactor)
2. **Task 2: Refactor cmd packages (schema_cmd, root, watch, batch, version, configure) to use jsonutil** - `5e7e62f` (refactor)

## Files Created/Modified
- `internal/client/client.go` - 5 encode-then-trim patterns replaced with jsonutil.MarshalNoEscape
- `internal/jq/jq.go` - marshalNoHTMLEscape deleted, 2 call sites use jsonutil.MarshalNoEscape
- `internal/errors/errors.go` - WriteJSON uses jsonutil.NewEncoder, encoding/json import removed
- `cmd/schema_cmd.go` - marshalNoEscape function deleted, 4 call sites use jsonutil.MarshalNoEscape
- `cmd/root.go` - Help uses jsonutil.MarshalNoEscape, Execute uses jsonutil.NewEncoder; bytes and encoding/json imports removed
- `cmd/watch.go` - Streaming encoder uses jsonutil.NewEncoder(c.Stdout)
- `cmd/batch.go` - Result array uses jsonutil.MarshalNoEscape
- `cmd/version.go` - Uses jsonutil.MarshalNoEscape
- `cmd/configure.go` - 3 call sites use jsonutil.MarshalNoEscape

## Decisions Made
- Removed unused imports (encoding/json from errors.go, bytes from jq.go, both from root.go) to keep imports clean
- Used different variable name `out` in jq.go to avoid shadowing the existing `data interface{}` variable at function scope
- Added `\n` to fmt.Fprintf in root.go help function to preserve trailing newline behavior after switching from buf.String() to MarshalNoEscape

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed variable shadowing in jq.go**
- **Found during:** Task 1 (internal/jq/jq.go refactoring)
- **Issue:** Using `data, _ := jsonutil.MarshalNoEscape(results)` at function scope conflicted with existing `var data interface{}` declaration, causing compile error
- **Fix:** Used variable name `out` instead of `data` for both MarshalNoEscape call sites
- **Files modified:** internal/jq/jq.go
- **Verification:** go build ./internal/... passes
- **Committed in:** 4c8b214 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Trivial variable naming adjustment. No scope creep.

## Issues Encountered
None beyond the variable shadowing fix above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 12 (Internal Utilities) is now complete: jsonutil package created (Plan 01), preset built-ins registered (Plan 02), all SetEscapeHTML sites refactored (Plan 03)
- Codebase is fully DRY for JSON no-escape encoding
- Ready for Phases 13/14/15 which can parallelize after Phase 12

## Self-Check: PASSED

- All 9 modified files verified present
- Commit 4c8b214 (Task 1) verified
- Commit 5e7e62f (Task 2) verified

---
*Phase: 12-internal-utilities*
*Completed: 2026-03-28*
