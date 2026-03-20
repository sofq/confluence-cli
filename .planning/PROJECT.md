# Confluence CLI (`cf`)

## What This Is

A command-line interface for Confluence Cloud's v2 REST API, mirroring the architecture of the existing Jira CLI (`jr`). Built in Go with Cobra, it auto-generates commands from the Confluence OpenAPI spec and provides hand-written workflow wrappers for common operations. Supports OAuth2, all content types (pages, blog posts, attachments, custom content), output presets, content templates, and long-running content monitoring. Primary audience is AI agents that need structured, JSON-based access to Confluence content.

## Core Value

Give AI agents and automation reliable, structured access to Confluence content through a CLI that outputs pure JSON and supports JQ filtering — enabling programmatic read/write of pages, spaces, and content without browser interaction.

## Requirements

### Validated

- [x] Auto-generated commands from Confluence v2 OpenAPI spec — v1.0
- [x] Pages CRUD (create, read, update, delete) — v1.0
- [x] CQL search across spaces and content — v1.0
- [x] Space listing and management — v1.0
- [x] Comments on content (create, read, delete) — v1.0
- [x] Label management on content — v1.0
- [x] Multi-auth support (basic/bearer/oauth2/oauth2-3lo) with profiles — v1.0, v1.1
- [x] JQ filtering on all JSON output — v1.0
- [x] Raw API command for unmapped endpoints — v1.0
- [x] Pure JSON stdout for agent consumption — v1.0
- [x] Structured error output with semantic exit codes — v1.0
- [x] Configuration profiles (~/.config/cf/config.json) — v1.0
- [x] Response caching for GET requests — v1.0
- [x] Pagination handling for list endpoints — v1.0
- [x] Raw Confluence storage format for page content (no conversion) — v1.0
- [x] Operation policy enforcement — v1.0
- [x] NDJSON audit logging — v1.0
- [x] Batch execution — v1.0
- [x] Avatar writing style analysis — v1.0
- [x] OAuth2 client credentials (2LO) + browser flow (3LO) with PKCE — v1.1
- [x] Automatic OAuth2 token refresh — v1.1
- [x] Secure per-profile token storage (0600 perms) — v1.1
- [x] Blog post CRUD operations — v1.1
- [x] Attachment upload (v1 multipart) and management — v1.1
- [x] Custom content type operations — v1.1
- [x] Watch command for polling content changes (NDJSON events) — v1.1
- [x] Output presets (named JQ shortcuts) — v1.1
- [x] Template system for content creation — v1.1

### Active

(None — planning next milestone)

### Out of Scope

- Markdown ↔ storage format conversion — adds complexity, agents can handle raw format
- Mobile/desktop app — CLI only
- Real-time collaboration features — not applicable to CLI
- Content rendering/preview — agents consume structured data, not rendered HTML
- Confluence v1 API full support — v2 primary, v1 used only for search/labels/attachments where v2 lacks endpoints

## Context

- **Reference implementation**: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` — architecture mirrored exactly
- **API spec**: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` (Confluence Cloud v2 REST API, OpenAPI 3.0.3)
- **Content format**: Confluence uses Atlassian Storage Format (XHTML-based) — cf passes this through as-is
- **Primary users**: AI agents (Claude, etc.) that need structured Confluence access
- **Binary name**: `cf` (matches `jr` pattern)
- **Config prefix**: `CF_` for environment variables (matches `JR_` pattern)
- **Codebase**: ~34,000 LOC Go, 14 internal packages, 212 generated operations
- **Live tested**: Confirmed working against Confluence Cloud (quanhh.atlassian.net) on 2026-03-20

## Constraints

- **Language**: Go — matches jr, ensures single binary distribution
- **CLI framework**: Cobra (spf13/cobra) — matches jr
- **API version**: Confluence Cloud v2 primary (v1 for search, labels, attachment upload)
- **Output format**: Pure JSON to stdout, errors to stderr — agent-friendly
- **Architecture**: Code generation from OpenAPI spec + hand-written wrappers — matches jr exactly
- **Dependencies**: Zero new Go deps in v1.1 — all features use stdlib only

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Mirror jr architecture exactly | Proven patterns, shared mental model, consistent tooling | ✓ Good |
| Raw storage format only | Agents handle raw format fine, avoids conversion complexity | ✓ Good |
| Confluence v2 API only (v1 for gaps) | Cleaner API design, better long-term support from Atlassian | ✓ Good |
| AI agent as primary user | Drives design toward structured JSON output, semantic exit codes | ✓ Good |
| OAuth2 token in PersistentPreRunE | Client stays stateless, receives bearer token | ✓ Good |
| No TokenURL config field | Atlassian uses single fixed endpoint — constant, not configurable | ✓ Good |
| PKCE included defensively | OAuth 2.1 recommends even though Atlassian doesn't enforce | ✓ Good |
| searchV1Domain for v1 API | Reusable domain extraction avoids URL doubling bug | ✓ Good |
| CQL lastModified + client-side dedup | Date-only granularity in CQL requires timestamp comparison | ✓ Good |
| map[string]string for template data | Prevents SSTI — no struct access from templates | ✓ Good |

---
*Last updated: 2026-03-20 after v1.1 milestone completion*
