# Pitfalls Research

**Domain:** Confluence Cloud v2 REST API CLI (Go/Cobra, code-generated from OpenAPI spec)
**Researched:** 2026-03-20
**Confidence:** HIGH (majority of pitfalls verified against official Atlassian docs and community reports)

---

## Critical Pitfalls

### Pitfall 1: Page Update Requires Version Increment — Silent 409 Failures

**What goes wrong:**
Update requests that don't include `version.number` incremented by exactly one are rejected with HTTP 409. Because the CLI reads a page and then writes it, any concurrent edit between the read and write causes the update to fail. If error handling doesn't map 409 to an explicit exit code and JSON error, agent callers receive a non-zero exit silently or a raw HTTP body they can't parse.

**Why it happens:**
Confluence uses optimistic locking. Developers assume a PUT just replaces the resource. The v2 docs mention versioning but don't make it obvious that the field is mandatory and must be the current version + 1, not the version you last read.

**How to avoid:**
- The `update page` command must always GET the current version first, then send `version.number = current + 1`.
- Map HTTP 409 to `ExitConflict` (exit code 6) with a structured JSON error on stderr that includes the resource ID and current version. (The reference `jr` errors package already does this — mirror it exactly.)
- For agent callers, include a hint like `"Fetch the current page version with 'cf pages get <id>' and retry with the updated version number."`

**Warning signs:**
- Intermittent update failures in tests that pass sequentially but fail when run concurrently.
- `409 Conflict` in API responses with message about stale version or `OptimisticLockException`.

**Phase to address:** Core pages CRUD phase (whichever phase implements `pages update`).

---

### Pitfall 2: v2 API Does Not Return Page Body by Default — Empty Content on GET

**What goes wrong:**
`GET /wiki/api/v2/pages/{id}` returns an empty `body` field unless the `?body-format=storage` (or `atlas_doc_format`) query parameter is explicitly set. Callers who omit this receive a page object with no content, which looks like success (HTTP 200) but is actually an incomplete response.

**Why it happens:**
The v2 API intentionally omits body content to improve performance by default. This differs from v1 where `expand=body.storage` was a well-known pattern. The query parameter requirement is documented but easy to miss, and generated code from the OpenAPI spec does not automatically include it.

**How to avoid:**
- The hand-written `pages get` wrapper command must always pass `body-format=storage` by default.
- For auto-generated commands, the code generator template or the command wrapper must inject `body-format` as a flag with a sensible default (`storage`).
- Integration tests must assert that body content is non-empty for a known test page.

**Warning signs:**
- `body` field is `null` or `{}` in GET responses.
- Tests that only validate status code 200 pass, but downstream consumers find no content.

**Phase to address:** Core pages CRUD phase; also in generator phase when deciding which query params to expose.

---

### Pitfall 3: v2 API Uses Space ID (Integer), Not Space Key — Forced Two-Step Lookup

**What goes wrong:**
The Confluence v2 API identifies spaces by numeric ID, not the human-readable space key (e.g., `"ENG"`). CLI users and agents naturally refer to spaces by key. Without a transparent key-to-ID resolution step, every space-scoped operation (list pages, search by space) fails with 400/404 unless the caller already knows the numeric ID.

**Why it happens:**
The v2 API was redesigned for performance and consistency: IDs are integers, not opaque strings. The v1 API accepted space keys directly. Developers building on v2 who come from v1 patterns hit this immediately.

**How to avoid:**
- The `spaces` command must expose a `--key` flag that transparently fetches the numeric ID via `GET /wiki/api/v2/spaces?keys=<key>` and uses that ID in subsequent calls.
- Document clearly in command help text: "Use `cf spaces list --key ENG` to resolve a space key to its numeric ID."
- Cache space-key-to-ID mappings with a short TTL (the cache module from `jr` is directly reusable here).

**Warning signs:**
- User or agent passes `--space ENG` and receives a 404 or 400 with a confusing error about invalid space ID.
- Integration tests that use hardcoded space keys rather than IDs fail against a fresh environment.

**Phase to address:** Spaces command phase and any command that accepts a space identifier.

---

### Pitfall 4: OpenAPI Spec for Confluence v2 Is Incomplete — Generated Code Misses Endpoints or Has Wrong Types

**What goes wrong:**
The Confluence Cloud v2 OpenAPI spec (`openapi-v2.v3.json`) has documented gaps: missing types, incorrect field definitions, and some endpoints missing from the spec entirely (e.g., attachment write operations are v1-only as of early 2025). Code generators targeting this spec will silently skip or mistype those endpoints. Generated CLI commands for missing endpoints will not exist, and malformed type definitions cause compile errors or runtime panics.

**Why it happens:**
Atlassian publishes the spec but has not reached full v2 feature parity with v1. The spec is also used for documentation, so some fields are documented incompletely. Community reports confirm `GenericLinks` type is missing fields and `ContentBody` is missing `ContentBodyExpandable`.

**How to avoid:**
- Pin the spec version at project start and commit the spec file to the repo (as `jr` does with `jira-v3-latest.json`).
- Run the code generator in CI; compile failures from spec issues are caught early.
- Maintain a `SPEC_GAPS.md` that lists known missing or broken endpoints and maps them to v1 fallback paths if needed.
- For attachment write operations (upload, update, delete), explicitly plan to use v1 API endpoints until v2 equivalents ship.

**Warning signs:**
- Code generator exits cleanly but some expected command groups are absent from the compiled binary.
- Compile errors referencing undefined types immediately after a spec update.
- `attachment` subcommands only support GET operations.

**Phase to address:** Code generation phase (the very first phase); also in attachment command phase.

---

### Pitfall 5: Deletion Is Soft by Default — "Delete" Does Not Actually Delete

**What goes wrong:**
`DELETE /wiki/api/v2/pages/{id}` moves the page to trash. The page still exists, is returned in some queries, and consumes storage. Agents or CI scripts that delete-and-recreate pages loop indefinitely because the old trashed page title conflicts with the new one. Permanent deletion ("purge") is a separate operation that requires admin permissions and is gated behind a Confluence admin setting.

**Why it happens:**
Confluence's UX distinguishes trash from permanent deletion, and the API mirrors this. Most CLI authors mirror HTTP DELETE semantics (permanent) without checking this behavior.

**How to avoid:**
- The `pages delete` command help text must explicitly state: "Moves page to trash. Use `--purge` to permanently delete (requires admin permissions)."
- Implement `--purge` as an explicit flag that calls the purge API endpoint separately.
- Structured JSON error output should include a hint when a 403 is received during purge: "Purge requires Confluence admin permissions and the purge setting to be enabled."
- Do not attempt to handle purge as the default delete path.

**Warning signs:**
- Page count in a space does not decrease after running `cf pages delete`.
- Recreating a page with the same title under the same parent fails with a title-conflict error.
- Integration test cleanup leaves behind dozens of trashed pages.

**Phase to address:** Pages CRUD phase.

---

### Pitfall 6: CQL Cursor Pagination Can Produce 413 Errors — Long Cursor Strings

**What goes wrong:**
As of September 2025, the Confluence v2 CQL search endpoint generates cursor values that are up to ~11,000 characters long. When the cursor is passed back as a URL query parameter in subsequent pagination calls, some reverse proxies and Confluence's own infrastructure return HTTP 413 (Request Entity Too Large). Pagination through large search result sets becomes unreliable.

**Why it happens:**
Atlassian changed cursor encoding in the search endpoint. This is a known, unfixed issue reported in the developer community (as of the research date). The cursor string exceeds typical URL length limits.

**How to avoid:**
- The pagination handler must detect 413 responses and surface them as a specific error: `"pagination_error"` with a hint explaining the cursor length limitation.
- For agent-facing usage, recommend page-by-page traversal strategies that avoid deep cursor pagination when result sets are large.
- Monitor the Atlassian developer community and update the pagination handler if the cursor encoding changes.

**Warning signs:**
- `search` commands succeed on the first page but fail with 413 on subsequent pages.
- Very long `cursor` values (thousands of characters) in the `_links.next` field.

**Phase to address:** Search/CQL phase; also pagination handler implementation.

---

### Pitfall 7: Binary Named `cf` Collides With Cloud Foundry CLI

**What goes wrong:**
Many developer and CI environments have the Cloud Foundry CLI installed, which is also named `cf`. On systems where both are on `$PATH`, the wrong binary is invoked depending on `$PATH` ordering. Agents that invoke `cf` expecting the Confluence CLI may be silently invoking Cloud Foundry commands, causing confusing errors or unintended Cloud Foundry operations.

**Why it happens:**
The binary name `cf` mirrors the `jr` pattern (short, ergonomic), but `cf` is an established, widely-used command name for a different tool.

**How to avoid:**
- Document the naming collision explicitly in the README and installation guide.
- Provide a `--version` subcommand and a distinctive startup message so the correct binary can be verified.
- Do not change the binary name (it matches the design intent), but instruct users to alias or verify `$PATH` order explicitly.
- Detect the conflict in documentation: "If `cf --help` shows Cloud Foundry output, your PATH ordering needs adjustment."

**Warning signs:**
- `cf --help` shows Cloud Foundry command list instead of Confluence commands.
- CI pipelines fail with CF-specific error messages when running Confluence CLI commands.

**Phase to address:** Release/packaging phase; also README and installation docs.

---

### Pitfall 8: Rate Limiting Changes — Points-Based Quota Enforced March 2, 2026

**What goes wrong:**
As of March 2, 2026, Atlassian enforces a points-based rate limit on OAuth 2.0, Forge, and Connect apps. Each API call costs points based on the work performed. Burst-heavy usage patterns (e.g., paginating through all pages in a large space, bulk label operations) can exhaust the hourly quota unexpectedly. The CLI will receive HTTP 429 responses with a `Retry-After` header.

**Why it happens:**
The new quota model is more complex than simple req/sec limits. The points cost of an operation is not documented per-endpoint, so it's hard to predict budget consumption. CLI tools that make many sequential GET requests (auto-pagination) are particularly at risk.

**Note:** API token-based traffic is explicitly excluded from the new points-based model and governed by the existing burst limit only. This CLI uses API token auth, which means the new quota system does not apply as long as tokens are used. However, if the project adds OAuth2 3LO support, this becomes critical.

**How to avoid:**
- The 429 handler (already present in the `jr` errors package) must parse `Retry-After` and surface it in the structured error JSON — mirror this directly.
- Auto-pagination must respect `Retry-After` delays between page fetches.
- Document: "Bulk operations against large spaces may hit rate limits. Use `--limit` flags to reduce page sizes."

**Warning signs:**
- 429 responses during `--paginate` operations on large spaces.
- `Retry-After` header present in 429 responses.

**Phase to address:** Pagination handler phase; rate-limit error handling should be in the initial client setup phase.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Using v1 API for attachment writes | Unblocks attachment support immediately | Requires v1+v2 dual-client; maintenance burden when v1 deprecated | MVP only — plan v2 migration |
| Skipping `--purge` flag on delete | Simpler initial implementation | Agents accumulate trash, title conflicts on re-create | Never — purge flag must ship with delete |
| Hardcoding `body-format=storage` (no flag) | Simpler command surface | Agents that want ADF format have no option | Acceptable for v1 of CLI |
| Skipping space key resolution cache | Avoids cache complexity | Extra API call per space-scoped operation; rate limit risk | Early development only — cache before first release |
| Pinning OpenAPI spec without update process | Stable code generation | Spec drift causes missed new endpoints | Acceptable if spec update process is documented |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Confluence v2 GET page | Omit `body-format` query param | Always pass `?body-format=storage` for content retrieval |
| Confluence v2 PUT page | Send content without version increment | GET current version first, send `version.number + 1` |
| Confluence v2 DELETE page | Assume delete is permanent | Default is soft-delete to trash; purge is separate |
| Space identification | Pass space key string to v2 endpoints | Resolve key to numeric ID via `GET /spaces?keys=<key>` |
| CQL search pagination | Use offset-based pagination from v1 patterns | v2 uses cursor-based pagination only |
| CQL special characters | Pass raw strings with double quotes | URL-encode CQL values; double quotes require escaping |
| Attachment write operations | Call v2 attachment endpoints | v2 is read-only for attachments; write via v1 `/wiki/rest/api/content/{id}/child/attachment` |
| OAuth2 rate limits | Assume API token and OAuth2 behave the same | API tokens use burst limits; OAuth2 3LO apps use points-based quota (enforced March 2026) |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unpaginated list requests on large spaces | Truncated results, user thinks full list was returned | Always use `--paginate` flag; warn when response `_links.next` is present | Spaces with >50 pages |
| Auto-pagination without delay | 429 rate limit errors mid-traversal | Respect `Retry-After` header; add configurable delay between pages | ~100+ sequential requests/hour (OAuth2) |
| Caching without profile-scoping | User A's cached response served to user B when profiles share a cache dir | Cache key must include profile name or base-url+username | Multi-profile setups |
| Re-requesting space ID on every command | Extra API call latency for every space-scoped command | Cache space key → ID mappings with short TTL | Every invocation |
| Cursor length exceeding URL limits in CQL search | 413 errors on page 2+ of search results | Detect cursor length, surface error with hint | Large CQL result sets (any depth > page 1) |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing API token in config file with world-readable permissions | Token exposed to other local users | Config file must be written with mode `0600`; check on load, warn if permissions are wrong |
| Logging full request URL to stderr in verbose mode | URL may contain sensitive CQL queries or page IDs in query params | Verbose logging is acceptable but must not log `Authorization` header; already handled in `jr` client |
| Cache files world-readable | Cached API responses (may include sensitive page content) accessible to other users | Cache dir and files must use mode `0700`/`0600`; `jr` cache already does this — mirror exactly |
| Passing API token as a CLI flag (vs env var or config) | Token visible in `ps` output and shell history | Prefer `CF_TOKEN` env var and config file; warn in docs against `--token` flag in scripts |

---

## UX Pitfalls

| Pitfall | User/Agent Impact | Better Approach |
|---------|-------------------|-----------------|
| Non-JSON error output when Confluence returns HTML error pages | Agent parser fails on HTML; breaks structured error contract | Detect HTML responses and replace with structured JSON error (already in `jr` — mirror `sanitizeBody`) |
| Ambiguous "delete" semantics | Agent thinks content is gone; it's in trash | `pages delete` output must include `"trashed": true` in JSON and a hint about `--purge` |
| No hint when 401 is returned | Agent retries forever or fails silently | 401 must include hint pointing to `cf configure` (mirror `jr` hint pattern) |
| Inconsistent ID vs key UX across commands | Some commands take IDs, some take keys | Standardize: all commands accept space keys (resolved internally) and page IDs |
| Silent truncation when JQ filter matches nothing | Agent interprets empty output as success | Output `[]` or `{}` for empty JQ results, never empty string; exit 0 is correct |

---

## "Looks Done But Isn't" Checklist

- [ ] **Pages GET:** Verify `body` field is non-empty in JSON output — omitting `body-format=storage` produces silent empty body.
- [ ] **Pages UPDATE:** Verify concurrent update produces structured `conflict` error (exit 6), not a raw 409 body or panic.
- [ ] **Pages DELETE:** Verify `--purge` flag exists and surfaces 403 with admin hint when user lacks permission.
- [ ] **Spaces LIST:** Verify `--key ENG` resolves to numeric ID transparently without user needing to know the ID.
- [ ] **Search:** Verify pagination past page 1 does not 413 — test with a space that has >50 pages.
- [ ] **Error output:** Verify all 4xx/5xx responses produce JSON on stderr, never raw HTML.
- [ ] **Exit codes:** Verify `echo $?` after each error type matches the semantic codes (2=auth, 3=not_found, 4=validation, 5=rate_limit, 6=conflict, 7=server).
- [ ] **Config file:** Verify permissions are `0600` after `cf configure` writes the file.
- [ ] **Binary name:** Verify `cf --version` returns Confluence CLI version string, not Cloud Foundry output.
- [ ] **Attachment commands:** Verify README documents v1 API fallback for write operations if v2 not yet supported.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Version conflict on page update | LOW | Add GET-before-write in the update command; bump version by 1; no structural change needed |
| Empty body on page GET | LOW | Add `body-format=storage` to generated command query params; single-line fix |
| Space key not resolving | MEDIUM | Add key-resolution layer to spaces client; cache integration; touches multiple commands |
| OpenAPI spec type errors in generated code | MEDIUM | Fix generator templates or add post-generation patch; rerun codegen; no API change needed |
| Trash accumulation in tests | LOW | Add `--purge` flag to delete command; update test teardown |
| `cf` binary collision in production | LOW | Document `$PATH` fix; no code change required |
| 413 on CQL pagination | MEDIUM | Add cursor-length detection and error surfacing; may require pagination strategy change |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Version conflict on page update | Pages CRUD phase | Integration test: concurrent update returns structured `conflict` error |
| Empty body on page GET | Pages CRUD phase | Integration test: GET page returns non-empty `body.storage.value` |
| Space key vs ID mismatch | Spaces command phase | Integration test: `--key` flag resolves to correct numeric ID |
| OpenAPI spec incompleteness | Code generation phase | CI: generated code compiles cleanly; attachment commands noted as v1-only |
| Soft-delete vs purge confusion | Pages CRUD phase | Manual test: delete + recreate same title succeeds with `--purge` |
| CQL cursor 413 | Search/CQL phase | Integration test: paginate search result set >50 items |
| Binary name `cf` collision | Release/packaging phase | Install docs; verification step in README |
| Rate limit 429 handling | Client setup phase (Phase 1) | Unit test: 429 response produces `rate_limited` JSON error with `retry_after` field |
| Config file permissions | Configuration phase | Test: config file mode is `0600` after write |
| HTML error response sanitization | Client setup phase (Phase 1) | Unit test: HTML body response produces structured JSON error, not raw HTML |

---

## Sources

- [Confluence Cloud REST API v2 — Introduction](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/) — official endpoint docs and body-format parameter
- [Confluence API: Page Updater Guide (Cotera)](https://cotera.co/articles/confluence-api-integration-guide) — version conflict and optimistic locking behavior
- [Confluence rate limiting — official docs](https://developer.atlassian.com/cloud/confluence/rate-limiting/) — 429 handling, Retry-After, points-based model
- [Evolving API rate limits — Atlassian blog](https://www.atlassian.com/blog/platform/evolving-api-rate-limits) — March 2, 2026 enforcement date for OAuth2 apps
- [CQL cursor 413 issue — Atlassian Developer Community (Sep 2025)](https://community.developer.atlassian.com/t/confluence-rest-v1-search-endpoint-fails-cursor-of-next-url-is-extraordinarily-long-leading-to-413-error/95098)
- [Get Body of a Page through API v2 — Developer Community](https://community.developer.atlassian.com/t/get-body-of-a-page-through-api-v2/67966) — empty body without body-format parameter
- [Confluence Cloud API v2 Space ID vs Key](https://community.atlassian.com/forums/Confluence-questions/How-to-get-space-ID-on-the-UI-or-how-to-utilise-space-key-in-v2/qaq-p/2680647) — numeric ID requirement
- [OpenAPI spec incomplete — Community report](https://community.atlassian.com/forums/Confluence-questions/OpenAPI-specification-seems-incomplete/qaq-p/2570847)
- [Error generating code from Confluence OpenAPI — oapi-codegen issue #721](https://github.com/oapi-codegen/oapi-codegen/issues/721)
- [v2 attachment write operations missing — Community](https://community.developer.atlassian.com/t/confluence-rest-api-v2-needs-ability-to-fetch-multiple-content-properties-plus-missing-attachment-trash/68568)
- [Deprecating many Confluence v1 APIs — Atlassian announcement](https://community.developer.atlassian.com/t/deprecating-many-confluence-v1-apis-that-have-v2-equivalents/66883)
- [Delete, restore, or purge — Confluence Cloud support](https://support.atlassian.com/confluence-cloud/docs/delete-restore-or-purge-a-page/) — soft-delete vs purge behavior
- [CQL execution inconsistencies — Developer Community](https://community.developer.atlassian.com/t/cql-execution-inconsistencies-on-the-rest-api/63365)
- [Basic auth for REST APIs — Confluence Cloud](https://developer.atlassian.com/cloud/confluence/basic-auth-for-rest-apis/) — API token authentication
- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/errors/errors.go` — exit code mapping, HTML sanitization, Retry-After parsing

---
*Pitfalls research for: Confluence Cloud v2 REST API CLI (Go/Cobra)*
*Researched: 2026-03-20*
