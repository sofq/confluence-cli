# Architecture Research

**Domain:** Confluence CLI v1.1 -- new capability integration into existing Go CLI
**Researched:** 2026-03-20
**Confidence:** HIGH

## System Overview (Current + v1.1 Extensions)

```
                            cf CLI (Cobra root)
                                   |
          +------------------------+-------------------------+
          |                        |                         |
    cmd/ (commands)          cmd/generated/          new cmd/ commands
    - pages, spaces          - blogposts             - blogposts (hand-written)
    - comments, labels       - attachments           - attachments (hand-written)
    - search, raw            - custom-content        - custom-content (hand-written)
    - configure, batch       - 23 other resources    - watch (new)
    - avatar, schema                                 - presets (new)
          |                        |                 - templates (new)
          +------------------------+---------+-------+
                                             |
                               internal/ (shared packages)
          +----------+----------+----------+---------+--------+
          |          |          |          |         |        |
       config    client     errors      jq      cache    policy
       (auth)    (HTTP)     (exit)    (filter) (GET)   (enforce)
          |          |
     +----+----+     +--- audit
     |         |
  MODIFY    NEW PACKAGE
  add oauth2  internal/oauth2
  auth types  (token mgmt)
                     |
               NEW PACKAGE
               internal/preset
               (output presets)
                     |
               NEW PACKAGE
               internal/template
               (content templates)
```

### Component Responsibilities (Existing)

| Component | Responsibility | Integration Point for v1.1 |
|-----------|----------------|---------------------------|
| `internal/config` | Profile/auth resolution, config file I/O | Add `oauth2` and `oauth2-3lo` auth types, store client_id/client_secret/refresh_token |
| `internal/client` | HTTP execution, auth headers, pagination, output | Add `ApplyAuth` case for OAuth2 bearer, add `DoMultipart` for attachment upload |
| `internal/errors` | Structured JSON errors, exit codes | No changes needed -- existing codes cover all new scenarios |
| `internal/jq` | In-process JQ filtering | No changes needed |
| `internal/cache` | GET response caching | No changes needed |
| `internal/policy` | Operation allow/deny enforcement | No changes needed -- new operations auto-covered by pattern matching |
| `internal/audit` | NDJSON audit logging | No changes needed -- new operations auto-logged via `Client.Do` |
| `cmd/root.go` | PersistentPreRunE builds Client from config | Add OAuth2 token refresh before creating Client |
| `cmd/generated/` | Auto-generated from OpenAPI spec | blogposts/attachments/custom-content already generated |

### New Components

| Component | Responsibility | Why New Package |
|-----------|----------------|-----------------|
| `internal/oauth2` | Token acquisition (client credentials + 3LO), refresh, storage | OAuth2 token lifecycle is complex enough to isolate; reusable across commands |
| `internal/preset` | Named JQ+fields combinations, load from config dir | Clean separation from JQ package; config-file-based |
| `internal/template` | Content template loading and rendering | Standalone concern, file-based |

## Recommended Project Structure (New/Modified Files)

```
internal/
  oauth2/
    oauth2.go            # TokenSource interface, token storage (file-based)
    client_credentials.go # 2LO: client_id+secret -> access_token
    threelo.go           # 3LO: authorization code flow with local HTTP server
    token_store.go       # Encrypted token persistence (~/.config/cf/tokens/)
  preset/
    preset.go            # Load/apply named output presets from config
  template/
    template.go          # Load/render content templates from config dir

cmd/
  blogposts.go           # Hand-written: create/update with body handling (merges with generated)
  attachments.go         # Hand-written: upload subcommand (multipart, v1 API fallback)
  custom_content.go      # Hand-written: create/update convenience wrappers
  watch.go               # Polling loop, NDJSON event output
  presets.go             # Preset management subcommand (list, show)
  templates.go           # Template management subcommand (list, apply)

internal/
  config/
    config.go            # MODIFIED: add oauth2/oauth2-3lo to validAuthTypes, new Profile fields
  client/
    client.go            # MODIFIED: add oauth2 case in ApplyAuth
    multipart.go         # NEW: DoMultipart method for file uploads
```

### Structure Rationale

- **internal/oauth2/**: Token lifecycle (acquire, refresh, persist) is a distinct concern from HTTP request execution. Isolating it allows `cmd/root.go` to handle token refresh in PersistentPreRunE before the Client is constructed, keeping the Client stateless regarding token management.
- **internal/preset/**: Presets are config-file data (not JQ logic). They map a name to a `{jq, fields}` pair. Keeping them separate from `internal/jq` avoids coupling a pure-function package to filesystem I/O.
- **internal/template/**: Templates are files on disk with variable substitution. Completely independent of other internal packages.
- **cmd/attachments.go**: Must handle multipart/form-data upload via the v1 API (`/rest/api/content/{id}/child/attachment`), which is fundamentally different from the JSON-only v2 API pattern used everywhere else. This justifies a hand-written command rather than relying on the generated one.

## Architectural Patterns

### Pattern 1: Hand-Written Command Merging (Existing Pattern)

**What:** Hand-written commands replace generated parent commands while preserving generated subcommands via `mergeCommand()`.
**When to use:** For blogposts, attachments, and custom-content -- add convenience subcommands (create-with-body, upload) while keeping generated CRUD operations.
**Trade-offs:** Clean UX for common operations; must avoid name collisions with generated subcommands.

**How it applies to v1.1:**
```go
// cmd/root.go init()
mergeCommand(rootCmd, blogpostsCmd)       // adds create/update convenience, keeps generated list/get/delete
mergeCommand(rootCmd, attachmentsCmd)     // adds upload subcommand, keeps generated list/get/delete
mergeCommand(rootCmd, customContentCmd)   // adds create convenience, keeps generated CRUD
rootCmd.AddCommand(watchCmd)             // no generated equivalent
rootCmd.AddCommand(presetsCmd)           // no generated equivalent
rootCmd.AddCommand(templatesCmd)         // no generated equivalent
```

### Pattern 2: OAuth2 Token Source in PersistentPreRunE

**What:** Token acquisition and refresh happen before the Client is constructed, so the Client just gets a bearer token and never deals with OAuth2 directly.
**When to use:** For all commands when auth type is `oauth2` or `oauth2-3lo`.
**Trade-offs:** Client stays simple (just sets `Authorization: Bearer <token>`); refresh logic is centralized; token storage is a separate concern.

**Example flow:**
```go
// In PersistentPreRunE (cmd/root.go)
if resolved.Auth.Type == "oauth2" || resolved.Auth.Type == "oauth2-3lo" {
    ts := oauth2.NewTokenSource(resolved)  // loads from token store
    token, err := ts.Token()               // refreshes if expired
    if err != nil {
        // For 3LO: trigger browser flow
        // For client_credentials: fetch new token
    }
    resolved.Auth.Type = "bearer"          // downstream Client just uses bearer
    resolved.Auth.Token = token.AccessToken
}
```

### Pattern 3: Multipart Upload via Separate Client Method

**What:** A `DoMultipart` method on Client handles file uploads with `multipart/form-data` content type and the required `X-Atlassian-Token: nocheck` header.
**When to use:** Attachment upload only (the only multipart endpoint).
**Trade-offs:** Keeps the main `Do` method clean; multipart is fundamentally different from JSON request/response.

**Key constraint:** Attachment upload uses the Confluence v1 API (`/rest/api/content/{id}/child/attachment`), not v2. The `DoMultipart` method must construct the URL differently -- the base URL needs to be the site URL (e.g., `https://site.atlassian.net`) rather than the v2 API base (`https://site.atlassian.net/wiki/api/v2`).

```go
// internal/client/multipart.go
func (c *Client) DoMultipart(ctx context.Context, path string, filePath string) int {
    // Derive site base URL by stripping /wiki/api/v2 from c.BaseURL
    siteURL := strings.TrimSuffix(c.BaseURL, "/wiki/api/v2")
    fullURL := siteURL + path  // path = /wiki/rest/api/content/{id}/child/attachment

    // Build multipart body with "file" field
    // Set X-Atlassian-Token: nocheck header
    // Apply auth, execute, write output
}
```

### Pattern 4: Watch Command as Polling Loop with NDJSON Output

**What:** A long-running command that polls an endpoint at intervals and emits changed items as NDJSON (one JSON object per line) to stdout.
**When to use:** `cf watch` for monitoring content changes.
**Trade-offs:** Simple to implement (no websockets needed); agents can consume NDJSON streams; exit on signal or `--count` limit.

**Design:**
```go
// cmd/watch.go
// cf watch pages --space-id ABC --interval 30s --jq '.title'
// Polls GET /pages with sort=-modified-date, compares to last-seen state
// Emits new/changed items as individual JSON lines (NDJSON)
// Respects --jq filtering per event
// Exits on SIGINT/SIGTERM or --count N
```

## Data Flow

### OAuth2 Client Credentials Flow (New)

```
cf configure --auth-type oauth2 --client-id X --client-secret Y --profile myprofile
    |
    v
config.json: { "auth": { "type": "oauth2", "client_id": "X", "client_secret": "Y" } }

cf pages list --profile myprofile
    |
    v
PersistentPreRunE:
    config.Resolve() -> auth.type == "oauth2"
    |
    v
    oauth2.ClientCredentials.Token()
    POST https://auth.atlassian.com/oauth/token
    { grant_type: "client_credentials", client_id: X, client_secret: Y }
    |
    v
    access_token (valid 60 min) -> stored in token_store
    |
    v
    Client.Auth = { type: "bearer", token: access_token }
    |
    v
    normal request flow (unchanged)
```

### OAuth2 3LO Flow (New)

```
cf configure --auth-type oauth2-3lo --client-id X --client-secret Y --profile myprofile
    |
    v
cf pages list --profile myprofile  (first time, no token cached)
    |
    v
PersistentPreRunE:
    config.Resolve() -> auth.type == "oauth2-3lo"
    oauth2.ThreeLO.Token() -> no cached token
    |
    v
    Start local HTTP server on random port (e.g., :18492)
    Open browser: https://auth.atlassian.com/authorize?
        client_id=X&scope=read:confluence-content.all+write:confluence-content+offline_access
        &redirect_uri=http://localhost:18492/callback&response_type=code&prompt=consent
    |
    v
    User authorizes in browser -> callback with ?code=AUTH_CODE
    |
    v
    POST https://auth.atlassian.com/oauth/token
    { grant_type: "authorization_code", code: AUTH_CODE, client_id: X, client_secret: Y,
      redirect_uri: http://localhost:18492/callback }
    |
    v
    { access_token, refresh_token, expires_in } -> stored in token_store
    |
    v
    GET https://api.atlassian.com/oauth/token/accessible-resources -> cloudId
    BaseURL = https://api.atlassian.com/ex/confluence/{cloudId}
    |
    v
    Client.Auth = { type: "bearer", token: access_token }
    subsequent calls use refresh_token when access_token expires
```

### Attachment Upload Flow (New)

```
cf attachments upload --page-id 12345 --file ./report.pdf
    |
    v
PersistentPreRunE: normal client setup (unchanged)
    |
    v
attachments upload RunE:
    c.DoMultipart(ctx, "/wiki/rest/api/content/12345/child/attachment", "./report.pdf")
    |
    v
    DoMultipart:
    - Derives site base URL from c.BaseURL (strip /wiki/api/v2)
    - Creates multipart/form-data body with "file" field
    - Sets X-Atlassian-Token: nocheck header
    - Applies auth (same as other requests)
    - Executes request
    - Writes JSON response to stdout (same WriteOutput path)
```

### Watch Command Flow (New)

```
cf watch pages --space-id ABC --interval 30s
    |
    v
    Initial fetch: GET /pages?space-id=ABC&sort=-modified-date&limit=25
    Record last-seen modified timestamp
    |
    v
    [loop every 30s]
        Fetch: GET /pages?space-id=ABC&sort=-modified-date&limit=25
        Compare to last-seen state
        For each new/changed item:
            Apply --jq filter (if set)
            Write JSON line to stdout (NDJSON)
        Update last-seen state
    [until SIGINT or --count reached]
```

### Output Preset Flow (New)

```
~/.config/cf/presets.json:
{
  "page-titles": { "jq": ".results[] | {id, title}", "fields": "" },
  "space-summary": { "jq": ".results[] | {key, name, type}", "fields": "" }
}

cf pages list --preset page-titles
    |
    v
PersistentPreRunE:
    --preset flag -> preset.Load("page-titles")
    -> sets c.JQFilter and c.Fields from preset definition
    |
    v
    normal request + output flow (unchanged -- jq/fields already wired)
```

### Content Template Flow (New)

```
~/.config/cf/templates/meeting-notes.json:
{
  "title": "Meeting Notes: {{.date}}",
  "space_id": "{{.space}}",
  "body": "<h1>Attendees</h1><p>{{.attendees}}</p><h1>Notes</h1><p></p>"
}

cf pages create --template meeting-notes --var date=2026-03-20 --var space=ABC123 --var attendees="Alice, Bob"
    |
    v
    template.Load("meeting-notes") -> render with vars -> JSON body
    |
    v
    POST /pages with rendered body (normal client.Do path)
```

## Integration Points

### Modifications to Existing Code

| File | Change | Scope |
|------|--------|-------|
| `internal/config/config.go` | Add `oauth2`, `oauth2-3lo` to `validAuthTypes`; add `ClientID`, `ClientSecret` fields to `AuthConfig`; add `CF_CLIENT_ID`, `CF_CLIENT_SECRET` env vars to `Resolve()` | Small, additive |
| `internal/client/client.go` | Add `oauth2` case in `ApplyAuth` (maps to bearer); no other changes | Minimal |
| `cmd/root.go` | Add `--preset` flag; add OAuth2 token resolution in PersistentPreRunE before Client construction; add `--client-id`, `--client-secret` flag overrides | Moderate |
| `cmd/configure.go` | Accept `--client-id`, `--client-secret` flags; handle `--auth-type oauth2` and `oauth2-3lo` | Moderate |

### New Internal Packages

| Package | Communicates With | Notes |
|---------|-------------------|-------|
| `internal/oauth2` | `internal/config` (reads credentials), filesystem (token store) | Does NOT import `internal/client` -- tokens flow upward to root.go |
| `internal/preset` | Filesystem only (reads preset config files) | Pure data loading, no HTTP |
| `internal/template` | Filesystem only (reads template files) | Pure data loading + Go template rendering |

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| `auth.atlassian.com` | OAuth2 token endpoint (POST) | Used by `internal/oauth2` only; NOT through `internal/client` (different base URL) |
| `api.atlassian.com` | Accessible resources endpoint (GET) | Used once during 3LO setup to discover cloudId |
| Confluence v1 API (`/wiki/rest/api/content/...`) | Multipart upload via `DoMultipart` | Only for attachment upload; everything else stays v2 |

## Anti-Patterns

### Anti-Pattern 1: OAuth2 Logic in Client

**What people do:** Put token refresh logic inside `ApplyAuth` or `Do`.
**Why it's wrong:** Client becomes stateful and complex; refresh failures mid-request are hard to handle; testing requires mocking OAuth2 servers.
**Do this instead:** Resolve tokens in PersistentPreRunE, pass a plain bearer token to Client. Client never knows about OAuth2.

### Anti-Pattern 2: Separate HTTP Client for v1 API

**What people do:** Create a second Client instance with a different BaseURL for v1 attachment upload.
**Why it's wrong:** Duplicates auth, verbose logging, error handling, audit logging. Two code paths to maintain.
**Do this instead:** Add `DoMultipart` to the existing Client that derives the v1 URL from the v2 BaseURL. All cross-cutting concerns (auth, logging, errors) remain centralized.

### Anti-Pattern 3: Watch Command Using Goroutines for Each Resource

**What people do:** Spawn goroutines per watched resource, multiplex output.
**Why it's wrong:** Concurrent writes to stdout corrupt NDJSON; complexity with no benefit for a CLI tool.
**Do this instead:** Single goroutine, sequential poll loop, one resource type at a time. Simple, correct, sufficient.

### Anti-Pattern 4: Templates with Full Templating Engine

**What people do:** Import a full templating engine (Jinja-like, Handlebars, etc.) for content templates.
**Why it's wrong:** Over-engineering for what amounts to string substitution in JSON. Go's `text/template` is in the stdlib and sufficient.
**Do this instead:** Use `text/template` from the Go standard library. Templates are JSON files with `{{.varName}}` placeholders.

### Anti-Pattern 5: Storing OAuth2 Tokens in the Config File

**What people do:** Put access_token and refresh_token directly in config.json alongside credentials.
**Why it's wrong:** Tokens are ephemeral (60-min lifetime), config.json is for persistent settings. Mixing them causes unnecessary config file churn and race conditions if multiple cf invocations run concurrently.
**Do this instead:** Store tokens in a separate file per profile (`~/.config/cf/tokens/{profile}.json`) with file locking for concurrent access.

## Build Order (Dependency-Driven)

The recommended implementation order based on dependency analysis:

```
Phase A: OAuth2 (foundation -- unblocks service account usage)
  1. internal/oauth2/client_credentials.go + token_store.go
  2. internal/config changes (new auth types, new fields)
  3. cmd/root.go PersistentPreRunE changes
  4. cmd/configure.go changes (accept OAuth2 flags)
  5. internal/oauth2/threelo.go (browser flow, depends on token_store)

Phase B: Content Types (independent of Phase A)
  6. cmd/blogposts.go (hand-written, merges with generated)
  7. cmd/custom_content.go (hand-written, merges with generated)
  8. internal/client/multipart.go (DoMultipart method)
  9. cmd/attachments.go (upload subcommand, depends on DoMultipart)

Phase C: Agent Features (independent of A and B)
  10. internal/preset/ + --preset flag in root.go
  11. internal/template/ + cmd/templates.go
  12. cmd/watch.go (polling loop)
```

Phases B and C can run in parallel with each other and with Phase A (after step 3 is complete for auth plumbing).

## Sources

- [Atlassian OAuth 2.0 3LO documentation](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) -- HIGH confidence (official docs)
- [Atlassian OAuth 2.0 client credentials for service accounts](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) -- HIGH confidence (official docs)
- [Confluence scopes for OAuth 2.0](https://developer.atlassian.com/cloud/confluence/scopes-for-oauth-2-3LO-and-forge-apps/) -- HIGH confidence
- [Confluence REST API v1 attachments](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content---attachments/) -- HIGH confidence (official docs, confirms v1-only for upload)
- [Confluence REST API v2 attachment endpoints](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-attachment/) -- HIGH confidence (confirms read-only in v2)
- Existing codebase: `internal/client/client.go`, `internal/config/config.go`, `cmd/root.go` -- direct code inspection

---
*Architecture research for: Confluence CLI v1.1 capability integration*
*Researched: 2026-03-20*
