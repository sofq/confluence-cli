# Feature Research

**Domain:** Confluence CLI v1.1 -- Extended capabilities (OAuth2, blog posts, attachments, custom content, watch, presets, templates)
**Researched:** 2026-03-20
**Confidence:** HIGH (core features verified against v2 API docs), MEDIUM (OAuth2 2LO availability for Confluence confirmed but newer feature), HIGH (attachment upload v1 fallback well-documented)

## Context

This research covers the **v1.1 milestone only** -- new features to add on top of the already-built v1.0 foundation. Existing capabilities (pages CRUD, spaces, search, comments, labels, batch, audit, policy, avatar analysis, auth profiles) are not repeated here.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that any v1.1 "extended content coverage" release must have. Without these, the version bump is not justified.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Blog post CRUD | Blog posts are a first-class v2 API resource alongside pages. Any tool claiming "content coverage" must include them. Agents doing content automation (meeting notes, status updates, announcements) need blog posts. | LOW | v2 endpoints mirror pages exactly: `POST/GET/PUT/DELETE /wiki/api/v2/blogposts`. Same body structure (spaceId, title, body with representation+value, status). Reuse 90% of pages.go -- version auto-increment, body format, status handling are identical. Create `blogposts.go` following same pattern as pages.go. |
| OAuth2 3LO (browser flow) | Atlassian is pushing OAuth2; many enterprise orgs disable API tokens or PATs. CLI tools without OAuth2 get locked out of these environments. This is the most-requested auth gap. | MEDIUM | Authorization code grant flow. Steps: (1) start local HTTP server on `127.0.0.1:<random-port>` for callback, (2) open browser to `https://auth.atlassian.com/authorize` with client_id, scope, redirect_uri, state, response_type=code, prompt=consent, (3) receive auth code at callback, (4) exchange code for access+refresh tokens at `https://auth.atlassian.com/oauth/token`, (5) call accessible-resources to get cloudId, (6) store tokens in profile. Must include `offline_access` scope for refresh token. Token refresh uses rotating refresh tokens. Go libraries: `cli/oauth` (GitHub CLI's own library) or `int128/oauth2cli`. API base changes to `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/api/v2/...`. |
| Attachment listing and metadata | Attachments are core Confluence content. Agents processing pages need to discover what files are attached. Not being able to list/get attachment metadata is a gap. | LOW | v2 API: `GET /attachments` (list all), `GET /attachments/{id}` (get one), `GET /pages/{id}/attachments` (per page), `GET /blogposts/{id}/attachments` (per blog post). Standard cursor pagination. Returns metadata: id, title, mediaType, fileSize, downloadUrl. Same fetch pattern as other list commands. |
| Attachment upload | Most users dealing with attachments need upload, not just read. Agents attaching generated reports, images, or exports need this. | MEDIUM | **No v2 upload endpoint exists** (tracked as CONFCLOUD-77196). Must use v1: `POST /wiki/rest/api/content/{id}/child/attachment` with `multipart/form-data` and `X-Atlassian-Token: nocheck` header. Requires adding multipart request support to internal/client (currently handles only JSON bodies). Need v1 path support (client currently assumes v2 base path `/wiki/api/v2`). Flags: `--file path/to/file`, `--content-type` (auto-detect from extension if omitted), `--comment "description"`. |

### Differentiators (Competitive Advantage)

Features that set cf apart from other Confluence CLI tools, particularly for the AI-agent audience. None of the competing tools (atlassian-python-api, confluence-go-api, pchuri/confluence-cli) offer these.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| OAuth2 client credentials / 2LO | Service accounts with zero user interaction -- critical for CI/CD pipelines and autonomous agent execution. No other Confluence CLI supports this. | LOW | `POST https://auth.atlassian.com/oauth/token` with `grant_type=client_credentials`, client_id, client_secret. Returns bearer token valid 60 minutes. No refresh token needed -- just re-request when expired. Admin creates service account credential in Atlassian admin console and selects Confluence scopes. Simpler than 3LO. Store client_id+client_secret in profile, request token on demand. |
| Watch command (polling + NDJSON) | Agents can subscribe to content changes without cron jobs or external schedulers. Enables reactive workflows. Unique for Confluence CLI tools. | MEDIUM | Poll `GET /pages/{id}` or CQL search at configurable interval. Compare version numbers between polls. Emit NDJSON events to stdout: `{"event":"changed","id":"...","version":5,"title":"...","timestamp":"..."}`. Flags: `--interval 30s` (default), `--pages id1,id2` or `--cql "space=DEV AND type=page"`, `--space KEY`. Use Go `time.Ticker` + `context.WithCancel`. Exit on SIGINT/SIGTERM. First poll emits `{"event":"baseline",...}` for each item. |
| Output presets | Named JQ + field combinations stored in config. Agents call `--preset compact` instead of remembering complex JQ expressions. Reduces prompt engineering effort and makes agent invocations more reliable. | LOW | Store in config.json under `presets` key: `{"compact": {"jq": ".results[] \| {id,title}", "fields": "id,title"}}`. Apply via `--preset` flag on any command. Ship with built-in defaults (e.g., `brief`, `ids-only`, `full`). User presets override built-ins. Resolution: `--jq` flag > preset jq > no filter. Pure config feature, zero API work needed. |
| Content templates (local) | Reusable content skeletons for page/blog creation. Agents create consistent content (meeting notes, decision logs, status reports) without remembering Confluence storage format syntax. | LOW | Local template files in `~/.config/cf/templates/` containing storage-format XHTML with Go template variables (`{{.Title}}`, `{{.Date}}`, `{{.SpaceKey}}`, `{{.Custom.key}}`). Commands: `cf templates list`, `cf templates show <name>`. Usage: `cf pages create --template meeting-notes --var key=value`. Go `text/template` stdlib -- no external deps. No Confluence template API needed (v2 has no template endpoints; v1 templates are Confluence-internal and fragile). |
| Custom content type CRUD | Full CRUD on app-defined custom content types (e.g., Confluence Questions, database entries, forms). Enables advanced integrations that competing CLIs cannot reach. | MEDIUM | v2 API: `POST/GET/PUT/DELETE /wiki/api/v2/custom-content`. Requires `type` parameter (app-defined string like `ac:my-app:my-type`). Same body structure as pages (title, body with representation+value, status, version). Users must know their app's custom content type string. Scope per-space or per-page/blogpost. Niche but uniquely powerful for teams using Confluence Connect apps. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Confluence API template integration | "Create pages from Confluence templates via API" | v2 API has zero template endpoints. v1 template API requires: fetch template body, extract storage format, handle blueprint variables, process inline images. Template images are known broken (Atlassian bug -- images returned without id/URL). Mixing v1 template calls with v2 page creation adds fragility and version confusion. | Local template system (see differentiators). Store storage-format snippets locally under version control. Simpler, more reliable, portable across instances. |
| Markdown-to-storage-format conversion | "Let me write content in Markdown" | Already explicitly out of scope in PROJECT.md. Atlassian Storage Format has Confluence-specific macros, panels, layouts that Markdown cannot represent. Lossy round-tripping. Conversion libraries break on edge cases. Adds a heavy dependency. | Pass raw storage format. Agents generate storage format directly. Provide example templates showing common patterns. |
| Real-time WebSocket streaming | "Watch should use WebSockets for instant updates" | Confluence has no WebSocket API. Would require building a proxy/relay service. Completely incompatible with CLI tool model. | Polling-based watch with configurable interval (default 30s). NDJSON output is streaming-friendly for piping. |
| Attachment inline preview/rendering | "Show me the attachment contents in terminal" | CLI is not a rendering environment. Adding image/PDF rendering introduces heavy dependencies. Agents consume metadata, not visual content. | Return attachment metadata (filename, mediaType, fileSize, downloadUrl). Agents download files separately with `curl` or `cf raw`. |
| OAuth2 device code flow | "Support headless OAuth without a browser" | Atlassian does not support the OAuth2 device authorization grant. Only authorization code (3LO) and client credentials (2LO) are available. Implementing unsupported flows wastes effort. | Use client credentials (2LO) for headless/CI environments. Use 3LO browser flow for interactive human use. These two cover all scenarios. |
| Bulk content export (PDF/Word) | "Export pages as PDF for offline reading" | Export endpoints produce binary blobs, not JSON. Breaks the JSON-stdout contract. Agents cannot meaningfully process binary output. | `cf raw GET /wiki/rest/api/content/{id}/export/pdf > file.pdf` for one-off needs. Not a first-class feature. |

---

## Feature Dependencies

```
OAuth2 3LO (browser flow)
    └──requires──> Token storage in config profiles (access_token, refresh_token, expiry, cloudId)
    └──requires──> Token refresh middleware in internal/client (transparent refresh before API calls)
    └──requires──> Dynamic base URL (api.atlassian.com/ex/confluence/{cloudId} instead of direct instance URL)

OAuth2 2LO (client credentials)
    └──requires──> Config profile extension (client_id, client_secret fields)
    └──requires──> Token cache with TTL in internal/client (simpler than 3LO -- no refresh token)
    └──requires──> Dynamic base URL (same as 3LO)

Blog post CRUD
    └──reuses──> Pages pattern (version auto-increment, body format, status handling)
    └──reuses──> Labels subsystem (already supports any content-id)
    └──reuses──> Comments subsystem (already supports any content-id)

Attachment upload
    └──requires──> Multipart form-data support in internal/client (NEW capability)
    └──requires──> v1 API path support in internal/client (currently only v2 base path)

Attachment list/get
    └──reuses──> Standard fetch + pagination pattern (no new client capabilities)

Custom content CRUD
    └──reuses──> Pages pattern (version auto-increment, body format)
    └──requires──> --type flag for custom content type string

Watch command
    └──requires──> Pages/Blogposts GET (already exists)
    └──requires──> CQL search (already exists, for space-level watching)
    └──enhances──> Output presets (watch output can use presets for event formatting)

Output presets
    └──requires──> Config extension (presets key in config.json)
    └──enhances──> ALL existing commands (global --preset flag in root.go)
    └──independent──> No API changes needed

Content templates
    └──enhances──> Pages create, Blogposts create (--template flag)
    └──independent──> No API dependency, pure local feature
    └──independent──> No dependency on other v1.1 features
```

### Dependency Notes

- **Attachment upload requires v1 API path**: The client currently constructs paths under `/wiki/api/v2`. Upload needs `POST /wiki/rest/api/content/{id}/child/attachment`. Add a method like `client.FetchV1()` or accept absolute path override in `client.Fetch()`. Also need multipart body encoding -- currently only `json.Marshal` bodies.
- **OAuth2 changes the API base URL**: With OAuth2, requests go through `api.atlassian.com/ex/confluence/{cloudId}` not the direct instance URL. The client must switch base URL based on auth type. This affects all commands transparently.
- **OAuth2 3LO and 2LO share infrastructure**: Both need token endpoint interaction and dynamic base URLs. Build shared OAuth2 token management, then add 3LO (interactive) and 2LO (non-interactive) as two grant type implementations.
- **Blog posts and custom content reuse pages patterns**: Version auto-increment, body format, status handling are identical across all three. Factor shared helpers (e.g., `fetchContentVersion`, `buildUpdateBody`) to avoid duplication.
- **Output presets should be implemented early**: It enhances every command globally via a `--preset` flag in root.go's PersistentPreRunE. Low effort, high leverage. Do it before feature-specific commands.
- **Watch depends on existing GET commands only**: No new API endpoints. Just polling + diff logic + NDJSON output formatting. Can be built independently after blog posts exist (to watch both pages and blog posts).
- **Content templates are fully independent**: Local file system feature with Go stdlib `text/template`. No dependency on any API or other v1.1 feature. Can be built in any order.

---

## MVP Definition

### Launch With (v1.1 core)

Must-have features that justify the v1.1 version bump. These complete content coverage and close the auth gap.

- [ ] Blog post CRUD -- direct v2 API parity with pages, lowest effort among new features, highest coverage impact
- [ ] OAuth2 3LO browser flow -- unblocks users in orgs that disabled API tokens; most-requested auth feature
- [ ] Attachment list/get/delete -- read-side attachment support via v2 API, completes content discovery
- [ ] Attachment upload -- write-side via v1 fallback, completes the attachment story
- [ ] Output presets -- low effort, high agent value, enhances every existing and new command

### Add After Core (v1.1 extended)

Features that add differentiation once core content+auth is stable.

- [ ] OAuth2 2LO client credentials -- when CI/CD and autonomous agent demand materializes
- [ ] Watch command (polling + NDJSON) -- when agents request change-driven rather than request-driven access
- [ ] Content templates (local) -- when users report friction creating repetitive content in storage format

### Future Consideration (v1.2+)

- [ ] Custom content CRUD -- niche use case, value depends on which Connect apps target users have installed
- [ ] Space-level watch (poll all pages in a space via CQL) -- scaling concern, needs rate limit awareness with new Atlassian quota system (enforced March 2026)

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Blog post CRUD | HIGH | LOW | P1 |
| OAuth2 3LO (browser flow) | HIGH | MEDIUM | P1 |
| Attachment list/get/delete | HIGH | LOW | P1 |
| Attachment upload (v1 fallback) | HIGH | MEDIUM | P1 |
| Output presets | MEDIUM | LOW | P1 |
| OAuth2 2LO (client credentials) | MEDIUM | LOW | P2 |
| Content templates (local) | MEDIUM | LOW | P2 |
| Watch command (polling + NDJSON) | MEDIUM | MEDIUM | P2 |
| Custom content CRUD | LOW | MEDIUM | P3 |

**Priority key:**
- P1: Must have for v1.1 launch -- completes content type coverage and auth story
- P2: Should have -- adds differentiation for agent and automation users
- P3: Nice to have -- serves niche use cases, defer until P2 is validated

---

## Competitor Feature Analysis

| Feature | atlassian-python-api | confluence-go-api | pchuri/confluence-cli | cf v1.1 (this release) |
|---------|---------------------|-------------------|----------------------|------------------------|
| Blog post CRUD | Yes (v1 API) | Partial | No | v2 API, version auto-increment |
| OAuth2 any flow | No (basic/token only) | No | No | 3LO browser + 2LO client credentials |
| Attachment upload | Yes (v1 multipart) | Yes (v1 multipart) | No | v1 fallback with multipart |
| Attachment list | Yes (v1) | Yes (v1) | No | v2 API with pagination |
| Custom content CRUD | No | No | No | v2 API with type parameter |
| Watch/polling | No | No | No | NDJSON streaming events |
| Output presets | N/A (library) | N/A (library) | No | Named JQ+fields in config |
| Content templates | `create_or_update_template()` (v1 API) | No | No | Local templates with Go text/template |
| Agent-optimized output | No | No | No | JSON-only stdout, semantic exit codes, JQ built-in |

---

## Key API Details for Implementation

### Blog Post v2 Endpoints
- `GET /wiki/api/v2/blogposts` -- list (cursor pagination, `space-id` filter)
- `POST /wiki/api/v2/blogposts` -- create (`spaceId`, `title`, `body`, `status`)
- `GET /wiki/api/v2/blogposts/{id}` -- get by ID
- `PUT /wiki/api/v2/blogposts/{id}` -- update (`id`, `title`, `body`, `status`, `version`)
- `DELETE /wiki/api/v2/blogposts/{id}` -- delete/trash

### Attachment v2 Endpoints (read + delete only)
- `GET /wiki/api/v2/attachments` -- list all
- `GET /wiki/api/v2/attachments/{id}` -- get by ID
- `GET /wiki/api/v2/pages/{id}/attachments` -- list for page
- `GET /wiki/api/v2/blogposts/{id}/attachments` -- list for blog post
- `GET /wiki/api/v2/custom-content/{id}/attachments` -- list for custom content
- `DELETE /wiki/api/v2/attachments/{id}` -- delete

### Attachment v1 Endpoint (upload -- no v2 equivalent)
- `POST /wiki/rest/api/content/{id}/child/attachment` -- multipart upload
  - Header: `X-Atlassian-Token: nocheck` (XSRF protection bypass)
  - Body: `multipart/form-data` with `file` field (the actual file)
  - Optional fields: `comment` (attachment description), `minorEdit` (boolean)

### Custom Content v2 Endpoints
- `GET /wiki/api/v2/custom-content` -- list (requires `type` query param)
- `POST /wiki/api/v2/custom-content` -- create (`type`, `spaceId`, `pageId`/`blogPostId`, `title`, `body`, `status`)
- `GET /wiki/api/v2/custom-content/{id}` -- get by ID
- `PUT /wiki/api/v2/custom-content/{id}` -- update (includes `version`)
- `DELETE /wiki/api/v2/custom-content/{id}` -- delete/trash

### OAuth2 Endpoints
- **Authorization** (3LO): `https://auth.atlassian.com/authorize?audience=api.atlassian.com&client_id={id}&scope={scopes}&redirect_uri={uri}&state={state}&response_type=code&prompt=consent`
- **Token exchange**: `POST https://auth.atlassian.com/oauth/token` (both 3LO code exchange and 2LO client credentials)
- **Accessible resources**: `GET https://api.atlassian.com/oauth/token/accessible-resources` (returns cloudId for site)
- **API base with OAuth**: `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/api/v2/...`
- **Key scopes**: `read:confluence-content.all`, `write:confluence-content`, `manage:confluence-configuration`, `offline_access` (for refresh tokens)

---

## Sources

- [Confluence REST API v2 -- Blog Post endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-blog-post/)
- [Confluence REST API v2 -- Attachment endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-attachment/)
- [Confluence REST API v2 -- Custom Content endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-custom-content/)
- [Confluence REST API v2 -- Introduction (all resource groups)](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/)
- [OAuth 2.0 (3LO) apps -- Confluence Cloud](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/)
- [Implementing OAuth 2.0 (3LO)](https://developer.atlassian.com/cloud/oauth/getting-started/implementing-oauth-3lo/)
- [OAuth 2.0 credentials for service accounts (2LO)](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/)
- [Custom content overview](https://developer.atlassian.com/cloud/confluence/custom-content/)
- [Attachment upload via v1 REST API](https://support.atlassian.com/confluence/kb/using-the-confluence-rest-api-to-upload-an-attachment-to-one-or-more-pages/)
- [Missing v2 attachment upload endpoint (CONFCLOUD-77196)](https://jira.atlassian.com/browse/CONFCLOUD-77196)
- [cli/oauth -- GitHub CLI's OAuth library for Go](https://github.com/cli/oauth)
- [oauth2cli -- Go OAuth2 CLI helper](https://pkg.go.dev/github.com/int128/oauth2cli)
- [Confluence REST API v1 -- Template endpoints](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-template/)

---
*Feature research for: Confluence CLI v1.1 extended capabilities*
*Researched: 2026-03-20*
