---
phase: 13-content-utilities
plan: 03
subsystem: cli
tags: [export, ndjson, tree-walk, body-format, confluence-v2, pagination]

# Dependency graph
requires:
  - phase: 13-01
    provides: setupTemplateEnv test helper, root.go command registration pattern
  - phase: 12-internal-utilities
    provides: jsonutil.NewEncoder for NDJSON streaming, errors.APIError for partial failures
provides:
  - exportCmd with --id, --format, --tree, --depth flags
  - Single-page body extraction with format selection (storage, view, atlas_doc_format)
  - Recursive depth-first tree export as NDJSON with pagination and partial failure handling
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [recursive tree walker with NDJSON streaming, children pagination with cursor following, partial failure logging to stderr]

key-files:
  created: [cmd/export.go, cmd/export_cmd_test.go]
  modified: [cmd/root.go]

key-decisions:
  - "Body field stored as json.RawMessage to preserve full API response body object including format metadata"
  - "Tree export uses depth-first traversal (parent emitted before children)"
  - "Children pagination strips /wiki/api/v2 prefix from _links.next since c.Fetch prepends BaseURL"
  - "Depth 0 means unlimited (default); depth N stops recursion at level N"

patterns-established:
  - "NDJSON tree export: depth-first walk with jsonutil.NewEncoder, partial failures to stderr"
  - "Children pagination: cursor-following loop with /pages/{id}/children endpoint"

requirements-completed: [CONT-06, CONT-07]

# Metrics
duration: 3min
completed: 2026-03-28
---

# Phase 13 Plan 03: Export Command Summary

**Export command with single-page body extraction and recursive tree export as NDJSON, supporting format selection, depth limiting, and partial failure handling**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-28T14:53:45Z
- **Completed:** 2026-03-28T14:57:01Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created export command supporting single-page body extraction with --format flag (storage, view, atlas_doc_format)
- Implemented recursive depth-first tree export as NDJSON with one JSON line per page containing id, title, parentId, depth, body
- Built children pagination following _links.next cursors for trees with >25 children per node
- Added partial failure handling that logs APIError JSON to stderr while continuing NDJSON stream
- Created 6 tests covering single export, view format, validation, tree traversal, depth limiting, and partial failure

## Task Commits

Each task was committed atomically:

1. **Task 1: Create export command with single-page and tree modes** - `5d0523e` (feat)
2. **Task 2: Create export command tests** - `a52e5f7` (test)

## Files Created/Modified
- `cmd/export.go` - Export command with runSingleExport, runTreeExport, walkTree, fetchAllChildren
- `cmd/export_cmd_test.go` - 6 tests: SinglePage, ViewFormat, MissingID, Tree, TreeDepthLimit, TreePartialFailure
- `cmd/root.go` - Registered exportCmd as root subcommand

## Decisions Made
- Body field uses `json.RawMessage` to preserve the full API response body object including format metadata (no double-parsing)
- Tree export uses depth-first traversal so parent is emitted before children (consistent ordering for streaming consumers)
- Children pagination strips `/wiki/api/v2` prefix from `_links.next` URLs since `c.Fetch` prepends `BaseURL`
- Depth 0 means unlimited (default); `--depth N` stops recursion when `currentDepth >= maxDepth`
- Tests use `os.Stdout`/`os.Stderr` pipe capture (matching watch_test.go pattern) since export writes through `c.Stdout`/`c.WriteOutput`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed HTML entity assertion in TestExport_SinglePage**
- **Found during:** Task 2 (export command tests)
- **Issue:** Test asserted `<p>Hello</p>` literal string but Go's JSON encoder escapes HTML entities to `\u003cp\u003e`
- **Fix:** Changed assertion to check for "Hello" content instead of raw HTML tags
- **Files modified:** cmd/export_cmd_test.go
- **Verification:** All 6 export tests pass
- **Committed in:** a52e5f7 (Task 2 commit)

**2. [Rule 1 - Bug] Adapted test pattern for c.Stdout output capture**
- **Found during:** Task 2 (export command tests)
- **Issue:** Plan used `rootCmd.SetOut(buf)` pattern but export command writes to `c.Stdout` (os.Stdout), not cobra's output
- **Fix:** Created `runExportCommand` helper using os.Stdout/os.Stderr pipe redirection (matching watch_test.go pattern)
- **Files modified:** cmd/export_cmd_test.go
- **Verification:** All 6 export tests pass
- **Committed in:** a52e5f7 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bug fixes in test code)
**Impact on plan:** Both fixes necessary for test correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Export command complete and fully tested
- Phase 13 (content utilities) now has all 3 plans complete: built-in templates (01), template management (02), and export (03)

## Self-Check: PASSED

All files verified present. All commits verified in git log.

---
*Phase: 13-content-utilities*
*Completed: 2026-03-28*
