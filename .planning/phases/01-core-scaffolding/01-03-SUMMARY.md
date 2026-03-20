---
phase: 01-core-scaffolding
plan: 03
subsystem: infra
tags: [cobra, cli, json, confluence-api, http-client]

# Dependency graph
requires:
  - phase: 01-core-scaffolding/01-01
    provides: internal/client, internal/config, internal/errors, internal/jq packages
  - phase: 01-core-scaffolding/01-02
    provides: cmd/generated/stub.go with RegisterAll, AllSchemaOps, AllResources

provides:
  - cmd/root.go: rootCmd, Execute(), PersistentPreRunE with client injection, mergeCommand, RootCommand
  - cmd/version.go: versionCmd with JSON output
  - cmd/schema_cmd.go: schemaCmd with --list/--compact, marshalNoEscape, schemaOutput, compactSchema helpers
  - cmd/configure.go: configureCmd flag-driven profile management, testConnection against /wiki/api/v2/spaces?limit=1
  - cmd/raw.go: rawCmd executing raw Confluence API calls with --body/--query flags
  - Working binary: go build -o cf . produces cf --version and cf schema with valid JSON output

affects:
  - 02-code-generator (will use generated.RegisterAll populated by real operations)
  - 03-content-commands (will add new commands via rootCmd.AddCommand)
  - all future phases (JSON stdout contract established, client injection pattern set)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - PersistentPreRunE client injection: client built from resolved config, stored in context via client.NewContext
    - skipClientCommands map: configure/version/completion/help/schema bypass client injection
    - mergeCommand: replaces generated parent command on rootCmd while preserving generated subcommands
    - marshalNoEscape/schemaOutput: JSON serialization without HTML escaping, with --jq and --pretty support
    - AlreadyWrittenError sentinel: errors written to stderr before returning, caller returns exit code only

key-files:
  created:
    - cmd/root.go
    - cmd/version.go
    - cmd/schema_cmd.go
    - cmd/configure.go
    - cmd/raw.go
  modified: []

key-decisions:
  - "Version variable declared in cmd/root.go (not version.go) to avoid undefined reference across package init order"
  - "schemaOutput uses encoding/json Indent for pretty-print instead of tidwall/pretty (no external dependency needed)"
  - "cf schema compact mode returns empty map {} when no generated ops exist (stub phase; Phase 2 populates AllSchemaOps)"
  - "configure.go testConnection uses /wiki/api/v2/spaces?limit=1 (Confluence v2) not /rest/api/3/myself (Jira)"

patterns-established:
  - "JSON stdout contract: all cmd output uses marshalNoEscape/schemaOutput or client.Do/WriteOutput; help/errors to stderr"
  - "Phase 4 exclusion: no audit/policy/preset/Operation/AuditLogger/Profile fields anywhere in Phase 1 commands"

requirements-completed: [INFRA-01, INFRA-03, INFRA-04, INFRA-09, INFRA-10, INFRA-12, INFRA-13]

# Metrics
duration: 5min
completed: 2026-03-20
---

# Phase 01 Plan 03: Core CLI Commands Summary

**Five Cobra command files wiring client/config/errors packages into a working `cf` binary — `cf --version`, `cf schema`, `cf configure`, and `cf raw` all operational with JSON-only stdout.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-20T00:55:00Z
- **Completed:** 2026-03-20T00:59:28Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- `go build -o cf .` succeeds with all five cmd/*.go files wired together
- `cf --version` outputs `{"version":"dev"}` and `cf schema` outputs `{}` (empty map, stub phase)
- `cf configure` saves profiles to config file using `/wiki/api/v2/spaces?limit=1` for connection test
- `cf raw` delegates to `client.Do()` with method validation, --body/@file/stdin handling, and --query params
- Zero jira-cli imports, zero JR_ references, zero Phase 4 fields (audit/policy/preset) in any cmd file

## Task Commits

Each task was committed atomically:

1. **Task 1: root.go, version.go, schema_cmd.go** - `fc65694` (feat)
2. **Task 2: configure.go, raw.go, final build verification** - `a625468` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `cmd/root.go` - rootCmd with PersistentPreRunE client injection, Version variable, Execute/RootCommand exports, mergeCommand helper
- `cmd/version.go` - versionCmd outputting `{"version":"dev"}` via marshalNoEscape/schemaOutput
- `cmd/schema_cmd.go` - schemaCmd with --list/--compact flags; marshalNoEscape, schemaOutput, compactSchema helpers
- `cmd/configure.go` - flag-driven profile save/delete, testConnection against Confluence /wiki/api/v2/spaces?limit=1
- `cmd/raw.go` - raw API calls with method validation, body/file/stdin resolution, --query parsing

## Decisions Made

- `Version` declared in `cmd/root.go` (not `version.go`) — Go package-level init has no ordering guarantee across files; keeping Version in root.go avoids any undefined-reference edge case during `go build`
- `schemaOutput` uses `encoding/json Indent` for pretty-print — avoids adding `tidwall/pretty` dependency; consistent with client.go WriteOutput pattern established in Plan 02
- Phase 4 boundary enforced: no `AuditLogger`, `Policy`, `Operation`, `Profile` fields referenced anywhere

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added `var Version = "dev"` to cmd/root.go**
- **Found during:** Task 1 (initial build after creating root.go, version.go, schema_cmd.go)
- **Issue:** The original stub `cmd/root.go` had `Version` declared; replacing root.go with the full implementation removed the declaration, causing `undefined: Version` compile error
- **Fix:** Added `var Version = "dev"` to root.go (ldflags target) rather than version.go, matching the plan's ldflags comment in version.go
- **Files modified:** cmd/root.go
- **Verification:** `go build ./...` exits 0 after fix
- **Committed in:** fc65694 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug)
**Impact on plan:** Necessary to restore the variable that the stub had provided. No scope creep.

## Issues Encountered

None beyond the Version variable auto-fix above.

## Next Phase Readiness

- Phase 02 (code generator) can now call `generated.RegisterAll(rootCmd)` with real operations — the hook is already in root.go init()
- All client injection infrastructure is in place for generated commands to use `client.FromContext`
- Binary name `cf` collision with Cloud Foundry CLI remains documented in STATE.md blockers — no code change required

---
*Phase: 01-core-scaffolding*
*Completed: 2026-03-20*
