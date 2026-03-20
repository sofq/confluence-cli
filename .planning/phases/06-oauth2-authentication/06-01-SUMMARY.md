---
phase: 06-oauth2-authentication
plan: 01
subsystem: auth
tags: [oauth2, client-credentials, 2lo, token-cache, atlassian]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: config package, CLI wiring, error handling patterns
provides:
  - OAuth2 client_credentials (2LO) end-to-end flow
  - AuthConfig with ClientID, ClientSecret, Scopes, CloudID fields
  - internal/oauth2 package with Token, FileStore, ClientCredentials
  - TokenDir() for OS-appropriate token storage
  - PersistentPreRunE OAuth2 token resolution with Atlassian API proxy URL rewrite
affects: [06-02-oauth2-3lo, api-commands, configure]

# Tech tracking
tech-stack:
  added: []
  patterns: [token-file-store-with-atomic-write, client-credentials-grant, overridable-test-endpoints]

key-files:
  created:
    - internal/oauth2/token.go
    - internal/oauth2/token_test.go
    - internal/oauth2/client_credentials.go
    - internal/oauth2/client_credentials_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/configure.go
    - cmd/root.go

key-decisions:
  - "No TokenURL in config -- Atlassian has a single fixed token endpoint, stored as constant"
  - "Token files use atomic write (temp + rename) for crash safety"
  - "OAuth2 auth type resolves to bearer before Client construction"
  - "Base URL rewritten to api.atlassian.com proxy for OAuth2 profiles"

patterns-established:
  - "OAuth2 token caching: FileStore with per-profile JSON files under TokenDir()"
  - "Overridable package-level var (tokenEndpoint) for httptest isolation"
  - "OAuth2 validation: client_id and client_secret required, cloud_id required for 2LO"

requirements-completed: [AUTH-01, AUTH-04]

# Metrics
duration: 6min
completed: 2026-03-20
---

# Phase 06 Plan 01: OAuth2 Client Credentials Summary

**OAuth2 client_credentials (2LO) flow with config schema, token file cache (0600/0700), and CLI wiring to Atlassian API proxy**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-20T08:19:08Z
- **Completed:** 2026-03-20T08:25:15Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Extended AuthConfig with ClientID, ClientSecret, Scopes, CloudID and full env/flag override chain
- Created internal/oauth2 package with Token, FileStore (atomic write, 0600/0700 perms), and ClientCredentials function
- Wired OAuth2 into configure command (--client-id, --client-secret, --cloud-id, --scopes flags) and PersistentPreRunE token resolution
- Cached unexpired tokens reused without network call; expired tokens trigger fresh fetch

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend config package with OAuth2 fields** - `f67a93d` (feat)
2. **Task 2: Create internal/oauth2 package** - `b88837d` (feat)
3. **Task 3: Wire OAuth2 into configure command and PersistentPreRunE** - `1cf0d7d` (feat)

_Note: TDD tasks (1 and 2) had tests written first (RED) then implementation (GREEN) in same commit._

## Files Created/Modified
- `internal/config/config.go` - AuthConfig extended with OAuth2 fields, FlagOverrides, validAuthTypes, TokenDir(), Resolve OAuth2 validation
- `internal/config/config_test.go` - Tests for OAuth2 auth type, resolve merging, env/flag overrides, TokenDir
- `internal/oauth2/token.go` - Token struct, FileStore with atomic write, Expired method, Atlassian endpoint constants
- `internal/oauth2/token_test.go` - Tests for Token.Expired, FileStore Load/Save, permissions, atomic write
- `internal/oauth2/client_credentials.go` - ClientCredentials function for 2LO token fetch with cache
- `internal/oauth2/client_credentials_test.go` - Tests for cached token, fresh fetch, HTTP errors, expired cache refresh
- `cmd/configure.go` - OAuth2 flags, validation, profile save with OAuth2 fields
- `cmd/root.go` - OAuth2 persistent flags, token resolution in PersistentPreRunE, API proxy URL rewrite

## Decisions Made
- No TokenURL config field -- Atlassian has a single fixed endpoint (`https://auth.atlassian.com/oauth/token`), so it is a constant, not per-profile config
- Token files use atomic write (temp file + rename) for crash safety
- OAuth2 resolves to bearer auth type before Client construction -- downstream Client is unaware of OAuth2
- Base URL rewritten to `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2` for OAuth2 profiles

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- OAuth2 2LO foundation complete, ready for Plan 02 (3LO authorization code flow)
- Token caching infrastructure shared by both 2LO and 3LO flows
- `oauth2-3lo` auth type registered but not yet implemented (validation requires client_id + client_secret, does not require cloud_id)

---
*Phase: 06-oauth2-authentication*
*Completed: 2026-03-20*
