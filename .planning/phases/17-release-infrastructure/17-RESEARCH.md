# Phase 17: Release Infrastructure - Research

**Researched:** 2026-03-28
**Domain:** CI/CD pipelines, cross-platform binary distribution, open-source project scaffolding
**Confidence:** HIGH

## Summary

Phase 17 creates the complete release infrastructure for the `cf` CLI by adapting the proven `jr` (jira-cli) reference implementation. Every file -- from GitHub Actions workflows to GoReleaser config to npm/PyPI scaffolds -- is a direct adaptation of the jr equivalent with s/jr/cf/ and s/jira/confluence/ substitutions plus cf-specific adjustments.

The jr reference provides 7 GitHub Actions workflows (ci, release, security, spec-drift, docs, dependabot-auto-merge, spec-auto-release), GoReleaser v2 config, npm and PyPI package scaffolds, Homebrew and Scoop distribution, Docker multi-arch images, and standard project files (README, LICENSE, SECURITY.md, .golangci.yml, .gitignore, Makefile). All of these have been read and analyzed from the canonical sources listed in CONTEXT.md.

**Primary recommendation:** Implement by directly adapting each jr file with mechanical substitutions, keeping the same structure, action versions (pinned by SHA), and patterns. The only creative work is the README content (cf-specific features) and the spec drift URL (Confluence OpenAPI).

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** npm package name: `confluence-cf` (mirrors `jira-jr` pattern, confirmed available)
- **D-02:** PyPI package name: `confluence-cf` (mirrors `jira-jr` pattern, confirmed available)
- **D-03:** Docker image: `ghcr.io/sofq/cf` (binary name pattern)
- **D-04:** Homebrew formula: `cf` in `sofq/homebrew-tap`
- **D-05:** Scoop manifest: `cf` in `sofq/scoop-bucket`
- **D-06:** Share Homebrew tap (`sofq/homebrew-tap`) and Scoop bucket (`sofq/scoop-bucket`) with jr -- add cf formula/manifest alongside jr
- **D-07:** First release version: `v0.1.0`
- **D-08:** npm/PyPI OIDC first-publish must be manual before automated workflows work (known blocker)
- **D-09:** Mimic jr README exactly -- comprehensive standalone doc with badges, install methods, quick start, agent feature showcase, integration guide, security section, development section, license
- **D-10:** Adapt jr's section structure: header + badges, install (brew/npm/pip/scoop/go), quick start, "Why agents love cf" feature sections, agent integration, security, development, license
- **D-11:** Mirror all 7 jr GitHub Actions workflows exactly, adapted for cf
- **D-12:** CI runs: build, test, lint (golangci-lint v2), npm smoke test, pypi smoke test, docs build, integration tests on main push
- **D-13:** Release triggered by version tag push -- GoReleaser produces binaries, Docker images, Homebrew formula, Scoop manifest
- **D-14:** npm/PyPI publish as post-release jobs with OIDC provenance
- **D-15:** Security pipeline: gosec + govulncheck on push/PR and weekly schedule
- **D-16:** Spec drift: daily cron checks Confluence OpenAPI spec, auto-regenerates, creates PR with auto-merge
- **D-17:** Spec auto-release: when spec-update PR merges with `auto-release` label, auto-tag next patch version
- **D-18:** Dependabot: gomod + github-actions weekly updates with auto-merge workflow
- **D-19:** Codecov integration for test coverage reporting
- **D-20:** `.golangci.yml` v2 format with standard linters, errcheck exclusions matching jr
- **D-21:** `.gitignore` comprehensive: binary, /dist/, OS files, IDE files, .env, docs output, website node_modules/dist/cache/commands, coverage.out, .claude/
- **D-22:** `Makefile` extended with: lint, spec-update, docs-generate, docs-dev, docs-build, docs targets
- **D-23:** `LICENSE` -- Apache 2.0
- **D-24:** `SECURITY.md` -- vulnerability reporting policy matching jr pattern (email security@sofq.dev, 48h ack)
- **D-25:** `Dockerfile.goreleaser` -- distroless/static:nonroot base, single COPY + ENTRYPOINT
- **D-26:** `.goreleaser.yml` v2 format mirroring jr exactly
- **D-27:** `npm/` directory: package.json + install.js + bin/ stub, postinstall downloads platform binary from GitHub release
- **D-28:** `python/` directory: pyproject.toml + jira_jr-style wrapper module, setuptools build, binary download on import

### Claude's Discretion
- Exact README copy/examples adapted from jr to cf context
- Spec URL for Confluence OpenAPI: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
- gosec exclusion codes (adapt from jr's G104,G301,G304,G306 as needed)
- Codecov token setup details
- Integration test environment variable names (CF_BASE_URL, CF_AUTH_TYPE, etc.)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CICD-01 | GitHub Actions CI pipeline runs build, test, lint on push/PR to main | jr ci.yml fully analyzed; direct adaptation with test paths adjusted for cf structure |
| CICD-02 | GitHub Actions release pipeline builds cross-platform binaries via GoReleaser on tag push | jr release.yml fully analyzed; GoReleaser v2 config read and documented |
| CICD-03 | GitHub Actions security pipeline runs gosec + govulncheck weekly and on push | jr security.yml fully analyzed; gosec exclusions and govulncheck version documented |
| CICD-04 | GitHub Actions docs pipeline builds and deploys VitePress site to GitHub Pages | jr docs.yml fully analyzed; path triggers need cf-specific adjustment |
| CICD-05 | Spec drift detection runs daily, auto-regenerates commands, creates PR | jr spec-drift.yml fully analyzed; Confluence spec URL and file names differ from Jira |
| CICD-06 | Auto-release workflow tags and releases when spec-update PR merges | jr spec-auto-release.yml fully analyzed; direct copy with no changes needed |
| CICD-07 | Dependabot configured for Go modules and GitHub Actions weekly updates | jr dependabot.yml fully analyzed; direct copy |
| CICD-08 | GoReleaser produces binaries for linux/darwin/windows (amd64/arm64) + Docker images | jr .goreleaser.yml fully analyzed; adaptation documented with all substitutions |
| CICD-09 | npm package scaffold with postinstall binary download | jr npm/ scaffold fully analyzed; package.json, install.js, bin stub documented |
| CICD-10 | Python package scaffold with binary wrapper | jr python/ scaffold fully analyzed; pyproject.toml, __init__.py documented |
| DOCS-01 | README.md with install methods, quick start, key features, agent integration guide | jr README structure analyzed; cf features catalogued for adaptation |
| DOCS-02 | LICENSE file (Apache 2.0) | jr LICENSE read; Apache 2.0 with "Copyright 2026 sofq" |
| DOCS-03 | SECURITY.md with vulnerability reporting policy | jr SECURITY.md read; direct adaptation with cf references |
| CONF-01 | `.golangci.yml` with standard linters and errcheck exclusions | jr .golangci.yml read; v2 format with exact exclusion list documented |
| CONF-02 | Comprehensive `.gitignore` covering binaries, IDE files, docs output, env files | jr .gitignore read; cf-specific adjustments identified |
| CONF-03 | Makefile extended with lint, docs-generate, docs-dev, docs-build, spec-update targets | jr Makefile read; existing cf Makefile has base targets, extension plan documented |

</phase_requirements>

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| GoReleaser | v2 (`~> v2` in action) | Cross-compile Go binaries, create GitHub releases, Docker images, Homebrew/Scoop | Industry standard for Go binary distribution; v2 is current major |
| golangci-lint | v2 (latest via action) | Go linting with standard linter set | Standard Go linter aggregator; v2 has new config format |
| gosec | v2.24.7 (pinned SHA in jr) | Go security static analysis | Standard Go security scanner |
| govulncheck | v1.1.4 | Go vulnerability database checker | Official Go team vulnerability tool |
| peter-evans/create-pull-request | v8 (pinned SHA in jr) | Automated PR creation for spec drift | Most popular GitHub Action for automated PRs |
| pypa/gh-action-pypi-publish | v1.13.0 (pinned SHA in jr) | PyPI publishing with OIDC | Official PyPA publishing action |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| actions/checkout | v6 (SHA pinned) | Repository checkout in workflows | Every workflow job |
| actions/setup-go | v6 (SHA pinned) | Go toolchain setup | Any job needing Go compiler |
| actions/setup-node | v6 (SHA pinned) | Node.js setup for npm smoke test | npm smoke test, docs build |
| actions/setup-python | v6 (SHA pinned) | Python setup for PyPI smoke test | PyPI smoke test, publish |
| goreleaser/goreleaser-action | v7 (SHA pinned) | GoReleaser execution in CI | Release workflow only |
| docker/setup-buildx-action | v4 (SHA pinned) | Docker buildx for multi-arch | Release workflow only |
| docker/login-action | v4 (SHA pinned) | GHCR authentication | Release workflow only |
| codecov/codecov-action | v5 (SHA pinned) | Coverage upload | CI test job |
| golangci/golangci-lint-action | v9 (SHA pinned) | Lint execution | CI lint job |
| actions/configure-pages | v5 (SHA pinned) | GitHub Pages config | Docs workflow |
| actions/upload-pages-artifact | v3 (SHA pinned) | Upload docs build | Docs workflow |
| actions/deploy-pages | v4 (SHA pinned) | Deploy to GitHub Pages | Docs workflow |
| python `build` | 1.4.0 | Python package building | PyPI smoke test and release |

### No Alternatives Needed

All tools are locked decisions from CONTEXT.md mirroring the jr reference. No alternatives to consider.

## Architecture Patterns

### File Structure (New Files)

```
.github/
  workflows/
    ci.yml                    # CICD-01: build + test + lint + smoke tests + docs build + integration
    release.yml               # CICD-02, CICD-08: GoReleaser + npm + PyPI publish
    security.yml              # CICD-03: gosec + govulncheck
    docs.yml                  # CICD-04: VitePress build + deploy
    spec-drift.yml            # CICD-05: daily Confluence spec check
    spec-auto-release.yml     # CICD-06: auto-tag on spec-update merge
    dependabot-auto-merge.yml # CICD-07: auto-merge dependabot PRs
  dependabot.yml              # CICD-07: dependabot config
npm/
  package.json                # CICD-09: confluence-cf npm package
  install.js                  # CICD-09: postinstall binary downloader
  bin/                        # CICD-09: stub directory (created at install time)
python/
  pyproject.toml              # CICD-10: confluence-cf PyPI package
  confluence_cf/
    __init__.py               # CICD-10: binary wrapper module
  README.md                   # CICD-10: PyPI readme
.goreleaser.yml               # CICD-08: GoReleaser v2 config
.golangci.yml                 # CONF-01: linter config
Dockerfile.goreleaser         # CICD-08: minimal Docker image
README.md                     # DOCS-01: project readme
LICENSE                       # DOCS-02: Apache 2.0
SECURITY.md                   # DOCS-03: vulnerability policy
```

### Files to Modify (Existing)

```
Makefile                      # CONF-03: extend with lint, spec-update, docs-* targets
.gitignore                    # CONF-02: replace with comprehensive version
```

### Pattern 1: SHA-Pinned GitHub Actions

**What:** All third-party actions are referenced by full commit SHA, not version tag.
**When to use:** Every `uses:` in every workflow file.
**Why:** Supply chain security -- prevents tag mutation attacks.

The jr workflows use this exact pattern. The SHA pins from jr should be used directly since they reference the same action versions:

```yaml
# Pattern: owner/repo@SHA # human-readable version comment
- uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6
- uses: actions/setup-go@4b73464bb391d4059bd26b0524d20df3927bd417 # v6
- uses: actions/setup-node@53b83947a5a98c8d113130e565377fae1a50d02f # v6
- uses: actions/setup-python@a309ff8b426b58ec0e2a45f0f869d46889d02405 # v6
- uses: goreleaser/goreleaser-action@9a127d869fb706213d29cdf8eef3a4ea2b869415 # v7
- uses: docker/setup-buildx-action@4d04d5d9486b7bd6fa91e7baf45bbb4f8b9deedd # v4
- uses: docker/login-action@b45d80f862d83dbcd57f89517bcf500b2ab88fb2 # v4
- uses: codecov/codecov-action@671740ac38dd9b0130fbe1cec585b89eea48d3de # v5
- uses: golangci/golangci-lint-action@1e7e51e771db61008b38414a730f564565cf7c20 # v9
- uses: securego/gosec@bb17e422fc34bf4c0a2e5cab9d07dc45a68c040c # v2.24.7
- uses: peter-evans/create-pull-request@c0f553fe549906ede9cf27b5156039d195d2ece0 # v8
- uses: pypa/gh-action-pypi-publish@ed0c53931b1dc9bd32cbe73a98c7f6766f8a527e # v1.13.0
- uses: actions/configure-pages@983d7736d9b0ae728b81ab479565c72886d7745b # v5
- uses: actions/upload-pages-artifact@56afc609e74202658d3ffba0e8f6dda462b719fa # v3
- uses: actions/deploy-pages@d6db90164ac5ed86f2b6aed7e0febac5b3c0c03e # v4
```

### Pattern 2: GoReleaser v2 Config Structure

**What:** GoReleaser configuration with before hooks, cross-compilation, multi-format archives, Homebrew/Scoop/Docker distribution.
**Verified from:** jr .goreleaser.yml (read directly)

Key substitutions from jr to cf:

| jr value | cf value |
|----------|----------|
| `binary: jr` | `binary: cf` |
| `github.com/sofq/jira-cli/cmd.Version` | `github.com/sofq/confluence-cli/cmd.Version` |
| `sofq/homebrew-tap` | `sofq/homebrew-tap` (shared) |
| `sofq/scoop-bucket` | `sofq/scoop-bucket` (shared) |
| `ghcr.io/sofq/jr` | `ghcr.io/sofq/cf` |
| `homepage: https://github.com/sofq/jira-cli` | `homepage: https://github.com/sofq/confluence-cli` |
| `description: Agent-friendly Jira CLI...` | `description: Agent-friendly Confluence CLI...` |
| `license: MIT` | `license: Apache-2.0` |
| `jira-cli_{{ .Version }}...` (archive name) | `confluence-cli_{{ .Version }}...` (archive name uses ProjectName) |

### Pattern 3: OIDC Publishing with Manual First-Publish

**What:** npm and PyPI publish jobs use OIDC (id-token: write) for tokenless publishing with provenance, but the very first publish must be done manually.
**Why:** npm does not support "pending" trusted publishers -- the package must exist on npmjs.com before OIDC can be configured. PyPI does support pending publishers, so its first publish can be OIDC-based if configured in advance.

Steps for first release:
1. **PyPI:** Configure pending trusted publisher on pypi.org before first release -- can be fully automated from day 1
2. **npm:** Must do `npm publish` manually for v0.1.0 to create the package, then configure OIDC on npmjs.com

Both publish jobs have `continue-on-error: true` in the release workflow (matching jr) to prevent npm/PyPI failures from blocking the GitHub Release.

### Pattern 4: Spec Drift with Auto-Regeneration

**What:** Daily cron checks the Confluence OpenAPI spec for changes, regenerates Go commands, runs tests, and creates a PR.

Key differences from jr:
- **Spec URL:** `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` (not the Jira swagger URL)
- **Spec filename:** `spec/confluence-v2.json` (not `spec/jira-v3.json`)
- **Latest temp file:** `spec/confluence-v2-latest.json`
- **Branch name:** `auto/spec-update` (same as jr)
- **PR commit message:** `deps: update Confluence OpenAPI spec`

### Pattern 5: Release Workflow with workflow_dispatch Re-publish

**What:** Release workflow supports both tag-push (creates release) and manual workflow_dispatch (re-publishes npm/PyPI for an existing release). The `release` job is skipped on dispatch; `npm-publish` and `pypi-publish` run regardless.

```yaml
env:
  TAG: ${{ github.event.inputs.tag || github.ref_name }}
```

This pattern is essential for recovering from npm/PyPI publish failures without re-creating the GitHub Release.

### Anti-Patterns to Avoid

- **Unpinned action versions:** Never use `@v6` without the full SHA pin. Every `uses:` must have `@FULL_SHA # vN` format.
- **Hardcoded Go version:** Always use `go-version-file: go.mod` instead of hardcoding a version number.
- **Missing `continue-on-error`** on npm/PyPI publish jobs: These external registries can have transient failures; the GitHub Release must not be blocked.
- **Forgetting `fetch-depth: 0`:** The release and spec-drift workflows need full git history for tag detection and changelog generation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform binary builds | Custom build scripts per OS/arch | GoReleaser v2 | CGO_ENABLED=0 cross-compilation, checksum generation, changelog, 6 targets in one config |
| Docker multi-arch images | Separate docker build commands | GoReleaser `dockers` + `docker_manifests` + buildx | Handles platform-specific builds and manifest list creation |
| Homebrew formula generation | Manual formula file | GoReleaser `brews` section | Auto-updates formula on release, handles checksums |
| npm binary installer | Custom download script | Adapt jr's install.js pattern | Handles redirect following, tar/zip extraction, platform detection |
| Python binary wrapper | Custom subprocess wrapper | Adapt jr's __init__.py pattern | Handles platform detection, download, extraction, exec |
| Automated PR creation | Custom git push + gh pr create | peter-evans/create-pull-request | Handles branch creation, commit, PR update, labels |
| OIDC publishing | Manual token management | GitHub OIDC id-token + npm/PyPI trusted publishers | Eliminates long-lived secrets, provides provenance |

**Key insight:** The jr reference implementation has already solved every distribution problem. The task is adaptation, not invention.

## Common Pitfalls

### Pitfall 1: npm OIDC First-Publish Deadlock
**What goes wrong:** The release workflow tries to publish to npm via OIDC, but the package does not exist on npmjs.com yet, and OIDC cannot create new packages.
**Why it happens:** npm (unlike PyPI) does not support "pending" trusted publishers. The package must exist before OIDC can be configured.
**How to avoid:** The first `v0.1.0` release requires a manual `npm publish` with a token. After that, configure OIDC on npmjs.com. The workflow has `continue-on-error: true` so this does not block the GitHub Release.
**Warning signs:** npm-publish job fails with authentication errors on first release.

### Pitfall 2: GoReleaser License Field Mismatch
**What goes wrong:** jr uses `license: MIT` in the brews and scoops sections. cf uses Apache 2.0.
**Why it happens:** Mechanical s/jr/cf/ substitution misses the license field.
**How to avoid:** Explicitly set `license: Apache-2.0` in both the `brews` and `scoops` sections of `.goreleaser.yml`.
**Warning signs:** Homebrew formula shows wrong license.

### Pitfall 3: Spec Drift URL and Filename Confusion
**What goes wrong:** Using the Jira spec URL or filenames instead of Confluence ones.
**Why it happens:** Copy-paste from jr reference without updating URL and filenames.
**How to avoid:** Use these exact values:
  - URL: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
  - Current spec: `spec/confluence-v2.json`
  - Latest temp: `spec/confluence-v2-latest.json`
**Warning signs:** Spec drift workflow downloads Jira spec instead of Confluence spec.

### Pitfall 4: Python Module Naming with Hyphens
**What goes wrong:** Python module uses hyphens (invalid) or wrong naming convention.
**Why it happens:** PyPI package name `confluence-cf` has a hyphen but Python modules cannot.
**How to avoid:** PyPI package: `confluence-cf`, Python module directory: `confluence_cf/`, import: `from confluence_cf import main`. Follow jr pattern: PyPI name `jira-jr`, module `jira_jr/`.
**Warning signs:** `ImportError` during PyPI smoke test.

### Pitfall 5: npm Archive Name Mismatch
**What goes wrong:** The npm install.js constructs a download URL with the wrong archive name pattern.
**Why it happens:** GoReleaser uses `ProjectName` from the repo name in archive templates. For cf, the project name derives from the Go module or the `builds[0].binary` name.
**How to avoid:** GoReleaser's `name_template` in archives uses `{{ .ProjectName }}` which defaults to the directory name or can be set explicitly. The npm install.js must construct URLs matching the actual release asset names. Use `confluence-cli_${version}_${platform}_${arch}.${ext}` matching the GoReleaser output.
**Warning signs:** npm postinstall fails with 404 errors on download.

### Pitfall 6: Missing Workflow Permissions
**What goes wrong:** Workflows fail with permission errors.
**Why it happens:** GitHub Actions default to read-only `GITHUB_TOKEN`. Each job needs explicit permissions.
**How to avoid:** Copy the exact `permissions` blocks from jr workflows:
  - `contents: read` (default for most jobs)
  - `contents: write` + `packages: write` (release job)
  - `id-token: write` (npm/PyPI OIDC publish)
  - `pages: write` + `id-token: write` (docs deploy)
  - `contents: write` + `pull-requests: write` (spec-drift, auto-merge)
**Warning signs:** Jobs fail immediately with 403/permission denied errors.

### Pitfall 7: Docs Workflow Path Triggers
**What goes wrong:** Docs workflow does not trigger on cf-specific paths, or triggers on irrelevant paths.
**Why it happens:** jr's docs.yml has path triggers for `cmd/**`, `gen/**`, `spec/**`, `website/**`, `internal/errors/**`, `skill/**`, `Makefile`. cf may not have a `skill/` directory.
**How to avoid:** Adjust path triggers for cf's actual directory structure. Include `cmd/**`, `gen/**`, `spec/**`, `website/**`, `internal/**`, `Makefile`, `.github/workflows/docs.yml`. Omit `skill/**` if no skill directory exists.
**Warning signs:** Docs not rebuilding after command changes, or unnecessary rebuilds.

### Pitfall 8: Forgetting `--provenance` for npm Publish
**What goes wrong:** npm publish succeeds but without provenance attestation.
**Why it happens:** While trusted publishing auto-generates provenance in some cases, explicitly passing `--provenance` ensures it. The jr release.yml includes `npm publish --provenance --access public`.
**How to avoid:** Always include `--provenance --access public` flags.
**Warning signs:** Package on npmjs.com lacks provenance badge.

### Pitfall 9: CI Test Path Must Include All Test Locations
**What goes wrong:** CI runs tests but misses some test packages.
**Why it happens:** jr uses specific test paths: `./internal/... ./gen/... ./test/e2e/`. cf's test structure may differ.
**How to avoid:** Verify cf's actual test locations. Currently cf has tests in `cmd/` (cmd/*_test.go), `internal/` (internal/**/test files), and `gen/` (gen/*_test.go). Use `go test ./...` for comprehensive coverage or list explicit paths.
**Warning signs:** CI passes but local `go test ./...` finds failures.

### Pitfall 10: Distroless Image SHA Pinning
**What goes wrong:** Docker builds fail because the distroless image SHA is wrong or outdated.
**Why it happens:** The jr Dockerfile.goreleaser pins `gcr.io/distroless/static:nonroot` by SHA.
**How to avoid:** Use the same SHA from jr's Dockerfile.goreleaser. The distroless images are immutable, so the same SHA works across projects: `gcr.io/distroless/static:nonroot@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39`.
**Warning signs:** Docker build fails with manifest not found.

## Code Examples

### GoReleaser Config (cf adaptation)

The cf `.goreleaser.yml` adapts jr's config with these key fields changed:

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - binary: cf
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X github.com/sofq/confluence-cli/cmd.Version={{.Version}}

archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}

# ... (checksum, changelog same as jr)

brews:
  - name: cf
    repository:
      owner: sofq
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: https://github.com/sofq/confluence-cli
    description: Agent-friendly Confluence CLI with structured JSON output and jq filtering
    license: Apache-2.0
    install: |
      bin.install "cf"
    test: |
      system "#{bin}/cf", "version"

scoops:
  - name: cf
    repository:
      owner: sofq
      name: scoop-bucket
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: https://github.com/sofq/confluence-cli
    description: Agent-friendly Confluence CLI with structured JSON output and jq filtering
    license: Apache-2.0

dockers:
  - image_templates:
      - "ghcr.io/sofq/cf:{{ .Version }}-amd64"
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
    dockerfile: Dockerfile.goreleaser
    goarch: amd64
  - image_templates:
      - "ghcr.io/sofq/cf:{{ .Version }}-arm64"
    use: buildx
    build_flag_templates:
      - "--platform=linux/arm64"
    dockerfile: Dockerfile.goreleaser
    goarch: arm64

docker_manifests:
  - name_template: "ghcr.io/sofq/cf:{{ .Version }}"
    image_templates:
      - "ghcr.io/sofq/cf:{{ .Version }}-amd64"
      - "ghcr.io/sofq/cf:{{ .Version }}-arm64"
  - name_template: "ghcr.io/sofq/cf:latest"
    image_templates:
      - "ghcr.io/sofq/cf:{{ .Version }}-amd64"
      - "ghcr.io/sofq/cf:{{ .Version }}-arm64"
```

### golangci-lint v2 Config

Direct copy from jr -- same errcheck exclusions apply to cf:

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

### Makefile Extension

Existing cf Makefile has: generate, build, install, test, clean. Add these targets (matching jr):

```makefile
.PHONY: generate build install test clean lint spec-update docs-generate docs-dev docs-build docs

VERSION ?= dev
LDFLAGS := -s -w -X github.com/sofq/confluence-cli/cmd.Version=$(VERSION)
SPEC_URL := https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json

# ... existing targets ...

lint:
	golangci-lint run

spec-update:
	curl -sL "$(SPEC_URL)" -o spec/confluence-v2.json

docs-generate:
	go run ./cmd/gendocs/... website

docs-dev: docs-generate
	cd website && npx vitepress dev

docs-build: docs-generate
	cd website && npx vitepress build

docs: docs-build
```

### npm Package Scaffold

**npm/package.json** key fields:
```json
{
  "name": "confluence-cf",
  "version": "0.1.0",
  "description": "Agent-friendly Confluence CLI with structured JSON output and jq filtering",
  "license": "Apache-2.0",
  "repository": {
    "type": "git",
    "url": "https://github.com/sofq/confluence-cli"
  },
  "homepage": "https://sofq.github.io/confluence-cli/",
  "keywords": ["confluence", "cli", "ai", "agent", "json"],
  "bin": {
    "cf": "bin/cf"
  },
  "scripts": {
    "postinstall": "node install.js"
  },
  "files": ["bin/", "install.js"]
}
```

**npm/install.js** key substitutions from jr:
- `REPO = "sofq/confluence-cli"`
- Binary name: `cf` (not `jr`)
- Archive pattern: `confluence-cli_${version}_${platform}_${arch}.${ext}`
- User-Agent: `cf-npm-installer`

### Python Package Scaffold

**python/pyproject.toml** key fields:
```toml
[project]
name = "confluence-cf"
version = "0.0.0"
description = "Agent-friendly Confluence CLI with structured JSON output and jq filtering"
license = "Apache-2.0"
keywords = ["confluence", "cli", "ai", "agent", "json"]

[project.scripts]
cf = "confluence_cf:main"
```

**python/confluence_cf/__init__.py** key substitutions from jr:
- `REPO = "sofq/confluence-cli"`
- Binary name: `cf` (not `jr`)
- Archive pattern: `confluence-cli_{version}_{plat}_{arch}.{ext}`
- Version source: `version("confluence-cf")`

### Dockerfile.goreleaser

```dockerfile
FROM gcr.io/distroless/static:nonroot@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39
COPY cf /usr/local/bin/cf
ENTRYPOINT ["cf"]
```

### CI Workflow Key Differences from jr

**Test job:** cf test paths differ from jr. Use `go test ./...` for comprehensive coverage or adapt to cf structure:
```yaml
- name: Unit tests
  run: go test ./... -v -coverprofile=coverage.out -covermode=atomic
```

**npm smoke test:** Package name changes:
```yaml
- name: Smoke test npm pack + install
  run: |
    cd npm
    npm pack
    mkdir /tmp/test-install && cd /tmp/test-install
    npm init -y
    npm install "$GITHUB_WORKSPACE"/npm/confluence-cf-*.tgz 2>&1 | tee install.log
    if grep -q "MODULE_NOT_FOUND" install.log; then
      echo "ERROR: install.js has missing Node.js dependencies"
      exit 1
    fi
```

**PyPI smoke test:** Module name changes:
```yaml
- name: Smoke test pip build + install
  run: |
    pip install build==1.4.0
    cd python && python -m build
    pip install dist/confluence_cf-*.whl 2>&1 | tee install.log
    python -c "from confluence_cf import _get_binary_path; print('import ok')"
```

**Integration test** environment variables:
```yaml
- name: Integration tests
  env:
    CF_BASE_URL: ${{ secrets.CF_BASE_URL }}
    CF_AUTH_TYPE: basic
    CF_AUTH_USER: ${{ secrets.CF_AUTH_USER }}
    CF_AUTH_TOKEN: ${{ secrets.CF_AUTH_TOKEN }}
  run: go test ./test/integration/ -v -timeout 120s
```

### Security Workflow gosec Exclusions

Adapt from jr's `G104,G301,G304,G306`:
- **G104:** Errors unhandled -- covered by errcheck exclusions in golangci-lint
- **G301:** Expect directory permissions to be 0750 or less
- **G304:** File path provided as taint input (spec file paths are controlled)
- **G306:** Expect WriteFile permissions to be 0600 or less

Also exclude generated code: `-exclude-dir=cmd/generated`

### README Section Structure (cf adaptation of jr)

```
1. Header (centered h1: cf)
2. Tagline: "The Confluence CLI that speaks JSON -- built for AI agents"
3. Badges: npm, PyPI, GitHub Release, CI, Codecov, Security, License
4. Blockquote: Pure JSON stdout, structured errors, semantic exit codes, auto-generated commands
5. ---
6. ## Install (brew/npm/pip/scoop/go)
7. ## Quick start (configure + basic usage)
8. ## Why agents love cf
   - ### Self-describing (schema command)
   - ### Token-efficient (--fields, --jq, --preset)
   - ### CQL search (powerful Confluence query)
   - ### Page management (create, update, diff)
   - ### Workflow commands (move, copy, publish, archive, comment, restrict)
   - ### Watch (NDJSON event stream)
   - ### Templates (structured page creation)
   - ### Diff (structured version comparison)
   - ### Export (page/tree export in multiple formats)
   - ### Batch (N operations, one process)
   - ### Error contract (exit codes table)
   - ### Raw escape hatch
9. ## Agent integration (Claude Code skill, generic instructions)
10. ## Security (operation policies, audit logging, batch limits)
11. ## Development (make targets)
12. ## License (Apache 2.0)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| npm classic tokens | npm OIDC trusted publishing | GA July 2025 | Classic tokens permanently deprecated Dec 2025; OIDC is the only way forward |
| golangci-lint v1 config | golangci-lint v2 config format | March 2025 | New `linters.default: standard` syntax, `settings` under `linters` not top-level |
| GoReleaser v1 | GoReleaser v2 | 2024 | `version: 2` required in config, some field renames |
| Manual Dependabot merge | Dependabot auto-merge workflow | Current | gh pr merge --auto --squash in dedicated workflow |
| PyPI API tokens | PyPI OIDC trusted publishers | 2023+ | Supports pending publishers (unlike npm), zero secrets needed |

**Deprecated/outdated:**
- npm classic tokens: Permanently revoked Dec 9, 2025. Cannot be created or restored. Use OIDC only.
- golangci-lint v1 config: Will not work with v2 binary. Must use v2 format.
- `gcr.io` hosting fears: Despite migration notices, distroless images still served on gcr.io domain via artifact registry backend.

## Open Questions

1. **Integration test directory existence**
   - What we know: jr has `./test/integration/` directory with integration tests. cf currently has no `test/` directory.
   - What's unclear: Whether cf has or will have integration tests before Phase 17.
   - Recommendation: Include the integration test job in ci.yml but conditionally -- use `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` (matching jr) and reference `./test/integration/` path. If the directory does not exist, the job will simply have no tests to run but will not fail. Alternatively, the integration job can be added in a future phase.

2. **Website directory for docs workflow**
   - What we know: jr has a `website/` directory with VitePress. cf does not yet have one (DOCS-04 is Phase 18).
   - What's unclear: Whether docs.yml should be included now or deferred to Phase 18.
   - Recommendation: Include docs.yml in Phase 17 since CICD-04 is a Phase 17 requirement. The workflow will not trigger until the website directory is created in Phase 18, thanks to path-based triggers. The docs-build job in ci.yml should be conditional or omitted until Phase 18.

3. **Codecov token configuration**
   - What we know: jr uses `${{ secrets.CODECOV_TOKEN }}` in the CI workflow.
   - What's unclear: Whether the Codecov project has been created for sofq/confluence-cli.
   - Recommendation: Include the Codecov step in ci.yml with the token reference. The step will gracefully fail if the token is not configured (codecov-action does not fail the build by default).

4. **GoReleaser ProjectName default**
   - What we know: GoReleaser defaults `ProjectName` to the repo directory name. For `confluence-cli` repo, archives will be named `confluence-cli_VERSION_OS_ARCH.EXT`.
   - What's unclear: Whether the user wants explicit `project_name: confluence-cli` in .goreleaser.yml.
   - Recommendation: Rely on the default (directory name = `confluence-cli`). The npm install.js and Python __init__.py must match this pattern exactly.

## Comprehensive Substitution Reference

For implementers: complete mapping of jr values to cf values across all files.

| Context | jr value | cf value |
|---------|----------|----------|
| Binary name | `jr` | `cf` |
| Module path | `github.com/sofq/jira-cli` | `github.com/sofq/confluence-cli` |
| Repo slug | `sofq/jira-cli` | `sofq/confluence-cli` |
| npm package | `jira-jr` | `confluence-cf` |
| PyPI package | `jira-jr` | `confluence-cf` |
| Python module | `jira_jr` | `confluence_cf` |
| Docker image | `ghcr.io/sofq/jr` | `ghcr.io/sofq/cf` |
| Brew formula name | `jr` | `cf` |
| Scoop manifest name | `jr` | `cf` |
| License | `MIT` | `Apache-2.0` |
| Spec URL | `https://dac-static.atlassian.com/cloud/jira/platform/swagger-v3.v3.json` | `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` |
| Spec file | `spec/jira-v3.json` | `spec/confluence-v2.json` |
| Spec temp file | `spec/jira-v3-latest.json` | `spec/confluence-v2-latest.json` |
| Description | `Agent-friendly Jira CLI with structured JSON output and jq filtering` | `Agent-friendly Confluence CLI with structured JSON output and jq filtering` |
| CI env prefix | `JR_` | `CF_` |
| npm tgz pattern | `jira-jr-*.tgz` | `confluence-cf-*.tgz` |
| PyPI whl pattern | `jira_jr-*.whl` | `confluence_cf-*.whl` |
| npm smoke import check | `MODULE_NOT_FOUND` | `MODULE_NOT_FOUND` (same) |
| PyPI smoke import | `from jira_jr import _get_binary_path` | `from confluence_cf import _get_binary_path` |
| Homepage | `https://sofq.github.io/jira-cli/` | `https://sofq.github.io/confluence-cli/` |
| PR commit message | `deps: update Jira OpenAPI spec` | `deps: update Confluence OpenAPI spec` |
| User-Agent | `jr-npm-installer` | `cf-npm-installer` |

## Sources

### Primary (HIGH confidence)
- jr reference files (all read directly from filesystem):
  - `.goreleaser.yml`, `.golangci.yml`, `.gitignore`, `Makefile`, `README.md`, `LICENSE`, `SECURITY.md`, `Dockerfile.goreleaser`
  - `.github/workflows/ci.yml`, `release.yml`, `security.yml`, `spec-drift.yml`, `spec-auto-release.yml`, `docs.yml`, `dependabot-auto-merge.yml`
  - `.github/dependabot.yml`
  - `npm/package.json`, `npm/install.js`
  - `python/pyproject.toml`, `python/jira_jr/__init__.py`, `python/README.md`
- cf existing files (read directly):
  - `Makefile`, `.gitignore`, `go.mod`, `spec/confluence-v2.json`, `cmd/root.go` Version injection

### Secondary (MEDIUM confidence)
- [npm trusted publishing docs](https://docs.npmjs.com/trusted-publishers/) - OIDC setup requirements
- [PyPI trusted publishers](https://docs.pypi.org/trusted-publishers/) - Pending publisher support confirmed
- [npm OIDC first-publish issue](https://github.com/npm/cli/issues/8544) - Manual first-publish requirement confirmed
- [golangci-lint v2 blog](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/) - v2 config format
- [GoReleaser releases](https://github.com/goreleaser/goreleaser/releases) - v2.14 is latest
- [gosec releases](https://github.com/securego/gosec/releases) - v2.24.7 pinned in jr

### Tertiary (LOW confidence)
- None -- all findings verified against primary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all tools read from working jr reference implementation
- Architecture: HIGH -- every file pattern read from canonical jr sources and cf existing code
- Pitfalls: HIGH -- npm OIDC limitation verified via official docs and GitHub issues; all other pitfalls derived from direct comparison of jr/cf codebases
- Substitution mapping: HIGH -- complete mapping derived from reading both codebases

**Research date:** 2026-03-28
**Valid until:** 2026-04-28 (stable toolchain, pinned versions)
