# Phase 4: Governance and Agent Optimization - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Add operation policy (allow/deny per profile), NDJSON audit logging, and batch command execution. These features make `cf` safe and efficient for production AI agent deployments — agents can be scoped to specific operations, all actions are logged, and multi-step workflows execute in a single process invocation.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

All implementation choices are at Claude's discretion — infrastructure phase. Mirror the reference implementation patterns:
- `internal/policy/policy.go` from `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/policy/policy.go`
- `internal/audit/audit.go` from `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/audit/audit.go`
- `cmd/batch.go` from `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/batch.go`

Key adaptations:
- Policy enforced pre-request in client.Do(), even during --dry-run
- Audit logger integrated into client as optional field
- Batch dispatches to existing command tree by name
- Config profiles extended with `allowed_operations`, `denied_operations`, `audit_log` fields

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/client/client.go` — Client struct needs Policy and AuditLogger fields added
- `internal/config/config.go` — Profile struct needs policy and audit config fields
- `cmd/root.go` — PersistentPreRunE needs policy/audit initialization

### Integration Points
- Policy check goes in client.Do() before HTTP request
- Audit log entry goes in client.Do() after HTTP response
- Batch command uses rootCmd.Find() to dispatch sub-commands
- Config profile JSON extended with new fields

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None.

</deferred>
