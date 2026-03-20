---
phase: 06-oauth2-authentication
verified: 2026-03-20T09:00:00Z
status: passed
score: 12/12 must-haves verified
re_verification: false
---

# Phase 6: OAuth2 Authentication Verification Report

**Phase Goal:** Users and service accounts can authenticate via OAuth2 (both machine-to-machine and interactive browser flow), with tokens managed transparently across sessions.
**Verified:** 2026-03-20
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `cf configure --auth-type oauth2 --client-id X --client-secret Y --cloud-id Z` saves an oauth2 profile | VERIFIED | `cmd/configure.go`: `--client-id`, `--client-secret`, `--cloud-id`, `--scopes` flags registered; profile saved with all OAuth2 fields at line 169-180 |
| 2 | A command using an oauth2 profile fetches a client_credentials token and uses it as Bearer auth | VERIFIED | `cmd/root.go` lines 102-158: `oauth2.ClientCredentials()` called, result set as `resolved.Auth.Token` with type switched to `"bearer"` |
| 3 | OAuth2 tokens cached in token file with 0600 permissions | VERIFIED | `internal/oauth2/token.go` line 84: `os.WriteFile(tmp, data, 0o600)` |
| 4 | Token directory created with 0700 permissions | VERIFIED | `internal/oauth2/token.go` line 73: `os.MkdirAll(s.dir, 0o700)` |
| 5 | Cached unexpired tokens reused without hitting token endpoint | VERIFIED | `internal/oauth2/client_credentials.go` lines 22-24: cache check with 60s margin before any HTTP call |
| 6 | `cf configure --auth-type oauth2-3lo --client-id X --client-secret Y` saves 3LO profile (cloud-id optional) | VERIFIED | `cmd/configure.go` lines 93-140: `isOAuth2` check, cloud-id only required for `"oauth2"` not `"oauth2-3lo"` |
| 7 | A command using oauth2-3lo with no cached token opens browser, receives auth code, exchanges for token | VERIFIED | `internal/oauth2/threelo.go`: `ThreeLO()` at line 234 — local listener, PKCE, `openBrowserFunc`, `waitForCallback`, `exchangeCode` |
| 8 | A command using oauth2-3lo with cached unexpired token reuses it without browser flow | VERIFIED | `internal/oauth2/threelo.go` lines 236-238: first check is cached token with 60s margin; returns immediately |
| 9 | A command using oauth2-3lo with expired access token but valid refresh token refreshes automatically | VERIFIED | `internal/oauth2/threelo.go` lines 241-247: `refreshToken()` called on cache miss when `RefreshToken != ""`; tests `TestThreeLORefreshSuccess` pass |
| 10 | 3LO flow discovers cloudId via accessible-resources and stores it in token file | VERIFIED | `internal/oauth2/threelo.go` lines 289-296: `discoverCloudID()` called when `cloudID == ""`; `token.CloudID = cloudID` then saved |
| 11 | Local callback server times out after 5 minutes | VERIFIED | `internal/oauth2/threelo.go` line 26: `callbackTimeout = 5 * time.Minute`; passed to `waitForCallback` at line 277 |
| 12 | PKCE (S256) parameters included in authorization URL | VERIFIED | `internal/oauth2/threelo.go` lines 37-41, 268: `s256Challenge()` generates SHA256 challenge; `code_challenge_method=S256` in auth URL |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | AuthConfig with ClientID, ClientSecret, Scopes, CloudID; validAuthTypes includes oauth2/oauth2-3lo | VERIFIED | All four fields present in `AuthConfig` struct (lines 23-27); `validAuthTypes` includes `"oauth2"` and `"oauth2-3lo"` (lines 134-137); `TokenDir()` present (lines 302-321) |
| `internal/oauth2/token.go` | Token struct, FileStore with Load/Save, Expired method | VERIFIED | `Token` struct at line 22 (with `CloudID` field added), `FileStore` at line 40, `NewFileStore` at line 46, `Expired` at line 34, `Save` at line 72, `Load` at line 57 |
| `internal/oauth2/client_credentials.go` | ClientCredentials function for 2LO token fetch | VERIFIED | `ClientCredentials` at line 20; `client_credentials` grant type at line 27; `tokenEndpoint` overridable var at line 15 |
| `internal/oauth2/threelo.go` | ThreeLO function, PKCE, browser flow, refresh, accessible-resources | VERIFIED | `ThreeLO` at line 234; `generateCodeVerifier` at line 31; `s256Challenge` at line 38; `refreshToken` at line 63; `discoverCloudID` at line 107; `waitForCallback` at line 156; `exchangeCode` at line 196; `offline_access` appended at line 263; `openBrowserFunc` var at line 25 |
| `cmd/root.go` | OAuth2 token resolution in PersistentPreRunE; flags --client-id, --client-secret, --cloud-id | VERIFIED | `oauth2` import at line 16; combined `oauth2`/`oauth2-3lo` block at lines 102-158; `oauth2.ClientCredentials` and `oauth2.ThreeLO` called; `effectiveCloudID` pattern; base URL rewritten at lines 154-157 |
| `cmd/configure.go` | OAuth2 configure flow with --client-id, --client-secret, --cloud-id flags | VERIFIED | All four OAuth2 flags registered in `init()` (lines 30-33); OAuth2 validation at lines 115-140; profile saved with all OAuth2 fields at lines 169-180 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/root.go` | `internal/oauth2/client_credentials.go` | `oauth2.ClientCredentials` in PersistentPreRunE | WIRED | Line 109: `token, tokenErr = oauth2.ClientCredentials(...)` |
| `cmd/root.go` | `internal/oauth2/token.go` | `oauth2.NewFileStore` for token persistence | WIRED | Line 103: `store := oauth2.NewFileStore(config.TokenDir(), resolved.ProfileName)` |
| `cmd/configure.go` | `internal/config/config.go` | AuthConfig fields populated from CLI flags | WIRED | Lines 169-180: `config.AuthConfig{..., ClientID: clientID, ClientSecret: clientSecret, Scopes: scopes, CloudID: cloudID}` |
| `cmd/root.go` | `internal/oauth2/threelo.go` | `oauth2.ThreeLO` in PersistentPreRunE | WIRED | Line 116: `token, tokenErr = oauth2.ThreeLO(...)` |
| `internal/oauth2/threelo.go` | `internal/oauth2/token.go` | FileStore for token persistence and cached token check | WIRED | Lines 65, 101, 237, 241, 298: `store.Load()` and `store.Save(...)` calls |
| `internal/oauth2/threelo.go` | `https://auth.atlassian.com/authorize` | Browser-opened authorization URL | WIRED | Lines 23, 268: `authorizationEndpoint = AuthorizationURL` used in auth URL construction |
| `internal/oauth2/threelo.go` | `https://api.atlassian.com/oauth/token/accessible-resources` | GET request to discover cloudId | WIRED | Lines 24, 108: `resourcesEndpoint = ResourcesURL` used in `discoverCloudID()` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| AUTH-01 | 06-01-PLAN.md | OAuth2 client credentials grant (2LO) for machine-to-machine | SATISFIED | `oauth2.ClientCredentials()` implemented, wired in PersistentPreRunE, tests pass |
| AUTH-02 | 06-02-PLAN.md | OAuth2 authorization code grant (3LO) with PKCE via browser flow | SATISFIED | `oauth2.ThreeLO()` implemented with PKCE S256, local callback, browser opener, all 11 tests pass |
| AUTH-03 | 06-02-PLAN.md | CLI automatically refreshes expired OAuth2 tokens before API calls | SATISFIED | `refreshToken()` in `threelo.go`; 2LO re-fetches on expiry; `TestThreeLORefreshSuccess` passes |
| AUTH-04 | 06-01-PLAN.md | OAuth2 tokens stored securely with 0600 permissions | SATISFIED | `FileStore.Save()` uses `0o600` for file, `0o700` for dir, atomic write via tmp+rename; `TestFileStoreSaveAndLoad` passes |

No orphaned requirements: all four AUTH-* requirements (AUTH-01 through AUTH-04) are claimed by plans 06-01 and 06-02, and all four are implemented and verified.

### Anti-Patterns Found

None. No TODOs, FIXMEs, placeholders, or stub implementations found in any phase-6 modified files.

### Human Verification Required

#### 1. Browser Launch in Real Environment

**Test:** Configure an oauth2-3lo profile and run any API command (e.g., `cf spaces list`)
**Expected:** Default browser opens the Atlassian authorization URL; after consent, token is cached at `~/.config/cf/tokens/{profile}.json`; command completes with JSON output
**Why human:** Browser interaction and real Atlassian OAuth2 app credentials required; httptest mocks cover the code path but not the actual browser redirect flow

#### 2. Token File Permissions on Disk

**Test:** After any OAuth2 command runs, inspect `~/.config/cf/tokens/{profile}.json` and its parent directory
**Expected:** File permissions are `0600` (`-rw-------`); directory permissions are `0700` (`drwx------`)
**Why human:** Permissions are set by the code and verified in unit tests, but filesystem behavior depends on umask and OS configuration in a real deployment

#### 3. Multi-Site Error Message (3LO)

**Test:** Configure 3LO with an account that has access to multiple Confluence Cloud sites
**Expected:** Command fails with a structured JSON error listing all site names and IDs with instruction to specify `--cloud-id`
**Why human:** Requires real Atlassian account with multiple sites; unit test `TestDiscoverCloudIDMultipleSites` covers code path

---

_Verified: 2026-03-20_
_Verifier: Claude (gsd-verifier)_
