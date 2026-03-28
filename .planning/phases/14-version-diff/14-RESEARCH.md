# Phase 14: Version Diff - Research

**Researched:** 2026-03-28
**Domain:** Confluence page version comparison (CLI diff command)
**Confidence:** HIGH

## Summary

Phase 14 implements a `cf diff` command that compares page versions using the Confluence Cloud REST API v2. The command must output structured JSON with version metadata and line-level change statistics. It supports three modes: default (two most recent versions), time-filtered (`--since`), and explicit version comparison (`--from`/`--to`).

The implementation mirrors jr's `cmd/diff.go` + `internal/changelog/` pattern, adapted for Confluence's version model. The v2 API provides two key endpoints: `GET /pages/{id}/versions` for listing version metadata, and `GET /pages/{id}?version=N&body-format=storage` for retrieving historical version body content. A new `internal/diff/` package handles version comparison logic, `parseSince()` time parsing, and line-level diff statistics. The diff algorithm uses Go stdlib only (no new dependencies), consistent with the project's zero-dependency policy.

**Primary recommendation:** Create `internal/diff/` package with `Compare()` function (mirrors jr's `changelog.Parse()`) and `cmd/diff.go` command that wires flags to API calls to the diff package. Use `c.Fetch()` for all API calls since the command assembles its own output rather than passing raw API responses through `WriteOutput()`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Output is always a `diffs` array for consistent shape -- even when comparing just two most recent versions (single-element array). Agents can always expect the same JSON structure
- **D-02:** Each diff entry contains `from` (version metadata: number, authorId, createdAt, message), `to` (same fields), and `stats` (linesAdded, linesRemoved)
- **D-03:** No body content in diff output -- agents use `pages get-by-id --version N` if they need the full body. Keeps diff output small and focused
- **D-04:** Line stats computed by splitting storage format on `\n` and running a simple line-level diff. No XHTML-aware parsing -- treat as plain text. Fast, zero dependencies
- **D-05:** `--since` supports both human durations (2h, 1d, 1w) via `duration.Parse()` and ISO date strings (2026-01-01, RFC3339) -- mirror jr's `parseSince()` pattern from `internal/changelog/`
- **D-06:** When `--since` captures multiple versions (e.g. v3, v4, v5 all within the time range), output contains pairwise diffs between all adjacent versions (v3->v4, v4->v5). Agents see the full change timeline
- **D-07:** `--from` and `--to` flags specify version numbers for explicit comparison. Output is a single-element `diffs` array with the diff between those two versions
- **D-08:** Use v2 `GET /pages/{id}` with `?version=N&body-format=storage` query params to retrieve historical version bodies for diff computation
- **D-09:** If v2 API doesn't return body content for historical versions (needs live API validation during research), fall back to metadata-only diff with `stats` field omitted and an informational note. Do NOT silently return zero stats
- **D-10:** No v1 API fallback for body retrieval -- maintain v2-primary approach consistent with project constraints
- **D-11:** Single version (no prior to diff): Return `diffs` array with single entry, `from: null`, `to` has version metadata, stats show all lines as added
- **D-12:** Empty `--since` range (no versions in time window): Return `{"pageId": "...", "since": "2h", "diffs": []}` -- empty array, not an error
- **D-13:** `--from` equals `--to`: Return diff entry with zero stats -- not an error
- **D-14:** Page not found or permission denied: Standard APIError JSON to stderr, same pattern as all other commands

### Claude's Discretion
- Internal package structure (likely `internal/diff/` mirroring jr's `internal/changelog/`)
- Exact diff algorithm implementation (Myers, patience, or simple LCS)
- Version metadata field names matching actual v2 API response shape
- Test case selection and organization
- Whether `--from`/`--to` are mutually exclusive with `--since`, or can combine

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DIFF-01 | User can compare two page versions and see structured JSON diff output | v2 API `GET /pages/{id}/versions` for version listing, `GET /pages/{id}?version=N&body-format=storage` for body retrieval; `internal/diff/` package with `Compare()` function; line-level diff via stdlib string splitting |
| DIFF-02 | User can filter version diffs by time range using `--since` with human-friendly durations | `parseSince()` pattern from jr's `internal/changelog/changelog.go`; reuses cf's `internal/duration.Parse()` (returns `time.Duration`); also parses ISO date strings |
| DIFF-03 | User can specify `--from` and `--to` version numbers for explicit comparison | Direct version body retrieval via `GET /pages/{id}?version=N&body-format=storage`; single-element `diffs` array output |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `strings` | 1.25.8 | Line splitting for diff | Zero dependencies per project policy |
| Go stdlib `time` | 1.25.8 | Time parsing for `--since` ISO dates | Matches jr's `parseSince()` approach |
| Go stdlib `encoding/json` | 1.25.8 | JSON marshaling/unmarshaling | Standard across all cf commands |
| `internal/duration` | existing | Human duration parsing (2h, 1d, 1w) | Already built in Phase 12 |
| `internal/jsonutil` | existing | `MarshalNoEscape()` for output | Already used by all commands |
| `internal/errors` | existing | `APIError` + `AlreadyWrittenError` | Standard error pattern |
| `internal/client` | existing | `FromContext()`, `Fetch()` | Standard API call pattern |
| `github.com/spf13/cobra` | 1.10.2 | Command/flag framework | Already in go.mod |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go stdlib `fmt` | 1.25.8 | URL path construction | `fmt.Sprintf` for API paths |
| Go stdlib `net/url` | 1.25.8 | Path escaping | `url.PathEscape` for page IDs |
| Go stdlib `strconv` | 1.25.8 | Version number parsing | `strconv.Atoi` for `--from`/`--to` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom line diff | `github.com/sergi/go-diff` | External dependency violates zero-dep policy; simple line counting is sufficient for stats-only output |
| Myers algorithm | Simple line-set counting | Myers gives optimal edit distance but is overkill when we only need linesAdded/linesRemoved counts, not a patch |

**Installation:**
```bash
# No new dependencies needed -- all stdlib + existing internal packages
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
  diff/
    diff.go           # Compare(), parseSince(), line diff logic, types
    diff_test.go       # Unit tests for Compare, parseSince, line stats
cmd/
  diff.go             # diffCmd cobra command, flag wiring, API calls
  diff_test.go        # Integration tests with httptest server
```

### Pattern 1: Internal Package with Parse/Compare Function
**What:** Mirror jr's `internal/changelog/changelog.go` pattern. The internal package defines types (`Result`, `DiffEntry`, `VersionMeta`, `Stats`, `Options`) and a `Compare()` function that accepts raw API responses and options, returning a structured result.
**When to use:** Always -- this is the established pattern for cf commands that process API responses.
**Example:**
```go
// Source: jr's internal/changelog/changelog.go pattern adapted for cf
package diff

import (
    "encoding/json"
    "time"
)

// VersionMeta holds metadata for a single page version.
type VersionMeta struct {
    Number    int    `json:"number"`
    AuthorID  string `json:"authorId"`
    CreatedAt string `json:"createdAt"`
    Message   string `json:"message"`
}

// Stats holds line-level change statistics.
type Stats struct {
    LinesAdded   int `json:"linesAdded"`
    LinesRemoved int `json:"linesRemoved"`
}

// DiffEntry represents a single version-to-version comparison.
type DiffEntry struct {
    From  *VersionMeta `json:"from"`  // nil for first version
    To    *VersionMeta `json:"to"`
    Stats *Stats       `json:"stats,omitempty"` // omitted if body unavailable
    Note  string       `json:"note,omitempty"`  // informational, e.g. "body not available"
}

// Result is the top-level output of the diff command.
type Result struct {
    PageID string      `json:"pageId"`
    Since  string      `json:"since,omitempty"` // present only when --since used
    Diffs  []DiffEntry `json:"diffs"`
}

// Options controls diff behavior.
type Options struct {
    Since string // duration or ISO date
    From  int    // explicit version number (0 = not set)
    To    int    // explicit version number (0 = not set)
    Now   time.Time // reference time for duration; zero = time.Now()
}
```

### Pattern 2: Command Structure (flag parsing -> API calls -> internal package -> output)
**What:** The `cmd/diff.go` command follows the established cf pattern: parse flags, get client from context, make API calls via `c.Fetch()`, pass raw response data to `internal/diff.Compare()`, marshal result, output via `c.WriteOutput()`.
**When to use:** Always -- matches `cmd/export.go`, jr's `cmd/diff.go`.
**Example:**
```go
// Source: cmd/export.go + jr's cmd/diff.go patterns
func runDiff(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    if err != nil {
        return err
    }

    id, _ := cmd.Flags().GetString("id")
    if strings.TrimSpace(id) == "" {
        apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
        apiErr.WriteJSON(c.Stderr)
        return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
    }

    since, _ := cmd.Flags().GetString("since")
    from, _ := cmd.Flags().GetInt("from")
    to, _ := cmd.Flags().GetInt("to")

    // Step 1: Fetch version list
    // Step 2: Determine which versions to compare (based on flags)
    // Step 3: Fetch body content for each version pair
    // Step 4: Call diff.Compare() with version data
    // Step 5: Marshal result and WriteOutput

    opts := diff.Options{Since: since, From: from, To: to}
    result, err := runDiffLogic(cmd.Context(), c, id, opts)
    // ... error handling, marshal, WriteOutput
}
```

### Pattern 3: Version List Fetching with Pagination
**What:** The `GET /pages/{id}/versions` endpoint returns paginated results. Use `c.Fetch()` in a loop following cursor-based pagination to get all versions, then filter by time range or select explicit versions.
**When to use:** When `--since` is used (need to enumerate versions in time range) or default mode (need the two most recent versions).
**Example:**
```go
// Fetch all versions for a page (paginated)
func fetchVersions(ctx context.Context, c *client.Client, pageID string) ([]VersionInfo, error) {
    path := fmt.Sprintf("/pages/%s/versions?limit=50&sort=-modified-date", url.PathEscape(pageID))
    // ... pagination loop similar to export.go fetchAllChildren()
}
```

### Pattern 4: Body Retrieval for Historical Versions
**What:** Use `GET /pages/{id}?version=N&body-format=storage` to retrieve the body content of a specific historical version. The `version` parameter is an integer query param supported by the v2 API (confirmed from the generated code: `pages_get_by_id` accepts `version` flag).
**When to use:** For each version pair that needs line-level diff stats.
**Example:**
```go
// Fetch body for a specific version
func fetchVersionBody(ctx context.Context, c *client.Client, pageID string, versionNum int) (string, error) {
    path := fmt.Sprintf("/pages/%s?version=%d&body-format=storage",
        url.PathEscape(pageID), versionNum)
    body, code := c.Fetch(ctx, "GET", path, nil)
    if code != cferrors.ExitOK {
        return "", fmt.Errorf("fetch version %d failed", versionNum)
    }
    // Extract body.storage.value from response
    var page struct {
        Body struct {
            Storage struct {
                Value string `json:"value"`
            } `json:"storage"`
        } `json:"body"`
    }
    json.Unmarshal(body, &page)
    return page.Body.Storage.Value, nil
}
```

### Anti-Patterns to Avoid
- **Using `c.Do()` instead of `c.Fetch()`:** The diff command assembles its own output from multiple API calls. `c.Do()` writes directly to stdout, which would break the multi-call assembly. Use `c.Fetch()` which returns raw bytes.
- **Passing API responses directly to `WriteOutput()`:** The diff command needs to combine data from multiple API calls (version list + body for each version). Build the result struct first, then marshal and WriteOutput once.
- **Implementing Myers diff for stats-only:** The decision (D-04) explicitly says "simple line-level diff, treat as plain text." A full edit-script algorithm is unnecessary when we only need linesAdded/linesRemoved counts.
- **Adding external diff dependencies:** Project has a strict zero-new-dependency policy. All diff logic must use stdlib only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Duration parsing | Custom time parser | `internal/duration.Parse()` | Already built in Phase 12 with all edge cases handled |
| ISO date parsing | Custom date parser | `time.Parse()` with standard layouts | Go stdlib handles RFC3339, ISO 8601 date-only |
| JSON output | Custom serializer | `internal/jsonutil.MarshalNoEscape()` | Prevents HTML entity corruption in storage format |
| Error handling | Custom error output | `cferrors.APIError` + `AlreadyWrittenError` | Standard pattern across all commands |
| HTTP client calls | Direct `http.Get` | `client.Fetch()` | Handles auth, verbose logging, audit, dry-run |
| Cursor pagination | Manual URL construction | Follow `fetchAllChildren()` pattern from export.go | Handles the /wiki/api/v2 prefix stripping |

**Key insight:** Almost all infrastructure is already built. This phase is primarily wiring -- connecting existing API endpoints and internal packages through a new command and a thin diff-logic layer.

## Common Pitfalls

### Pitfall 1: Historical Version Body Retrieval May Return Empty
**What goes wrong:** The v2 API `GET /pages/{id}?version=N&body-format=storage` may return an empty body for historical versions on some Confluence instances or configurations.
**Why it happens:** Confluence Cloud's v2 API has had inconsistencies with body content retrieval, as documented in community discussions. The `version` parameter is supported (confirmed in the generated OpenAPI spec), but body content for historical versions is not guaranteed.
**How to avoid:** Per decision D-09, check if the body field is empty/missing after fetching. If so, return a metadata-only diff with `stats` omitted and a `note` field explaining body was not available. Never silently return zero stats.
**Warning signs:** `body.storage.value` is empty string or `body` field is null in the API response.

### Pitfall 2: cf duration.Parse Returns time.Duration, Not int
**What goes wrong:** jr's `duration.Parse()` returns `int` (seconds), so jr's `parseSince()` does `time.Duration(secs) * time.Second`. cf's `duration.Parse()` returns `time.Duration` directly. Copying jr's code verbatim will cause a type mismatch.
**Why it happens:** cf's Phase 12 modernized the duration package to return Go-native `time.Duration` instead of raw seconds.
**How to avoid:** In `parseSince()`, use `dur, err := duration.Parse(s)` then `return now.Add(-dur), nil` -- no multiplication by `time.Second` needed.
**Warning signs:** Compile error on `time.Duration(secs) * time.Second` where secs is already a `time.Duration`.

### Pitfall 3: Version List Sorting and Pagination
**What goes wrong:** Versions may not come back in order, or may require multiple pages of results. Assuming the first page contains the two most recent versions is unsafe.
**Why it happens:** The `GET /pages/{id}/versions` endpoint supports a `sort` parameter. Default sort order may vary. Large pages could have many versions.
**How to avoid:** Always request `sort=-modified-date` (descending by modification date) and handle cursor pagination. For default mode (two most recent), fetch with `limit=2`. For `--since` mode, fetch enough to cover the time range.
**Warning signs:** Diff showing wrong version pairs, or missing versions in `--since` range.

### Pitfall 4: Nil Diffs Array vs Empty Array
**What goes wrong:** Go's `json.Marshal` encodes a nil slice as `null`, not `[]`. Per decision D-12, empty results must be `"diffs": []`, not `"diffs": null`.
**Why it happens:** Go's zero value for a slice is nil. If no diffs are computed, the slice is nil.
**How to avoid:** Initialize `diffs` as `[]DiffEntry{}` (empty non-nil slice), same pattern as jr's `changes = []Change{}` in changelog.go.
**Warning signs:** JSON output contains `"diffs": null` instead of `"diffs": []`.

### Pitfall 5: Version Number as Int vs String
**What goes wrong:** The generated code uses `string` for `--version-number` flag, but version numbers are integers in the API response. Mixing types causes confusion.
**Why it happens:** The generated command code uses strings for all flag values, but the actual API version numbers are integers. The `--from` and `--to` flags should be `int` type in the hand-written command.
**How to avoid:** Define `--from` and `--to` as `cmd.Flags().Int()` in the hand-written diff command. When building the API URL for body retrieval, use `fmt.Sprintf("%d", versionNum)`.
**Warning signs:** Passing "0" as version number to the API when the flag wasn't set.

### Pitfall 6: parseSince ISO Date Parsing Order
**What goes wrong:** If ISO date parsing is tried after duration parsing, a string like "2026-01-01" could be partially matched or rejected confusingly.
**Why it happens:** The order of parsing attempts matters. Duration parser would reject "2026-01-01" cleanly, but the error message would be confusing.
**How to avoid:** Follow jr's exact order: try ISO date formats first (RFC3339, datetime, date-only), then fall back to duration parsing. This matches user expectations and produces clear error messages.
**Warning signs:** Confusing error messages when ISO dates are passed to `--since`.

## Code Examples

### Line-Level Diff Statistics (stdlib only)
```go
// Source: Decision D-04 -- simple line-level diff, zero dependencies
// Computes linesAdded and linesRemoved by comparing line sets.
func lineStats(oldBody, newBody string) Stats {
    oldLines := strings.Split(oldBody, "\n")
    newLines := strings.Split(newBody, "\n")

    oldSet := make(map[string]int)
    for _, line := range oldLines {
        oldSet[line]++
    }

    added, removed := 0, 0
    newSet := make(map[string]int)
    for _, line := range newLines {
        newSet[line]++
    }

    // Lines in old but not in new = removed
    for line, count := range oldSet {
        newCount := newSet[line]
        if newCount < count {
            removed += count - newCount
        }
    }

    // Lines in new but not in old = added
    for line, count := range newSet {
        oldCount := oldSet[line]
        if oldCount < count {
            added += count - oldCount
        }
    }

    return Stats{LinesAdded: added, LinesRemoved: removed}
}
```

### parseSince Adapted for cf's duration.Parse
```go
// Source: jr's internal/changelog/changelog.go parseSince(), adapted for cf
// cf's duration.Parse returns time.Duration (not int seconds like jr's)
func parseSince(s string, now time.Time) (time.Time, error) {
    // Try ISO date formats first (same order as jr).
    for _, layout := range []string{
        time.RFC3339,
        "2006-01-02T15:04:05",
        "2006-01-02",
    } {
        if t, err := time.Parse(layout, s); err == nil {
            return t, nil
        }
    }

    // Try duration format via cf's duration parser.
    dur, err := duration.Parse(s)
    if err != nil {
        return time.Time{}, fmt.Errorf(
            "invalid --since value %q: expected duration (e.g. 2h, 1d) or date (e.g. 2026-01-01)", s)
    }
    if now.IsZero() {
        now = time.Now()
    }
    return now.Add(-dur), nil  // Note: no * time.Second -- dur is already time.Duration
}
```

### Version Metadata Extraction from API Response
```go
// Source: Confluence v2 API response shape (from generated code + official docs)
// The versions endpoint returns objects with these fields.
type apiVersion struct {
    Number    int    `json:"number"`
    AuthorID  string `json:"authorId"`
    CreatedAt string `json:"createdAt"`
    Message   string `json:"message"`
}

type apiVersionList struct {
    Results []apiVersion `json:"results"`
    Links   struct {
        Next string `json:"next"`
    } `json:"_links"`
}
```

### DryRun Support
```go
// Source: jr's cmd/diff.go dry-run pattern
if c.DryRun {
    dryOut := map[string]any{
        "method": "GET",
        "url":    c.BaseURL + versionsPath,
        "note":   fmt.Sprintf("would fetch version diff for page %s", id),
    }
    out, _ := jsonutil.MarshalNoEscape(dryOut)
    if ec := c.WriteOutput(out); ec != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: ec}
    }
    return nil
}
```

### Command Registration
```go
// Source: cmd/root.go pattern -- add to init()
rootCmd.AddCommand(diffCmd)  // Phase 14: version diff
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| v1 API `/rest/api/content/{id}/version` | v2 API `/pages/{id}/versions` | 2023 (v2 GA) | Cleaner response, cursor pagination, body-format param |
| v1 `expand=body.storage` | v2 `body-format=storage` query param | 2023 (v2 GA) | Simpler param, no nested expand syntax |
| External diff libs (go-diff, difflib) | Stdlib-only line counting | Project decision | Zero dependency policy; stats-only (not patch output) |

**Deprecated/outdated:**
- v1 content versions endpoint (`/rest/api/content/{id}/version`): Still works but project exclusively uses v2 per D-10
- `expand=` parameter syntax: v2 replaced with explicit query params like `body-format`, `include-version`, etc.

## Open Questions

1. **Historical version body availability via v2 API**
   - What we know: The generated OpenAPI spec confirms `GET /pages/{id}` accepts a `version` integer parameter described as "retrieve a previously published version." The `body-format` parameter is also accepted. Both `get-versions` and `get-by-id` list `body-format` in their query parameters.
   - What's unclear: Whether combining `?version=N&body-format=storage` actually returns the body content for historical versions in all Confluence Cloud instances. Community reports have noted empty body issues with the v2 API. This needs live API validation.
   - Recommendation: Per decision D-09, implement the happy path (body retrieval works) with a graceful fallback (metadata-only diff with `note` field) when body is empty. This handles both cases without blocking implementation.

2. **`--from`/`--to` mutual exclusivity with `--since`**
   - What we know: These represent two different modes of version selection. Claude's discretion per CONTEXT.md.
   - What's unclear: Whether combining them makes semantic sense.
   - Recommendation: Make `--from`/`--to` mutually exclusive with `--since`. If both are provided, return a validation error. Rationale: `--from`/`--to` specifies exact versions while `--since` specifies a time window -- combining them adds complexity with no clear use case.

3. **Version list sort parameter values**
   - What we know: The generated code shows a `sort` flag on `get-versions`. The API likely accepts `-modified-date` or similar.
   - What's unclear: Exact valid sort values for the versions endpoint.
   - Recommendation: Use `-modified-date` for descending order (most recent first). If the API rejects this, fall back to client-side sorting by `createdAt`.

## Sources

### Primary (HIGH confidence)
- Generated `cmd/generated/pages.go` lines 854-921, 1303-1424 -- Confluence v2 API endpoint shapes, flag definitions, query parameters (from OpenAPI spec)
- jr `cmd/diff.go` -- Reference implementation for diff command pattern
- jr `internal/changelog/changelog.go` -- Reference implementation for parseSince(), Options struct, Result struct patterns
- cf `internal/duration/duration.go` -- Existing duration parser (returns `time.Duration`, not int)
- cf `internal/client/client.go` -- `Fetch()`, `WriteOutput()`, `FromContext()` patterns
- cf `cmd/export.go` -- Recent hand-written command pattern reference
- cf `cmd/root.go` -- Command registration, flag wiring, `skipClientCommands` list

### Secondary (MEDIUM confidence)
- [Atlassian v2 API Page endpoint docs](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-page/#api-pages-id-get) -- `version` integer parameter confirmed: "Allows you to retrieve a previously published version"
- [Atlassian v2 API Version endpoint docs](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-version/) -- Version list and detail endpoints

### Tertiary (LOW confidence)
- [Community: Confluence Cloud API v2 get page by ID - Empty Body](https://community.developer.atlassian.com/t/confluence-cloud-api-v2-get-page-by-id-empty-body/80857) -- Reports of empty body with v2 API (validates need for D-09 fallback)
- [Community: Confluence REST API v2 history](https://community.developer.atlassian.com/t/confluence-rest-api-v2-update-get-page-rest-to-return-history/78041) -- Limitations of v2 history data

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use, no new dependencies
- Architecture: HIGH -- directly mirrors jr's proven pattern with minor adaptations
- Pitfalls: HIGH -- identified from real code inspection (cf vs jr duration types, nil slice, version types)
- API body retrieval: MEDIUM -- v2 API `version` param confirmed in OpenAPI spec but live behavior for body content needs runtime validation; fallback per D-09 mitigates risk

**Research date:** 2026-03-28
**Valid until:** 2026-04-28 (stable -- core patterns won't change)
