---
phase: 17-release-infrastructure
plan: 03
subsystem: infra
tags: [github-actions, ci-cd, goreleaser, dependabot, security, docs, spec-drift]

# Dependency graph
requires:
  - phase: 17-01
    provides: GoReleaser config, golangci-lint config, Makefile targets
  - phase: 17-02
    provides: npm scaffold (package.json, install.js), Python scaffold (pyproject.toml, __init__.py)
provides:
  - CI workflow with build, test, lint, npm/pypi smoke tests, docs build, integration
  - Release workflow with GoReleaser, npm publish (OIDC), PyPI publish (OIDC)
  - Security workflow with gosec and govulncheck
  - Spec drift detection with auto-PR and auto-merge
  - Spec auto-release tagging on spec-update PR merge
  - Docs deployment to GitHub Pages
  - Dependabot auto-merge and dependency config
affects: [17-04, release-process, documentation]

# Tech tracking
tech-stack:
  added: [github-actions, codecov, gosec, govulncheck, peter-evans/create-pull-request, pypa/gh-action-pypi-publish]
  patterns: [SHA-pinned actions for supply chain security, OIDC token publishing, auto-merge dependency PRs]

key-files:
  created:
    - .github/workflows/ci.yml
    - .github/workflows/release.yml
    - .github/workflows/security.yml
    - .github/workflows/spec-drift.yml
    - .github/workflows/spec-auto-release.yml
    - .github/workflows/docs.yml
    - .github/workflows/dependabot-auto-merge.yml
    - .github/dependabot.yml
  modified: []

key-decisions:
  - "All actions SHA-pinned identically to jr reference implementation for supply chain security"
  - "Confluence spec URL uses openapi-v2.v3.json endpoint for drift detection"
  - "Docs workflow uses broader internal/** path trigger instead of jr's internal/errors/** and skill/**"

patterns-established:
  - "SHA-pinned GitHub Actions: all external actions use commit SHA with version comment"
  - "OIDC publishing: npm and PyPI publish use id-token: write with continue-on-error: true"
  - "Spec drift auto-merge: daily check + auto-PR + auto-release on merge with label"

requirements-completed: [CICD-01, CICD-02, CICD-03, CICD-04, CICD-05, CICD-06, CICD-07]

# Metrics
duration: 2min
completed: 2026-03-28
---

# Phase 17 Plan 03: GitHub Actions Workflows Summary

**7 GitHub Actions workflows and Dependabot config covering CI, release, security, spec drift, docs, and dependency management -- all SHA-pinned and adapted from jr reference**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-28T17:41:41Z
- **Completed:** 2026-03-28T17:44:10Z
- **Tasks:** 2
- **Files created:** 8

## Accomplishments
- Complete CI pipeline with 6 jobs: test (coverage), lint, npm smoke test, PyPI smoke test, docs build, integration
- Release pipeline triggered on tag push with GoReleaser, npm OIDC publish, PyPI OIDC publish
- Security pipeline with gosec and govulncheck on push/PR and weekly schedule
- Spec drift detection downloading Confluence OpenAPI spec daily with auto-PR, auto-merge, and auto-release
- VitePress docs deployment to GitHub Pages with proper permissions and concurrency
- Dependabot configured for gomod and github-actions weekly updates with auto-merge workflow

## Task Commits

Each task was committed atomically:

1. **Task 1: Create CI, release, and security workflows** - `3809fe6` (feat)
2. **Task 2: Create spec-drift, docs, dependabot workflows and config** - `6521a86` (feat)

## Files Created/Modified
- `.github/workflows/ci.yml` - CI pipeline with test, lint, smoke tests, docs build, integration jobs
- `.github/workflows/release.yml` - Release pipeline with GoReleaser, npm/PyPI OIDC publishing
- `.github/workflows/security.yml` - Security scanning with gosec and govulncheck
- `.github/workflows/spec-drift.yml` - Daily Confluence OpenAPI spec drift check with auto-PR
- `.github/workflows/spec-auto-release.yml` - Auto-tag patch version on spec-update PR merge
- `.github/workflows/docs.yml` - VitePress docs build and GitHub Pages deployment
- `.github/workflows/dependabot-auto-merge.yml` - Auto-merge for dependabot PRs
- `.github/dependabot.yml` - Dependabot config for gomod and github-actions ecosystems

## Decisions Made
- All actions SHA-pinned identically to jr reference implementation -- supply chain security
- Confluence spec URL uses `openapi-v2.v3.json` endpoint for spec drift detection
- Docs workflow uses broader `internal/**` path trigger instead of jr's `internal/errors/**` and `skill/**` (cf has no skill/ directory and broader internal coverage is appropriate)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All CI/CD workflows in place for future releases
- First npm/PyPI publish will require manual steps (OIDC not configured until first manual publish)
- Ready for Phase 17-04 (documentation site and project files)

## Self-Check: PASSED

All 8 created files verified present. Both task commits (3809fe6, 6521a86) verified in git log.

---
*Phase: 17-release-infrastructure*
*Completed: 2026-03-28*
