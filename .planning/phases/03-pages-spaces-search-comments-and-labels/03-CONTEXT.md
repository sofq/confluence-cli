# Phase 3: Pages, Spaces, Search, Comments, and Labels - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Hand-written workflow commands for the five primary Confluence resources: pages (CRUD + version auto-increment), spaces (list/get + key-to-ID resolution), search (CQL + pagination), comments (list/create/delete), and labels (list/add/remove). These override the generated commands via `mergeCommand` to handle Confluence-specific edge cases that the generated code cannot address.

</domain>

<decisions>
## Implementation Decisions

### Pages — Version Auto-Increment
- `cf pages update` must automatically fetch current version, increment, and include in PUT body
- Handle 409 Conflict by retrying with latest version (single retry)
- `cf pages get` must always include `?body-format=storage` to avoid empty body responses

### Pages — Soft Delete
- `cf pages delete` sends HTTP DELETE (moves to trash) — this is the expected Confluence behavior
- No purge command in v1 (admin-only, dangerous)

### Space Key Resolution
- `cf spaces list --key <KEY>` resolves key to numeric ID via `GET /wiki/api/v2/spaces?keys=<KEY>`
- Other commands accepting space references should accept either key or numeric ID

### CQL Search — Cursor Handling
- CQL pagination may produce very long cursor strings (up to 11KB)
- Use POST-based search if cursor exceeds URL length limits, or truncate gracefully
- The client's existing `doCursorPagination` handles the merge; search just needs to call `c.Do()`

### Comments and Labels
- Simple CRUD wrappers — no complex edge cases
- Comments use storage format body (same as pages)
- Labels are plain strings

### Claude's Discretion
- File organization (one file per resource vs grouped)
- Internal helper functions for version fetching
- Error message wording
- Test structure

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/root.go` — `mergeCommand(rootCmd, cmd)` for overriding generated commands
- `internal/client/client.go` — `Do()`, `Fetch()`, `WriteOutput()`, cursor pagination already working
- `cmd/generated/*.go` — 24 resource files with basic CRUD already generated
- Reference workflow commands: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/workflow.go`

### Established Patterns
- Generated commands handle basic REST ops; workflow commands add business logic
- `mergeCommand` replaces generated command with hand-written version
- `c.Fetch()` for commands needing to process response before output
- `c.Do()` for simple pass-through commands

### Integration Points
- `cmd/root.go` init() — add new commands via `rootCmd.AddCommand()` or `mergeCommand()`
- Generated pages/spaces/search commands exist — workflow wrappers override specific operations
- All commands use `client.FromContext(cmd)` to get the HTTP client

</code_context>

<specifics>
## Specific Ideas

- Mirror the pattern from jr's `cmd/workflow.go` which has `transition`, `assign`, `comment`, `create` wrappers
- Keep commands focused: one file per resource group (pages.go, spaces.go, search.go, comments.go, labels.go)
- Each workflow command should document which generated command it overrides

</specifics>

<deferred>
## Deferred Ideas

- Blog post CRUD (v2 requirement)
- Attachment management (v1 API only, documented in SPEC_GAPS.md)
- Bulk page operations

</deferred>
