---
phase: 04-governance-and-agent-optimization
plan: "01"
subsystem: governance
tags: [policy, audit, config, client]
dependency_graph:
  requires: []
  provides: [internal/policy, internal/audit, config.Profile.governance-fields, client.Client.governance-fields]
  affects: [internal/client/client.go, internal/config/config.go]
tech_stack:
  added: [internal/policy, internal/audit]
  patterns: [nil-safe-receiver, glob-matching, NDJSON-append, mutex-concurrent-log, TDD-red-green]
key_files:
  created:
    - internal/policy/policy.go
    - internal/policy/policy_test.go
    - internal/audit/audit.go
    - internal/audit/audit_test.go
  modified:
    - internal/config/config.go
    - internal/client/client.go
decisions:
  - "Policy uses path.Match standard library glob — no external deps"
  - "nil *Policy and nil *Logger are always safe no-ops via nil receiver checks"
  - "Policy.Check called BEFORE DryRun block in Do() so dry-run also enforces policy (GOVN-02)"
  - "Audit entry written in doOnce() for live requests and in Do() DryRun block with DryRun=true flag"
  - "operationName in Do() = c.Operation if set, else '<METHOD> <path>' fallback"
  - "doOnce() signature extended with operationName parameter to pass through for audit entries"
metrics:
  duration: "5 minutes"
  completed_date: "2026-03-20"
  tasks_completed: 2
  files_changed: 6
---

# Phase 04 Plan 01: Policy and Audit Foundations Summary

**One-liner:** Operation-level policy enforcement (allow/deny glob) and NDJSON audit logging wired into Client.Do() and Client.doOnce().

## What Was Built

### Task 1: internal/policy package

`internal/policy/policy.go` implements `Policy`, `NewFromConfig`, `Check`, and `DeniedError`:

- `NewFromConfig(nil, nil)` returns `(nil, nil)` — unrestricted mode
- `NewFromConfig(allowed, nil)` builds an allow-list policy using `path.Match` glob patterns
- `NewFromConfig(nil, denied)` builds a deny-list policy
- `NewFromConfig(allowed, denied)` returns an error — conflict
- Invalid glob patterns return an error immediately
- `(*Policy).Check(op)` is nil-safe; nil Policy allows everything
- `DeniedError` carries `Operation` and `Reason` fields

11 unit tests in `internal/policy/policy_test.go` (external test package), all passing.

### Task 2: internal/audit package + Config and Client extensions

`internal/audit/audit.go` implements `Logger`, `Entry`, `NewLogger`, `DefaultPath`, `Log`, `Close`:

- `NewLogger` creates parent directories and opens file for append with 0o600 perms
- `Log()` stamps `ts` as RFC3339 UTC, writes one JSON line ending in `\n`, mutex-protected
- `nil` Logger `Log()` and `Close()` are no-ops
- `DefaultPath()` returns `<UserConfigDir>/cf/audit.log` (not `jr`)
- `Close()` sets `file=nil` so subsequent `Log()` calls are no-ops

7 unit tests including concurrent-write safety test.

`internal/config/config.go` — `Profile` extended with three omitempty fields:
- `AllowedOperations []string json:"allowed_operations,omitempty"`
- `DeniedOperations []string json:"denied_operations,omitempty"`
- `AuditLog string json:"audit_log,omitempty"`

`internal/client/client.go` — `Client` extended with four new fields:
- `Policy *policy.Policy` — nil = unrestricted
- `AuditLogger *audit.Logger` — nil = no logging
- `Profile string` — active profile name for audit entries
- `Operation string` — operation name for audit entries, set by batch

`Do()` now:
1. Derives `operationName` from `c.Operation` or falls back to `"<method> <path>"`
2. Calls `c.Policy.Check(operationName)` — BEFORE the DryRun block (enforces policy even in dry-run)
3. On denial, writes `APIError{error_type:"policy_denied"}` to Stderr and returns `ExitValidation` (4)
4. In the DryRun block: calls `c.AuditLogger.Log(...)` with `DryRun: true`

`doOnce()` now:
1. Accepts `operationName` parameter
2. Captures `statusCode` before body read
3. Calls `c.AuditLogger.Log(...)` after response on both success and HTTP error paths

## Test Results

```
ok  github.com/sofq/confluence-cli/internal/policy   11 tests
ok  github.com/sofq/confluence-cli/internal/audit     7 tests
ok  github.com/sofq/confluence-cli/internal/config    8 tests
ok  github.com/sofq/confluence-cli/internal/client   10 tests
go build ./... — clean
go vet ./internal/... — clean
```

## Deviations from Plan

None — plan executed exactly as written.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| bc472d3 | test | Add failing tests for policy package (RED) |
| 1dcd5ba | feat | Implement internal/policy package |
| 5766b04 | test | Add failing tests for audit package (RED) |
| f928ca2 | feat | Implement audit, extend config.Profile and client.Client |

## Self-Check: PASSED

All created files exist. All 4 commits verified.
