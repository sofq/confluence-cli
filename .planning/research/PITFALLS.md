# Domain Pitfalls

**Domain:** Adding workflow commands, version diff, export, CI/CD, GoReleaser, VitePress docs, npm/Python packaging to existing Go CLI for Confluence Cloud
**Researched:** 2026-03-28

---

## Critical Pitfalls

Mistakes that cause rewrites, broken releases, or data loss.

### Pitfall 1: Confluence Version Number Lag Causes Silent 409 Conflicts

**What goes wrong:** The Confluence Cloud v2 API has a documented lag in propagating page version numbers. After updating a page, immediately reading it back may return the OLD version number. A subsequent update using that stale number triggers a 409 Conflict because the version must be exactly `current + 1`.

**Why it happens:** Confluence Cloud is eventually consistent. The version number returned by GET may lag behind the actual state after a PUT. This is worse for rapid sequential operations (e.g., a workflow that creates then immediately updates a page, or a copy followed by a restriction change).

**Consequences:** Silent data loss -- the second update fails with 409, and if not handled, the user's changes are lost. Batch workflows that chain operations (move -> rename -> restrict) are especially vulnerable.

**Prevention:**
- After any write operation, treat the version number from the response as authoritative -- do NOT re-fetch to get the version
- Implement retry with re-fetch: on 409, re-read the page, get the new version, and retry the update
- For the `diff` command: always fetch both version bodies in a single flow rather than caching version numbers across calls
- Document this behavior in CLI help text so agents know to expect occasional 409s

**Detection:** Test by rapidly updating the same page twice in succession. If the second update fails with 409, the version propagation lag is present.

**Confidence:** HIGH -- confirmed by [Atlassian developer community reports](https://community.developer.atlassian.com/t/lag-in-updating-page-version-number-v2-confluence-rest-api/68821) and the existing `fetchPageVersion` pattern in `cmd/pages.go` already handles this for single updates.

---

### Pitfall 2: Restrictions API is v1-Only with Non-Obvious Model

**What goes wrong:** The `restrict` workflow command needs page restriction management, but this is a v1-only API with no v2 equivalent. The restriction model has two independent axes (operation: read/update) x (subject: user/group) and four non-obvious behaviors:
1. View restrictions cascade to children; edit restrictions do NOT
2. The GET restrictions API does NOT return inherited restrictions -- only explicit ones
3. PUT replaces ALL restrictions for an operation (not additive) -- adding one user to "read" wipes all other read restrictions unless you re-include them
4. You cannot evict yourself from read/update restrictions -- you must always include the current user in the restriction set, or the API rejects the request

**Why it happens:** The v2 API simply does not have restriction endpoints yet (as of March 2026). The v1 restriction model was designed for the UI where users see the full picture, not for atomic CLI operations.

**Consequences:** A `cf pages restrict --user alice --operation read` command that naively PUTs only Alice's restriction will silently remove all existing read restrictions. Users lose access to pages they previously could read. Worse, if the calling user is not included in the restriction set, the API rejects the entire request, leaving the user confused.

**Prevention:**
- For the `restrict` command: always GET existing restrictions first, merge the new restriction, then PUT the full set
- Implement `--replace` vs `--add` flags (default to `--add` for safety)
- Use v1 endpoints: `GET/PUT /wiki/rest/api/content/{id}/restriction/byOperation/{operationKey}`
- Auto-include the current authenticated user in any restriction set to prevent self-lockout
- Warn users that inherited restrictions are invisible to the API -- what they see in the UI may not match what the API returns
- Add a `--dry-run` mode that shows the before/after restriction state
- Use `accountId` for user identification -- `username` parameter is deprecated and being removed

**Detection:** Set a restriction on a parent page in the UI, then query the child page's restrictions via API -- the API returns empty even though the child is restricted.

**Confidence:** HIGH -- confirmed by [Atlassian support KB](https://support.atlassian.com/confluence/kb/confluence-get-page-restrictions-api-doesnt-display-inherited-restrictions/), [developer community discussion](https://community.developer.atlassian.com/t/page-restrictions-via-the-v2-api/93094), and [v1 restriction update discussion](https://community.developer.atlassian.com/t/update-content-restrictions-with-api-v1/88400).

---

### Pitfall 3: Move/Copy Are v1 Async Operations with Dangerous Edge Cases

**What goes wrong:** The move page API (`PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}`) and copy page API (`POST /wiki/rest/api/content/{id}/copy`) are v1-only endpoints that return immediately but process asynchronously. The copy endpoint returns a long-running task ID. There is no webhook or callback -- you must poll `/longtask/{taskId}`.

**Why it happens:** Moving page trees or copying with attachments can take significant time server-side. Atlassian chose async processing rather than long-lived HTTP connections.

**Consequences:**
- A `cf pages move` command that returns "success" when the server merely accepted the request -- the actual move may fail later
- Copy operations for pages with many attachments can take minutes; without polling, the CLI has no idea if it succeeded
- The `OptimisticLockException` error has been reported when copying pages that are being edited simultaneously
- Move operations that fail partway can leave page trees in an inconsistent state
- Using `before` or `after` position with a top-level page as target can orphan pages -- they become top-level and invisible in the UI page tree

**Prevention:**
- For `move` and `copy` commands: implement poll-and-wait pattern using `/longtask/{taskId}` endpoint
- Show progress percentage from the long-running task API (it reports `percentageComplete`)
- Default to synchronous behavior (poll until done) with `--async` flag for fire-and-forget
- Handle `OptimisticLockException` with a clear error message suggesting retry
- For copy: document that comments and version history are NOT copied (only content, attachments, labels, restrictions)
- For move: validate the position parameter -- warn if `before`/`after` is used and target has no parent (top-level page). Prefer `append` (make child of target) as the default position
- Handle the `INITIALIZING_TASK` status that appears before actual processing begins -- do not treat it as an error

**Detection:** Copy a page with 50+ attachments and check if the CLI waits for completion or returns immediately with incomplete data.

**Confidence:** HIGH -- confirmed by [Atlassian bulk move documentation](https://confluence.atlassian.com/confkb/bulk-move-pages-using-the-confluence-cloud-api-1540735700.html), [long-running task status announcement](https://community.developer.atlassian.com/t/added-status-message-for-initiating-copy-page-hierarchy-and-long-task-apis/44749), and [developer community discussion on OptimisticLockException](https://community.developer.atlassian.com/t/replacing-a-page-with-copy-single-page-api-fails-with-optimisticlockexception/37622).

---

### Pitfall 4: GoReleaser ldflags Path Must Match Go Module Path Exactly

**What goes wrong:** The ldflags `-X` flag for version injection requires the EXACT Go package path to the variable. The existing code has `Version` in `github.com/sofq/confluence-cli/cmd.Version`. If the GoReleaser config uses `-X main.Version={{.Version}}` (the GoReleaser default), the version will silently remain "dev" in all release binaries.

**Why it happens:** GoReleaser defaults to `-X main.version={{.Version}}` which only works when the version variable is in `package main`. The cf CLI has it in `package cmd` at a different import path.

**Consequences:** Every released binary reports `{"version":"dev"}` instead of the actual version. This breaks version checking, upgrade notifications, and user trust. Since the binary otherwise works fine, this can go undetected for multiple releases.

**Prevention:**
- Use the EXACT ldflags path from the existing code: `-X github.com/sofq/confluence-cli/cmd.Version={{.Version}}`
- The jr reference already does this correctly: `-X github.com/sofq/jira-cli/cmd.Version={{.Version}}`
- Add a CI smoke test that builds with GoReleaser snapshot and verifies `cf version` output is not "dev"
- Keep `CGO_ENABLED=0` as in the jr reference -- the confluence-cli uses no CGO dependencies

**Detection:** After first release, run `cf version` and check if it outputs the tag version or "dev".

**Confidence:** HIGH -- verified by reading `cmd/version.go` and the [GoReleaser ldflags documentation](https://goreleaser.com/cookbooks/using-main.version/).

---

### Pitfall 5: HOMEBREW_TAP_TOKEN Must Be a PAT, Not GITHUB_TOKEN

**What goes wrong:** GoReleaser needs to push formula/cask updates to a separate `homebrew-tap` repository. The default `GITHUB_TOKEN` in GitHub Actions is scoped to only the current repository. Using it for the Homebrew tap push fails silently or with a permission error.

**Why it happens:** GitHub Actions `GITHUB_TOKEN` cannot make changes to other repositories by design. GoReleaser's brew section commits to `sofq/homebrew-tap` which requires cross-repo write access.

**Consequences:** The GoReleaser release job succeeds (binaries are published) but the Homebrew formula is never updated. Users running `brew upgrade cf` get stale versions indefinitely. The error is buried in GoReleaser output and easy to miss.

**Prevention:**
- Create a fine-grained Personal Access Token (PAT) with `contents: write` scope on the `homebrew-tap` repository
- Store as `HOMEBREW_TAP_TOKEN` secret in the confluence-cli repository
- The same token works for both Homebrew and Scoop (as the jr reference shows)
- Consider creating a dedicated bot account for the PAT to avoid coupling to a personal account
- Add a post-release check that verifies the tap was actually updated

**Detection:** After first release, check if `homebrew-tap` repo received a commit with the new formula.

**Confidence:** HIGH -- confirmed by [GoReleaser GitHub Actions documentation](https://goreleaser.com/ci/actions/) and the [goreleaser community discussion on Homebrew tokens](https://github.com/orgs/goreleaser/discussions/4926).

---

### Pitfall 6: npm Classic Tokens Deprecated -- OIDC Trusted Publishing Required

**What goes wrong:** As of December 2025, npm permanently deprecated Classic Tokens. The old pattern of storing an `NPM_TOKEN` secret and using it with `npm publish` no longer works. Publishing will fail with authentication errors.

**Why it happens:** npm migrated to OIDC-based trusted publishing to eliminate long-lived secrets and enable provenance attestation. This requires specific GitHub Actions configuration: `id-token: write` permission, an `environment` declaration, and npm CLI 11.5.1+.

**Consequences:** The release workflow's `npm-publish` job fails on every release. Since it runs with `continue-on-error: true` (as in the jr reference), this failure is silent -- no npm package is ever published.

**Prevention:**
- The first version of the package MUST be published manually or using a granular access token -- OIDC trusted publishing can only be configured after the package exists on npmjs.com
- Configure trusted publishing on npmjs.com for the `confluence-cf` package (link GitHub repo + workflow + environment)
- Set `permissions.id-token: write` on the npm-publish job
- Declare `environment: npm-publish` in the job
- Do NOT set `NODE_AUTH_TOKEN` environment variable -- it conflicts with OIDC authentication
- Ensure `node-version: 24` (includes npm 11.5.1+)
- The `--provenance` flag is automatically applied with trusted publishing, but adding it explicitly is safest
- Ensure `repository.url` in package.json exactly matches the GitHub repo URL
- Trusted publishing only works on cloud-hosted runners, not self-hosted
- Test with a pre-release tag first (`v0.0.1-rc.1`) before the real release

**Detection:** Check npm publish job logs in GitHub Actions after first release.

**Confidence:** HIGH -- confirmed by [npm trusted publishing docs](https://docs.npmjs.com/trusted-publishers/), [GitHub changelog announcement](https://github.blog/changelog/2025-07-31-npm-trusted-publishing-with-oidc-is-generally-available/), and [practical gotchas blog post](https://philna.sh/blog/2026/01/28/trusted-publishing-npm/).

---

## Moderate Pitfalls

### Pitfall 7: Archive Endpoint Has Free-Tier and Batch Limits

**What goes wrong:** The `POST /wiki/rest/api/content/archive` endpoint has several undocumented constraints:
- Free edition tenants: limited to 1 page per request
- Standard/Premium: up to 300 pages per request
- Only one archival job can run per tenant at a time -- concurrent requests fail
- Requires "Archive" permission per page per space
- Setting status to "archived" via PUT returns 200 but silently does nothing

**Prevention:**
- Batch archive requests with configurable chunk size (default: 1 for safety)
- Check for "archival job already running" error and implement backoff/queue
- The `archive` command should support both single page and batch (from stdin) modes
- Validate permissions before attempting bulk archive
- Use the dedicated archive endpoint, not `PUT /pages/{id}` with `status: archived` (the latter silently fails)

**Confidence:** HIGH -- documented in the [Confluence REST API v1 archive endpoint](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content/) and confirmed by [CONFCLOUD-72078 discussion](https://jira.atlassian.com/browse/CONFCLOUD-72078).

---

### Pitfall 8: Export Has No Official PDF API for Confluence Cloud

**What goes wrong:** Unlike Data Center/Server, Confluence Cloud has NO REST API endpoint for exporting a single page as PDF. The only official PDF export is space-level export through the UI. Developers attempting to build `cf pages export --format pdf` will find no endpoint to call.

**Prevention:**
- The `export` command should focus on formats the API supports: storage format (XHTML), view format (rendered HTML), and atlas_doc_format (ADF)
- Do NOT promise PDF export -- it requires third-party apps (Scroll PDF Exporter) or browser automation
- For storage format export: use `GET /pages/{id}?body-format=storage` (v2) or `GET /content/{id}?expand=body.storage` (v1)
- For historical version export: use `GET /content/{id}/version/{versionNumber}?expand=content.body.storage` (v1 only)
- Document the format limitation clearly in help text

**Confidence:** HIGH -- confirmed by [Atlassian support documentation](https://support.atlassian.com/confluence/kb/rest-api-to-export-and-download-a-page-in-pdf-format/) and [CONFCLOUD-61557 feature request](https://jira.atlassian.com/browse/CONFCLOUD-61557).

---

### Pitfall 9: Version Diff Requires v1 API for Historical Body Content

**What goes wrong:** The `diff` command needs to fetch the body content of specific historical page versions. The v2 API supports `GET /pages/{id}/versions` for listing versions and `GET /pages/{page-id}/versions/{version-number}` for version details, but body content retrieval for specific historical versions is more reliable through the v1 endpoint: `GET /content/{id}/version/{versionNumber}?expand=content.body.storage`.

**Why it happens:** The v2 page version endpoints were designed for version metadata (author, timestamp, message). Historical body retrieval with format specification is more established through the v1 expansion system.

**Prevention:**
- Use the v2 endpoint for listing versions (metadata): `GET /pages/{id}/versions` with `body-format=storage`
- Fallback to v1 endpoint for version body retrieval if v2 does not return body: `GET /wiki/rest/api/content/{id}/version/{versionNumber}?expand=content.body.storage`
- The diff should compare storage format (XHTML) since that is the canonical representation
- Consider a simple line-based diff on the storage format rather than structural XML diff (simpler, more useful for AI agents)
- Handle the case where a version has no body (deleted and recreated pages)
- The `searchV1Domain` helper already exists in the codebase for building v1 URLs

**Confidence:** MEDIUM -- the v2 version endpoints exist in the generated schema (confirmed in `schema_data.go` at lines 5025-5093) with `body-format` parameter, but the body content behavior for historical versions needs live testing. The v1 approach is the safe fallback based on [community reports](https://community.atlassian.com/forums/Confluence-questions/Confluence-API-get-page-content-from-historical-versions/qaq-p/1398857).

---

### Pitfall 10: VitePress Base Path Breaks Nested Page Links on GitHub Pages

**What goes wrong:** When deploying to GitHub Pages at `sofq.github.io/confluence-cli/`, the VitePress `base` config must be `/confluence-cli/`. However, nested pages (e.g., `/commands/pages-get`) have a known issue where the base path is missing from generated links, causing 404s in production even though development mode works fine.

**Why it happens:** VitePress generates relative links that work in dev (where base is `/`) but break in production (where base is `/confluence-cli/`). The auto-generated sidebar from `sidebar-commands.json` compounds this if links don't start with `/`.

**Prevention:**
- Set `base: '/confluence-cli/'` in `.vitepress/config.ts`
- Ensure ALL sidebar links start with `/` (e.g., `/commands/pages` not `commands/pages`) -- missing the leading `/` breaks prev/next navigation even when sidebar navigation works
- The jr reference sets `ignoreDeadLinks: true` because generated command pages contain external URLs (Atlassian doc URLs) that VitePress mistakes for relative links -- copy this pattern
- Add a `.nojekyll` file in the output directory to prevent GitHub Pages from running Jekyll processing, which strips files beginning with `_` or `.` (VitePress uses `_` prefixed directories)
- Test the production build locally: `npx vitepress build && npx vitepress preview` before deploying
- The `docs-build` CI job catches this -- ensure it runs on PRs, not just main

**Confidence:** HIGH -- confirmed by [VitePress issue #3243](https://github.com/vuejs/vitepress/issues/3243) on sidebar link formatting and [GitHub Pages Jekyll processing documentation](https://docs.github.com/en/pages/getting-started-with-github-pages/about-github-pages#static-site-generators).

---

### Pitfall 11: npm Postinstall Binary Download Fails Silently on Restricted Environments

**What goes wrong:** The npm package uses a `postinstall` script to download the Go binary from GitHub Releases. This fails in environments where: (1) `--ignore-scripts` is set (common security recommendation), (2) corporate proxies block GitHub, (3) the binary architecture is not supported (e.g., `linux/s390x`), or (4) GitHub rate-limits unauthenticated downloads.

**Prevention:**
- The `postinstall` pattern is the proven approach (used by esbuild, turbo, etc.) -- keep it
- Handle download failure gracefully: print a clear error with manual install instructions rather than `process.exit(1)`
- Support `CF_BINARY_PATH` environment variable as an override for pre-installed binaries
- The smoke test in CI (`npm pack && npm install`) catches MODULE_NOT_FOUND errors -- keep this pattern from jr
- Set executable bits after extraction: `fs.chmodSync(binPath, 0o755)` -- archive extraction strips the executable bit
- On macOS arm64: binaries may need code signing. Cross-compiled binaries from Linux CI may lack it. GoReleaser with `CGO_ENABLED=0` produces binaries that run unsigned on arm64, so this is only a concern with CGO
- Map `process.platform` and `process.arch` correctly: `win32` -> `windows`, `x64` -> `amd64`, `arm64` -> `arm64`

**Confidence:** HIGH -- confirmed by examining the jr reference `npm/install.js` and [community discussion on binary distribution](https://github.com/evanw/esbuild/issues/789).

---

### Pitfall 12: GitHub Actions Pages Deployment Needs Specific Permission Trio

**What goes wrong:** The GitHub Pages deployment workflow requires three specific permissions that are non-obvious: `contents: read`, `pages: write`, and `id-token: write`. Missing any one causes a cryptic failure. The `id-token: write` is particularly confusing because it seems unrelated to Pages -- it is needed for OIDC verification that the workflow is authorized to deploy.

**Prevention:**
- Copy the exact permission block from the jr reference docs workflow:
  ```yaml
  permissions:
    contents: read
    pages: write
    id-token: write
  ```
- Enable GitHub Pages in repository settings with "GitHub Actions" as the source (not "Deploy from a branch")
- Set `concurrency.group: pages` with `cancel-in-progress: false` (let in-progress deploys finish)
- The `environment: github-pages` declaration is also required for the deployment to be tracked
- Use path triggers that include both Go source and website source:
  ```yaml
  paths:
    - 'cmd/**'
    - 'gen/**'
    - 'spec/**'
    - 'website/**'
    - 'Makefile'
  ```

**Confidence:** HIGH -- confirmed by [GitHub Actions deploy-pages documentation](https://github.com/actions/deploy-pages) and the working jr reference workflow.

---

### Pitfall 13: Points-Based Rate Limiting Coming for OAuth2 3LO Apps

**What goes wrong:** Starting March 2, 2026, Atlassian is rolling out points-based quota rate limits for Confluence Cloud. Each API call consumes points based on complexity. OAuth2 3LO apps are affected; API token-based traffic is NOT (yet). Workflow commands that chain multiple API calls (move -> restrict -> comment) could hit rate limits faster than expected.

**Prevention:**
- API token (basic auth) traffic is currently exempt -- document this for users who hit rate limits
- Implement rate limit response handling: check for `429 Too Many Requests` and `Retry-After` header
- The `watch` command's polling loop is the highest risk for rate limiting -- ensure configurable poll intervals
- Batch operations should include inter-request delays
- This is a gradual rollout affecting "a small percentage of apps" initially

**Confidence:** MEDIUM -- confirmed by [Atlassian rate limiting documentation](https://developer.atlassian.com/cloud/confluence/rate-limiting/) and [blog announcement](https://www.atlassian.com/blog/platform/evolving-api-rate-limits). Exact impact on CLI-style usage is unclear since the points model targets Connect/Forge apps primarily.

---

### Pitfall 14: Content Body Convert API Deprecation Affects Export Format Conversion

**What goes wrong:** The synchronous `POST /wiki/rest/api/contentbody/convert/{to}` endpoint (used to convert between storage, view, export_view, and editor formats) is deprecated with an extended deadline of August 5, 2026. The replacement is an async endpoint. If the `export` command relies on format conversion, it needs the async pattern.

**Prevention:**
- For the `export` command: prefer fetching the body in the desired format directly (`body-format` parameter on v2 endpoints) rather than fetching storage format and converting
- If conversion is needed, use the async endpoint and poll for completion
- The async convert endpoint does NOT support `wiki` to `storage` conversion (only the deprecated sync endpoint does)
- Keep the existing pattern of passing through raw storage format rather than converting

**Confidence:** HIGH -- confirmed by [Atlassian developer community report](https://community.developer.atlassian.com/t/new-async-convert-content-body-api-does-not-support-wiki-conversion-to-storage-but-old-api-does/87658) and [Confluence Cloud changelog](https://developer.atlassian.com/cloud/confluence/changelog/).

---

## Minor Pitfalls

### Pitfall 15: PyPI Trusted Publishing Requires Environment Configuration

**What goes wrong:** PyPI trusted publishing requires creating a "trusted publisher" on pypi.org BEFORE the first publish attempt. Without this, the OIDC token exchange fails with a generic authentication error. The workflow file name, environment name, and repository must match exactly -- a typo anywhere causes a 403.

**Prevention:**
- Create the trusted publisher on pypi.org: specify GitHub repo, workflow file name, and environment name (`pypi-publish`)
- The `environment: pypi-publish` declaration in the workflow MUST match the trusted publisher config exactly
- Use `pypi` as the OIDC audience (not `testpypi` -- that is for TestPyPI only)
- Test with TestPyPI first using the same OIDC flow (configure a separate trusted publisher for testpypi.org)
- The `pypa/gh-action-pypi-publish` action handles the OIDC exchange automatically
- Removing a PyPI project maintainer does NOT remove their registered trusted publishers -- review after offboarding

**Confidence:** HIGH -- confirmed by [PyPI trusted publishing docs](https://docs.pypi.org/trusted-publishers/) and [troubleshooting guide](https://docs.pypi.org/trusted-publishers/troubleshooting/).

---

### Pitfall 16: GoReleaser Docker Manifest Requires Buildx and GHCR Login

**What goes wrong:** Multi-architecture Docker images require Docker Buildx and explicit GHCR login. Without Buildx, `docker manifest` commands fail. Without `packages: write` permission, GHCR push fails. Snapshot builds cannot create manifests -- only individual platform images.

**Prevention:**
- Include `docker/setup-buildx-action` before GoReleaser
- Include `docker/login-action` with GHCR credentials
- Set `permissions.packages: write` on the release job
- Use `Dockerfile.goreleaser` (minimal, FROM distroless/static, COPY binary) as in the jr reference
- Pin the distroless base image with a SHA256 digest for reproducibility
- Test with `goreleaser release --snapshot --clean` locally -- expect separate images per platform, not manifests

**Confidence:** HIGH -- confirmed by [GoReleaser multi-platform Docker cookbook](https://goreleaser.com/cookbooks/multi-platform-docker-images/) and the working jr reference workflow.

---

### Pitfall 17: Generated Sidebar JSON Must Be Created Before VitePress Build

**What goes wrong:** The VitePress config imports `sidebar-commands.json` which is generated by `go run ./cmd/gendocs/...`. If the docs CI runs `npm ci && vitepress build` without running the Go docs generator first, the sidebar will be empty (the config has a try/catch fallback to `[]`).

**Prevention:**
- The Makefile must chain: `docs-generate` -> `docs-build` (as the jr reference does: `make docs-build` which depends on `docs-generate`)
- The GitHub Actions docs workflow must include `setup-go` AND run the generator before `vitepress build`
- The `gendocs` tool should generate both sidebar JSON and individual command markdown files
- Include `cmd/**` and `gen/**` in the docs workflow path triggers so command changes trigger docs rebuilds

**Confidence:** HIGH -- verified from the jr reference Makefile and the `docs.yml` workflow which includes both `setup-go` and `make docs-build`.

---

### Pitfall 18: Scoop Bucket Uses Same Token as Homebrew Tap

**What goes wrong:** The GoReleaser config for Scoop also needs cross-repo push access to `sofq/scoop-bucket`. Developers often set up `HOMEBREW_TAP_TOKEN` but forget that Scoop uses the same mechanism and needs the same PAT to have write access to the Scoop bucket repo.

**Prevention:**
- Reuse the same `HOMEBREW_TAP_TOKEN` PAT for both Homebrew and Scoop (as the jr reference does)
- Ensure the PAT has `contents: write` on BOTH `homebrew-tap` and `scoop-bucket` repositories
- Or use a single PAT with org-wide repo access if both repos are in the same org

**Confidence:** HIGH -- verified from the jr `.goreleaser.yml` where both `brews` and `scoops` sections reference `HOMEBREW_TAP_TOKEN`.

---

### Pitfall 19: gosec Exclusions Needed for Generated Code

**What goes wrong:** The security scanning workflow with `gosec` will flag issues in auto-generated code (e.g., unchecked errors in generated commands, file permission warnings). Without exclusions, every CI run produces noise that masks real security issues.

**Prevention:**
- Exclude generated code directories: `-exclude-dir=cmd/generated`
- Exclude common false positives: `-exclude=G104,G301,G304,G306` (as the jr reference does)
- Set `GOFLAGS: -buildvcs=false` to avoid VCS-related gosec failures in CI
- Run `govulncheck` separately for dependency vulnerability scanning

**Confidence:** HIGH -- verified from the jr `security.yml` workflow.

---

### Pitfall 20: Python Package Version Must Be Injected at Release Time

**What goes wrong:** The `pyproject.toml` has `version = "0.0.0"` as a placeholder. The release workflow must `sed` the version from the git tag into `pyproject.toml` before building the wheel. If this step is missing or the sed pattern does not match, PyPI rejects the upload as a duplicate of version 0.0.0.

**Prevention:**
- Use the exact sed pattern from the jr reference: `sed -i "s/^version = .*/version = \"$VERSION\"/" python/pyproject.toml`
- The `VERSION` must strip the `v` prefix: `VERSION="${TAG#v}"`
- Test locally: `cd python && python -m build` should produce a wheel with the correct version
- The smoke test in CI should verify the version is set correctly

**Confidence:** HIGH -- verified from the jr `release.yml` workflow.

---

### Pitfall 21: Confluence Storage Format CDATA Gotchas in Diff Output

**What goes wrong:** When building the `diff` command output, the Confluence storage format (XHTML) contains CDATA sections within macro bodies. CDATA sections cannot contain `]]>` sequences and Confluence silently mangles them by adding a space (`]] >`). A naive diff will show spurious differences on CDATA boundaries.

**Prevention:**
- Treat the storage format as opaque XML -- do not attempt to "normalize" CDATA sections before diffing
- Use a line-based diff approach that agents can parse, not an XML-aware structural diff
- Document that the diff is text-based on storage format, not semantic
- Handle macros like `ac:structured-macro` with `ac:plain-text-body` children that contain the actual content in CDATA -- these are the most diff-relevant parts

**Confidence:** MEDIUM -- based on [Confluence storage format documentation](https://confluence.atlassian.com/doc/confluence-storage-format-790796544.html) and [CONFSERVER-81553 CDATA issue](https://jira.atlassian.com/browse/CONFSERVER-81553). The practical impact on diff output needs testing.

---

### Pitfall 22: Spec Drift Workflow Requires Careful URL Matching for cf

**What goes wrong:** The spec-drift workflow for cf must use the Confluence v2 API spec URL, not the Jira spec URL. Copying from jr and forgetting to update the URL means the drift checker silently passes every time (it compares the old Confluence spec against itself since the Jira spec download would fail or be stored incorrectly).

**Prevention:**
- Use the correct spec URL: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
- Update file paths: `spec/confluence-v2.json` not `spec/jira-v3.json`
- The spec-auto-release workflow that tags on merge also needs updating for cf's branch naming
- Update the PR auto-merge branch filter to `auto/spec-update` (matching the spec-drift workflow output)

**Confidence:** HIGH -- verified by checking the existing `spec/confluence-v2.json` file in the codebase and the jr reference `spec-drift.yml` workflow.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| `diff` command | Historical version body may need v1 API fallback (Pitfall 9); CDATA in storage format (Pitfall 21) | Try v2 `body-format=storage` first, fallback to v1 expansion; use line-based diff |
| `workflow move` | Async operation, must poll long-running task; top-level orphaning risk (Pitfall 3) | Implement poll loop with `--async` escape hatch; default position to `append` |
| `workflow copy` | Comments and version history NOT copied; OptimisticLockException possible (Pitfall 3) | Document limitations; handle 409 with retry; poll `/longtask/{taskId}` |
| `workflow restrict` | v1-only API; PUT replaces all restrictions; self-lockout risk (Pitfall 2) | GET-merge-PUT pattern; default `--add` not `--replace`; auto-include current user |
| `workflow archive` | Free-tier limit of 1 page; one job per tenant; PUT status silently fails (Pitfall 7) | Use dedicated archive endpoint; chunk batches; backoff on concurrent job error |
| `workflow comment` | Version number lag on rapid operations (Pitfall 1) | Use response version, not re-fetched version |
| `export` command | No PDF API; format conversion API being deprecated (Pitfalls 8, 14) | Export storage/view/ADF formats only; skip PDF |
| `preset/template` built-ins | No pitfall -- straightforward feature using existing internal packages | Standard implementation |
| GoReleaser setup | ldflags path mismatch (Pitfall 4); CGO must be disabled | Match exact module path: `github.com/sofq/confluence-cli/cmd.Version` |
| Homebrew/Scoop | PAT required for cross-repo push (Pitfalls 5, 18) | Single PAT with write access to tap + bucket repos |
| npm publish | Classic tokens dead; OIDC required; first publish must be manual (Pitfall 6) | Manual first publish, then configure trusted publisher on npmjs.com |
| PyPI publish | Trusted publisher must exist before first release; env name must match (Pitfall 15) | Create on pypi.org before tagging v1.2.0 |
| VitePress docs | Base path breaks nested links; .nojekyll needed (Pitfall 10); sidebar must be pre-generated (Pitfall 17) | Set `base`, ensure links start with `/`, add `.nojekyll`, chain `docs-generate` -> `docs-build` |
| GitHub Actions CI | Permission trios for Pages and GHCR (Pitfalls 12, 16) | Copy exact permission blocks from jr reference |
| Security scanning | gosec noise on generated code (Pitfall 19) | Exclude `cmd/generated` directory |
| Spec drift | Wrong URL if copied from jr (Pitfall 22) | Use Confluence v2 OpenAPI spec URL |
| Rate limiting | Points-based quotas rolling out for OAuth2 3LO (Pitfall 13) | Handle 429 + Retry-After; note basic auth exemption |

---

## Sources

### Confluence API
- [Version number lag in v2 API](https://community.developer.atlassian.com/t/lag-in-updating-page-version-number-v2-confluence-rest-api/68821)
- [Page restrictions via v2 API (not available)](https://community.developer.atlassian.com/t/page-restrictions-via-the-v2-api/93094)
- [Inherited restrictions not returned by API](https://support.atlassian.com/confluence/kb/confluence-get-page-restrictions-api-doesnt-display-inherited-restrictions/)
- [Restriction update gotchas (v1)](https://community.developer.atlassian.com/t/update-content-restrictions-with-api-v1/88400)
- [Bulk move pages documentation](https://confluence.atlassian.com/confkb/bulk-move-pages-using-the-confluence-cloud-api-1540735700.html)
- [Copy page hierarchy long-running task status](https://community.developer.atlassian.com/t/added-status-message-for-initiating-copy-page-hierarchy-and-long-task-apis/44749)
- [Copy page OptimisticLockException](https://community.developer.atlassian.com/t/replacing-a-page-with-copy-single-page-api-fails-with-optimisticlockexception/37622)
- [Archive page API request (CONFCLOUD-72078)](https://jira.atlassian.com/browse/CONFCLOUD-72078)
- [PDF export feature request (CONFCLOUD-61557)](https://jira.atlassian.com/browse/CONFCLOUD-61557)
- [Historical version body retrieval](https://community.atlassian.com/forums/Confluence-questions/Confluence-API-get-page-content-from-historical-versions/qaq-p/1398857)
- [Content body convert async deprecation](https://community.developer.atlassian.com/t/new-async-convert-content-body-api-does-not-support-wiki-conversion-to-storage-but-old-api-does/87658)
- [Confluence Cloud changelog](https://developer.atlassian.com/cloud/confluence/changelog/)
- [Points-based rate limiting](https://developer.atlassian.com/cloud/confluence/rate-limiting/)
- [Rate limit evolution blog](https://www.atlassian.com/blog/platform/evolving-api-rate-limits)
- [Content restrictions v1 API](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content-restrictions/)
- [PDF export limitations](https://support.atlassian.com/confluence/kb/rest-api-to-export-and-download-a-page-in-pdf-format/)
- [Confluence storage format](https://confluence.atlassian.com/doc/confluence-storage-format-790796544.html)
- [CDATA issue in storage format (CONFSERVER-81553)](https://jira.atlassian.com/browse/CONFSERVER-81553)
- [Confluence v1 deprecation RFC](https://community.developer.atlassian.com/t/rfc-19-deprecation-of-confluence-cloud-rest-api-v1-endpoints/71752)

### GoReleaser
- [GoReleaser ldflags cookbook](https://goreleaser.com/cookbooks/using-main.version/)
- [GoReleaser CGO limitations](https://goreleaser.com/limitations/cgo/)
- [GoReleaser GitHub Actions docs](https://goreleaser.com/ci/actions/)
- [Homebrew tokens discussion](https://github.com/orgs/goreleaser/discussions/4926)
- [Multi-platform Docker images](https://goreleaser.com/cookbooks/multi-platform-docker-images/)
- [GoReleaser Go builds customization](https://goreleaser.com/customization/builds/go/)

### npm/PyPI Publishing
- [npm trusted publishing docs](https://docs.npmjs.com/trusted-publishers/)
- [npm OIDC GA announcement](https://github.blog/changelog/2025-07-31-npm-trusted-publishing-with-oidc-is-generally-available/)
- [Things to do for npm trusted publishing](https://philna.sh/blog/2026/01/28/trusted-publishing-npm/)
- [PyPI trusted publishing docs](https://docs.pypi.org/trusted-publishers/)
- [PyPI trusted publishing troubleshooting](https://docs.pypi.org/trusted-publishers/troubleshooting/)
- [Publishing Go binaries on npm (Sentry)](https://sentry.engineering/blog/publishing-binaries-on-npm)
- [esbuild platform-specific binary strategy](https://github.com/evanw/esbuild/issues/789)

### VitePress
- [VitePress sidebar link formatting #3243](https://github.com/vuejs/vitepress/issues/3243)
- [VitePress deploy documentation](https://vitepress.dev/guide/deploy)
- [GitHub Pages Jekyll processing](https://docs.github.com/en/pages/getting-started-with-github-pages/about-github-pages#static-site-generators)

### GitHub Actions
- [deploy-pages documentation](https://github.com/actions/deploy-pages)
- [pypa/gh-action-pypi-publish](https://github.com/pypa/gh-action-pypi-publish)

### Reference Implementation
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.goreleaser.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/release.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/ci.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/docs.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/security.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/spec-drift.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/.github/workflows/spec-auto-release.yml`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/npm/install.js`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/python/jira_jr/__init__.py`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/website/.vitepress/config.ts`
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/Dockerfile.goreleaser`
