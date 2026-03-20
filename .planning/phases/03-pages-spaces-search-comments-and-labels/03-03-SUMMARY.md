---
phase: 03-pages-spaces-search-comments-and-labels
plan: 03
subsystem: api
tags: [cobra, confluence-v1-api, confluence-v2-api, cql-search, labels, comments, net/http]

requires:
  - phase: 03-pages-spaces-search-comments-and-labels
    provides: cmd/pages.go and cmd/spaces.go workflow command files (plans 01-02)
  - phase: 02-code-generation-pipeline
    provides: cmd/generated/ with footer_comments, labels generated commands to merge

provides:
  - cmd/search.go — CQL search command with manual v1 pagination loop and cursor guard
  - cmd/comments.go — footer comments list/create/delete with v2 paths
  - cmd/labels.go — labels list (v2) and add/remove (v1 API via direct net/http)
  - searchV1Domain() helper — extracts scheme+host from c.BaseURL for v1 path construction
  - fetchV1() / fetchV1WithBody() helpers — direct net/http with c.ApplyAuth() for v1 calls

affects:
  - 03-pages-spaces-search-comments-and-labels/03-04 (wiring plan that calls rootCmd.AddCommand/mergeCommand)

tech-stack:
  added: []
  patterns:
    - v1 API calls use direct net/http with c.ApplyAuth() since c.Fetch() always prepends c.BaseURL (which includes /wiki/api/v2 prefix)
    - searchV1Domain() extracts domain from c.BaseURL by splitting on first /wiki/ occurrence
    - v2 API calls use c.Do() (auto-pagination) or c.Fetch() (manual response handling)
    - Command parents do NOT call rootCmd.AddCommand in init() — wiring deferred to Plan 04

key-files:
  created:
    - cmd/search.go
    - cmd/comments.go
    - cmd/labels.go
  modified: []

key-decisions:
  - "c.BaseURL is https://domain/wiki/api/v2 (includes v2 prefix); v1 paths need domain extraction via strings.Index(baseURL, /wiki/)"
  - "v1 API calls (search, label add/remove) use direct net/http.NewRequest + c.ApplyAuth() instead of c.Fetch() to avoid URL doubling"
  - "Search pagination: fetchV1 helper makes raw HTTP calls accumulating results[] into flat JSON array, with 4000-char URL length guard"
  - "Labels add uses StringSlice flag (supports --label foo --label bar); labels remove uses single string flag"
  - "Comments create encodes body as {pageId, body:{representation:storage, value:...}} with c.Fetch() POST to /footer-comments"

requirements-completed:
  - SRCH-01
  - SRCH-02
  - SRCH-03
  - CMNT-01
  - CMNT-02
  - CMNT-03
  - LABL-01
  - LABL-02
  - LABL-03

duration: 4min
completed: 2026-03-20
---

# Phase 03 Plan 03: Search, Comments, and Labels Summary

**CQL search command with manual v1 pagination loop (4000-char guard), plus comments/labels workflow commands using domain-extracted v1 paths for mutations**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-20T03:07:55Z
- **Completed:** 2026-03-20T03:11:29Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- searchCmd: CQL search via v1 API with manual pagination accumulating results[], stopping at 4000-char cursor URLs with stderr warning
- commentsCmd: list/create/delete footer comments using v2 paths (c.Do() for list/delete, c.Fetch() for create)
- labelsCmd: list (v2), add (v1 POST), remove (v1 DELETE) with correct domain extraction for v1 paths

## Task Commits

1. **Task 1: Implement cmd/search.go** - `f427b78` (feat)
2. **Task 2: Implement cmd/comments.go and cmd/labels.go** - `e68c9e1` (feat)

## Files Created/Modified

- `cmd/search.go` - CQL search command with manual v1 pagination; fetchV1 helper using direct net/http + c.ApplyAuth()
- `cmd/comments.go` - Footer comments list/create/delete subcommands using v2 API paths
- `cmd/labels.go` - Labels list (v2)/add/remove (v1) with fetchV1WithBody helper; searchV1Domain reused from search.go

## Decisions Made

- `c.BaseURL` confirmed as `"https://domain/wiki/api/v2"` (includes v2 prefix) — evidenced by pages.go comment "BaseURL already includes /wiki/api/v2" and client test patterns using `/wiki/api/v2/pages` as Do() path
- v1 paths cannot use `c.Fetch()` (which prepends c.BaseURL, causing URL doubling). Used direct `net/http.NewRequest` + `c.ApplyAuth()` + `c.HTTPClient.Do()` for all v1 calls
- `searchV1Domain()` splits on first `/wiki/` occurrence: `strings.Index(baseURL, "/wiki/")` returns idx > 0, then `baseURL[:idx]` gives the domain
- All three init() functions do NOT call rootCmd.AddCommand — wiring is deferred to Plan 04 which uses mergeCommand() for comments/labels and rootCmd.AddCommand() for search

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed v1 URL construction in search.go**
- **Found during:** Task 1 (search.go implementation)
- **Issue:** Initial implementation used `c.Fetch()` with path `/wiki/rest/api/search?cql=...` — would produce `https://domain/wiki/api/v2/wiki/rest/api/search` (doubled path) since c.BaseURL includes `/wiki/api/v2`
- **Fix:** Used direct `net/http` request with domain extracted via `searchV1Domain()`, building full absolute URL
- **Files modified:** cmd/search.go
- **Verification:** `go build ./cmd/...` and `go vet ./cmd/...` pass; logic verified against pages.go comment confirming BaseURL format
- **Committed in:** e68c9e1 (Task 2 commit which also updates search.go)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug in v1 URL construction)
**Impact on plan:** Fix essential for correct API routing. No scope creep.

## Issues Encountered

- Initial ambiguity about whether `c.BaseURL` is domain-only or includes `/wiki/api/v2` prefix — resolved by reading `cmd/pages.go` comment ("BaseURL already includes /wiki/api/v2") and cross-referencing client tests

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All five workflow command files ready: cmd/pages.go, cmd/spaces.go, cmd/search.go, cmd/comments.go, cmd/labels.go
- Plan 04 can wire all commands via rootCmd.AddCommand(searchCmd) and mergeCommand(rootCmd, commentsCmd/labelsCmd/etc.)
- All files compile and vet-clean

---
*Phase: 03-pages-spaces-search-comments-and-labels*
*Completed: 2026-03-20*
