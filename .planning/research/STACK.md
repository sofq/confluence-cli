# Stack Research

**Domain:** Confluence Cloud CLI (Go) -- v1.1 Stack Additions
**Researched:** 2026-03-20
**Confidence:** HIGH -- core stack verified against reference `jr` implementation, Atlassian official docs, and Go stdlib capabilities.

## Existing Stack (DO NOT CHANGE)

These are validated in v1.0 and remain unchanged:
- Go 1.25.8, Cobra v1.10.2, pflag v1.0.9, libopenapi v0.34.3, gojq v0.12.18
- net/http stdlib client, encoding/json, filesystem cache
- OpenAPI code generation pipeline via gen/ binary

## New Stack Additions for v1.1

### OAuth2 Authentication

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (net/http, net/url, encoding/json, crypto/rand, crypto/sha256) | (stdlib) | OAuth2 client_credentials grant and authorization code + PKCE flow | **No new dependencies.** The jr reference implements `fetchOAuth2Token()` for client_credentials in ~30 lines using stdlib `net/http` POST + `url.Values` + JSON decode. The 3LO browser flow needs a local HTTP callback server (`net/http.ListenAndServe` on localhost) and PKCE code challenge (`crypto/sha256` + `encoding/base64`). Both flows are simple enough that golang.org/x/oauth2 adds unnecessary complexity. |
| Go stdlib (os, encoding/json) | (stdlib) | Token persistence (refresh tokens, access token cache) | Store OAuth2 tokens in a separate file (`~/.config/cf/tokens.json`) alongside config.json. Tokens are short-lived (60 min for Atlassian) and refresh tokens rotate, so file-based persistence with 0600 permissions is the right model. |

**Atlassian OAuth2 Details (verified from official docs):**

| Parameter | Value | Source |
|-----------|-------|--------|
| Authorization URL | `https://auth.atlassian.com/authorize` | [Atlassian 3LO docs](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) |
| Token URL | `https://auth.atlassian.com/oauth/token` | Same |
| Audience | `api.atlassian.com` | Same |
| Accessible Resources | `GET https://api.atlassian.com/oauth/token/accessible-resources` | Same |
| API Base URL Pattern | `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2` | Same |
| Access Token Lifetime | 60 minutes | [Service account docs](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) |
| Refresh Token Expiry | 90 days inactivity | Atlassian 3LO docs |
| Client Credentials Support | YES, via service accounts (2LO) | [Service account OAuth2](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) |

**Key Scopes Needed:**

| Scope | Purpose |
|-------|---------|
| `read:confluence-content.all` | Read pages, blogs, attachments, custom content |
| `write:confluence-content` | Create/update pages, blogs, comments |
| `write:confluence-file` | Upload attachments |
| `read:confluence-space.summary` | List spaces |
| `search:confluence` | CQL search |
| `offline_access` | Obtain refresh tokens (3LO only) |

### Attachment Upload (Binary/Multipart)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (mime/multipart, io, os) | (stdlib) | Multipart form-data encoding for file uploads | **No new dependencies.** Go's `mime/multipart.Writer` handles multipart encoding. The pattern is: create a pipe, write file content via `multipart.Writer.CreateFormFile()`, set `X-Atlassian-Token: nocheck` header. This is well-documented stdlib usage. |

**Confluence Attachment API Details:**

Attachment upload uses the **v1 API** (not v2). The v2 API only supports listing/getting attachments, not creating them.

| Parameter | Value | Source |
|-----------|-------|--------|
| Upload Endpoint | `POST /wiki/rest/api/content/{id}/child/attachment` | [Confluence v1 attachment API](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content---attachments/) |
| Update Endpoint | `POST /wiki/rest/api/content/{id}/child/attachment/{attachmentId}/data` | Same |
| Content-Type | `multipart/form-data` | Same |
| Required Header | `X-Atlassian-Token: nocheck` | Same |
| Form Field Name | `file` | Same |
| Comment Field | `comment` (optional, same count as files) | Same |

**Integration point:** The `client.Client` needs a new method (e.g., `DoMultipart`) that:
1. Accepts a file path instead of `io.Reader` JSON body
2. Sets `Content-Type: multipart/form-data` (NOT `application/json`)
3. Adds `X-Atlassian-Token: nocheck` header
4. Constructs the URL using the v1 API base path, not the v2 path
5. Streams the file via `io.Pipe` to avoid loading entire files into memory

### Watch/Polling Command

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (time.Ticker, context, encoding/json) | (stdlib) | Periodic polling with configurable interval and NDJSON output | **No new dependencies.** `time.NewTicker` + `context.WithCancel` for graceful shutdown on SIGINT. Each poll result is written as a single JSON line (NDJSON) to stdout. The pattern: compare current state hash with previous, emit only changed items. |

**Design notes:**
- Use `time.Ticker` (not `time.Sleep`) for consistent intervals that account for request latency
- NDJSON output (one JSON object per line) for streaming consumption by agents
- `context.WithCancel` + signal handling for clean shutdown
- Content change detection via `version.number` field comparison (pages/blogs) or CQL `lastModified` ordering

### Output Presets

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (encoding/json, os, embed) | (stdlib) | Named combinations of --jq + --fields stored in config | **No new dependencies.** Presets are JSON objects mapping a name to `{"jq": "...", "fields": "..."}` stored in config.json under a `presets` key. Built-in defaults can be embedded via `//go:embed` or simply hardcoded as Go map literals. |

**Implementation:** Presets resolve at flag parsing time in `PersistentPreRunE` -- if `--preset <name>` is set, it overrides `--jq` and `--fields` with the preset's values. User-defined presets in config.json override built-in ones.

### Content Templates

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (text/template, embed, os) | (stdlib) | Parameterized content creation templates | **No new dependencies.** Go's `text/template` handles variable substitution in Confluence storage format (XHTML) templates. Templates stored as `.xml` files in a templates directory, loaded via `//go:embed` for built-ins or from `~/.config/cf/templates/` for user-defined. |

**Design notes:**
- Use `text/template` not `html/template` because Confluence storage format is XHTML that needs literal `<` and `>` chars (html/template would escape them)
- Template variables use Go template syntax: `{{.Title}}`, `{{.SpaceKey}}`
- Built-in templates: blank page, meeting notes, decision record, status report
- User templates override built-ins by name

## Summary: Zero New Dependencies

All v1.1 features are implementable using Go stdlib only. This is deliberate:

| Feature | Stdlib Packages Used | External Dependency |
|---------|---------------------|---------------------|
| OAuth2 client_credentials | net/http, net/url, encoding/json | None |
| OAuth2 3LO browser flow | net/http, crypto/rand, crypto/sha256, encoding/base64 | None |
| Token persistence | os, encoding/json | None |
| Attachment upload | mime/multipart, io, os | None |
| Watch/polling | time, context, encoding/json | None |
| Output presets | encoding/json, os | None |
| Content templates | text/template, embed, os | None |

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| Stdlib net/http for OAuth2 | golang.org/x/oauth2 | x/oauth2 adds 5+ transitive dependencies and its `TokenSource` abstraction is designed for long-running servers, not CLIs that make 1-2 requests per invocation. The jr pattern (hand-rolled `fetchOAuth2Token`) is simpler, already proven, and easier to debug. The only thing x/oauth2 gives us is automatic token refresh -- but for a CLI that runs for seconds, fetching a fresh token per invocation is fine. |
| Stdlib net/http for local callback | pkg/browser (via exec) | For opening the browser during 3LO auth, use `exec.Command("open", url)` on macOS, `xdg-open` on Linux. No library needed for this. |
| Stdlib mime/multipart for uploads | go-resty multipart | go-resty wraps net/http with its own abstractions. For a single upload endpoint, the stdlib multipart.Writer is simpler and avoids adding a dependency. |
| Stdlib text/template for content | Sprig / pongo2 / Handlebars | Content templates need basic variable substitution only (title, space, date). Go's text/template provides `{{.Var}}` syntax, conditionals, and range -- more than enough. Sprig adds 100+ template functions we don't need. |
| time.Ticker for polling | fsnotify / file watchers | We're polling a remote HTTP API, not watching local files. fsnotify is irrelevant. |
| File-based token storage | OS keychain (keyring lib) | Keychain access libraries (go-keyring, zalando/go-keyring) add CGO dependencies on Linux and complicate cross-compilation. File-based storage with 0600 permissions is the standard pattern for CLI tools (cf. gh, gcloud). |

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| golang.org/x/oauth2 | Over-engineered for CLI use. Designed for long-running server token management with automatic refresh. Our CLI invocations are short-lived. Adds transitive deps including google.golang.org/appengine. | Stdlib net/http POST to token endpoint (copy jr pattern) |
| go-keyring / keychain | CGO dependency on Linux (libsecret), breaks cross-compilation. Not needed when config dir has 0600 permissions. | File-based token storage in ~/.config/cf/tokens.json |
| gorilla/websocket | Confluence has no WebSocket API for change notifications. Watch is polling-based. | time.Ticker + HTTP polling |
| cobra-cli (scaffolding tool) | Commands are generated from OpenAPI spec, not scaffolded. Adding manual scaffolding tools would conflict with the gen/ pipeline. | Hand-written or generated Cobra commands |
| html/template | Would HTML-escape the Confluence storage format XHTML, breaking `<ac:structured-macro>` tags. | text/template (no escaping) |

## Config Schema Changes

The `AuthConfig` struct needs new fields (matching jr's existing pattern):

```go
type AuthConfig struct {
    Type         string `json:"type"`                    // "basic", "bearer", "oauth2"
    Username     string `json:"username,omitempty"`       // basic auth
    Token        string `json:"token,omitempty"`          // basic/bearer
    ClientID     string `json:"client_id,omitempty"`      // oauth2
    ClientSecret string `json:"client_secret,omitempty"`  // oauth2
    TokenURL     string `json:"token_url,omitempty"`      // oauth2 (default: https://auth.atlassian.com/oauth/token)
    Scopes       string `json:"scopes,omitempty"`         // oauth2 (space-separated)
    CloudID      string `json:"cloud_id,omitempty"`       // oauth2 (Atlassian site identifier)
}
```

The `Profile` struct needs a presets field:

```go
type Profile struct {
    // ... existing fields ...
    Presets map[string]Preset `json:"presets,omitempty"`
}

type Preset struct {
    JQ     string `json:"jq,omitempty"`
    Fields string `json:"fields,omitempty"`
}
```

## Version Compatibility

| Component | Compatible With | Notes |
|-----------|-----------------|-------|
| OAuth2 client_credentials | Atlassian service accounts | Token URL: `https://auth.atlassian.com/oauth/token`. Requires org admin to create service account + OAuth2 credential. |
| OAuth2 3LO | Atlassian developer console app | Requires app registration at `developer.atlassian.com/console`. Callback URL must be `http://localhost:{port}/callback`. |
| Attachment v1 API | Confluence Cloud v1 REST API | NOT part of v2 API. Must use `/wiki/rest/api/content/{id}/child/attachment`. Auth headers (basic/bearer/oauth2) work the same as v2. |
| mime/multipart | All Go versions 1.1+ | Stable stdlib package, no compatibility concerns. |
| text/template | All Go versions 1.0+ | Stable stdlib package. |
| time.Ticker | All Go versions 1.0+ | Stable stdlib package. |

## Sources

- [Atlassian OAuth 2.0 3LO for Confluence Cloud](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) -- authorization URL, token URL, flow details (HIGH confidence)
- [Atlassian Service Account OAuth2 Credentials](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) -- client_credentials grant confirmed for Confluence Cloud (HIGH confidence)
- [Confluence Scopes for OAuth 2.0](https://developer.atlassian.com/cloud/confluence/scopes-for-oauth-2-3LO-and-forge-apps/) -- classic and granular scopes (HIGH confidence)
- [Confluence v1 Attachment API](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content---attachments/) -- upload endpoint, headers, multipart format (HIGH confidence)
- [Atlassian Community: Client Credentials Support](https://community.developer.atlassian.com/t/does-atlassian-supports-ouath-2-0-client-credentials-grant-flow-for-token-generation/73705) -- historical context on 2LO support (MEDIUM confidence)
- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/client/client.go` lines 102-131 -- proven `fetchOAuth2Token` pattern (HIGH confidence)
- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/config/config.go` -- OAuth2 config struct pattern (HIGH confidence)
- [Go stdlib mime/multipart](https://pkg.go.dev/mime/multipart) -- multipart form-data encoding (HIGH confidence)
- [Go stdlib text/template](https://pkg.go.dev/text/template) -- template engine (HIGH confidence)

---
*Stack research for: Confluence Cloud CLI v1.1 -- OAuth2, attachments, watch, presets, templates*
*Researched: 2026-03-20*
