---
phase: 03-pages-spaces-search-comments-and-labels
plan: "04"
subsystem: testing
tags: [cobra, httptest, unit-testing, pages, spaces, search, comments, labels]

requires:
  - phase: 03-pages-spaces-search-comments-and-labels/03-01
    provides: pagesCmd with fetchPageVersion, doPageUpdate, pages_workflow_update, pages_workflow_get_by_id, pages_workflow_create
  - phase: 03-pages-spaces-search-comments-and-labels/03-02
    provides: spacesCmd with resolveSpaceID, spaces_workflow_list, spaces_workflow_get_by_id
  - phase: 03-pages-spaces-search-comments-and-labels/03-03
    provides: searchCmd with runSearch/fetchV1/searchV1Domain, commentsCmd, labelsCmd with fetchV1WithBody

provides:
  - "cmd/root.go init() wires all five Phase 3 workflow commands via mergeCommand/AddCommand"
  - "cmd/pages_test.go: tests for FetchPageVersion, DoPageUpdate, 409 retry, body-format injection, create validation"
  - "cmd/search_test.go: tests for single-page result, two-page merge, cursor-too-long guard, missing CQL validation"
  - "cmd/comments_test.go: tests for create body/path, validation, list path, delete path+method"
  - "cmd/labels_test.go: tests for add v1 body, validation, remove DELETE+query, remove confirmation output, list v2 path"
  - "cmd/export_test.go: FetchPageVersion, DoPageUpdate, SearchV1Domain, LabelsAddValidation exported for white-box tests"

affects:
  - phase-04-authentication
  - phase-05-attachments

tech-stack:
  added: []
  patterns:
    - "cobra singleton flag state isolation: tests that need clean flag state pass explicit flag values (e.g. --cql '', --label '') rather than omitting flags to avoid cross-test contamination"
    - "httptest server pattern for v1 API: CF_BASE_URL set to srv.URL+/wiki/api/v2 so searchV1Domain extracts domain correctly"
    - "White-box test helpers via export_test.go (package cmd) callable from package cmd_test"

key-files:
  created:
    - cmd/pages_test.go
    - cmd/search_test.go
    - cmd/comments_test.go
    - cmd/labels_test.go
  modified:
    - cmd/root.go
    - cmd/export_test.go

key-decisions:
  - "Cobra singleton flag state: tests using rootCmd.SetArgs must explicitly set all flags to avoid cross-test contamination from prior test runs"
  - "v1 API test clients need CF_BASE_URL=srv.URL+/wiki/api/v2 so searchV1Domain can strip /wiki/ suffix to extract domain"
  - "Labels 'missing label' validation tested via exported LabelsAddValidation helper rather than root command (StringSlice flag accumulates across test runs)"

patterns-established:
  - "Cobra singleton flag state pattern: always pass explicit flag values in SetArgs even for 'empty' cases"
  - "Phase 3 wiring: mergeCommand for pagesCmd/spacesCmd/commentsCmd/labelsCmd; AddCommand for searchCmd (no generated counterpart)"

requirements-completed:
  - PAGE-01
  - PAGE-02
  - PAGE-03
  - PAGE-04
  - PAGE-05
  - SPCE-01
  - SPCE-02
  - SPCE-03
  - SRCH-01
  - SRCH-02
  - SRCH-03
  - CMNT-01
  - CMNT-02
  - CMNT-03
  - LABL-01
  - LABL-02
  - LABL-03

duration: 9min
completed: "2026-03-20"
---

# Phase 03 Plan 04: Wiring and Tests Summary

**Five Phase 3 workflow commands wired into root.go and unit-tested via httptest servers covering helpers, validation, pagination, v1 path construction, and 409 retry logic**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-20T03:14:16Z
- **Completed:** 2026-03-20T03:23:07Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Wired all five Phase 3 commands into `cmd/root.go` init(): `mergeCommand` for pagesCmd/spacesCmd/commentsCmd/labelsCmd and `AddCommand` for searchCmd
- Created four test files with 26 passing tests covering all key behaviors: FetchPageVersion success/404, DoPageUpdate body validation, 409 retry simulation, body-format=storage injection, search single/multi-page merge, cursor-too-long guard, comments create/list/delete paths, labels v1 POST/DELETE path construction with query params
- Extended `cmd/export_test.go` with `FetchPageVersion`, `DoPageUpdate`, `SearchV1Domain`, and `LabelsAddValidation` helpers for white-box testing of unexported functions

## Task Commits

1. **Task 1: Wire workflow commands into cmd/root.go** - `96a422d` (feat)
2. **Task 2: Write unit tests for all five workflow files** - `e04aa83` (test)

## Files Created/Modified

- `cmd/root.go` - Added five mergeCommand/AddCommand calls for Phase 3 workflow commands
- `cmd/export_test.go` - Added FetchPageVersion, DoPageUpdate, SearchV1Domain, LabelsAddValidation export helpers
- `cmd/pages_test.go` - Tests for fetchPageVersion, doPageUpdate, 409 retry, get-by-id body-format, create validation
- `cmd/search_test.go` - Tests for runSearch single page, two-page merge, cursor guard, missing CQL validation
- `cmd/comments_test.go` - Tests for create body/path, create validation, list path, delete path+method
- `cmd/labels_test.go` - Tests for add v1 body, missing page-id validation, remove DELETE+query, remove JSON output, list v2 path

## Decisions Made

- Cobra singleton flag state: when the `rootCmd` singleton is reused across tests via `cmd.RootCommand()`, cobra flag values from prior test runs persist. Fix: always pass explicit flag values (e.g. `--cql ""`, `--label ""`) rather than omitting flags.
- Labels "missing label" validation tested via exported `LabelsAddValidation` helper rather than root command, because `StringSlice` flags accumulate values across cobra singleton reuse, making it impossible to reset to empty via CLI args alone.
- v1 API test clients: `CF_BASE_URL` must be set to `srv.URL + "/wiki/api/v2"` for search and labels tests so `searchV1Domain()` correctly extracts the domain prefix.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Cobra singleton flag state causing cross-test contamination**
- **Found during:** Task 2 (writing search_test.go and labels_test.go)
- **Issue:** `TestRunSearch_MissingCQL` failed when run after `TestRunSearch_CursorTooLong` because the `--cql` flag retained its value from the previous test on the singleton cobra rootCmd. Same issue for `TestLabelsAdd_MissingLabel` with `--label` StringSlice flag.
- **Fix:** For search: pass `--cql ""` explicitly. For labels missing-label: added `LabelsAddValidation` export helper to test validation logic directly without going through cobra.
- **Files modified:** cmd/search_test.go, cmd/labels_test.go, cmd/export_test.go
- **Verification:** `go test ./cmd/... -count=1` passes with all tests green

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Required fix for test correctness. Established cobra singleton state pattern for all future tests using `cmd.RootCommand()`.

## Issues Encountered

- Cobra singleton: the package-level `var rootCmd = &cobra.Command{...}` in cmd/root.go is reused across all tests in the same test binary. Flag values (especially `StringSlice` and scalar flags set by prior test runs) persist unless explicitly reset. Pattern: always pass explicit flag values in `SetArgs`, even for "empty" cases. This applies to all existing tests and was not a regression — existing tests used unique args or `t.Setenv` approaches that avoided the issue.

## Next Phase Readiness

- Phase 3 is fully complete: all five workflow commands (pages, spaces, search, comments, labels) are wired, implemented, and tested.
- `cf pages`, `cf spaces`, `cf search`, `cf comments`, `cf labels` all function as expected.
- Phase 4 (authentication enhancements / OAuth2) can proceed — no blockers from Phase 3.

---
*Phase: 03-pages-spaces-search-comments-and-labels*
*Completed: 2026-03-20*
