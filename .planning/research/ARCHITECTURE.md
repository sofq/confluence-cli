# Architecture Research

**Domain:** CLI tool — REST API client with code generation (Go + Cobra)
**Researched:** 2026-03-20
**Confidence:** HIGH (derived directly from the reference implementation at `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2`)

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Entry Point                                 │
│                        main.go → cmd.Execute()                       │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│                          CMD Layer (cmd/)                            │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  ┌──────────┐  │
│  │  root.go     │  │  workflow.go │  │   raw.go    │  │configure │  │
│  │ (PersistentP │  │ (hand-written│  │ (raw API    │  │  .go     │  │
│  │  reRunE,     │  │  wrappers)   │  │  passthru)  │  │          │  │
│  │  client inj) │  │              │  │             │  │          │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬──────┘  └────┬─────┘  │
│         │                 │                  │              │         │
│  ┌──────▼─────────────────▼──────────────────▼──────────────▼──────┐ │
│  │                   cmd/generated/  (DO NOT EDIT)                  │ │
│  │           One .go file per OpenAPI tag/resource group            │ │
│  │    init.go (RegisterAll) + <resource>.go (Cobra commands)        │ │
│  └─────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────┬───────────────────────────────────────┘
                              │ context.Value(clientKey)
┌─────────────────────────────▼───────────────────────────────────────┐
│                       Internal Packages (internal/)                  │
├────────────┬────────────┬────────────┬────────────┬─────────────────┤
│  client/   │  config/   │  errors/   │  cache/    │  jq/            │
│  Client    │  Profile   │  APIError  │  file-based│  gojq wrapper   │
│  .Do()     │  Resolve() │  exit codes│  SHA256 key│                 │
│  .Fetch()  │  LoadFrom()│  semantic  │            │                 │
│  .Write    │  SaveTo()  │  exit map  │            │                 │
│  Output()  │            │            │            │                 │
├────────────┴────────────┴────────────┴────────────┴─────────────────┤
│  policy/   │  audit/    │  preset/   │  (cf-new: storage format)    │
│  Op allow/ │  JSONL log │  named     │                              │
│  deny list │  entries   │  --fields  │                              │
│            │            │  + --jq    │                              │
└────────────┴────────────┴────────────┴──────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│                      Code Generator (gen/)                           │
├─────────────────────────────────────────────────────────────────────┤
│  main.go → parser.go → grouper.go → generator.go                    │
│  Reads: spec/confluence-v2.json (OpenAPI 3.0)                       │
│  Writes: cmd/generated/*.go                                          │
│  Templates: gen/templates/{resource.go.tmpl, init.go.tmpl, ...}     │
└─────────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│                         External                                     │
│          Confluence Cloud v2 REST API (HTTPS)                        │
│          ~/.config/cf/config.json  |  ~/.cache/cf/ (response cache) │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `main.go` | Binary entry point, delegates to `cmd.Execute()`, translates return code to `os.Exit` | `cmd/` |
| `cmd/root.go` | Root Cobra command; `PersistentPreRunE` builds and injects `client.Client` into context | `internal/config`, `internal/client`, `internal/policy`, `internal/audit`, `internal/preset`, `cmd/generated` |
| `cmd/generated/` | Auto-generated Cobra resource/verb commands; one file per OpenAPI tag; `RegisterAll` mounts all to root | `internal/client` (via context) |
| `cmd/workflow.go` | Hand-written high-level commands (`page create`, `page update`, `search`, etc.) that orchestrate multiple API calls | `internal/client`, `internal/errors` |
| `cmd/raw.go` | Passthrough command for any unmapped endpoint: `cf raw GET /wiki/api/v2/...` | `internal/client` |
| `cmd/configure.go` | Reads/writes `~/.config/cf/config.json`; no client required | `internal/config` |
| `internal/client` | Core HTTP client: auth, pagination, dry-run, jq, cache, audit, output | `internal/cache`, `internal/jq`, `internal/errors`, `internal/audit`, `internal/policy` |
| `internal/config` | Load/save/resolve config profiles; env var override; `CF_` prefix | filesystem |
| `internal/errors` | `APIError` struct, `AlreadyWrittenError` sentinel, semantic exit codes (0-7) | `os.Stderr` (writes JSON) |
| `internal/cache` | File-based response cache in `~/.cache/cf/`; SHA256 keyed; TTL-based invalidation | filesystem |
| `internal/jq` | Thin wrapper around `gojq`; applies `--jq` filter to raw JSON bytes | — |
| `internal/policy` | Allowlist/denylist of `"resource verb"` operations; per-profile config | — |
| `internal/audit` | JSONL append-only audit log; one entry per API call | filesystem |
| `internal/preset` | Named output presets expanding to `--fields` + `--jq` defaults | filesystem |
| `gen/` | Standalone Go program (`go run ./gen/...`); reads OpenAPI spec, writes `cmd/generated/` | `spec/confluence-v2.json`, `gen/templates/`, `cmd/generated/` |
| `spec/` | Vendored copy of the Confluence Cloud v2 OpenAPI JSON spec | — |

## Recommended Project Structure

```
confluence-cli/
├── main.go                     # os.Exit(cmd.Execute())
├── go.mod                      # module github.com/<org>/confluence-cli
├── go.sum
├── Makefile                    # generate, build, install, test, spec-update, clean
├── spec/
│   └── confluence-v2.json      # vendored Confluence OpenAPI 3.0 spec
├── gen/                        # code generator (go run ./gen/...)
│   ├── main.go                 # entry: parse spec, call generator
│   ├── parser.go               # OpenAPI → internal operation structs
│   ├── grouper.go              # group operations by tag/resource
│   ├── generator.go            # render templates → cmd/generated/*.go
│   └── templates/
│       ├── resource.go.tmpl    # one Cobra resource+verb command file
│       ├── init.go.tmpl        # RegisterAll() aggregator
│       └── schema_data.go.tmpl # optional: schema introspection data
├── cmd/
│   ├── root.go                 # rootCmd, PersistentPreRunE, Execute()
│   ├── configure.go            # cf configure --base-url ... --token ...
│   ├── raw.go                  # cf raw GET /wiki/api/v2/...
│   ├── workflow.go             # hand-written: page create, page update, search, etc.
│   ├── version.go              # Version var (set by ldflags)
│   └── generated/              # DO NOT EDIT — output of gen/
│       ├── init.go             # RegisterAll(root)
│       ├── page.go             # page resource commands
│       ├── space.go            # space resource commands
│       ├── comment.go          # comment resource commands
│       └── ...                 # one file per OpenAPI tag
├── internal/
│   ├── client/
│   │   └── client.go           # Client struct, Do(), Fetch(), WriteOutput()
│   ├── config/
│   │   └── config.go           # Profile, Config, Resolve(), LoadFrom(), SaveTo()
│   ├── errors/
│   │   └── errors.go           # APIError, AlreadyWrittenError, exit codes
│   ├── cache/
│   │   └── cache.go            # file-based TTL cache in ~/.cache/cf/
│   ├── jq/
│   │   └── jq.go               # Apply(data, filter) via gojq
│   ├── policy/
│   │   └── policy.go           # op allow/deny list, Check(operation)
│   ├── audit/
│   │   └── audit.go            # JSONL append logger
│   └── preset/
│       └── preset.go           # named presets: fields + jq defaults
└── docs/                       # optional VitePress docs site
```

### Structure Rationale

- **`gen/` is a standalone program:** Run with `go run ./gen/...`; never imported by the CLI. This keeps the generator's OpenAPI parser dependencies (e.g. `pb33f/libopenapi`) completely out of the CLI binary.
- **`cmd/generated/` is committed but marked DO NOT EDIT:** Generated files are checked in so the binary can be built without running the generator in CI. Regenerate locally with `make generate` after spec changes.
- **`internal/` packages are unexported:** No public API surface. This enforces the boundary: only `cmd/` and `gen/` consume `internal/`.
- **`cmd/workflow.go` alongside generated code:** Hand-written wrappers live next to generated commands in `cmd/`. They merge into the command tree via `mergeCommand()` — the generated parent command's subcommands are preserved; the hand-written parent replaces it with richer help text.
- **`spec/` vendored:** The spec is committed to the repo so the build is reproducible. Update with `make spec-update` (curl from Atlassian's spec CDN).

## Architectural Patterns

### Pattern 1: Client Injection via Cobra Context

**What:** `PersistentPreRunE` on the root command resolves config, builds a `client.Client`, and stores it in the Cobra command context via `client.NewContext`. All `RunE` functions retrieve it with `client.FromContext(cmd.Context())`.

**When to use:** Every generated and hand-written command that calls the Confluence API.

**Trade-offs:** Clean separation between setup (root) and execution (leaf commands). Avoids global variables. Makes commands testable — inject a fake client via context in tests.

**Example:**
```go
// In PersistentPreRunE (root.go):
c := &client.Client{BaseURL: resolved.BaseURL, Auth: resolved.Auth, ...}
cmd.SetContext(client.NewContext(cmd.Context(), c))

// In any RunE (generated or hand-written):
c, err := client.FromContext(cmd.Context())
if err != nil { /* write structured error, return AlreadyWrittenError */ }
code := c.Do(cmd.Context(), "GET", "/wiki/api/v2/pages", query, nil)
```

### Pattern 2: Two-Method Client API (Do vs Fetch)

**What:** `client.Do()` executes a request AND writes the result to stdout (handles pagination, jq, pretty-print). `client.Fetch()` executes a request and returns the raw body to the caller — no output. Hand-written workflow commands use `Fetch()` for intermediate API calls (e.g. resolving a space ID before creating a page) and `WriteOutput()` at the end.

**When to use:** Generated commands always use `Do()`. Workflow commands use `Fetch()` for multi-step operations, then call `c.WriteOutput(finalBody)` once.

**Trade-offs:** Prevents double-writing output in multi-step flows. The caller owns the output contract for complex operations.

### Pattern 3: AlreadyWrittenError Sentinel

**What:** When a command writes a JSON error to stderr and wants to return a non-zero exit code, it returns `&errors.AlreadyWrittenError{Code: n}`. The root `Execute()` function checks for this type and suppresses a second error write. All other errors get JSON-encoded to stderr by Execute.

**When to use:** Everywhere a `RunE` calls an API or detects an error after writing to stderr. This is the only correct way to signal failure from a command.

**Trade-offs:** Slightly unusual pattern but solves the "double error" problem cleanly. Required by the JSON-only stdout contract.

### Pattern 4: mergeCommand for Generated+Hand-written Coexistence

**What:** `generated.RegisterAll(root)` mounts all generated resource commands. Then `mergeCommand(root, workflowCmd)` finds the generated parent (e.g. `page`) on root, copies its generated subcommands onto the hand-written parent, removes the generated parent, and adds the hand-written parent. Result: the hand-written parent has both its own subcommands and all generated subcommands.

**When to use:** Any resource where hand-written workflow commands should coexist with generated CRUD commands under the same parent name.

**Trade-offs:** Requires care when adding new hand-written commands — name collisions with generated subcommands are silently skipped (hand-written wins). Always check with `cf schema <resource>` after merging.

### Pattern 5: Config Resolution Hierarchy

**What:** Config is resolved in strict priority order: (1) config file profile, (2) `CF_*` environment variables, (3) CLI flags. Flags beat env vars beat config file.

**When to use:** All config resolution goes through `config.Resolve()`. Never read env vars or flags directly in command code.

**Trade-offs:** Predictable, testable, documented. The `FlagOverrides` struct is the clean interface between Cobra flag parsing and config resolution.

## Data Flow

### Request Flow (Generated Command)

```
User: cf page get --page-id 123 --jq '.title'
       │
       ▼
main.go: cmd.Execute()
       │
       ▼
root PersistentPreRunE:
  config.Resolve(path, profile, flags) → ResolvedConfig
  build client.Client{...}
  cmd.SetContext(client.NewContext(ctx, c))
       │
       ▼
generated/page.go RunE:
  c = client.FromContext(ctx)
  path = fmt.Sprintf("/wiki/api/v2/pages/%s", url.PathEscape(pageId))
  query = client.QueryFromFlags(cmd, "body-format", "get-draft", ...)
  code = c.Do(ctx, "GET", path, query, nil)
       │
       ▼
client.Do():
  → DryRun? emit JSON, return
  → GET? → doWithPagination()
       │
       ▼
client.doOnce():
  cache.Get(key, ttl) → hit? WriteOutput, return
  http.NewRequestWithContext(ctx, "GET", url, nil)
  c.ApplyAuth(req)   → Basic / Bearer / OAuth2
  HTTPClient.Do(req)
  status >= 400? → errors.NewFromHTTP → WriteJSON(stderr) → return ExitCode
  cache.Set(key, body)
  c.WriteOutput(body)
       │
       ▼
client.WriteOutput():
  jq.Apply(body, "--jq filter")   ← gojq
  pretty.Pretty(body)             ← if --pretty
  fmt.Fprintf(Stdout, "%s\n", data)
  return ExitOK
       │
       ▼
os.Exit(0)
```

### Request Flow (Workflow Command — multi-step)

```
User: cf workflow page-update --page-id 123 --title "New Title" --body "..."
       │
       ▼
[root PersistentPreRunE — same as above, client injected]
       │
       ▼
cmd/workflow.go RunE:
  c = client.FromContext(ctx)

  Step 1: c.Fetch(ctx, "GET", "/wiki/api/v2/pages/123", nil)
          → returns (currentPageJSON, ExitOK)

  Step 2: extract current version from currentPageJSON

  Step 3: c.Fetch(ctx, "PUT", "/wiki/api/v2/pages/123", updateBody)
          → returns (updatedPageJSON, ExitOK)

  c.WriteOutput(updatedPageJSON)   ← single write to stdout
  return nil
```

### Config Resolution Flow

```
CLI flags (--profile, --base-url, --auth-type, --auth-token)
       ↓ (lowest priority — overridden by flags)
~/.config/cf/config.json  →  config.LoadFrom()
       ↓
CF_BASE_URL, CF_AUTH_TYPE, CF_AUTH_USER, CF_AUTH_TOKEN  (env vars)
       ↓ (highest priority)
CLI flag values (non-empty strings win)
       ↓
config.ResolvedConfig{BaseURL, Auth, ProfileName, ...}
       ↓
client.Client{...}  →  context
```

### Key Data Flows

1. **Spec to commands:** `spec/confluence-v2.json` → `go run ./gen/...` → `cmd/generated/*.go` → committed. Re-run after spec changes.
2. **API response to caller:** Confluence API → `client.doOnce` → optional cache write → `WriteOutput` → optional jq → optional pretty → stdout.
3. **Error to caller:** Any failure → `errors.APIError.WriteJSON(stderr)` → `AlreadyWrittenError{Code}` → `Execute()` returns code → `os.Exit(code)`.
4. **Config to client:** File + env vars + flags → `config.Resolve()` → `client.Client` → `context.Context` → every `RunE`.

## Suggested Build Order

Components have hard dependencies that dictate implementation order:

1. **`internal/errors`** — no dependencies; needed by everything else.
2. **`internal/config`** — no dependencies; needed by `root.go` and `configure.go`.
3. **`internal/cache`** — no dependencies; needed by `internal/client`.
4. **`internal/jq`** — depends on `gojq` only; needed by `internal/client`.
5. **`internal/policy`**, **`internal/audit`**, **`internal/preset`** — no internal dependencies; needed by `internal/client` / `root.go`.
6. **`internal/client`** — depends on all internal packages above; needed by all commands.
7. **`cmd/configure.go`** and **`cmd/root.go`** skeleton — can wire up configure + version before generated commands exist.
8. **`gen/` code generator** — depends on spec, produces `cmd/generated/`.
9. **`cmd/generated/`** — output of generator; wire into `root.go` via `generated.RegisterAll`.
10. **`cmd/raw.go`** — depends on client; standalone command.
11. **`cmd/workflow.go`** — depends on client; hand-written multi-step commands for pages, search, labels, comments.

This ordering means phases can be: (1) scaffold + core plumbing, (2) code generation pipeline, (3) hand-written workflow wrappers.

## Anti-Patterns

### Anti-Pattern 1: Global Variables for Client State

**What people do:** Store auth credentials, base URL, or the HTTP client as package-level `var` globals.
**Why it's wrong:** Makes commands impossible to unit-test without real credentials. Causes subtle state leakage between invocations in test suites.
**Do this instead:** Pass the client through `cmd.Context()` via `client.NewContext` / `client.FromContext`. All commands receive the client through context, exactly as the reference implementation does.

### Anti-Pattern 2: Writing to stdout from Multiple Places

**What people do:** Have both the `Do()` call and the workflow handler write to stdout — once the API response, once a status message.
**Why it's wrong:** Breaks the JSON-only stdout contract. Callers (AI agents) cannot parse two concatenated JSON objects.
**Do this instead:** Workflow commands use `Fetch()` for intermediate calls (no output) and call `c.WriteOutput()` exactly once at the end with the final result or a status envelope like `{"status":"updated","id":"123"}`.

### Anti-Pattern 3: Writing Generated Code Manually

**What people do:** Hand-edit files in `cmd/generated/` to add extra flags or change behavior for one endpoint.
**Why it's wrong:** Running `make generate` overwrites all changes. The `// Code generated by cf gen. DO NOT EDIT.` header is the warning.
**Do this instead:** If a generated command needs customization, use `mergeCommand` to replace its parent with a hand-written version that provides a custom subcommand. The generator preserves subcommands not already present on the hand-written parent.

### Anti-Pattern 4: Silent Config Fallback

**What people do:** When config is missing or invalid, fall back to empty strings and proceed — letting the HTTP request fail with a confusing error.
**Why it's wrong:** The error message from the API (`401 Unauthorized`) is less useful than "base_url is not set; run `cf configure`".
**Do this instead:** Check `resolved.BaseURL == ""` in `PersistentPreRunE` before building the client, and emit a structured `config_error` with an actionable hint, exactly as `jr` does.

### Anti-Pattern 5: Markdown Conversion for Content

**What people do:** Accept Markdown as page body input and convert it to Confluence Storage Format before sending.
**Why it's wrong:** Confluence Storage Format (XHTML-based ASF) is not a lossless Markdown target. The conversion is complex, lossy, and adds a large dependency. Agents can produce Storage Format directly.
**Do this instead:** Accept raw Storage Format body strings (or files via `--body @file`). Document that `--body` accepts ASF XML. This is already in the project's out-of-scope decisions.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Confluence Cloud v2 REST API | HTTPS with basic/bearer/OAuth2 auth | Base URL format: `https://<domain>.atlassian.net/wiki` — the client appends paths like `/wiki/api/v2/pages` |
| Confluence OpenAPI spec CDN | `curl` during `make spec-update` only | `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `gen/` → `cmd/generated/` | File I/O (write .go files) | Generator never imported at runtime; build-time only |
| `cmd/` → `internal/client` | Direct Go function calls + context | Client struct passed via `context.Value` |
| `cmd/` → `internal/config` | Direct Go function calls | Only `root.go` and `configure.go` touch config |
| `internal/client` → `internal/cache` | Direct Go function calls | Cache is transparent to callers of `Do()` |
| `internal/client` → Confluence API | `net/http` | All HTTP goes through `client.Client.HTTPClient` |

## Sources

- Direct code analysis of `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2` (the reference implementation)
- Files examined: `main.go`, `cmd/root.go`, `cmd/workflow.go`, `cmd/raw.go`, `cmd/configure.go`, `cmd/generated/init.go`, `internal/client/client.go`, `internal/config/config.go`, `internal/errors/errors.go`, `internal/cache/cache.go`, `gen/generator.go`, `gen/templates/resource.go.tmpl`, `go.mod`, `Makefile`
- Confluence Cloud v2 OpenAPI spec reference: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`

---
*Architecture research for: Confluence CLI (`cf`) — Go + Cobra + OpenAPI code generation*
*Researched: 2026-03-20*
