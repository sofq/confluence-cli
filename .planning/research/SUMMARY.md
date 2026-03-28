# Project Research Summary

**Project:** Confluence CLI v1.2 — Workflow, Parity & Release Infrastructure
**Domain:** Go CLI tool expansion with API workflow commands, content utilities, and release pipeline
**Researched:** 2026-03-28
**Confidence:** HIGH

## Executive Summary

The v1.2 milestone extends an already-functioning Confluence Cloud CLI (cf) built on Go + Cobra. The project has a verified reference implementation — the Jira CLI (jr) — that has already solved every feature in scope: workflow commands, version diff, export, built-in presets, built-in templates, GoReleaser, VitePress docs, npm/PyPI binary distribution, and full GitHub Actions CI/CD. Every major architectural and tooling decision can be direct-ported from jr with Confluence-specific adaptations. The zero-new-Go-dependencies constraint is confirmed viable: all new internal packages use only stdlib, with one exception (`spf13/pflag` direct dep needed for `cmd/gendocs` flag introspection).

The recommended approach is methodical: build pure-logic internal packages first (`internal/jsonutil`, `internal/duration`, `internal/preset`), extend the template system, add a v1 HTTP POST/PUT helper to unlock the workflow commands that have no v2 API equivalents (move, copy, restrict, archive), then build CLI commands in dependency order, followed by release infrastructure. The architecture mirrors jr exactly — new commands register in `cmd/`, schema ops are appended in `schema_cmd.go`, and the preset/template systems follow three-tier lookup (profile override > user file > builtin).

The key risks concentrate in three areas: (1) Confluence Cloud API specifics — several operations use v1-only async APIs with long-running tasks that must be polled, and the restrictions API has a non-obvious replace-not-merge model that can silently delete access; (2) release infrastructure — npm classic tokens are deprecated (OIDC required), Homebrew/Scoop need a PAT with cross-repo write access, and the GoReleaser ldflags path must exactly match the Go module path; (3) VitePress docs — base path configuration breaks nested links on GitHub Pages unless `base`, leading-slash links, and `.nojekyll` are all set correctly. All risks have documented prevention strategies from the jr reference.

---

## Key Findings

### Recommended Stack

The existing stack (Go 1.25.8, Cobra, gojq, libopenapi) is unchanged and validated. For v1.2, all new Go code uses only stdlib — no new module dependencies except adding `spf13/pflag` as a direct dep for `cmd/gendocs` flag introspection. Release infrastructure adds GoReleaser v2.14.x (OSS), VitePress 1.x stable (not v2 alpha), golangci-lint v2, gosec, govulncheck, and SHA-pinned GitHub Actions.

**Core technologies:**

| Technology | Purpose | Why |
|------------|---------|-----|
| Go stdlib (`bytes`, `encoding/json`) | `internal/jsonutil.MarshalNoEscape()` | Confluence XHTML body corrupts with default HTML-escaping of `&`, `<`, `>` |
| Go stdlib (`regexp`, `strconv`) | `internal/duration.Parse()` | Human-readable durations for `diff --since`; 1d=24h (calendar, not Jira workday) |
| Go stdlib (`encoding/json`, `os`) | `internal/preset` package | Built-in presets with user override; JQ-only (no API-level field filter in Confluence v2) |
| Go `embed.FS` | Built-in templates in `internal/template/builtin/*.json` | Keep JSON format (no yaml.v3 dep); embed 4 templates |
| GoReleaser v2.14.x | Cross-platform builds, GitHub releases, Homebrew, Scoop, Docker | Industry standard; jr reference validated; OSS covers all needs |
| VitePress ^1.6.4 | Static docs site with auto-generated command reference | v2.0 is alpha-only; 1.x stable with Node 24 + esbuild ^0.25.0 override |
| golangci-lint v2 | Lint aggregator | v2 config format (`version: "2"`, `linters.default: standard`) |
| gosec + govulncheck | SAST + dependency vulnerability scanning | Standard Go security toolchain; exclude `cmd/generated` from gosec |
| npm + PyPI packages | Binary distribution for Node/AI and Python ecosystems | Postinstall binary download pattern (same as esbuild, turbo) |
| `spf13/pflag` | Direct dep for `cmd/gendocs` flag introspection | Needed for Cobra flag walking in docs generator |

**Version constraints that matter:**
- VitePress: `^1.6.4` only — v2.0.0-alpha.17 is NOT suitable; `overrides: {"esbuild": "^0.25.0"}` required
- GoReleaser: use `goreleaser-action@v7` with `~> v2` constraint
- All GitHub Actions: SHA-pinned with tag comment for readability

### Expected Features

**Must have (table stakes):**
- `diff` command — page version comparison using v2 version history API; `--since` duration filter; structured JSON output with change stats
- `workflow move` — move page to different parent/space; v1-only API (`PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}`)
- `workflow copy` — copy page with options; v1-only async API; must poll `/longtask/{taskId}`
- `workflow publish` — publish draft page; wraps existing v2 page update with `status: "current"`
- `workflow comment` — convenience wrapper around existing footer-comments create; plain text to storage format
- Built-in presets + `preset list` — 7 built-in presets: `brief`, `titles`, `agent`, `tree`, `meta`, `search`, `diff`
- Built-in templates + `templates show`/`templates create` — 4 built-ins: blank, meeting-notes, decision-record, project-update
- `export` command — page body extraction (storage/view/atlas_doc_format); NOT PDF (no API exists)
- Full CI/CD: 7 GitHub Actions workflows (ci, release, docs, security, spec-drift, spec-auto-release, dependabot-auto-merge)
- GoReleaser config, Homebrew tap, Scoop bucket, Docker multi-arch images
- VitePress docs site with auto-generated command reference + guide pages
- npm + PyPI binary distribution packages

**Should have (differentiators):**
- `diff --since` with symbolic user resolution (`--user me`, `--user email@example.com`) in workflow restrict
- `templates create --from-page` to reverse-engineer a template from an existing page
- Schema registration for all new commands (agents discover via `cf schema`)
- `export --tree` for recursive page tree export as NDJSON stream

**Defer to future milestone:**
- `workflow restrict` — v1-only API, non-obvious replace-not-merge semantics, self-lockout risk; defer unless specifically requested
- `workflow archive` — async, free-tier limit of 1 page, one-job-per-tenant constraint
- PDF/Word export — no Confluence Cloud REST API exists (CONFCLOUD-61557 open)
- Version restore — destructive operation, needs explicit UX design

### Architecture Approach

v1.2 follows the established cf + jr pattern exactly. New internal packages (`jsonutil`, `duration`, `preset`) are pure utilities with no HTTP and no new module deps. New command files follow the existing `mergeCommand()` + `client.FromContext()` + `client.Fetch()` pattern. Schema discovery is extended by appending `*SchemaOps()` functions from each new `*_schema.go` file into `schema_cmd.go`. The preset system is upgraded from config-only map lookup to three-tier resolution: profile presets > `internal/preset` user file > builtins. The template system gains `embed.FS` builtins in JSON format, avoiding yaml.v3. The `fetchV1()` helper must be extended to support POST and PUT methods — it is currently GET-only.

**Major components:**

| Component | Action | Responsibility |
|-----------|--------|----------------|
| `internal/jsonutil` | NEW | `MarshalNoEscape()` — fixes XHTML corruption in JSON output |
| `internal/duration` | NEW | `Parse()` — human-readable durations for `diff --since`; calendar time (1d=24h) |
| `internal/preset` | NEW | Built-in presets, `Lookup()`, `List()` with source attribution |
| `internal/template` | MODIFY | Add `embed.FS` builtins + `loadBuiltinTemplates()` + `show`/`create` subcommands |
| `cmd/diff.go` + `diff_schema.go` | NEW | Page version comparison command |
| `cmd/workflow.go` + `workflow_schema.go` | NEW | Workflow subcommands: move, copy, publish, comment |
| `cmd/preset.go` | NEW | `preset list` subcommand |
| `cmd/export.go` + `export_schema.go` | NEW | Body extraction wrapper |
| `cmd/gendocs/main.go` | NEW | Walks Cobra tree, generates VitePress docs + sidebar JSON |
| `cmd/schema_cmd.go` | MODIFY | Aggregate all `*SchemaOps()` functions from new schema files |
| `cmd/root.go` | MODIFY | Register new commands + three-tier preset resolution |
| `.github/workflows/` (7 files) | NEW | Full CI/CD pipeline |
| `.goreleaser.yml` + Dockerfiles | NEW | Multi-arch release config |
| `website/` | NEW | VitePress docs site |
| `npm/` + `python/` | NEW | Binary distribution packages |

**Build order (dependency graph):**
1. `internal/jsonutil`, `internal/duration` — no deps, pure logic
2. `internal/preset` — no deps, pure logic
3. `internal/template` extension — embed.FS, builtin JSON files
4. Extend `fetchV1()` for POST/PUT
5. `cmd/diff.go` (needs duration + jsonutil)
6. `cmd/workflow.go` (needs extended fetchV1 + jsonutil)
7. `cmd/preset.go` (needs internal/preset)
8. `cmd/export.go`
9. All `*_schema.go` files + `schema_cmd.go` aggregation
10. `cmd/gendocs/main.go` (needs all schema ops complete)
11. GoReleaser config, npm scaffold, Python scaffold
12. `website/` (needs gendocs to generate sidebar)
13. GitHub Actions workflows (needs all above)

### Critical Pitfalls

1. **Version number lag causes silent 409 conflicts** — After any write, use the response version number, not a re-fetch. Implement retry-with-re-fetch on 409. Most critical for workflow commands that chain operations.

2. **Restrictions API replaces, does not merge** — A `PUT` to the restriction endpoint replaces ALL restrictions for that operation. Always GET first, merge the new restriction, then PUT the full set. Default to `--add` mode. Auto-include current authenticated user to prevent self-lockout.

3. **Move/copy are async v1 operations** — Both return a long-task ID. Must poll `/longtask/{taskId}` for completion. Default to synchronous wait; provide `--async` flag for fire-and-forget. Handle `OptimisticLockException` (409) with retry.

4. **GoReleaser ldflags path must be exact** — Must be `-X github.com/sofq/confluence-cli/cmd.Version={{.Version}}`. Wrong path silently produces binaries reporting `{"version":"dev"}` in every release.

5. **npm classic tokens deprecated — OIDC required** — As of December 2025, Classic Tokens are dead. Must configure OIDC trusted publishing on npmjs.com. First publish must be done manually; OIDC trust cannot be configured until the package exists on npmjs.com.

6. **Diff needs v1 API fallback for historical body content** — v2 version endpoints exist but historical body retrieval is more reliable via v1 `GET /content/{id}/version/{N}?expand=content.body.storage`. Try v2 `body-format=storage` first, fall back to v1.

7. **VitePress base path + .nojekyll** — Must set `base: '/confluence-cli/'`, all sidebar links must start with `/`, and `.nojekyll` must be in the output dir. Missing any one causes production 404s.

---

## Implications for Roadmap

Based on combined research, the natural phase structure follows the dependency graph with a logical grouping: pure logic before HTTP, utilities before commands, CLI features before release infrastructure, and release infrastructure before the docs site (which needs all schema ops complete for gendocs).

### Phase 1: Internal Utilities

**Rationale:** Pure Go packages with no HTTP and no deps. Highest confidence, lowest risk, fastest to build. These unblock everything else and can be fully unit-tested in isolation.

**Delivers:** `internal/jsonutil`, `internal/duration`, `internal/preset` packages with unit tests

**Addresses:** Foundation for diff `--since` flag, preset system upgrade, HTML-escape fix in JSON output

**Avoids:** Building commands before their utilities exist; avoids coupling unrelated logic into cmd files

**Research flag:** Skip — well-documented patterns ported directly from jr reference with Confluence-specific constant adaptations (1d=24h vs jr's 1d=8h)

---

### Phase 2: Content Utilities (Preset + Template + Export)

**Rationale:** Low-to-medium complexity. No new API patterns needed — all use existing `client.Fetch()` and v2 endpoints. Extends existing systems (template, preset). Can be built and tested end-to-end quickly.

**Delivers:** `preset list` command, built-in presets in three-tier resolution, embedded built-in templates (4), `templates show`/`templates create` subcommands, `export` command

**Addresses:** jr parity for presets and templates; clean body extraction for agents

**Avoids:** Adding yaml.v3 dep (keep JSON for templates); PDF export (no API); YAML complexity from jr not needed for cf's simpler content model; export format conversion API (deprecated August 2026, prefer direct `body-format` param)

**Research flag:** Skip — straightforward; Go embed pattern is standard; preset pattern direct from jr

---

### Phase 3: Version Diff

**Rationale:** Medium complexity, uses v2 API and the new `internal/duration` package. Isolated enough to build and test without the v1 workflow infrastructure. High agent value as a standalone feature.

**Delivers:** `diff` command with `--id`, `--from`, `--to`, `--since`, `--count` flags; structured JSON output; `diff_schema.go` schema registration

**Addresses:** Agent need for "what changed on this page?" without knowing version numbers

**Avoids:** Pitfall 9 (v2 may need v1 fallback for historical body — test v2 first); Pitfall 21 (CDATA spurious diffs — use line-based not XML-aware diff); Pitfall 1 (version lag — fetch both versions in single flow)

**Research flag:** Needs validation — the v2 `body-format=storage` parameter on historical version endpoints should be tested against a live Confluence instance before finalizing the fetch approach. v1 fallback should be implemented from the start.

---

### Phase 4: Workflow Commands

**Rationale:** v1 async APIs require the most care. Depends on extending `fetchV1()` for POST/PUT. Move and copy require long-task polling — a new pattern for cf. Build together since they share the v1 POST/PUT helper and common async polling logic.

**Delivers:** `workflow move`, `workflow copy`, `workflow publish`, `workflow comment`; `workflow_schema.go` schema registration. Restrict and archive explicitly deferred.

**Addresses:** Core content lifecycle management; draft-to-published workflow; agent-friendly comment creation

**Avoids:** Pitfall 3 (async move/copy — implement `/longtask/{taskId}` polling with `--async` escape hatch); Pitfall 1 (409 on version lag — use response version number); position `before`/`after` orphaning top-level pages (default to `append`)

**Research flag:** Needs validation — move endpoint async behavior is unclear (some reports say synchronous for simple moves). Test against live Confluence to determine if long-task polling is always required for move.

---

### Phase 5: Schema + Gendocs

**Rationale:** Schema aggregation in `schema_cmd.go` must be done after all `*_schema.go` files are complete. Gendocs depends on all schema ops being registered. Natural integration gate between CLI features and release infrastructure.

**Delivers:** Unified schema discovery for all new commands; `cmd/gendocs/main.go` generating VitePress sidebar JSON + per-resource docs + error codes page; `watch_schema.go` for the existing watch command

**Addresses:** Agent discoverability of workflow, diff, export, template commands via `cf schema`; complete foundation for VitePress docs

**Avoids:** Pitfall 17 (sidebar JSON must be generated before VitePress build — chain `docs-generate` -> `docs-build` in Makefile)

**Research flag:** Skip — direct port of jr's gendocs binary with cf-specific command tree

---

### Phase 6: Release Infrastructure

**Rationale:** GoReleaser, npm, PyPI, and GitHub Actions can only be validated end-to-end after the CLI is stable. This phase is primarily configuration with high-risk edge cases around token management and OIDC publishing.

**Delivers:** `.goreleaser.yml`, `Dockerfile.goreleaser`, npm scaffold, Python scaffold, `.golangci.yml`, 7 GitHub Actions workflows, Makefile additions (`lint`, `docs-*`, `spec-update`), project root files (README, LICENSE, SECURITY.md)

**Addresses:** Full cross-platform binary distribution; CI quality gates; automated spec drift detection

**Avoids:** Pitfall 4 (ldflags exact path: `github.com/sofq/confluence-cli/cmd.Version`); Pitfall 5 (HOMEBREW_TAP_TOKEN as PAT, not GITHUB_TOKEN); Pitfall 6 (npm OIDC — first publish is manual); Pitfall 15 (PyPI trusted publisher must exist before v1.2.0 tag); Pitfall 16 (Buildx + `packages: write` for Docker); Pitfall 18 (Scoop reuses same PAT as Homebrew); Pitfall 19 (gosec exclude `cmd/generated`); Pitfall 22 (spec-drift uses Confluence spec URL)

**Research flag:** Pre-execution setup required — npm and PyPI first-publish must be done manually before OIDC can be configured. Must complete before tagging v1.2.0.

---

### Phase 7: Documentation Site

**Rationale:** VitePress site is the final deliverable, depends on gendocs being complete and all commands being stable. Guide pages require knowing the final command surface.

**Delivers:** `website/` with VitePress config, guide pages (getting-started, filtering, discovery, templates, global-flags, agent-integration), auto-generated command reference, deployed to GitHub Pages via `docs.yml` workflow

**Addresses:** User and agent discoverability; project public face

**Avoids:** Pitfall 10 (base path `/confluence-cli/`, leading-slash links, `.nojekyll`); Pitfall 12 (permission trio: `contents: read`, `pages: write`, `id-token: write`); set `ignoreDeadLinks: true` for generated pages

**Research flag:** Skip — direct port of jr's website structure; VitePress patterns well-documented

---

### Phase Ordering Rationale

- Phases 1-2 establish internal packages before any HTTP command needs them; no integration risk
- Phase 3 (diff) is isolated and high-value and can ship as a partial release if needed
- Phase 4 (workflow) groups all commands sharing the v1 POST/PUT helper and async polling pattern
- Phase 5 (schema/gendocs) is a mandatory integration gate before docs
- Phases 6-7 are release-time concerns that do not affect the CLI's operational value and can proceed in parallel with late Phase 5 work

### Research Flags Summary

| Phase | Flag | Reason |
|-------|------|--------|
| Phase 1: Internal Utilities | Skip | Direct port from jr; pure stdlib logic |
| Phase 2: Content Utilities | Skip | Existing patterns extended; no new API behavior |
| Phase 3: Version Diff | Needs validation | v2 historical body retrieval needs live API testing |
| Phase 4: Workflow Commands | Needs validation | Move async behavior (sync vs async) needs live testing |
| Phase 5: Schema + Gendocs | Skip | Direct port of jr's gendocs; well-documented pattern |
| Phase 6: Release Infrastructure | Pre-execution setup | npm + PyPI first-publish manual steps must precede tagging |
| Phase 7: Documentation | Skip | VitePress port of jr; all patterns documented |

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All tools verified against jr reference (local codebase) + official release pages; GoReleaser v2.14.3, VitePress 1.6.4, golangci-lint v2.11.4 versions confirmed |
| Features | HIGH | API endpoints verified in generated code (`cmd/generated/pages.go`) + official Atlassian docs; PDF limitation confirmed via Atlassian KB + open JIRA issue |
| Architecture | HIGH | Based on direct codebase inspection of both cf and jr; no inference required for patterns |
| Pitfalls | HIGH / MEDIUM (2) | 20 of 22 pitfalls are HIGH confidence from official docs + community; Pitfall 9 (v2 body retrieval) and Pitfall 21 (CDATA diff) are MEDIUM pending live testing |

**Overall confidence:** HIGH

### Gaps to Address

- **v2 historical version body retrieval** (Pitfall 9): The v2 endpoint has `body-format` parameter but community reports suggest incomplete body return for historical versions. Validate against live Confluence in Phase 3 planning; implement v1 fallback from the start rather than as an afterthought.

- **Move endpoint async behavior** (Pitfall 3): The Atlassian announcement says move returns a long-running task, but some reports suggest synchronous behavior for simple moves. Test against a live instance in Phase 4 planning; polling should be implemented defensively regardless.

- **Rate limiting impact** (Pitfall 13): Points-based quotas rolling out March 2026 for OAuth2 3LO apps. Basic auth (API token) users are currently exempt. Add 429 + `Retry-After` handling proactively in the HTTP client layer.

- **npm OIDC first-publish**: Manual first-publish step for `confluence-cf` on npmjs.com must be completed before Phase 6 workflows can run end-to-end. This is a people/process dependency, not a code dependency — plan it before tagging v1.2.0.

---

## Sources

### Primary (HIGH confidence)

- jr reference implementation (`/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/`) — all patterns, versions, SHA pins; primary source for every component
- Generated code `cmd/generated/pages.go` — v2 version endpoints verified at lines 854-921; `version` param at line 146
- [Confluence Cloud REST API v2](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/) — endpoint verification
- [Confluence REST API v1 restrictions](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content-restrictions/) — v1-only restriction endpoints
- [Added Move and Copy Page APIs (Atlassian)](https://community.developer.atlassian.com/t/added-move-and-copy-page-apis/37749) — v1 move/copy confirmation
- [GoReleaser v2.14 release notes](https://goreleaser.com/blog/goreleaser-v2.14/) + [releases](https://github.com/goreleaser/goreleaser/releases) — v2.14.3 current
- [golangci-lint v2 announcement](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/) + [releases](https://github.com/golangci/golangci-lint/releases) — v2.11.4 current
- [VitePress npm](https://www.npmjs.com/package/vitepress) — v1.6.4 stable; v2.0.0-alpha.17 not suitable
- [npm trusted publishing docs](https://docs.npmjs.com/trusted-publishers/) — OIDC requirement
- [PyPI trusted publishing docs](https://docs.pypi.org/trusted-publishers/) — OIDC setup
- [GoReleaser ldflags cookbook](https://goreleaser.com/cookbooks/using-main.version/) — exact path requirement
- [REST API for PDF export (Atlassian KB)](https://support.atlassian.com/confluence/kb/rest-api-to-export-and-download-a-page-in-pdf-format/) — confirms no PDF API
- [GitHub Actions deploy-pages](https://github.com/actions/deploy-pages) — permission trio documentation
- [Atlassian rate limiting](https://developer.atlassian.com/cloud/confluence/rate-limiting/) — points-based quota announcement

### Secondary (MEDIUM confidence)

- [Confluence version lag community report](https://community.developer.atlassian.com/t/lag-in-updating-page-version-number-v2-confluence-rest-api/68821) — 409 version lag behavior
- [Archive content via REST API (community)](https://community.developer.atlassian.com/t/how-to-archive-and-restore-archived-confluence-content-via-rest-api/82062) — archive endpoint constraints
- [Page restrictions via v2 API (community)](https://community.developer.atlassian.com/t/page-restrictions-via-the-v2-api/93094) — v2 restrictions unavailability confirmed
- [Historical page version content (community)](https://community.atlassian.com/forums/Confluence-questions/Confluence-API-get-page-content-from-historical-versions/qaq-p/1398857) — v1 fallback for body retrieval
- govulncheck v1.1.4 — installed via `go install`; may have newer patch version

---

*Research completed: 2026-03-28*
*Ready for roadmap: yes*
