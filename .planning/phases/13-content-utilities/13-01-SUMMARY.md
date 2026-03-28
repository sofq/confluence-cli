---
phase: 13-content-utilities
plan: 01
subsystem: cli
tags: [template, preset, cobra, go-template, regex]

# Dependency graph
requires:
  - phase: 12-internal-utilities
    provides: preset.List(), template.List()/Load()/Render(), jsonutil.MarshalNoEscape()
provides:
  - builtinTemplates map with 6 content templates (blank, meeting-notes, decision, runbook, retrospective, adr)
  - Refactored template.List() returning []TemplateEntry with source attribution
  - template.Show() for full template detail with extracted variables
  - template.Save() for writing templates to user directory
  - template.ExtractVariables() for discovering template variables via regex
  - template.Load() fallback to built-in templates
  - cf preset list command with --jq/--pretty pipeline
affects: [13-02, 13-03]

# Tech tracking
tech-stack:
  added: []
  patterns: [built-in data map with source attribution, regex variable extraction]

key-files:
  created: [internal/template/builtin.go, cmd/preset.go]
  modified: [internal/template/template.go, internal/template/template_test.go, cmd/root.go, cmd/templates_test.go]

key-decisions:
  - "Built-in templates in separate builtin.go file to keep template.go clean"
  - "User templates override built-in for same name (user wins in merge)"
  - "Show() checks user directory first then falls back to builtin (consistent priority)"
  - "Save() rejects overwrite (no --overwrite flag per design decisions)"

patterns-established:
  - "Built-in data maps with source attribution: pattern from preset package replicated for templates"
  - "Regex variable extraction: varPattern captures {{.varName}} from template content"

requirements-completed: [CONT-01, CONT-02, CONT-03]

# Metrics
duration: 4min
completed: 2026-03-28
---

# Phase 13 Plan 01: Built-in Templates and Preset List Summary

**6 built-in templates with source-attributed listing, Show/Save/ExtractVariables API, and cf preset list command through --jq/--pretty pipeline**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-28T14:46:09Z
- **Completed:** 2026-03-28T14:50:01Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Created 6 built-in templates (blank, meeting-notes, decision, runbook, retrospective, adr) in Confluence storage format with {{.variable}} placeholders
- Refactored template.List() to return []TemplateEntry with name and source fields; user templates overlay built-ins
- Added Show(), Save(), ExtractVariables() functions and made Load() fall back to built-in templates
- Created cf preset list command with profile-aware three-tier preset resolution through --jq/--pretty pipeline

## Task Commits

Each task was committed atomically:

1. **Task 1: Create built-in templates and refactor template package API** - `d216b27` (feat)
2. **Task 2: Create preset list command and wire to root** - `64fe64c` (feat)

## Files Created/Modified
- `internal/template/builtin.go` - 6 built-in template definitions as embedded Go map
- `internal/template/template.go` - Refactored List(), new Show/Save/ExtractVariables, Load() builtin fallback
- `internal/template/template_test.go` - Updated existing tests, added 10 new tests for new functionality
- `cmd/preset.go` - presetCmd parent + presetListCmd child with inline config resolution
- `cmd/root.go` - Added "preset" to skipClientCommands, registered presetCmd
- `cmd/templates_test.go` - Updated for new List() return type ([]TemplateEntry)

## Decisions Made
- Built-in templates stored in separate `builtin.go` file rather than inline in `template.go` to keep the main file readable (follows anti-pattern guidance from research)
- User templates override built-in templates with the same name (user wins in merge, source shows "user")
- Show() checks user directory first, then built-in map, consistent with Load() priority
- Save() returns error if file already exists (no overwrite capability per D-09 design decisions)
- Preset list uses `cmd.OutOrStdout()` instead of `os.Stdout` for testability

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated cmd/templates_test.go for new List() return type**
- **Found during:** Task 2 (preset list command)
- **Issue:** Existing TestTemplatesList_WithTemplates and TestTemplatesList_EmptyDir unmarshal output to []string but List() now returns []TemplateEntry
- **Fix:** Updated tests to unmarshal to struct with name/source fields, adjusted expected counts to include built-in templates
- **Files modified:** cmd/templates_test.go
- **Verification:** go test ./cmd/ -count=1 passes
- **Committed in:** 64fe64c (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Expected deviation documented in research as Pitfall 4. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Built-in templates and refactored template API ready for Plan 02 (templates show/create commands)
- Preset list command complete; no further preset work needed in this phase
- template.Show() and template.Save() functions ready for cmd-layer wiring in Plan 02

## Self-Check: PASSED

All files verified present. All commits verified in git log.

---
*Phase: 13-content-utilities*
*Completed: 2026-03-28*
