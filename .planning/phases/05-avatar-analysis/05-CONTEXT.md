# Phase 5: Avatar Analysis - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement `cf avatar analyze --user <accountId>` which fetches a user's Confluence pages via CQL search, analyzes their writing style, and outputs a structured JSON persona profile that AI agents can consume for content generation or style matching.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

All implementation choices are at Claude's discretion. Mirror the avatar feature from the reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/avatar/` and `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/avatar.go`.

Key adaptations:
- Use CQL search to fetch user's pages: `creator = "<accountId>" ORDER BY lastModified DESC`
- Extract text content from Confluence storage format (strip HTML tags)
- Analyze writing patterns: tone, vocabulary, structure, formatting preferences
- Output structured JSON profile directly consumable by AI agents
- The search command already handles v1 CQL search — reuse `searchV1Domain()` pattern

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/search.go` — `searchV1Domain()` pattern for v1 API calls
- `internal/client/client.go` — Fetch() for getting raw response bytes
- Reference: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/avatar/` — full avatar implementation
- Reference: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/avatar.go` — avatar command

### Integration Points
- New `cmd/avatar.go` registered via `rootCmd.AddCommand(avatarCmd)` in root.go
- New `internal/avatar/` package for analysis logic

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the reference implementation pattern.

</specifics>

<deferred>
## Deferred Ideas

None.

</deferred>
