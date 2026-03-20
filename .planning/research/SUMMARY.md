# Project Research Summary

**Project:** Confluence Cloud CLI (`cf`) — v1.1 Extended Capabilities
**Domain:** Go CLI tool for Confluence Cloud REST API v2 (agent-optimized) — additive extension to shipped v1.0
**Researched:** 2026-03-20
**Confidence:** HIGH

## Executive Summary

`cf` v1.1 is an incremental extension to an already-shipped Go/Cobra CLI (v1.0). The v1.0 foundation is solid: code-generated from the Confluence v2 OpenAPI spec, strict JSON-stdout contract, semantic exit codes, JQ filtering, batch operations, and API token authentication. v1.1 closes three gaps that block real-world adoption: missing content types (blog posts, attachments, custom content), missing enterprise authentication (OAuth2 2LO and 3LO), and missing agent ergonomics (output presets, content templates, watch/polling). All features are implementable with zero new Go dependencies — entirely Go stdlib — which preserves the project's clean dependency profile and mirrors the reference `jr` implementation pattern.

The recommended approach is to build in three tracks: OAuth2 authentication infrastructure first (it changes the base URL and auth model for all subsequent commands), then content types (blog posts, attachment CRUD, custom content), then agent features (presets, templates, watch). The OAuth2 track requires the most architectural care: tokens must be stored separately from config, refresh must be single-flight to avoid rotating-token race conditions, and the base URL must switch from the direct instance URL to `api.atlassian.com/ex/confluence/{cloudId}` when OAuth2 is active. These are known failure modes with clear, well-documented solutions.

The highest-risk area is attachment upload: Confluence v2 has no upload endpoint (tracked in CONFCLOUD-77196), so upload must fall back to the v1 API, which uses a different URL prefix than the v2 base URL stored in config. The fix is a `SiteRoot()` method that strips `/wiki/api/v2` from the stored base URL — but it must be implemented before any v1 endpoint integration or it causes subtly broken double-prefixed URLs. This is not theoretical: this exact bug already occurred in this codebase and was fixed in commit `a6e99ef`.

## Key Findings

### Recommended Stack

All v1.1 features require zero new Go dependencies. The existing stack (Go 1.25.8, Cobra v1.10.2, pflag v1.0.9, libopenapi v0.34.3, gojq v0.12.18) remains unchanged. New capabilities are implemented entirely with Go stdlib packages.

**New stdlib usage (no new deps):**
- `net/http`, `crypto/rand`, `crypto/sha256`, `encoding/base64` — OAuth2 client_credentials and 3LO browser flow; no `golang.org/x/oauth2` needed
- `mime/multipart`, `io`, `os` — attachment upload form-data encoding; no `go-resty` needed
- `text/template`, `embed` — content template rendering; `html/template` must NOT be used (it would escape Confluence storage format XHTML macro tags)
- `time.Ticker`, `context` — watch/polling loop with clean SIGINT shutdown
- `encoding/json` + file I/O — token persistence (`~/.config/cf/tokens/{profile}.json`) and output preset storage

**Critical config struct changes needed:**
- `AuthConfig`: add `ClientID`, `ClientSecret`, `TokenURL`, `Scopes`, `CloudID` fields for OAuth2
- `Profile`: add `Presets map[string]Preset` for named output presets
- Token file (separate from config.json): `~/.config/cf/tokens/{profile}.json` with 0600 permissions and atomic writes

**Do not use:** `golang.org/x/oauth2` (over-engineered for CLI, adds transitive deps, designed for long-running servers), `go-keyring` (CGO dependency on Linux, breaks cross-compilation), `gorilla/websocket` (Confluence has no WebSocket API), `html/template` (escapes Confluence XHTML).

### Expected Features

Research identifies a clear P1/P2/P3 split. P1 features justify the v1.1 version bump and close the content coverage and auth gaps. P2 features differentiate the tool for agent and automation users. P3 is deferred.

**Must have for v1.1 — P1 (table stakes):**
- Blog post CRUD — v2 API endpoints mirror pages exactly; 90% code reuse from `pages.go`; lowest effort among new features
- OAuth2 3LO browser flow — unblocks enterprise orgs that disable API tokens; most-requested auth gap
- Attachment list/get/delete — v2 API with cursor pagination; completes content discovery
- Attachment upload — v1 API fallback (`POST /wiki/rest/api/content/{id}/child/attachment`), multipart/form-data; no v2 equivalent exists
- Output presets — named JQ+fields combinations in config; `--preset` global flag in root.go; zero API work, enhances every command

**Should have — P2 (agent/CI differentiators):**
- OAuth2 2LO client credentials — service accounts for CI/CD pipelines and autonomous agents; zero user interaction; unique among Confluence CLI tools
- Watch command (polling + NDJSON) — reactive agent workflows without cron jobs; no other Confluence CLI has this
- Content templates (local) — Go `text/template` XHTML skeletons in `~/.config/cf/templates/`; `--template` flag on pages/blogposts create

**Defer to v1.2+ — P3:**
- Custom content CRUD — v2 API exists but niche demand (depends on which Connect apps target users have installed)
- Space-level watch via CQL — scaling concern under new Atlassian points-based rate limit (enforced March 2, 2026)

**Explicitly out of scope (anti-features to reject):**
- Markdown-to-storage-format conversion (already excluded in PROJECT.md; lossy, heavy dependency)
- Confluence template API integration (v2 has no template endpoints; v1 template API has known broken image handling)
- OAuth2 device code flow (Atlassian does not support it; 2LO covers all headless scenarios)
- Real-time WebSocket watch (Confluence has no WebSocket API; polling is the only option)
- Attachment inline preview/rendering (breaks JSON-stdout contract; agents use metadata + download separately)

**Feature dependency notes:**
- OAuth2 2LO and 3LO share infrastructure (token endpoint, dynamic base URL) — build shared foundation, then add grant types
- Blog posts and custom content reuse pages patterns (version auto-increment, body format) — factor shared helpers to avoid duplication
- Attachment upload requires v1 API path support and multipart client method before any attachment write command can be implemented
- Output presets should be built early — they enhance all existing and new commands globally with minimal effort

### Architecture Approach

v1.1 integrates three new internal packages alongside targeted modifications to existing `internal/config` and `internal/client`. The dominant architectural decision is that OAuth2 token management is resolved entirely in `PersistentPreRunE` before the Client is constructed, so the Client remains stateless with respect to token refresh — it just receives a bearer token. The existing `mergeCommand()` pattern extends the generated command tree with hand-written blogposts, attachments, and custom-content commands.

**Major components (new or modified):**
1. `internal/oauth2/` — Token acquisition (client_credentials + 3LO), single-flight refresh, file-based token store per profile; does NOT import `internal/client`; tokens flow upward to `cmd/root.go`
2. `internal/client/multipart.go` — New `DoMultipart` method on existing Client; derives site root from BaseURL via `strings.TrimSuffix(BaseURL, "/wiki/api/v2")` for v1 API path construction
3. `internal/preset/` — Load/resolve named output presets from config; pure filesystem I/O, no HTTP
4. `internal/template/` — Load Go `text/template` content templates from `~/.config/cf/templates/`; fully independent
5. `cmd/watch.go` — Long-running polling loop with `signal.NotifyContext` for clean shutdown; single goroutine, NDJSON output to stdout

**Key integration points in existing code:**
- `internal/config/config.go`: add `oauth2`, `oauth2-3lo` to `validAuthTypes`; add OAuth2 fields to `AuthConfig`
- `internal/client/client.go`: add `oauth2` case in `ApplyAuth` (maps to bearer); minimal change
- `cmd/root.go`: add `--preset` flag; add OAuth2 token resolution in `PersistentPreRunE`; add `--client-id`, `--client-secret` flag overrides
- `cmd/configure.go`: accept `--client-id`, `--client-secret`; handle `oauth2` and `oauth2-3lo` auth types

**Anti-patterns to avoid:**
- OAuth2 logic inside `Client.ApplyAuth` or `Client.Do` — puts refresh in the wrong layer; Client must stay stateless
- Separate HTTP Client instance for v1 API — duplicates auth, logging, error handling; use `DoMultipart` on the same Client
- Watch command spawning goroutines per resource — concurrent stdout writes corrupt NDJSON; use single goroutine sequential loop
- Full templating engine (Sprig, Handlebars) for content — `text/template` stdlib is sufficient for variable substitution
- Storing OAuth2 tokens in `config.json` — tokens are ephemeral, config is persistent; separate files prevent churn and race conditions

### Critical Pitfalls

The following are the highest-priority pitfalls specific to v1.1 features:

1. **Mixed v1/v2 URL prefix doubling (CRITICAL)** — `BaseURL` already contains `/wiki/api/v2`. Naively appending a v1 attachment path creates `.../wiki/api/v2/wiki/rest/api/...`. Fix: add `SiteRoot()` method: `strings.TrimSuffix(c.BaseURL, "/wiki/api/v2")`. This exact bug already occurred in this codebase (commit `a6e99ef`). Must be resolved before any v1 endpoint work starts.

2. **OAuth2 rotating refresh token race condition** — Atlassian rotates refresh tokens on use. Two sequential requests in a batch that both detect 401 and both attempt refresh: the second fails with "invalid refresh token". Fix: implement token refresh as single-flight (mutex or `golang.org/x/sync/singleflight`); proactively refresh when `AccessTokenExpiry` is within 60 seconds to avoid reactive 401s entirely.

3. **OAuth2 token storage in config.json** — Refresh tokens are long-lived secrets equivalent to passwords. Mixing them with static credentials in `config.json` risks accidental git commits and backup exposure. Fix: separate token file per profile (`~/.config/cf/tokens/{profile}.json`) with atomic writes (temp file + rename) for concurrent safety.

4. **Watch command goroutine/connection leaks** — The existing CLI is request-response; watch is a long-running loop. Without `signal.NotifyContext` and `ctx.Done()` checks between iterations, Ctrl+C leaves HTTP connections open and can produce partial NDJSON output. Fix: wire OS signals to context cancellation; write only complete JSON lines; emit `{"type":"shutdown","reason":"signal"}` on clean exit.

5. **Attachment upload breaks JSON stdout contract** — Upload response is JSON (correct); but attachment download (which users expect next) returns binary. Binary content cannot be piped through `WriteOutput` + JQ. Fix: upload uses `DoMultipart` (new method, not a hack on `Do`); download must write to a file (`--output` flag), never stdout; stdout emits JSON metadata about what was downloaded.

6. **Atlassian points-based rate limit (enforced March 2, 2026)** — Applies to OAuth2 apps; NOT to API token auth. OAuth2-authenticated watch and batch operations may exhaust the hourly quota. Fix: 429 handler must parse `Retry-After`; auto-pagination must respect delays; document bulk operation risks; recommend watch interval of 60s minimum for OAuth2-authenticated usage until per-endpoint point costs are determined empirically.

**v1.0 pitfalls that remain relevant to v1.1 work:**
- Page GET requires `?body-format=storage` — still applies to blogposts GET (same v2 behavior)
- Delete is soft-trash — same behavior for blogposts DELETE
- CQL cursor 413 on pagination page 2+ — watch command using CQL search must handle 413 gracefully

## Implications for Roadmap

Based on the dependency graph, a three-phase structure is recommended. OAuth2 must come first because it changes the base URL for all subsequent commands. Content types and agent features can follow in parallel tracks.

### Phase 1: OAuth2 Authentication Infrastructure

**Rationale:** OAuth2 changes the auth model and base URL for all API commands. Building it first ensures Phase 2 content commands can be tested with both API-token and OAuth2 auth from day one. Client credentials (2LO) is simpler than 3LO and provides immediate CI/CD value — build it first within this phase, then add 3LO.

**Delivers:** `cf configure --auth-type oauth2` (both 2LO and 3LO); transparent token refresh with expiry tracking; per-profile token store (`~/.config/cf/tokens/`); `PersistentPreRunE` OAuth2 resolution and dynamic base URL switching; updated `cmd/configure.go`.

**Addresses features:** OAuth2 2LO client credentials (P2, promoted to P1 as foundation), OAuth2 3LO browser flow (P1)

**Avoids pitfalls:** Token storage in config.json (Pitfall 9), rotating refresh token race condition (Pitfall 12), March 2026 rate limit misconfiguration for OAuth2 traffic (Pitfall 8)

**Research flag:** Standard patterns — Atlassian OAuth2 docs are authoritative and complete; reference `jr` `fetchOAuth2Token` implementation available; PKCE (S256 method) is required for 3LO and must not be skipped.

### Phase 2: Content Type Completion

**Rationale:** Blog posts reuse 90% of the pages pattern and provide rapid wins. Attachment listing (v2) is a standard fetch command. Attachment upload (v1) requires the `SiteRoot()` fix and `DoMultipart` — tackle it last within this phase to build confidence before the v1 integration.

**Delivers:** `cf blogposts` CRUD; `cf attachments` list/get/delete (v2 API); `cf attachments upload` (v1 multipart); `internal/client/multipart.go` with `DoMultipart`; `SiteRoot()` method on Client.

**Addresses features:** Blog post CRUD (P1), Attachment list/get/delete (P1), Attachment upload (P1)

**Avoids pitfalls:** v1/v2 URL prefix doubling (Pitfall 11 — implement `SiteRoot()` first, before upload subcommand), JSON stdout contract break for binary download (Pitfall 10), OpenAPI spec gaps for attachment write (Pitfall 4 — plan v1 fallback explicitly)

**Research flag:** No additional research needed — v1 attachment API is stable and well-documented; v2 blog post endpoints mirror pages exactly; `DoMultipart` is standard stdlib. Validate `X-Atlassian-Token: nocheck` behavior with OAuth2 bearer tokens during development (minor uncertainty).

### Phase 3: Agent Features

**Rationale:** Presets, templates, and watch are independent of each other and of Phase 2. They add differentiation for the AI-agent audience. Output presets are lowest effort and highest leverage (enhances all existing commands globally) — build first within this phase. Watch requires careful signal handling and is highest complexity — build last.

**Delivers:** `--preset` global flag; `cf presets` list/show commands; `cf templates` list/apply commands; `cf watch pages/blogposts` with NDJSON event streaming; built-in preset defaults (e.g., `brief`, `ids-only`, `full`).

**Addresses features:** Output presets (P1), Content templates (P2), Watch command (P2)

**Avoids pitfalls:** Watch goroutine/connection leaks (Pitfall 13 — `signal.NotifyContext` required; single goroutine loop; complete-line-only stdout writes), rate limit exhaustion during watch polling (Pitfall 8 — conservative 60s default interval for OAuth2), `html/template` escaping Confluence XHTML (use `text/template`)

**Research flag:** No research needed for presets or templates (pure config/file features with established stdlib patterns). Watch: validate safe polling interval under Atlassian points-based quota empirically (one spike test against real Confluence instance).

### Phase Ordering Rationale

- **OAuth2 before content types:** The base URL changes with OAuth2 (switches from direct instance URL to `api.atlassian.com/ex/confluence/{cloudId}`). Content commands built without OAuth2 support would need retroactive base URL handling. Building auth first lets all feature commands be tested with both auth modes from day one.
- **Blog posts before attachment upload within Phase 2:** Blog posts are pure v2, low-risk, fast to implement. Attachment upload requires the v1 URL fix and a new multipart client method. Build confidence with blog posts first, then tackle the v1 integration point.
- **Presets early in Phase 3:** Presets are global flag additions that enhance all commands (including Phase 2 commands). Building them first means watch can use presets for event formatting immediately.
- **Watch last in Phase 3:** Watch is the most architecturally different command (long-running, signal handling, NDJSON). Building it last means all supporting commands (pages GET, blogposts GET) are stable and tested.
- **Custom content deferred to v1.2:** Well-defined v2 API exists but real-world demand is uncertain. Deferring avoids scope creep; v1.1 users can validate demand before v1.2 planning.

### Research Flags

Phases with well-documented patterns (skip `/gsd:research-phase`):
- **Phase 1 (OAuth2):** Atlassian OAuth2 docs are authoritative; reference `jr` 2LO implementation is available; token store pattern is standard CLI practice (cf. `gh`, `gcloud`)
- **Phase 2 (Content types):** Blog post endpoints mirror pages exactly; v1 attachment API is stable and well-documented; `DoMultipart` is standard stdlib multipart pattern
- **Phase 3 (Agent features):** Presets and templates are pure config/file features; watch is standard `time.Ticker` + `signal.NotifyContext` pattern

Targeted validation needed (spike, not full research):
- **Phase 2 attachment upload:** Confirm `X-Atlassian-Token: nocheck` header behavior with OAuth2 bearer tokens vs. basic/token auth — one test upload against a real Confluence instance
- **Phase 3 watch:** Measure points-based rate limit consumption of polling under OAuth2 to determine safe default interval — one empirical test

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All v1.1 features verified as implementable with Go stdlib. Zero new deps confirmed against reference `jr` implementation. Atlassian OAuth2 endpoints verified against official docs. |
| Features | HIGH | v2 API endpoints for blog posts verified against official Atlassian REST API docs. v1 attachment upload confirmed as the only upload path (CONFCLOUD-77196 is an open tracked bug). OAuth2 2LO for Confluence confirmed via official service account docs. |
| Architecture | HIGH | Architecture is additive to a well-understood existing codebase with direct code inspection. New packages follow established patterns (config, client, errors). Key risks (URL prefix, token storage, refresh race) have specific, proven solutions. |
| Pitfalls | HIGH | Critical pitfalls sourced from official Atlassian docs (rate limits, token rotation, v1/v2 gap), community reports (CQL cursor 413 bug), and direct codebase inspection (URL prefix doubling in commit `a6e99ef`). |

**Overall confidence:** HIGH

### Gaps to Address

- **Atlassian rate limit point costs per endpoint:** Not published by Atlassian. Watch polling interval defaults cannot be precisely calibrated without empirical measurement. Recommend conservative 60s default; validate in Phase 3 spike test.
- **OAuth2 PKCE requirement verification:** PKCE with S256 method is documented as required for Atlassian 3LO. Must be included in authorization request (`code_challenge`, `code_challenge_method=S256`) and token exchange (`code_verifier`). Not optional — Atlassian rejects 3LO without it.
- **`X-Atlassian-Token: nocheck` with OAuth2:** Behavior of this header with OAuth2 bearer tokens (vs. basic/token auth) is not explicitly documented. Validate during Phase 2 attachment upload development.
- **Custom content type string discovery:** If custom content CRUD is added in v1.2, users must know their app's custom content type string (e.g., `ac:my-app:my-type`). There is no v2 endpoint to list registered types. Must be clearly documented as a prerequisite.

## Sources

### Primary (HIGH confidence)
- [Atlassian OAuth 2.0 3LO for Confluence Cloud](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) — authorization URL, token URL, PKCE requirement, scopes
- [Atlassian Service Account OAuth2 Credentials](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) — client_credentials grant confirmed for Confluence Cloud
- [Confluence Scopes for OAuth 2.0](https://developer.atlassian.com/cloud/confluence/scopes-for-oauth-2-3LO-and-forge-apps/) — required scopes for all v1.1 operations
- [Confluence REST API v2 — Blog Post endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-blog-post/) — CRUD endpoints and body structure
- [Confluence REST API v2 — Attachment endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-attachment/) — read-only v2 attachment operations confirmed
- [Confluence REST API v1 — Attachments](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content---attachments/) — upload endpoint, multipart format, required headers
- Reference implementation: `jira-cli-v2/internal/client/client.go` — proven `fetchOAuth2Token` pattern for client_credentials (HIGH confidence, direct code inspection)
- Existing codebase: `internal/client/client.go`, `internal/config/config.go`, `cmd/root.go` — direct code inspection, including commit `a6e99ef` (URL prefix doubling bug fix)

### Secondary (MEDIUM confidence)
- [CONFCLOUD-77196](https://jira.atlassian.com/browse/CONFCLOUD-77196) — v2 attachment upload endpoint missing (community-tracked open bug)
- [Atlassian Community: CQL cursor 413 issue](https://community.developer.atlassian.com/) — 11,000-character cursor values causing 413 errors on paginated search (reported Sep 2025)
- [Evolving API rate limits — Atlassian blog](https://www.atlassian.com/blog/platform/evolving-api-rate-limits) — points-based quota enforced March 2, 2026; applies to OAuth2 apps
- [pchuri/confluence-cli](https://github.com/pchuri/confluence-cli), [atlassian-python-api](https://github.com/atlassian-api/atlassian-python-api), [confluence-go-api](https://github.com/virtomize/confluence-go-api) — competitor feature comparison

### Tertiary (LOW confidence)
- `X-Atlassian-Token: nocheck` behavior with OAuth2 bearer tokens — not explicitly documented; requires empirical validation
- Per-endpoint rate limit point costs — not published by Atlassian; requires empirical measurement

---
*Research completed: 2026-03-20*
*Ready for roadmap: yes*
