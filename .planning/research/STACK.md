# Stack Research

**Domain:** Confluence Cloud CLI (Go) -- v1.2 Stack Additions
**Researched:** 2026-03-28
**Confidence:** HIGH -- all Go features use stdlib only (zero new deps constraint validated); infrastructure tools verified against jr reference implementation and current releases.

## Existing Stack (DO NOT CHANGE)

Validated in v1.0 and v1.1, unchanged:
- Go 1.25.8, Cobra v1.10.2, pflag v1.0.9, libopenapi v0.34.3, gojq v0.12.18
- net/http stdlib client, encoding/json, filesystem cache
- OpenAPI code generation pipeline via gen/ binary
- OAuth2 (2LO + 3LO with PKCE) via stdlib
- NDJSON audit logging, watch/polling, template system, preset system

## New Go Packages (stdlib only -- ZERO new deps)

### internal/jsonutil

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (`bytes`, `encoding/json`) | (stdlib) | `MarshalNoEscape()` -- JSON marshal without HTML escaping of `&`, `<`, `>` | Confluence storage format is XHTML-based and contains `<`, `>`, `&` extensively. `encoding/json.Marshal()` HTML-escapes these by default, corrupting Confluence content in JSON output. The jr reference uses `json.NewEncoder(&buf).SetEscapeHTML(false)` in a 10-line helper. Direct port, no deps. |

**Implementation note:** Replace all ad-hoc `marshalNoEscape` usages in cmd/ with calls to `internal/jsonutil.MarshalNoEscape()`. The jr reference shows this exact pattern -- single function, single file, well-tested.

### internal/duration

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (`regexp`, `strconv`, `strings`, `fmt`) | (stdlib) | Parse human-friendly duration strings (e.g. "2h", "1d 3h", "30m") to seconds | Used by `diff --since` flag to filter version changes by relative time. Confluence does not use Jira's 1d=8h convention -- cf should use 1d=24h, 1w=7d (standard calendar time). The jr reference is ~55 lines with regex `(\d+)\s*(w|d|h|m)`. Port with modified time constants. |

**Confluence-specific adaptation:** Unlike Jira (1d = 8h workday), Confluence page versioning uses real calendar time. Constants: `1m = 60s`, `1h = 3600s`, `1d = 86400s`, `1w = 604800s`. This is the only change from the jr implementation.

### internal/preset (enhancement)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (`encoding/json`, `os`, `path/filepath`, `sort`) | (stdlib) | Built-in presets for Confluence content types + `preset list` subcommand | cf already has a preset system in cmd/ but needs to be promoted to `internal/preset` package with built-in presets (matching jr pattern). Adds `List()` function that merges built-in + user presets with source attribution. |

**Built-in presets for Confluence (domain-specific):**

| Preset | JQ Filter | Use Case |
|--------|-----------|----------|
| `agent` | `.results[] \| {id, title, status, spaceId}` | AI agent consumption -- minimal page fields |
| `detail` | `.results[] \| {id, title, status, spaceId, version, body}` | Full page detail with body content |
| `titles` | `.results[] \| {id, title}` | Quick page listing -- IDs and titles only |
| `versions` | `.version \| {number, message, createdAt, authorId}` | Version history summary |
| `spaces` | `.results[] \| {id, key, name, type, status}` | Space listing |

### internal/changelog (new -- version diff)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (`encoding/json`, `time`, `strings`, `fmt`) | (stdlib) | Parse Confluence page version history, flatten changes, filter by time/field | Powers the `diff` command. Fetches `/api/v2/pages/{id}/versions`, compares version bodies/titles/statuses. The jr reference has `internal/changelog` with `Parse()` function that flattens Jira changelog entries. Confluence's version model is different (full snapshots vs field-level changelog) but the output structure (timestamp, author, field, from, to) can match. |

**Confluence diff vs Jira diff -- key difference:** Jira has field-level changelog entries. Confluence stores full page snapshots per version. The `diff` command must fetch two versions and compute the difference (title changed, body changed, status changed). This is a structural adaptation, not a port.

### Workflow Commands (cmd/ layer -- no new packages)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (`net/http`, `net/url`, `encoding/json`, `fmt`) | (stdlib) | Workflow subcommands: move, copy, publish, restrict, archive, comment | All workflow commands use existing `internal/client` for HTTP + existing patterns in cmd/. No new internal packages needed -- these are API orchestration commands that compose existing HTTP primitives. |

**Confluence API endpoints for workflow commands:**

| Command | API Endpoint | HTTP Method | API Version | Notes |
|---------|-------------|-------------|-------------|-------|
| `workflow move` | `/wiki/rest/api/content/{id}/move/{position}/{targetId}` | PUT | v1 | No v2 equivalent. Position: `append` (child), `before`/`after` (sibling) |
| `workflow copy` | `/wiki/rest/api/content/{id}/copy` | POST | v1 | Body: `{destination: {type, value}, copyAttachments, copyPermissions, copyProperties, copyLabels}` |
| `workflow publish` | `/api/v2/pages/{id}` | PUT | v2 | Set `status: "current"` on a draft page |
| `workflow restrict` | `/wiki/rest/api/content/{id}/restriction` | PUT | v1 | Set read/update restrictions by user/group |
| `workflow archive` | `/wiki/rest/api/content/{id}/archive` | POST | v1 | Archives page + optional descendants |
| `workflow comment` | `/api/v2/pages/{id}/footer-comments` | POST | v2 | Already exists as `comments create`, this is a convenience alias |

**Important:** Move, copy, restrict, and archive use v1 API because Confluence v2 does not expose these operations. The existing `searchV1Domain()` helper in `internal/client` already handles v1 URL construction.

### Export Command (cmd/ layer)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib (`encoding/json`, `os`, `io`, `fmt`) | (stdlib) | Export page content as JSON (storage format), with optional version selection | Fetches page body via `GET /api/v2/pages/{id}?body-format=storage` and writes to stdout or file. Simple orchestration command -- no new packages. Supports `--format storage` (default, raw XHTML) and `--format atlas_doc_format` (Atlassian Document Format JSON). |

## Infrastructure Tools (external -- not Go dependencies)

### GoReleaser

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| GoReleaser | v2.14.x (via `goreleaser-action@v7`) | Cross-platform builds, GitHub releases, Homebrew tap, Scoop bucket, Docker images | Industry standard for Go CLI release automation. The jr reference uses GoReleaser v2 with `goreleaser-action@v7` in GitHub Actions. Version constraint `~> v2` in the action ensures latest v2 patch without breaking changes. Produces: tar.gz (linux/darwin), zip (windows), multi-arch Docker images (amd64/arm64), Homebrew formula, Scoop manifest. |

**Configuration (.goreleaser.yml):**
- `version: 2` -- GoReleaser v2 config format
- `builds`: binary `cf`, `CGO_ENABLED=0`, targets: `linux/darwin/windows` x `amd64/arm64`
- `ldflags`: `-s -w -X github.com/sofq/confluence-cli/cmd.Version={{.Version}}`
- `archives`: tar.gz default, zip for Windows
- `brews`: Homebrew tap to `sofq/homebrew-tap`
- `scoops`: Scoop bucket to `sofq/scoop-bucket`
- `dockers`: Multi-arch via buildx (ghcr.io/sofq/cf)
- `docker_manifests`: Version + latest tags
- `changelog`: Exclude docs/test/ci/chore commits

**Dockerfile.goreleaser:**
```dockerfile
FROM gcr.io/distroless/static:nonroot
COPY cf /usr/local/bin/cf
ENTRYPOINT ["cf"]
```

### VitePress Documentation Site

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| VitePress | ^1.6.4 | Static documentation site with auto-generated command reference | VitePress 1.x is the current stable release. v2.0 is still in alpha (v2.0.0-alpha.17) and NOT suitable for production. The jr reference uses `^1.6.4` successfully. Vue/Vite-powered, fast builds, built-in search, dark mode, sidebar. |
| Node.js | 24.x | VitePress build runtime | LTS track used in jr reference CI. Required for `npm ci` and `npx vitepress build`. |
| esbuild | ^0.25.0 (override) | Bundler used by VitePress internals | The jr reference uses `overrides: {"esbuild": "^0.25.0"}` to pin esbuild and avoid resolution issues. Mirror this. |

**website/package.json:**
```json
{
  "name": "cf-docs",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vitepress dev",
    "build": "vitepress build",
    "preview": "vitepress preview"
  },
  "devDependencies": {
    "vitepress": "^1.6.4"
  },
  "overrides": {
    "esbuild": "^0.25.0"
  }
}
```

**Auto-generated docs (cmd/gendocs/):** Go binary that walks the Cobra command tree, extracts flags/descriptions/API paths from schema ops, and generates:
1. Per-resource markdown pages (`website/commands/{resource}.md`)
2. Index page with command counts
3. Sidebar JSON (`website/.vitepress/sidebar-commands.json`)
4. Error codes reference page

### GitHub Actions CI/CD

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `actions/checkout` | `@v6` (SHA-pinned) | Repository checkout | Standard, pinned for supply chain security. Use the same SHA as jr reference: `de0fac2e4500dabe0009e67214ff5f5447ce83dd` |
| `actions/setup-go` | `@v6` (SHA-pinned) | Go toolchain setup with go.mod version | SHA: `4b73464bb391d4059bd26b0524d20df3927bd417`. Uses `go-version-file: go.mod` to stay in sync. |
| `actions/setup-node` | `@v6` (SHA-pinned) | Node.js for docs build + npm smoke test | SHA: `53b83947a5a98c8d113130e565377fae1a50d02f`. Node 24 for VitePress. |
| `actions/setup-python` | `@v6` (SHA-pinned) | Python for PyPI smoke test | SHA: `a309ff8b426b58ec0e2a45f0f869d46889d02405`. Python 3.12. |
| `goreleaser/goreleaser-action` | `@v7` (SHA-pinned) | GoReleaser execution in release workflow | SHA: `9a127d869fb706213d29cdf8eef3a4ea2b869415`. Version constraint: `~> v2`. |
| `golangci/golangci-lint-action` | `@v9` (SHA-pinned) | Lint runner in CI | SHA: `1e7e51e771db61008b38414a730f564565cf7c20`. Uses `version: latest` which resolves to golangci-lint v2.11.x. |
| `securego/gosec` | `@v2.24.7+` (SHA-pinned) | SAST security scanner | SHA: `bb17e422fc34bf4c0a2e5cab9d07dc45a68c040c` (jr reference). Exclude: `G104,G301,G304,G306`. Exclude dir: `cmd/generated`. |
| `codecov/codecov-action` | `@v5` (SHA-pinned) | Coverage upload | SHA: `671740ac38dd9b0130fbe1cec585b89eea48d3de`. |
| `actions/configure-pages` | `@v5` (SHA-pinned) | GitHub Pages config for docs deploy | SHA: `983d7736d9b0ae728b81ab479565c72886d7745b`. |
| `actions/upload-pages-artifact` | `@v3` (SHA-pinned) | Upload built docs for Pages deploy | SHA: `56afc609e74202658d3ffba0e8f6dda462b719fa`. |
| `actions/deploy-pages` | `@v4` (SHA-pinned) | Deploy to GitHub Pages | SHA: `d6db90164ac5ed86f2b6aed7e0febac5b3c0c03e`. |
| `docker/setup-buildx-action` | `@v4` (SHA-pinned) | Docker buildx for multi-arch images | SHA: `4d04d5d9486b7bd6fa91e7baf45bbb4f8b9deedd`. |
| `docker/login-action` | `@v4` (SHA-pinned) | GHCR login for Docker push | SHA: `b45d80f862d83dbcd57f89517bcf500b2ab88fb2`. |
| `peter-evans/create-pull-request` | `@v8` (SHA-pinned) | Auto PR for spec drift detection | SHA: `c0f553fe549906ede9cf27b5156039d195d2ece0`. |
| `pypa/gh-action-pypi-publish` | `@v1.13.0` (SHA-pinned) | PyPI package publishing | SHA: `ed0c53931b1dc9bd32cbe73a98c7f6766f8a527e`. |
| govulncheck | v1.1.4 (go install) | Go vulnerability scanner | Installed via `go install golang.org/x/vuln/cmd/govulncheck@v1.1.4`. |

**Workflow files to create (7 total, mirroring jr):**

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | push/PR to main | Build, test, lint, npm/pypi smoke test, docs build |
| `release.yml` | tag push `v*` + manual dispatch | GoReleaser + npm publish + PyPI publish |
| `docs.yml` | push to main (paths filter) | Build and deploy VitePress docs to GitHub Pages |
| `security.yml` | push/PR + weekly cron | gosec + govulncheck |
| `spec-drift.yml` | daily cron + manual | Download latest Confluence OpenAPI spec, diff, auto-PR |
| `spec-auto-release.yml` | PR merged with auto-release label | Auto-tag patch version for spec-driven changes |
| `dependabot-auto-merge.yml` | Dependabot PRs | Auto-merge Dependabot updates |

### golangci-lint Configuration

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| golangci-lint | v2.11.x (latest via action) | Go linter aggregator | v2 config format (`.golangci.yml` with `version: "2"`). Uses `linters.default: standard` (v2's replacement for enable-all/disable-all). The jr reference config is minimal and effective. |

**.golangci.yml:**
```yaml
version: "2"

linters:
  default: standard
  settings:
    errcheck:
      exclude-functions:
        - fmt.Fprintf
        - fmt.Fprintln
        - fmt.Fprint
        - (io.Writer).Write
        - (*net/http.Response.Body).Close
        - (io.Closer).Close
        - os.Setenv
        - os.Unsetenv
        - os.Remove
        - os.WriteFile
        - (*os.File).Close
```

### npm Package Scaffold

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Node.js npm package | N/A | `npm install confluence-cf` -- downloads platform binary on postinstall | Binary distribution via npm for Node.js/AI agent ecosystems. The jr reference pattern: `package.json` + `install.js` postinstall script that downloads the correct platform binary from GitHub Releases. No runtime dependencies. |

**Key files:**
- `npm/package.json` -- name: `confluence-cf`, bin: `{"cf": "bin/cf"}`, postinstall: `node install.js`
- `npm/install.js` -- Platform/arch detection, GitHub Release download, tar/zip extraction

### Python Package Scaffold

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Python pip package | N/A | `pip install confluence-cf` -- downloads platform binary on first run | Binary distribution via PyPI for Python/AI agent ecosystems. Uses `setuptools>=68.0` build backend. The jr reference pattern: `pyproject.toml` + `__init__.py` with lazy binary download on first `main()` call. |

**Key files:**
- `python/pyproject.toml` -- name: `confluence-cf`, requires-python: `>=3.8`, entry point: `cf = "confluence_cf:main"`
- `python/confluence_cf/__init__.py` -- Platform detection, lazy binary download, subprocess delegation

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| VitePress 1.x stable | VitePress 2.x alpha | Never for production. v2.0.0-alpha.17 is experimental. When v2 reaches stable, consider migration. |
| GoReleaser OSS v2 | goreleaser-pro | Only if you need features like NSIS installers, macOS .pkg signing, or Fury.io. OSS covers all cf needs. |
| golangci-lint v2 config | golangci-lint v1 config | Never. v2 config format is current, v1 format still works but deprecated path. |
| SHA-pinned actions | Tag-only actions (e.g. `@v6`) | Never for security-sensitive repos. SHA pinning prevents tag retargeting attacks. Use both: SHA + comment tag for readability. |
| Distroless Docker base | Alpine base | Alpine if you need shell access for debugging. Distroless is smaller and has zero CVEs. Use distroless for production. |
| gosec v2.24.7+ | None | gosec is the standard Go SAST scanner. No viable alternative with same coverage. |
| govulncheck | nancy, trivy | govulncheck is official Go team tooling, understands Go call graphs, zero false positives for unreachable code. |
| stdlib `text/template` for gendocs | html/template | gendocs output is markdown, not HTML. `text/template` avoids unwanted HTML escaping. |
| Custom Go test runner | testify | Zero deps constraint. Go stdlib `testing` + table-driven tests are sufficient. |

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/sergi/go-diff` or any Go diff library | Zero new deps constraint. Confluence versions are full snapshots, not patches. | Compare version fields manually (title, body hash, status) using stdlib `strings` and `crypto/sha256` for body diffing. |
| `golang.org/x/oauth2` | Already rejected in v1.1. Stdlib HTTP client handles OAuth2 token exchange. | Existing `internal/oauth2` package. |
| `github.com/tidwall/pretty` | jr uses it for JSON prettification, but cf outputs to agents (not humans). | Raw JSON output. If prettification needed later, `json.MarshalIndent`. |
| `gopkg.in/yaml.v3` | jr uses it for ADF (Atlassian Document Format). cf uses Confluence storage format (XHTML), not ADF. | Not needed -- Confluence storage format is handled as raw strings. |
| VitePress v2.0 alpha | Unstable, breaking changes expected. | VitePress ^1.6.4 (stable). |
| GoReleaser Pro | No features needed beyond OSS. | GoReleaser OSS v2. |
| Separate markdown diff library | Overkill for version comparison. | Body hash comparison + raw body output for human review. |
| `cobra-cli` scaffolding tool | Commands are hand-written or generated from OpenAPI. | Manual command creation matching existing patterns. |

## Version Compatibility

| Tool | Compatible With | Notes |
|------|-----------------|-------|
| GoReleaser v2.14.x | Go 1.25.x, goreleaser-action@v7 | v2 config format required (`version: 2` in `.goreleaser.yml`) |
| VitePress ^1.6.4 | Node 24.x, Vite 6.x | esbuild override ^0.25.0 recommended for stable resolution |
| golangci-lint v2.11.x | Go 1.25.x, golangci-lint-action@v9 | v2 config format (`version: "2"` in `.golangci.yml`) |
| gosec v2.24.7+ | Go 1.25.x | Requires `GOFLAGS=-buildvcs=false` in CI to avoid git metadata issues |
| govulncheck v1.1.4 | Go 1.25.x | Installed via `go install`, not a module dependency |
| goreleaser-action@v7 | GoReleaser v2.7.0+ | Required for GoReleaser v2 without `-pro` suffix |
| actions/setup-go@v6 | `go-version-file: go.mod` | Reads Go version from go.mod automatically |

## Makefile Additions

The existing Makefile needs these new targets:

```makefile
# Existing targets: generate, build, install, test, clean

# New targets for v1.2
lint:
	golangci-lint run

docs-generate:
	go run ./cmd/gendocs/... website

docs-dev: docs-generate
	cd website && npx vitepress dev

docs-build: docs-generate
	cd website && npx vitepress build

docs: docs-build
```

## Secrets Required for CI/CD

| Secret | Used By | Purpose |
|--------|---------|---------|
| `GITHUB_TOKEN` | release.yml, spec-drift.yml, dependabot-auto-merge.yml | Automatic -- GitHub provides this. Used for GoReleaser, auto-PR, auto-merge. |
| `HOMEBREW_TAP_TOKEN` | release.yml (GoReleaser) | PAT with repo scope for pushing to sofq/homebrew-tap. |
| `CODECOV_TOKEN` | ci.yml | Codecov upload token for coverage reporting. |
| `CF_BASE_URL` | ci.yml (integration tests) | Confluence Cloud base URL for integration tests. |
| `CF_AUTH_USER` | ci.yml (integration tests) | Basic auth user for integration tests. |
| `CF_AUTH_TOKEN` | ci.yml (integration tests) | Basic auth API token for integration tests. |

## Sources

- jr reference implementation (`/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/`) -- PRIMARY source for all patterns, versions, SHA pins (HIGH confidence)
- [GoReleaser v2.14 release](https://goreleaser.com/blog/goreleaser-v2.14/) -- Current stable version (HIGH confidence)
- [GoReleaser GitHub releases](https://github.com/goreleaser/goreleaser/releases) -- v2.14.3 latest (2026-03-09) (HIGH confidence)
- [golangci-lint v2 announcement](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/) -- v2 config format details (HIGH confidence)
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) -- v2.11.4 latest (2026-03-22) (HIGH confidence)
- [VitePress npm](https://www.npmjs.com/package/vitepress) -- v1.6.4 latest stable (HIGH confidence)
- [VitePress GitHub releases](https://github.com/vuejs/vitepress/releases) -- v2.0.0-alpha.17 is latest alpha, NOT stable (HIGH confidence)
- [gosec GitHub releases](https://github.com/securego/gosec/releases) -- v2.25.0 latest (2026-03-19) (HIGH confidence)
- [govulncheck Go docs](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) -- v1.1.4 (MEDIUM confidence -- may have newer)
- [Confluence Cloud REST API v2](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/) -- v2 endpoints (HIGH confidence)
- [Confluence move/copy API](https://community.developer.atlassian.com/t/added-move-and-copy-page-apis/37749) -- v1 endpoints for move/copy (HIGH confidence)
- [Confluence archive API](https://community.developer.atlassian.com/t/how-to-archive-and-restore-archived-confluence-content-via-rest-api/82062) -- Archive uses v1 (MEDIUM confidence)
- [Confluence content restrictions API](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content-restrictions/) -- v1 restrictions endpoints (HIGH confidence)

---
*Stack research for: Confluence CLI v1.2 (Workflow, Parity & Release Infrastructure)*
*Researched: 2026-03-28*
