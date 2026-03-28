# Requirements: Confluence CLI (`cf`)

**Defined:** 2026-03-28
**Core Value:** Give AI agents reliable, structured JSON access to Confluence content through a CLI

## v1.2 Requirements

Requirements for v1.2 Workflow, Parity & Release Infrastructure milestone. Each maps to roadmap phases.

### Version Diff

- [x] **DIFF-01**: User can compare two page versions and see structured JSON diff output
- [x] **DIFF-02**: User can filter version diffs by time range using `--since` with human-friendly durations
- [x] **DIFF-03**: User can specify `--from` and `--to` version numbers for explicit comparison

### Workflow Commands

- [x] **WKFL-01**: User can move a page to a different parent or space via `workflow move`
- [x] **WKFL-02**: User can copy a page with options (attachments, permissions, labels) via `workflow copy`
- [x] **WKFL-03**: User can publish a draft page via `workflow publish`
- [x] **WKFL-04**: User can add a plain-text comment to a page via `workflow comment`
- [x] **WKFL-05**: User can view, add, and remove page restrictions via `workflow restrict`
- [x] **WKFL-06**: User can archive pages via `workflow archive`

### Content Utilities

- [x] **CONT-01**: User can list all available presets (built-in + user) via `preset list`
- [x] **CONT-02**: CLI ships 7 built-in presets (brief, titles, agent, tree, meta, search, diff)
- [x] **CONT-03**: CLI ships 6 built-in templates (blank, meeting-notes, decision, runbook, retrospective, adr)
- [x] **CONT-04**: User can inspect a template definition via `templates show <name>`
- [x] **CONT-05**: User can create a template from an existing page via `templates create --from-page`
- [x] **CONT-06**: User can export page body in requested format via `export` command
- [x] **CONT-07**: User can recursively export a page tree as NDJSON via `export --tree`

### Internal Packages

- [x] **UTIL-01**: JSON output uses `MarshalNoEscape()` to prevent HTML entity corruption in XHTML content
- [x] **UTIL-02**: Duration parsing supports human-friendly format (2h, 1d, 1w) with calendar time conventions
- [x] **UTIL-03**: Preset resolution follows three-tier lookup: profile > user file > built-in

### CI/CD & Release

- [ ] **CICD-01**: GitHub Actions CI pipeline runs build, test, lint on push/PR to main
- [ ] **CICD-02**: GitHub Actions release pipeline builds cross-platform binaries via GoReleaser on tag push
- [ ] **CICD-03**: GitHub Actions security pipeline runs gosec + govulncheck weekly and on push
- [ ] **CICD-04**: GitHub Actions docs pipeline builds and deploys VitePress site to GitHub Pages
- [ ] **CICD-05**: Spec drift detection runs daily, auto-regenerates commands, creates PR
- [ ] **CICD-06**: Auto-release workflow tags and releases when spec-update PR merges
- [ ] **CICD-07**: Dependabot configured for Go modules and GitHub Actions weekly updates
- [ ] **CICD-08**: GoReleaser produces binaries for linux/darwin/windows (amd64/arm64) + Docker images
- [ ] **CICD-09**: npm package scaffold with postinstall binary download
- [ ] **CICD-10**: Python package scaffold with binary wrapper

### Documentation

- [ ] **DOCS-01**: README.md with install methods, quick start, key features, agent integration guide
- [ ] **DOCS-02**: LICENSE file (Apache 2.0)
- [ ] **DOCS-03**: SECURITY.md with vulnerability reporting policy
- [ ] **DOCS-04**: VitePress documentation site with guide pages and auto-generated command reference
- [ ] **DOCS-05**: `gendocs` binary generates VitePress sidebar JSON and per-command docs from Cobra tree

### Project Config

- [ ] **CONF-01**: `.golangci.yml` with standard linters and errcheck exclusions
- [ ] **CONF-02**: Comprehensive `.gitignore` covering binaries, IDE files, docs output, env files
- [ ] **CONF-03**: Makefile extended with `lint`, `docs-generate`, `docs-dev`, `docs-build`, `spec-update` targets

### Schema Integration

- [x] **SCHM-01**: All new commands (diff, workflow, export, preset) registered in `cf schema` output
- [x] **SCHM-02**: Schema ops aggregated in `schema_cmd.go` for agent discoverability

## Future Requirements

Deferred to future milestone. Tracked but not in current roadmap.

### Advanced Workflow

- **WKFL-07**: User can restore a previous page version via `workflow restore`
- **WKFL-08**: User can bulk move multiple pages in a single operation

### Advanced Export

- **CONT-08**: User can export page as PDF (blocked: no Confluence Cloud REST API -- CONFCLOUD-61557)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| PDF/Word export | Confluence Cloud has no REST API for PDF/Word export (CONFCLOUD-61557) |
| Markdown conversion | Agents handle raw storage format; avoids conversion complexity |
| Content rendering/preview | CLI outputs JSON, not HTML; not useful for agents |
| Real-time collaboration | WebSocket-based; not applicable to CLI polling model |
| Version merge conflict UI | No interactive mode in agent-focused CLI |
| Bulk space export | Long-running operation with no API support |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| UTIL-01 | Phase 12 | Complete |
| UTIL-02 | Phase 12 | Complete |
| UTIL-03 | Phase 12 | Complete |
| CONT-01 | Phase 13 | Complete |
| CONT-02 | Phase 13 | Complete |
| CONT-03 | Phase 13 | Complete |
| CONT-04 | Phase 13 | Complete |
| CONT-05 | Phase 13 | Complete |
| CONT-06 | Phase 13 | Complete |
| CONT-07 | Phase 13 | Complete |
| DIFF-01 | Phase 14 | Complete |
| DIFF-02 | Phase 14 | Complete |
| DIFF-03 | Phase 14 | Complete |
| WKFL-01 | Phase 15 | Complete |
| WKFL-02 | Phase 15 | Complete |
| WKFL-03 | Phase 15 | Complete |
| WKFL-04 | Phase 15 | Complete |
| WKFL-05 | Phase 15 | Complete |
| WKFL-06 | Phase 15 | Complete |
| SCHM-01 | Phase 16 | Complete |
| SCHM-02 | Phase 16 | Complete |
| DOCS-05 | Phase 16 | Pending |
| CICD-01 | Phase 17 | Pending |
| CICD-02 | Phase 17 | Pending |
| CICD-03 | Phase 17 | Pending |
| CICD-04 | Phase 17 | Pending |
| CICD-05 | Phase 17 | Pending |
| CICD-06 | Phase 17 | Pending |
| CICD-07 | Phase 17 | Pending |
| CICD-08 | Phase 17 | Pending |
| CICD-09 | Phase 17 | Pending |
| CICD-10 | Phase 17 | Pending |
| DOCS-01 | Phase 17 | Pending |
| DOCS-02 | Phase 17 | Pending |
| DOCS-03 | Phase 17 | Pending |
| CONF-01 | Phase 17 | Pending |
| CONF-02 | Phase 17 | Pending |
| CONF-03 | Phase 17 | Pending |
| DOCS-04 | Phase 18 | Pending |

**Coverage:**
- v1.2 requirements: 39 total
- Mapped to phases: 39
- Unmapped: 0

---
*Requirements defined: 2026-03-28*
*Last updated: 2026-03-28 after roadmap creation*
