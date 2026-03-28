# Phase 15: Workflow Commands - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Dedicated `cf workflow` subcommands for content lifecycle operations: move, copy, publish, comment, restrict, and archive. Each subcommand follows the established hand-written command pattern (flag parsing, client creation, API call, JSON output). Uses v2 API where available, v1 content API for operations without v2 equivalents (copy, restrict). Mirrors jr's `workflow` parent command structure.

</domain>

<decisions>
## Implementation Decisions

### API strategy per subcommand
- **D-01:** `workflow move --id <pageId> --target-id <parentId>` — v2 `PUT /pages/{id}` with updated `parentId` field. Optional `--space-id` for cross-space moves. If API returns async task, poll for completion
- **D-02:** `workflow copy --id <pageId> --target-id <parentId>` — v1 `POST /wiki/rest/api/content/{id}/copy` (no v2 copy endpoint). Uses `searchV1Domain` pattern for v1 base URL construction
- **D-03:** `workflow publish --id <pageId>` — v2 `PUT /pages/{id}` updating `status` from "draft" to "current". Requires page title and version number bump (standard update semantics)
- **D-04:** `workflow comment --id <pageId> --body "text"` — v2 `POST /pages/{id}/footer-comments` reusing the existing footer comments endpoint pattern from `cmd/comments.go`. Plain text input auto-wrapped in `<p>` storage format tags
- **D-05:** `workflow restrict --id <pageId>` — v1 restrictions API (`/wiki/rest/api/content/{id}/restriction`). GET for viewing, PUT for adding, DELETE for removing. Uses `searchV1Domain` for v1 base URL
- **D-06:** `workflow archive --id <pageId>` — v2 `POST /pages/archive` bulk archive endpoint with single-page payload `{"pages": [{"id": "..."}]}`

### Async operation handling
- **D-07:** Move and copy operations block and poll by default until completion. Poll interval: 1 second. Default timeout: 60 seconds
- **D-08:** `--no-wait` flag on move and copy returns the operation/task response immediately without polling — agents can poll separately if needed
- **D-09:** `--timeout <duration>` flag overrides the default 60s timeout for async operations. Uses `duration.Parse()` from Phase 12

### Comment convenience model
- **D-10:** `workflow comment` takes `--body` as plain text string, wraps in `<p>...</p>` storage format tags automatically. No XHTML parsing or complex conversion — simple paragraph wrapping
- **D-11:** This is a convenience wrapper over the existing footer comments API. Agents needing full control (inline comments, rich formatting) use `cf comments create` directly

### Restriction management
- **D-12:** No flags = view mode: `workflow restrict --id <pageId>` GETs and displays current restrictions as JSON
- **D-13:** `--add` flag = add restriction: `--add --operation read|update --user <accountId>` or `--group <groupName>`
- **D-14:** `--remove` flag = remove restriction: `--remove --operation read|update --user <accountId>` or `--group <groupName>`
- **D-15:** `--operation` supports `read` and `update` (the two Confluence restriction operations)
- **D-16:** Supports both `--user` (accountId) and `--group` (group name) identifiers for restrictions

### Copy flags
- **D-17:** `--copy-attachments` boolean (default false) — include attachments in copy
- **D-18:** `--copy-labels` boolean (default false) — include labels in copy
- **D-19:** `--copy-permissions` boolean (default false) — include permissions in copy
- **D-20:** `--title` string — title for the copied page (v1 copy API uses `destination.value` + `name` fields)
- **D-21:** `--target-id` string (required) — destination parent page ID for copy

### Command structure
- **D-22:** `workflowCmd` parent command with Use: "workflow", Short: "Content lifecycle operations". Registered to root via `rootCmd.AddCommand(workflowCmd)`
- **D-23:** Each subcommand (move, copy, publish, comment, restrict, archive) is a child of workflowCmd
- **D-24:** All subcommands require `--id` flag (page ID) — consistent with diff, export, and other page-targeting commands

### Claude's Discretion
- Exact v1 copy API request body structure (validate against Confluence docs during research)
- Whether move actually needs async polling or if v2 PUT is synchronous (validate during research)
- Exact v1 restrictions API request/response shape (validate during research)
- Whether archive v2 endpoint requires any additional fields beyond page ID
- Test case organization and helper patterns
- Error message wording for validation failures
- Whether `--space-id` on move is a separate flag or inferred from target

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### jr reference implementation (architecture mirror)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/workflow.go` — Workflow parent + subcommand pattern: transition, assign, comment, move, create, link. Flag design, init() registration, RunE handlers
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/workflow_test.go` — Test patterns for workflow commands

### Existing cf commands (pattern reference)
- `cmd/export.go` — Recent hand-written command: flag validation, API calls, NDJSON output, error patterns
- `cmd/diff.go` — Recent hand-written command: multiple API calls, structured output, flag parsing
- `cmd/comments.go` — Footer comment create/list pattern, v2 footer-comments endpoint, storage format body
- `cmd/pages.go` — Page update pattern (for publish/move status changes)
- `cmd/root.go` lines 282-301 — Command registration pattern (AddCommand for standalone, mergeCommand for overrides)

### Existing cf packages
- `internal/client/client.go` — `FromContext()`, `Do()`, `Fetch()` for API calls; `searchV1Domain()` for v1 URL construction
- `internal/duration/duration.go` — `Parse()` for `--timeout` flag values
- `internal/jsonutil/jsonutil.go` — `MarshalNoEscape()` for JSON output
- `internal/errors/errors.go` — `APIError` struct, `WriteJSON()`, exit codes

### Generated API endpoints
- `cmd/generated/pages.go` — v2 pages API: create, update, delete (status field for publish/archive)
- `cmd/generated/footer_comments.go` — v2 footer comments API (reference for comment wrapper)

### Phase context
- `.planning/phases/14-version-diff/14-CONTEXT.md` — Recent phase context with similar pattern decisions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/comments.go`: Footer comment create pattern — reuse for workflow comment
- `internal/client/client.go`: `searchV1Domain()` — constructs v1 API base URL from v2 base, needed for copy and restrict
- `internal/client/client.go`: `Do()` and `Fetch()` — standard API call patterns
- `internal/duration/duration.go`: `Parse()` — for --timeout flag
- `internal/jsonutil/jsonutil.go`: `MarshalNoEscape()` — for JSON output
- `internal/errors/errors.go`: `APIError`, exit codes — error handling

### Established Patterns
- Hand-written commands: flag parsing → `client.FromContext()` → API call → marshal → WriteOutput
- v1 API calls: `searchV1Domain` pattern from search, labels, attachments
- Command registration: `rootCmd.AddCommand()` for new parent commands
- Required flags: `cmd.MarkFlagRequired()` in init()
- Validation: empty string check → APIError → AlreadyWrittenError pattern

### Integration Points
- `cmd/root.go`: Register `workflowCmd` as `rootCmd.AddCommand(workflowCmd)`
- `cmd/generated/pages.go`: v2 page update for move/publish
- `cmd/generated/footer_comments.go`: v2 footer comments for comment wrapper
- `internal/client`: v1 domain construction for copy/restrict endpoints

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following jr patterns adapted for cf.

</specifics>

<deferred>
## Deferred Ideas

- **WKFL-07 (restore)**: Restore a previous page version — deferred to future milestone per REQUIREMENTS.md
- **WKFL-08 (bulk move)**: Bulk move multiple pages — deferred to future milestone per REQUIREMENTS.md

</deferred>

---

*Phase: 15-workflow-commands*
*Context gathered: 2026-03-28*
