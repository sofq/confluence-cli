# Phase 7: Blog Posts - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Full CRUD operations for Confluence blog posts mirroring the pages pattern from Phase 3. Users can list, get, create, update (with automatic version increment), and delete blog posts. Blog posts use the same v2 API conventions as pages but under the `/blogposts` resource path.

</domain>

<decisions>
## Implementation Decisions

### Command naming
- Command is `cf blogposts` — matches generated command name and Confluence v2 API resource
- Consistent with existing resource naming: `cf pages`, `cf spaces`, `cf comments`
- No alias or shorthand needed

### Create flags
- Same flags as pages: `--space-id`, `--title`, `--body` (all required)
- No `--parent-id` flag — blog posts don't nest under parent pages
- No `--status` flag — keep it simple, consistent with pages create

### Mirroring pages pattern
- get-by-id: inject `body-format=storage` by default, allow override via `--body-format`
- create: build JSON body internally with spaceId, title, body.representation=storage
- update: automatic version increment with single 409 retry (same optimistic locking as pages)
- delete: soft-delete via HTTP DELETE
- list: optional `--space-id` filter with auto-pagination

### Claude's Discretion
- Exact variable naming (blogpost vs blog_post prefix)
- Whether to extract shared helpers between pages.go and blogposts.go or keep them independent
- Test structure and test helper patterns

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Pages pattern (reference implementation)
- `cmd/pages.go` — Complete pages CRUD pattern to mirror: get-by-id, create, update (version auto-increment + 409 retry), delete, list
- `cmd/pages_test.go` — Test patterns for workflow commands

### Generated blogposts
- `cmd/generated/blogposts.go` — Already-generated blogposts parent command and subcommands from OpenAPI spec

### Command wiring
- `cmd/root.go` — mergeCommand pattern for overriding generated commands with hand-written wrappers

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/pages.go`: Complete CRUD pattern — blog posts is essentially the same file with `/pages` → `/blogposts`
- `fetchPageVersion` / `doPageUpdate` helpers: Same pattern needed for blog posts (fetchBlogpostVersion / doBlogpostUpdate)
- `cmd/generated/blogposts.go`: Parent command already exists, subcommands already generated

### Established Patterns
- mergeCommand: Hand-written wrappers override generated commands by matching Use field
- body-format=storage: Default injection on get-by-id
- Version auto-increment: Fetch current version, increment, retry once on 409
- Flag validation: TrimSpace checks with structured JSON errors

### Integration Points
- `cmd/root.go init()`: mergeCommand(rootCmd, blogpostsCmd) to wire blog post commands
- `client.Client`: All existing Fetch/Do methods work for blog post endpoints

</code_context>

<specifics>
## Specific Ideas

No specific requirements — mirror the pages pattern exactly.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-blog-posts*
*Context gathered: 2026-03-20*
