---
phase: 16-schema-gendocs
plan: 01
subsystem: api
tags: [schema, batch, agent-discovery, cobra, json]

requires:
  - phase: 13-content-utilities
    provides: export, preset, templates commands
  - phase: 14-version-diff
    provides: diff command
  - phase: 15-workflow-commands
    provides: workflow move/copy/publish/comment/restrict/archive commands
provides:
  - Schema op definitions for 11 hand-written operations (diff, 6 workflow, export, preset list, templates show/create)
  - Aggregated allOps in schema_cmd.go and batch.go for agent discovery
  - Tests verifying hand-written ops appear in schema output
affects: [16-02, batch, schema]

tech-stack:
  added: []
  patterns: [per-resource *_schema.go files exporting []SchemaOp, allOps aggregation via append]

key-files:
  created: [cmd/diff_schema.go, cmd/workflow_schema.go, cmd/export_schema.go, cmd/preset_schema.go, cmd/templates_schema.go]
  modified: [cmd/schema_cmd.go, cmd/batch.go, cmd/schema_cmd_test.go]

key-decisions:
  - "Per-resource schema files (diff_schema.go, workflow_schema.go, etc.) following jr pattern for separation of concerns"
  - "Flag types match init() declarations exactly: Int flags as 'integer', Bool flags as 'boolean'"
  - "Explicit --compact=false in tests to handle Cobra singleton flag state persistence between test runs"

patterns-established:
  - "Hand-written schema pattern: one *_schema.go file per resource, function returns []generated.SchemaOp"
  - "Aggregation pattern: append(*SchemaOps()...) after generated.AllSchemaOps() in both schema_cmd.go and batch.go"

requirements-completed: [SCHM-01, SCHM-02]

duration: 3min
completed: 2026-03-28
---

# Phase 16 Plan 01: Schema Registration Summary

**Registered 11 hand-written operations (diff, 6 workflow, export, preset, templates) in schema system for agent discovery and batch execution**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-28T16:42:29Z
- **Completed:** 2026-03-28T16:45:57Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Created 5 schema op files covering all Phase 13-15 hand-written commands (11 operations total)
- Updated schema_cmd.go and batch.go to aggregate hand-written ops alongside generated ones
- Added 3 tests verifying hand-written resources appear in --list, --compact, and resource-specific schema output

## Task Commits

Each task was committed atomically:

1. **Task 1: Create five *_schema.go files with hand-written schema ops** - `7a90502` (feat)
2. **Task 2: Update schema_cmd.go and batch.go to aggregate hand-written ops + update tests** - `313f6ed` (feat)

## Files Created/Modified
- `cmd/diff_schema.go` - DiffSchemaOps() returning 1 op with id, since, from (integer), to (integer) flags
- `cmd/workflow_schema.go` - WorkflowSchemaOps() returning 6 ops (move, copy, publish, comment, restrict, archive)
- `cmd/export_schema.go` - ExportSchemaOps() returning 1 op with id, format, tree (boolean), depth (integer) flags
- `cmd/preset_schema.go` - PresetSchemaOps() returning 1 op with empty flags
- `cmd/templates_schema.go` - TemplatesSchemaOps() returning 2 ops (show, create)
- `cmd/schema_cmd.go` - Added 5 append calls to aggregate hand-written ops into allOps
- `cmd/batch.go` - Same 5 append calls for batch opMap resolution
- `cmd/schema_cmd_test.go` - Added TestSchemaIncludesHandWrittenOps, TestSchemaWorkflowListsSixVerbs, TestSchemaCompactIncludesHandWritten

## Decisions Made
- Per-resource schema files following jr pattern (one file per resource, function returns []generated.SchemaOp)
- Flag types match init() declarations exactly: Int flags as "integer", Bool flags as "boolean", String flags as "string"
- Added explicit --compact=false in test args to handle Cobra singleton flag state persistence between test runs

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Cobra flag state leaking between schema tests**
- **Found during:** Task 2 (test creation)
- **Issue:** TestSchemaCompactReturnsJSONObject sets --compact flag on singleton command; subsequent tests inherit the flag, causing --list to return compact JSON object instead of string array
- **Fix:** Added explicit --compact=false and --list=false in SetArgs for new tests
- **Files modified:** cmd/schema_cmd_test.go
- **Verification:** All 7 schema tests pass when run together
- **Committed in:** 313f6ed (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary fix for test correctness. No scope creep.

## Issues Encountered
None beyond the Cobra flag state issue documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 11 hand-written operations now discoverable via `cf schema` and resolvable via `cf batch`
- Ready for Phase 16 Plan 02 (documentation generation)

## Self-Check: PASSED

All 9 files verified present. Both task commits (7a90502, 313f6ed) confirmed in git log.

---
*Phase: 16-schema-gendocs*
*Completed: 2026-03-28*
