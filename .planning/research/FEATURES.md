# Feature Landscape

**Domain:** Confluence CLI v1.2 -- Workflow commands, version diff, export, built-in presets/templates, CI/CD, docs
**Researched:** 2026-03-28
**Confidence:** HIGH (API endpoints verified against generated code + official docs), MEDIUM (export limitations confirmed via multiple sources)

## Context

This research covers the **v1.2 milestone only** -- new features to add on top of the already-built v1.0 + v1.1 foundation. Existing capabilities are not repeated. The v1.2 milestone has two categories: (1) CLI feature additions (diff, workflow, export, presets, templates) and (2) release infrastructure (CI/CD, GoReleaser, docs site).

This document focuses on **CLI features**. Release infrastructure is covered separately in STACK.md.

---

## Table Stakes

Features users expect from a Confluence CLI that already has CRUD. Missing = product feels incomplete.

### 1. `diff` Command -- Page Version Comparison

| Aspect | Detail |
|--------|--------|
| **Why expected** | Users editing pages need to see what changed between versions. The `jr diff` reference implementation already does this for Jira issues. Confluence has native version history. |
| **Complexity** | Medium |
| **API** | v2: `GET /pages/{id}?version=N&body-format=storage` returns any historical version's body. `GET /pages/{id}/versions` lists all versions with metadata (number, createdAt, message, authorId). `GET /pages/{id}/versions/{versionNumber}` returns version details. |
| **Implementation** | Fetch two versions (default: current vs previous), compute line diff on storage format body. Output structured JSON diff. |
| **Dependencies** | Uses existing `c.Fetch()`, `fetchPageVersion()`. New `internal/diff` package for line-level comparison. |
| **v2 API support** | FULL. All three endpoints exist in v2 and are already generated as `pages get-versions`, `pages get-version-details`, and `pages get-by-id --version N`. Confidence: HIGH (verified in generated code). |

**Flags:**
- `--id` (required) -- page ID
- `--from` (optional) -- version number to compare from (default: current - 1)
- `--to` (optional) -- version number to compare to (default: current)
- `--since` (optional) -- show changes since duration/date (like jr diff)
- `--context` (optional) -- lines of context around changes (default: 3)

**Output format (structured JSON):**
```json
{
  "pageId": "12345",
  "title": "My Page",
  "from": {"number": 4, "createdAt": "...", "authorId": "..."},
  "to": {"number": 5, "createdAt": "...", "authorId": "..."},
  "changes": [
    {"type": "modified", "lineFrom": 10, "lineTo": 12, "old": "...", "new": "..."},
    {"type": "added", "lineFrom": -1, "lineTo": 15, "old": "", "new": "..."},
    {"type": "removed", "lineFrom": 20, "lineTo": -1, "old": "...", "new": ""}
  ],
  "stats": {"added": 5, "removed": 2, "modified": 3}
}
```

### 2. `workflow move` -- Move Page to Different Parent/Space

| Aspect | Detail |
|--------|--------|
| **Why expected** | Reorganizing content is a core Confluence operation. CLI users managing documentation trees need this. |
| **Complexity** | Low |
| **API** | v1 ONLY: `PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}`. No v2 equivalent exists. |
| **Implementation** | Uses `fetchV1()` pattern from search/watch. Position values: `append` (child of target), `before`, `after` (sibling of target). |
| **Dependencies** | Existing `fetchV1()`, `client.SearchV1Domain()` for v1 URL construction. |
| **v1 fallback required** | YES. This endpoint has no v2 equivalent. Confidence: HIGH (verified via Atlassian announcement and API docs). |

**Flags:**
- `--id` (required) -- page ID to move
- `--target-id` (required) -- target page ID (parent for append, sibling for before/after)
- `--position` (optional, default: `append`) -- `append`, `before`, `after`

### 3. `workflow copy` -- Copy Page

| Aspect | Detail |
|--------|--------|
| **Why expected** | Creating variants from existing pages is common. Agents duplicating templates or content trees need this. |
| **Complexity** | Medium (request body has several options) |
| **API** | v1 ONLY: `POST /wiki/rest/api/content/{id}/copy`. No v2 equivalent exists. |
| **Implementation** | Uses `fetchV1()`. Constructs JSON body with destination and copy options. Returns long task ID for async tracking. |
| **Dependencies** | Existing `fetchV1()`. New v1 POST helper needed (current `fetchV1` is GET-only). |
| **v1 fallback required** | YES. Confidence: HIGH. |

**Flags:**
- `--id` (required) -- source page ID
- `--destination-space-id` (optional) -- target space (default: same space)
- `--destination-parent-id` (optional) -- target parent page
- `--title` (optional) -- new title (default: "Copy of {original}")
- `--copy-attachments` (optional, default: true)
- `--copy-permissions` (optional, default: true)
- `--copy-labels` (optional, default: true)

**Request body:**
```json
{
  "copyAttachments": true,
  "copyPermissions": true,
  "copyLabels": true,
  "copyCustomContents": true,
  "destination": {
    "type": "parent_page",
    "value": "67890"
  },
  "pageTitle": "New Title"
}
```

### 4. `workflow restrict` -- Set Page Restrictions

| Aspect | Detail |
|--------|--------|
| **Why expected** | Access control is critical for enterprise Confluence. Agents need to restrict sensitive pages. |
| **Complexity** | Medium (REST structure is complex) |
| **API** | v1 ONLY: `GET/PUT/POST/DELETE /wiki/rest/api/content/{id}/restriction`. Operations: `read` (view restriction), `update` (edit restriction). |
| **Implementation** | Uses `fetchV1()`. Supports add/remove restrictions by user or group for read/update operations. |
| **Dependencies** | Existing `fetchV1()`, new v1 POST/PUT helper. |
| **v1 fallback required** | YES. Confidence: HIGH (verified in API docs + community confirmations). |

**Subcommands:**
- `workflow restrict get --id <pageId>` -- list current restrictions
- `workflow restrict add --id <pageId> --operation read --user <accountId>` -- add restriction
- `workflow restrict remove --id <pageId>` -- remove all restrictions

### 5. `workflow archive` -- Archive Pages

| Aspect | Detail |
|--------|--------|
| **Why expected** | Content lifecycle management. Old pages should be archived, not deleted. |
| **Complexity** | Low (single POST, async response) |
| **API** | v1 ONLY: `POST /wiki/rest/api/content/archive`. Accepts list of page IDs. Returns long task ID. |
| **Implementation** | Simple POST with `{"pages": [{"id": 12345}]}`. Async -- returns task ID for status polling. |
| **Dependencies** | Existing `fetchV1()`, new v1 POST helper. |
| **v1 fallback required** | YES. Confidence: HIGH (verified via community thread + API docs). |

**Flags:**
- `--id` (required, repeatable) -- page ID(s) to archive

### 6. `workflow publish` -- Publish Draft Page

| Aspect | Detail |
|--------|--------|
| **Why expected** | Draft-to-published workflow is common. Agents creating draft content need to publish it. |
| **Complexity** | Low (update with status change) |
| **API** | v2: `PUT /pages/{id}` with `{"status": "current"}` updates a draft to published. |
| **Implementation** | Wrapper around existing page update. Fetches current version, sets status to "current", increments version. |
| **Dependencies** | Existing `fetchPageVersion()`, `doPageUpdate()`. Minor modification to `pageUpdateBody` to support status field. |
| **v2 API support** | FULL. Confidence: HIGH. |

**Flags:**
- `--id` (required) -- draft page ID
- `--title` (optional) -- override title on publish

### 7. `workflow comment` -- Add Comment to Page

| Aspect | Detail |
|--------|--------|
| **Why expected** | Already exists as `comments create` but the jr pattern puts it under `workflow comment` for discoverability. Agents commenting on pages is very common. |
| **Complexity** | Low (wraps existing comments create) |
| **API** | v2: `POST /footer-comments` or `POST /inline-comments`. Already generated. |
| **Implementation** | Convenience wrapper: `workflow comment --page-id X --text "..."` instead of requiring full JSON body. Converts plain text to storage format `<p>text</p>`. |
| **Dependencies** | Existing generated comment endpoints. |
| **v2 API support** | FULL. Confidence: HIGH. |

**Flags:**
- `--page-id` (required) -- page to comment on
- `--text` (required) -- comment text (plain text, wrapped in `<p>` storage format)

### 8. Built-in Presets

| Aspect | Detail |
|--------|--------|
| **Why expected** | The jr reference implementation ships 4 built-in presets. cf currently only supports custom presets in profile config. Users expect useful presets out of the box. |
| **Complexity** | Low |
| **Implementation** | New `internal/preset` package with `Lookup()` and `List()` following jr pattern exactly. Built-in presets are Go map constants. User presets (from config) override built-ins. `preset list` command outputs merged list with source tags. |
| **Dependencies** | Existing `--preset` flag resolution in root.go. Refactor from inline config lookup to `preset.Lookup()`. |

**Built-in preset definitions:**

| Preset Name | JQ Expression | Purpose |
|-------------|---------------|---------|
| `brief` | `.results[] \| {id, title, status}` | Quick page listing -- ID, title, status only |
| `titles` | `.results[].title` | Extract just titles from list responses |
| `agent` | `{id, title, status, spaceId, version: .version.number, body: .body.storage.value}` | Agent-optimized: all fields an AI agent needs for a single page |
| `tree` | `.results[] \| {id, title, parentId: .parentId, position: .position}` | Page hierarchy view -- for building content trees |
| `meta` | `{id, title, status, spaceId, authorId, createdAt, version: .version}` | Metadata only, no body content |
| `search` | `.results[] \| {id, title, type: .content.type, space: .content.space.key, excerpt: .excerpt}` | Search result extraction |
| `diff` | `.changes[] \| {type, old, new}` | Diff output simplification |

**Preset structure (matching jr):**
```go
type Preset struct {
    JQ string `json:"jq"`
}
```

Note: jr presets have a `Fields` field for server-side field filtering. Confluence v2 API does not support equivalent field selection on most endpoints, so cf presets use JQ only. If a `--fields` flag is needed, it can be set alongside the preset.

### 9. Built-in Templates

| Aspect | Detail |
|--------|--------|
| **Why expected** | jr ships 6 built-in templates. cf currently only supports user-defined JSON templates. Agents need common page scaffolds without manual template creation. |
| **Complexity** | Medium (requires embed, YAML format change to match jr) |
| **Implementation** | New `internal/template/builtin/*.yaml` embedded via `go:embed`. Template system extended to support builtin + user overlay (user overrides builtin with same name). Template format migrated from JSON to YAML for consistency with jr. |
| **Dependencies** | Existing `internal/template` package. Add `go:embed`, YAML parsing (gopkg.in/yaml.v3 or stdlib-only alternative). |

**Built-in template definitions:**

| Template Name | Content Type | Variables | Storage Format Body |
|---------------|-------------|-----------|---------------------|
| `blank` | page | `title` (required), `space_id` (required) | `<p></p>` |
| `meeting-notes` | page | `title` (required), `space_id` (required), `date`, `attendees`, `agenda`, `notes`, `action_items` | Meeting notes with sections: Date, Attendees, Agenda, Discussion, Action Items |
| `decision` | page | `title` (required), `space_id` (required), `status`, `stakeholders`, `background`, `decision`, `rationale` | Decision record: Status, Stakeholders, Background, Decision, Rationale |
| `runbook` | page | `title` (required), `space_id` (required), `service`, `description`, `prerequisites`, `steps`, `rollback`, `contacts` | Runbook: Service, Description, Prerequisites, Steps, Rollback, Emergency Contacts |
| `retrospective` | page | `title` (required), `space_id` (required), `sprint`, `went_well`, `improve`, `action_items` | Retro: What Went Well, What To Improve, Action Items |
| `adr` | page | `title` (required), `space_id` (required), `status`, `context`, `decision`, `consequences` | Architecture Decision Record: Status, Context, Decision, Consequences |

**Template YAML format (matching jr pattern):**
```yaml
name: meeting-notes
description: Meeting notes with standard sections
variables:
  - name: title
    required: true
    description: Page title
  - name: space_id
    required: true
    description: Space ID to create page in
  - name: date
    description: Meeting date
    default: "{{current_date}}"
  - name: attendees
    description: Comma-separated attendee names
  - name: agenda
    description: Meeting agenda items
  - name: notes
    description: Discussion notes
  - name: action_items
    description: Action items from the meeting
body: |
  <h2>Date</h2>
  <p>{{.date}}</p>
  <h2>Attendees</h2>
  <p>{{.attendees}}</p>
  <h2>Agenda</h2>
  <p>{{.agenda}}</p>
  <h2>Discussion</h2>
  <p>{{.notes}}</p>
  <h2>Action Items</h2>
  <p>{{.action_items}}</p>
space_id: "{{.space_id}}"
```

### 10. Template Management Subcommands

| Aspect | Detail |
|--------|--------|
| **Why expected** | jr has `template list`, `template show`, `template apply`, `template create`. cf only has `templates list`. Users need to inspect, apply, and create templates. |
| **Complexity** | Medium |
| **Implementation** | Add `templates show <name>`, `templates create <name>` (scaffold or `--from-page`), enhance `templates list` to show builtin+user with source tags. |
| **Dependencies** | Extended `internal/template` package. |

**Subcommands:**
- `templates list` -- (existing, enhanced) show name, description, source (builtin/user), variables
- `templates show <name>` -- display full template definition as JSON
- `templates create <name>` -- scaffold a new user template (or `--from-page <pageId>` to reverse-engineer from existing page)

### 11. `preset list` Subcommand

| Aspect | Detail |
|--------|--------|
| **Why expected** | jr has `preset list`. Users need to discover available presets. |
| **Complexity** | Low |
| **Implementation** | New `preset` parent command with `list` subcommand. Outputs merged builtin + user presets as JSON array with source tags. |
| **Dependencies** | New `internal/preset` package. |

**Output format:**
```json
[
  {"name": "brief", "jq": ".results[] | {id, title, status}", "source": "builtin"},
  {"name": "titles", "jq": ".results[].title", "source": "builtin"},
  {"name": "my-custom", "jq": ".foo", "source": "user"}
]
```

### 12. `export` Command -- Export Page Content

| Aspect | Detail |
|--------|--------|
| **Why expected** | Agents need to extract page content for processing. While `pages get-by-id --body-format storage` works, a dedicated export provides a cleaner interface for content extraction. |
| **Complexity** | Low (wrapper around existing GET) |
| **API** | v2: `GET /pages/{id}?body-format=storage` (or `atlas_doc_format`, `editor`). Already exists. |
| **Implementation** | Convenience wrapper that fetches page body in requested format and outputs just the body content (no metadata wrapper). Supports `--format storage|view|editor|atlas_doc_format`. |
| **Dependencies** | Existing `c.Fetch()`, page GET endpoint. |
| **v2 API support** | FULL. Confidence: HIGH. |

**Flags:**
- `--id` (required) -- page ID
- `--format` (optional, default: `storage`) -- body format
- `--output` (optional) -- write to file instead of stdout

**CRITICAL NOTE on PDF/Word export:** Confluence Cloud has NO REST API endpoint for PDF or Word export. This is a known limitation (Atlassian JIRA issue CONFCLOUD-61557). Only third-party apps (Scroll PDF Exporter, FlyingPDF) support this via their own APIs. The `export` command should support storage/view/atlas_doc_format body extraction only. PDF/Word are explicitly out of scope. Confidence: HIGH (verified via multiple community threads + Atlassian KB article).

---

## Differentiators

Features that set the product apart. Not expected, but valued.

### 1. `diff` with `--since` Duration Filter

| Aspect | Detail |
|--------|--------|
| **Value** | Filter version diffs by time range (e.g., `--since 2h`, `--since 2026-01-01`). Matches jr's diff behavior. Agents can ask "what changed today?" without knowing version numbers. |
| **Complexity** | Medium (parse duration/date, filter versions by timestamp) |
| **Dependencies** | New `internal/duration` package (matching jr) for parsing human-friendly durations. |

### 2. `workflow restrict` with Symbolic User Resolution

| Aspect | Detail |
|--------|--------|
| **Value** | Instead of raw account IDs, support `--user me` (current user) or `--user email@example.com` for restriction commands. Matches jr's assign command pattern. |
| **Complexity** | Medium (requires user lookup API call) |
| **Dependencies** | v2: `GET /users?email=...` or v1 user search endpoint. |

### 3. `templates create --from-page` Reverse Engineering

| Aspect | Detail |
|--------|--------|
| **Value** | Create a template from an existing Confluence page. Fetches page, extracts structure, saves as template with variable placeholders. Matches jr's `template create --from` pattern. |
| **Complexity** | Medium |
| **Dependencies** | Existing page GET, template save logic. |

### 4. `export --tree` Recursive Page Export

| Aspect | Detail |
|--------|--------|
| **Value** | Export an entire page tree (page + all descendants) as NDJSON stream. Useful for agents processing documentation hierarchies. |
| **Complexity** | High (recursive tree walk, rate limiting) |
| **Dependencies** | v2: `GET /pages/{id}/children` (already generated as `pages get-child`), `GET /pages/{id}/descendants`. |

### 5. Workflow Schema Registration

| Aspect | Detail |
|--------|--------|
| **Value** | All workflow commands appear in `cf schema workflow` output and are available for `cf batch` operations. Matches jr's `HandWrittenSchemaOps()` pattern. |
| **Complexity** | Low (schema data registration, no new logic) |
| **Dependencies** | Existing schema infrastructure in `cmd/generated/schema_data.go`. |

---

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| PDF/Word export | Confluence Cloud has NO REST API for PDF/Word export (CONFCLOUD-61557). Would require browser automation or third-party app integration. | Export storage format body only. Agents can convert storage XML to other formats themselves. |
| Markdown conversion | Adds complexity. Agents handle raw storage format fine. Already listed in PROJECT.md Out of Scope. | Pass through storage format as-is. Users can pipe through external converters. |
| Content rendering/preview | CLI outputs JSON, not HTML. Not useful for agents. | Output raw storage format. Agents parse XML directly. |
| Real-time collaboration | WebSocket-based. Not applicable to CLI polling model. | Use `watch` command for change detection via CQL polling. |
| Page tree visual rendering | ASCII tree display is not JSON and breaks agent consumption. | Output `--preset tree` JSON with parentId/position. Agents build trees themselves. |
| Version restore via diff | Restoring versions is destructive and should be an explicit action, not part of diff. | Provide restore as separate `workflow restore-version` if needed (can use v1 `POST /content/{id}/version`). |
| Bulk space export | Space-level export (all pages) is a long-running operation with no API support. | Export individual pages or use `export --tree` for subtrees. |
| Interactive merge conflicts | No interactive mode in agent-focused CLI. | Expose version numbers and let agents handle conflict resolution logic. |

---

## Feature Dependencies

```
Built-in Presets --> preset list command (presets must exist before list can show them)
Built-in Templates --> templates show/create commands (templates must exist before show works)
diff command --> internal/diff package (line comparison logic)
diff --since --> internal/duration package (human-friendly duration parsing)
workflow move/copy/restrict/archive --> v1 POST/PUT helper (extend fetchV1 to support non-GET methods)
workflow publish --> existing doPageUpdate (minor status field addition)
workflow comment --> existing footer-comments create (convenience wrapper)
export command --> existing pages get-by-id (body extraction wrapper)
template create --from-page --> existing pages get-by-id + template save
preset list --> internal/preset package (builtin + user merge)
```

**Critical dependency chain:**
1. `internal/preset` package must be built before `preset list` command or `--preset` refactor
2. `internal/template` must be extended (embed, YAML) before built-in templates work
3. v1 POST/PUT helper must be built before move, copy, restrict, archive
4. `internal/diff` package must be built before `diff` command

---

## API Endpoint Summary

### v2 Endpoints (native, already generated)

| Feature | Endpoint | Method | Generated Command |
|---------|----------|--------|-------------------|
| Diff (list versions) | `/pages/{id}/versions` | GET | `pages get-versions` |
| Diff (version details) | `/pages/{id}/versions/{versionNumber}` | GET | `pages get-version-details` |
| Diff (get version body) | `/pages/{id}?version=N&body-format=storage` | GET | `pages get-by-id --version N` |
| Publish draft | `/pages/{id}` | PUT | `pages update` (with status change) |
| Comment | `/footer-comments` | POST | `footer-comments create` |
| Export body | `/pages/{id}?body-format=storage` | GET | `pages get-by-id` |
| Page children | `/pages/{id}/children` | GET | `pages get-child` |
| Page descendants | `/pages/{id}/descendants` | GET | `pages get-descendants` |
| Page ancestors | `/pages/{id}/ancestors` | GET | `pages get-ancestors` |

### v1 Endpoints (fallback, require fetchV1)

| Feature | Endpoint | Method | Notes |
|---------|----------|--------|-------|
| Move page | `/wiki/rest/api/content/{id}/move/{position}/{targetId}` | PUT | Position: append, before, after |
| Copy page | `/wiki/rest/api/content/{id}/copy` | POST | Async, returns long task ID |
| Archive pages | `/wiki/rest/api/content/archive` | POST | Batch, async, returns long task ID |
| Get restrictions | `/wiki/rest/api/content/{id}/restriction` | GET | Returns current restrictions |
| Add restrictions | `/wiki/rest/api/content/{id}/restriction` | PUT | Sets restrictions (replaces all) |
| Delete restrictions | `/wiki/rest/api/content/{id}/restriction` | DELETE | Removes all restrictions |
| Restore version | `/wiki/rest/api/content/{id}/version` | POST | Restores previous version |

---

## MVP Recommendation

### Must-have for v1.2 (table stakes):

1. **diff command** -- highest agent value, uses only v2 endpoints
2. **workflow move** -- essential content management, simple v1 wrapper
3. **workflow copy** -- essential content management, v1 wrapper with options
4. **workflow comment** -- convenience wrapper, very low complexity
5. **workflow publish** -- draft lifecycle, uses existing v2 update
6. **built-in presets + preset list** -- parity with jr, low complexity
7. **built-in templates + template show/create** -- parity with jr, medium complexity
8. **export command** -- clean body extraction, low complexity

### Defer to future milestone:

- **workflow restrict** -- complex REST structure, lower agent usage frequency. Defer unless specifically requested.
- **workflow archive** -- async operations add complexity. Defer unless specifically requested.
- **export --tree** -- recursive tree walk with rate limiting is high complexity.
- **version restore** -- destructive operation, needs careful UX design.

### Implementation order rationale:

1. Build `internal/preset` and `internal/diff` packages first (no HTTP, pure logic)
2. Build `internal/duration` package (matches jr, pure logic)
3. Extend `fetchV1()` to support PUT/POST methods (unlocks all v1 workflow commands)
4. Build workflow commands (use new v1 helper)
5. Build diff command (uses v2 + internal/diff)
6. Build preset list command (uses internal/preset)
7. Extend template system (embed, YAML, builtin templates)
8. Build export command (thin wrapper)

---

## Sources

- [Confluence Cloud REST API v2 Reference](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/) -- HIGH confidence
- [Confluence Cloud REST API v1 Content Versions](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content-versions/) -- HIGH confidence
- [Confluence Cloud REST API v1 Content Restrictions](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content-restrictions/) -- HIGH confidence
- [Added Move and Copy Page APIs (Atlassian announcement)](https://community.developer.atlassian.com/t/added-move-and-copy-page-apis/37749) -- HIGH confidence
- [Archive content via REST API (community)](https://community.developer.atlassian.com/t/how-to-archive-and-restore-archived-confluence-content-via-rest-api/82062) -- MEDIUM confidence
- [REST API to export PDF (Atlassian KB)](https://support.atlassian.com/confluence/kb/rest-api-to-export-and-download-a-page-in-pdf-format/) -- HIGH confidence (confirms NO native PDF export API)
- [CONFCLOUD-61557: Create PDF export API endpoint](https://jira.atlassian.com/browse/CONFCLOUD-61557) -- HIGH confidence (open feature request)
- [Confluence v1 API Deprecation Timeline](https://community.developer.atlassian.com/t/confluence-rest-api-v2-update-to-v1-deprecation-timeline/75126) -- MEDIUM confidence
- Generated code in `cmd/generated/pages.go` lines 854-921: verified v2 version endpoints exist -- HIGH confidence
- Generated code in `cmd/generated/pages.go` line 146: verified `version` query param on get-by-id -- HIGH confidence
- Jira CLI reference: `cmd/workflow.go`, `cmd/diff.go`, `cmd/preset.go`, `cmd/template.go`, `internal/preset/preset.go`, `internal/template/template.go` -- HIGH confidence (local codebase)
