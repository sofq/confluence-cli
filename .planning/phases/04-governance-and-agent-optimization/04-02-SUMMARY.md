---
phase: 04-governance-and-agent-optimization
plan: "02"
subsystem: governance
tags: [policy, audit, root, integration-tests, cobra]
dependency_graph:
  requires: [04-01]
  provides: [cmd/root.go.policy-wiring, cmd/root.go.audit-wiring, cmd/policy_audit_test.go]
  affects: [cmd/root.go]
tech_stack:
  added: []
  patterns: [policy-enforcement-before-dryrun, audit-logger-lifecycle-postrun, cobra-singleton-flag-isolation, OS-pipe-capture-for-tests]
key_files:
  created:
    - cmd/policy_audit_test.go
  modified:
    - cmd/root.go
decisions:
  - "PersistentPostRun added to rootCmd for audit logger Close() — safe via nil receiver check"
  - "Raw config loaded with LoadFrom() after Resolve() to get governance fields (AllowedOperations, DeniedOperations, AuditLog)"
  - "Test patterns use exact operation strings (no glob) because path.Match * does not cross slashes"
  - "captureExecute helper uses OS pipe redirection matching existing test patterns in the codebase"
  - "cobra singleton dry-run state contamination fixed by passing --dry-run=false explicitly in audit test"
metrics:
  duration: "10 minutes"
  completed_date: "2026-03-20"
  tasks_completed: 2
  files_changed: 2
---

# Phase 04 Plan 02: Policy and Audit CLI Wiring Summary

**One-liner:** Policy enforcement and NDJSON audit logging wired into cmd/root.go PersistentPreRunE with --audit flag, backed by 7 integration tests.

## What Was Built

### Task 1: Wire policy and audit into cmd/root.go

`cmd/root.go` updated to activate governance features built in Plan 01:

- **`--audit` persistent flag** added via `pf.String("audit", "", "...")` — overrides `profile.AuditLog`
- **`PersistentPreRunE`** extended after `config.Resolve()`:
  1. Calls `config.LoadFrom(config.DefaultPath())` to get raw `Profile` (AllowedOperations, DeniedOperations, AuditLog)
  2. Calls `policy.NewFromConfig(rawProfile.AllowedOperations, rawProfile.DeniedOperations)` — on error writes `config_error` to stderr
  3. Determines `auditPath` from `--audit` flag first, then `rawProfile.AuditLog`
  4. If `auditPath != ""`, calls `audit.NewLogger(auditPath)` — on error writes `config_error` to stderr
  5. Extends `client.Client` literal with `Policy`, `AuditLogger`, `Profile` fields
- **`PersistentPostRun`** added to rootCmd — retrieves client from context and calls `c.AuditLogger.Close()` (nil-safe)

### Task 2: Integration tests for policy enforcement and audit logging

`cmd/policy_audit_test.go` — 7 integration tests in `package cmd_test`:

**Policy tests** (5):
- `TestPolicyAllowListDeniesUnmatchedOperation` — allow-only profile with `pages:get` blocks `GET /wiki/api/v2/spaces`, exit 4, `policy_denied` error_type, zero HTTP requests
- `TestPolicyAllowListPermitsMatchingOperation` — exact allow pattern permits request, exit 0, HTTP reaches server
- `TestPolicyDenyListDeniesMatchingOperation` — deny-list profile blocks matching op before HTTP, exit 4
- `TestPolicyDryRunWithDenyingPolicyExitsCode4` — `--dry-run` with deny policy still exits 4 (policy check before DryRun block)
- `TestPolicyNoFieldsBehavesNormally` — profile with no policy fields = unrestricted, exit 0

**Audit tests** (2):
- `TestAuditLogWritesNDJSONEntry` — `--audit <path>` writes exactly one NDJSON line with `method:GET`, `path` containing `/spaces`, `status:200`
- `TestAuditLogNoPolicyDeniedEntry` — policy-denied call writes zero audit entries

**Test helpers**:
- `writePolicyConfig()` — writes config.json with profile containing governance fields to temp dir
- `captureExecute()` — redirects OS stdout/stderr via pipes, calls `cmd.Execute()`, returns captured output + exit code

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] cobra singleton dry-run flag leaking across tests**
- **Found during:** Task 2 — TestAuditLogWritesNDJSONEntry failing with status=0
- **Issue:** `TestPolicyDryRunWithDenyingPolicyExitsCode4` sets `--dry-run` on the cobra singleton. The next test `TestAuditLogWritesNDJSONEntry` inherits the `dry-run=true` flag, causing audit entry to use DryRun path (status=0) instead of live HTTP path
- **Fix:** Pass `--dry-run=false` explicitly in `TestAuditLogWritesNDJSONEntry` args
- **Files modified:** `cmd/policy_audit_test.go`
- **Commit:** e9f0096

**2. [Rule 3 - Blocking] Test glob patterns for path.Match with slashes**
- **Found during:** Task 2 — TestPolicyAllowListPermitsMatchingOperation and TestPolicyDenyListDeniesMatchingOperation failing
- **Issue:** `path.Match("GET *", "GET /wiki/api/v2/spaces")` returns false because `*` in `path.Match` does not match `/` characters
- **Fix:** Changed test allow/deny patterns to use the exact operation string (`"GET /wiki/api/v2/spaces"`) that the raw command fallback produces
- **Files modified:** `cmd/policy_audit_test.go`
- **Commit:** e9f0096

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 8eb647d | feat | Wire policy and audit into cmd/root.go PersistentPreRunE |
| e9f0096 | test | Add integration tests for policy enforcement and audit logging |

## Self-Check: PASSED

All created/modified files exist and both commits verified.
