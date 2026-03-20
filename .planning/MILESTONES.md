# Milestones: Confluence CLI (`cf`)

## v1.1 Extended Capabilities (Shipped: 2026-03-20)

**Phases:** 6–11 (6 phases, 8 plans)
**Requirements:** 23/23 complete

### What shipped
- Enhanced Auth: OAuth2 client credentials (2LO) + browser flow (3LO) with PKCE, automatic token refresh, secure per-profile token storage
- Blog Posts: Full CRUD mirroring pages pattern with version auto-increment
- Attachments: v2 list/get/delete + v1 multipart upload with XSRF bypass
- Custom Content: CRUD for Connect/Forge app content types with --type flag
- Output Presets: Per-profile named JQ shortcuts via --preset flag
- Content Templates: File-based Go text/template with --var variable substitution
- Watch: Long-running CQL polling with NDJSON event streaming and graceful shutdown

---

## v1.0 — Core CLI

**Completed:** 2026-03-20
**Phases:** 1–5 (5 phases, 16 plans)
**Requirements:** 42/42 complete

### What shipped

- Infrastructure: Pure JSON output, auth profiles (basic/bearer), JQ filtering, caching, pagination, dry-run, verbose, raw API, schema
- Code Generation: Full OpenAPI → Cobra pipeline (212 operations)
- Content Ops: Pages CRUD, spaces, CQL search, comments, labels
- Governance: Operation policy, audit logging, batch execution
- Avatar: Writing style analysis with JSON persona profiles

---
*Last updated: 2026-03-20*
