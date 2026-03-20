# Roadmap: Confluence CLI (`cf`)

## Milestones

- ✅ **v1.0 Core CLI** - Phases 1-5 (shipped 2026-03-20)
- 🚧 **v1.1 Extended Capabilities** - Phases 6-11 (in progress)

## Phases

<details>
<summary>✅ v1.0 Core CLI (Phases 1-5) - SHIPPED 2026-03-20</summary>

- [x] **Phase 1: Core Scaffolding** - HTTP client, config profiles, auth, and the pure JSON output contract (completed 2026-03-20)
- [x] **Phase 2: Code Generation Pipeline** - OpenAPI spec parser/generator producing all Cobra commands (completed 2026-03-20)
- [x] **Phase 3: Pages, Spaces, Search, Comments, and Labels** - Primary resources with Confluence-specific workflow wrappers (completed 2026-03-20)
- [x] **Phase 4: Governance and Agent Optimization** - Operation policy, audit logging, response caching, and batch execution (completed 2026-03-20)
- [x] **Phase 5: Avatar Analysis** - AI-ready writing style analysis from Confluence user content (completed 2026-03-20)

### Phase 1: Core Scaffolding
**Goal**: AI agents and users can authenticate and make raw API calls, with all infrastructure guarantees in place.
**Depends on**: Nothing (first phase)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06, INFRA-07, INFRA-08, INFRA-09, INFRA-10, INFRA-11, INFRA-12, INFRA-13
**Plans**: 4 plans

Plans:
- [x] 01-01: Go module scaffold, internal packages
- [x] 01-02: HTTP client with cursor-based pagination
- [x] 01-03: Cobra commands (root, configure, raw, version, schema)
- [x] 01-04: Test suite for all Phase 1 packages and commands

### Phase 2: Code Generation Pipeline
**Goal**: The gen/ pipeline reads the OpenAPI spec and produces all Cobra commands; generated commands can be overridden by hand-written wrappers.
**Depends on**: Phase 1
**Requirements**: CGEN-01, CGEN-02, CGEN-03, CGEN-04, CGEN-05
**Plans**: 3 plans

Plans:
- [x] 02-01: Download spec, add libopenapi, document spec gaps
- [x] 02-02: gen/ core: parser, grouper, generator, templates, unit tests
- [x] 02-03: gen/main.go, conformance tests, make generate

### Phase 3: Pages, Spaces, Search, Comments, and Labels
**Goal**: AI agents can perform all primary Confluence content operations with all v2 API edge cases handled.
**Depends on**: Phase 2
**Requirements**: PAGE-01, PAGE-02, PAGE-03, PAGE-04, PAGE-05, SPCE-01, SPCE-02, SPCE-03, SRCH-01, SRCH-02, SRCH-03, CMNT-01, CMNT-02, CMNT-03, LABL-01, LABL-02, LABL-03
**Plans**: 4 plans

Plans:
- [x] 03-01: cmd/pages.go CRUD
- [x] 03-02: cmd/spaces.go with key resolution
- [x] 03-03: cmd/search.go, cmd/comments.go, cmd/labels.go
- [x] 03-04: Wire all commands + unit tests

### Phase 4: Governance and Agent Optimization
**Goal**: Production deployments can enforce operation policies, maintain audit trails, and execute multi-step workflows via batch.
**Depends on**: Phase 3
**Requirements**: GOVN-01, GOVN-02, GOVN-03, GOVN-04, BTCH-01, BTCH-02, BTCH-03
**Plans**: 3 plans

Plans:
- [x] 04-01: internal/policy and internal/audit packages
- [x] 04-02: Wire policy and audit into cmd/root.go
- [x] 04-03: cmd/batch.go command with test suite

### Phase 5: Avatar Analysis
**Goal**: AI agents can obtain structured JSON persona profiles from Confluence user writing history.
**Depends on**: Phase 3
**Requirements**: AVTR-01, AVTR-02
**Plans**: 2 plans

Plans:
- [x] 05-01: internal/avatar/ package
- [x] 05-02: cmd/avatar.go analyze subcommand + tests

</details>

### v1.1 Extended Capabilities (In Progress)

**Milestone Goal:** Add OAuth2 authentication, content type coverage (blogs, attachments, custom types), and advanced agent features (watch, output presets, content templates).

- [ ] **Phase 6: OAuth2 Authentication** - Client credentials and browser-based OAuth2 with automatic token refresh
- [ ] **Phase 7: Blog Posts** - Full CRUD for blog posts mirroring the pages pattern
- [ ] **Phase 8: Attachments** - Attachment listing, metadata, upload (v1 API), and deletion
- [ ] **Phase 9: Custom Content** - CRUD for custom content types via v2 API
- [ ] **Phase 10: Output Presets and Templates** - Named output presets and content template system
- [ ] **Phase 11: Watch** - Long-running content change polling with NDJSON event streaming

## Phase Details

### Phase 6: OAuth2 Authentication
**Goal**: Users and service accounts can authenticate via OAuth2 (both machine-to-machine and interactive browser flow), with tokens managed transparently across sessions.
**Depends on**: Phase 5 (v1.0 complete)
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04
**Success Criteria** (what must be TRUE):
  1. `cf configure --auth-type oauth2` accepts client ID and client secret, and subsequent API calls authenticate via client credentials grant without user interaction
  2. `cf configure --auth-type oauth2-3lo` initiates a browser-based authorization flow with PKCE, and the resulting tokens enable API access
  3. An expired OAuth2 access token is automatically refreshed before the API call executes, without user intervention or visible errors
  4. OAuth2 tokens are stored in `~/.config/cf/tokens/{profile}.json` with 0600 file permissions, separate from the main config file
**Plans**: 2 plans

Plans:
- [ ] 06-01-PLAN.md -- Config schema + token store + OAuth2 client credentials (2LO)
- [ ] 06-02-PLAN.md -- 3LO browser flow with PKCE + automatic token refresh

### Phase 7: Blog Posts
**Goal**: AI agents can perform full CRUD operations on Confluence blog posts with the same reliability as pages.
**Depends on**: Phase 6
**Requirements**: BLOG-01, BLOG-02, BLOG-03, BLOG-04, BLOG-05
**Success Criteria** (what must be TRUE):
  1. `cf blogposts list --space-id <id>` returns a paginated JSON array of blog posts in the space
  2. `cf blogposts get <id>` returns a JSON response with `body.storage.value` containing the blog post content
  3. `cf blogposts create --space-id <id> --title "Post" --body "<p>content</p>"` creates a blog post and returns its JSON representation
  4. `cf blogposts update <id> --title "New Title"` succeeds with automatic version increment (same optimistic locking as pages)
  5. `cf blogposts delete <id>` soft-deletes the blog post and exits 0
**Plans**: 1 plan

Plans:
- [ ] 07-01-PLAN.md -- Blog post CRUD (cmd/blogposts.go) + tests + root wiring

### Phase 8: Attachments
**Goal**: Users can discover, inspect, upload, and remove file attachments on Confluence content.
**Depends on**: Phase 7
**Requirements**: ATCH-01, ATCH-02, ATCH-03, ATCH-04
**Success Criteria** (what must be TRUE):
  1. `cf attachments list --page-id <id>` returns a paginated JSON array of attachments on the content
  2. `cf attachments get <id>` returns attachment metadata as JSON (file name, media type, size, download link)
  3. `cf attachments upload --page-id <id> --file ./report.pdf` uploads the file via multipart/form-data (v1 API) and returns the attachment JSON
  4. `cf attachments delete <id>` removes the attachment and exits 0
**Plans**: TBD

### Phase 9: Custom Content
**Goal**: Users can manage custom content types (from Connect and Forge apps) through the same CRUD pattern as pages and blog posts.
**Depends on**: Phase 7
**Requirements**: CUST-01, CUST-02, CUST-03, CUST-04
**Success Criteria** (what must be TRUE):
  1. `cf custom-content list --type "ac:app:type"` returns a paginated JSON array of custom content of that type
  2. `cf custom-content create --type "ac:app:type" --title "Item" --body "<ac:...>"` creates custom content and returns its JSON representation
  3. `cf custom-content update <id>` updates the custom content with automatic version increment
  4. `cf custom-content delete <id>` removes the custom content and exits 0
**Plans**: TBD

### Phase 10: Output Presets and Templates
**Goal**: Users can save and reuse output formatting configurations and create content from reusable templates with variable substitution.
**Depends on**: Phase 6
**Requirements**: PRST-01, PRST-02, TMPL-01, TMPL-02
**Success Criteria** (what must be TRUE):
  1. User can define a named preset in profile config with a JQ expression and fields list, and it persists across sessions
  2. `cf pages list --preset brief` applies the preset's JQ expression to the output, producing the configured subset of fields
  3. `cf templates list` shows available content templates from `~/.config/cf/templates/`
  4. `cf pages create --template meeting-notes --var "date=2026-03-20" --var "attendees=Alice,Bob"` creates a page with the template's storage format body and variables substituted
**Plans**: TBD

### Phase 11: Watch
**Goal**: AI agents can reactively monitor Confluence content for changes via a long-running polling command that emits structured NDJSON events.
**Depends on**: Phase 7
**Requirements**: WTCH-01, WTCH-02
**Success Criteria** (what must be TRUE):
  1. `cf watch --cql "space = ENG" --interval 60` polls for content changes and emits one NDJSON line per detected change to stdout, each containing the content ID, type, title, modifier, and timestamp
  2. Pressing Ctrl+C (SIGINT) or sending SIGTERM causes the watch command to emit a `{"type":"shutdown"}` event and exit cleanly without partial JSON lines or leaked connections
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 6 -> 7 -> 8 -> 9 -> 10 -> 11

Note: Phase 9 (Custom Content) and Phase 10 (Output Presets and Templates) can execute in parallel after Phase 7 completes, since they have no mutual dependency. Phase 8 and 9 both depend on Phase 7 but not on each other.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Core Scaffolding | v1.0 | 4/4 | Complete | 2026-03-20 |
| 2. Code Generation Pipeline | v1.0 | 3/3 | Complete | 2026-03-20 |
| 3. Pages, Spaces, Search, Comments, and Labels | v1.0 | 4/4 | Complete | 2026-03-20 |
| 4. Governance and Agent Optimization | v1.0 | 3/3 | Complete | 2026-03-20 |
| 5. Avatar Analysis | v1.0 | 2/2 | Complete | 2026-03-20 |
| 6. OAuth2 Authentication | v1.1 | 0/2 | In Progress | - |
| 7. Blog Posts | v1.1 | 0/1 | Not started | - |
| 8. Attachments | v1.1 | 0/? | Not started | - |
| 9. Custom Content | v1.1 | 0/? | Not started | - |
| 10. Output Presets and Templates | v1.1 | 0/? | Not started | - |
| 11. Watch | v1.1 | 0/? | Not started | - |
