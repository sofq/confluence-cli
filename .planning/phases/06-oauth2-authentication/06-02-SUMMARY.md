---
phase: 06-oauth2-authentication
plan: 02
subsystem: auth
tags: [oauth2, 3lo, pkce, browser-flow, token-refresh, atlassian]

# Dependency graph
requires:
  - phase: 06-oauth2-authentication (plan 01)
    provides: Token struct, FileStore, ClientCredentials, constants, PersistentPreRunE oauth2 case
provides:
  - ThreeLO function for OAuth2 authorization code (3LO) browser flow
  - PKCE (S256) code_challenge generation
  - Automatic token refresh with rotating refresh tokens
  - CloudID discovery via accessible-resources endpoint
  - Combined oauth2/oauth2-3lo handling in PersistentPreRunE
affects: [phase-08-attachments, phase-11-polling]

# Tech tracking
tech-stack:
  added: []
  patterns: [browser-based-oauth2-callback, pkce-s256, token-refresh-rotation, cloud-id-discovery]

key-files:
  created: [internal/oauth2/threelo.go, internal/oauth2/threelo_test.go]
  modified: [internal/oauth2/token.go, cmd/root.go]

key-decisions:
  - "PKCE included defensively despite Atlassian not requiring it -- future-proofs for OAuth 2.1"
  - "callbackTimeout as package var for testability -- tests use 200ms instead of 5min"
  - "CloudID stored in Token struct so 3LO discovery persists across invocations"

patterns-established:
  - "Combined oauth2/oauth2-3lo block in PersistentPreRunE with effectiveCloudID fallback"
  - "Package-level var overrides for HTTP endpoints and browser opener in tests"

requirements-completed: [AUTH-02, AUTH-03]

# Metrics
duration: 4min
completed: 2026-03-20
---

# Phase 06 Plan 02: OAuth2 3LO Browser Flow Summary

**OAuth2 authorization code (3LO) with PKCE S256, automatic refresh token rotation, and cloudId auto-discovery via accessible-resources**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-20T08:29:09Z
- **Completed:** 2026-03-20T08:33:12Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Full 3LO browser flow: local callback server on random port, PKCE S256, authorization code exchange
- Automatic token refresh with rotating refresh tokens (preserves CloudID across refreshes)
- CloudID discovery via accessible-resources when not configured (single site auto-selects, multiple sites error with list)
- Combined PersistentPreRunE block handles both 2LO and 3LO with effectiveCloudID fallback from config to token

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/oauth2/threelo.go (TDD)**
   - `b2abdc8` (test: failing tests for 3LO, PKCE, refresh, callback, discovery)
   - `500722c` (feat: implement 3LO browser flow with PKCE and refresh)
2. **Task 2: Wire 3LO into PersistentPreRunE and configure** - `b1c922b` (feat)

## Files Created/Modified
- `internal/oauth2/threelo.go` - ThreeLO function, PKCE, browser flow, refresh, cloudId discovery
- `internal/oauth2/threelo_test.go` - 11 unit tests covering all 3LO paths
- `internal/oauth2/token.go` - Added CloudID field to Token struct
- `cmd/root.go` - Combined oauth2/oauth2-3lo block with effectiveCloudID

## Decisions Made
- PKCE included defensively -- Atlassian does not enforce it but OAuth 2.1 recommends it
- callbackTimeout exposed as package var for fast test execution (200ms vs 5min)
- CloudID persisted in Token JSON so 3LO discovery only happens once per profile

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- OAuth2 authentication complete (both 2LO and 3LO)
- Both flows tested and wired into the command pipeline
- Ready for subsequent phases that require authenticated API access

---
*Phase: 06-oauth2-authentication*
*Completed: 2026-03-20*
