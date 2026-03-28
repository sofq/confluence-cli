# Phase 15: Workflow Commands - Research

**Researched:** 2026-03-28
**Domain:** Confluence CLI workflow subcommands -- move, copy, publish, comment, restrict, archive
**Confidence:** HIGH

## Summary

Phase 15 adds six workflow subcommands under a `cf workflow` parent command: move, copy, publish, comment, restrict, and archive. The codebase already has all necessary infrastructure: `fetchV1WithBody()` handles v1 POST/PUT/DELETE with JSON body (used by labels add/remove), `SearchV1Domain()` extracts the base domain for v1 URL construction, `c.Fetch()` handles v2 API calls, and `fetchPageVersion()` + `doPageUpdate()` provide page update primitives. The jr workflow command pattern (`workflowCmd` parent + child commands registered in `init()`) maps directly.

Four of six subcommands use v1-only endpoints (move, copy, restrict, archive) while two use v2 (publish, comment). The v1 helper `fetchV1WithBody()` already supports arbitrary HTTP methods with JSON bodies -- no new infrastructure is needed. The copy and archive endpoints are asynchronous (return long task IDs), requiring a simple poll loop with configurable timeout using the existing `internal/duration` package.

**Primary recommendation:** Implement as a single `cmd/workflow.go` file with the parent command and all six subcommands. Use `fetchV1WithBody()` for v1 endpoints and `c.Fetch()` for v2 endpoints. Register via `rootCmd.AddCommand(workflowCmd)` in `cmd/root.go`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** `workflow move --id <pageId> --target-id <parentId>` -- v2 `PUT /pages/{id}` with updated `parentId` field. Optional `--space-id` for cross-space moves. If API returns async task, poll for completion
- **D-02:** `workflow copy --id <pageId> --target-id <parentId>` -- v1 `POST /wiki/rest/api/content/{id}/copy` (no v2 copy endpoint). Uses `searchV1Domain` pattern for v1 base URL construction
- **D-03:** `workflow publish --id <pageId>` -- v2 `PUT /pages/{id}` updating `status` from "draft" to "current". Requires page title and version number bump (standard update semantics)
- **D-04:** `workflow comment --id <pageId> --body "text"` -- v2 `POST /pages/{id}/footer-comments` reusing the existing footer comments endpoint pattern from `cmd/comments.go`. Plain text input auto-wrapped in `<p>` storage format tags
- **D-05:** `workflow restrict --id <pageId>` -- v1 restrictions API (`/wiki/rest/api/content/{id}/restriction`). GET for viewing, PUT for adding, DELETE for removing. Uses `searchV1Domain` for v1 base URL
- **D-06:** `workflow archive --id <pageId>` -- v2 `POST /pages/archive` bulk archive endpoint with single-page payload `{"pages": [{"id": "..."}]}`
- **D-07:** Move and copy operations block and poll by default until completion. Poll interval: 1 second. Default timeout: 60 seconds
- **D-08:** `--no-wait` flag on move and copy returns the operation/task response immediately without polling -- agents can poll separately if needed
- **D-09:** `--timeout <duration>` flag overrides the default 60s timeout for async operations. Uses `duration.Parse()` from Phase 12
- **D-10:** `workflow comment` takes `--body` as plain text string, wraps in `<p>...</p>` storage format tags automatically. No XHTML parsing or complex conversion -- simple paragraph wrapping
- **D-11:** This is a convenience wrapper over the existing footer comments API. Agents needing full control (inline comments, rich formatting) use `cf comments create` directly
- **D-12:** No flags = view mode: `workflow restrict --id <pageId>` GETs and displays current restrictions as JSON
- **D-13:** `--add` flag = add restriction: `--add --operation read|update --user <accountId>` or `--group <groupName>`
- **D-14:** `--remove` flag = remove restriction: `--remove --operation read|update --user <accountId>` or `--group <groupName>`
- **D-15:** `--operation` supports `read` and `update` (the two Confluence restriction operations)
- **D-16:** Supports both `--user` (accountId) and `--group` (group name) identifiers for restrictions
- **D-17:** `--copy-attachments` boolean (default false) -- include attachments in copy
- **D-18:** `--copy-labels` boolean (default false) -- include labels in copy
- **D-19:** `--copy-permissions` boolean (default false) -- include permissions in copy
- **D-20:** `--title` string -- title for the copied page (v1 copy API uses `destination.value` + `name` fields)
- **D-21:** `--target-id` string (required) -- destination parent page ID for copy
- **D-22:** `workflowCmd` parent command with Use: "workflow", Short: "Content lifecycle operations". Registered to root via `rootCmd.AddCommand(workflowCmd)`
- **D-23:** Each subcommand (move, copy, publish, comment, restrict, archive) is a child of workflowCmd
- **D-24:** All subcommands require `--id` flag (page ID) -- consistent with diff, export, and other page-targeting commands

### Claude's Discretion
- Exact v1 copy API request body structure (validated during research -- see Code Examples below)
- Whether move actually needs async polling or if v2 PUT is synchronous (validated -- see Architecture Patterns below)
- Exact v1 restrictions API request/response shape (validated -- see Code Examples below)
- Whether archive v2 endpoint requires any additional fields beyond page ID
- Test case organization and helper patterns
- Error message wording for validation failures
- Whether `--space-id` on move is a separate flag or inferred from target

### Deferred Ideas (OUT OF SCOPE)
- **WKFL-07 (restore)**: Restore a previous page version -- deferred to future milestone per REQUIREMENTS.md
- **WKFL-08 (bulk move)**: Bulk move multiple pages -- deferred to future milestone per REQUIREMENTS.md
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| WKFL-01 | User can move a page to a different parent or space via `workflow move` | v1 move API `PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}` verified. `fetchV1WithBody()` helper already supports PUT. Position "append" = child of target. |
| WKFL-02 | User can copy a page with options via `workflow copy` | v1 copy API `POST /wiki/rest/api/content/{id}/copy` verified. Request body shape documented. Returns long task for async polling. |
| WKFL-03 | User can publish a draft page via `workflow publish` | v2 `PUT /pages/{id}` with status "current" + version bump. Reuses existing `fetchPageVersion()` + page update pattern from `cmd/pages.go`. |
| WKFL-04 | User can add a plain-text comment via `workflow comment` | v2 `POST /footer-comments` already implemented in `cmd/comments.go`. Wrapper wraps plain text in `<p>` tags and calls same endpoint. |
| WKFL-05 | User can view/add/remove page restrictions via `workflow restrict` | v1 restrictions API GET/PUT/DELETE at `/wiki/rest/api/content/{id}/restriction` verified. PUT body shape documented. |
| WKFL-06 | User can archive pages via `workflow archive` | v1 `POST /wiki/rest/api/content/archive` with page ID list. Returns 202 with long task. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| cobra | (existing) | CLI command framework | Already used throughout project |
| net/http | stdlib | HTTP requests for v1 API | Already used for v1 calls |
| encoding/json | stdlib | JSON marshal/unmarshal | Already used throughout |
| time | stdlib | Poll intervals, timeouts | Already used in duration package |

### Supporting (all already in project)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| internal/client | existing | `FromContext()`, `Fetch()`, `SearchV1Domain()`, `WriteOutput()` | Every subcommand |
| internal/errors | existing | `APIError`, `AlreadyWrittenError`, exit codes | All error handling |
| internal/jsonutil | existing | `MarshalNoEscape()` | JSON output |
| internal/duration | existing | `Parse()` for `--timeout` flag | Copy and move polling timeout |

**No new dependencies required.** All six subcommands use existing packages and patterns.

## Architecture Patterns

### Recommended Project Structure
```
cmd/
  workflow.go          # Parent command + all 6 subcommands + helper functions
  workflow_test.go     # Tests for all workflow subcommands
  export_test.go       # Add FetchV1WithBody export (for white-box testing)
  root.go              # Add rootCmd.AddCommand(workflowCmd) in init()
```

### Pattern 1: Workflow Parent + Child Registration (from jr)
**What:** Single file defines parent `workflowCmd` and all child commands. Children registered via `workflowCmd.AddCommand()` in `init()`. Parent registered to root in `cmd/root.go`.
**When to use:** All workflow subcommands.
**Example:**
```go
// Source: jr cmd/workflow.go pattern adapted for cf
var workflowCmd = &cobra.Command{
    Use:   "workflow",
    Short: "Content lifecycle operations",
}

var workflow_move = &cobra.Command{
    Use:   "move",
    Short: "Move a page to a different parent",
    RunE:  runWorkflowMove,
}

func init() {
    workflow_move.Flags().String("id", "", "page ID to move (required)")
    workflow_move.Flags().String("target-id", "", "target parent page ID (required)")
    // ... more flags

    workflowCmd.AddCommand(workflow_move)
    workflowCmd.AddCommand(workflow_copy)
    workflowCmd.AddCommand(workflow_publish)
    workflowCmd.AddCommand(workflow_comment)
    workflowCmd.AddCommand(workflow_restrict)
    workflowCmd.AddCommand(workflow_archive)
}
```

### Pattern 2: v1 API Call with fetchV1WithBody
**What:** Construct v1 URL using `SearchV1Domain()`, call `fetchV1WithBody()` with method, URL, and JSON body.
**When to use:** Move, copy, restrict, archive (all v1-only endpoints).
**Example:**
```go
// Source: cmd/labels.go lines 146-155 (existing pattern)
domain := client.SearchV1Domain(c.BaseURL)
fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/copy", url.PathEscape(pageID))
encoded, _ := json.Marshal(reqBody)
respBody, code := fetchV1WithBody(cmd, c, "POST", fullURL, bytes.NewReader(encoded))
if code != cferrors.ExitOK {
    return &cferrors.AlreadyWrittenError{Code: code}
}
```

### Pattern 3: v2 Page Update for Publish/Move-via-v2
**What:** Fetch current version, build update body with status change, PUT to v2 pages endpoint.
**When to use:** Publish (status draft->current).
**Example:**
```go
// Source: cmd/pages.go lines 70-84 (existing doPageUpdate pattern)
currentVersion, code := fetchPageVersion(cmd.Context(), c, id)
if code != cferrors.ExitOK {
    return &cferrors.AlreadyWrittenError{Code: code}
}
// Build update body with status: "current", version: currentVersion+1
```

### Pattern 4: Async Poll Loop for Long-Running Tasks
**What:** After POST that returns a long task ID, poll the task status endpoint at 1s intervals until complete or timeout.
**When to use:** Copy (always async), archive (returns 202). Possibly move (needs validation).
**Example:**
```go
// Poll pattern for async operations
func pollLongTask(ctx context.Context, cmd *cobra.Command, c *client.Client, taskID string, timeout time.Duration) ([]byte, int) {
    domain := client.SearchV1Domain(c.BaseURL)
    deadline := time.After(timeout)
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-deadline:
            apiErr := &cferrors.APIError{ErrorType: "timeout_error", Message: "operation timed out"}
            apiErr.WriteJSON(c.Stderr)
            return nil, cferrors.ExitError
        case <-ctx.Done():
            return nil, cferrors.ExitError
        case <-ticker.C:
            taskURL := domain + fmt.Sprintf("/wiki/rest/api/longtask/%s", url.PathEscape(taskID))
            body, code := fetchV1WithBody(cmd, c, "GET", taskURL, nil)
            if code != cferrors.ExitOK {
                return nil, code
            }
            var task struct {
                Successful bool            `json:"successful"`
                Finished   bool            `json:"finished"`
                Messages   []json.RawMessage `json:"messages"`
            }
            json.Unmarshal(body, &task)
            if task.Finished {
                if !task.Successful {
                    apiErr := &cferrors.APIError{ErrorType: "api_error", Message: "long task failed"}
                    apiErr.WriteJSON(c.Stderr)
                    return nil, cferrors.ExitError
                }
                return body, cferrors.ExitOK
            }
        }
    }
}
```

### Pattern 5: Flag Validation (from existing commands)
**What:** Get flag values, validate non-empty with `strings.TrimSpace()`, write APIError for validation failures.
**When to use:** Every subcommand, for `--id` and other required flags.
**Example:**
```go
// Source: cmd/export.go lines 43-47 (established pattern)
id, _ := cmd.Flags().GetString("id")
if strings.TrimSpace(id) == "" {
    apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--id must not be empty"}
    apiErr.WriteJSON(c.Stderr)
    return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
}
```

### Anti-Patterns to Avoid
- **Separate files per subcommand:** The jr pattern puts all workflow commands in one file. Six small subcommands do not warrant separate files.
- **Using `c.Do()` for v1 endpoints:** `c.Do()` prepends `c.BaseURL` (which is v2). v1 endpoints need full URL via `fetchV1WithBody()`.
- **Using `MarkFlagRequired()` for validation:** The project uses manual validation with APIError + AlreadyWrittenError (more consistent error format). Cobra's `MarkFlagRequired()` writes plain text errors to stderr, breaking the JSON-only contract.
- **Importing new packages:** Zero new Go dependencies. All features use stdlib + existing internal packages.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| v1 URL construction | Custom URL builder | `client.SearchV1Domain(c.BaseURL) + path` | Already used by search, labels, attachments, watch |
| v1 HTTP calls with body | Custom HTTP wrapper | `fetchV1WithBody(cmd, c, method, url, body)` | Already exists in labels.go, handles auth + errors |
| v2 page update | Custom PUT logic | `fetchPageVersion()` + `c.Fetch()` with PUT | Already in pages.go, handles version increment |
| Duration parsing | Custom parser | `duration.Parse()` from `internal/duration` | Already built in Phase 12, handles w/d/h/m |
| JSON output | Custom formatter | `c.WriteOutput(body)` or `jsonutil.MarshalNoEscape()` | Handles --jq, --pretty, no-escape |
| Error handling | Custom error types | `cferrors.APIError` + `AlreadyWrittenError` | Consistent JSON stderr across all commands |

**Key insight:** Every infrastructure component needed for Phase 15 already exists in the codebase. The work is purely wiring subcommands to existing helpers and API endpoints.

## Common Pitfalls

### Pitfall 1: Move API -- v1 vs v2 Endpoint Choice
**What goes wrong:** D-01 says "v2 PUT /pages/{id} with updated parentId". However, research shows the v2 page update may not support parentId changes for moving. The dedicated v1 move endpoint `PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}` is the reliable approach.
**Why it happens:** The v2 PUT /pages/{id} endpoint accepts parentId for page creation context but the behavior for changing parentId on an existing page is not clearly documented to trigger a move.
**How to avoid:** Use the v1 move endpoint `PUT /wiki/rest/api/content/{id}/move/append/{targetId}` which is explicitly designed for moving pages. This is what the FEATURES.md research (HIGH confidence) recommends.
**Warning signs:** If using v2 PUT and the parentId does not change, the page was not moved -- switch to v1.
**Research recommendation:** Use v1 move endpoint. The position parameter should default to `append` (child of target). The v1 move endpoint is synchronous (returns 200 with empty body on success). No async polling needed for move.

### Pitfall 2: Copy API Request Body Shape
**What goes wrong:** Using wrong field names or structure for the copy request body.
**Why it happens:** The v1 copy API has a specific shape with `destination.type` and `destination.value` fields that differ from the v2 page create shape.
**How to avoid:** Use the exact verified request body structure (see Code Examples section). The `destination.type` must be `"parent_page"` and `destination.value` is the target parent page ID.
**Warning signs:** 400 Bad Request errors from the copy endpoint.

### Pitfall 3: Restrictions API -- Must Include Self
**What goes wrong:** Setting restrictions that lock out the API caller.
**Why it happens:** The v1 restrictions PUT endpoint replaces all restrictions. If the calling user is not included in the new restrictions, they lose access.
**How to avoid:** For `--add` mode, use the individual user/group endpoints (PUT byOperation/user or byOperation/group) rather than the bulk PUT that replaces all. For `--remove`, use DELETE on the specific restriction.
**Warning signs:** 403 Forbidden after setting restrictions.

### Pitfall 4: Archive Endpoint Path
**What goes wrong:** D-06 says `POST /pages/archive` (v2 path) but the archive endpoint is actually v1 `POST /wiki/rest/api/content/archive`.
**Why it happens:** Confusion between v1 and v2 endpoint paths.
**How to avoid:** Use the v1 endpoint via `SearchV1Domain()` + `/wiki/rest/api/content/archive`. The request body is `{"pages": [{"id": "12345"}]}` (v1 content IDs).
**Warning signs:** 404 errors if using v2 path.

### Pitfall 5: Cobra Flag Contamination in Tests
**What goes wrong:** Global singleton command state leaks flag values between test cases.
**Why it happens:** Cobra retains parsed flag values on global command variables. Tests that set flags pollute subsequent tests.
**How to avoid:** Use the `ResetFlags()` + re-register pattern from `diff_test.go` (lines 32-41), or create fresh command instances per test like jr's `newTransitionCmd()` pattern.
**Warning signs:** Tests pass individually but fail when run together.

### Pitfall 6: fetchV1WithBody Scope
**What goes wrong:** Trying to use `fetchV1WithBody()` from workflow.go but it is defined in labels.go (same package `cmd`, accessible).
**Why it happens:** Developer thinks function needs to be imported.
**How to avoid:** Both `fetchV1()` (in search.go) and `fetchV1WithBody()` (in labels.go) are in package `cmd` -- directly accessible from workflow.go.
**Warning signs:** Compilation errors about undefined functions -- not possible since same package.

### Pitfall 7: Comment Endpoint -- Use Correct v2 Path
**What goes wrong:** Using wrong path for footer comments creation.
**Why it happens:** D-04 says `POST /pages/{id}/footer-comments` but the existing `comments_create` in comments.go uses `POST /footer-comments` (top-level) with `pageId` in the request body.
**How to avoid:** Follow the existing `comments_create` pattern exactly: POST to `/footer-comments` with `{"pageId": "...", "body": {"representation": "storage", "value": "..."}}`.
**Warning signs:** 404 if using `/pages/{id}/footer-comments`.

## Code Examples

### Move Page (v1 API)
```go
// Endpoint: PUT /wiki/rest/api/content/{id}/move/append/{targetId}
// Source: FEATURES.md + Atlassian API docs (HIGH confidence)
// Position values: "append" (child of target), "before" (sibling before), "after" (sibling after)
// Response: 200 OK with page content JSON (synchronous, no polling needed)
func runWorkflowMove(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    if err != nil { return err }

    id, _ := cmd.Flags().GetString("id")
    targetID, _ := cmd.Flags().GetString("target-id")
    // ... validation ...

    domain := client.SearchV1Domain(c.BaseURL)
    fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/move/append/%s",
        url.PathEscape(id), url.PathEscape(targetID))

    respBody, code := fetchV1WithBody(cmd, c, "PUT", fullURL, nil) // no body needed
    if code != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: ec}
    }
    return nil
}
```

### Copy Page (v1 API -- async)
```go
// Endpoint: POST /wiki/rest/api/content/{id}/copy
// Source: FEATURES.md + Atlassian community verified examples (HIGH confidence)
// Request body shape:
type copyRequestBody struct {
    CopyAttachments    bool              `json:"copyAttachments"`
    CopyPermissions    bool              `json:"copyPermissions"`
    CopyLabels         bool              `json:"copyLabels"`
    CopyProperties     bool              `json:"copyProperties"`
    CopyCustomContents bool              `json:"copyCustomContents"`
    Destination        copyDestination   `json:"destination"`
    PageTitle          string            `json:"pageTitle,omitempty"`
}

type copyDestination struct {
    Type  string `json:"type"`  // "parent_page" or "space"
    Value string `json:"value"` // parent page ID or space key
}

// Example request:
// {
//   "copyAttachments": false,
//   "copyPermissions": false,
//   "copyLabels": false,
//   "copyProperties": false,
//   "copyCustomContents": false,
//   "destination": {
//     "type": "parent_page",
//     "value": "67890"
//   },
//   "pageTitle": "My Copy"
// }
```

### Publish Draft (v2 API)
```go
// Endpoint: PUT /pages/{id} with status change
// Source: cmd/pages.go existing pattern (HIGH confidence)
// Reuses fetchPageVersion() + builds update body
func runWorkflowPublish(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    if err != nil { return err }

    id, _ := cmd.Flags().GetString("id")
    // ... validation ...

    // Fetch current page to get title and version
    body, code := c.Fetch(cmd.Context(), "GET", fmt.Sprintf("/pages/%s", url.PathEscape(id)), nil)
    if code != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    var page struct {
        Title   string `json:"title"`
        Version struct { Number int `json:"number"` } `json:"version"`
    }
    json.Unmarshal(body, &page)

    // Build publish request (status change to "current")
    var reqBody struct {
        ID      string `json:"id"`
        Status  string `json:"status"`
        Title   string `json:"title"`
        Version struct { Number int `json:"number"` } `json:"version"`
    }
    reqBody.ID = id
    reqBody.Status = "current"
    reqBody.Title = page.Title
    reqBody.Version.Number = page.Version.Number + 1

    encoded, _ := json.Marshal(reqBody)
    respBody, code := c.Fetch(cmd.Context(), "PUT", fmt.Sprintf("/pages/%s", url.PathEscape(id)), bytes.NewReader(encoded))
    if code != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: ec}
    }
    return nil
}
```

### Comment (v2 API -- wraps existing pattern)
```go
// Endpoint: POST /footer-comments
// Source: cmd/comments.go lines 83-98 (HIGH confidence -- exact existing pattern)
// Request body: {"pageId": "...", "body": {"representation": "storage", "value": "<p>text</p>"}}
func runWorkflowComment(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    if err != nil { return err }

    id, _ := cmd.Flags().GetString("id")
    bodyText, _ := cmd.Flags().GetString("body")
    // ... validation ...

    // Wrap plain text in storage format paragraph tags
    storageBody := "<p>" + bodyText + "</p>"

    var reqBody createCommentBody
    reqBody.PageID = id
    reqBody.Body.Representation = "storage"
    reqBody.Body.Value = storageBody

    encoded, _ := json.Marshal(reqBody)
    respBody, code := c.Fetch(cmd.Context(), "POST", "/footer-comments", bytes.NewReader(encoded))
    if code != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    if ec := c.WriteOutput(respBody); ec != cferrors.ExitOK {
        return &cferrors.AlreadyWrittenError{Code: ec}
    }
    return nil
}
```

### Restrict -- View Current Restrictions (v1 API)
```go
// Endpoint: GET /wiki/rest/api/content/{id}/restriction
// Source: Atlassian v1 API docs (HIGH confidence)
// Response: returns array of restriction objects with operation + user/group details
domain := client.SearchV1Domain(c.BaseURL)
fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/restriction", url.PathEscape(id))
body, code := fetchV1WithBody(cmd, c, "GET", fullURL, nil)
```

### Restrict -- Add Restriction (v1 API)
```go
// Endpoint: PUT /wiki/rest/api/content/{id}/restriction/byOperation/{operationKey}/user?accountId={accountId}
// Source: Atlassian community verified (MEDIUM-HIGH confidence)
// For individual user restriction:
domain := client.SearchV1Domain(c.BaseURL)
fullURL := domain + fmt.Sprintf(
    "/wiki/rest/api/content/%s/restriction/byOperation/%s/user?accountId=%s",
    url.PathEscape(id),
    url.PathEscape(operation), // "read" or "update"
    url.QueryEscape(userAccountID),
)
_, code := fetchV1WithBody(cmd, c, "PUT", fullURL, nil) // no body for individual add

// For individual group restriction:
fullURL = domain + fmt.Sprintf(
    "/wiki/rest/api/content/%s/restriction/byOperation/%s/byGroupId/%s",
    url.PathEscape(id),
    url.PathEscape(operation),
    url.PathEscape(groupID), // or group name
)
```

### Restrict -- Remove Restriction (v1 API)
```go
// Endpoint: DELETE /wiki/rest/api/content/{id}/restriction/byOperation/{operationKey}/user?accountId={accountId}
// Source: Atlassian v1 API docs (HIGH confidence)
domain := client.SearchV1Domain(c.BaseURL)
fullURL := domain + fmt.Sprintf(
    "/wiki/rest/api/content/%s/restriction/byOperation/%s/user?accountId=%s",
    url.PathEscape(id),
    url.PathEscape(operation),
    url.QueryEscape(userAccountID),
)
_, code := fetchV1WithBody(cmd, c, "DELETE", fullURL, nil)
```

### Archive Page (v1 API -- async)
```go
// Endpoint: POST /wiki/rest/api/content/archive
// Source: FEATURES.md + Atlassian community (HIGH confidence)
// Request body: {"pages": [{"id": "12345"}]}
// Response: 202 Accepted with long task info
type archiveRequest struct {
    Pages []struct {
        ID string `json:"id"`
    } `json:"pages"`
}

domain := client.SearchV1Domain(c.BaseURL)
fullURL := domain + "/wiki/rest/api/content/archive"
reqBody := archiveRequest{Pages: []struct{ ID string `json:"id"` }{{ID: id}}}
encoded, _ := json.Marshal(reqBody)
respBody, code := fetchV1WithBody(cmd, c, "POST", fullURL, bytes.NewReader(encoded))
```

### Test Pattern (from diff_test.go + jr workflow_test.go)
```go
// Source: cmd/diff_test.go pattern for running commands with test server
func runWorkflowCommand(t *testing.T, srvURL string, args ...string) (stdout, stderr string) {
    t.Helper()
    setupTemplateEnv(t, srvURL, nil)

    // Pipe stdout/stderr
    oldStdout := os.Stdout
    rOut, wOut, _ := os.Pipe()
    os.Stdout = wOut
    oldStderr := os.Stderr
    rErr, wErr, _ := os.Pipe()
    os.Stderr = wErr

    root := cmd.RootCommand()
    root.SetArgs(append([]string{"workflow"}, args...))
    _ = root.Execute()

    wOut.Close(); wErr.Close()
    os.Stdout = oldStdout; os.Stderr = oldStderr
    var outBuf, errBuf bytes.Buffer
    outBuf.ReadFrom(rOut); errBuf.ReadFrom(rErr)
    return outBuf.String(), errBuf.String()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| v1 content API for all operations | v2 for pages/comments, v1 for move/copy/restrict/archive | 2022-2023 | Must use v1 for 4 of 6 subcommands |
| Page move via v2 parentId update | v1 dedicated move endpoint (more reliable) | N/A | v1 move is the only reliable approach |
| Bulk restrictions PUT | Individual user/group add/remove endpoints | Current best practice | Prevents self-lockout |

**Note on v1 API deprecation:** Atlassian has committed to not removing v1 endpoints until at least 6 months after v2 feature parity. As of 2026-03, the move, copy, restrict, and archive endpoints have no v2 equivalents. v1 usage is safe for the foreseeable future.

## Open Questions

1. **Move endpoint async behavior**
   - What we know: FEATURES.md says move position values are "append", "before", "after". Prior research flagged this as needing live validation.
   - What's unclear: Whether the v1 move endpoint returns synchronously (200 with page JSON) or asynchronously (202 with long task ID).
   - Recommendation: Implement as synchronous first (return response directly). If 202 is returned, fall back to polling. This handles both cases. STATE.md flagged this as needing validation.

2. **Archive endpoint -- v1 vs v2**
   - What we know: D-06 says `POST /pages/archive` (v2 path). FEATURES.md says `POST /wiki/rest/api/content/archive` (v1 path).
   - What's unclear: Whether a v2 archive endpoint has been added since FEATURES.md was written.
   - Recommendation: Use v1 endpoint (verified, HIGH confidence). If implementation finds a v2 endpoint works, it can be switched. The v1 endpoint is documented and confirmed working.

3. **Restrictions -- group identifier format**
   - What we know: D-16 says `--group <groupName>`. API endpoints have both `/byGroupId/{groupId}` and possibly `/group/{groupName}` variants.
   - What's unclear: Whether the v1 API accepts group names or only group IDs.
   - Recommendation: Implement with group name first (matches D-16). The API endpoint path `/byGroupId/{id}` may accept names -- verify during implementation. If not, may need a group lookup step.

## Sources

### Primary (HIGH confidence)
- `cmd/labels.go` lines 64-108 -- `fetchV1WithBody()` helper pattern (verified in codebase)
- `cmd/search.go` lines 24-60 -- `fetchV1()` GET helper pattern (verified in codebase)
- `cmd/comments.go` lines 53-98 -- v2 footer comment create pattern (verified in codebase)
- `cmd/pages.go` lines 30-84 -- `fetchPageVersion()` + `doPageUpdate()` pattern (verified in codebase)
- `cmd/diff_test.go` lines 17-57 -- Test helper pattern (verified in codebase)
- `.planning/research/FEATURES.md` -- Prior milestone research with API endpoint summary (HIGH confidence)
- `internal/client/client.go` -- `SearchV1Domain()`, `Fetch()`, `WriteOutput()` (verified in codebase)

### Secondary (MEDIUM confidence)
- [Atlassian API v1 Content Restrictions](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content-restrictions/) -- Restriction endpoint structure
- [Move and Copy Page APIs announcement](https://community.developer.atlassian.com/t/added-move-and-copy-page-apis/37749) -- v1 move/copy confirmed
- [Confluence Cloud v2 Page API](https://developer.atlassian.com/cloud/confluence/rest/v2/api-group-page/) -- v2 page update for publish
- [Restrictions update community thread](https://community.developer.atlassian.com/t/update-content-restrictions-with-api-v1/88400) -- PUT body shape verified

### Tertiary (LOW confidence)
- Archive v2 endpoint existence -- not confirmed in official docs; using v1 fallback

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all packages already exist in project, no new dependencies
- Architecture: HIGH - follows established jr workflow pattern + existing cf command patterns
- API endpoints: HIGH for v2 (publish, comment), MEDIUM-HIGH for v1 (move, copy, restrict, archive) -- verified via FEATURES.md research + community docs
- Pitfalls: HIGH - identified from codebase patterns and API documentation

**Research date:** 2026-03-28
**Valid until:** 2026-04-28 (stable -- no API changes expected for v1 endpoints)
