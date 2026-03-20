# Phase 8: Attachments - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Attachment operations on Confluence content: list, get metadata, upload (v1 API multipart), and delete. Upload is the only v1 API operation — list, get, and delete use v2 endpoints. Generated `cmd/generated/attachments.go` already provides the parent command.

</domain>

<decisions>
## Implementation Decisions

### Upload source
- `--file` flag with filesystem path only — no stdin support
- Multipart form-data requires Content-Length which stdin can't provide reliably
- Agent passes explicit file path

### Upload output
- Return full JSON response from the API (id, title, mediaType, fileSize, download link)
- Consistent with all other commands — no special minimal mode

### v1 API upload pattern
- Upload uses v1 REST API: POST `/rest/api/content/{id}/child/attachment`
- Requires `X-Atlassian-Token: no-check` header
- Uses multipart/form-data with file part
- Domain extraction via `searchV1Domain()` pattern from cmd/search.go (or new shared SiteRoot helper)
- SiteRoot() method needed to avoid URL doubling bug (flagged in STATE.md blockers)

### v2 API operations
- List: GET `/attachments` with optional `--page-id` filter (generated command exists)
- Get: GET `/attachments/{id}` for metadata (generated command exists)
- Delete: DELETE `/attachments/{id}` (generated command exists)

### Claude's Discretion
- Whether to extract searchV1Domain into a shared helper or duplicate for attachments
- Exact multipart form construction (mime/multipart vs manual boundary)
- Whether list/get/delete need hand-written wrappers or can use generated commands as-is

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### v1 API pattern (reference)
- `cmd/search.go` — searchV1Domain() domain extraction, fetchV1() for authenticated v1 calls
- `cmd/labels.go` — Another v1 API consumer (label add/remove uses v1 endpoints)

### Generated attachments
- `cmd/generated/attachments.go` — Already-generated parent command and v2 subcommands

### Command wiring
- `cmd/root.go` — mergeCommand pattern for overriding generated commands

### Known issues
- `.planning/research/PITFALLS.md` — URL doubling bug (commit a6e99ef), X-Atlassian-Token requirement

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `searchV1Domain(baseURL string)`: Extracts scheme+host from BaseURL for v1 API calls
- `fetchV1()`: Authenticated GET against v1 URL — can be adapted for POST multipart
- `client.ApplyAuth()`: Applies auth headers to any *http.Request

### Established Patterns
- v1 API calls use direct net/http + c.ApplyAuth() to avoid URL doubling from c.Fetch()
- c.BaseURL is "https://domain/wiki/api/v2" — v1 paths need domain extraction
- mergeCommand for hand-written wrappers overriding generated commands

### Integration Points
- `cmd/root.go init()`: mergeCommand(rootCmd, attachmentsCmd)
- `client.Client`: ApplyAuth for v1 requests, Do/Fetch for v2 requests
- Generated v2 attachment commands may be sufficient for list/get/delete without hand-written wrappers

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard attachment operations with v1 upload fallback.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-attachments*
*Context gathered: 2026-03-20*
