# Phase 11: Watch - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Long-running content change polling command that emits NDJSON events to stdout. Users run `cf watch --cql <query> --interval 60` and receive one JSON line per detected content change. SIGINT/SIGTERM triggers clean shutdown with a `{"type":"shutdown"}` event. This is the only long-running command in the CLI — all others are request-response.

</domain>

<decisions>
## Implementation Decisions

### Change detection
- CQL `lastModified >= 'timestamp'` polling each interval
- Compare against last-seen timestamps to emit only new changes
- Track last poll timestamp, advance after each successful poll
- Content edits only — page/blogpost modifications, not comments or labels

### Event format
- NDJSON: one JSON object per line to stdout
- Change event fields: `{"type":"change", "id":"...", "contentType":"page|blogpost", "title":"...", "spaceId":"...", "modifier":"...", "modifiedAt":"..."}`
- Shutdown event: `{"type":"shutdown"}` emitted on SIGINT/SIGTERM before exit
- Metadata only — no body content in events. Agent fetches full content separately if needed.

### Polling behavior
- Default interval: 60 seconds (configurable via `--interval` flag)
- Minimum interval: not enforced (user's responsibility to respect API quotas)
- On API error: emit JSON error to stderr, continue polling on next interval
- No exponential backoff — transient errors don't kill the watcher

### Shutdown
- `signal.NotifyContext` for SIGINT and SIGTERM
- On signal: emit `{"type":"shutdown"}` event, exit cleanly with code 0
- No partial JSON lines — complete events or nothing

### Claude's Discretion
- Whether to use the existing CQL search command internally or make direct API calls
- Exact CQL query construction for lastModified filtering
- Whether to deduplicate events within the same poll (same content modified multiple times)
- Internal state management (in-memory timestamp vs file persistence)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### CQL search (existing v1 pattern)
- `cmd/search.go` — CQL search implementation with v1 API, searchV1Domain, fetchV1 helper

### Pitfalls
- `.planning/research/PITFALLS.md` — Watch command pitfalls: signal handling, NDJSON streaming, new execution model

### Client
- `internal/client/client.go` — ApplyAuth for authenticated requests

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/search.go`: CQL search via v1 API — watch can reuse `searchV1Domain()` and `fetchV1()` pattern
- `client.ApplyAuth()`: Auth headers for direct HTTP requests

### Established Patterns
- NDJSON: No existing NDJSON streaming in the codebase — this is a new pattern
- Signal handling: No existing signal handling — `signal.NotifyContext` is new
- All existing commands are request-response via `c.Do()` or `c.Fetch()` — watch needs its own loop

### Integration Points
- `cmd/root.go init()`: Register watchCmd via `rootCmd.AddCommand(watchCmd)` (not mergeCommand — no generated watch command)
- Auth: PersistentPreRunE handles auth before watch starts polling
- JQ/preset: Can apply to individual events (optional enhancement)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard polling watcher with NDJSON output.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-watch*
*Context gathered: 2026-03-20*
