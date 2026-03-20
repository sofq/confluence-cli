---
phase: 10-output-presets-and-templates
plan: 01
subsystem: cli
tags: [jq, presets, config, output-filtering]

requires:
  - phase: 01-foundation
    provides: Profile struct, config roundtrip, --jq flag
provides:
  - Presets map[string]string field on Profile struct
  - --preset flag with JQ expression resolution
  - Mutual exclusion between --preset and --jq
affects: [10-02, cli-help, documentation]

tech-stack:
  added: []
  patterns: [named-preset-resolution, flag-mutual-exclusion-with-json-errors]

key-files:
  created: [cmd/preset_test.go]
  modified: [internal/config/config.go, cmd/root.go, internal/config/config_test.go]

key-decisions:
  - "Preset resolution happens after rawProfile load, before Client construction -- downstream JQ pipeline unaware of source"
  - "Empty --preset string treated as not-set to avoid interfering with --jq"

patterns-established:
  - "Flag mutual exclusion: check both flags, return validation_error JSON if both set"
  - "Named config lookups: resolve from rawProfile, list available options in error message"

requirements-completed: [PRST-01, PRST-02]

duration: 3min
completed: 2026-03-20
---

# Phase 10 Plan 01: Output Presets Summary

**Named output presets stored in profile config as JQ expressions, applied via --preset flag with conflict detection**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-20T13:46:44Z
- **Completed:** 2026-03-20T13:49:45Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Profile struct supports `presets` map field with backward-compatible JSON serialization
- `--preset <name>` resolves named presets from profile config to JQ filter expressions
- Mutual exclusion with `--jq` enforced with clear JSON error messages
- Missing preset errors list available presets for discoverability

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Presets field to Profile struct and test config roundtrip** - `decb0b6` (feat)
2. **Task 2: Register --preset flag and resolve to JQ expression in PersistentPreRunE** - `ee6b18c` (feat)

## Files Created/Modified
- `internal/config/config.go` - Added Presets map[string]string field to Profile struct
- `internal/config/config_test.go` - Added TestProfilePresets with roundtrip, backward compat, and unmarshal tests
- `cmd/root.go` - Registered --preset flag, added resolution logic and availablePresets helper
- `cmd/preset_test.go` - Tests for preset resolution, not-found error, --preset+--jq conflict, empty preset

## Decisions Made
- Preset resolution happens after rawProfile load but before Client construction, so the downstream JQ pipeline is unaware whether the filter came from --jq or --preset
- Empty --preset string is treated as "not set" to avoid false conflicts with --jq

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Preset infrastructure ready for plan 02 (output templates or additional preset features)
- All existing tests continue to pass

---
*Phase: 10-output-presets-and-templates*
*Completed: 2026-03-20*
