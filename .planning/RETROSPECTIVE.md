# Retrospective: Confluence CLI (`cf`)

## Milestone: v1.1 — Extended Capabilities

**Shipped:** 2026-03-20
**Phases:** 6 | **Plans:** 8

### What Was Built
- OAuth2 2LO + 3LO with PKCE, auto-refresh, secure token storage
- Blog post CRUD mirroring pages pattern
- Attachment list/get/upload (v1 multipart)/delete
- Custom content CRUD with --type flag
- Output presets (per-profile JQ shortcuts)
- Content templates with variable substitution
- Watch command with CQL polling and NDJSON events

### What Worked
- Mirroring pages.go for blogposts/custom-content was extremely fast — copy-adapt pattern
- Single-plan phases for straightforward CRUD reduced overhead significantly
- Project-level research (PITFALLS.md) caught the URL doubling bug and XSRF header before they became issues
- Live testing against real Confluence Cloud validated all features end-to-end

### What Was Inefficient
- Cobra singleton flag contamination caused test suite failures across phases 8-10; each fix was ad-hoc
- Some verifier gaps_found results were test isolation issues, not real gaps

### Patterns Established
- v1 API pattern via searchV1Domain() + fetchV1() reused across search, labels, attachments
- mergeCommand for all CRUD resources (pages, blogposts, attachments, custom-content)
- Per-profile config extensions (presets map, OAuth2 fields) follow established pattern
- rootCmd.AddCommand for non-generated commands (avatar, watch, templates)

### Key Lessons
- Pass explicit flag values (--dry-run=false, --jq "", --preset "") in tests to counter Cobra singleton contamination
- CQL lastModified has date-only granularity — always need client-side timestamp comparison
- Zero new dependencies possible for all v1.1 features — Go stdlib is sufficient

---

## Milestone: v1.0 — Core CLI

**Shipped:** 2026-03-20
**Phases:** 5 | **Plans:** 16

### What Was Built
- Full CLI infrastructure, OpenAPI code generation, pages/spaces/search/comments/labels CRUD, governance, avatar analysis

---

## Cross-Milestone Trends

| Metric | v1.0 | v1.1 |
|--------|------|------|
| Phases | 5 | 6 |
| Plans | 16 | 8 |
| Go LOC | ~22k | ~34k |
| Avg plans/phase | 3.2 | 1.3 |

**Trend:** v1.1 phases were simpler (1-2 plans each) because they built on established patterns from v1.0.
