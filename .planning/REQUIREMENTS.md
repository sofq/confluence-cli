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

- [ ] **CGEN-01**: CLI auto-generates Cobra commands from Confluence v2 OpenAPI spec
- [ ] **CGEN-02**: Generator groups operations by resource (pages, spaces, search, etc.)
- [ ] **CGEN-03**: Generated commands include all path/query/body parameters from spec
- [ ] **CGEN-04**: Hand-written workflow commands can override generated commands via `mergeCommand`
- [x] **CGEN-05**: Spec is pinned locally at `spec/confluence-v2.json` with known gaps documented

### Pages

- [ ] **PAGE-01**: User can get a page by ID with content body (storage format)
- [ ] **PAGE-02**: User can create a page in a space with title and storage format body
- [ ] **PAGE-03**: User can update a page with automatic version increment (handles 409 conflicts)
- [ ] **PAGE-04**: User can delete a page (soft-delete to trash)
- [ ] **PAGE-05**: User can list pages in a space with pagination

### Spaces

- [ ] **SPCE-01**: User can list all spaces with pagination
- [ ] **SPCE-02**: User can get space details by ID
- [ ] **SPCE-03**: CLI transparently resolves space keys to numeric IDs where needed

### Search

- [ ] **SRCH-01**: User can search content via CQL with `cf search --cql "<query>"`
- [ ] **SRCH-02**: Search results are automatically paginated and merged
- [ ] **SRCH-03**: Search handles long cursor strings without 413 errors

### Comments

- [ ] **CMNT-01**: User can list comments on a page
- [ ] **CMNT-02**: User can create a comment on a page (storage format body)
- [ ] **CMNT-03**: User can delete a comment

### Labels

- [ ] **LABL-01**: User can list labels on content
- [ ] **LABL-02**: User can add labels to content
- [ ] **LABL-03**: User can remove labels from content

### Governance

- [ ] **GOVN-01**: User can configure allowed/denied operations per profile (glob patterns)
- [ ] **GOVN-02**: Policy is enforced pre-request, even in dry-run mode
- [ ] **GOVN-03**: Every API call is logged to NDJSON audit file with timestamp, profile, operation, method, path, status
- [ ] **GOVN-04**: Audit logging is configurable per-profile or per-invocation via `--audit` flag

### Batch

- [ ] **BTCH-01**: User can execute multiple operations from JSON array input via `cf batch`
- [ ] **BTCH-02**: Batch output is JSON array with per-operation exit codes and data/error
- [ ] **BTCH-03**: Batch supports partial failure (some ops succeed, some fail)

### Avatar

- [ ] **AVTR-01**: User can analyze a Confluence user's writing style from their content
- [ ] **AVTR-02**: Avatar analysis outputs structured JSON persona profile for AI agent consumption

## v2 Requirements

### Enhanced Auth

- **AUTH-01**: OAuth2 client credentials grant support
- **AUTH-02**: OAuth2 browser flow for interactive use

### Content Types

- **CONT-01**: Blog post CRUD operations
- **CONT-02**: Attachment upload and management
- **CONT-03**: Custom content type operations

### Advanced Features

- **ADVN-01**: Watch command for polling content changes (NDJSON events)
- **ADVN-02**: Output presets (named JQ + fields combinations)
- **ADVN-03**: Template system for content creation

## Out of Scope

| Feature | Reason |
|---------|--------|
| Markdown ↔ Storage Format conversion | Lossless round-tripping not achievable; agents handle raw format |
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
| CGEN-01 | Phase 2 | Pending |
| CGEN-02 | Phase 2 | Pending |
| CGEN-03 | Phase 2 | Pending |
| CGEN-04 | Phase 2 | Pending |
| CGEN-05 | Phase 2 | Complete |
| PAGE-01 | Phase 3 | Pending |
| PAGE-02 | Phase 3 | Pending |
| PAGE-03 | Phase 3 | Pending |
| PAGE-04 | Phase 3 | Pending |
| PAGE-05 | Phase 3 | Pending |
| SPCE-01 | Phase 3 | Pending |
| SPCE-02 | Phase 3 | Pending |
| SPCE-03 | Phase 3 | Pending |
| SRCH-01 | Phase 3 | Pending |
| SRCH-02 | Phase 3 | Pending |
| SRCH-03 | Phase 3 | Pending |
| CMNT-01 | Phase 3 | Pending |
| CMNT-02 | Phase 3 | Pending |
| CMNT-03 | Phase 3 | Pending |
| LABL-01 | Phase 3 | Pending |
| LABL-02 | Phase 3 | Pending |
| LABL-03 | Phase 3 | Pending |
| GOVN-01 | Phase 4 | Pending |
| GOVN-02 | Phase 4 | Pending |
| GOVN-03 | Phase 4 | Pending |
| GOVN-04 | Phase 4 | Pending |
| BTCH-01 | Phase 4 | Pending |
| BTCH-02 | Phase 4 | Pending |
| BTCH-03 | Phase 4 | Pending |
| AVTR-01 | Phase 5 | Pending |
| AVTR-02 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 42 total
- Mapped to phases: 42
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-20*
*Last updated: 2026-03-20 after initial definition*
