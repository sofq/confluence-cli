# Phase 17: Release Infrastructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 17-release-infrastructure
**Areas discussed:** Package naming, Distribution repos, First-publish strategy, README scope

---

## Package Naming

| Option | Description | Selected |
|--------|-------------|----------|
| confluence-cf | Mirrors jira-jr pattern exactly, confirmed available on npm and PyPI | |
| confluence-cli | More descriptive, breaks naming pattern | initially selected |
| confluence-cf | After discovering confluence-cli taken on PyPI, reverted to pattern match | ✓ |

**User's choice:** Initially chose `confluence-cli`, but after checking availability found PyPI name taken. Switched to `confluence-cf`.
**Notes:** Verified via web search that `confluence-cli` exists on PyPI (v0.7.2, 2022). `confluence-cf` confirmed available on both npm and PyPI.

---

## Distribution Repos

| Option | Description | Selected |
|--------|-------------|----------|
| Share repos | Add cf to existing sofq/homebrew-tap and sofq/scoop-bucket | ✓ |
| Separate repos | Create dedicated sofq/homebrew-cf and sofq/scoop-cf | |

**User's choice:** Share repos with jr
**Notes:** Standard practice for orgs with multiple CLIs

---

## First-publish Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| v0.1.0 | New project signal, iterate from there | ✓ |
| v1.2.0 | Match milestone version | |
| v1.0.0 | Clean semver start | |

**User's choice:** v0.1.0
**Notes:** npm/PyPI OIDC needs manual first publish before automated workflows work

---

## README Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal | Install + one-liner + badges + link to docs | |
| Moderate | Install + quick start + feature list + docs link | |
| Comprehensive | Full standalone matching jr README structure | ✓ |

**User's choice:** Mimic jr README exactly — comprehensive standalone doc
**Notes:** User explicitly said "mimic exactly jira-jr". Read full jr README to understand structure.

---

## Claude's Discretion

- Exact README examples adapted from jr to cf context
- gosec exclusion codes
- Codecov token setup
- Integration test env var names
- Spec URL for Confluence OpenAPI

## Deferred Ideas

None — discussion stayed within phase scope.
