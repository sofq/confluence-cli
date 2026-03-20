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

## v1.1 Milestone Pitfalls: OAuth2, Attachments, Watch, Templates

The following pitfalls are specific to the v1.1 features being added to the existing CLI.

---

### Pitfall 9: OAuth2 Token Storage Leaking Credentials in Config File

**What goes wrong:**
OAuth2 tokens (access + refresh) get stored alongside basic/bearer credentials in the existing `config.json` with 0o600 permissions. The refresh token is a long-lived secret equivalent to a password. If the config file is accidentally committed, backed up unencrypted, or read by another tool, both token types are exposed. Worse, Atlassian uses rotating refresh tokens, so a leaked old refresh token may still work briefly.

**Why it happens:**
The existing `AuthConfig` struct has `Type`, `Username`, and `Token` fields. Developers naturally add `RefreshToken`, `ClientID`, `ClientSecret`, `AccessTokenExpiry` to the same struct, treating it like any other config. The existing `SaveTo` writes JSON with `0o600`, which feels secure enough.

**How to avoid:**
- Store OAuth2 tokens in a separate file (`~/.config/cf/tokens.json`) from static credentials, so the token file can be `.gitignore`d independently and rotated without touching the main config.
- Add `AccessTokenExpiry` as a timestamp so the CLI knows when to refresh without hitting a 401 first.
- The `ClientID` and `ClientSecret` belong in the profile config (they are app credentials, not user credentials). Only the access/refresh token pair needs the separate token store.
- Never log token values in `--verbose` output. The existing `VerboseLog` in `client.go` does not log auth headers, which is correct -- maintain this discipline.

**Warning signs:**
- `RefreshToken` field appears in `config.json` alongside `base_url`
- No expiry tracking -- CLI always tries the token and waits for 401
- `--verbose` output showing `Authorization: Bearer <actual-token>`

**Phase to address:** OAuth2 authentication phase (first phase of v1.1)

---

### Pitfall 10: Attachment Upload Breaks the JSON Stdout Contract

**What goes wrong:**
The CLI has a strict contract: stdout is always valid JSON, stderr is for errors. Attachment upload via v1 API returns a JSON response (the attachment metadata), which fits the existing `WriteOutput` pipeline. But attachment *download* (which users will expect once upload exists) returns binary content that cannot be written to stdout without breaking every downstream consumer that does `cf attachments get-by-id <id> | jq ...`.

**Why it happens:**
Developers implement upload (which returns JSON), then naturally add download. The download response is binary (image, PDF, etc.), which breaks `WriteOutput` because it tries to apply JQ filtering and pretty-printing to non-JSON bytes.

**How to avoid:**
- Upload: use the v1 endpoint `POST /wiki/rest/api/content/{id}/child/attachment` with `multipart/form-data`. The JSON response fits the existing `WriteOutput` pipeline.
- Download: if implemented, write binary to a file (`--output <path>` flag), never to stdout. Stdout should still emit JSON metadata about what was downloaded (`{"path": "/tmp/file.png", "size": 12345}`).
- The `Client.Do` method hardcodes `Content-Type: application/json` for request bodies (`client.go` line 196). Attachment upload needs `multipart/form-data` -- this requires a new code path (e.g., `DoMultipart`), not a hack on `Do`.
- The `Client.Do` method also sets `Accept: application/json` (`client.go` line 194). The v1 attachment upload endpoint returns JSON, so this works.

**Warning signs:**
- `Client.Do` being called with `Content-Type: multipart/form-data` via header override hacks
- Binary content appearing on stdout in test output
- JQ filter errors on attachment operations

**Phase to address:** Attachment upload phase -- design the upload-specific `DoMultipart` method before implementing any attachment write commands

---

### Pitfall 11: Mixed v1/v2 API Base URL Construction

**What goes wrong:**
The existing CLI builds URLs as `BaseURL + path` where `BaseURL` is something like `https://mysite.atlassian.net/wiki/api/v2`. All generated commands use v2 paths like `/pages`, `/spaces`. Attachment upload needs v1 paths like `/wiki/rest/api/content/{id}/child/attachment`. If the developer naively does `BaseURL + v1Path`, the URL becomes `https://mysite.atlassian.net/wiki/api/v2/wiki/rest/api/content/...` -- doubly-prefixed and broken.

**Why it happens:**
The `BaseURL` in config already includes `/wiki/api/v2`. The v1 attachment endpoint has its own full path. `Client.Do` in `client.go` line 105-106 simply concatenates: `rawURL := c.BaseURL + path`. There is no path resolution logic. This codebase has already had this exact bug -- commit `a6e99ef` fixed `/wiki/api/v2` prefix doubling in the `--test` connection check.

**How to avoid:**
- Extract the site root from `BaseURL` (strip `/wiki/api/v2` suffix) and use that for v1 API calls. Add a computed `SiteRoot()` method: `strings.TrimSuffix(BaseURL, "/wiki/api/v2")`.
- Create a dedicated method like `Client.DoV1(ctx, method, v1Path, query, body)` that uses the site root instead of BaseURL.
- Alternatively, store only the site root in config and append API version prefixes in the client -- but this is a bigger refactor that breaks existing configs.

**Warning signs:**
- 404 errors on attachment operations that work fine in curl
- URLs in `--verbose` output containing `/wiki/api/v2/wiki/rest/api/`
- Tests passing with hardcoded base URLs that skip the prefix

**Phase to address:** Attachment upload phase -- must be solved before any v1 endpoint integration

---

### Pitfall 12: OAuth2 Token Refresh Race Condition in Concurrent/Batch Operations

**What goes wrong:**
The batch command runs multiple API operations sequentially. If the access token expires mid-batch, the first 401 triggers a refresh. But Atlassian uses rotating refresh tokens: each refresh invalidates the previous refresh token. If two sequential requests both detect 401 and both try to refresh, the second refresh fails with "invalid refresh token" because the first already rotated it.

**Why it happens:**
The existing `Client` struct stores auth as a simple `AuthConfig` value with no refresh logic. There is no mutex, no single-flight pattern. The `ApplyAuth` method (`client.go` line 79-87) reads `c.Auth.Token` directly.

**How to avoid:**
- Implement token refresh as a single-flight operation using `golang.org/x/sync/singleflight` or a mutex-protected refresh method that returns the new token to all waiters.
- Proactively refresh before expiry: if `AccessTokenExpiry` is within 60 seconds, refresh before making the request. This avoids the 401-triggered race entirely.
- After refresh, update the token file atomically (write temp file + rename) and update the in-memory `Client.Auth.Token`.
- With OAuth2, the `ApplyAuth` method must go through a `TokenSource` interface that handles refresh transparently.

**Warning signs:**
- "Unknown or invalid refresh token" errors in batch operations
- Intermittent 401 errors that resolve on retry
- Token file being written by multiple goroutines

**Phase to address:** OAuth2 authentication phase -- the token refresh mechanism must be correct from day one

---

### Pitfall 13: Watch Command Leaking Goroutines and HTTP Connections

**What goes wrong:**
A `cf watch pages --space KEY` command polls the API every N seconds and emits NDJSON events for changes. If the user hits Ctrl+C, the polling goroutine, HTTP connections, and file descriptors must all be cleaned up. Without proper signal handling, the process hangs, leaks connections, or produces partial JSON on stdout (breaking the NDJSON contract).

**Why it happens:**
The existing CLI is request-response: one command, one HTTP call (or a pagination chain), done. The `Client.Do` method returns an int exit code. There is no long-running loop, no signal handling beyond what Cobra provides, no context cancellation wired to OS signals. The watch command is a fundamentally different execution model.

**How to avoid:**
- Use `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` to create a cancellable context for the watch loop.
- The polling loop must check `ctx.Done()` between iterations, not just between HTTP calls.
- Buffer the current NDJSON line and only write complete lines to stdout. If cancelled mid-fetch, discard the partial result.
- Set a shutdown timeout (e.g., 5 seconds) so `--timeout` on the HTTP client does not prevent exit.
- Emit a final `{"type":"shutdown","reason":"signal"}` event on clean exit so consumers know the stream ended intentionally.

**Warning signs:**
- Process not exiting on Ctrl+C (hangs in HTTP read)
- Partial JSON lines appearing on stdout when interrupted
- `--verbose` showing requests after Ctrl+C was pressed
- Connection pool exhaustion after running watch for extended periods

**Phase to address:** Watch command phase

---

### Pitfall 14: OAuth2 Requires Accessible-Resources Lookup — Changes URL Construction

**What goes wrong:**
OAuth2 3LO for Atlassian Cloud requires calling `https://api.atlassian.com/oauth/token/accessible-resources` to get the `cloudId`, then using `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/api/v2/` as the base URL. This is a completely different URL scheme from basic auth (`https://mysite.atlassian.net/wiki/api/v2/`). If the CLI just swaps the auth header without changing the base URL, all API calls fail.

**Why it happens:**
Basic auth and API tokens use the site URL directly. OAuth2 3LO routes through `api.atlassian.com` as a gateway. The existing `BaseURL` in config points to `mysite.atlassian.net`, which is wrong for OAuth2. Client credentials grant for service accounts uses the same `api.atlassian.com` gateway.

**How to avoid:**
- During `cf configure` with `--auth-type oauth2`, automatically resolve the `cloudId` via the accessible-resources endpoint and store it in the profile.
- Compute the effective base URL at resolve time in `config.Resolve()`: if auth type is `oauth2`, construct `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/api/v2` instead of using the stored `base_url`.
- Keep `base_url` in config for basic/bearer (user-friendly), but add `cloud_id` field for oauth2 profiles.

**Warning signs:**
- OAuth2 profiles working for token exchange but failing for actual API calls
- 404 or "site not found" errors with valid OAuth2 tokens
- Users manually entering `api.atlassian.com` as base_url

**Phase to address:** OAuth2 authentication phase -- URL construction is inseparable from auth type

---

### Pitfall 15: Template Data Object Exposing Methods — SSTI Risk

**What goes wrong:**
Content templates use Go's `text/template` to expand variables (e.g., `{{.SpaceKey}}`, `{{.Title}}`). If the template data object is a Go struct with exported methods, template authors can call arbitrary methods. In a CLI context, this could leak environment variables or file contents if the data object has broad access.

**Why it happens:**
Go's `text/template` executes any exported method on the data object passed to `Execute()`. Developers naturally use a struct with helper methods as the data context.

**How to avoid:**
- Use `map[string]string` as the template data type, never a struct with methods. Maps have no callable methods in Go templates.
- Ship built-in templates as embedded Go files (`embed.FS`), not loaded from the config directory where they could be tampered with.
- For user-defined templates, document that they are "trusted code" (like a Makefile) and should not come from untrusted sources.

**Warning signs:**
- Template data object is a struct with exported methods
- Templates loaded from API responses or untrusted network sources
- No template parsing/validation before execution

**Phase to address:** Template system phase -- design the data model to be a plain map from the start

---

### Pitfall 16: Missing `offline_access` Scope in OAuth2 Browser Flow

**What goes wrong:**
The OAuth2 3LO authorization request must include `offline_access` in the scope parameter to receive a refresh token. Without it, Atlassian returns only an access token (valid for ~60 minutes). After expiry, the user must re-authorize via browser -- completely unusable for CLI automation and agent workflows.

**Why it happens:**
The `offline_access` scope is not a Confluence-specific scope -- it is an OAuth2 standard scope that developers forget to include. The authorization flow "works" without it (you get a valid access token), so the omission is not caught until the token expires an hour later.

**How to avoid:**
- Hardcode `offline_access` into the scope list for the browser flow. It is not optional for CLI use.
- Validate during `cf configure --auth-type oauth2` that the stored token includes a refresh token. If not, warn the user.
- In the token response handler, check for the presence of `refresh_token`. If absent, emit a warning: `"No refresh token received. Re-authorize with 'offline_access' scope to enable automatic token refresh."`

**Warning signs:**
- Token exchange response has no `refresh_token` field
- CLI works for one hour then fails with 401 on all operations
- Users reporting "I have to re-authenticate every hour"

**Phase to address:** OAuth2 authentication phase

---

### Pitfall 17: Attachment Upload Missing `X-Atlassian-Token: no-check` Header

**What goes wrong:**
The Confluence v1 attachment upload endpoint requires the `X-Atlassian-Token: no-check` header on all multipart upload requests. Without it, Confluence rejects the request as a potential XSRF attack, returning a 403 with a confusing error message about "XSRF check failed."

**Why it happens:**
This header is a Confluence-specific XSRF protection mechanism. It is not part of standard HTTP multipart upload patterns. Developers who test with permissive dev environments may not encounter the error until production.

**How to avoid:**
- Always include `X-Atlassian-Token: no-check` in the `DoMultipart` method, not in individual command implementations. This prevents forgetting it on one command.
- The multipart form field for the file must be named exactly `file` (not `attachment`, not `upload`).

**Warning signs:**
- 403 errors on attachment upload with message containing "XSRF"
- Upload works via curl but fails in the CLI

**Phase to address:** Attachment upload phase

---

### Pitfall 18: OAuth2 Browser Flow Fails in Headless/CI Environments

**What goes wrong:**
The OAuth2 3LO browser flow opens a browser window for user authorization. In headless environments (CI, Docker, SSH sessions), there is no browser. The CLI hangs waiting for the callback that never comes, or crashes trying to open a browser.

**Why it happens:**
Developers test on their local machine with a browser. The code works perfectly. Then an agent or CI pipeline tries to use OAuth2 and discovers it requires interactive browser access.

**How to avoid:**
- Detect TTY with `os.Stdin` + `term.IsTerminal()`. If no TTY, print the authorization URL and ask the user to paste the authorization code manually (device-code-like pattern).
- For CI/automation, recommend client credentials grant (service accounts) instead of 3LO browser flow.
- Use port 0 for the local callback server (`net.Listen("tcp", ":0")`) to avoid port conflicts. Extract the OS-assigned port after `Listen()`.
- Add a `--no-browser` flag to force the manual paste flow even in interactive terminals.

**Warning signs:**
- CLI hanging after printing "Opening browser for authorization..."
- Port 8080/8888 conflicts on developer machines
- CI logs showing "failed to open browser" errors

**Phase to address:** OAuth2 authentication phase

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Using v1 API for attachment writes | Unblocks attachment support immediately | Requires v1+v2 dual-client; maintenance burden when v1 deprecated | MVP only -- plan v2 migration |
| Skipping `--purge` flag on delete | Simpler initial implementation | Agents accumulate trash, title conflicts on re-create | Never -- purge flag must ship with delete |
| Hardcoding `body-format=storage` (no flag) | Simpler command surface | Agents that want ADF format have no option | Acceptable for v1 of CLI |
| Skipping space key resolution cache | Avoids cache complexity | Extra API call per space-scoped operation; rate limit risk | Early development only -- cache before first release |
| Pinning OpenAPI spec without update process | Stable code generation | Spec drift causes missed new endpoints | Acceptable if spec update process is documented |
| Hardcoding v1 attachment URL in command files | Fast to implement | Every v1 endpoint repeats the URL construction logic; if base URL format changes, all break | Never -- extract to `Client.V1BaseURL()` or `Client.SiteRoot()` method |
| Storing OAuth2 tokens in same config.json | No new file to manage | Token rotation writes mix with profile edits; file locking issues | Never -- separate token store |
| Polling with `time.Sleep` in watch loop | Simple implementation | Cannot respond to signals during sleep; sleep timer not cancelled on context done | Only in prototype -- use `time.NewTimer` + `select` with `ctx.Done()` |
| Skipping `X-Atlassian-Token: no-check` header | Works in some dev setups | Fails in production with XSRF protection enabled | Never -- always include it |
| Embedding OAuth2 client secret in binary | No config needed for built-in app | Secret extracted via `strings` command on binary | Never -- require user to provide credentials |

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
| Confluence v1 attachment upload | Omitting `X-Atlassian-Token: no-check` header | Always include this header -- Confluence rejects multipart uploads without it as XSRF protection |
| Confluence v1 attachment upload | Using `application/json` Content-Type | Must use `multipart/form-data` with the file part named exactly `file` |
| Atlassian OAuth2 3LO | Using site URL (`mysite.atlassian.net`) with OAuth2 token | Must use `api.atlassian.com/ex/confluence/{cloudId}` gateway URL |
| Atlassian OAuth2 refresh | Retrying refresh with already-rotated token | Use singleflight pattern; the first successful refresh returns the new token to all callers |
| Atlassian OAuth2 scopes | Requesting broad scopes or forgetting `offline_access` | Request minimum scopes plus `offline_access` for refresh token |
| Atlassian OAuth2 3LO browser flow | Hardcoding `localhost:PORT` for callback | Use port 0 (OS-assigned) to avoid conflicts; extract assigned port after `Listen()` |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unpaginated list requests on large spaces | Truncated results, user thinks full list was returned | Always use `--paginate` flag; warn when response `_links.next` is present | Spaces with >50 pages |
| Auto-pagination without delay | 429 rate limit errors mid-traversal | Respect `Retry-After` header; add configurable delay between pages | ~100+ sequential requests/hour (OAuth2) |
| Caching without profile-scoping | User A's cached response served to user B when profiles share a cache dir | Cache key must include profile name or base-url+username | Multi-profile setups |
| Re-requesting space ID on every command | Extra API call latency for every space-scoped command | Cache space key to ID mappings with short TTL | Every invocation |
| Cursor length exceeding URL limits in CQL search | 413 errors on page 2+ of search results | Detect cursor length, surface error with hint | Large CQL result sets (any depth > page 1) |
| Watch command polling too aggressively | 429 rate limit errors from Confluence | Default interval of 30s minimum; respect `Retry-After` header; exponential backoff on errors | Immediately with intervals under 10s; at scale with multiple watchers |
| Caching attachment metadata but not invalidating on upload | Stale attachment lists after upload | Invalidate cache entries matching the parent page after any attachment write operation | First time user uploads then lists in same session |
| Buffering all NDJSON watch events in memory | OOM on long-running watch sessions | Stream events directly to stdout; never accumulate a slice of events | After hours of watching active spaces |
| Token refresh on every request when clock skew exists | Excessive token refreshes hammering auth endpoint | Add 30-second buffer to expiry check; tolerate minor clock skew | When system clock drifts or in containers with poor NTP |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing API token in config file with world-readable permissions | Token exposed to other local users | Config file must be written with mode `0600`; check on load, warn if permissions are wrong |
| Logging full request URL to stderr in verbose mode | URL may contain sensitive CQL queries or page IDs in query params | Verbose logging is acceptable but must not log `Authorization` header; already handled in `jr` client |
| Cache files world-readable | Cached API responses (may include sensitive page content) accessible to other users | Cache dir and files must use mode `0700`/`0600`; `jr` cache already does this -- mirror exactly |
| Passing API token as a CLI flag (vs env var or config) | Token visible in `ps` output and shell history | Prefer `CF_TOKEN` env var and config file; warn in docs against `--token` flag in scripts |
| Logging OAuth2 tokens in `--verbose` or audit log | Token theft from log files | Redact `Authorization` header values in verbose output; never log token values in audit entries |
| Storing client_secret in plaintext config | Credential theft if config file exposed | Document that client_secret should come from environment variable `CF_OAUTH_CLIENT_SECRET`, not config file; warn on `cf configure` if client_secret is being saved to file |
| Not validating OAuth2 `state` parameter in browser flow | CSRF attacks redirecting auth codes to attacker | Generate cryptographic random state, store in memory, validate on callback |
| Template expansion reading local files | Information disclosure via template injection | Use `map[string]string` data type (no methods); do not add `include` or `file` template functions |

---

## UX Pitfalls

| Pitfall | User/Agent Impact | Better Approach |
|---------|-------------------|-----------------|
| Non-JSON error output when Confluence returns HTML error pages | Agent parser fails on HTML; breaks structured error contract | Detect HTML responses and replace with structured JSON error (already in `jr` -- mirror `sanitizeBody`) |
| Ambiguous "delete" semantics | Agent thinks content is gone; it's in trash | `pages delete` output must include `"trashed": true` in JSON and a hint about `--purge` |
| No hint when 401 is returned | Agent retries forever or fails silently | 401 must include hint pointing to `cf configure` (mirror `jr` hint pattern) |
| Inconsistent ID vs key UX across commands | Some commands take IDs, some take keys | Standardize: all commands accept space keys (resolved internally) and page IDs |
| Silent truncation when JQ filter matches nothing | Agent interprets empty output as success | Output `[]` or `{}` for empty JQ results, never empty string; exit 0 is correct |
| OAuth2 browser flow with no fallback | Fails in headless/CI environments | Detect TTY; if no TTY, print the auth URL and ask user to paste the code manually |
| Silent token refresh with no indication | User confused when `--verbose` shows different token than configured | Log `{"type":"token_refresh","message":"access token refreshed"}` to stderr in verbose mode |
| Watch command with no initial state | First event is "changed" but user does not know "from what" | Emit initial snapshot event with `{"type":"snapshot","data":[...]}` before starting change polling |
| Template errors showing Go template syntax | Non-Go users confused by `template: :1: unexpected "}" in operand` | Wrap template errors with user-friendly messages: `"template error at position 15: unexpected closing brace"` |
| `cf attachments upload` expecting `--file` but user pipes stdin | Inconsistent with how other CLI tools work | Support both: `--file <path>` and stdin piping with `--filename <name>` for the Content-Disposition |

---

## "Looks Done But Isn't" Checklist

### v1.0 Items
- [ ] **Pages GET:** Verify `body` field is non-empty in JSON output -- omitting `body-format=storage` produces silent empty body.
- [ ] **Pages UPDATE:** Verify concurrent update produces structured `conflict` error (exit 6), not a raw 409 body or panic.
- [ ] **Pages DELETE:** Verify `--purge` flag exists and surfaces 403 with admin hint when user lacks permission.
- [ ] **Spaces LIST:** Verify `--key ENG` resolves to numeric ID transparently without user needing to know the ID.
- [ ] **Search:** Verify pagination past page 1 does not 413 -- test with a space that has >50 pages.
- [ ] **Error output:** Verify all 4xx/5xx responses produce JSON on stderr, never raw HTML.
- [ ] **Exit codes:** Verify `echo $?` after each error type matches the semantic codes (2=auth, 3=not_found, 4=validation, 5=rate_limit, 6=conflict, 7=server).
- [ ] **Config file:** Verify permissions are `0600` after `cf configure` writes the file.
- [ ] **Binary name:** Verify `cf --version` returns Confluence CLI version string, not Cloud Foundry output.
- [ ] **Attachment commands:** Verify README documents v1 API fallback for write operations if v2 not yet supported.

### v1.1 Items
- [ ] **OAuth2 browser flow:** Verify `offline_access` scope is included -- without it, no refresh token is issued, and the user must re-auth every hour
- [ ] **OAuth2 token refresh:** Verify atomic file write for token store -- concurrent CLI invocations must not corrupt the token file
- [ ] **OAuth2 URL construction:** Verify OAuth2 profiles use `api.atlassian.com/ex/confluence/{cloudId}` gateway URL, not the site URL
- [ ] **OAuth2 configure:** Verify `cloudId` is resolved and stored during `cf configure --auth-type oauth2` -- profile saved without it causes every subsequent command to fail
- [ ] **Attachment upload:** Verify `X-Atlassian-Token: no-check` header is present -- works in dev, fails in production without it
- [ ] **Attachment upload:** Verify non-ASCII filenames are properly encoded in Content-Disposition header
- [ ] **Attachment upload:** Verify `cf attachments upload --file x.png | jq .id` works -- JSON response on stdout, not binary
- [ ] **Watch command:** Verify Ctrl+C exits cleanly within 2 seconds -- no hanging on HTTP read
- [ ] **Watch command:** Verify no partial JSON lines on stdout when interrupted mid-fetch
- [ ] **Watch command:** Verify duplicate detection -- same page edit does not produce multiple events
- [ ] **Template system:** Verify required variables produce an error when missing, not silent `<no value>` output
- [ ] **Template system:** Verify template data type is `map[string]string` (no callable methods)
- [ ] **Output presets:** Verify invalid JQ in a preset is caught at load time, not at execution time

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
| Token stored in config.json alongside credentials | LOW | Move to separate file; add migration in `cf configure` that detects old format |
| Broken v1/v2 URL construction | MEDIUM | Refactor `Client` to have `SiteRoot()` method; migration path for existing configs |
| Watch goroutine leak | LOW | Add context cancellation; no data loss since events are ephemeral |
| Template SSTI exposure | LOW | Switch data type from struct to map; no user-facing change |
| OAuth2 race on token refresh | HIGH | Requires redesigning auth layer to use `TokenSource` pattern with singleflight; affects all HTTP calls |
| Missing XSRF header on uploads | LOW | Add header to `DoMultipart`; one-line fix once identified |
| Missing `offline_access` scope | MEDIUM | Re-authorize all OAuth2 users; stored tokens without refresh are useless |

---

## Pitfall-to-Phase Mapping

### v1.0 Pitfalls
| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Version conflict on page update | Pages CRUD phase | Integration test: concurrent update returns structured `conflict` error |
| Empty body on page GET | Pages CRUD phase | Integration test: GET page returns non-empty `body.storage.value` |
| Space key vs ID mismatch | Spaces command phase | Integration test: `--key` flag resolves to correct numeric ID |
| OpenAPI spec incompleteness | Code generation phase | CI: generated code compiles cleanly; attachment commands noted as v1-only |
| Soft-delete vs purge confusion | Pages CRUD phase | Manual test: delete + recreate same title succeeds with `--purge` |
| CQL cursor 413 | Search/CQL phase | Integration test: paginate search result set >50 items |
| Binary name `cf` collision | Release/packaging phase | Install docs; verification step in README |
| Rate limit 429 handling | Client setup phase | Unit test: 429 response produces `rate_limited` JSON error with `retry_after` field |
| Config file permissions | Configuration phase | Test: config file mode is `0600` after write |
| HTML error response sanitization | Client setup phase | Unit test: HTML body response produces structured JSON error, not raw HTML |

### v1.1 Pitfalls
| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| OAuth2 token storage leak | OAuth2 auth phase | Token file separate from config; `cat config.json` shows no access/refresh tokens |
| JSON stdout contract break (attachments) | Attachment upload phase | `cf attachments upload --file x.png \| jq .id` works; no binary on stdout |
| Mixed v1/v2 URL construction | Attachment upload phase | `--verbose` shows correct v1 URL for upload, v2 URL for list |
| Token refresh race condition | OAuth2 auth phase | Batch of 10 operations with expired token: all succeed, only one refresh logged |
| Watch goroutine leak | Watch command phase | `cf watch pages --space X` exits cleanly within 2s of Ctrl+C |
| OAuth2 URL gateway change | OAuth2 auth phase | OAuth2 profile API calls use `api.atlassian.com` URL in `--verbose` |
| Template SSTI | Template system phase | Template data type is `map[string]string` in code review |
| Missing `offline_access` scope | OAuth2 auth phase | Token exchange response includes `refresh_token` field |
| Missing XSRF header on uploads | Attachment upload phase | Upload succeeds against production Confluence instance |
| OAuth2 headless environment failure | OAuth2 auth phase | `cf configure --auth-type oauth2 --no-browser` prints URL and accepts pasted code |
| Missing `cloudId` resolution | OAuth2 auth phase | `cf configure --auth-type oauth2 --test` resolves and stores cloudId |

---

## Sources

### v1.0 Sources
- [Confluence Cloud REST API v2 -- Introduction](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/) -- official endpoint docs and body-format parameter
- [Confluence API: Page Updater Guide (Cotera)](https://cotera.co/articles/confluence-api-integration-guide) -- version conflict and optimistic locking behavior
- [Confluence rate limiting -- official docs](https://developer.atlassian.com/cloud/confluence/rate-limiting/) -- 429 handling, Retry-After, points-based model
- [Evolving API rate limits -- Atlassian blog](https://www.atlassian.com/blog/platform/evolving-api-rate-limits) -- March 2, 2026 enforcement date for OAuth2 apps
- [CQL cursor 413 issue -- Atlassian Developer Community (Sep 2025)](https://community.developer.atlassian.com/t/confluence-rest-v1-search-endpoint-fails-cursor-of-next-url-is-extraordinarily-long-leading-to-413-error/95098)
- [Get Body of a Page through API v2 -- Developer Community](https://community.developer.atlassian.com/t/get-body-of-a-page-through-api-v2/67966) -- empty body without body-format parameter
- [Confluence Cloud API v2 Space ID vs Key](https://community.atlassian.com/forums/Confluence-questions/How-to-get-space-ID-on-the-UI-or-how-to-utilise-space-key-in-v2/qaq-p/2680647) -- numeric ID requirement
- [OpenAPI spec incomplete -- Community report](https://community.atlassian.com/forums/Confluence-questions/OpenAPI-specification-seems-incomplete/qaq-p/2570847)
- [Error generating code from Confluence OpenAPI -- oapi-codegen issue #721](https://github.com/oapi-codegen/oapi-codegen/issues/721)
- [v2 attachment write operations missing -- Community](https://community.developer.atlassian.com/t/confluence-rest-api-v2-needs-ability-to-fetch-multiple-content-properties-plus-missing-attachment-trash/68568)
- [Deprecating many Confluence v1 APIs -- Atlassian announcement](https://community.developer.atlassian.com/t/deprecating-many-confluence-v1-apis-that-have-v2-equivalents/66883)
- [Delete, restore, or purge -- Confluence Cloud support](https://support.atlassian.com/confluence-cloud/docs/delete-restore-or-purge-a-page/) -- soft-delete vs purge behavior
- [CQL execution inconsistencies -- Developer Community](https://community.developer.atlassian.com/t/cql-execution-inconsistencies-on-the-rest-api/63365)
- [Basic auth for REST APIs -- Confluence Cloud](https://developer.atlassian.com/cloud/confluence/basic-auth-for-rest-apis/) -- API token authentication
- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/errors/errors.go` -- exit code mapping, HTML sanitization, Retry-After parsing

### v1.1 Sources
- [Confluence Cloud REST API v2 -- Attachment endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-attachment/) -- confirmed no POST/PUT for attachments in v2
- [Confluence Cloud REST API v1 -- Content Attachments](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content---attachments/) -- v1 upload endpoints with multipart/form-data
- [Community: Upload Attachment via API v2](https://community.atlassian.com/forums/Confluence-questions/Upload-Attachment-via-API-v2/qaq-p/2352854) -- confirmed v1 required for upload
- [Atlassian OAuth 2.0 3LO apps](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) -- OAuth flow for Confluence Cloud
- [Implementing OAuth 2.0 3LO](https://developer.atlassian.com/cloud/oauth/getting-started/implementing-oauth-3lo/) -- token endpoint at `https://auth.atlassian.com/oauth/token`
- [Refresh Token Flow](https://developer.atlassian.com/cloud/oauth/getting-started/refresh-tokens/) -- rotating refresh tokens, `offline_access` scope requirement
- [OAuth 2.0 credentials for service accounts](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) -- client credentials grant (2LO)
- [Community: OAuth rotating tokens issues](https://community.developer.atlassian.com/t/oauth-rotating-tokens-unknown-or-invalid-refresh-token/54555) -- real-world refresh token race failures
- [Community: Access tokens invalidated on refresh](https://community.atlassian.com/forums/Jira-questions/Jira-Cloud-OAuth-2-0-Are-existing-access-tokens-invalidated-when/qaq-p/3141463) -- token invalidation behavior
- [Graceful Shutdown in Go: Practical Patterns](https://victoriametrics.com/blog/go-graceful-shutdown/) -- signal handling, context cancellation
- [Go SSTI vulnerabilities (Snyk)](https://snyk.io/articles/understanding-server-side-template-injection-in-golang/) -- template injection prevention with `map` data types
- Existing codebase: `internal/client/client.go` (URL construction, `ApplyAuth`, `WriteOutput`), `internal/config/config.go` (`AuthConfig` struct, `validAuthTypes`), `cmd/root.go` (client initialization)
- Commit `a6e99ef` in this repo -- prior URL doubling bug proving Pitfall 11 is a real pattern

---
*Pitfalls research for: Confluence CLI v1.1 extended capabilities (OAuth2, attachments, watch, templates)*
*Researched: 2026-03-20*
