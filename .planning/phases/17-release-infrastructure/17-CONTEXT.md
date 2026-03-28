# Phase 17: Release Infrastructure - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Complete CI/CD pipeline, cross-platform binary distribution (GoReleaser + Docker + Homebrew + Scoop + npm + PyPI), and standard open-source project files (README, LICENSE, SECURITY.md, .golangci.yml, .gitignore, Makefile) for public release. Everything needed to go from `git push tag` to binaries available on all platforms.

</domain>

<decisions>
## Implementation Decisions

### Package naming
- **D-01:** npm package name: `confluence-cf` (mirrors `jira-jr` pattern, confirmed available)
- **D-02:** PyPI package name: `confluence-cf` (mirrors `jira-jr` pattern, confirmed available)
- **D-03:** Docker image: `ghcr.io/sofq/cf` (binary name pattern)
- **D-04:** Homebrew formula: `cf` in `sofq/homebrew-tap`
- **D-05:** Scoop manifest: `cf` in `sofq/scoop-bucket`

### Distribution repos
- **D-06:** Share Homebrew tap (`sofq/homebrew-tap`) and Scoop bucket (`sofq/scoop-bucket`) with jr — add cf formula/manifest alongside jr

### Versioning strategy
- **D-07:** First release version: `v0.1.0` — signals new public project, iterate from there
- **D-08:** npm/PyPI OIDC first-publish must be manual before automated workflows work (known blocker)

### README structure
- **D-09:** Mimic jr README exactly — comprehensive standalone doc with badges, install methods, quick start, agent feature showcase, integration guide, security section, development section, license
- **D-10:** Adapt jr's section structure: header + badges, install (brew/npm/pip/scoop/go), quick start, "Why agents love cf" feature sections, agent integration, security, development, license

### CI/CD pipeline
- **D-11:** Mirror all 7 jr GitHub Actions workflows exactly, adapted for cf: ci.yml, release.yml, security.yml, spec-drift.yml, docs.yml, dependabot-auto-merge.yml, spec-auto-release.yml
- **D-12:** CI runs: build, test, lint (golangci-lint v2), npm smoke test, pypi smoke test, docs build, integration tests on main push
- **D-13:** Release triggered by version tag push — GoReleaser produces binaries (linux/darwin/windows, amd64/arm64), Docker multi-arch images, Homebrew formula, Scoop manifest
- **D-14:** npm/PyPI publish as post-release jobs with OIDC provenance
- **D-15:** Security pipeline: gosec + govulncheck on push/PR and weekly schedule
- **D-16:** Spec drift: daily cron checks Confluence OpenAPI spec, auto-regenerates, creates PR with auto-merge
- **D-17:** Spec auto-release: when spec-update PR merges with `auto-release` label, auto-tag next patch version
- **D-18:** Dependabot: gomod + github-actions weekly updates with auto-merge workflow
- **D-19:** Codecov integration for test coverage reporting

### Project files
- **D-20:** `.golangci.yml` v2 format with standard linters, errcheck exclusions matching jr (fmt.Fprintf, io.Writer.Write, http.Response.Body.Close, io.Closer.Close, os.Setenv/Unsetenv/Remove/WriteFile, os.File.Close)
- **D-21:** `.gitignore` comprehensive: binary, /dist/, OS files, IDE files, .env, docs output, website node_modules/dist/cache/commands, coverage.out, .claude/
- **D-22:** `Makefile` extended with: lint, spec-update, docs-generate, docs-dev, docs-build, docs targets (matching jr Makefile)
- **D-23:** `LICENSE` — Apache 2.0
- **D-24:** `SECURITY.md` — vulnerability reporting policy matching jr pattern (email security@sofq.dev, 48h ack)
- **D-25:** `Dockerfile.goreleaser` — distroless/static:nonroot base, single COPY + ENTRYPOINT

### GoReleaser config
- **D-26:** `.goreleaser.yml` v2 format mirroring jr exactly: CGO_ENABLED=0, before hooks (go mod tidy, go generate), archives (tar.gz + windows zip), checksums, changelog with exclude filters, brews, scoops, dockers (buildx multi-arch), docker_manifests (version + latest)

### npm/PyPI package scaffolds
- **D-27:** `npm/` directory: package.json + install.js + bin/ stub, postinstall downloads platform binary from GitHub release
- **D-28:** `python/` directory: pyproject.toml + jira_jr-style wrapper module, setuptools build, binary download on import

### Claude's Discretion
- Exact README copy/examples adapted from jr to cf context
- Spec URL for Confluence OpenAPI: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
- gosec exclusion codes (adapt from jr's G104,G301,G304,G306 as needed)
- Codecov token setup details
- Integration test environment variable names (CF_BASE_URL, CF_AUTH_TYPE, etc.)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### jr reference implementation (mirror source)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.goreleaser.yml` — GoReleaser config to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.golangci.yml` — Linter config to copy
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.gitignore` — Gitignore to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/Makefile` — Makefile targets to replicate
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/README.md` — README structure to mirror
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/LICENSE` — License to copy
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/SECURITY.md` — Security policy to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/Dockerfile.goreleaser` — Docker build to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/ci.yml` — CI pipeline to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/release.yml` — Release pipeline to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/security.yml` — Security pipeline to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/spec-drift.yml` — Spec drift to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/spec-auto-release.yml` — Auto-release to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/docs.yml` — Docs deploy to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/dependabot-auto-merge.yml` — Auto-merge to copy
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/dependabot.yml` — Dependabot config to copy
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/npm/package.json` — npm package scaffold to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/npm/install.js` — npm install script to adapt
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/python/pyproject.toml` — Python package to adapt

### Existing cf files to update
- `Makefile` — Extend with lint, spec-update, docs targets
- `.gitignore` — Replace with comprehensive version
- `go.mod` — Module path: `github.com/sofq/confluence-cli`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile` — Already has generate, build, install, test, clean targets; extend with lint, spec-update, docs
- `cmd/gendocs/main.go` — Phase 16 gendocs binary ready for `docs-generate` Makefile target
- `cmd.Version` — LDFLAGS already set up for version injection via `-X github.com/sofq/confluence-cli/cmd.Version`

### Established Patterns
- LDFLAGS version injection in Makefile matches jr pattern
- `go.mod` module path: `github.com/sofq/confluence-cli`
- Spec stored in `spec/` directory (same as jr)
- Generated commands in `cmd/generated/` (same as jr)

### Integration Points
- `.github/workflows/` — New directory, no conflicts
- `npm/`, `python/` — New directories
- `Makefile` — Extend existing file
- `.gitignore` — Replace existing file
- Root files: README.md, LICENSE, SECURITY.md, .golangci.yml, Dockerfile.goreleaser, .goreleaser.yml — All new

</code_context>

<specifics>
## Specific Ideas

- "Mimic exactly jira-jr" — the jr reference implementation is the definitive template for all files
- Every file should be a direct adaptation of the jr equivalent with s/jr/cf/, s/jira/confluence/ substitutions and cf-specific adjustments
- README should showcase cf-specific features (pages, spaces, CQL search, blog posts, templates, diff, workflow commands, watch, export)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 17-release-infrastructure*
*Context gathered: 2026-03-29*
