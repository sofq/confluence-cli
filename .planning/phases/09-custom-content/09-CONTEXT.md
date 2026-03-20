# Phase 9: Custom Content - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

CRUD operations for custom content types (from Connect and Forge apps) via v2 API. Custom content uses the same CRUD pattern as pages and blogposts but with a required `--type` flag to identify the content type. All operations use v2 endpoints under `/custom-content`.

</domain>

<decisions>
## Implementation Decisions

### Type flag behavior
- `--type` is REQUIRED on list — listing without type returns mixed results, not useful for agents
- `--type` is REQUIRED on create — agent must explicitly specify the content type
- `--type` is NOT needed on get-by-id, update, or delete — the API resolves by ID alone

### CRUD pattern
- Mirror pages/blogposts pattern exactly with added --type parameter
- get-by-id: inject body-format=storage by default (same as pages)
- create: --type, --space-id, --title, --body (all required)
- update: version auto-increment with single 409 retry (same as pages/blogposts)
- delete: soft-delete via HTTP DELETE
- list: --type required, optional --space-id filter

### Command naming
- Command is `cf custom-content` — matches generated command name and API resource
- Hyphenated to match Confluence v2 API convention

### Claude's Discretion
- Whether to extract shared version-fetch/update helpers across pages, blogposts, and custom-content
- Test structure details
- Whether generated subcommands suffice for get-by-id and delete or need wrappers

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Pages/blogposts pattern (reference)
- `cmd/pages.go` — Original CRUD pattern with version auto-increment + 409 retry
- `cmd/blogposts.go` — Mirror of pages for blog posts (closest analog)

### Generated custom-content
- `cmd/generated/custom_content.go` — Already-generated parent command and subcommands

### Command wiring
- `cmd/root.go` — mergeCommand pattern

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/blogposts.go`: Closest analog — CRUD with version fetch, update body, 409 retry
- `cmd/generated/custom_content.go`: Parent command + generated subcommands already exist

### Established Patterns
- mergeCommand: Hand-written wrappers override generated commands
- body-format=storage: Default injection on get-by-id
- Version auto-increment: Fetch current, increment, retry on 409
- All v2 API — no v1 fallback needed (unlike attachments)

### Integration Points
- `cmd/root.go init()`: mergeCommand(rootCmd, custom_contentCmd)
- API paths: `/custom-content` (list/create), `/custom-content/{id}` (get/update/delete)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard CRUD with --type flag.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-custom-content*
*Context gathered: 2026-03-20*
