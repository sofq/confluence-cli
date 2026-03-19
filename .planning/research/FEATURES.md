# Feature Research

**Domain:** Confluence CLI for AI agents (structured JSON access to Confluence Cloud)
**Researched:** 2026-03-20
**Confidence:** HIGH (reference implementation examined directly; Confluence v2 API surveyed; competing CLIs reviewed)

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features agents and automation scripts assume exist. Missing these = the tool is not usable.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Pages CRUD (get, create, update, delete) | Core Confluence resource; every competing CLI has this | MEDIUM | v2 API: `GET/POST/PUT/DELETE /pages/{id}`. Body is Atlassian Storage Format (XHTML). Pass through as-is — no conversion. |
| Space listing and get | Spaces are the top-level namespace; agents need to discover them before operating on pages | LOW | `GET /spaces`, `GET /spaces/{id}`. Supports pagination. |
| CQL search | Confluence Query Language is the standard discovery mechanism across all content types | MEDIUM | `GET /search?cql=...`. Returns mixed content types. Pagination required. CQL supports pages, blog posts, comments, attachments, labels. |
| Pure JSON stdout | AI agents cannot parse human-readable output; every command must emit JSON to stdout, errors to stderr | LOW | Already the design mandate. Mirrors jr exactly. |
| Semantic exit codes | Agents need to distinguish auth failure (retry) from not-found (recoverable) from server error (transient) | LOW | Map HTTP status ranges → exit codes. Pattern established in jr: ExitOK=0, ExitError=1, ExitValidation=2. |
| Multi-profile configuration | Agents operate against multiple Confluence instances or with different auth contexts | LOW | `~/.config/cf/config.json`. Profiles selectable with `--profile` flag or `CF_PROFILE` env var. |
| Auth: Basic + Bearer | Atlassian Cloud supports both API token (basic) and PAT/service account bearer tokens | LOW | Both are already supported by the underlying HTTP auth layer in jr. OAuth2 can be deferred. |
| Automatic pagination | List endpoints return paginated results; agents need complete result sets without manual cursor handling | MEDIUM | Confluence v2 uses cursor-based pagination (`cursor` parameter). Must auto-follow and merge results. `--no-paginate` flag to opt out. |
| JQ filtering on all output | Agents slice large JSON responses down to needed fields in a single invocation | LOW | `--jq` persistent flag applied after every API call. gojq library used in jr — same in cf. |
| Raw API command (`raw`) | OpenAPI codegen cannot map 100% of endpoints; agents need escape hatch for unmapped operations | LOW | `cf raw GET /wiki/rest/api/v2/...` — passes request through, applies JQ, respects profile auth. |
| Help / schema discovery as JSON | AI agents cannot parse plain-text help; `--help` output must be parseable or use JSON schema introspection | LOW | Mirror jr's `schema` command: outputs command tree and parameter schemas as JSON to stdout. |
| Configure command | First-time setup for non-agent users; agents use env vars but humans need interactive init | LOW | `cf configure --base-url <url> --token <token>` stores to config profile. |

### Differentiators (Competitive Advantage)

Features that distinguish `cf` from existing Confluence CLIs — specifically optimized for AI agent consumption.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| OpenAPI-generated commands | Every Confluence v2 endpoint available automatically; no manual gaps as Atlassian adds endpoints | HIGH | Core architectural differentiator vs hand-written CLIs (pchuri/confluence-cli, farbodsz/confluence-cli, cfl). Codegen from `dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`. Generated commands live under `cmd/generated/`. |
| Operation policy (allow/deny lists per profile) | Agents can be scoped to read-only or specific resource types without trusting the agent not to write | MEDIUM | Profile config: `allowed_operations: ["page get", "space get"]` or `denied_operations: ["page delete"]`. Glob patterns. Enforced pre-request. Critical for AI agent governance. |
| Audit logging | Every API call an agent makes is logged with timestamp, profile, operation, HTTP method/path/status | MEDIUM | Append-only NDJSON file. `--audit` flag or `audit_log: true` in profile. Non-fatal if log open fails. Enables post-hoc review of agent actions. |
| Batch command | Agents frequently need N operations; spawning N processes is expensive; batch executes from JSON array in one invocation | HIGH | Input: JSON array of `{command, args, jq}`. Output: JSON array of `{index, exit_code, data, error}`. Allows partial failure reporting. |
| Dry-run mode | Agents can preview what a write would do before committing; useful during development and in cautious workflows | LOW | `--dry-run` flag prints request JSON to stdout without executing. Applies at client layer. |
| Preset system | Agents repeat the same `--jq` + `--fields` combination; presets name and reuse these output shapes | MEDIUM | User-defined named presets stored in config. `--preset page-summary` expands to `--fields id,title,version --jq .results[]`. Makes agent prompts shorter and more reliable. |
| Response caching for GET | Agents reading the same page repeatedly (context-gathering loops) waste API quota; caching is free deduplication | MEDIUM | `--cache 5m` TTL on GET requests. Stored in `~/.cache/cf/`. LRU or file-based. Pattern from jr. |
| Verbose mode for debugging | When an agent misbehaves, `--verbose` lets developers see raw HTTP request/response on stderr without changing stdout | LOW | Logs method, URL, status, response time to stderr as JSON lines. Stdout stays clean. |
| Version as JSON | `cf --version` outputs `{"version":"1.0.0"}` not a human string — agents can parse build info | LOW | Override Cobra's version template. Trivial to implement. |
| `--fields` sparse fieldset | Agents only need a subset of fields; requesting full page objects wastes context window tokens | LOW | Maps to Confluence `body-format` and sparse field selection where API supports it. Falls back to post-response JQ projection. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Markdown ↔ Storage Format conversion | Humans want to write in Markdown; LLMs generate Markdown naturally | Atlassian Storage Format is XHTML-based with Confluence-specific macros. Lossless round-tripping is not achievable. Conversion adds a moving-target dependency (third-party libs break on complex macros, tables, panels). Agents handle raw format fine once they know what it is. | Pass storage format as-is. Document the format for agents. Provide a `--format storage` flag to be explicit. |
| Confluence v1 API support | Some older docs or tutorials reference v1 endpoints | v1 is legacy. Atlassian is deprecating it. Maintaining dual-version support doubles the surface area and creates confusion. All new endpoints are v2-only. | v2 API only. `raw` command handles one-off v1 calls if ever needed. |
| Interactive TUI / prompts | Nice for human developers unfamiliar with flags | Interactive mode breaks agent invocation entirely. Any prompt causes the process to hang waiting for stdin. Also incompatible with `--dry-run` inspection patterns. | Non-interactive flags only. `configure` command uses flags, not prompts. `--help` outputs schema. |
| Content rendering / HTML preview | Humans want to read rendered Confluence pages in terminal | Agents consume structured data, not rendered HTML. Rendering adds an HTML parser dependency. Output is ambiguous (HTML vs JSON on stdout). | Return raw storage format. Agents extract text with JQ. Humans use Confluence UI. |
| Real-time push / webhooks | Agents want to react to page changes without polling | Webhooks require a running server, not a CLI. Adds infrastructure complexity incompatible with the CLI model. | `watch` command (polling) covers the agent use case. Low-frequency polling is sufficient for most automation. |
| OAuth2 browser flow | Enterprise users want SSO login | OAuth2 device/browser flows are interactive (require browser redirect). They block non-interactive agent execution. | API token (basic auth) and PAT bearer tokens cover all agent scenarios. OAuth2 can be added later as a non-default auth type for human-interactive use. |
| Bulk content export (PDF/Word) | Some CLI tools offer export formats | Export endpoints produce binary blobs (PDF/Word), not JSON. Breaks the JSON-stdout contract. Agents cannot process binary output. | `raw` command for one-off export if needed. Not a first-class feature. |

---

## Feature Dependencies

```
[OpenAPI codegen]
    └──produces──> [Generated commands] (all resource CRUD)
                       └──requires──> [HTTP client with auth]
                                          └──requires──> [Profile / config system]

[Profile / config system]
    └──enables──> [Multi-profile auth]
    └──enables──> [Operation policy]
    └──enables──> [Audit logging]
    └──enables──> [Preset system]

[HTTP client with auth]
    └──enables──> [Automatic pagination]
    └──enables──> [Response caching]
    └──enables──> [JQ filtering]
    └──enables──> [Dry-run mode]
    └──enables──> [Verbose mode]

[Generated commands]
    └──enhances──> [Raw command] (escape hatch for unmapped endpoints)

[Batch command]
    └──requires──> [Generated commands] (dispatches to them)
    └──requires──> [Semantic exit codes] (reports per-op exit codes)

[CQL search]
    └──requires──> [Automatic pagination] (search results are paginated)

[Preset system]
    └──enhances──> [JQ filtering] (presets expand to --jq defaults)
    └──enhances──> [--fields] (presets expand to --fields defaults)

[Operation policy] ──conflicts with──> [Dry-run mode as a bypass]
    Note: policy must be enforced even in dry-run — dry-run shows what would happen but must still validate policy
```

### Dependency Notes

- **Generated commands require profile/config system:** The HTTP client is initialized in `PersistentPreRunE` from the resolved profile. All generated commands inherit this — none can run without it.
- **Batch requires generated commands:** Batch dispatches operations by name (e.g., `"command": "page get"`) to the same codegen command registry. Must be built after codegen pipeline is established.
- **Pagination required before CQL search:** CQL search returns paginated results. Auto-pagination must work correctly before search is reliable for agent use.
- **Preset enhances JQ/fields but is non-blocking:** Agents can use `--jq` directly. Presets are a convenience layer, safe to defer to later phases.
- **Policy must come with profiles:** If profiles exist without policy enforcement, the governance story is incomplete. Both belong in the same phase.

---

## MVP Definition

### Launch With (v1)

Minimum viable product — what an AI agent needs to perform real Confluence work.

- [ ] Profile / config system (`cf configure`, `~/.config/cf/config.json`, `--profile`, env vars) — nothing else works without auth
- [ ] OpenAPI codegen pipeline — generates all CRUD commands from spec; establishes the pattern
- [ ] Pages CRUD via generated commands — primary resource agents interact with
- [ ] Space list/get via generated commands — discovery of spaces before page operations
- [ ] CQL search — agents need search to find pages without knowing IDs upfront
- [ ] Pure JSON stdout + errors to stderr — non-negotiable for agent consumption
- [ ] Semantic exit codes — agents need to distinguish failure modes
- [ ] Automatic pagination — list results must be complete; partial results silently break agents
- [ ] JQ filtering (`--jq`) — agents reduce large responses in-call; reduces context window load
- [ ] Raw command — escape hatch for any gap between codegen and real API needs
- [ ] Schema command (JSON) — agents discover commands without parsing human-readable help
- [ ] Dry-run mode — standard safety feature for write operations; low effort to include early

### Add After Validation (v1.x)

Features to add once core CRUD is working and agents are using it.

- [ ] Operation policy (allow/deny) — add when agents are deployed in shared/governed environments
- [ ] Audit logging — add when teams need accountability for agent actions
- [ ] Response caching — add when agents report quota pressure or repeated reads slow down
- [ ] Preset system — add when agents/users have established recurring output patterns
- [ ] Comments CRUD via generated commands — secondary resource; not needed for page read/write
- [ ] Label management via generated commands — useful for categorization workflows
- [ ] Batch command — add when agent orchestration overhead (process spawning) is measurable

### Future Consideration (v2+)

- [ ] OAuth2 auth type — add only if human-interactive use cases emerge beyond API tokens
- [ ] Watch/polling command — add if agents need reactive (change-driven) rather than request-driven access
- [ ] Blog post CRUD — add when agents need to publish blog content (niche use case)
- [ ] Space permissions management — add if agent needs to provision access (admin scenario)
- [ ] Content version history/diff — add if agents need change tracking workflows

---

## Feature Prioritization Matrix

| Feature | Agent Value | Implementation Cost | Priority |
|---------|-------------|---------------------|----------|
| Profile / config system | HIGH | LOW | P1 |
| OpenAPI codegen pipeline | HIGH | HIGH | P1 |
| Pages CRUD | HIGH | LOW (via codegen) | P1 |
| Space list/get | HIGH | LOW (via codegen) | P1 |
| CQL search | HIGH | MEDIUM | P1 |
| Pure JSON stdout + stderr | HIGH | LOW | P1 |
| Semantic exit codes | HIGH | LOW | P1 |
| Automatic pagination | HIGH | MEDIUM | P1 |
| JQ filtering | HIGH | LOW | P1 |
| Raw command | HIGH | LOW | P1 |
| Schema discovery (JSON) | MEDIUM | LOW | P1 |
| Dry-run mode | MEDIUM | LOW | P1 |
| Operation policy | HIGH | MEDIUM | P2 |
| Audit logging | MEDIUM | MEDIUM | P2 |
| Preset system | MEDIUM | MEDIUM | P2 |
| Response caching | MEDIUM | MEDIUM | P2 |
| Comments CRUD | MEDIUM | LOW (via codegen) | P2 |
| Label management | MEDIUM | LOW (via codegen) | P2 |
| Batch command | MEDIUM | HIGH | P2 |
| Watch/polling | LOW | HIGH | P3 |
| OAuth2 auth | LOW | HIGH | P3 |
| Blog post CRUD | LOW | LOW (via codegen) | P3 |

**Priority key:**
- P1: Must have for launch — agent cannot function without it
- P2: Should have — adds significant value once core works
- P3: Nice to have — defer until P2 is stable

---

## Competitor Feature Analysis

| Feature | pchuri/confluence-cli (Python) | cfl / atlassian-cli (Go) | Appfire Confluence CLI (Java) | cf (this project) |
|---------|-------------------------------|--------------------------|-------------------------------|-------------------|
| Output format | Human text + optional JSON | Markdown-first, human text | Human text | Pure JSON always |
| AI agent optimized | Partial (read-only mode) | No | No | Yes (primary design goal) |
| OpenAPI codegen | No (hand-written) | No (hand-written) | No (hand-written) | Yes |
| JQ filtering | No | No | No | Yes (persistent flag) |
| Semantic exit codes | Basic (0/1) | Basic | Basic | Yes (typed exit codes) |
| Operation policy | No | No | No | Yes (allow/deny globs) |
| Audit logging | No | No | No | Yes (NDJSON) |
| Batch operations | No | No | No | Yes |
| Preset system | No | No | No | Yes |
| Pagination | Manual | Auto | Auto | Auto |
| Auth: Basic + Bearer | Yes | Yes | Yes | Yes |
| Auth: OAuth2 | No | No | Yes | Deferred |
| Dry-run mode | No | No | No | Yes |
| Response caching | No | No | No | Yes |
| Markdown conversion | Yes (differentiator) | Yes (core feature) | Partial | No (anti-feature) |
| Confluence v2 API | Partial | Partial | No (v1 focus) | Yes (v2 only) |
| Confluence v1 API | Yes | Yes | Yes | No (anti-feature) |

---

## Sources

- [pchuri/confluence-cli — GitHub](https://github.com/pchuri/confluence-cli) — feature list and README reviewed directly
- [open-cli-collective/atlassian-cli (cfl) — GitHub](https://github.com/open-cli-collective/atlassian-cli) — feature list reviewed
- [Confluence Cloud REST API v2 — Atlassian Developer](https://developer.atlassian.com/cloud/confluence/rest/v2/) — resource categories enumerated
- [Appfire Confluence CLI — Atlassian Marketplace](https://marketplace.atlassian.com/apps/284/confluence-command-line-interface-cli) — enterprise feature comparison
- [CQL field reference — Atlassian Developer](https://developer.atlassian.com/server/confluence/cql-field-reference/) — CQL scope and limitations
- [MCP vs CLI for AI agents — CircleCI blog](https://circleci.com/blog/mcp-vs-cli/) — CLI vs MCP tradeoffs for agent context
- [Why CLI Tools Are Beating MCP for AI Agents — Jan Reinhard, 2026](https://jannikreinhard.com/2026/02/22/why-cli-tools-are-beating-mcp-for-ai-agents/) — CLI efficiency advantage
- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` — examined directly (root.go, workflow.go, batch.go, watch.go, policy.go, preset.go)

---
*Feature research for: Confluence CLI (cf) — AI agent domain*
*Researched: 2026-03-20*
