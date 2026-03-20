# Phase 6: OAuth2 Authentication - Research

**Researched:** 2026-03-20
**Domain:** OAuth2 authentication for Atlassian Confluence Cloud (client_credentials 2LO + authorization code 3LO)
**Confidence:** HIGH

## Summary

Phase 6 adds two OAuth2 authentication flows to the `cf` CLI: client_credentials (2LO) for machine-to-machine service accounts and authorization code (3LO) for interactive browser-based user authentication. Both flows use Atlassian's centralized auth infrastructure at `auth.atlassian.com` and require switching the API base URL from the direct instance URL to `api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2` when OAuth2 is active. Tokens are short-lived (60 minutes) and must be persisted separately from config.json in per-profile token files with 0600 permissions.

The reference `jr` (Jira CLI) implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` provides a proven pattern for client_credentials (2LO) that can be directly adapted. The `jr` approach places `fetchOAuth2Token()` inside `Client.ApplyAuth()`, which works for 2LO (stateless, no refresh tokens) but is insufficient for 3LO (which requires refresh token management, token persistence, and browser flow orchestration). For `cf`, the architecture research recommends resolving OAuth2 tokens in `PersistentPreRunE` before the Client is constructed, keeping the Client stateless. This is the correct approach for supporting both flows.

A critical finding from this research: Atlassian's official 3LO documentation does NOT mention PKCE (code_challenge/code_verifier) as a requirement. The authorization URL parameters are: audience, client_id, scope, redirect_uri, state, response_type, prompt. However, PKCE is an OAuth2 best practice and implementing it defensively is recommended since Atlassian may enforce it in the future. The implementation should include PKCE (S256) parameters but must handle the case where the server ignores them.

**Primary recommendation:** Build a standalone `internal/oauth2` package with three files: `token.go` (token types + file-based store), `client_credentials.go` (2LO flow), `threelo.go` (3LO browser flow). Wire token resolution into `PersistentPreRunE` in `cmd/root.go`. Update `internal/config` and `cmd/configure.go` to accept OAuth2 parameters.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTH-01 | User can authenticate via OAuth2 client credentials grant (2LO) for machine-to-machine access | Reference `jr` `fetchOAuth2Token` provides proven pattern; Atlassian token URL and grant format verified against official docs |
| AUTH-02 | User can authenticate via OAuth2 authorization code grant (3LO) with PKCE via browser flow | Atlassian 3LO authorization URL, token exchange, and accessible-resources endpoint verified; PKCE not required by Atlassian but included as best practice |
| AUTH-03 | CLI automatically refreshes expired OAuth2 access tokens before API calls | Token expiry is 60 minutes; refresh tokens rotate on use (3LO only); proactive refresh when expiry < 60 seconds avoids 401 race conditions |
| AUTH-04 | OAuth2 tokens are stored securely per profile in separate token files with 0600 permissions | Per-profile token files at `~/.config/cf/tokens/{profile}.json` with atomic writes (temp file + rename) |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/http` | (stdlib) | OAuth2 token endpoint requests, local callback server for 3LO | Zero new dependencies; proven in `jr` reference implementation |
| Go stdlib `net/url` | (stdlib) | Form-encoded token request bodies | Standard for `application/x-www-form-urlencoded` |
| Go stdlib `crypto/rand` + `crypto/sha256` + `encoding/base64` | (stdlib) | PKCE code_verifier and code_challenge generation | S256 method for PKCE |
| Go stdlib `os` + `encoding/json` | (stdlib) | Token file persistence with 0600 permissions | Standard CLI pattern (cf. gh, gcloud) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Stdlib net/http for OAuth2 | `golang.org/x/oauth2` | x/oauth2 adds 5+ transitive deps; designed for long-running servers, not CLI tools that run for seconds; its `TokenSource` abstraction is over-engineered for our use case |
| File-based token storage | OS keychain via `go-keyring` | CGO dependency on Linux (libsecret); breaks cross-compilation; not needed when token files have 0600 permissions |
| Stdlib local HTTP server | External browser-open library | `exec.Command("open", url)` on macOS, `xdg-open` on Linux is sufficient; no library needed |

**Installation:** No new dependencies. All OAuth2 code uses Go stdlib only.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  oauth2/
    token.go               # Token struct, TokenStore (file-based persistence)
    client_credentials.go   # 2LO: client_id+secret -> access_token
    threelo.go              # 3LO: browser auth code flow + local callback server
internal/
  config/
    config.go              # MODIFIED: add oauth2, oauth2-3lo auth types; add OAuth2 fields to AuthConfig
cmd/
  configure.go             # MODIFIED: accept --client-id, --client-secret, --cloud-id flags for oauth2
  root.go                  # MODIFIED: add OAuth2 token resolution in PersistentPreRunE
```

### Pattern 1: Token Resolution in PersistentPreRunE
**What:** OAuth2 token acquisition and refresh happen in `PersistentPreRunE` before the Client is constructed. The Client receives a plain bearer token and never knows about OAuth2.
**When to use:** Every command execution when auth type is `oauth2` or `oauth2-3lo`.
**Why:** Keeps Client stateless; centralizes token lifecycle; avoids refresh logic scattered across request methods.

```go
// In PersistentPreRunE (cmd/root.go) -- after config.Resolve()
if resolved.Auth.Type == "oauth2" || resolved.Auth.Type == "oauth2-3lo" {
    store := oauth2.NewFileStore(tokenDir, resolved.ProfileName)

    var token *oauth2.Token
    var err error

    switch resolved.Auth.Type {
    case "oauth2":
        token, err = oauth2.ClientCredentials(resolved.Auth, store)
    case "oauth2-3lo":
        token, err = oauth2.ThreeLO(resolved.Auth, store)
    }
    if err != nil {
        // write structured error, return ExitAuth
    }

    // Switch to bearer for downstream Client
    resolved.Auth.Type = "bearer"
    resolved.Auth.Token = token.AccessToken

    // Switch base URL to api.atlassian.com proxy
    resolved.BaseURL = fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s/wiki/rest/api/v2",
        resolved.Auth.CloudID)
}
```

### Pattern 2: Client Credentials (2LO) Token Fetch
**What:** Simple POST to token endpoint with client_id, client_secret, and grant_type=client_credentials.
**When to use:** `cf configure --auth-type oauth2` profiles (service accounts).
**Why:** Stateless -- no refresh tokens, no browser flow. Fetch a new token each invocation (tokens last 60 min, CLI runs for seconds).

```go
// Source: reference jr implementation at jira-cli-v2/internal/client/client.go:102-131
func ClientCredentials(auth config.AuthConfig, store *FileStore) (*Token, error) {
    // Check store for unexpired token first
    if cached := store.Load(); cached != nil && !cached.Expired() {
        return cached, nil
    }

    data := url.Values{
        "grant_type":    {"client_credentials"},
        "client_id":     {auth.ClientID},
        "client_secret": {auth.ClientSecret},
    }
    if auth.Scopes != "" {
        data.Set("scope", auth.Scopes)
    }

    resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded",
        strings.NewReader(data.Encode()))
    // ... decode response, store token, return
}
```

### Pattern 3: Authorization Code (3LO) with Local Callback Server
**What:** Start a local HTTP server on a random port, open browser to Atlassian authorization URL, receive callback with auth code, exchange for tokens.
**When to use:** `cf configure --auth-type oauth2-3lo` profiles (interactive user auth).
**Why:** Standard pattern for CLI OAuth2 with user consent; used by `gh`, `gcloud`, `az`.

```go
func ThreeLO(auth config.AuthConfig, store *FileStore) (*Token, error) {
    // 1. Check store for unexpired token
    if cached := store.Load(); cached != nil && !cached.Expired() {
        return cached, nil
    }
    // 2. Check for refresh token
    if cached := store.Load(); cached != nil && cached.RefreshToken != "" {
        return refreshToken(auth, cached.RefreshToken, store)
    }
    // 3. Full browser flow
    listener, _ := net.Listen("tcp", "127.0.0.1:0")
    port := listener.Addr().(*net.TCPAddr).Port
    redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

    // Generate PKCE verifier + challenge (best practice)
    verifier := generateCodeVerifier()   // 43-128 char random string
    challenge := s256Challenge(verifier)  // SHA256 + base64url

    authURL := fmt.Sprintf("%s?audience=%s&client_id=%s&scope=%s&redirect_uri=%s&state=%s&response_type=code&prompt=consent&code_challenge=%s&code_challenge_method=S256",
        authorizationURL, "api.atlassian.com", auth.ClientID,
        url.QueryEscape(auth.Scopes+" offline_access"),
        url.QueryEscape(redirectURI), state, challenge)

    openBrowser(authURL)
    code := waitForCallback(listener) // blocks until callback received
    token := exchangeCode(auth, code, redirectURI, verifier)
    store.Save(token)
    return token, nil
}
```

### Pattern 4: Base URL Switching for OAuth2
**What:** When auth type is oauth2 or oauth2-3lo, the API base URL must change from the direct instance URL to the Atlassian API proxy.
**When to use:** All OAuth2 authenticated requests.
**Critical detail:** The CloudID must be known at configuration time (2LO) or discovered via accessible-resources (3LO).

```
Direct:  https://mysite.atlassian.net/wiki/api/v2
OAuth2:  https://api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2
```

Note the path difference: direct uses `/wiki/api/v2`, OAuth2 proxy uses `/wiki/rest/api/v2`. This must be verified empirically -- both v1 and v2 API paths work through the proxy, but the base path may differ.

### Anti-Patterns to Avoid
- **OAuth2 logic inside Client.ApplyAuth or Client.Do:** The `jr` reference puts `fetchOAuth2Token()` in `ApplyAuth`, which works for 2LO but fails for 3LO (no refresh token handling, no token persistence, no browser flow). For `cf`, resolve tokens BEFORE Client construction.
- **Storing tokens in config.json:** Tokens are ephemeral (60 min), config is persistent. Mixing them causes unnecessary config file churn and risks git commits of secrets.
- **Refreshing tokens inside request execution:** Race conditions when multiple batch operations hit 401 simultaneously; second refresh invalidates first (rotating refresh tokens).
- **Skipping the accessible-resources call for 3LO:** The cloudId is NOT part of the token response; it must be fetched separately via `GET https://api.atlassian.com/oauth/token/accessible-resources`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OAuth2 token exchange | Custom HTTP request builder | Stdlib `net/http.Post` with `url.Values` | Token endpoint is a simple form POST; 30 lines in `jr` reference |
| PKCE code challenge | Custom crypto | `crypto/sha256` + `encoding/base64.RawURLEncoding` | S256 is SHA256 hash + base64url encode; 5 lines |
| Browser opening | Custom platform detection | `exec.Command("open", url)` on darwin, `exec.Command("xdg-open", url)` on linux | OS-standard commands, no library needed |
| Token file locking | Custom file lock | Atomic write via temp file + `os.Rename` | Rename is atomic on POSIX; sufficient for CLI concurrent access |
| Random state parameter | Custom RNG | `crypto/rand.Read` + `encoding/hex` | Cryptographically secure; 3 lines |

## Common Pitfalls

### Pitfall 1: OAuth2 Base URL Uses Different Path Than Direct Instance URL
**What goes wrong:** The OAuth2 API proxy at `api.atlassian.com` uses a different URL structure than direct instance access. Naively appending the v2 API path creates wrong URLs.
**Why it happens:** Direct instance: `https://site.atlassian.net/wiki/api/v2/pages`. OAuth2 proxy: `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2/pages`. The path prefix changes.
**How to avoid:** When auth type is oauth2/oauth2-3lo, override BaseURL completely in PersistentPreRunE. Do not try to transform the existing BaseURL.
**Warning signs:** 404 errors on all API calls when using OAuth2 auth type.

### Pitfall 2: Rotating Refresh Token Race Condition (3LO)
**What goes wrong:** Atlassian rotates refresh tokens on use. If two concurrent `cf` invocations (e.g., in a batch script) both detect an expired access token and both attempt to refresh, the second one fails because the first refresh invalidated the old refresh token.
**Why it happens:** Refresh token rotation is a security feature. Each refresh response returns a NEW refresh token; the old one is invalidated.
**How to avoid:** Use proactive refresh (refresh when access token has < 60 seconds remaining, rather than waiting for 401). Use atomic file operations for the token store so concurrent readers see the latest token. For the CLI's typical usage pattern (sequential commands), this is rarely an issue -- but batch operations could trigger it.
**Warning signs:** Intermittent "invalid_grant" errors on refresh token exchange, especially during batch operations.

### Pitfall 3: Missing CloudID for Client Credentials (2LO)
**What goes wrong:** The client_credentials token response does NOT include a cloudId. Without it, the CLI cannot construct the API proxy URL.
**Why it happens:** 2LO tokens are not site-scoped by default; the cloudId must be discovered via accessible-resources or provided by the user.
**How to avoid:** Require `--cloud-id` during `cf configure --auth-type oauth2`, OR auto-discover via `GET https://api.atlassian.com/oauth/token/accessible-resources` during first token fetch (if exactly one site is accessible, use it; if multiple, error with a list for the user to choose from).
**Warning signs:** Empty or missing cloudId at request time; 404 on API proxy calls.

### Pitfall 4: Accessible-Resources Returns Multiple Sites
**What goes wrong:** The `accessible-resources` endpoint may return multiple Confluence sites for a single token. The CLI must know which site to target.
**Why it happens:** OAuth2 app permissions can span multiple Atlassian sites in an organization.
**How to avoid:** During 3LO setup, if multiple sites are returned, either prompt the user to select one (but this CLI is agent-optimized and avoids prompts), or require `--cloud-id` flag, or store the cloudId after first successful discovery. Best approach for agent-friendly CLI: require cloudId in config, provide a discovery helper command.
**Warning signs:** Wrong site's data returned; confusing 403 errors if the token lacks permissions on the selected site.

### Pitfall 5: Token File Permissions on macOS vs Linux
**What goes wrong:** `os.WriteFile` with mode 0600 works correctly, but if the directory permissions are too open (0755 for the parent), other users can potentially read the token files by guessing the filename.
**Why it happens:** File permissions are necessary but not sufficient; directory permissions also matter.
**How to avoid:** Create the token directory with `os.MkdirAll(dir, 0700)` -- note 0700 not 0755 for the tokens subdirectory specifically. The parent config directory can remain 0755.
**Warning signs:** Security audit tools flagging token files as accessible.

### Pitfall 6: 3LO Callback Server Hangs if User Closes Browser
**What goes wrong:** The local HTTP server blocks waiting for the callback. If the user closes the browser without completing authorization, the CLI hangs indefinitely.
**Why it happens:** No timeout on the callback listener.
**How to avoid:** Set a context timeout (e.g., 5 minutes) on the callback server. After timeout, shut down the listener and return an error.
**Warning signs:** CLI process stuck with no output after browser window is closed.

## Code Examples

### Token Struct and File Store
```go
// internal/oauth2/token.go
package oauth2

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

const (
    AuthorizationURL = "https://auth.atlassian.com/authorize"
    TokenURL         = "https://auth.atlassian.com/oauth/token"
    ResourcesURL     = "https://api.atlassian.com/oauth/token/accessible-resources"
)

// Token represents an OAuth2 token response.
type Token struct {
    AccessToken  string    `json:"access_token"`
    TokenType    string    `json:"token_type"`
    ExpiresIn    int       `json:"expires_in"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    Scope        string    `json:"scope,omitempty"`
    ObtainedAt   time.Time `json:"obtained_at"`
}

// Expired reports whether the access token has expired or will expire
// within the given margin.
func (t *Token) Expired(margin time.Duration) bool {
    return time.Now().After(t.ObtainedAt.Add(time.Duration(t.ExpiresIn)*time.Second - margin))
}

// FileStore manages per-profile token persistence.
type FileStore struct {
    dir     string
    profile string
}

func NewFileStore(dir, profile string) *FileStore {
    return &FileStore{dir: dir, profile: profile}
}

func (s *FileStore) path() string {
    return filepath.Join(s.dir, s.profile+".json")
}

func (s *FileStore) Load() *Token {
    data, err := os.ReadFile(s.path())
    if err != nil {
        return nil
    }
    var t Token
    if json.Unmarshal(data, &t) != nil {
        return nil
    }
    return &t
}

func (s *FileStore) Save(t *Token) error {
    if err := os.MkdirAll(s.dir, 0o700); err != nil {
        return err
    }
    data, _ := json.MarshalIndent(t, "", "  ")
    // Atomic write: temp file + rename
    tmp := s.path() + ".tmp"
    if err := os.WriteFile(tmp, data, 0o600); err != nil {
        return err
    }
    return os.Rename(tmp, s.path())
}
```

### Client Credentials (2LO) Flow
```go
// internal/oauth2/client_credentials.go
// Source: adapted from jr reference at jira-cli-v2/internal/client/client.go:102-131
func ClientCredentials(clientID, clientSecret, scopes string, store *FileStore) (*Token, error) {
    // Check for cached, unexpired token
    if cached := store.Load(); cached != nil && !cached.Expired(60*time.Second) {
        return cached, nil
    }

    data := url.Values{
        "grant_type":    {"client_credentials"},
        "client_id":     {clientID},
        "client_secret": {clientSecret},
    }
    if scopes != "" {
        data.Set("scope", scopes)
    }

    resp, err := http.Post(TokenURL, "application/x-www-form-urlencoded",
        strings.NewReader(data.Encode()))
    if err != nil {
        return nil, fmt.Errorf("token request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("token request failed: HTTP %d: %s",
            resp.StatusCode, strings.TrimSpace(string(body)))
    }

    var token Token
    if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
        return nil, fmt.Errorf("token decode failed: %w", err)
    }
    token.ObtainedAt = time.Now()

    _ = store.Save(&token) // best-effort persist
    return &token, nil
}
```

### PKCE Generation
```go
// internal/oauth2/threelo.go
func generateCodeVerifier() string {
    b := make([]byte, 32)
    _, _ = rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)
}

func s256Challenge(verifier string) string {
    h := sha256.Sum256([]byte(verifier))
    return base64.RawURLEncoding.EncodeToString(h[:])
}
```

### Config Changes
```go
// internal/config/config.go -- AuthConfig additions
type AuthConfig struct {
    Type         string `json:"type"`
    Username     string `json:"username,omitempty"`
    Token        string `json:"token,omitempty"`
    ClientID     string `json:"client_id,omitempty"`     // oauth2, oauth2-3lo
    ClientSecret string `json:"client_secret,omitempty"` // oauth2, oauth2-3lo
    Scopes       string `json:"scopes,omitempty"`        // oauth2 (space-separated)
    CloudID      string `json:"cloud_id,omitempty"`      // oauth2, oauth2-3lo
}

// validAuthTypes update
var validAuthTypes = map[string]bool{
    "basic": true, "bearer": true,
    "oauth2": true, "oauth2-3lo": true,
}
```

### FlagOverrides Extensions
```go
// internal/config/config.go -- FlagOverrides additions
type FlagOverrides struct {
    BaseURL      string
    AuthType     string
    Username     string
    Token        string
    ClientID     string  // new
    ClientSecret string  // new
    CloudID      string  // new
}
```

## Atlassian OAuth2 Endpoints (Verified)

| Parameter | Value | Source | Confidence |
|-----------|-------|--------|------------|
| Authorization URL | `https://auth.atlassian.com/authorize` | [Official 3LO docs](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) | HIGH |
| Token URL | `https://auth.atlassian.com/oauth/token` | Same | HIGH |
| Audience parameter | `api.atlassian.com` | Same | HIGH |
| Accessible Resources | `GET https://api.atlassian.com/oauth/token/accessible-resources` | Same | HIGH |
| API Base URL (OAuth2) | `https://api.atlassian.com/ex/confluence/{cloudId}` | Same | HIGH |
| Access Token Lifetime | 60 minutes | [Service account docs](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) | HIGH |
| Refresh Token Expiry | 90 days inactivity | 3LO docs | HIGH |
| Refresh Token Rotation | Yes, new token on each use | 3LO docs | HIGH |
| PKCE Requirement | NOT mentioned in official docs; best practice to include | [3LO docs](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/), [community thread](https://community.developer.atlassian.com/t/oauth-2-0-with-proof-key-for-code-exchange-pkce/80173) | MEDIUM |
| Client Credentials Grant | Supported via service accounts | [Service account docs](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) | HIGH |

## Required Scopes

| Scope | Purpose | When Needed |
|-------|---------|-------------|
| `read:confluence-content.all` | Read pages, blogs, attachments, custom content | Always |
| `write:confluence-content` | Create/update pages, blogs, comments | Write operations |
| `write:confluence-file` | Upload attachments | Attachment upload |
| `read:confluence-space.summary` | List/get spaces | Space operations |
| `search:confluence` | CQL search | Search operations |
| `offline_access` | Obtain refresh tokens | 3LO only (required for persistent auth) |

**Source:** [Confluence scopes documentation](https://developer.atlassian.com/cloud/confluence/scopes-for-oauth-2-3LO-and-forge-apps/)

## Key Differences: 2LO vs 3LO

| Aspect | 2LO (client_credentials) | 3LO (authorization_code) |
|--------|--------------------------|--------------------------|
| User interaction | None | Browser consent flow |
| Refresh tokens | No (fetch new token each time) | Yes (rotating, 90-day expiry) |
| Token persistence | Optional (cache for 60 min) | Required (refresh token) |
| CloudID discovery | Must be configured or auto-discovered | Auto-discovered via accessible-resources |
| PKCE | N/A | Recommended (not required by Atlassian) |
| `offline_access` scope | N/A | Required for refresh tokens |
| Use case | CI/CD, service accounts, automation | Interactive user sessions |
| Atlassian prerequisite | Org admin creates service account + OAuth2 credential | Developer registers app in developer console |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OAuth2 token fetch per request (jr pattern) | Token caching with proactive refresh | Current best practice | Avoids unnecessary token endpoint calls; critical for 3LO refresh token rotation |
| Storing tokens in config.json | Separate per-profile token files | Standard CLI pattern | Prevents config churn, concurrent access issues, accidental git commits |
| PKCE optional | PKCE recommended for all authorization code flows | OAuth 2.1 (RFC 9700, 2025) | Atlassian does not enforce yet, but include for forward compatibility |

## Open Questions

1. **OAuth2 proxy base URL path structure**
   - What we know: Direct instance uses `/wiki/api/v2`, OAuth2 proxy uses `api.atlassian.com/ex/confluence/{cloudId}/...`
   - What's unclear: Whether the path after `/{cloudId}` is `/wiki/api/v2` or `/wiki/rest/api/v2` -- different sources use different paths
   - Recommendation: During implementation, test both paths empirically. The SUMMARY.md research says `/wiki/rest/api/v2` but this needs validation. Store the full base URL in resolved config so it can be corrected easily.

2. **PKCE acceptance by Atlassian**
   - What we know: Official docs do NOT list code_challenge/code_verifier parameters. Community thread (2023) says PKCE is not publicly available. OAuth changelog has no PKCE entries.
   - What's unclear: Whether Atlassian silently accepts PKCE parameters (ignores them) or rejects them as invalid parameters
   - Recommendation: Implement PKCE but test with the actual Atlassian endpoint. If the authorization endpoint rejects the code_challenge parameter, make PKCE opt-in via a flag or remove it. Do NOT block the implementation on PKCE.

3. **Rate limits for OAuth2 apps**
   - What we know: Points-based rate limiting was introduced March 2, 2026 for OAuth2 apps. API token auth is exempt.
   - What's unclear: Per-endpoint point costs are not published
   - Recommendation: Not a Phase 6 concern (rate limiting affects all commands, not just auth). Document that OAuth2-authenticated usage may have different rate limits than API token usage.

## Sources

### Primary (HIGH confidence)
- [Atlassian OAuth 2.0 3LO for Confluence Cloud](https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/) -- authorization URL, token URL, flow parameters, refresh token rotation, accessible-resources endpoint
- [Atlassian Service Account OAuth2 Credentials](https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/) -- client_credentials grant confirmed, token lifetime 60 min, API proxy URL pattern
- [Confluence Scopes for OAuth 2.0](https://developer.atlassian.com/cloud/confluence/scopes-for-oauth-2-3LO-and-forge-apps/) -- classic and granular scope names
- Reference implementation: `jira-cli-v2/internal/client/client.go` lines 102-131 -- proven `fetchOAuth2Token` pattern for client_credentials
- Reference implementation: `jira-cli-v2/internal/config/config.go` -- AuthConfig struct with OAuth2 fields
- Existing codebase: `internal/config/config.go`, `internal/client/client.go`, `cmd/root.go`, `cmd/configure.go` -- direct code inspection

### Secondary (MEDIUM confidence)
- [Atlassian OAuth 2.0 Changelog](https://developer.atlassian.com/cloud/oauth/changelog/) -- no PKCE-related changes listed through March 2026
- [Atlassian Community: PKCE for OAuth 2.0](https://community.developer.atlassian.com/t/oauth-2-0-with-proof-key-for-code-exchange-pkce/80173) -- PKCE not publicly available as of thread date; internal capability exists but not exposed

### Tertiary (LOW confidence)
- OAuth2 proxy URL path structure (`/wiki/api/v2` vs `/wiki/rest/api/v2`) -- conflicting sources; needs empirical validation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new deps, all stdlib; proven in `jr` reference
- Architecture: HIGH -- token-in-PersistentPreRunE pattern is well-understood; direct codebase inspection confirms integration points
- Pitfalls: HIGH -- Atlassian-specific pitfalls (token rotation, cloudId discovery, base URL switching) verified against official docs
- PKCE requirement: MEDIUM -- official docs silent on it; community says not available; including as best practice but flagged as uncertain

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (stable domain; Atlassian OAuth2 endpoints rarely change)
