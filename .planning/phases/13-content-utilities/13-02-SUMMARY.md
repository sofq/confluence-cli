---
phase: 13-content-utilities
plan: 02
subsystem: cli
tags: [template, cobra, json, xhtml, api-client]

# Dependency graph
requires:
  - phase: 13-content-utilities-01
    provides: template.Show(), template.Save(), template.List() with TemplateEntry, ExtractVariables(), builtinTemplates map
provides:
  - cf templates show <name> command outputting full template JSON with variables
  - cf templates create --from-page --name command for page-to-template creation
  - Refactored cf templates list with jsonutil.MarshalNoEscape and --jq/--pretty pipeline
affects: [13-03]

# Tech tracking
tech-stack:
  added: []
  patterns: [manual client construction for skipClientCommands subcommands needing API access]

key-files:
  created: []
  modified: [cmd/templates.go, cmd/templates_test.go]

key-decisions:
  - "Manual client construction in templates create rather than removing templates from skipClientCommands"
  - "Explicit --name flag (empty string default) for create to avoid cobra flag state leaking across tests"

patterns-established:
  - "Manual client construction pattern: when a subcommand under skipClientCommands needs API access, resolve config and build client inline"

requirements-completed: [CONT-04, CONT-05]

# Metrics
duration: 3min
completed: 2026-03-28
---

# Phase 13 Plan 02: Templates Show and Create Commands Summary

**Templates show with full JSON output (variables, source, XHTML body), create --from-page for page-to-template conversion, and list refactored with MarshalNoEscape pipeline**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-28T14:53:36Z
- **Completed:** 2026-03-28T14:57:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added templates show command with ExactArgs(1) returning full template JSON including name, title, body (unescaped XHTML), source attribution, and extracted variables array
- Added templates create command with --from-page and --name flags that fetches page storage body via API and saves as user template
- Refactored templates list to use jsonutil.MarshalNoEscape with --jq/--pretty pipeline matching preset list pattern
- Added 5 new tests covering show (builtin, user, not-found), create (from-page, missing-name)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add templates show and create commands, refactor templates list** - `918e4d6` (feat)
2. **Task 2: Update templates tests for refactored output format** - `2084f5c` (test)

## Files Created/Modified
- `cmd/templates.go` - Added templatesShowCmd, templatesCreateCmd, refactored templates_list, updated init() with new subcommands
- `cmd/templates_test.go` - Added TestTemplatesShow_Builtin, TestTemplatesShow_UserTemplate, TestTemplatesShow_NotFound, TestTemplatesCreate_FromPage, TestTemplatesCreate_MissingName

## Decisions Made
- Kept "templates" in skipClientCommands and used manual client construction in create command (config.Resolve + client.Client inline) rather than removing templates from skip list -- preserves list/show working without config
- Used explicit `--name ""` in MissingName test to avoid cobra flag state leaking from prior test runs (global command state persists across tests)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All template management commands complete (list, show, create)
- Ready for Plan 03 (export command or remaining content utilities)
- template.Load() builtin fallback working for both show and resolve paths

## Self-Check: PASSED

All files verified present. All commits verified in git log.
