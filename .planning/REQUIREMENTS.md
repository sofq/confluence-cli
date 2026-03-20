# Requirements: Confluence CLI (`cf`)

**Defined:** 2026-03-20
**Core Value:** Give AI agents reliable, structured JSON access to Confluence content through a CLI

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Infrastructure

- [x] **INFRA-01**: CLI outputs pure JSON to stdout for all commands
- [x] **INFRA-02**: CLI outputs structured JSON errors to stderr with semantic exit codes (0=OK, 1=error, 2=auth, 3=not-found, 4=validation, 5=rate-limit, 6=conflict, 7=server-error)
- [x] **INFRA-03**: User can configure profiles with base URL, auth type, and credentials via `cf configure`
- [x] **INFRA-04**: User can select profile via `--profile` flag or `CF_PROFILE` env var
- [x] **INFRA-05**: CLI supports basic auth (email + API token) and bearer token auth
- [x] **INFRA-06**: User can apply JQ filter to any command output via `--jq` flag
- [x] **INFRA-07**: CLI automatically paginates list endpoints and merges results (cursor-based)
- [x] **INFRA-08**: User can cache GET responses with configurable TTL via `--cache` flag
- [x] **INFRA-09**: User can make raw API calls via `cf raw <METHOD> <path>`
- [x] **INFRA-10**: User can preview write operations without executing via `--dry-run` flag
- [x] **INFRA-11**: User can inspect HTTP request/response details via `--verbose` flag (output to stderr)
- [x] **INFRA-12**: `cf --version` outputs version info as JSON
- [x] **INFRA-13**: User can discover command tree and parameter schemas as JSON via `cf schema`

### Code Generation

- [x] **CGEN-01**: CLI auto-generates Cobra commands from Confluence v2 OpenAPI spec
- [x] **CGEN-02**: Generator groups operations by resource (pages, spaces, search, etc.)
- [x] **CGEN-03**: Generated commands include all path/query/body parameters from spec
- [x] **CGEN-04**: Hand-written workflow commands can override generated commands via `mergeCommand`
- [x] **CGEN-05**: Spec is pinned locally at `spec/confluence-v2.json` with known gaps documented

### Pages

- [x] **PAGE-01**: User can get a page by ID with content body (storage format)
- [x] **PAGE-02**: User can create a page in a space with title and storage format body
- [x] **PAGE-03**: User can update a page with automatic version increment (handles 409 conflicts)
- [x] **PAGE-04**: User can delete a page (soft-delete to trash)
- [x] **PAGE-05**: User can list pages in a space with pagination

### Spaces

- [x] **SPCE-01**: User can list all spaces with pagination
- [x] **SPCE-02**: User can get space details by ID
- [x] **SPCE-03**: CLI transparently resolves space keys to numeric IDs where needed

### Search

- [x] **SRCH-01**: User can search content via CQL with `cf search --cql "<query>"`
- [x] **SRCH-02**: Search results are automatically paginated and merged
- [x] **SRCH-03**: Search handles long cursor strings without 413 errors

### Comments

- [x] **CMNT-01**: User can list comments on a page
- [x] **CMNT-02**: User can create a comment on a page (storage format body)
- [x] **CMNT-03**: User can delete a comment

### Labels

- [x] **LABL-01**: User can list labels on content
- [x] **LABL-02**: User can add labels to content
- [x] **LABL-03**: User can remove labels from content

### Governance

- [x] **GOVN-01**: User can configure allowed/denied operations per profile (glob patterns)
- [x] **GOVN-02**: Policy is enforced pre-request, even in dry-run mode
- [x] **GOVN-03**: Every API call is logged to NDJSON audit file with timestamp, profile, operation, method, path, status
- [x] **GOVN-04**: Audit logging is configurable per-profile or per-invocation via `--audit` flag

### Batch

- [x] **BTCH-01**: User can execute multiple operations from JSON array input via `cf batch`
- [x] **BTCH-02**: Batch output is JSON array with per-operation exit codes and data/error
- [x] **BTCH-03**: Batch supports partial failure (some ops succeed, some fail)

### Avatar

- [x] **AVTR-01**: User can analyze a Confluence user's writing style from their content
- [x] **AVTR-02**: Avatar analysis outputs structured JSON persona profile for AI agent consumption

## v1.1 Requirements

Requirements for milestone v1.1 (Extended Capabilities). Each maps to roadmap phases.

### Enhanced Auth

- [x] **AUTH-01**: User can authenticate via OAuth2 client credentials grant (2LO) for machine-to-machine access
- [x] **AUTH-02**: User can authenticate via OAuth2 authorization code grant (3LO) with PKCE via browser flow
- [x] **AUTH-03**: CLI automatically refreshes expired OAuth2 access tokens before API calls
- [x] **AUTH-04**: OAuth2 tokens are stored securely per profile in separate token files with 0600 permissions

### Blog Posts

- [ ] **BLOG-01**: User can list blog posts in a space with pagination
- [ ] **BLOG-02**: User can get a blog post by ID with content body (storage format)
- [ ] **BLOG-03**: User can create a blog post in a space with title and storage format body
- [ ] **BLOG-04**: User can update a blog post with automatic version increment
- [ ] **BLOG-05**: User can delete a blog post

### Attachments

- [ ] **ATCH-01**: User can list attachments on content
- [ ] **ATCH-02**: User can get attachment metadata by ID
- [ ] **ATCH-03**: User can upload an attachment to content (v1 API multipart)
- [ ] **ATCH-04**: User can delete an attachment

### Custom Content

- [ ] **CUST-01**: User can list custom content of a given type
- [ ] **CUST-02**: User can create custom content with type, title, and body
- [ ] **CUST-03**: User can update custom content
- [ ] **CUST-04**: User can delete custom content

### Output Presets

- [ ] **PRST-01**: User can define named output presets in profile config (JQ expression + fields)
- [ ] **PRST-02**: User can apply a preset to any command output via `--preset <name>`

### Content Templates

- [ ] **TMPL-01**: User can list available content templates
- [ ] **TMPL-02**: User can create content from a template with variable substitution

### Watch

- [ ] **WTCH-01**: User can watch content for changes via `cf watch --cql <query>` with NDJSON event output
- [ ] **WTCH-02**: Watch command handles graceful shutdown on SIGINT/SIGTERM

## Out of Scope

| Feature | Reason |
|---------|--------|
| Markdown <-> Storage Format conversion | Lossless round-tripping not achievable; agents handle raw format |
| Confluence v1 API support | Legacy, being deprecated; `raw` command covers one-off v1 calls |
| Interactive TUI / prompts | Breaks agent invocation; process hangs on stdin |
| Content rendering / HTML preview | Agents consume structured data, not rendered HTML |
| Real-time webhooks | Requires running server, incompatible with CLI model |
| Bulk export (PDF/Word) | Binary output breaks JSON stdout contract |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 1 | Complete |
| INFRA-02 | Phase 1 | Complete |
| INFRA-03 | Phase 1 | Complete |
| INFRA-04 | Phase 1 | Complete |
| INFRA-05 | Phase 1 | Complete |
| INFRA-06 | Phase 1 | Complete |
| INFRA-07 | Phase 1 | Complete |
| INFRA-08 | Phase 1 | Complete |
| INFRA-09 | Phase 1 | Complete |
| INFRA-10 | Phase 1 | Complete |
| INFRA-11 | Phase 1 | Complete |
| INFRA-12 | Phase 1 | Complete |
| INFRA-13 | Phase 1 | Complete |
| CGEN-01 | Phase 2 | Complete |
| CGEN-02 | Phase 2 | Complete |
| CGEN-03 | Phase 2 | Complete |
| CGEN-04 | Phase 2 | Complete |
| CGEN-05 | Phase 2 | Complete |
| PAGE-01 | Phase 3 | Complete |
| PAGE-02 | Phase 3 | Complete |
| PAGE-03 | Phase 3 | Complete |
| PAGE-04 | Phase 3 | Complete |
| PAGE-05 | Phase 3 | Complete |
| SPCE-01 | Phase 3 | Complete |
| SPCE-02 | Phase 3 | Complete |
| SPCE-03 | Phase 3 | Complete |
| SRCH-01 | Phase 3 | Complete |
| SRCH-02 | Phase 3 | Complete |
| SRCH-03 | Phase 3 | Complete |
| CMNT-01 | Phase 3 | Complete |
| CMNT-02 | Phase 3 | Complete |
| CMNT-03 | Phase 3 | Complete |
| LABL-01 | Phase 3 | Complete |
| LABL-02 | Phase 3 | Complete |
| LABL-03 | Phase 3 | Complete |
| GOVN-01 | Phase 4 | Complete |
| GOVN-02 | Phase 4 | Complete |
| GOVN-03 | Phase 4 | Complete |
| GOVN-04 | Phase 4 | Complete |
| BTCH-01 | Phase 4 | Complete |
| BTCH-02 | Phase 4 | Complete |
| BTCH-03 | Phase 4 | Complete |
| AVTR-01 | Phase 5 | Complete |
| AVTR-02 | Phase 5 | Complete |
| AUTH-01 | Phase 6 | Complete |
| AUTH-02 | Phase 6 | Complete |
| AUTH-03 | Phase 6 | Complete |
| AUTH-04 | Phase 6 | Complete |
| BLOG-01 | Phase 7 | Pending |
| BLOG-02 | Phase 7 | Pending |
| BLOG-03 | Phase 7 | Pending |
| BLOG-04 | Phase 7 | Pending |
| BLOG-05 | Phase 7 | Pending |
| ATCH-01 | Phase 8 | Pending |
| ATCH-02 | Phase 8 | Pending |
| ATCH-03 | Phase 8 | Pending |
| ATCH-04 | Phase 8 | Pending |
| CUST-01 | Phase 9 | Pending |
| CUST-02 | Phase 9 | Pending |
| CUST-03 | Phase 9 | Pending |
| CUST-04 | Phase 9 | Pending |
| PRST-01 | Phase 10 | Pending |
| PRST-02 | Phase 10 | Pending |
| TMPL-01 | Phase 10 | Pending |
| TMPL-02 | Phase 10 | Pending |
| WTCH-01 | Phase 11 | Pending |
| WTCH-02 | Phase 11 | Pending |

**Coverage (v1.0):**
- v1.0 requirements: 42 total
- Mapped to phases: 42
- Unmapped: 0

**Coverage (v1.1):**
- v1.1 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0

---
*Requirements defined: 2026-03-20*
*Last updated: 2026-03-20 after v1.1 roadmap creation*
