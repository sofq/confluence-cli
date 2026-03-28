# Roadmap: Confluence CLI (`cf`)

## Milestones

- ✅ **v1.0 Core CLI** - Phases 1-5 (shipped 2026-03-20)
- ✅ **v1.1 Extended Capabilities** - Phases 6-11 (shipped 2026-03-20)
- 🚧 **v1.2 Workflow, Parity & Release Infrastructure** - Phases 12-18 (in progress)

## Phases

<details>
<summary>v1.0 Core CLI (Phases 1-5) - SHIPPED 2026-03-20</summary>

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

<details>
<summary>v1.1 Extended Capabilities (Phases 6-11) - SHIPPED 2026-03-20</summary>

- [x] **Phase 6: OAuth2 Authentication** - Client credentials and browser-based OAuth2 with automatic token refresh (completed 2026-03-20)
- [x] **Phase 7: Blog Posts** - Full CRUD for blog posts mirroring the pages pattern (completed 2026-03-20)
- [x] **Phase 8: Attachments** - Attachment listing, metadata, upload (v1 API), and deletion (completed 2026-03-20)
- [x] **Phase 9: Custom Content** - CRUD for custom content types via v2 API (completed 2026-03-20)
- [x] **Phase 10: Output Presets and Templates** - Named output presets and content template system (completed 2026-03-20)
- [x] **Phase 11: Watch** - Long-running content change polling with NDJSON event streaming (completed 2026-03-20)

### Phase 6: OAuth2 Authentication
**Goal**: Users and service accounts can authenticate via OAuth2 (both machine-to-machine and interactive browser flow), with tokens managed transparently across sessions.
**Depends on**: Phase 5 (v1.0 complete)
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04
**Plans**: 2 plans

Plans:
- [x] 06-01: Config schema + token store + OAuth2 client credentials (2LO)
- [x] 06-02: 3LO browser flow with PKCE + automatic token refresh

### Phase 7: Blog Posts
**Goal**: AI agents can perform full CRUD operations on Confluence blog posts with the same reliability as pages.
**Depends on**: Phase 6
**Requirements**: BLOG-01, BLOG-02, BLOG-03, BLOG-04, BLOG-05
**Plans**: 1 plan

Plans:
- [x] 07-01: Blog post CRUD (cmd/blogposts.go) + tests + root wiring

### Phase 8: Attachments
**Goal**: Users can discover, inspect, upload, and remove file attachments on Confluence content.
**Depends on**: Phase 7
**Requirements**: ATCH-01, ATCH-02, ATCH-03, ATCH-04
**Plans**: 1 plan

Plans:
- [x] 08-01: Attachment list (v2 --page-id), upload (v1 multipart), mergeCommand wiring

### Phase 9: Custom Content
**Goal**: Users can manage custom content types (from Connect and Forge apps) through the same CRUD pattern as pages and blog posts.
**Depends on**: Phase 7
**Requirements**: CUST-01, CUST-02, CUST-03, CUST-04
**Plans**: 1 plan

Plans:
- [x] 09-01: Custom content CRUD (cmd/custom_content.go) with --type flag + tests + root wiring

### Phase 10: Output Presets and Templates
**Goal**: Users can save and reuse output formatting configurations and create content from reusable templates with variable substitution.
**Depends on**: Phase 6
**Requirements**: PRST-01, PRST-02, TMPL-01, TMPL-02
**Plans**: 2 plans

Plans:
- [x] 10-01: Named output presets (config Presets field + --preset flag + resolution)
- [x] 10-02: Content template system (internal/template + cf templates list + --template/--var on create commands)

### Phase 11: Watch
**Goal**: AI agents can reactively monitor Confluence content for changes via a long-running polling command that emits structured NDJSON events.
**Depends on**: Phase 7
**Requirements**: WTCH-01, WTCH-02
**Plans**: 1 plan

Plans:
- [x] 11-01: Watch command with CQL polling, NDJSON events, signal shutdown

</details>

### v1.2 Workflow, Parity & Release Infrastructure (In Progress)

**Milestone Goal:** Close the feature gap with jr by adding workflow commands, version diff, built-in presets/templates, and replicate the full CI/CD, documentation, and release infrastructure.

- [x] **Phase 12: Internal Utilities** - jsonutil, duration, and preset packages providing foundation for all subsequent commands (completed 2026-03-28)
- [ ] **Phase 13: Content Utilities** - Built-in presets/templates, preset list, template management, and export commands
- [ ] **Phase 14: Version Diff** - Page version comparison with time-range and explicit version filtering
- [ ] **Phase 15: Workflow Commands** - Move, copy, publish, comment, restrict, and archive operations
- [ ] **Phase 16: Schema + Gendocs** - Schema registration for all new commands and VitePress docs generator binary
- [ ] **Phase 17: Release Infrastructure** - GoReleaser, GitHub Actions CI/CD, npm/Python packages, and project config files
- [ ] **Phase 18: Documentation Site** - VitePress site with guide pages and auto-generated command reference

## Phase Details

### Phase 12: Internal Utilities
**Goal**: Pure-logic internal packages exist and are fully tested, providing the foundation that all subsequent CLI commands depend on.
**Depends on**: Phase 11 (v1.1 complete)
**Requirements**: UTIL-01, UTIL-02, UTIL-03
**Success Criteria** (what must be TRUE):
  1. `internal/jsonutil.MarshalNoEscape()` serializes Go values to JSON without HTML-escaping `&`, `<`, `>` characters in XHTML content, and existing commands can adopt it
  2. `internal/duration.Parse("2h")`, `Parse("1d")`, `Parse("1w")` return correct `time.Duration` values using calendar conventions (1d=24h, 1w=168h), and invalid input returns a descriptive error
  3. `internal/preset.Lookup(name, profile)` resolves presets through the three-tier chain (profile config > user preset file > built-in), and `List()` returns all available presets with their source attribution (built-in, user, profile)
**Plans**: 3 plans

Plans:
- [x] 12-01-PLAN.md — Create jsonutil and duration internal packages with tests
- [x] 12-02-PLAN.md — Create preset package with three-tier resolution and wire into cmd/root.go
- [x] 12-03-PLAN.md — Refactor all existing SetEscapeHTML call sites to use jsonutil

### Phase 13: Content Utilities
**Goal**: Users have access to built-in presets and templates out of the box, can manage templates, and can extract page content via export.
**Depends on**: Phase 12
**Requirements**: CONT-01, CONT-02, CONT-03, CONT-04, CONT-05, CONT-06, CONT-07
**Success Criteria** (what must be TRUE):
  1. `cf preset list` displays all available presets grouped by source (built-in, user, profile), showing the preset name and its JQ expression
  2. A fresh install of cf includes 7 built-in presets (brief, titles, agent, tree, meta, search, diff) that work with `--preset <name>` on any list/get command
  3. A fresh install of cf includes 6 built-in templates (blank, meeting-notes, decision, runbook, retrospective, adr) accessible via `--template <name>` on create commands
  4. `cf templates show <name>` prints the full template definition (body, variables, description) as JSON, and `cf templates create --from-page <id>` reverse-engineers a template from an existing page
  5. `cf export --id <pageId>` outputs the page body in the requested format (storage/view/atlas_doc_format), and `cf export --id <pageId> --tree` recursively exports a page tree as NDJSON (one line per page)
**Plans**: 3 plans

Plans:
- [ ] 13-01-PLAN.md — Built-in templates, template package refactoring, and preset list command
- [ ] 13-02-PLAN.md — Templates show, create --from-page commands, and list output refactoring
- [ ] 13-03-PLAN.md — Export command with single-page and recursive tree NDJSON modes

### Phase 14: Version Diff
**Goal**: Users can compare page versions and understand what changed, when, and by whom.
**Depends on**: Phase 12
**Requirements**: DIFF-01, DIFF-02, DIFF-03
**Success Criteria** (what must be TRUE):
  1. `cf diff --id <pageId>` outputs a structured JSON diff comparing the two most recent versions, including change statistics (lines added/removed) and version metadata (author, timestamp)
  2. `cf diff --id <pageId> --since 2h` filters the version history to only show changes within the last 2 hours, using the duration parser from Phase 12
  3. `cf diff --id <pageId> --from 3 --to 5` compares two explicit version numbers and outputs the structured diff between them
**Plans**: TBD

### Phase 15: Workflow Commands
**Goal**: Users can perform content lifecycle operations (move, copy, publish, comment, restrict, archive) through dedicated workflow subcommands.
**Depends on**: Phase 12
**Requirements**: WKFL-01, WKFL-02, WKFL-03, WKFL-04, WKFL-05, WKFL-06
**Success Criteria** (what must be TRUE):
  1. `cf workflow move --id <pageId> --target-id <parentId>` moves a page to a new parent (or space), waits for the async operation to complete by default, and returns the updated page JSON
  2. `cf workflow copy --id <pageId> --target-id <parentId>` copies a page with configurable options (--copy-attachments, --copy-labels, --copy-permissions), polls the long-running task, and returns the new page JSON
  3. `cf workflow publish --id <pageId>` transitions a draft page to published status, and `cf workflow comment --id <pageId> --body "text"` adds a plain-text comment (converted to storage format) to the specified page
  4. `cf workflow restrict --id <pageId> --operation read --user <accountId>` views, adds, or removes page restrictions using the v1 restrictions API, and `cf workflow archive --id <pageId>` archives a page
**Plans**: TBD

### Phase 16: Schema + Gendocs
**Goal**: All new commands are discoverable via `cf schema` and a docs generator binary can produce the complete VitePress command reference.
**Depends on**: Phase 15, Phase 14, Phase 13
**Requirements**: SCHM-01, SCHM-02, DOCS-05
**Success Criteria** (what must be TRUE):
  1. `cf schema` output includes operations for all new commands (diff, workflow move/copy/publish/comment/restrict/archive, export, preset list, templates show/create), with correct verb, resource, description, and flags
  2. All schema operations are aggregated in `schema_cmd.go` from individual `*_schema.go` files, maintaining the existing registration pattern
  3. `go run cmd/gendocs/main.go --output website/` generates per-command Markdown files and a sidebar JSON file from the Cobra command tree, suitable for VitePress consumption
**Plans**: TBD

### Phase 17: Release Infrastructure
**Goal**: The project has complete CI/CD, cross-platform binary distribution, and standard open-source project files ready for public release.
**Depends on**: Phase 16
**Requirements**: CICD-01, CICD-02, CICD-03, CICD-04, CICD-05, CICD-06, CICD-07, CICD-08, CICD-09, CICD-10, DOCS-01, DOCS-02, DOCS-03, CONF-01, CONF-02, CONF-03
**Success Criteria** (what must be TRUE):
  1. Pushing to main triggers CI (build + test + lint via golangci-lint v2); pushing a version tag triggers GoReleaser producing binaries for linux/darwin/windows (amd64/arm64) plus Docker multi-arch images
  2. Security pipeline (gosec + govulncheck) runs on push and weekly; spec-drift detection runs daily, auto-regenerates commands, and creates a PR; Dependabot submits weekly PRs for Go modules and GitHub Actions
  3. `npm install -g confluence-cf` and `pip install confluence-cf` install working binary wrappers that download the correct platform binary on postinstall
  4. Repository root contains README.md (with install methods, quick start, agent integration guide), LICENSE (Apache 2.0), SECURITY.md (vulnerability reporting policy), .golangci.yml, .gitignore, and Makefile with lint/docs/spec-update targets
**Plans**: TBD

### Phase 18: Documentation Site
**Goal**: A public documentation site provides getting-started guides, command reference, and agent integration documentation.
**Depends on**: Phase 16, Phase 17
**Requirements**: DOCS-04
**Success Criteria** (what must be TRUE):
  1. `npm run docs:dev` inside `website/` serves a local VitePress site with navigation, guide pages (getting-started, filtering, discovery, templates, global-flags, agent-integration), and auto-generated command reference
  2. The docs GitHub Actions workflow builds the VitePress site and deploys it to GitHub Pages at the correct base path (`/confluence-cli/`), with `.nojekyll` present and no broken internal links
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 12 -> 13 -> 14 -> 15 -> 16 -> 17 -> 18

Note: Phases 13, 14, and 15 all depend on Phase 12 but not on each other, so they can execute in parallel after Phase 12 completes. Phase 16 depends on 13, 14, and 15 all being complete. Phase 18 depends on both 16 and 17.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Core Scaffolding | v1.0 | 4/4 | Complete | 2026-03-20 |
| 2. Code Generation Pipeline | v1.0 | 3/3 | Complete | 2026-03-20 |
| 3. Pages, Spaces, Search, Comments, and Labels | v1.0 | 4/4 | Complete | 2026-03-20 |
| 4. Governance and Agent Optimization | v1.0 | 3/3 | Complete | 2026-03-20 |
| 5. Avatar Analysis | v1.0 | 2/2 | Complete | 2026-03-20 |
| 6. OAuth2 Authentication | v1.1 | 2/2 | Complete | 2026-03-20 |
| 7. Blog Posts | v1.1 | 1/1 | Complete | 2026-03-20 |
| 8. Attachments | v1.1 | 1/1 | Complete | 2026-03-20 |
| 9. Custom Content | v1.1 | 1/1 | Complete | 2026-03-20 |
| 10. Output Presets and Templates | v1.1 | 2/2 | Complete | 2026-03-20 |
| 11. Watch | v1.1 | 1/1 | Complete | 2026-03-20 |
| 12. Internal Utilities | v1.2 | 3/3 | Complete | 2026-03-28 |
| 13. Content Utilities | v1.2 | 0/3 | Planned | - |
| 14. Version Diff | v1.2 | 0/0 | Not started | - |
| 15. Workflow Commands | v1.2 | 0/0 | Not started | - |
| 16. Schema + Gendocs | v1.2 | 0/0 | Not started | - |
| 17. Release Infrastructure | v1.2 | 0/0 | Not started | - |
| 18. Documentation Site | v1.2 | 0/0 | Not started | - |
