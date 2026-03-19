# Project Research Summary

**Project:** Confluence Cloud CLI (`cf`)
**Domain:** REST API CLI tool with OpenAPI code generation — optimized for AI agent consumption
**Researched:** 2026-03-20
**Confidence:** HIGH

## Executive Summary

`cf` is a Go CLI tool that exposes the full Confluence Cloud v2 REST API as shell commands, with AI agents as the primary consumer. Research establishes a clear, well-validated architecture based on a direct reference implementation (`jira-cli-v2`, referred to as `jr` throughout) — the design is effectively a port of that tool adapted for the Confluence v2 API and OpenAPI spec. The core differentiator is a code-generation pipeline (`gen/`) that reads the Confluence OpenAPI spec and produces Cobra command files for every endpoint automatically, ensuring complete API coverage without manual maintenance. All output is pure JSON to stdout, with structured JSON errors to stderr and semantic exit codes — a contract that makes the tool usable by AI agents without parsing human-readable text.

The recommended approach is to build the project in three natural layers that mirror the architecture's dependency graph: (1) core scaffolding — errors, config, HTTP client, and CLI skeleton; (2) the code generation pipeline plus the generated commands for the primary resources (pages, spaces, search); and (3) governance and agent-optimization features (operation policy, audit logging, caching, batch operations, presets). This order is enforced by hard dependencies: nothing works without the HTTP client, and no generated commands exist until the generator runs. Stack choices are constrained and well-validated — Go 1.26, Cobra v1.10.2, `pb33f/libopenapi` for spec parsing, `gojq` for JQ filtering, and stdlib `net/http` for the HTTP client.

The highest-risk areas are Confluence v2 API behaviors that differ subtly from expectations: page bodies are not returned by default on GET, DELETE is a soft trash (not permanent), page updates require version increment via optimistic locking, and spaces use numeric IDs not human-readable keys. These are API-level traps that require explicit handling in the command implementations, not architectural problems. The CQL search cursor length issue (producing 413 errors on page 2+) is the only infrastructure-level risk that may require a workaround rather than a clean fix.

## Key Findings

### Recommended Stack

The stack is entirely constrained by the reference implementation (`jr`) and the project mandate (Go + Cobra). No decision-making is required — copy dependency versions directly from `jr`'s `go.mod` with minor version upgrades to current stable releases. The only new dependency relative to `jr` is `pb33f/libopenapi` (for the `gen/` code generator); everything else is identical.

The `gen/` binary uses `libopenapi` to parse the OpenAPI spec but is a standalone program never imported by the compiled CLI binary. This means libopenapi's transitive dependencies (yaml, ordered-map) do not affect the CLI binary size or build. The CLI itself depends only on Cobra, pflag, gojq, and tidwall/pretty — a minimal, well-audited set.

**Core technologies:**
- Go 1.26.1 — implementation language; single-binary, cross-compile, no runtime dependencies
- github.com/spf13/cobra v1.10.2 — CLI framework; command tree, persistent flags, shell completions
- github.com/pb33f/libopenapi v0.34.3 — OpenAPI spec parsing in `gen/`; exposes typed model for paths and parameters
- github.com/itchyny/gojq v0.12.18 — pure-Go jq filtering embedded in binary; `--jq` flag on every command
- github.com/tidwall/pretty v1.2.1 — fast JSON pretty-print without unmarshaling; `--pretty` flag
- stdlib net/http — HTTP client; no additional HTTP library needed

**Do not use:** oapi-codegen (generates server stubs, not CLI commands), spf13/viper (20+ transitive deps for simple config needs), go-resty or cleanhttp (obscures headers/timeouts), logrus/zerolog (structured logging adds deps; use encoding/json + fmt.Fprintf directly).

### Expected Features

The feature set divides cleanly into: (P1) table stakes that an AI agent cannot function without, (P2) governance and agent-optimization features that add significant value after core CRUD works, and (P3) deferred features not essential for agent use cases.

**Must have — table stakes (P1):**
- Profile/config system (`cf configure`, `~/.config/cf/config.json`, `--profile`, `CF_*` env vars) — nothing else works without auth
- OpenAPI codegen pipeline — generates all CRUD commands from spec; the architectural foundation
- Pages CRUD (get, create, update, delete) — primary resource; required for real Confluence work
- Space list/get — discovery layer before page operations
- CQL search — agents find pages without knowing IDs upfront
- Pure JSON stdout + structured JSON errors to stderr — non-negotiable for agent consumption
- Semantic exit codes (0=ok, 1=error, 2=auth, 3=not_found, 4=validation, 5=rate_limit, 6=conflict, 7=server)
- Automatic pagination with cursor-based following
- JQ filtering (`--jq`) on all output
- Raw API passthrough command (`cf raw`)
- Schema/help as JSON (`cf schema`)
- Dry-run mode (`--dry-run`)

**Should have — competitive differentiators (P2):**
- Operation policy (allow/deny lists per profile) — agent governance in shared environments
- Audit logging (NDJSON, append-only) — accountability for agent actions
- Response caching for GET with TTL — reduces quota pressure in agent context-gathering loops
- Preset system (named `--jq` + `--fields` combinations)
- Batch command (JSON array of commands executed in one invocation)
- Comments and label management via generated commands

**Defer to v2+:**
- OAuth2 browser/device flow — only for human-interactive scenarios; API tokens cover all agent cases
- Watch/polling command — reactive agents are a future use case
- Blog post CRUD, space permissions management, content version history

### Architecture Approach

The architecture is a clean three-layer design: a `gen/` standalone program that reads the OpenAPI spec and writes Go source files into `cmd/generated/`; a `cmd/` layer with a root command (client injection via `PersistentPreRunE`), generated commands, hand-written workflow wrappers, and utility commands; and an `internal/` layer of unexported packages (client, config, errors, cache, jq, policy, audit, preset). The generated code is committed to the repo — the generator is a development tool, not a build step. All commands receive the HTTP client via `cmd.Context()`, never via global variables.

**Major components:**
1. `gen/` (parser → grouper → generator + templates) — reads `spec/confluence-v2.json`, writes `cmd/generated/*.go`; standalone, never imported at runtime
2. `cmd/root.go` — `PersistentPreRunE` builds and context-injects `client.Client`; only place config resolution happens
3. `cmd/generated/` — auto-generated Cobra commands, one file per OpenAPI resource tag; `RegisterAll()` mounts all to root; marked DO NOT EDIT
4. `cmd/workflow.go` — hand-written multi-step commands (page create, page update, search) that use `client.Fetch()` for intermediate calls and `WriteOutput()` once at the end
5. `internal/client` — `Do()` (execute + output) vs `Fetch()` (execute + return body); handles pagination, auth, cache, jq, audit, policy
6. `internal/config` — config file + env var + flag resolution in strict priority order; `CF_` prefix for env vars
7. `internal/errors` — `APIError`, `AlreadyWrittenError` sentinel, semantic exit code map; prevents double-writing errors

### Critical Pitfalls

1. **Page GET returns empty body by default** — v2 API omits `body` unless `?body-format=storage` is passed. The `pages get` command must inject this query param by default; generated commands need it as a flag with a `storage` default. Verify with integration test asserting `body.storage.value` is non-empty.

2. **Page UPDATE requires version increment via optimistic locking** — `PUT /pages/{id}` with stale `version.number` returns 409. The `pages update` workflow command must always GET current version first, then send `current + 1`. Map 409 to `ExitConflict` (6) with structured error including resource ID and version hint.

3. **DELETE is soft-delete (trash), not permanent** — `DELETE /pages/{id}` moves to trash. Agents that delete-and-recreate the same page title loop on title conflict. The `pages delete` command must: output `"trashed": true`, provide `--purge` flag, and map 403 on purge to a structured error with admin hint.

4. **Space identifier is numeric ID, not key** — v2 API uses integer space IDs everywhere. The `spaces` command must expose `--key` that transparently resolves via `GET /spaces?keys=<key>` and cache the key→ID mapping. Without this, users passing `--space ENG` receive confusing 404 errors.

5. **OpenAPI spec has known gaps** — Atlassian's `openapi-v2.v3.json` is incomplete: missing types, some attachment write operations are v1-only. Commit the spec to `spec/` at project start, run code gen in CI to catch compile errors early, and maintain a `SPEC_GAPS.md` for known omissions.

6. **CQL cursor length causes 413 on pagination page 2+** — As of September 2025, cursor values can be ~11,000 characters. Detect 413 in the pagination handler and surface as `pagination_error` with a hint. This is a known unfixed Atlassian bug.

7. **Binary name `cf` collides with Cloud Foundry CLI** — Many CI environments have `cf` on `$PATH` for Cloud Foundry. Document the collision explicitly; `cf --version` as a verification step.

## Implications for Roadmap

Based on the architecture's dependency graph and the feature dependency tree, a four-phase structure is recommended:

### Phase 1: Core Scaffolding and HTTP Client

**Rationale:** Everything in the project depends on the HTTP client being correct. Errors, config, and client are the foundation — nothing else compiles or runs without them. Rate limit handling, HTML error sanitization, auth, and config file permissions (0600) must be correct before any command code is written, because every command inherits these behaviors from the client.

**Delivers:** A working CLI skeleton with `cf configure`, `cf --version` (JSON output), and a `cf raw` command that can make any authenticated API call.

**Addresses features:** Profile/config system, Basic + Bearer auth, semantic exit codes, JQ filtering, dry-run mode, verbose mode, pure JSON stdout contract.

**Avoids pitfalls:** Rate limit 429 handling (client setup phase), HTML error response sanitization (mirror `jr`'s `sanitizeBody`), config file 0600 permissions, cache file permissions, `AlreadyWrittenError` sentinel pattern.

**Needs research-phase:** No — well-documented patterns from reference implementation.

### Phase 2: OpenAPI Code Generation Pipeline

**Rationale:** The code generator must be built and validated before any resource commands can exist. Generated commands are the primary delivery mechanism for all CRUD operations. The generator also forces confrontation with the OpenAPI spec's gaps early — compile errors in CI catch type issues before they affect command implementations.

**Delivers:** The `gen/` pipeline (parser, grouper, generator, templates) producing `cmd/generated/` with page, space, and core resource commands. The Makefile `generate` and `spec-update` targets. A committed `spec/confluence-v2.json`. A `SPEC_GAPS.md` documenting known omissions.

**Addresses features:** OpenAPI-generated commands (core differentiator), `RegisterAll` command registration, schema introspection command.

**Avoids pitfalls:** OpenAPI spec incompleteness (commit spec, run gen in CI), `body-format=storage` injection in page commands, generator template correctness.

**Needs research-phase:** Possibly — libopenapi's API for walking paths and extracting parameters should be validated against the actual Confluence spec structure before writing the generator.

### Phase 3: Pages, Spaces, and Search

**Rationale:** These are the P1 table stakes that make the tool actually usable for AI agents. Pages and spaces are the primary resources; CQL search is the discovery mechanism. All three have Confluence-specific API behaviors (optimistic locking, numeric IDs, cursor pagination, body-format) that require hand-written workflow wrappers on top of the generated commands.

**Delivers:** Working `cf pages` commands (get with body, create, update with version increment, delete with purge flag), `cf spaces` commands (list, get with `--key` resolution), `cf search` with CQL and auto-pagination, and the `raw` passthrough command.

**Addresses features:** Pages CRUD, space list/get, CQL search, automatic pagination, `--fields` sparse fieldset.

**Avoids pitfalls:** Empty body on GET (inject `body-format=storage`), version conflict on update (GET-before-write), soft-delete confusion (`--purge` flag, `"trashed": true` output), space key vs ID (transparent `--key` resolution with cache), CQL cursor 413 (detect and surface as `pagination_error`).

**Needs research-phase:** No — all behaviors are documented in PITFALLS.md with specific prevention strategies.

### Phase 4: Governance and Agent Optimization

**Rationale:** Once core CRUD is working and agents can interact with Confluence, the governance and optimization features add significant value for production deployments. Operation policy and audit logging are tightly coupled to the profile system and belong together. Caching, batch, and presets build on a working client and are safe to defer until the core is validated.

**Delivers:** Operation policy (allow/deny lists per profile), audit logging (NDJSON), response caching with TTL, preset system (named `--jq` + `--fields`), batch command.

**Addresses features:** Operation policy, audit logging, response caching, preset system, batch operations, comments CRUD, label management.

**Avoids pitfalls:** Policy must be enforced even in dry-run (not a bypass). Cache keys must be scoped per-profile to prevent cross-profile cache hits. Space key→ID cache integrates here.

**Needs research-phase:** No for policy/audit/cache (patterns from `jr`). Possibly for batch — the dispatch mechanism against the command registry needs design validation.

### Phase Ordering Rationale

- Phase 1 before Phase 2: The client is a dependency of every generated command; must be correct first.
- Phase 2 before Phase 3: Generated commands do not exist until the generator runs; the workflow wrappers in Phase 3 sit on top of them.
- Phase 3 before Phase 4: Operation policy is meaningless without commands to apply it to; caching requires real API calls to validate TTL behavior; presets require established `--jq` patterns to name.
- Phase 4 after validation: The feature dependency tree in FEATURES.md shows that policy, audit, and presets all require the profile system (Phase 1) and generated commands (Phase 2) as prerequisites.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (Code Generation):** The libopenapi v0.34.3 API for walking OpenAPI paths, extracting parameter types, and generating Go-compatible identifiers against the actual Confluence spec shape should be validated before writing generator templates. The spec has known gaps that may affect generator design.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Core Scaffolding):** Patterns are directly copied from `jr`. No novel decisions.
- **Phase 3 (Pages/Spaces/Search):** All API behaviors are documented in PITFALLS.md with specific prevention strategies and integration test requirements.
- **Phase 4 (Governance):** Policy, audit, cache, and preset patterns are all present in `jr` and can be ported directly.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All library versions verified against pkg.go.dev; exact copies from reference implementation with minor version bumps |
| Features | HIGH | Reference implementation examined directly; Confluence v2 API surveyed; competitor CLIs reviewed |
| Architecture | HIGH | Derived directly from reference implementation code analysis; component boundaries and patterns verified |
| Pitfalls | HIGH | Majority verified against official Atlassian docs and community reports; CQL 413 issue reported Sep 2025 |

**Overall confidence:** HIGH

### Gaps to Address

- **libopenapi API shape for Confluence spec:** The generator templates need to be designed around what libopenapi actually exposes for the Confluence spec's specific structure. Recommend a spike at the start of Phase 2 to parse the spec and log what the model exposes before committing to template design.

- **Attachment commands:** v2 API is read-only for attachments; write operations require v1 endpoints. This needs a decision during Phase 3 planning: implement v1 attachment write as a special case, or defer attachments entirely and document in `SPEC_GAPS.md`.

- **golangci-lint version:** Confirmed as v2.11.3 from WebSearch only (MEDIUM confidence). Verify against the official releases page at project start.

- **Binary naming final decision:** The `cf` name collision with Cloud Foundry CLI is documented as a known issue with no code change required. If the project determines the collision is unacceptable in target environments, the binary name must be changed before any release — this cannot be changed after users have scripts.

## Sources

### Primary (HIGH confidence)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` (reference implementation) — stack, architecture, and pattern decisions
- [pkg.go.dev/github.com/spf13/cobra](https://pkg.go.dev/github.com/spf13/cobra) — v1.10.2 confirmed
- [pkg.go.dev/github.com/pb33f/libopenapi](https://pkg.go.dev/github.com/pb33f/libopenapi) — v0.34.3 confirmed
- [pkg.go.dev/github.com/itchyny/gojq](https://pkg.go.dev/github.com/itchyny/gojq) — v0.12.18 confirmed
- [go.dev/doc/devel/release](https://go.dev/doc/devel/release) — Go 1.26.1 current stable
- [Confluence Cloud REST API v2 — Atlassian Developer](https://developer.atlassian.com/cloud/confluence/rest/v2/) — endpoint and parameter behaviors
- [Confluence rate limiting — official docs](https://developer.atlassian.com/cloud/confluence/rate-limiting/) — 429 and Retry-After
- [Delete, restore, or purge — Confluence Cloud support](https://support.atlassian.com/confluence-cloud/docs/delete-restore-or-purge-a-page/) — soft-delete behavior
- [Get Body of a Page through API v2 — Developer Community](https://community.developer.atlassian.com/t/get-body-of-a-page-through-api-v2/67966) — empty body without body-format
- [Confluence Cloud API v2 Space ID vs Key — Community](https://community.atlassian.com/forums/Confluence-questions/How-to-get-space-ID-on-the-UI-or-how-to-utilise-space-key-in-v2/qaq-p/2680647) — numeric ID requirement

### Secondary (MEDIUM confidence)
- [CQL cursor 413 issue — Developer Community (Sep 2025)](https://community.developer.atlassian.com/t/confluence-rest-v1-search-endpoint-fails-cursor-of-next-url-is-extraordinarily-long-leading-to-413-error/95098) — cursor length bug
- [OpenAPI spec incomplete — Community report](https://community.atlassian.com/forums/Confluence-questions/OpenAPI-specification-seems-incomplete/qaq-p/2570847) — spec gaps confirmed
- [Evolving API rate limits — Atlassian blog](https://www.atlassian.com/blog/platform/evolving-api-rate-limits) — March 2026 OAuth2 points-based quota
- [pchuri/confluence-cli — GitHub](https://github.com/pchuri/confluence-cli) — competitor feature comparison
- [open-cli-collective/atlassian-cli — GitHub](https://github.com/open-cli-collective/atlassian-cli) — competitor feature comparison
- [Why CLI Tools Are Beating MCP for AI Agents — Jan Reinhard, 2026](https://jannikreinhard.com/2026/02/22/why-cli-tools-are-beating-mcp-for-ai-agents/) — CLI vs MCP for agents

### Tertiary (LOW confidence)
- [github.com/golangci/golangci-lint/releases](https://github.com/golangci/golangci-lint/releases) — v2.11.3 from WebSearch only; verify at project start

---
*Research completed: 2026-03-20*
*Ready for roadmap: yes*
