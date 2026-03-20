# Phase 8: Attachments - Research

**Researched:** 2026-03-20
**Domain:** Confluence attachment CRUD (v2 list/get/delete + v1 multipart upload)
**Confidence:** HIGH

## Summary

Phase 8 adds four attachment operations to the `cf` CLI: list, get metadata, upload, and delete. Three of these (list, get, delete) use standard v2 API endpoints that already have generated commands in `cmd/generated/attachments.go`. The fourth (upload) requires the v1 REST API because Confluence v2 has no upload endpoint (tracked as CONFCLOUD-77196). This is the same v1 API pattern already established by `cmd/search.go` (searchV1Domain + fetchV1) and `cmd/labels.go` (fetchV1WithBody).

The primary technical challenge is constructing the correct v1 URL for upload. The `c.BaseURL` contains either `https://domain/wiki/api/v2` (direct auth) or `https://api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2` (OAuth2). The existing `searchV1Domain()` function splits on `/wiki/` to extract the domain prefix, which works correctly for both URL patterns. Upload also requires a `X-Atlassian-Token: no-check` header and `multipart/form-data` encoding, both handled via Go stdlib `mime/multipart`.

**Primary recommendation:** Create a hand-written `cmd/attachments.go` with `list` and `upload` subcommands. Use `mergeCommand` to overlay onto the generated `attachments` parent, inheriting generated `get-by-id`, `delete`, and other v2 subcommands. Extract `searchV1Domain` into a shared helper or import from `cmd/search.go` (it is already package-level in `cmd`).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Upload source: `--file` flag with filesystem path only -- no stdin support (multipart Content-Length requires seekable file)
- Upload output: Return full JSON response from the API (id, title, mediaType, fileSize, download link) -- consistent with all other commands
- v1 API upload: POST `/rest/api/content/{id}/child/attachment` with `X-Atlassian-Token: no-check` header, multipart/form-data with file part
- Domain extraction via `searchV1Domain()` pattern from cmd/search.go
- v2 API for list/get/delete: standard generated commands or hand-written wrappers as needed

### Claude's Discretion
- Whether to extract searchV1Domain into a shared helper or reuse existing package-level function (it is already accessible within `cmd` package)
- Exact multipart form construction (mime/multipart vs manual boundary)
- Whether list/get/delete need hand-written wrappers or can use generated commands as-is

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ATCH-01 | User can list attachments on content | Generated `pages get-attachments` exists for page-specific listing; hand-written `attachments list --page-id` wrapper provides the required UX via `GET /pages/{id}/attachments` v2 endpoint |
| ATCH-02 | User can get attachment metadata by ID | Generated `attachments get-by-id --id` already works via `GET /attachments/{id}` v2 endpoint; no hand-written wrapper needed |
| ATCH-03 | User can upload an attachment to content (v1 API multipart) | Requires hand-written upload command using v1 API `POST /rest/api/content/{id}/child/attachment` with multipart/form-data, `X-Atlassian-Token: no-check`, and `searchV1Domain` for URL construction |
| ATCH-04 | User can delete an attachment | Generated `attachments delete --id` already works via `DELETE /attachments/{id}` v2 endpoint; no hand-written wrapper needed |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `mime/multipart` | Go 1.25.8 | Multipart form-data encoding for upload | Standard Go approach; no external deps needed |
| Go stdlib `os` | Go 1.25.8 | File I/O for reading upload file | Required for `--file` flag filesystem access |
| Go stdlib `path/filepath` | Go 1.25.8 | Extract filename from path for multipart form field | Provides `filepath.Base()` for portable filename extraction |
| Go stdlib `net/http` | Go 1.25.8 | Direct HTTP request for v1 API upload | Already used by `fetchV1` and `fetchV1WithBody` patterns |
| Go stdlib `mime` | Go 1.25.8 | Detect content type from file extension | `mime.TypeByExtension()` for setting correct media type in multipart |

### Supporting
No new dependencies. All attachment operations use existing project infrastructure.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `mime/multipart` | Manual boundary construction | Manual is error-prone with boundary escaping; `mime/multipart` is correct by construction |
| `mime.TypeByExtension` | `http.DetectContentType` (sniffing) | Extension-based is simpler and sufficient; Confluence accepts any media type |

**Installation:**
No new packages needed. Zero dependency change.

## Architecture Patterns

### Recommended File Structure
```
cmd/
  attachments.go       # hand-written: list (with --page-id), upload subcommands + parent command
  attachments_test.go  # tests for upload multipart construction and list behavior
cmd/generated/
  attachments.go       # already exists: get-by-id, delete, get-labels, get-versions, etc.
```

### Pattern 1: Hand-Written Parent with mergeCommand
**What:** Create a hand-written `attachmentsCmd` parent in `cmd/attachments.go` that overrides the generated parent via `mergeCommand(rootCmd, attachmentsCmd)` in `cmd/root.go init()`. Generated subcommands (get-by-id, delete, get-labels, etc.) are automatically preserved.
**When to use:** When some subcommands need custom logic (list, upload) while others work fine as generated.
**Example:**
```go
// cmd/attachments.go
var attachmentsCmd = &cobra.Command{
    Use:   "attachments",
    Short: "Confluence attachment operations",
    FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
    RunE: func(cmd *cobra.Command, args []string) error {
        if len(args) > 0 {
            return fmt.Errorf("unknown command %q for %q; run `cf schema attachments` to list operations", args[0], cmd.CommandPath())
        }
        return fmt.Errorf("missing subcommand for %q; run `cf schema attachments` to list operations", cmd.CommandPath())
    },
}
```

### Pattern 2: v1 API Upload with searchV1Domain
**What:** Use `searchV1Domain(c.BaseURL)` to extract the domain prefix, then construct the v1 upload URL as `domain + "/wiki/rest/api/content/{id}/child/attachment"`. This works for both direct auth (`https://domain/wiki/api/v2` -> `https://domain`) and OAuth2 (`https://api.atlassian.com/ex/confluence/{cloudId}/wiki/rest/api/v2` -> `https://api.atlassian.com/ex/confluence/{cloudId}`).
**When to use:** Any v1 API call where `c.BaseURL` contains the v2 path suffix.
**Example:**
```go
domain := searchV1Domain(c.BaseURL)
fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/child/attachment", url.PathEscape(pageID))

// Build multipart body
var buf bytes.Buffer
writer := multipart.NewWriter(&buf)
part, _ := writer.CreateFormFile("file", filepath.Base(filePath))
f, _ := os.Open(filePath)
io.Copy(part, f)
f.Close()
writer.Close()

req, _ := http.NewRequestWithContext(ctx, "POST", fullURL, &buf)
req.Header.Set("Content-Type", writer.FormDataContentType())
req.Header.Set("X-Atlassian-Token", "no-check")
req.Header.Set("Accept", "application/json")
c.ApplyAuth(req)
```

### Pattern 3: Reuse fetchV1WithBody for Non-Multipart v1 Calls
**What:** The existing `fetchV1WithBody()` in `cmd/labels.go` handles method, URL, body, auth, and error writing. For upload, a similar but distinct function is needed because upload requires multipart Content-Type (not application/json) and the `X-Atlassian-Token` header.
**When to use:** Upload command needs a specialized v1 request function that sets multipart headers instead of JSON headers.

### Anti-Patterns to Avoid
- **Using `c.Do()` or `c.Fetch()` for upload:** These append path to `c.BaseURL` (v2), causing URL doubling. Upload must construct the full v1 URL independently.
- **Setting Content-Type manually with fixed boundary:** Always use `writer.FormDataContentType()` from `mime/multipart` to get the correct boundary.
- **Reading entire file into memory:** Use `io.Copy` from `os.File` to the multipart writer. For the `bytes.Buffer` approach, the file does get buffered, but this is acceptable for typical attachment sizes. For very large files, `io.Pipe` could be used but adds complexity.
- **Forgetting `X-Atlassian-Token: no-check`:** Without this header, Confluence returns 403 XSRF check failed. This is the most common upload bug.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Multipart encoding | Manual boundary/CRLF construction | `mime/multipart.Writer` | Boundary escaping, CRLF line endings, Content-Disposition headers are subtle |
| MIME type detection | File magic number sniffing | `mime.TypeByExtension(filepath.Ext(path))` | Simpler, no file read needed; Confluence infers type anyway |
| v2 attachment get/delete | Hand-written wrappers | Generated commands (already exist) | Generated code handles path construction, flags, and error handling correctly |
| URL domain extraction | Custom URL parsing | `searchV1Domain()` (already exists in cmd package) | Battle-tested in search and labels; handles both direct and OAuth2 URL formats |

**Key insight:** Only two subcommands need hand-written code: `list` (to provide `--page-id` UX matching success criteria) and `upload` (v1 multipart). The generated commands handle get-by-id, delete, and all other attachment operations correctly.

## Common Pitfalls

### Pitfall 1: URL Doubling with v1 Paths
**What goes wrong:** Using `c.BaseURL + "/wiki/rest/api/content/..."` creates `https://domain/wiki/api/v2/wiki/rest/api/content/...` -- a 404.
**Why it happens:** `c.BaseURL` already contains `/wiki/api/v2`. Developers forget to strip this before appending v1 paths.
**How to avoid:** Always use `searchV1Domain(c.BaseURL)` to get the scheme+host prefix, then append the full v1 path.
**Warning signs:** 404 errors on upload; URL in verbose output contains `/api/v2/wiki/rest/`.

### Pitfall 2: Missing X-Atlassian-Token Header
**What goes wrong:** Upload returns 403 with "XSRF check failed" message.
**Why it happens:** Confluence requires `X-Atlassian-Token: no-check` on all content-modifying v1 API requests to bypass XSRF protection. This is not required for v2 API calls.
**How to avoid:** Always set `req.Header.Set("X-Atlassian-Token", "no-check")` on the upload request.
**Warning signs:** 403 on upload that works fine with curl (curl examples in docs always include this header).

### Pitfall 3: Wrong Content-Type on Multipart Upload
**What goes wrong:** Upload fails with 400 or 415 because Content-Type header doesn't include the multipart boundary.
**Why it happens:** Setting `Content-Type: multipart/form-data` without the boundary parameter. The boundary is generated by `mime/multipart.Writer` and must be included.
**How to avoid:** Use `writer.FormDataContentType()` which returns the full Content-Type with boundary (e.g., `multipart/form-data; boundary=abc123`).
**Warning signs:** 400 error with message about malformed multipart data.

### Pitfall 4: Multipart Field Name Must Be "file"
**What goes wrong:** Upload returns 400 with "Required request part 'file' is not present".
**Why it happens:** Confluence v1 attachment API expects the multipart part to be named `file`. Using other names like `attachment` or `data` fails.
**How to avoid:** Use `writer.CreateFormFile("file", filename)` -- first argument is the field name.
**Warning signs:** 400 with "Required request part" message despite sending valid multipart data.

### Pitfall 5: Generated `attachments get` Lists ALL Attachments, Not Per-Page
**What goes wrong:** User expects `cf attachments list --page-id 123` but the generated `attachments get` command hits `GET /attachments` (global list) with no page filter.
**Why it happens:** The v2 API has separate endpoints: `/attachments` (global) vs `/pages/{id}/attachments` (page-specific). The generated command only covers the global endpoint.
**How to avoid:** Hand-written `list` subcommand that uses `GET /pages/{id}/attachments` when `--page-id` is provided, or `GET /attachments` for global listing.
**Warning signs:** List returns attachments from all pages instead of the specified page.

## Code Examples

### Upload Attachment (v1 API, multipart/form-data)
```go
// Source: established pattern from cmd/labels.go fetchV1WithBody + Confluence v1 attachment docs
func uploadAttachment(cmd *cobra.Command, c *client.Client, pageID, filePath string) ([]byte, int) {
    f, err := os.Open(filePath)
    if err != nil {
        apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "cannot open file: " + err.Error()}
        apiErr.WriteJSON(c.Stderr)
        return nil, cferrors.ExitValidation
    }
    defer f.Close()

    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)
    part, err := writer.CreateFormFile("file", filepath.Base(filePath))
    if err != nil {
        apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to create multipart: " + err.Error()}
        apiErr.WriteJSON(c.Stderr)
        return nil, cferrors.ExitError
    }
    if _, err := io.Copy(part, f); err != nil {
        apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to copy file: " + err.Error()}
        apiErr.WriteJSON(c.Stderr)
        return nil, cferrors.ExitError
    }
    writer.Close()

    domain := searchV1Domain(c.BaseURL)
    fullURL := domain + fmt.Sprintf("/wiki/rest/api/content/%s/child/attachment", url.PathEscape(pageID))

    req, err := http.NewRequestWithContext(cmd.Context(), "POST", fullURL, &buf)
    if err != nil {
        apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to create request: " + err.Error()}
        apiErr.WriteJSON(c.Stderr)
        return nil, cferrors.ExitError
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())
    req.Header.Set("X-Atlassian-Token", "no-check")
    req.Header.Set("Accept", "application/json")
    if err := c.ApplyAuth(req); err != nil {
        apiErr := &cferrors.APIError{ErrorType: "auth_error", Message: err.Error()}
        apiErr.WriteJSON(c.Stderr)
        return nil, cferrors.ExitAuth
    }

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: err.Error()}
        apiErr.WriteJSON(c.Stderr)
        return nil, cferrors.ExitError
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode >= 400 {
        apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(body)), "POST", fullURL, resp)
        apiErr.WriteJSON(c.Stderr)
        return nil, apiErr.ExitCode()
    }
    return body, cferrors.ExitOK
}
```

### List Attachments for Page (v2 API)
```go
// Source: follows pages_get_attachments pattern from cmd/generated/pages.go line 261
func runListAttachments(cmd *cobra.Command, args []string) error {
    c, err := client.FromContext(cmd.Context())
    if err != nil {
        return err
    }
    pageID, _ := cmd.Flags().GetString("page-id")
    if strings.TrimSpace(pageID) == "" {
        apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--page-id must not be empty"}
        apiErr.WriteJSON(c.Stderr)
        return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
    }
    path := fmt.Sprintf("/pages/%s/attachments", url.PathEscape(pageID))
    code := c.Do(cmd.Context(), "GET", path, nil, nil)
    if code != 0 {
        return &cferrors.AlreadyWrittenError{Code: code}
    }
    return nil
}
```

### Root Registration
```go
// In cmd/root.go init():
mergeCommand(rootCmd, attachmentsCmd) // Phase 8: attachment workflow overrides
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| v1 attachment CRUD | v2 for read/delete, v1 for upload only | Confluence v2 API (2024+) | Upload still requires v1 fallback; CONFCLOUD-77196 tracks the gap |
| `searchV1Domain` in each file | Reusable within `cmd` package | Already in place | No extraction needed; all `cmd/*.go` files share the package |

**Deprecated/outdated:**
- v1 attachment GET/DELETE endpoints: Still functional but v2 equivalents are preferred and already generated

## Open Questions

1. **Upload response format (v1 vs v2)**
   - What we know: v1 `POST .../child/attachment` returns a JSON array of attachment objects (even for single file upload). v2 returns a single object.
   - What's unclear: Whether the v1 response array should be unwrapped to a single object for consistency.
   - Recommendation: Return the raw v1 response. The user gets the full API response; they can use `--jq '.[0]'` if they want a single object. Document this in the command help text.

2. **Dry-run behavior for upload**
   - What we know: `c.DryRun` is handled by `c.Do()` but upload bypasses `Do` for the v1 path.
   - What's unclear: How to present dry-run for multipart upload.
   - Recommendation: Check `c.DryRun` before making the HTTP call; emit JSON with method, URL, filename, and file size. Skip the actual upload.

## Sources

### Primary (HIGH confidence)
- `cmd/search.go` -- `searchV1Domain()` implementation, `fetchV1()` pattern (direct code inspection)
- `cmd/labels.go` -- `fetchV1WithBody()` implementation for v1 POST/DELETE with auth (direct code inspection)
- `cmd/generated/attachments.go` -- Generated v2 attachment commands with all flags (direct code inspection)
- `cmd/generated/pages.go` -- Generated `pages get-attachments` command showing v2 page-attachment endpoint (direct code inspection)
- `cmd/blogposts.go` -- Recent Phase 7 hand-written command pattern with mergeCommand (direct code inspection)
- `cmd/root.go` -- mergeCommand pattern, OAuth2 base URL transformation (direct code inspection)
- `internal/client/client.go` -- ApplyAuth, Do, Fetch, WriteOutput methods (direct code inspection)
- `.planning/research/SUMMARY.md` -- v1 attachment upload confirmed as only upload path; zero new deps policy
- `.planning/research/PITFALLS.md` -- URL doubling bug (Pitfall 11), X-Atlassian-Token requirement

### Secondary (MEDIUM confidence)
- [Confluence REST API v1 - Content Attachments](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content---attachments/) -- v1 upload endpoint documentation
- [CONFCLOUD-77196](https://jira.atlassian.com/browse/CONFCLOUD-77196) -- v2 upload endpoint missing, confirmed open

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all stdlib, no new deps, patterns verified in existing codebase
- Architecture: HIGH - follows exact same pattern as blogposts.go (Phase 7) and labels.go (Phase 3)
- Pitfalls: HIGH - URL doubling already occurred and was fixed in this codebase; all other pitfalls from official Atlassian docs

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (stable v1 API; v2 upload gap unlikely to be resolved within 30 days)
