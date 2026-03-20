# Phase 3: Pages, Spaces, Search, Comments, and Labels - Research

**Researched:** 2026-03-20
**Domain:** Confluence v2 API workflow commands; Go/Cobra; v1 API fallback for labels/search
**Confidence:** HIGH

## Summary

Phase 3 hand-writes workflow commands that override the generated CRUD commands for five resource groups: pages, spaces, search, comments, and labels. The generated code in `cmd/generated/` already handles basic REST pass-through; the workflow layer adds Confluence-specific business logic — primarily version auto-increment for page updates, `?body-format=storage` injection for GET, space key-to-ID resolution, CQL search, and v1 API fallback for label mutations.

A critical API finding: **the Confluence v2 spec has no label add/remove endpoints and no search endpoint.** Both operations require the v1 API at `/wiki/rest/api/`. Label add/remove uses `POST/DELETE /wiki/rest/api/content/{id}/label` (note: v1 uses "content" IDs, which for pages equal page IDs in practice). Search uses `GET /wiki/rest/api/search?cql=...` with cursor-based `_links.next` pagination in the response envelope. This is explicitly documented in Atlassian's community (CONFCLOUD-76866 tracks the v2 gap for label mutations). The generated `labels.go` only has GET operations; there are no generated add/remove commands to override — the workflow command must register new subcommands entirely.

The five workflow files can all follow the `jr` `cmd/workflow.go` reference pattern: declare a parent `*cobra.Command`, declare per-operation commands, register flags in `init()`, add subcommands to the parent, and call `mergeCommand(rootCmd, parentCmd)` in `cmd/root.go`. For the search command there is no existing generated parent to merge into — a new `search` top-level command must be added via `rootCmd.AddCommand()` directly (no generated `search.go` exists). All five files go in the `cmd/` package.

**Primary recommendation:** Five files (`cmd/pages.go`, `cmd/spaces.go`, `cmd/search.go`, `cmd/comments.go`, `cmd/labels.go`) using `c.Fetch()` for all operations requiring pre/post-processing, `c.Do()` only for pure pass-through delegates. Labels and search must use the v1 API path (`/wiki/rest/api/...`), all other commands use v2 (`/wiki/api/v2/...` is handled transparently via `c.BaseURL + path`).

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- `cf pages update` must automatically fetch current version, increment, and include in PUT body
- Handle 409 Conflict by retrying with latest version (single retry)
- `cf pages get` must always include `?body-format=storage` to avoid empty body responses
- `cf pages delete` sends HTTP DELETE (moves to trash) — this is the expected Confluence behavior
- No purge command in v1 (admin-only, dangerous)
- `cf spaces list --key <KEY>` resolves key to numeric ID via `GET /wiki/api/v2/spaces?keys=<KEY>`
- Other commands accepting space references should accept either key or numeric ID
- CQL pagination may produce very long cursor strings (up to 11KB)
- Use POST-based search if cursor exceeds URL length limits, or truncate gracefully
- The client's existing `doCursorPagination` handles the merge; search just needs to call `c.Do()`
- Simple CRUD wrappers for comments and labels — no complex edge cases
- Comments use storage format body (same as pages)
- Labels are plain strings

### Claude's Discretion

- File organization (one file per resource vs grouped)
- Internal helper functions for version fetching
- Error message wording
- Test structure

### Deferred Ideas (OUT OF SCOPE)

- Blog post CRUD (v2 requirement)
- Attachment management (v1 API only, documented in SPEC_GAPS.md)
- Bulk page operations
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PAGE-01 | User can get a page by ID with content body (storage format) | `GET /wiki/api/v2/pages/{id}?body-format=storage` — override `pages get-by-id` via mergeCommand to inject body-format=storage by default |
| PAGE-02 | User can create a page in a space with title and storage format body | `POST /wiki/api/v2/pages` — override `pages create` with friendly flags: --space-id, --title, --body, --parent-id |
| PAGE-03 | User can update a page with automatic version increment (handles 409 conflicts) | `GET /wiki/api/v2/pages/{id}` to fetch current version, then `PUT /wiki/api/v2/pages/{id}` with version.number incremented; retry once on 409 |
| PAGE-04 | User can delete a page (soft-delete to trash) | `DELETE /wiki/api/v2/pages/{id}` — already in generated code, override to document soft-delete behavior and block --purge flag |
| PAGE-05 | User can list pages in a space with pagination | `GET /wiki/api/v2/spaces/{id}/pages` or `GET /wiki/api/v2/pages?space-id=<id>` — already generated, thin override for space key resolution |
| SPCE-01 | User can list all spaces with pagination | `GET /wiki/api/v2/spaces` — already generated; thin override or use as-is if no extra logic needed |
| SPCE-02 | User can get space details by ID | `GET /wiki/api/v2/spaces/{id}` — already generated; thin override for key resolution |
| SPCE-03 | CLI transparently resolves space keys to numeric IDs where needed | Helper `resolveSpaceID(ctx, c, keyOrID string) (string, int)` using `GET /wiki/api/v2/spaces?keys=<KEY>` → extract results[0].id |
| SRCH-01 | User can search content via CQL with `cf search --cql "<query>"` | `GET /wiki/rest/api/search?cql=<query>` (v1 API) — new `search` parent command, no generated file to merge |
| SRCH-02 | Search results are automatically paginated and merged | v1 search returns `_links.next` cursor envelope; `c.Do()` triggers existing `doWithPagination` which calls `doCursorPagination` |
| SRCH-03 | Search handles long cursor strings without 413 errors | CONTEXT.md says truncate gracefully; historical cursor bloat was a temporary Atlassian bug (fixed Sept 2025); implement URL-length guard (>4000 chars) that stops pagination and logs a warning to stderr |
| CMNT-01 | User can list comments on a page | `GET /wiki/api/v2/pages/{id}/footer-comments` — already generated in `pages get-footer-comments`; override with simpler flag interface |
| CMNT-02 | User can create a comment on a page (storage format body) | `POST /wiki/api/v2/footer-comments` with `{"pageId": "<id>", "body": {"representation": "storage", "value": "<content>"}}` |
| CMNT-03 | User can delete a comment | `DELETE /wiki/api/v2/footer-comments/{comment-id}` — already generated |
| LABL-01 | User can list labels on content | `GET /wiki/api/v2/pages/{id}/labels` — already generated in `pages get-labels`; wrap with --page-id alias |
| LABL-02 | User can add labels to content | `POST /wiki/rest/api/content/{id}/label` (v1 API) with body `[{"prefix": "global", "name": "<label>"}]` — no v2 equivalent exists |
| LABL-03 | User can remove labels from content | `DELETE /wiki/rest/api/content/{id}/label?name=<label>` (v1 API) — no v2 equivalent exists |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/spf13/cobra | already in go.mod | CLI commands and flags | Project standard |
| encoding/json | stdlib | JSON marshaling | Project standard |
| net/url | stdlib | URL path/query escaping | Project standard |
| bytes | stdlib | Body construction | Project standard |
| fmt / os / strings / io | stdlib | Error handling, I/O | Project standard |

### Internal Packages
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `github.com/sofq/confluence-cli/internal/client` | HTTP client with pagination, jq, dry-run | All commands via `client.FromContext(cmd.Context())` |
| `github.com/sofq/confluence-cli/internal/errors` | Structured JSON errors, exit codes | All error paths — `cferrors.APIError`, `cferrors.AlreadyWrittenError` |

No new external dependencies required.

---

## Architecture Patterns

### Recommended Project Structure
```
cmd/
├── pages.go         # PAGE-01..05 — overrides generated pages commands
├── spaces.go        # SPCE-01..03 — overrides generated spaces commands
├── search.go        # SRCH-01..03 — new top-level search command (no generated file)
├── comments.go      # CMNT-01..03 — overrides footer-comments subsets
├── labels.go        # LABL-01..03 — overrides pages get-labels + new v1 add/remove
└── root.go          # Add mergeCommand() and AddCommand() calls in init()
```

### Pattern 1: mergeCommand Override
**What:** Hand-written parent command replaces the generated one while preserving generated subcommands not explicitly overridden.
**When to use:** For pages, spaces, comments, labels where generated parent + some generated subcommands exist.
**Example:**
```go
// In cmd/root.go init():
mergeCommand(rootCmd, pagesCmd)    // replaces generated pagesCmd
rootCmd.AddCommand(searchCmd)      // no generated parent to merge
```

### Pattern 2: Version Auto-Increment (PAGE-03)
**What:** Fetch current version via `c.Fetch()`, parse `version.number`, increment, inject into PUT body.
**When to use:** Any `cf pages update` call.
**Example:**
```go
// Source: Confluence v2 API docs + community confirmation
func fetchPageVersion(ctx context.Context, c *client.Client, id string) (int, int) {
    body, code := c.Fetch(ctx, "GET",
        fmt.Sprintf("/wiki/api/v2/pages/%s", url.PathEscape(id)), nil)
    if code != cferrors.ExitOK {
        return 0, code
    }
    var page struct {
        Version struct {
            Number int `json:"number"`
        } `json:"version"`
    }
    if err := json.Unmarshal(body, &page); err != nil {
        return 0, cferrors.ExitError
    }
    return page.Version.Number, cferrors.ExitOK
}
```

The PUT body for page update must include (at minimum):
```json
{
  "id": "<pageId>",
  "status": "current",
  "title": "<title>",
  "body": {
    "representation": "storage",
    "value": "<storageXML>"
  },
  "version": {
    "number": <currentVersion + 1>
  }
}
```

### Pattern 3: Space Key Resolution (SPCE-03)
**What:** Accept either a numeric ID or a space key string; resolve key to numeric ID when non-numeric.
**When to use:** Any command accepting `--space-id` or `--space`.
**Example:**
```go
// Source: Confluence v2 spec — GET /spaces?keys=<KEY> returns results[0].id
func resolveSpaceID(ctx context.Context, c *client.Client, keyOrID string) (string, int) {
    // If it looks like a number, return as-is
    if _, err := strconv.ParseInt(keyOrID, 10, 64); err == nil {
        return keyOrID, cferrors.ExitOK
    }
    body, code := c.Fetch(ctx, "GET",
        fmt.Sprintf("/wiki/api/v2/spaces?keys=%s", url.QueryEscape(keyOrID)), nil)
    if code != cferrors.ExitOK {
        return "", code
    }
    var resp struct {
        Results []struct {
            ID string `json:"id"`
        } `json:"results"`
    }
    if err := json.Unmarshal(body, &resp); err != nil || len(resp.Results) == 0 {
        // write not_found error
        return "", cferrors.ExitNotFound
    }
    return resp.Results[0].ID, cferrors.ExitOK
}
```

### Pattern 4: Search with CQL (SRCH-01..03)
**What:** New `search` parent command with a single `run` subcommand (or RunE on the parent itself). Uses the v1 API because the v2 spec has no search endpoint.
**When to use:** `cf search --cql "..."`.
**Example:**
```go
// Source: Confluence v1 API — GET /wiki/rest/api/search?cql=...
// The v1 response envelope IS a cursor-paginated response:
// { "results": [...], "totalSize": N, "_links": { "next": "/wiki/rest/api/search?cursor=..." } }
// c.Do() with Paginate=true will call doCursorPagination automatically.
func runSearch(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    // ...
    cqlQuery, _ := cmd.Flags().GetString("cql")
    query := url.Values{}
    query.Set("cql", cqlQuery)
    // c.Do handles pagination via _links.next
    code := c.Do(cmd.Context(), "GET", "/wiki/rest/api/search", query, nil)
    if code != 0 {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    return nil
}
```
Note: The `c.Do()` path calls `doWithPagination` which calls `doCursorPagination`. However, `doCursorPagination` currently constructs `nextURL = c.BaseURL + nextPath` where it strips leading domain up to `/wiki/`. The v1 search `_links.next` contains a path like `/wiki/rest/api/search?cursor=...` — the stripping logic `if idx := strings.Index(nextLink, "/wiki/"); idx > 0` will handle this correctly since `/wiki/` appears in the next path.

### Pattern 5: Label Mutations via v1 API (LABL-02, LABL-03)
**What:** Label add/remove have no v2 equivalent. Use `c.Fetch()` to call v1 API directly.
**When to use:** `cf labels add` and `cf labels remove`.
**Example:**
```go
// Source: Atlassian v1 API — POST /wiki/rest/api/content/{id}/label
// Add: POST with body [{"prefix": "global", "name": "<label>"}]
// Remove: DELETE /wiki/rest/api/content/{id}/label?name=<label>
func runLabelsAdd(cmd *cobra.Command, args []string) error {
    pageID, _ := cmd.Flags().GetString("page-id")
    names, _ := cmd.Flags().GetStringSlice("labels") // comma-separated or repeated flags
    type labelBody struct {
        Prefix string `json:"prefix"`
        Name   string `json:"name"`
    }
    var body []labelBody
    for _, n := range names {
        body = append(body, labelBody{Prefix: "global", Name: n})
    }
    encoded, _ := json.Marshal(body)
    path := fmt.Sprintf("/wiki/rest/api/content/%s/label", url.PathEscape(pageID))
    respBody, code := c.Fetch(ctx, "POST", path, bytes.NewReader(encoded))
    // ...
}
```

### Pattern 6: 409 Conflict Retry (PAGE-03)
**What:** Single retry on 409 by re-fetching the current version.
**When to use:** Only for `cf pages update`.
**Example:**
```go
// First attempt
code = doPageUpdate(ctx, c, id, title, body, currentVersion+1)
if code == cferrors.ExitConflict {
    // Single retry: re-fetch version
    currentVersion, code = fetchPageVersion(ctx, c, id)
    if code != cferrors.ExitOK { return &cferrors.AlreadyWrittenError{Code: code} }
    code = doPageUpdate(ctx, c, id, title, body, currentVersion+1)
}
```

### Anti-Patterns to Avoid
- **Calling `c.Do()` when you need the response body:** Use `c.Fetch()` for any command that needs to inspect, transform, or construct the response before output.
- **Hardcoding `body-format=storage` into the URL string instead of query params:** Use `url.Values` consistently so other query flags still work.
- **Building v1 label URLs as `c.BaseURL + "/wiki/rest/api/..."`:** The `c.Fetch()` method already prepends `c.BaseURL`. Pass only the path starting with `/wiki/rest/api/...`.
- **Forgetting the `strconv` import for space key detection:** `resolveSpaceID` needs `strconv.ParseInt`.
- **Writing `{}` to stdout on DELETE success:** Generated code's `doOnce` already emits `{}` for 204 No Content. Workflow commands using `c.Do()` for DELETE get this for free; those using `c.Fetch()` must call `c.WriteOutput([]byte("{}"))` manually.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cursor pagination | Custom next-page loop | `c.Do()` with `Paginate: true` | Already handles all Confluence `_links.next` patterns, including v1 search |
| JQ filtering | Post-process JSON | `c.WriteOutput()` | Already applies `--jq` and `--pretty` |
| Dry-run output | Manual dry-run JSON | `c.DryRun` check + `c.WriteOutput()` | Pattern already established in generated code and jr reference |
| Auth headers | Manual `Authorization:` | `c.Fetch()` / `c.Do()` | `ApplyAuth()` already handles basic + bearer |
| Structured errors | Custom error JSON | `cferrors.APIError{}.WriteJSON()` + `cferrors.AlreadyWrittenError` | Exit codes and JSON format established by INFRA-01/02 |
| Body reader from stdin/flag | Custom flag parsing | Copy pattern from generated `pages_create.go` | Tested and handles `@file`, `-`, and inline string |

**Key insight:** The generated commands already handle 90% of all operations correctly. The workflow layer adds only Confluence-specific multi-step logic. When in doubt, delegate to `c.Do()`.

---

## Common Pitfalls

### Pitfall 1: Empty Page Body on GET
**What goes wrong:** `GET /wiki/api/v2/pages/{id}` returns `{"body": {}}` — no content.
**Why it happens:** v2 API requires explicit `?body-format=storage` to include the body field.
**How to avoid:** The `pages get-by-id` override must unconditionally add `body-format=storage` to the query unless the user explicitly provides a different value. Use `url.Values{"body-format": []string{"storage"}}` as the default and allow override.
**Warning signs:** Tests that check body content returning empty objects.

### Pitfall 2: Version Conflict on Page Update
**What goes wrong:** `PUT /wiki/api/v2/pages/{id}` returns 409 with message "Version must be incremented when updating a page. Current Version: [N]. Provided version: [N]".
**Why it happens:** Another process updated the page between the GET (fetch version) and PUT (update).
**How to avoid:** Implement single-retry pattern — on 409, re-fetch version and retry once. Do not retry more than once to avoid infinite loops.
**Warning signs:** Integration tests against a shared Confluence instance.

### Pitfall 3: v2 Path Prefix Mismatch
**What goes wrong:** `c.Fetch(ctx, "GET", "/wiki/api/v2/pages/123", nil)` works, but `c.Fetch(ctx, "GET", "/pages/123", nil)` does not — the client prepends `c.BaseURL` which already includes the `/wiki/api/v2` base path or it does not, depending on how the user configured `base_url`.
**Why it happens:** The generated code uses `/pages` (no prefix), while Confluence Cloud's actual URL is `https://example.atlassian.net/wiki/api/v2/pages`. The `configure` command stores `base_url` as `https://example.atlassian.net/wiki/api/v2` — so paths must be `/pages`, not `/wiki/api/v2/pages`.
**How to avoid:** Use paths WITHOUT the `/wiki/api/v2` prefix for all v2 calls (matching the generated code). For v1 calls, use full path starting with `/wiki/rest/api/` since the base URL does not include it.
**Warning signs:** 404 errors where the URL in --verbose shows a doubled path segment.

### Pitfall 4: Label Mutations — ContentID vs PageID
**What goes wrong:** v1 `POST /wiki/rest/api/content/{id}/label` uses "content IDs." Some Atlassian docs claim content IDs and page IDs differ.
**Why it happens:** In Confluence, a Page IS a Content object; its content ID equals its page ID for standard pages. This is confirmed in practice and in many community discussions.
**How to avoid:** Pass the page ID directly to the v1 label endpoint. No ID translation is needed.
**Warning signs:** 404 or 400 errors when the ID is structurally valid (numeric string).

### Pitfall 5: Search _links.next Path Prefix
**What goes wrong:** `doCursorPagination` in `client.go` strips the domain from `_links.next` with `strings.Index(nextLink, "/wiki/")`. The v1 search returns next links like `/wiki/rest/api/search?cql=...&cursor=...`. The stripping condition is `idx > 0` (not `idx >= 0`), meaning it only strips when `/wiki/` is NOT at position 0. Since v1 search paths start with `/wiki/`, `idx = 0`, so `idx > 0` is false and the path is used as-is. This is correct: `nextPath = nextLink = "/wiki/rest/api/search?cursor=..."`, and `nextURL = c.BaseURL + nextPath` where `c.BaseURL = "https://example.atlassian.net/wiki/api/v2"` — resulting in `"https://example.atlassian.net/wiki/api/v2/wiki/rest/api/search?..."` which is WRONG.
**Why it happens:** The `doCursorPagination` logic was designed for v2 endpoints. V1 search has a different URL structure.
**How to avoid:** The `search` workflow command must NOT rely on `c.Do()` with auto-pagination for v1 search. Instead, implement its own pagination loop using `c.Fetch()` that constructs next URLs from `_links.next` by appending to the base domain only (strip the path prefix entirely and use `c.BaseURL` up to the domain). Alternatively, derive the full next URL from `_links.base` + `_links.next` as the v1 response includes both. The CONTEXT.md confirms: "The client's existing `doCursorPagination` handles the merge; search just needs to call `c.Do()`" — this means the planner intends `c.Do()` to work. **Verify this carefully**: the v1 response `_links.next` may be a full URL like `https://example.atlassian.net/wiki/rest/api/search?cql=...&cursor=...`. If so, `strings.Index(fullURL, "/wiki/")` would be `> 0` (since scheme comes before), and the stripping logic correctly extracts `/wiki/rest/api/search?...`, then `nextURL = c.BaseURL + "/wiki/rest/api/search?..."` = doubled domain. **This is a verified pitfall that needs a workaround.** The search command should use `--no-paginate` mode (call `c.Do()` with `Paginate: false` equivalent) and handle its own pagination, or accept only one page of results (set `--no-paginate` by default).

**Recommended resolution:** Implement search pagination manually using `c.Fetch()` in a loop, accumulate results, then call `c.WriteOutput()` with the merged result — same pattern as `doCursorPagination` but with the correct URL construction for v1 (`nextURL = strings.Split(c.BaseURL, "/wiki/")[0] + _links.next`).

### Pitfall 6: Missing `init()` call for new commands
**What goes wrong:** A new parent command file (e.g., `cmd/search.go`) with `func init()` that calls `rootCmd.AddCommand(searchCmd)` — but `rootCmd` is declared in `cmd/root.go`, not in `cmd/search.go`.
**Why it happens:** Go packages initialize all `init()` functions, but the variable `rootCmd` is unexported and accessible within the `cmd` package. The pattern works: `cmd/raw.go` already does `rootCmd.AddCommand(rawCmd)` in its own `init()`.
**How to avoid:** Follow the existing `cmd/raw.go` and `cmd/configure.go` pattern exactly.

---

## Code Examples

Verified patterns from the codebase and official sources:

### Fetch Current Page Version
```go
// Source: internal/client/client.go Fetch() method
func fetchPageVersion(ctx context.Context, c *client.Client, id string) (int, int) {
    body, code := c.Fetch(ctx, "GET",
        fmt.Sprintf("/pages/%s", url.PathEscape(id)), nil)
    if code != cferrors.ExitOK {
        return 0, code
    }
    var page struct {
        Version struct {
            Number int `json:"number"`
        } `json:"version"`
        Title string `json:"title"`
    }
    if err := json.Unmarshal(body, &page); err != nil {
        apiErr := &cferrors.APIError{
            ErrorType: "connection_error",
            Message:   "failed to parse page version: " + err.Error(),
        }
        apiErr.WriteJSON(c.Stderr)
        return 0, cferrors.ExitError
    }
    return page.Version.Number, cferrors.ExitOK
}
```

### Page Update with Version Increment
```go
// Source: Confluence v2 API docs (confirmed PUT /wiki/api/v2/pages/{id} schema)
type pageUpdateBody struct {
    ID      string `json:"id"`
    Status  string `json:"status"`
    Title   string `json:"title"`
    Body    struct {
        Representation string `json:"representation"`
        Value          string `json:"value"`
    } `json:"body"`
    Version struct {
        Number int `json:"number"`
    } `json:"version"`
}

func doPageUpdate(ctx context.Context, c *client.Client, id, title, storageValue string, versionNumber int) int {
    var reqBody pageUpdateBody
    reqBody.ID = id
    reqBody.Status = "current"
    reqBody.Title = title
    reqBody.Body.Representation = "storage"
    reqBody.Body.Value = storageValue
    reqBody.Version.Number = versionNumber
    encoded, _ := json.Marshal(reqBody)
    respBody, code := c.Fetch(ctx, "PUT",
        fmt.Sprintf("/pages/%s", url.PathEscape(id)),
        bytes.NewReader(encoded))
    if code != cferrors.ExitOK {
        return code
    }
    return c.WriteOutput(respBody)
}
```

### Space Key Resolution
```go
// Source: Confluence v2 spec — GET /spaces?keys=<KEY>
func resolveSpaceID(ctx context.Context, c *client.Client, keyOrID string) (string, int) {
    if _, err := strconv.ParseInt(keyOrID, 10, 64); err == nil {
        return keyOrID, cferrors.ExitOK
    }
    body, code := c.Fetch(ctx, "GET",
        fmt.Sprintf("/spaces?keys=%s", url.QueryEscape(keyOrID)), nil)
    if code != cferrors.ExitOK {
        return "", code
    }
    var resp struct {
        Results []struct {
            ID string `json:"id"`
        } `json:"results"`
    }
    if err := json.Unmarshal(body, &resp); err != nil || len(resp.Results) == 0 {
        apiErr := &cferrors.APIError{
            ErrorType: "not_found",
            Message:   fmt.Sprintf("no space found with key %q", keyOrID),
        }
        apiErr.WriteJSON(c.Stderr)
        return "", cferrors.ExitNotFound
    }
    return resp.Results[0].ID, cferrors.ExitOK
}
```

### Add Label via v1 API
```go
// Source: Confluence v1 API — confirmed POST /wiki/rest/api/content/{id}/label
// Body is array of {prefix, name} objects
type labelItem struct {
    Prefix string `json:"prefix"`
    Name   string `json:"name"`
}
func runLabelsAdd(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    // ...
    pageID, _ := cmd.Flags().GetString("page-id")
    labelsStr, _ := cmd.Flags().GetStringSlice("label") // --label flag, repeatable
    var items []labelItem
    for _, l := range labelsStr {
        items = append(items, labelItem{Prefix: "global", Name: l})
    }
    encoded, _ := json.Marshal(items)
    path := fmt.Sprintf("/wiki/rest/api/content/%s/label", url.PathEscape(pageID))
    respBody, code := c.Fetch(cmd.Context(), "POST", path, bytes.NewReader(encoded))
    if code != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    return c.WriteOutput(respBody) // returns list of labels now on page... or use {}
}
```

### Remove Label via v1 API
```go
// Source: Confluence v1 API — DELETE /wiki/rest/api/content/{id}/label?name=<label>
func runLabelsRemove(cmd *cobra.Command, args []string) error {
    // ...
    pageID, _ := cmd.Flags().GetString("page-id")
    labelName, _ := cmd.Flags().GetString("label")
    path := fmt.Sprintf("/wiki/rest/api/content/%s/label?name=%s",
        url.PathEscape(pageID), url.QueryEscape(labelName))
    _, code := c.Fetch(cmd.Context(), "DELETE", path, nil)
    if code != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    out, _ := json.Marshal(map[string]string{"status": "removed", "label": labelName})
    return c.WriteOutput(out)
}
```

### mergeCommand Registration (in root.go init())
```go
// Source: cmd/root.go existing mergeCommand pattern
// Add after generated.RegisterAll(rootCmd):
mergeCommand(rootCmd, pagesCmd)     // overrides generated pagesCmd
mergeCommand(rootCmd, spacesCmd)    // overrides generated spacesCmd
mergeCommand(rootCmd, commentsCmd)  // overrides generated commentsCmd (footer-comments)
mergeCommand(rootCmd, labelsCmd)    // overrides generated labelsCmd
rootCmd.AddCommand(searchCmd)       // no generated search command to merge
```

Note: `commentsCmd` in the workflow file should use `Use: "comments"` to allow `mergeCommand` to match. The generated `commentsCmd` is the properties-only `comments` parent — the workflow override adds the user-facing list/create/delete subcommands on top of it, preserving the generated property subcommands.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| v1 `GET /wiki/rest/api/content/{id}?expand=body.storage` | v2 `GET /wiki/api/v2/pages/{id}?body-format=storage` | v2 API launch 2023 | `expand` no longer works in v2 |
| v1 offset-based search `?start=0&limit=25` | v1 cursor `_links.next` | Atlassian migration ~2022 | Don't use `start` parameter |
| Manual version tracking for page updates | Read version from GET then increment | Always in v2 | Can't skip the GET step |
| v2 label add/remove | Still v1 only (CONFCLOUD-76866) | Not yet implemented as of 2026-03 | Must use v1 API for mutations |

**Deprecated/outdated:**
- `?expand=body.storage`: Only works in v1, returns empty in v2 — replaced by `?body-format=storage`
- `start=N` pagination: Deprecated in favor of cursor; do not use in new code
- `GET /wiki/api/v2/pages?body-format=storage` on list endpoint: Body format on list is expensive; only inject it on single-page GET (get-by-id)

---

## Open Questions

1. **Search pagination and doCursorPagination compatibility**
   - What we know: v1 search `_links.next` may be a full absolute URL or a relative path. `doCursorPagination` logic strips domain only when `/wiki/` is found at `idx > 0`.
   - What's unclear: Whether production Confluence Cloud returns `_links.next` as full URL or relative path in search responses.
   - Recommendation: Implement search with its own pagination loop using `c.Fetch()` to avoid assumptions about `doCursorPagination` compatibility. This is safer and adds ~20 lines.

2. **Comments — "comments" vs "footer-comments" parent command**
   - What we know: The spec has `footer-comments` (v2) and `inline-comments` (v2). The generated `commentsCmd` (Use: "comments") only has content property subcommands. CMNT-01/02/03 requirements use "comments" terminology.
   - What's unclear: Should `cf comments` map to footer-comments (the most common type) or be a neutral dispatch?
   - Recommendation: Map `cf comments` to footer-comments exclusively; document inline-comments as available via `cf inline-comments` (the generated command).

3. **Label `--label` flag design — single vs multi**
   - What we know: v1 POST accepts an array of label objects; DELETE only removes one label per call.
   - What's unclear: Whether LABL-02 should accept multiple labels at once (`--label foo --label bar`) or single.
   - Recommendation: Accept `StringSlice` for add (builds array in one API call), single `String` for remove (one label per delete call). This matches the v1 API shape naturally.

---

## Sources

### Primary (HIGH confidence)
- Internal codebase: `cmd/generated/pages.go`, `cmd/generated/spaces.go`, `cmd/generated/comments.go`, `cmd/generated/labels.go`, `cmd/generated/footer_comments.go` — confirmed generated API surface
- Internal codebase: `internal/client/client.go` — confirmed `Do()`, `Fetch()`, `WriteOutput()`, `doCursorPagination()` signatures
- Internal codebase: `cmd/root.go` — confirmed `mergeCommand()` implementation
- Internal codebase: `spec/confluence-v2.json` — confirmed absence of search endpoint and label mutation endpoints in v2
- Reference: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/workflow.go` — confirmed workflow command pattern

### Secondary (MEDIUM confidence)
- [Confluence v2 REST API — Page Group](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-page/) — GET page body-format=storage requirement
- [Confluence Community — Empty Body v2](https://community.developer.atlassian.com/t/confluence-cloud-api-v2-get-page-by-id-empty-body/80857) — body-format=storage confirmed as fix
- [Cotera — Page Updater Guide](https://cotera.co/articles/confluence-api-integration-guide) — version increment pattern confirmed
- [Atlassian Community — v2 Labels Gap](https://community.atlassian.com/forums/Confluence-questions/How-do-i-use-the-Confluence-v2-REST-api-to-create-labels-on-new/qaq-p/2720407) — v2 label mutations not yet available; v1 required
- [CONFCLOUD-76866](https://jira.atlassian.com/browse/CONFCLOUD-76866) — Atlassian issue tracking v2 label mutations

### Tertiary (LOW confidence)
- [WebSearch result: Label POST body format](https://community.atlassian.com/t5/Answers-Developer-Questions/Confluence-Rest-API-Add-label/qaq-p/499641) — `[{"prefix": "global", "name": "..."}]` format; consistent across multiple sources
- [Cursor bloat fix](https://community.developer.atlassian.com/t/confluence-rest-v1-search-endpoint-fails-cursor-of-next-url-is-extraordinarily-long-leading-to-413-error/95098) — cursor 413 issue was temporary Atlassian bug, now resolved

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all Go stdlib and existing internal packages, no new dependencies
- Architecture: HIGH — directly derived from existing codebase patterns (root.go, client.go, jira-cli workflow.go)
- Pitfalls: HIGH for items confirmed in official docs/community; MEDIUM for search pagination URL compatibility (open question)
- API shapes: HIGH for v2 pages/spaces/footer-comments (from spec); MEDIUM for v1 label and search (from official docs + community)

**Research date:** 2026-03-20
**Valid until:** 2026-06-20 (stable v2 API; 90-day estimate)
