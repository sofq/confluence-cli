---
phase: 03-pages-spaces-search-comments-and-labels
plan: "02"
subsystem: api
tags: [cobra, confluence-v2, spaces, key-resolution]

requires:
  - phase: 02-code-generation-pipeline
    provides: "cmd/generated/spaces.go with all generated space subcommands"

provides:
  - "cmd/spaces.go: spacesCmd parent, resolveSpaceID helper, spaces_workflow_list and spaces_workflow_get_by_id"
  - "resolveSpaceID: package-level helper in cmd package, usable by pages and other resources needing space key resolution"

affects:
  - 03-pages-spaces-search-comments-and-labels (Plan 04 wires spacesCmd via mergeCommand)

tech-stack:
  added: []
  patterns:
    - "resolveSpaceID pattern: numeric pass-through + alpha key API lookup via GET /spaces?keys=<KEY>"
    - "Workflow command overrides generated parent, preserving generated subcommands via Plan 04 mergeCommand"
    - "export_test.go exposes unexported package helpers for external cmd_test package"

key-files:
  created:
    - cmd/spaces.go
    - cmd/spaces_test.go
    - cmd/export_test.go
  modified: []

key-decisions:
  - "resolveSpaceID uses strconv.ParseInt to determine numeric vs alpha before calling API — avoids unnecessary round-trip for numeric IDs"
  - "spaces_workflow_list (Use: get) adds --key flag on top of generated spaces_get flags; does not shadow the generated command (Plan 04 replaces it via mergeCommand)"
  - "spaces_workflow_get_by_id (Use: get-by-id) replaces the generated command with identical Use string, adding transparent key resolution to the --id flag"
  - "export_test.go in package cmd (not cmd_test) exposes resolveSpaceID as ResolveSpaceID for white-box testing — follows existing project pattern"
  - "No calls to mergeCommand or rootCmd.AddCommand in spaces.go init() — Plan 04 handles all root wiring"

patterns-established:
  - "Workflow override: define var <resource>Cmd + init() with AddCommand only; no rootCmd wiring"
  - "Key resolution pattern: strconv.ParseInt check first, then Fetch /resource?keys=<KEY>, extract results[0].id"

requirements-completed:
  - SPCE-01
  - SPCE-02
  - SPCE-03

duration: 2min
completed: 2026-03-20
---

# Phase 03 Plan 02: Spaces Workflow Commands Summary

**Space key-to-numeric-ID resolution helper (resolveSpaceID) plus workflow overrides for `cf spaces get` (list with --key) and `cf spaces get-by-id` (transparent key/ID via --id)**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-20T03:07:18Z
- **Completed:** 2026-03-20T03:09:20Z
- **Tasks:** 2 (implemented together in one atomic file)
- **Files modified:** 3

## Accomplishments
- Implemented `resolveSpaceID` as a package-level helper in the `cmd` package accessible to all future workflow commands (pages, search, etc.)
- Added `spaces_workflow_list` (Use: "get") with optional `--key` flag — resolves key then fetches single space, or lists all spaces with auto-pagination when no key is given
- Added `spaces_workflow_get_by_id` (Use: "get-by-id") that accepts either numeric IDs or alpha keys transparently via the `--id` flag
- Full TDD cycle: failing tests written first, implementation driven to green, verified with `go build`, `go vet`, and `go test`

## Task Commits

Each task was committed atomically:

1. **Task 1+2: resolveSpaceID, spacesCmd, workflow subcommands** - `e59b158` (feat)

**Plan metadata:** (final docs commit below)

## Files Created/Modified
- `cmd/spaces.go` - spacesCmd parent, resolveSpaceID helper, spaces_workflow_list (get), spaces_workflow_get_by_id
- `cmd/spaces_test.go` - External package tests for resolveSpaceID (numeric pass-through, alpha key resolution, not-found), and smoke tests for list/get-by-id commands
- `cmd/export_test.go` - Exposes resolveSpaceID as ResolveSpaceID for cmd_test package

## Decisions Made
- Used `strconv.ParseInt` for numeric check before making any API call — no unnecessary round trips for numeric IDs
- `spaces_workflow_get_by_id` writes validation error to `c.Stderr` (not `os.Stderr`) for testability
- `resolveSpaceID` writes not-found errors to `os.Stderr` (matching the plan spec) since it doesn't have a `c.Stderr` reference in the not-found path
- Tests merged into one file (no separate RED/GREEN commits) since both tasks target the same file and the test suite needed to compile as a unit

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed strings.Builder used as io.ReadFrom in test**
- **Found during:** Task 1 (TDD RED phase test writing)
- **Issue:** `strings.Builder` does not implement `ReadFrom`; test file used it incorrectly
- **Fix:** Changed `var buf strings.Builder; buf.ReadFrom(r)` to `var buf bytes.Buffer; _, _ = buf.ReadFrom(r)`
- **Files modified:** cmd/spaces_test.go
- **Verification:** `go test ./cmd/...` passes
- **Committed in:** e59b158 (task commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug in test helper)
**Impact on plan:** Minor test correctness fix, no scope creep.

## Issues Encountered
- The `export_test.go` approach is required because `resolveSpaceID` is unexported and the project convention uses external `cmd_test` package for all tests. This pattern is documented for future workflow commands.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `resolveSpaceID` is accessible from any file in the `cmd` package — pages and other workflow commands can call it
- Plan 03 (pages workflow) can reference `resolveSpaceID` for `--space` key resolution
- Plan 04 (root wiring) must call `mergeCommand(rootCmd, spacesCmd)` to make `cf spaces get` and `cf spaces get-by-id` live

---
*Phase: 03-pages-spaces-search-comments-and-labels*
*Completed: 2026-03-20*
