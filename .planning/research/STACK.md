# Stack Research

**Domain:** Confluence Cloud CLI (Go)
**Researched:** 2026-03-20
**Confidence:** HIGH — all libraries are direct copies from the reference `jr` implementation with versions verified against pkg.go.dev and official release pages.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.26.1 | Implementation language | Constraint from PROJECT.md. Single-binary distribution, cross-compile, no runtime dependencies. Go 1.26 (Feb 2026) is the current stable release; the reference `jr` uses 1.25.8, so 1.26.x is a safe upgrade. |
| github.com/spf13/cobra | v1.10.2 | CLI framework — command tree, flags, completions | Constraint from PROJECT.md. Industry standard for Go CLIs (used by kubectl, Hugo, GitHub CLI). Provides persistent flags, subcommand routing, and shell completion generation. Latest stable release (Dec 2024). |
| github.com/spf13/pflag | v1.0.10 | POSIX-style flag parsing | Cobra's underlying flag library. Required transitively; used directly in `cmd/` for declaring persistent flags. Upgraded to v1.0.10 (Sep 2025) from v1.0.9 in `jr`. |
| github.com/pb33f/libopenapi | v0.34.3 | OpenAPI 3.x spec parsing for code generation | Used in the `gen/` binary to parse the Confluence v2 OpenAPI spec and walk paths/operations. Chosen over `kin-openapi` because it exposes a high-level typed model for paths, parameters, and schemas — exactly what the generator needs. Latest version (Mar 2026). |
| github.com/itchyny/gojq | v0.12.18 | In-process jq filtering on JSON output | Provides `--jq` flag on every command. Pure Go, no cgo, ships inside the binary. The only mature pure-Go jq library. Same version as `jr`. |
| github.com/tidwall/pretty | v1.2.1 | JSON pretty-printing with color | Used for `--pretty` flag output. Faster than `encoding/json` indent + a terminal color library. Zero dependencies. Same version as `jr`. |

### Supporting Libraries (transitive, from libopenapi)

| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| github.com/pb33f/ordered-map/v2 | v2.3.0 | Ordered map for OpenAPI object iteration | Required by libopenapi for deterministic path ordering in generated code. |
| go.yaml.in/yaml/v4 | v4.0.0-rc.4 | YAML parsing for OpenAPI specs | libopenapi's YAML backend. The Confluence spec is JSON, but libopenapi normalises through YAML internally. |
| golang.org/x/sync | v0.20.0 | Concurrent spec processing | Used by libopenapi for parallel schema resolution. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| golangci-lint v2 | Static analysis and linting | v2.11.3 is current (Mar 2026). Run with `golangci-lint run`. Mirror `jr`'s existing `.golangci.yml` config if it exists, otherwise use `default: standard`. |
| go generate / `go run ./gen/...` | Code generation from OpenAPI spec | The `gen/` binary is a plain Go program — no external codegen tools (no `oapi-codegen`, no Swagger UI). Run via `make generate`. |
| curl | Downloading the Confluence OpenAPI spec | Used in `make spec-update`. Target URL: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` |

## Installation

```bash
# The project uses go modules — no npm/pip required.
# After cloning, dependencies are fetched automatically by go build/test.

go mod init github.com/sofq/confluence-cli   # or your chosen module path

# Core dependencies
go get github.com/spf13/cobra@v1.10.2
go get github.com/spf13/pflag@v1.0.10
go get github.com/pb33f/libopenapi@v0.34.3
go get github.com/itchyny/gojq@v0.12.18
go get github.com/tidwall/pretty@v1.2.1

# Dev tool (install globally, not a go.mod dependency)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| spf13/cobra | urfave/cli v3 | Never for this project — cobra is the constraint. Even without the constraint, cobra's persistent-flags + PersistentPreRunE hook is the right model for injecting the HTTP client into every command. |
| pb33f/libopenapi | kin-openapi (getkin/kin-openapi) | kin-openapi if you need request/response validation at runtime. We only need spec parsing at code-generation time, and libopenapi's high-level model is cleaner for walking paths and extracting parameters. |
| pb33f/libopenapi | oapi-codegen | oapi-codegen generates Go client/server stubs, not CLI commands. Our gen/ binary generates Cobra Command structs — oapi-codegen's output would require significant post-processing. Do not use. |
| itchyny/gojq | jq binary (exec.Command) | Never — exec.Command introduces a hard dependency on jq being installed; gojq is embedded in the binary. |
| tidwall/pretty | encoding/json with MarshalIndent | MarshalIndent re-parses the entire JSON payload. tidwall/pretty operates on the raw byte stream without unmarshaling, which is significantly faster on large Confluence page responses. |
| filesystem cache (internal/cache) | groupcache / Redis / bbolt | External cache stores are over-engineered for a CLI tool. The os.UserCacheDir() + file-per-request approach from jr is sufficient, simple, and requires zero dependencies. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| oapi-codegen | Generates HTTP client stubs and server handlers, not CLI commands. Output is incompatible with the hand-written + generated Cobra command architecture. | Custom gen/ binary (already in jr, copy directly) |
| spf13/viper | Adds 20+ transitive dependencies and brings in HCL/TOML/etc. parsers. Config needs are simple: JSON file + env vars. jr solves this with encoding/json + os.Getenv directly. | encoding/json + os.Getenv (same as jr) |
| mitchellh/go-homedir | Deprecated. Use os.UserHomeDir() (stdlib, Go 1.12+) and os.UserConfigDir() (Go 1.13+). | stdlib os package |
| github.com/rs/zerolog or logrus | Structured logging libraries add dependencies and complexity. All output is JSON to stdout/stderr via encoding/json directly — no logging library needed. | encoding/json + fmt.Fprintf(os.Stderr) |
| go-resty or cleanhttp | HTTP client abstractions. net/http stdlib is sufficient; the client layer is thin (auth injection, pagination, caching). Extra HTTP libraries obscure what headers/timeouts are actually set. | stdlib net/http with http.Client{Timeout: timeout} |

## Stack Patterns by Variant

**If the Confluence OpenAPI spec changes format (e.g., moves from JSON to YAML):**
- libopenapi handles both transparently — no change needed in gen/parser.go.

**If OAuth2 support needs to be extended (e.g., Atlassian OAuth 2.0 3LO flow):**
- Implement inside internal/client, not as an external oauth2 library. The current jr pattern (fetchOAuth2Token via client credentials grant) is the right template; extend it with PKCE if needed without adding golang.org/x/oauth2 as a dependency.

**If shell completions are needed:**
- cobra provides bash/zsh/fish/powershell completion generation via `rootCmd.GenBashCompletion` etc. No additional library required.

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| cobra v1.10.2 | pflag v1.0.10 | cobra v1.10.x requires pflag v1.0.9+. |
| libopenapi v0.34.3 | go.yaml.in/yaml/v4 v4.0.0-rc.4 | libopenapi migrated from gopkg.in/yaml.v3 to go.yaml.in/yaml/v4 around v0.30. Do not mix old yaml v3 and new yaml v4 in the same gen/ binary. |
| Go 1.26.1 | All listed libraries | All libraries support Go 1.21+ module toolchain directive. No compatibility issues with 1.26. |

## Sources

- [pkg.go.dev/github.com/spf13/cobra](https://pkg.go.dev/github.com/spf13/cobra) — confirmed v1.10.2, Dec 2024 (HIGH confidence)
- [pkg.go.dev/github.com/pb33f/libopenapi](https://pkg.go.dev/github.com/pb33f/libopenapi) — confirmed v0.34.3, Mar 2026 (HIGH confidence)
- [pkg.go.dev/github.com/itchyny/gojq](https://pkg.go.dev/github.com/itchyny/gojq) — confirmed v0.12.18, Dec 2025 (HIGH confidence)
- [pkg.go.dev/github.com/tidwall/pretty](https://pkg.go.dev/github.com/tidwall/pretty) — confirmed v1.2.1 (HIGH confidence)
- [pkg.go.dev/github.com/spf13/pflag](https://pkg.go.dev/github.com/spf13/pflag) — confirmed v1.0.10, Sep 2025 (HIGH confidence)
- [go.dev/doc/devel/release](https://go.dev/doc/devel/release) — Go 1.26.1 current stable (HIGH confidence)
- [github.com/golangci/golangci-lint/releases](https://github.com/golangci/golangci-lint/releases) — golangci-lint v2.11.3 current (MEDIUM confidence — version from WebSearch, not directly fetched)
- Reference implementation: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/go.mod` — primary source for dependency selection (HIGH confidence)

---
*Stack research for: Confluence Cloud CLI (Go + Cobra + OpenAPI codegen)*
*Researched: 2026-03-20*
