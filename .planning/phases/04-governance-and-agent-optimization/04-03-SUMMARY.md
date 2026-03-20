---
phase: 04-governance-and-agent-optimization
plan: "03"
subsystem: batch
tags: [batch, cobra, generated, policy, jq, json]

dependency_graph:
  requires:
    - phase: 04-01
      provides: "Policy struct with Check() method; AuditLogger; Client.Policy/AuditLogger/Profile/Operation fields"
  provides:
    - "cmd/batch.go: BatchOp, BatchResult types, runBatch, executeBatchOp, batchCmd registered on rootCmd"
    - "cmd/batch_test.go: 10 test cases covering all BTCH-01/02/03 scenarios"
  affects:
    - cmd (exports ExecuteBatchOps via export_test.go)

tech-stack:
  added: []
  patterns:
    - "Per-op client clone: copy Client struct fields into new Client with captured stdout/stderr builders"
    - "executeBatchOp takes context.Context (not *cobra.Command) for testability"
    - "Cobra singleton flag contamination: pass --jq '' explicitly in tests after jq-setting tests"
    - "ExecuteBatchOps export in export_test.go enables direct policy testing without full CLI"

key-files:
  created:
    - cmd/batch.go
    - cmd/batch_test.go
  modified:
    - cmd/export_test.go

key-decisions:
  - "executeBatchOp takes context.Context directly (not *cobra.Command) so it can be tested without Cobra overhead"
  - "encoding/json.Indent used for batch pretty-print (no tidwall/pretty — consistent with earlier decision)"
  - "Explicit policy check in executeBatchOp (before client clone) produces clean BatchResult error format; implicit check in Do() acts as safety net"
  - "ExitBatchOps exported via export_test.go to enable direct policy testing without wiring through PersistentPreRunE"
  - "unknown command returns ExitError (1) not ExitValidation (4) — client errors vs validation errors distinction"

requirements-completed: [BTCH-01, BTCH-02, BTCH-03]

duration: 6min
completed: 2026-03-20
---

# Phase 04 Plan 03: Batch Command Summary

**`cf batch` command dispatching JSON op arrays to generated schema ops with per-op exit codes, policy enforcement, and JQ filtering of the output array**

## Performance

- **Duration:** 6 minutes
- **Started:** 2026-03-20T03:45:53Z
- **Completed:** 2026-03-20T03:51:50Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- `cmd/batch.go` — full `cf batch` implementation: reads `--input` file or stdin, enforces `--max-batch`, dispatches each op through generated schema ops, returns JSON array with per-op exit codes
- Per-op client cloning with captured stdout/stderr builders for clean result capture
- Policy denial handled at batch level (explicit check before HTTP) producing `BatchResult.Error` with `policy_denied` type
- 10 comprehensive tests covering all plan-specified scenarios (partial failure, policy deny, unknown command, jq filter, max-batch exceeded, empty array, missing path param)

## Task Commits

1. **Task 1: cmd/batch.go implementation** - `00e3734` (feat)
2. **Task 2: Batch tests** - `b710c2f` (test, also refactors executeBatchOp signature + updates export_test.go)

## Files Created/Modified
- `cmd/batch.go` — BatchOp, BatchResult, batchCmd, runBatch, executeBatchOp, helpers
- `cmd/batch_test.go` — 10 TestBatch_* test cases
- `cmd/export_test.go` — added ExecuteBatchOps export for direct testing

## Decisions Made
- Refactored `executeBatchOp` to take `context.Context` instead of `*cobra.Command` — cleaner signature, enables direct unit testing without Cobra
- `ExecuteBatchOps` exported via `export_test.go` (not a public API) — lets `TestBatch_PolicyDeny` verify policy enforcement without requiring 04-02's PersistentPreRunE wiring to load the config-based policy
- Cobra singleton flag contamination fix: `TestBatch_MultiOpSuccess` passes `--jq ""` to reset flag state set by `TestBatch_JQFilter` — consistent with Phase 03 decision pattern

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Refactored executeBatchOp to take context.Context**
- **Found during:** Task 2 (writing tests)
- **Issue:** Original signature `executeBatchOp(cmd *cobra.Command, ...)` required a real Cobra command to call `cmd.Context()`, making direct unit testing impossible (policy test needed a client with `Policy` set, not reachable via CLI env vars since 04-02 wasn't yet run)
- **Fix:** Changed signature to `executeBatchOp(ctx context.Context, ...)`, extracted `ctx := cmd.Context()` in `runBatch` before the loop
- **Files modified:** cmd/batch.go, cmd/export_test.go
- **Verification:** All 10 tests pass, build clean
- **Committed in:** b710c2f (Task 2 commit)

**2. [Rule 1 - Bug] Fixed cobra singleton jq flag contamination**
- **Found during:** Task 2 test run
- **Issue:** `TestBatch_MultiOpSuccess` failed because `TestBatch_JQFilter` set `--jq .[0].exit_code` on the singleton rootCmd; next test inherited the filter, output was `0` instead of a JSON array
- **Fix:** Added `--jq ""` to `TestBatch_MultiOpSuccess` args to explicitly reset the flag
- **Files modified:** cmd/batch_test.go
- **Verification:** All 10 tests pass in sequence
- **Committed in:** b710c2f (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 - Bug)
**Impact on plan:** Both fixes necessary for testability and correctness. No scope creep.

## Issues Encountered
None beyond the two auto-fixed bugs above.

## Next Phase Readiness
- `cf batch` fully implemented and tested — BTCH-01, BTCH-02, BTCH-03 satisfied
- Policy enforcement in batch is already wired (Plan 01 client fields + Plan 03 explicit check)
- Phase 04 Plan 02 (policy wiring in PersistentPreRunE) was already completed before this plan ran — no ordering issue

## Self-Check: PASSED
