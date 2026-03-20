# Phase 11: Watch - Research

**Researched:** 2026-03-20
**Domain:** Long-running CQL polling, NDJSON streaming, Go signal handling
**Confidence:** HIGH

## Summary

Phase 11 implements a long-running `cf watch` command that polls Confluence via CQL `lastModified` queries and emits NDJSON change events to stdout. This is the only long-running command in the CLI -- all others are request-response. The command uses `signal.NotifyContext` for clean SIGINT/SIGTERM shutdown with a `{"type":"shutdown"}` event.

The core technical challenge is that CQL `lastModified` filtering has date-level granularity only (the time component is ignored despite being accepted in the query syntax). This means the watch command must perform client-side timestamp comparison against individual result `version.when` fields to deduplicate and detect actual changes since the last poll. The v1 search API returns results with content metadata including `content.id`, `content.type`, `content.title`, and can include `lastModified`/`version` data when expanded.

**Primary recommendation:** Build a standalone `cmd/watch.go` using the existing `searchV1Domain()`/`fetchV1()` pattern from `cmd/search.go`, with `signal.NotifyContext` for cancellation and a `time.Ticker` for polling intervals. Track seen content IDs with their `version.when` timestamps in memory to detect changes across polls.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- CQL `lastModified >= 'timestamp'` polling each interval
- Compare against last-seen timestamps to emit only new changes
- Track last poll timestamp, advance after each successful poll
- Content edits only -- page/blogpost modifications, not comments or labels
- NDJSON: one JSON object per line to stdout
- Change event fields: `{"type":"change", "id":"...", "contentType":"page|blogpost", "title":"...", "spaceId":"...", "modifier":"...", "modifiedAt":"..."}`
- Shutdown event: `{"type":"shutdown"}` emitted on SIGINT/SIGTERM before exit
- Metadata only -- no body content in events
- Default interval: 60 seconds (configurable via `--interval` flag)
- Minimum interval: not enforced
- On API error: emit JSON error to stderr, continue polling on next interval
- No exponential backoff
- `signal.NotifyContext` for SIGINT and SIGTERM
- On signal: emit `{"type":"shutdown"}` event, exit cleanly with code 0
- No partial JSON lines -- complete events or nothing

### Claude's Discretion
- Whether to use the existing CQL search command internally or make direct API calls
- Exact CQL query construction for lastModified filtering
- Whether to deduplicate events within the same poll (same content modified multiple times)
- Internal state management (in-memory timestamp vs file persistence)

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| WTCH-01 | User can watch content for changes via `cf watch --cql <query>` with NDJSON event output | CQL lastModified polling pattern, v1 search API reuse, NDJSON encoding, client-side timestamp dedup |
| WTCH-02 | Watch command handles graceful shutdown on SIGINT/SIGTERM | `signal.NotifyContext` pattern, shutdown event emission, clean exit code 0 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/signal` | stdlib | Signal handling with `signal.NotifyContext` | Go stdlib; creates context cancelled on SIGINT/SIGTERM |
| `syscall` | stdlib | SIGINT, SIGTERM constants | Go stdlib for signal constants |
| `time` | stdlib | `time.NewTicker` for polling intervals | Go stdlib; tick-based polling loop |
| `encoding/json` | stdlib | NDJSON line encoding | Go stdlib; `json.NewEncoder` writes one object per line |
| `context` | stdlib | Cancellation propagation from signal to HTTP requests | Go stdlib |

### Supporting (existing project code)
| Component | Location | Purpose | How Watch Uses It |
|-----------|----------|---------|-------------------|
| `searchV1Domain()` | `cmd/search.go` | Extract scheme+host from `c.BaseURL` | Build v1 search URL for CQL polling |
| `fetchV1()` | `cmd/search.go` | Authenticated v1 GET request | Execute each poll's CQL search |
| `client.Client` | `internal/client/client.go` | Auth, stderr, verbose logging | Injected via PersistentPreRunE as usual |
| `cferrors.APIError` | `internal/errors/errors.go` | Structured error JSON | Error events to stderr on poll failure |

**No new Go dependencies required.** All v1.1 features use stdlib only (project decision).

## Architecture Patterns

### Recommended Project Structure
```
cmd/
  watch.go          # watchCmd definition, runWatch function, init registration
```

Single file in `cmd/` -- no `internal/` package needed. The watch command is self-contained with the polling loop, event emission, and signal handling all in one file.

### Pattern 1: Signal-Aware Polling Loop
**What:** Use `signal.NotifyContext` to create a cancellable context, then loop with `time.Ticker`, checking `ctx.Done()` between polls.
**When to use:** Any long-running CLI command that needs clean shutdown.
**Example:**
```go
// Source: Go stdlib signal.NotifyContext docs + verified patterns
func runWatch(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    if err != nil {
        return err
    }

    cqlQuery, _ := cmd.Flags().GetString("cql")
    interval, _ := cmd.Flags().GetDuration("interval")

    // Create signal-aware context from the command's existing context.
    ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    enc := json.NewEncoder(c.Stdout)
    enc.SetEscapeHTML(false)

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    // Track last-seen modification times per content ID.
    seen := make(map[string]string) // contentID -> modifiedAt ISO timestamp

    // Initial poll immediately, then on tick.
    pollAndEmit(ctx, cmd, c, cqlQuery, seen, enc)

    for {
        select {
        case <-ctx.Done():
            // Signal received -- emit shutdown event and exit cleanly.
            _ = enc.Encode(map[string]string{"type": "shutdown"})
            return nil
        case <-ticker.C:
            pollAndEmit(ctx, cmd, c, cqlQuery, seen, enc)
        }
    }
}
```

### Pattern 2: NDJSON Event Emission
**What:** Use `json.NewEncoder` writing to stdout. Each `Encode()` call appends a newline automatically.
**When to use:** Streaming structured events to stdout for agent consumption.
**Example:**
```go
// json.NewEncoder writes one JSON object + newline per Encode() call.
// This naturally produces NDJSON format.
event := map[string]string{
    "type":        "change",
    "id":          contentID,
    "contentType": contentType,  // "page" or "blogpost"
    "title":       title,
    "spaceId":     spaceID,
    "modifier":    modifier,
    "modifiedAt":  modifiedAt,
}
_ = enc.Encode(event)
```

### Pattern 3: Client-Side Timestamp Deduplication
**What:** Track `version.when` per content ID in memory. Only emit events when a content item's `version.when` is newer than last seen.
**When to use:** Required because CQL `lastModified` has date-level granularity only (time component is ignored).
**Example:**
```go
// For each result from CQL search:
modifiedAt := result.Version.When  // ISO 8601 from Confluence
if prev, ok := seen[contentID]; ok && prev >= modifiedAt {
    continue  // Already emitted for this version
}
seen[contentID] = modifiedAt
// Emit change event...
```

### Pattern 4: CQL Query Construction with lastModified
**What:** Combine user's CQL query with `lastModified >= "date"` filter.
**When to use:** Each poll iteration to narrow results to recent changes.
**Example:**
```go
// CQL lastModified only has DATE precision (time is ignored).
// Use yesterday's date on first poll, then today's date on subsequent polls.
// Client-side dedup handles the actual precision.
dateFilter := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
fullCQL := fmt.Sprintf("(%s) AND lastModified >= \"%s\" ORDER BY lastModified DESC", userCQL, dateFilter)
```

### Anti-Patterns to Avoid
- **Writing partial JSON on interrupt:** Never write to stdout outside the `enc.Encode()` call. The json.Encoder guarantees atomic writes.
- **Using time component in CQL:** CQL `lastModified` ignores the `HH:mm` portion. Do not rely on `"2026-03-20 14:30"` for precision filtering.
- **Blocking on HTTP during shutdown:** Always pass the signal-aware `ctx` to `http.NewRequestWithContext` so in-flight requests are cancelled on shutdown.
- **Growing the `seen` map unboundedly:** For very active spaces, the seen map could grow large. Consider periodic pruning of entries older than a threshold (e.g., 24 hours).

## Discretion Recommendations

### Direct API calls vs reusing search command
**Recommendation: Make direct API calls using `fetchV1()` helper.**
Rationale: The search command collects all paginated results, marshals to JSON, and writes to stdout via `c.WriteOutput()`. The watch command needs to process individual results programmatically (parse each, compare timestamps, selectively emit). Reusing `fetchV1()` and `searchV1Domain()` gives authenticated HTTP access without the output pipeline. The search command's `runSearch` function is not designed to return structured data to callers.

### Deduplication within same poll
**Recommendation: Yes, deduplicate.** CQL search results could theoretically contain the same content ID if pagination overlaps or if the API returns duplicates. Using the `seen` map naturally handles this -- only the first occurrence per content ID per `modifiedAt` is emitted.

### Internal state management
**Recommendation: In-memory only.** The watch command is a long-running process. Persisting state to disk adds complexity (file locking, crash recovery) for minimal benefit. If the process restarts, it re-polls with a recent date and may re-emit a few events -- agents should be idempotent. Keep it simple.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal-aware context | Manual signal channel + select | `signal.NotifyContext` | Stdlib handles edge cases (double-signal, race conditions) |
| Periodic timer | `time.Sleep` in a loop | `time.NewTicker` | Ticker is cancellable, doesn't drift, works with select |
| NDJSON encoding | Manual `json.Marshal` + `\n` + `os.Stdout.Write` | `json.NewEncoder(stdout)` | Encoder handles newline appending and buffering atomically |
| HTTP request cancellation | Manual timeout/cancel | `http.NewRequestWithContext(ctx, ...)` | Context propagation cancels in-flight requests on shutdown |

## Common Pitfalls

### Pitfall 1: CQL lastModified Has Date-Only Granularity
**What goes wrong:** Developer writes `lastModified >= "2026-03-20 14:30"` expecting time-level filtering. CQL accepts the syntax without error but returns all content modified on 2026-03-20, ignoring the 14:30 component.
**Why it happens:** Confluence CQL documentation shows datetime format `"yyyy-MM-dd HH:mm"` as supported syntax, but the filtering engine operates at date granularity only. Multiple community reports confirm this.
**How to avoid:** Use date-only in CQL (`lastModified >= "2026-03-20"`). Perform client-side comparison against `version.when` (which has full ISO 8601 precision) to detect actual changes.
**Warning signs:** Watch command re-emits the same content changes every poll cycle despite no new edits.
**Confidence:** HIGH -- verified via multiple Atlassian community reports.

### Pitfall 2: Shutdown Event Race with Polling
**What goes wrong:** A signal arrives while `fetchV1()` is mid-request. The HTTP request is cancelled (context done), `fetchV1` returns an error, and the error handler writes to stderr. Then the shutdown handler also tries to write the shutdown event. Interleaved output.
**Why it happens:** The signal cancels the context, which cancels the HTTP request. The poll error path and shutdown path both execute.
**How to avoid:** After `fetchV1` returns, check `ctx.Err() != nil` before processing errors. If context is cancelled, skip the error handling and let the main select loop handle shutdown.
**Warning signs:** Error JSON on stderr appears alongside the shutdown event during Ctrl+C.

### Pitfall 3: First Poll Window Too Narrow
**What goes wrong:** On first invocation, if the CQL date filter is set to "now", no historical changes are returned. The watcher appears to do nothing until the next edit occurs.
**Why it happens:** The first poll has no "last seen" timestamp. Using the current date/time returns nothing because no changes have occurred since the process started.
**How to avoid:** On the first poll, use a lookback window (e.g., current date minus 1 day) so the watcher immediately emits any recent changes. Subsequent polls use the advancing date.
**Warning signs:** Watch command starts silently, no events emitted until a new edit happens.

### Pitfall 4: Atlassian Rate Limits on Frequent Polling
**What goes wrong:** Short intervals (e.g., `--interval 5`) cause HTTP 429 rate limit errors. The watch command logs errors to stderr but keeps retrying, potentially getting the API token temporarily blocked.
**Why it happens:** Atlassian rate limit point costs per endpoint are not published (noted in STATE.md blockers). The v1 search endpoint may consume significant rate limit budget per call.
**How to avoid:** Default to 60s interval. On HTTP 429, respect the `Retry-After` header value from the error response. Log a clear warning to stderr with the retry-after duration.
**Warning signs:** Repeated `rate_limited` errors on stderr, especially with intervals under 30 seconds.

### Pitfall 5: v1 Search Response Structure Parsing
**What goes wrong:** Developer assumes v1 search results have a flat structure. Actually, v1 search wraps content in a `content` field within each result, and metadata fields like `lastModified` are at the result level, not inside `content`.
**Why it happens:** v1 search results are `SearchResult` objects (with `content`, `title`, `excerpt`, `lastModified` fields), not raw `Content` objects.
**How to avoid:** Parse the v1 search response carefully. Each result has: `result.content.id`, `result.content.type`, `result.title`, `result.lastModified`, and space info may need `result.content.space` or `result.resultGlobalContainer`.
**Warning signs:** Nil pointer panics or empty fields when extracting content metadata from search results.

## Code Examples

### Watch Command Registration
```go
// cmd/watch.go init()
func init() {
    watchCmd.Flags().String("cql", "", "CQL query to watch (required)")
    watchCmd.Flags().Duration("interval", 60*time.Second, "polling interval (e.g. 30s, 2m)")
}

// cmd/root.go init() -- add this line:
rootCmd.AddCommand(watchCmd)  // Phase 11: content change watcher
```

### V1 Search Result Parsing
```go
// v1 search response envelope (same as search.go but with result-level fields)
type searchResponse struct {
    Results []searchResult `json:"results"`
    Links   struct {
        Next string `json:"next"`
    } `json:"_links"`
}

type searchResult struct {
    Content struct {
        ID    string `json:"id"`
        Type  string `json:"type"`   // "page" or "blogpost"
        Title string `json:"title"`
        Space struct {
            ID  int    `json:"id"`
            Key string `json:"key"`
        } `json:"space"`
    } `json:"content"`
    LastModified string `json:"lastModified"` // ISO 8601
    // version.by for modifier info requires expand=content.version
}
```

### Graceful Shutdown with Event Emission
```go
// In the main select loop:
case <-ctx.Done():
    // Emit shutdown event as the final NDJSON line.
    _ = enc.Encode(map[string]string{"type": "shutdown"})
    // Return nil -- exit code 0 (clean shutdown).
    return nil
```

### Error Handling During Poll (Continue on Error)
```go
// In pollAndEmit function:
body, code := fetchV1(cmd, c, fullURL)
if code != cferrors.ExitOK {
    // Check if this is a shutdown cancellation, not a real error.
    if ctx.Err() != nil {
        return // Let the main loop handle shutdown
    }
    // Real error -- already written to stderr by fetchV1.
    // Continue polling on next tick (don't exit).
    return
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `signal.Notify` + manual channel | `signal.NotifyContext` | Go 1.16 (2021) | Simpler signal handling, context-native |
| `time.Sleep` loop | `time.NewTicker` + `select` | Always preferred | Cancellable, no drift accumulation |
| Manual JSON line formatting | `json.NewEncoder.Encode()` | Always preferred | Atomic writes, automatic newline |

## Open Questions

1. **V1 search result expand parameters**
   - What we know: `fetchV1` returns raw JSON. The `version.by` field (modifier info) likely requires `expand=content.version` in the query parameters.
   - What's unclear: Exact expand parameters needed to get `version.by.displayName` for the `modifier` field.
   - Recommendation: During implementation, test with `expand=content.version,content.space` to get modifier and space info. If expand is not available on search, extract modifier from `lastModified` context or emit the field as empty string.

2. **Seen map memory growth**
   - What we know: For typical usage (watching a single space), the map stays small. For broad CQL queries across large instances, it could grow.
   - What's unclear: Practical upper bound on unique content IDs in a typical watch session.
   - Recommendation: Implement periodic pruning -- remove entries from `seen` map that are older than 2x the polling interval. This bounds memory while maintaining dedup accuracy.

## Sources

### Primary (HIGH confidence)
- `cmd/search.go` -- CQL search implementation, `searchV1Domain()`, `fetchV1()` patterns
- `cmd/root.go` -- Command registration pattern, `PersistentPreRunE` client injection
- `internal/client/client.go` -- `ApplyAuth`, `Client` struct, context patterns
- Go stdlib `os/signal` -- `signal.NotifyContext` documentation
- [Atlassian CQL documentation](https://developer.atlassian.com/cloud/confluence/advanced-searching-using-cql/) -- field operators, date formats

### Secondary (MEDIUM confidence)
- [Atlassian community: CQL datetime precision](https://community.atlassian.com/forums/Confluence-questions/Confluence-query-created-and-lastModified-datetime-filter/qaq-p/917014) -- confirmed date-only granularity for lastModified
- [Atlassian v1 search API](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-search/) -- response structure
- [Go graceful shutdown patterns](https://victoriametrics.com/blog/go-graceful-shutdown/) -- verified signal.NotifyContext patterns

### Tertiary (LOW confidence)
- V1 search expand parameters for `version.by` -- needs empirical validation during implementation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, patterns verified from existing codebase
- Architecture: HIGH -- follows established `cmd/*.go` patterns, reuses `fetchV1`/`searchV1Domain`
- Pitfalls: HIGH -- CQL date granularity verified via multiple community sources; signal handling patterns well-documented
- V1 response parsing: MEDIUM -- exact field structure needs implementation-time validation

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (stable domain, Go stdlib does not change)
