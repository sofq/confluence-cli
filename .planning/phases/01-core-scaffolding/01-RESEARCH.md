# Phase 1: Core Scaffolding - Research

**Researched:** 2026-03-20
**Domain:** Go CLI infrastructure (Cobra, HTTP client, config, auth, JQ, caching, pagination)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

None explicitly locked — all implementation choices are at Claude's discretion for this pure infrastructure phase.

### Claude's Discretion

All implementation choices are at Claude's discretion. Mirror the `jr` (jira-cli-v2) reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` for all patterns: directory structure, client architecture, config resolution, error handling, and flag design.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | CLI outputs pure JSON to stdout for all commands | `json.NewEncoder` with `SetEscapeHTML(false)`; `fmt.Fprintf(os.Stdout, ...)` only; help text redirected to stderr |
| INFRA-02 | CLI outputs structured JSON errors to stderr with semantic exit codes (0=OK, 1=error, 2=auth, 3=not-found, 4=validation, 5=rate-limit, 6=conflict, 7=server-error) | `internal/errors` package: `APIError`, `ExitCodeFromStatus`, `AlreadyWrittenError` sentinel — exact constants verified in reference |
| INFRA-03 | User can configure profiles with base URL, auth type, and credentials via `cf configure` | `cmd/configure.go` pattern: flag-driven, no prompts, JSON confirmation output; config at CF_CONFIG_PATH or OS default |
| INFRA-04 | User can select profile via `--profile` flag or `CF_PROFILE` env var | `config.Resolve()` merges flags > env > file; `--profile` / `CF_PROFILE` precedence; default profile fallback chain |
| INFRA-05 | CLI supports basic auth (email + API token) and bearer token auth | `client.ApplyAuth()`: switch on `c.Auth.Type`; "basic" calls `SetBasicAuth(username, token)`; "bearer" sets Authorization header |
| INFRA-06 | User can apply JQ filter to any command output via `--jq` flag | `internal/jq` package: gojq in-process wrapper; `jq.Apply(data, filter)` called inside `client.WriteOutput()` |
| INFRA-07 | CLI automatically paginates list endpoints and merges results (cursor-based) | Confluence v2 uses cursor-based (`_links.next` or `cursor`/`limit` params) — differs from Jira's startAt; new `doCursorPagination()` needed |
| INFRA-08 | User can cache GET responses with configurable TTL via `--cache` flag | `internal/cache`: SHA-256 key from method+URL+auth context; file-system store at `os.UserCacheDir()/cf`; TTL via file mtime |
| INFRA-09 | User can make raw API calls via `cf raw <METHOD> <path>` | `cmd/raw.go` pattern: positional args; `--body`, `--query` flags; policy check deferred to RunE |
| INFRA-10 | User can preview write operations without executing via `--dry-run` flag | `c.DryRun` field in Client; checked at top of `Do()`, emits `{method, url, body}` JSON to stdout and returns |
| INFRA-11 | User can inspect HTTP request/response details via `--verbose` flag (output to stderr) | `c.VerboseLog()`: marshals `{type, method/status, url}` to stderr when `c.Verbose` is true |
| INFRA-12 | `cf --version` outputs version info as JSON | `rootCmd.SetVersionTemplate` for `--version` flag; `cmd/version.go` `versionCmd` for `cf version` subcommand |
| INFRA-13 | User can discover command tree and parameter schemas as JSON via `cf schema` | `cmd/schema_cmd.go`: uses `generated.AllSchemaOps()` + `HandWrittenSchemaOps()`; `--list`, `--compact` modes; empty `AllSchemaOps()` stub until Phase 2 |
</phase_requirements>

---

## Summary

Phase 1 establishes the entire infrastructure foundation for the `cf` CLI — no resource-specific commands, only the plumbing every subsequent phase uses. The reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` provides the complete, battle-tested pattern. The mapping from `jr` to `cf` is nearly 1:1 with two meaningful differences: (1) the env/config prefix changes from `JR_` to `CF_`, the binary name from `jr` to `cf`, and the module path to `github.com/sofq/confluence-cli`; (2) Confluence v2 uses **cursor-based pagination** (`_links.next` URL in response) rather than Jira's offset-based `startAt/total/values` pattern — this requires a new pagination handler in the client.

Everything else — the Cobra command structure, PersistentPreRunE client injection, `internal/errors` exit codes, `internal/jq` gojq wrapper, `internal/cache` file-system TTL store, `internal/config` four-level resolution, the `raw` command, the `configure` command, the `schema` command, and the `version` command — is a direct copy-and-adapt from the reference with mechanical `jr`→`cf` / `JR_`→`CF_` substitution.

**Primary recommendation:** Scaffold `go.mod`, copy the internal packages verbatim (renaming module path), adapt root.go and configure.go, implement cursor-pagination in client.go, then wire up the stub `generated.RegisterAll()` and stub `schema`. Every task is well-defined by the reference; the only design decision is the cursor-pagination implementation.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/spf13/cobra | v1.10.2 | CLI framework, command tree, flags | Exact version used by jr; mature, widely adopted Go CLI framework |
| github.com/spf13/pflag | v1.0.9 | POSIX-compliant flag parsing (cobra dep) | Pulled in by cobra; also used directly for PersistentFlags |
| github.com/itchyny/gojq | v0.12.18 | In-process JQ filter execution | No external `jq` binary dependency; same version as jr |
| github.com/tidwall/pretty | v1.2.1 | JSON pretty-printing | Zero-dependency, fast; same version as jr |
| github.com/pb33f/libopenapi | v0.34.3 | OpenAPI spec parsing (Phase 2 code-gen) | Already in go.mod from jr; needed for generator in Phase 2 — include now to match reference go.mod |

### Supporting (transitive, no direct use in Phase 1)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/bahlo/generic-list-go | v0.2.0 | libopenapi dep | Indirect only |
| github.com/buger/jsonparser | v1.1.1 | libopenapi dep | Indirect only |
| go.yaml.in/yaml/v4 | v4.0.0-rc.4 | libopenapi dep | Indirect only |
| golang.org/x/sync | v0.20.0 | libopenapi dep | Indirect only |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gojq (in-process) | exec("jq") subprocess | Subprocess requires jq installed, breaks in CI; gojq is pure Go and self-contained |
| File-system TTL cache | Redis/memcached | Overkill for CLI; file mtime-based TTL is zero-dependency and persists across invocations |
| tidwall/pretty | encoding/json indent | tidwall/pretty handles ANSI and is faster; matches jr exactly |

**Installation:**
```bash
go mod init github.com/sofq/confluence-cli
go get github.com/spf13/cobra@v1.10.2
go get github.com/spf13/pflag@v1.0.9
go get github.com/itchyny/gojq@v0.12.18
go get github.com/tidwall/pretty@v1.2.1
go get github.com/pb33f/libopenapi@v0.34.3
```

**Version verification:** Versions confirmed against `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/go.mod` — the exact go.mod that drives the reference build. These are production-verified versions, not speculative.

---

## Architecture Patterns

### Recommended Project Structure

```
confluence-cli/
├── main.go                    # os.Exit(cmd.Execute())
├── go.mod                     # module github.com/sofq/confluence-cli
├── go.sum
├── Makefile                   # generate, build, install, test, clean
├── spec/
│   └── confluence-v2.json     # pinned Confluence OpenAPI spec (Phase 2)
├── cmd/
│   ├── root.go                # rootCmd, PersistentPreRunE, Execute(), init()
│   ├── version.go             # versionCmd + Version ldflags variable
│   ├── configure.go           # configureCmd (no prompts, flag-driven)
│   ├── raw.go                 # rawCmd (cf raw GET /wiki/api/v2/pages)
│   ├── schema_cmd.go          # schemaCmd (cf schema)
│   └── generated/             # Phase 2: generated Cobra commands + schema_data.go
│       └── .gitkeep           # placeholder so directory is committed
├── internal/
│   ├── client/
│   │   └── client.go          # Client struct, Do(), Fetch(), WriteOutput(), cursor pagination
│   ├── config/
│   │   └── config.go          # Profile, AuthConfig, Resolve(), DefaultPath()
│   ├── errors/
│   │   └── errors.go          # APIError, exit codes, AlreadyWrittenError
│   ├── jq/
│   │   └── jq.go              # Apply(input, filter) gojq wrapper
│   └── cache/
│       └── cache.go           # Key(), Get(), Set() file-system TTL cache
└── gen/                       # Phase 2: code generator
    └── .gitkeep               # placeholder
```

### Pattern 1: PersistentPreRunE Client Injection

**What:** Build the `client.Client` once per invocation in `PersistentPreRunE`, inject into `cmd.Context()`. All RunE handlers pull the client via `client.FromContext(cmd.Context())`.

**When to use:** Every command that makes HTTP calls (all commands except `configure`, `version`, `completion`, `help`, `schema`).

**Example:**
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/root.go
var skipClientCommands = map[string]bool{
    "configure": true, "version": true, "completion": true,
    "help": true, "schema": true,
}

PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    if skipClientCommands[cmd.Name()] { return nil }
    // ... resolve config, build client ...
    cmd.SetContext(client.NewContext(cmd.Context(), c))
    return nil
},
```

### Pattern 2: AlreadyWrittenError Sentinel

**What:** When an error is written to stderr as structured JSON, return `&errors.AlreadyWrittenError{Code: exitCode}` instead of a raw `error`. The `Execute()` function checks for this type and returns the exit code directly without writing a second error.

**When to use:** Every place where `apiErr.WriteJSON(os.Stderr)` is called in RunE or PersistentPreRunE.

**Example:**
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/errors/errors.go
type AlreadyWrittenError struct{ Code int }
func (e *AlreadyWrittenError) Error() string { return "error already written" }

// In Execute():
if err := rootCmd.Execute(); err != nil {
    if aw, ok := err.(*jrerrors.AlreadyWrittenError); ok {
        return aw.Code
    }
    // fallback: encode unexpected error to stderr
    return jrerrors.ExitError
}
```

### Pattern 3: Do() vs Fetch()

**What:** Two HTTP execution paths on the Client:
- `Do(ctx, method, path, query, body) int` — executes request, writes output to Stdout, returns exit code. Used by generated commands and `raw`.
- `Fetch(ctx, method, path, body) ([]byte, int)` — executes request, returns raw bytes. Used by workflow commands that need to read/transform the response before writing.

**When to use:**
- `Do()`: any command that just passes the API response through (generated commands, `raw`)
- `Fetch()`: workflow commands like `page update` that need to read current state before writing (Phase 3+)

### Pattern 4: Cursor-Based Pagination (Confluence-specific)

**What:** Confluence v2 REST API uses cursor-based pagination via `_links.next` in the response envelope. Unlike Jira's `startAt`/`total`/`values` pattern, Confluence returns:
```json
{
  "results": [...],
  "_links": {
    "next": "/wiki/api/v2/pages?cursor=<opaque_cursor>&limit=25"
  }
}
```

**When to use:** All list endpoints in Confluence v2. The `doWithPagination()` dispatch in the client needs a new `doCursorPagination()` handler that:
1. Detects `_links.next` in the response
2. Fetches the next page URL verbatim (cursor is already embedded)
3. Accumulates `results` arrays
4. Writes a merged envelope with all results

**Example (new pattern, not in jr reference):**
```go
// Source: Confluence v2 API spec - cursor pagination pattern
type cursorPage struct {
    Results []json.RawMessage `json:"results"`
    Links   struct {
        Next string `json:"next"`
    } `json:"_links"`
}

func detectCursorPagination(body []byte) bool {
    var probe map[string]json.RawMessage
    if err := json.Unmarshal(body, &probe); err != nil { return false }
    _, hasResults := probe["results"]
    _, hasLinks := probe["_links"]
    return hasResults && hasLinks
}
```

### Pattern 5: Config Resolution Priority

**What:** Four-level merge: CLI flags > environment variables > config file profile > defaults.

**Mapping for cf (changed from jr):**
```
JR_CONFIG_PATH  → CF_CONFIG_PATH
JR_BASE_URL     → CF_BASE_URL
JR_AUTH_TYPE    → CF_AUTH_TYPE
JR_AUTH_USER    → CF_AUTH_USER
JR_AUTH_TOKEN   → CF_AUTH_TOKEN
JR_PROFILE      → CF_PROFILE  (new — add this, not in jr)
```

**Config file location:**
- macOS: `~/Library/Application Support/cf/config.json`
- Linux: `~/.config/cf/config.json`
- Windows: `%APPDATA%/cf/config.json`

**Cache directory:** `os.UserCacheDir()/cf` (replaces `jr` subdirectory)

### Pattern 6: JSON-Only stdout Contract

**What:** stdout must contain only valid JSON at all times. Help text, warnings, and errors go to stderr.

**Implementation points:**
```go
// --version flag: JSON template
rootCmd.SetVersionTemplate(`{"version":"{{.Version}}"}` + "\n")

// Help function: redirect to stderr for subcommands, JSON hint for root
rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
    if cmd == rootCmd {
        // write JSON hint to stdout
        return
    }
    cmd.SetOut(os.Stderr) // send help text to stderr
    defaultHelp(cmd, args)
})

// HTTP 204: emit {} to maintain contract
if len(respBody) == 0 || resp.StatusCode == http.StatusNoContent {
    respBody = []byte("{}")
}
```

### Anti-Patterns to Avoid

- **fmt.Println() or log.Print() in RunE:** Breaks JSON-only stdout contract. Use `apiErr.WriteJSON(os.Stderr)` for errors, `c.WriteOutput()` for success.
- **Returning raw errors from RunE after writing to stderr:** Causes double error output. Always return `&AlreadyWrittenError{Code: code}` after writing structured JSON error.
- **Reading stdin implicitly for body:** Hangs agent invocations. The `raw` command requires explicit `--body -` to read stdin.
- **Relative URLs in BaseURL:** `config.Resolve()` must `strings.TrimRight(baseURL, "/")` to prevent double-slash issues.
- **Paginating non-GET requests:** Pagination is GET-only. Check `method == "GET"` before entering pagination path.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JQ filtering | Custom JSON path syntax | gojq (`internal/jq`) | JQ has 50+ operators; custom parsers miss edge cases; gojq is the pure-Go standard |
| JSON pretty-printing | manual indentation | tidwall/pretty | Handles edge cases like trailing commas, ANSI; matches jr exactly |
| HTTP caching | Redis, in-memory map | File-system TTL cache (`internal/cache`) | CLI invocations are short-lived; file mtime is zero-dependency and cross-invocation persistent |
| Flag parsing | custom arg parser | cobra + pflag | `f.Changed` flag is critical for "was this flag set by user?" logic in pagination, config merge |
| Structured errors | fmt.Errorf | `internal/errors.APIError` | Agents parse `error_type` field; bare strings break agent error handling |
| Config merging | single config file | 4-level resolution in `config.Resolve()` | CI uses env vars, interactive uses flags, profiles handle multi-instance — all three are needed |

**Key insight:** The reference implementation already solved every infrastructure problem correctly. The only genuine new work in Phase 1 is (1) mechanical renaming and (2) implementing cursor-based pagination instead of startAt pagination.

---

## Common Pitfalls

### Pitfall 1: Cursor Pagination URL Construction

**What goes wrong:** Confluence's `_links.next` contains a full path relative to the domain (e.g., `/wiki/api/v2/pages?cursor=abc&limit=25`). Naively appending it to BaseURL results in duplicate path segments.

**Why it happens:** Jira's pagination constructs URLs by adding `startAt` as a query param to the original path. Confluence's cursor is already a complete path, not a delta.

**How to avoid:** When following `_links.next`, use the path verbatim — strip the domain prefix if present, do not concatenate the original path. The cursor replaces the path, not adds to it.

**Warning signs:** `404` errors on paginated calls; double `/wiki/wiki/` in URL logs with `--verbose`.

### Pitfall 2: HTML Error Pages from Confluence

**What goes wrong:** Confluence (like Jira) returns HTML error pages (Atlassian login page, 403 Forbidden) when auth fails or session expires. Passing raw HTML as the `message` field in APIError produces unreadable output.

**Why it happens:** Atlassian CDN returns HTML before the API layer gets to process auth.

**How to avoid:** The `sanitizeBody()` function in `internal/errors` already detects HTML prefixes (`<!`, `<html`, `<head`, `<body`) and replaces with `HTTP {status}: server returned HTML error page`. Copy this exactly.

**Warning signs:** `message` fields containing `<!DOCTYPE html>` in error output.

### Pitfall 3: Double-Writing Errors

**What goes wrong:** An error is written to stderr by `apiErr.WriteJSON(os.Stderr)`, then Cobra also writes the error returned from `RunE`, producing two JSON objects to stderr.

**Why it happens:** Cobra writes any non-nil error from `RunE` to its configured error output.

**How to avoid:** Always use `SilenceErrors: true` on rootCmd AND return `&AlreadyWrittenError{Code: code}` (not the APIError itself) from RunE after writing to stderr.

**Warning signs:** Two JSON objects on stderr in test output; `{"error_type":...}{"Use":...}` pattern.

### Pitfall 4: Profile Not Found vs Unconfigured

**What goes wrong:** When no config file exists yet, `config.Resolve()` returns empty BaseURL. This should trigger "run cf configure" error, not "profile not found" error.

**Why it happens:** `LoadFrom()` returns empty Config (not error) when file doesn't exist — this is correct behavior. But the caller must check `resolved.BaseURL == ""` explicitly.

**How to avoid:** After `config.Resolve()`, check `if resolved.BaseURL == ""` and emit a descriptive error pointing to `cf configure --base-url <url> --token <token>`.

### Pitfall 5: Go Module Path Mismatch

**What goes wrong:** If `go.mod` module path doesn't match import paths used in the package, `go build` fails immediately.

**Why it happens:** Copy-paste from `github.com/sofq/jira-cli` without updating all import references.

**How to avoid:** Module path must be `github.com/sofq/confluence-cli`. Verify with `grep -r "jira-cli" . --include="*.go"` after copying internal packages.

---

## Code Examples

Verified patterns from the reference implementation:

### Exit Code Constants (internal/errors/errors.go)
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/errors/errors.go
const (
    ExitOK         = 0
    ExitError      = 1
    ExitAuth       = 2
    ExitNotFound   = 3
    ExitValidation = 4
    ExitRateLimit  = 5
    ExitConflict   = 6
    ExitServer     = 7
)
```

### Config DefaultPath() with CF_ prefix
```go
// Source: adapted from /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/config/config.go
func DefaultPath() string {
    if v := os.Getenv("CF_CONFIG_PATH"); v != "" {
        return v
    }
    switch runtime.GOOS {
    case "darwin":
        home, _ := os.UserHomeDir()
        return filepath.Join(home, "Library", "Application Support", "cf", "config.json")
    default: // linux
        home, _ := os.UserHomeDir()
        return filepath.Join(home, ".config", "cf", "config.json")
    }
}
```

### Cache key generation (internal/cache/cache.go)
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/internal/cache/cache.go
func Key(method, url string, authContext ...string) string {
    input := method + " " + url
    for _, ctx := range authContext {
        input += "\x00" + ctx
    }
    h := sha256.Sum256([]byte(input))
    return hex.EncodeToString(h[:])
}

func Dir() string {
    cacheDirOnce.Do(func() {
        dir, _ := os.UserCacheDir()
        cacheDir = filepath.Join(dir, "cf") // "cf" not "jr"
        _ = os.MkdirAll(cacheDir, 0o700)
    })
    return cacheDir
}
```

### Cursor Pagination Detection (new, Confluence-specific)
```go
// Source: Confluence v2 API response structure (verified from spec)
type cursorPage struct {
    Results []json.RawMessage `json:"results"`
    Links   struct {
        Next string `json:"next"`
    } `json:"_links"`
}

func detectCursorPagination(body []byte) bool {
    var probe struct {
        Results json.RawMessage `json:"results"`
        Links   json.RawMessage `json:"_links"`
    }
    if err := json.Unmarshal(body, &probe); err != nil {
        return false
    }
    return probe.Results != nil && probe.Links != nil
}
```

### Version JSON Output
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/root.go
// --version flag: JSON via SetVersionTemplate
rootCmd.SetVersionTemplate(`{"version":"{{.Version}}"}` + "\n")

// cf version subcommand: via marshalNoEscape
var Version = "dev" // set at build time: -X github.com/sofq/confluence-cli/cmd.Version=$(VERSION)
```

### Confluence v2 Connection Test Endpoint
```go
// For cf configure --test, use Confluence v2 "current user" endpoint
// instead of Jira's /rest/api/3/myself
testURL := baseURL + "/wiki/api/v2/spaces?limit=1"
// OR use Confluence user endpoint if available:
testURL := baseURL + "/wiki/rest/api/user/current"
```

### schema Command Stub for Phase 1
```go
// cmd/generated must exist as a package with empty RegisterAll and AllSchemaOps
// so that root.go compiles before Phase 2 code-gen is implemented
package generated

func RegisterAll(root interface{}) {} // stub — Phase 2 fills this in

func AllSchemaOps() []SchemaOp { return nil }
func AllResources() []string { return nil }
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| jq subprocess (`exec("jq", ...)`) | gojq in-process | ~2019 (gojq released) | No external binary dependency; works in minimal containers |
| Offset pagination (startAt) | Cursor pagination (`_links.next`) | Confluence v2 API design | Cursor is opaque; cannot seek to arbitrary page; must follow links sequentially |
| Interactive config prompts | Flag-driven configure command | jr design decision | Agent-friendly; no stdin hang; fully scriptable |
| Cobra default help to stdout | Help redirected to stderr | jr design decision | Preserves JSON-only stdout contract |

**Deprecated/outdated:**
- Confluence v1 REST API (`/wiki/rest/api`): Still functional but deprecated; v2 (`/wiki/api/v2`) is the target. The `raw` command covers any one-off v1 calls.
- OAuth2 browser flow: v2 scope, not Phase 1. Phase 1 supports basic (email+token) and bearer only.

---

## Open Questions

1. **Confluence connection test endpoint**
   - What we know: Jira uses `GET /rest/api/3/myself`. Confluence v2 has `GET /wiki/api/v2/spaces?limit=1` and `GET /wiki/rest/api/user/current`.
   - What's unclear: Which endpoint is guaranteed accessible with minimal permissions across all Confluence Cloud plans.
   - Recommendation: Use `GET /wiki/api/v2/spaces?limit=1` — it's a v2 endpoint and a 200 response confirms auth and base URL.

2. **Confluence cursor format in `_links.next`**
   - What we know: Confluence v2 returns `_links.next` as a full path string like `/wiki/api/v2/pages?cursor=<base64>&limit=25`.
   - What's unclear: Whether cursor paths always include `/wiki` prefix or are relative to the API base.
   - Recommendation: When following `_links.next`, extract the path+query portion and append directly to the configured BaseURL. Do not re-add `/wiki/api/v2`.

3. **`CF_PROFILE` env var (not in jr)**
   - What we know: jr uses `--profile` flag only; no `JR_PROFILE` env var.
   - What's unclear: Whether INFRA-04 requires env var support for profile selection.
   - Recommendation: INFRA-04 explicitly states `CF_PROFILE` env var — add this to `config.Resolve()`. Read `os.Getenv("CF_PROFILE")` and apply it before the flag override.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (no external framework) |
| Config file | None — `go test ./...` discovers tests automatically |
| Quick run command | `go test ./internal/... ./cmd/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | JSON-only stdout contract | unit | `go test ./cmd/... -run TestRoot -count=1` | Wave 0 |
| INFRA-02 | Structured JSON errors with exit codes | unit | `go test ./internal/errors/... -count=1` | Wave 0 |
| INFRA-03 | `cf configure` saves profile to JSON | unit | `go test ./cmd/... -run TestConfigure -count=1` | Wave 0 |
| INFRA-04 | Profile selection via flag + env var | unit | `go test ./internal/config/... -count=1` | Wave 0 |
| INFRA-05 | Basic + bearer auth headers applied | unit | `go test ./internal/client/... -run TestApplyAuth -count=1` | Wave 0 |
| INFRA-06 | JQ filter transforms output | unit | `go test ./internal/jq/... -count=1` | Wave 0 |
| INFRA-07 | Cursor pagination merges all pages | unit | `go test ./internal/client/... -run TestPagination -count=1` | Wave 0 |
| INFRA-08 | Cache stores and TTL-expires GET responses | unit | `go test ./internal/cache/... -count=1` | Wave 0 |
| INFRA-09 | `cf raw GET /path` makes HTTP call | unit | `go test ./cmd/... -run TestRaw -count=1` | Wave 0 |
| INFRA-10 | `--dry-run` prints request, no HTTP call | unit | `go test ./internal/client/... -run TestDryRun -count=1` | Wave 0 |
| INFRA-11 | `--verbose` logs to stderr only | unit | `go test ./internal/client/... -run TestVerbose -count=1` | Wave 0 |
| INFRA-12 | `--version` / `cf version` outputs JSON | unit | `go test ./cmd/... -run TestVersion -count=1` | Wave 0 |
| INFRA-13 | `cf schema` returns JSON operation list | unit | `go test ./cmd/... -run TestSchema -count=1` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./... -count=1`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

All test files are new — no existing test infrastructure:

- [ ] `internal/errors/errors_test.go` — covers INFRA-02
- [ ] `internal/config/config_test.go` — covers INFRA-03, INFRA-04
- [ ] `internal/client/client_test.go` — covers INFRA-05, INFRA-07, INFRA-08, INFRA-10, INFRA-11
- [ ] `internal/jq/jq_test.go` — covers INFRA-06
- [ ] `internal/cache/cache_test.go` — covers INFRA-08
- [ ] `cmd/root_test.go` — covers INFRA-01, INFRA-12
- [ ] `cmd/raw_test.go` — covers INFRA-09
- [ ] `cmd/schema_cmd_test.go` — covers INFRA-13
- [ ] `cmd/configure_test.go` — covers INFRA-03

Note: Reference tests at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/*_test.go` and `internal/*/` can serve as starting templates; adapt for `cf` module path and Confluence-specific behavior.

---

## Sources

### Primary (HIGH confidence)

- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/` — full reference implementation read directly; all patterns verified against live source code
  - `main.go`, `cmd/root.go`, `cmd/configure.go`, `cmd/raw.go`, `cmd/schema_cmd.go`, `cmd/version.go`
  - `internal/client/client.go` (685 LOC — complete)
  - `internal/config/config.go`, `internal/errors/errors.go`, `internal/jq/jq.go`, `internal/cache/cache.go`
  - `go.mod` — exact dependency versions
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/.planning/phases/01-core-scaffolding/01-CONTEXT.md` — phase constraints and code context
- `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/.planning/REQUIREMENTS.md` — all 13 INFRA requirements

### Secondary (MEDIUM confidence)

- Confluence v2 API cursor pagination pattern: `_links.next` structure inferred from Confluence API documentation knowledge and requirement INFRA-07 stating "cursor-based" — flagged in Open Questions for validation during implementation.

### Tertiary (LOW confidence)

- Confluence connection test endpoint selection (`/wiki/api/v2/spaces?limit=1`): Reasonable choice but not verified against live Confluence Cloud instance. Verify during `cf configure --test` implementation.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions read directly from reference go.mod
- Architecture: HIGH — entire architecture read from reference source code
- Cursor pagination: MEDIUM — pattern inferred from requirement spec + Confluence API knowledge; exact `_links` structure should be validated against spec/live API in implementation
- Pitfalls: HIGH — sourced from reference implementation patterns and Go CLI conventions

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (stable ecosystem; Go/Cobra versions unlikely to change)
