# Confluence CLI (`cf`)

## What This Is

A command-line interface for Confluence Cloud's v2 REST API, mirroring the architecture of the existing Jira CLI (`jr`). Built in Go with Cobra, it auto-generates commands from the Confluence OpenAPI spec and provides hand-written workflow wrappers for common operations. Primary audience is AI agents that need structured, JSON-based access to Confluence content.

## Core Value

Give AI agents and automation reliable, structured access to Confluence content through a CLI that outputs pure JSON and supports JQ filtering — enabling programmatic read/write of pages, spaces, and content without browser interaction.

## Requirements

### Validated

(None yet — ship to validate)

### Validated

- [x] Auto-generated commands from Confluence v2 OpenAPI spec — v1.0
- [x] Pages CRUD (create, read, update, delete) — v1.0
- [x] CQL search across spaces and content — v1.0
- [x] Space listing and management — v1.0
- [x] Comments on content (create, read, delete) — v1.0
- [x] Label management on content — v1.0
- [x] Multi-auth support (basic/bearer/oauth2/oauth2-3lo) with profiles — v1.0, v1.1 Phase 6
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

### Active

- [x] OAuth2 client credentials grant support — v1.1 Phase 6
- [x] OAuth2 browser flow for interactive use — v1.1 Phase 6
- [x] Automatic OAuth2 token refresh — v1.1 Phase 6
- [x] Secure per-profile token storage (0600 perms) — v1.1 Phase 6
- [x] Blog post CRUD operations — v1.1 Phase 7
- [x] Attachment upload and management — v1.1 Phase 8
- [ ] Custom content type operations
- [ ] Watch command for polling content changes (NDJSON events)
- [ ] Output presets (named JQ + fields combinations)
- [ ] Template system for content creation

### Out of Scope

- Markdown ↔ storage format conversion — adds complexity, agents can handle raw format
- Mobile/desktop app — CLI only
- Real-time collaboration features — not applicable to CLI
- Content rendering/preview — agents consume structured data, not rendered HTML
- Confluence v1 API support — v2 only for cleaner, modern endpoints

## Current Milestone: v1.1 Extended Capabilities

**Goal:** Add OAuth2 authentication, content type coverage (blogs, attachments, custom types), and advanced agent features (watch, output presets, content templates).

**Target features:**
- OAuth2 client credentials grant and browser-based interactive flow
- Blog post CRUD operations
- Attachment upload and management
- Custom content type operations
- Watch command for polling content changes (NDJSON events)
- Output presets (named JQ + fields combinations)
- Template system for content creation

## Context

- **Reference implementation**: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` — mirror this architecture exactly
- **API spec**: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` (Confluence Cloud v2 REST API, OpenAPI 3.0.3)
- **Content format**: Confluence uses Atlassian Storage Format (XHTML-based) — cf passes this through as-is
- **Primary users**: AI agents (Claude, etc.) that need structured Confluence access
- **Binary name**: `cf` (matches `jr` pattern)
- **Config prefix**: `CF_` for environment variables (matches `JR_` pattern)

## Constraints

- **Language**: Go — matches jr, ensures single binary distribution
- **CLI framework**: Cobra (spf13/cobra) — matches jr
- **API version**: Confluence Cloud v2 only
- **Output format**: Pure JSON to stdout, errors to stderr — agent-friendly
- **Architecture**: Code generation from OpenAPI spec + hand-written wrappers — matches jr exactly

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Mirror jr architecture exactly | Proven patterns, shared mental model, consistent tooling | — Pending |
| Raw storage format only | Agents handle raw format fine, avoids conversion complexity | — Pending |
| Confluence v2 API only | Cleaner API design, better long-term support from Atlassian | — Pending |
| AI agent as primary user | Drives design toward structured JSON output, semantic exit codes | — Pending |

---
*Last updated: 2026-03-20 after Phase 8 completion*
