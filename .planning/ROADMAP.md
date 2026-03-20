# Roadmap: Confluence CLI (`cf`)

## Overview

Build `cf`, a Go CLI that exposes the full Confluence Cloud v2 REST API as shell commands, optimized for AI agent consumption. The project mirrors the existing `jr` (Jira CLI) architecture exactly: a core HTTP client and config layer (Phase 1), then an OpenAPI code-generation pipeline that produces all Cobra commands from the spec (Phase 2), then hand-written workflow wrappers for the primary resources — pages, spaces, search, comments, and labels (Phase 3), and finally governance and agent-optimization features including operation policy, audit logging, caching, and batch execution (Phase 4). The Avatar analysis capability ships as a standalone Phase 5.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Core Scaffolding** - HTTP client, config profiles, auth, and the pure JSON output contract (completed 2026-03-20)
- [x] **Phase 2: Code Generation Pipeline** - OpenAPI spec parser/generator producing all Cobra commands (completed 2026-03-20)
- [x] **Phase 3: Pages, Spaces, Search, Comments, and Labels** - Primary resources with Confluence-specific workflow wrappers (completed 2026-03-20)
- [ ] **Phase 4: Governance and Agent Optimization** - Operation policy, audit logging, response caching, and batch execution
- [ ] **Phase 5: Avatar Analysis** - AI-ready writing style analysis from Confluence user content

## Phase Details

### Phase 1: Core Scaffolding
**Goal**: AI agents and users can authenticate and make raw API calls, with all infrastructure guarantees (pure JSON stdout, structured JSON errors, semantic exit codes, JQ filtering, dry-run, verbose, pagination, caching, `cf raw`, `cf schema`, `cf --version`) in place.
**Depends on**: Nothing (first phase)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06, INFRA-07, INFRA-08, INFRA-09, INFRA-10, INFRA-11, INFRA-12, INFRA-13
**Success Criteria** (what must be TRUE):
  1. `cf configure` creates a profile with base URL, auth type, and credentials; `cf --profile <name>` or `CF_PROFILE` selects it
  2. `cf raw GET /wiki/api/v2/pages` returns a pure JSON response to stdout and any error as structured JSON to stderr with a non-zero semantic exit code
  3. Any command output can be filtered in-process with `--jq '.results[].title'` and the result is valid JSON
  4. `cf raw GET /wiki/api/v2/pages --dry-run` prints the request that would be made without executing it
  5. `cf --version` outputs version info as JSON and `cf schema` outputs the command tree and parameter schemas as JSON
**Plans**: 4 plans

Plans:
- [x] 01-01-PLAN.md — Go module scaffold, internal packages (errors, config, jq, cache, generated stub)
- [x] 01-02-PLAN.md — HTTP client with cursor-based pagination
- [x] 01-03-PLAN.md — Cobra commands (root, configure, raw, version, schema)
- [x] 01-04-PLAN.md — Test suite for all Phase 1 packages and commands (completed 2026-03-20)

### Phase 2: Code Generation Pipeline
**Goal**: The `gen/` pipeline reads `spec/confluence-v2.json` and produces `cmd/generated/` with a complete, compilable Cobra command tree covering all OpenAPI operations; generated commands can be overridden by hand-written wrappers.
**Depends on**: Phase 1
**Requirements**: CGEN-01, CGEN-02, CGEN-03, CGEN-04, CGEN-05
**Success Criteria** (what must be TRUE):
  1. Running `make generate` against the pinned spec produces `cmd/generated/*.go` files that compile without errors
  2. Each generated command includes all path, query, and body parameters from the OpenAPI spec as Cobra flags
  3. Commands are grouped by OpenAPI resource tag (pages, spaces, search, etc.) under corresponding subcommands
  4. A hand-written workflow command registered via `mergeCommand` replaces the generated command for the same operation without build errors
**Plans**: 3 plans

Plans:
- [x] 02-01-PLAN.md — Download spec, add libopenapi dependency, document spec gaps
- [x] 02-02-PLAN.md — gen/ core: parser, grouper, generator, templates, unit tests
- [ ] 02-03-PLAN.md — gen/main.go, conformance tests, make generate, commit generated output

### Phase 3: Pages, Spaces, Search, Comments, and Labels
**Goal**: AI agents can perform all primary Confluence content operations — finding spaces, discovering pages via CQL, reading page bodies, creating and updating pages, managing comments and labels — with all Confluence v2 API edge cases handled correctly.
**Depends on**: Phase 2
**Requirements**: PAGE-01, PAGE-02, PAGE-03, PAGE-04, PAGE-05, SPCE-01, SPCE-02, SPCE-03, SRCH-01, SRCH-02, SRCH-03, CMNT-01, CMNT-02, CMNT-03, LABL-01, LABL-02, LABL-03
**Success Criteria** (what must be TRUE):
  1. `cf pages get <id>` returns a JSON response that includes a non-empty `body.storage.value` field
  2. `cf pages update <id> --title "New Title"` succeeds even when run back-to-back without manually tracking version numbers (optimistic locking handled internally)
  3. `cf search --cql "space = ENG AND type = page"` paginates through all results and returns a merged JSON array, including pages beyond the first cursor
  4. `cf spaces list --key ENG` resolves the space key to a numeric ID and returns space details without a 404
  5. `cf comments list --page-id <id>` and `cf labels list --content-id <id>` return JSON arrays; add and delete operations exit 0 on success
**Plans**: 4 plans

Plans:
- [ ] 03-01-PLAN.md — cmd/pages.go: get-by-id (body-format=storage), create, update (version auto-increment + 409 retry), delete, list
- [ ] 03-02-PLAN.md — cmd/spaces.go: resolveSpaceID helper, list and get-by-id with key resolution
- [ ] 03-03-PLAN.md — cmd/search.go (CQL + manual v1 pagination), cmd/comments.go, cmd/labels.go (v1 add/remove)
- [ ] 03-04-PLAN.md — Wire all commands into cmd/root.go + unit tests for all five workflow files

### Phase 4: Governance and Agent Optimization
**Goal**: Production deployments of AI agents using `cf` can enforce operation policies, maintain an audit trail, reduce API quota consumption through caching, and execute multi-step workflows atomically via batch.
**Depends on**: Phase 3
**Requirements**: GOVN-01, GOVN-02, GOVN-03, GOVN-04, BTCH-01, BTCH-02, BTCH-03
**Success Criteria** (what must be TRUE):
  1. A profile with `allow: ["pages:read", "spaces:read"]` rejects a `cf pages create` call before the HTTP request is made, including in `--dry-run` mode, and exits with code 4
  2. Every API call appended to the NDJSON audit log includes timestamp, profile, operation, method, path, and response status
  3. `cf pages get <id> --cache 300` on a second invocation within the TTL returns the cached response without making an HTTP request
  4. `cf batch --input ops.json` executes all operations and returns a JSON array with per-operation exit codes and data/error, continuing past individual failures
**Plans**: TBD

### Phase 5: Avatar Analysis
**Goal**: AI agents can obtain a structured JSON persona profile derived from a Confluence user's writing history for downstream use in content generation or style matching.
**Depends on**: Phase 3
**Requirements**: AVTR-01, AVTR-02
**Success Criteria** (what must be TRUE):
  1. `cf avatar analyze --user <accountId>` fetches the user's pages via CQL and outputs a structured JSON persona profile without error
  2. The persona profile JSON contains fields consumable by an AI agent (e.g., tone, vocabulary level, structural patterns) without requiring post-processing
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Core Scaffolding | 4/4 | Complete    | 2026-03-20 |
| 2. Code Generation Pipeline | 2/3 | Complete    | 2026-03-20 |
| 3. Pages, Spaces, Search, Comments, and Labels | 3/4 | Complete    | 2026-03-20 |
| 4. Governance and Agent Optimization | 0/? | Not started | - |
| 5. Avatar Analysis | 0/? | Not started | - |
