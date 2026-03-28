# Architecture Patterns

**Domain:** CLI tool feature expansion and release infrastructure (v1.2 milestone)
**Researched:** 2026-03-28
**Confidence:** HIGH (based on direct inspection of cf codebase + jr reference implementation)

## Context

This document analyzes how v1.2 features integrate with the existing cf architecture. All recommendations are derived from direct comparison of the cf codebase (`/Users/quan.hoang/quanhh/quanhoang/confluence-cli/`) against the jr reference implementation (`/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/`), which already has all the target features implemented.

The key constraint: cf mirrors jr's architecture exactly. Every new feature should follow the established pattern unless there is a Confluence-specific reason to deviate.

---

## Existing Architecture Summary

```
confluence-cli/
  main.go                    -- entry point, calls cmd.Execute()
  Makefile                   -- generate/build/install/test/clean
  go.mod                     -- cobra, gojq, libopenapi (no tidwall/pretty, no yaml.v3)
  spec/                      -- OpenAPI spec files
  gen/                       -- code generator (reads spec/, writes cmd/generated/)
  cmd/
    root.go                  -- PersistentPreRunE (config, auth, oauth2, preset, policy, audit, client injection)
    generated/               -- auto-generated commands + schema_data.go
    schema_cmd.go            -- schema discovery (currently only uses generated.AllSchemaOps())
    pages.go, spaces.go, ... -- hand-written workflow overrides
    batch.go                 -- multi-op execution
    watch.go                 -- CQL polling (inline, no internal/watch package)
    templates.go             -- template list + resolveTemplate helper
    export_test.go           -- test helpers exposing internal symbols
  internal/
    audit/                   -- NDJSON audit logging
    avatar/                  -- writing style analysis
    cache/                   -- GET response caching
    client/                  -- HTTP client with auth, pagination, jq, dry-run
    config/                  -- profile config (~/.config/cf/config.json)
    errors/                  -- structured errors, exit codes
    jq/                      -- gojq wrapper
    oauth2/                  -- client credentials + 3LO flows
    policy/                  -- operation allow/deny
    template/                -- file-based templates (JSON, user dir only)
```

### Key Patterns Already Established

1. **mergeCommand()** -- hand-written commands replace generated parents while preserving generated subcommands not overridden
2. **client.FromContext()** -- every command retrieves `*client.Client` from cobra context (injected in PersistentPreRunE)
3. **client.Do() / client.Fetch()** -- `Do()` writes to stdout automatically; `Fetch()` returns raw bytes for commands that need post-processing
4. **SchemaOp registration** -- currently cf's schema_cmd.go only calls `generated.AllSchemaOps()`, unlike jr which appends hand-written schema ops
5. **Error pattern** -- build `cferrors.APIError`, call `WriteJSON(c.Stderr)`, return `&cferrors.AlreadyWrittenError{Code: exitCode}`
6. **Presets** -- currently config-only (profile `presets` map[string]string), no built-in presets, no internal/preset package
7. **Templates** -- user dir JSON files only (no embedded builtins), simpler than jr's YAML + embed.FS model

---

## New Components: Detailed Integration Plan

### 1. `internal/jsonutil` package (NEW)

**What:** Extract the `marshalNoEscape()` helper duplicated across cmd files into a shared utility.

**Jr reference:** `internal/jsonutil/jsonutil.go` -- single function `MarshalNoEscape(v any) ([]byte, error)`.

**Integration:**
- Create `internal/jsonutil/jsonutil.go` with `MarshalNoEscape()`
- Replace the private `marshalNoEscape()` in `cmd/schema_cmd.go` with `jsonutil.MarshalNoEscape` or a package-level alias
- Use in all new cmd files (diff, workflow, export, preset)
- No new dependencies -- pure stdlib

**Files:**
| File | Action |
|------|--------|
| `internal/jsonutil/jsonutil.go` | NEW -- `MarshalNoEscape(v any) ([]byte, error)` |
| `internal/jsonutil/jsonutil_test.go` | NEW -- unit tests |
| `cmd/schema_cmd.go` | MODIFY -- replace private `marshalNoEscape` with `jsonutil.MarshalNoEscape` or alias |

**Build order dependency:** None. Pure utility. Build first.

---

### 2. `internal/duration` package (NEW)

**What:** Human-friendly duration parsing (e.g. "2h", "1d 3h", "30m") for the diff command's `--since` flag.

**Jr reference:** `internal/duration/duration.go` -- `Parse(s string) (int, error)` with regex-based parsing. Uses Jira conventions (1d=8h, 1w=5d).

**Confluence adaptation:** Confluence does not have the same work-day convention as Jira. For the diff `--since` flag, the duration is purely a time offset. The implementation should use **calendar conventions** (1d=24h, 1w=7d) rather than Jira's work-day model, since Confluence version history is calendar-based.

**Files:**
| File | Action |
|------|--------|
| `internal/duration/duration.go` | NEW -- `Parse(s string) (int, error)` with 1d=24h, 1w=7d |
| `internal/duration/duration_test.go` | NEW -- unit tests |

**Build order dependency:** None. Pure utility. Build first (alongside jsonutil).

---

### 3. `cmd/diff.go` + `cmd/diff_schema.go` (NEW)

**What:** Page version history viewer. Fetches page versions and shows field-level changes as structured JSON.

**Jr reference:** `cmd/diff.go` fetches `/rest/api/3/issue/{key}/changelog`, delegates to `internal/changelog.Parse()`. Supports `--issue`, `--since`, `--field` flags.

**Confluence adaptation:** Confluence has a page versions API: `GET /pages/{id}/versions` (v2). Unlike Jira's changelog which has field-level change items, Confluence versions are whole-page snapshots. The diff command should:
1. Fetch version history for a page
2. Support `--since` (duration or ISO date) filtering using `internal/duration`
3. Support `--count` to limit number of versions returned
4. Output structured JSON with version number, author, timestamp, and optionally body comparison between two versions (`--from` and `--to` version numbers)

**Data flow:**
```
User: cf diff --id 12345 --since 2h
  -> PersistentPreRunE injects client
  -> runDiff() gets client from context
  -> c.Fetch(GET /pages/{id}/versions)
  -> Parse response, filter by --since via duration.Parse()
  -> Output via c.WriteOutput()
```

**Schema registration:** Create `DiffSchemaOps()` in `cmd/diff_schema.go`, append in `schema_cmd.go`.

**Files:**
| File | Action |
|------|--------|
| `cmd/diff.go` | NEW -- `diffCmd` + `runDiff()` |
| `cmd/diff_schema.go` | NEW -- `DiffSchemaOps() []generated.SchemaOp` |
| `cmd/root.go` | MODIFY -- add `rootCmd.AddCommand(diffCmd)` in init() |
| `cmd/schema_cmd.go` | MODIFY -- append `DiffSchemaOps()` to allOps |

**Build order dependency:** Depends on `internal/duration` and `internal/jsonutil`.

---

### 4. `cmd/workflow.go` + `cmd/workflow_schema.go` (NEW)

**What:** High-level workflow commands: move, copy, publish, restrict, archive, comment.

**Jr reference:** `cmd/workflow.go` defines `workflowCmd` as parent with subcommands: transition, assign, comment, move, create, link, log-work, sprint. Each is a separate `*cobra.Command` with its own `RunE`. Schema ops registered in `cmd/workflow_schema.go`.

**Confluence adaptation:** Confluence workflows differ fundamentally from Jira workflows. There are no transition IDs or status names to resolve. Instead:

| Subcommand | API | Purpose |
|------------|-----|---------|
| `workflow move` | PUT /pages/{id} with new spaceId or parentId | Move page to different space/parent |
| `workflow copy` | POST /pages/{id}/copy (v1) | Copy page with options |
| `workflow publish` | PUT /pages/{id} with status "current" | Publish a draft page |
| `workflow archive` | POST /pages/{id}/archive (v1) or PUT status | Archive a page |
| `workflow restrict` | PUT /pages/{id}/restrictions | Set page restrictions |
| `workflow comment` | POST /footer-comments | Add a comment to a page (simplified) |

**Pattern:** Same as jr -- `workflowCmd` parent with individual subcommands. Each subcommand uses `client.Fetch()` for multi-step operations (e.g., move requires fetching current page, then updating with new parent/space).

**Data flow:**
```
User: cf workflow move --id 12345 --to-space-id 67890
  -> PersistentPreRunE injects client
  -> runMove() gets client from context
  -> c.Fetch(GET /pages/{id}) to get current title/version
  -> c.Fetch(PUT /pages/{id}) with updated spaceId and version+1
  -> Output via c.WriteOutput()
```

**Files:**
| File | Action |
|------|--------|
| `cmd/workflow.go` | NEW -- workflowCmd parent + move, copy, publish, archive, restrict, comment subcommands |
| `cmd/workflow_schema.go` | NEW -- `HandWrittenSchemaOps() []generated.SchemaOp` |
| `cmd/root.go` | MODIFY -- add `rootCmd.AddCommand(workflowCmd)` in init() |
| `cmd/schema_cmd.go` | MODIFY -- append `HandWrittenSchemaOps()` to allOps |

**Build order dependency:** Depends on `internal/jsonutil`. Uses existing `client.Fetch()` and `fetchPageVersion()` patterns.

---

### 5. `internal/preset` package (NEW) + `cmd/preset.go` (NEW)

**What:** Built-in presets with a `preset list` subcommand. Currently cf has presets only in profile config (map[string]string of JQ expressions). Jr has a full `internal/preset` package with built-in presets, user presets, and a `preset list` command.

**Jr reference:** `internal/preset/preset.go` defines:
- `Preset` struct with `Fields` and `JQ` fields
- `builtinPresets` map (agent, detail, triage, board)
- `Lookup(name) (Preset, bool, error)` -- checks user file then builtins
- `List() ([]byte, error)` -- merged list with source attribution

**Confluence adaptation:** Presets for Confluence differ from Jira:
1. Confluence v2 API does not have a generic `fields` query parameter like Jira. The `--fields` flag in cf maps to a specific API param only where supported.
2. Confluence presets should be **JQ-only** (matching the existing profile-level `presets` map[string]string pattern).
3. Built-in presets should target Confluence content patterns:

```go
var builtinPresets = map[string]Preset{
    "titles":    {JQ: `.results[] | {id, title, status}`},
    "agent":     {JQ: `.results[] | {id, title, status, spaceId, version: .version.number}`},
    "detail":    {JQ: `{id, title, status, spaceId, body: .body.storage.value, version: .version}`},
    "versions":  {JQ: `.results[] | {number: .number, authorId: .authorId, createdAt: .createdAt}`},
}
```

**Integration with root.go:** The preset resolution in `PersistentPreRunE` currently looks up presets from `rawProfile.Presets[preset]`. It needs to be extended to also check `preset.Lookup()` from the new package as a fallback. Order: profile presets override built-in presets (consistent with jr).

**Files:**
| File | Action |
|------|--------|
| `internal/preset/preset.go` | NEW -- Preset struct, builtinPresets, Lookup(), List() |
| `internal/preset/preset_test.go` | NEW -- unit tests |
| `cmd/preset.go` | NEW -- presetCmd parent + presetListCmd subcommand |
| `cmd/root.go` | MODIFY -- extend preset resolution to call `preset.Lookup()` as fallback after profile |
| `cmd/root.go` | MODIFY -- add `rootCmd.AddCommand(presetCmd)` in init() |

**Important design decision:** The Preset struct should be `{JQ string}` only (not `{Fields, JQ}` like jr) because Confluence's API does not support field filtering at the API level in the same way. The preset system is purely about JQ output shaping.

**Build order dependency:** None for the package itself. Root.go modification depends on the package.

---

### 6. Built-in Templates + `cmd/templates.go` expansion (MODIFY)

**What:** Ship embedded built-in templates and add `show`, `create` subcommands to the existing `templates` command.

**Jr reference:** `internal/template/template.go` uses `//go:embed builtin/*.yaml` for embedded templates, has YAML format with Variables and Fields, supports `loadBuiltinTemplates()` + `loadUserTemplates()` merge. Commands: list, show, apply, create.

**Confluence adaptation:** cf's template system is simpler (JSON format, no Variables metadata, no YAML). The recommended approach:

**Keep JSON format, add embedded builtins via `embed.FS`, add show/create subcommands.** This avoids adding `gopkg.in/yaml.v3` as a dependency (cf currently has zero YAML deps). cf's templates are content-body templates (storage format XML), not issue-field templates like jr. The simpler model fits better.

**Built-in templates to embed:**
```
internal/template/builtin/
  meeting-notes.json
  decision-record.json
  project-update.json
  blank.json
```

**Files:**
| File | Action |
|------|--------|
| `internal/template/template.go` | MODIFY -- add `//go:embed builtin/*.json`, loadBuiltinTemplates(), merge with user templates in List()/Load() |
| `internal/template/builtin/meeting-notes.json` | NEW -- embedded template |
| `internal/template/builtin/decision-record.json` | NEW -- embedded template |
| `internal/template/builtin/project-update.json` | NEW -- embedded template |
| `internal/template/builtin/blank.json` | NEW -- embedded template |
| `internal/template/template_test.go` | MODIFY -- add tests for builtins |
| `cmd/templates.go` | MODIFY -- add templates_show, templates_create subcommands |
| `cmd/templates_schema.go` | NEW -- `TemplateSchemaOps() []generated.SchemaOp` |
| `cmd/schema_cmd.go` | MODIFY -- append `TemplateSchemaOps()` to allOps |

**Build order dependency:** Template package modification before cmd changes.

---

### 7. `cmd/export.go` (NEW)

**What:** Export page content to file or stdout. Fetches a page and writes body content (storage format) directly.

**Design:**
```
cf export --id 12345                      # outputs storage format body to stdout
cf export --id 12345 --format storage     # explicit format (storage is default)
cf export --id 12345 --output page.xml    # write to file instead of stdout
```

The export command is a thin wrapper: fetch page with body-format=storage, extract `.body.storage.value`, output it. This bypasses the normal JSON output pipeline (no JQ wrapping of raw XML/HTML).

**Files:**
| File | Action |
|------|--------|
| `cmd/export.go` | NEW -- exportCmd + runExport() |
| `cmd/export_schema.go` | NEW -- `ExportSchemaOps() []generated.SchemaOp` |
| `cmd/root.go` | MODIFY -- add `rootCmd.AddCommand(exportCmd)` in init() |
| `cmd/schema_cmd.go` | MODIFY -- append `ExportSchemaOps()` to allOps |

**Build order dependency:** Uses existing client.Fetch(). No new internal deps.

---

### 8. Schema Command Integration (MODIFY)

**What:** cf's `schema_cmd.go` currently only uses `generated.AllSchemaOps()`. Jr's version appends hand-written schema ops from multiple sources. cf must do the same.

**Current cf schema_cmd.go:**
```go
allOps := generated.AllSchemaOps()
```

**Target (matching jr):**
```go
allOps := generated.AllSchemaOps()
allOps = append(allOps, HandWrittenSchemaOps()...)   // workflow
allOps = append(allOps, DiffSchemaOps()...)           // diff
allOps = append(allOps, ExportSchemaOps()...)          // export
allOps = append(allOps, TemplateSchemaOps()...)        // templates
allOps = append(allOps, WatchSchemaOps()...)           // watch (existing cmd, new schema)
```

**Also needed:** The existing watch command should get a `WatchSchemaOps()` function so agents can discover it via `cf schema watch`.

**Files:**
| File | Action |
|------|--------|
| `cmd/schema_cmd.go` | MODIFY -- aggregate all hand-written schema ops |
| `cmd/watch_schema.go` | NEW -- `WatchSchemaOps() []generated.SchemaOp` for existing watch command |

**Build order dependency:** Must be done after all *_schema.go files are created.

---

### 9. GitHub Actions CI/CD (NEW)

**What:** Full CI/CD pipeline matching jr's 7-workflow setup.

**Jr reference:** `.github/workflows/` contains:
1. `ci.yml` -- test, lint, npm-smoke-test, pypi-smoke-test, docs-build, integration
2. `release.yml` -- GoReleaser + npm publish + PyPI publish (triggered by tag push)
3. `docs.yml` -- VitePress build + GitHub Pages deploy
4. `security.yml` -- gosec + govulncheck (PR + weekly schedule)
5. `spec-drift.yml` -- daily check for Confluence OpenAPI spec changes, auto-PR
6. `spec-auto-release.yml` -- auto-tag when spec-drift PR merges with code changes
7. `dependabot-auto-merge.yml` -- auto-merge dependabot PRs

**Confluence adaptation:** Mirror exactly, with these substitutions:

| Jr value | Cf value |
|----------|----------|
| `jr` binary name | `cf` binary name |
| `sofq/jira-cli` repo | `sofq/confluence-cli` repo |
| `jira-jr` npm package | `confluence-cf` npm package |
| `jira_jr` python package | `confluence_cf` python package |
| Jira OpenAPI spec URL | `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json` |
| `JR_BASE_URL` env vars | `CF_BASE_URL` env vars |

**Files:**
| File | Action |
|------|--------|
| `.github/workflows/ci.yml` | NEW -- test, lint, npm/pypi smoke tests, docs-build, integration |
| `.github/workflows/release.yml` | NEW -- GoReleaser + npm + PyPI publish |
| `.github/workflows/docs.yml` | NEW -- VitePress build + GitHub Pages deploy |
| `.github/workflows/security.yml` | NEW -- gosec + govulncheck |
| `.github/workflows/spec-drift.yml` | NEW -- daily Confluence spec check |
| `.github/workflows/spec-auto-release.yml` | NEW -- auto-tag on spec-drift merge |
| `.github/workflows/dependabot-auto-merge.yml` | NEW -- dependabot auto-merge |

**Build order dependency:** Depends on GoReleaser, VitePress, npm, and python scaffolds being in place.

---

### 10. GoReleaser Configuration (NEW)

**What:** Cross-platform binary builds, Docker images, Homebrew tap, Scoop bucket.

**Jr reference:** `.goreleaser.yml` -- version 2, builds linux/darwin/windows amd64/arm64, tar.gz/zip archives, checksums, Homebrew tap, Scoop bucket, Docker multi-arch images via ghcr.io.

**Files:**
| File | Action |
|------|--------|
| `.goreleaser.yml` | NEW -- builds, archives, checksums, brews, scoops, dockers, docker_manifests |
| `Dockerfile` | NEW -- multi-stage build (for local builds) |
| `Dockerfile.goreleaser` | NEW -- minimal FROM distroless (for GoReleaser) |
| `.dockerignore` | NEW |

**GoReleaser config key points:**
```yaml
version: 2
builds:
  - binary: cf
    ldflags: -s -w -X github.com/sofq/confluence-cli/cmd.Version={{.Version}}
brews:
  - name: cf
    repository: {owner: sofq, name: homebrew-tap}
scoops:
  - name: cf
    repository: {owner: sofq, name: scoop-bucket}
dockers:
  - image_templates: ["ghcr.io/sofq/cf:{{ .Version }}-amd64"]
```

**Build order dependency:** None. Configuration only.

---

### 11. npm Package Scaffold (NEW)

**What:** npm package that downloads the appropriate cf binary on `postinstall`.

**Jr reference:** `npm/` directory with `package.json` (bin: {"jr": "bin/jr"}, postinstall: "node install.js") and `install.js` (downloads platform/arch binary from GitHub Releases).

**Files:**
| File | Action |
|------|--------|
| `npm/package.json` | NEW -- name: "confluence-cf", bin: {"cf": "bin/cf"} |
| `npm/install.js` | NEW -- download logic (same structure, cf-specific URLs) |
| `npm/README.md` | NEW -- installation docs |

**Build order dependency:** Depends on GoReleaser config (download URLs must match archive naming).

---

### 12. Python Package Scaffold (NEW)

**What:** Python package (pip install) that downloads the appropriate cf binary.

**Jr reference:** `python/` directory with `pyproject.toml` (setuptools build, entry point) and `confluence_cf/__init__.py` (binary path resolution + download).

**Files:**
| File | Action |
|------|--------|
| `python/pyproject.toml` | NEW -- name: "confluence-cf" |
| `python/confluence_cf/__init__.py` | NEW -- binary path resolution + download |
| `python/README.md` | NEW -- installation docs |

**Build order dependency:** Depends on GoReleaser config (download URLs must match archive naming).

---

### 13. VitePress Documentation Site (NEW)

**What:** Auto-generated command reference docs + hand-written guide pages, deployed to GitHub Pages.

**Jr reference:** Complex but well-structured:
- `cmd/gendocs/main.go` -- walks Cobra command tree, generates per-resource markdown pages + index + sidebar JSON + error codes page
- `website/.vitepress/config.ts` -- VitePress config with dynamic sidebar import from generated `sidebar-commands.json`
- `website/guide/*.md` -- hand-written guide pages (getting-started, filtering, discovery, templates, global-flags, error-codes, agent-integration, skill-setup)
- `website/commands/*.md` -- auto-generated (in .gitignore)
- `website/index.md` -- landing page
- `Makefile` targets: `docs-generate`, `docs-dev`, `docs-build`

**Key detail from jr's gendocs:** The program imports the cmd package's `RootCommand()` and all `*SchemaOps()` functions to build a complete schema lookup, then walks the command tree to generate markdown. It also generates a `sidebar-commands.json` for VitePress and an `error-codes.md` from the errors package.

**Files:**
| File | Action |
|------|--------|
| `cmd/gendocs/main.go` | NEW -- walks cf command tree, generates docs |
| `website/package.json` | NEW -- vitepress dependency |
| `website/.vitepress/config.ts` | NEW -- VitePress config for cf |
| `website/index.md` | NEW -- landing page |
| `website/guide/getting-started.md` | NEW |
| `website/guide/filtering.md` | NEW |
| `website/guide/discovery.md` | NEW |
| `website/guide/templates.md` | NEW |
| `website/guide/global-flags.md` | NEW |
| `website/guide/error-codes.md` | GENERATED -- by gendocs |
| `website/guide/agent-integration.md` | NEW |
| `website/commands/*.md` | GENERATED -- by gendocs |
| `website/public/logo.svg` | NEW -- site logo |
| `Makefile` | MODIFY -- add docs-generate, docs-dev, docs-build, docs targets |

**Build order dependency:** Depends on all *_schema.go files being complete (gendocs uses them).

---

### 14. Project Root Files (NEW)

**Files:**
| File | Action |
|------|--------|
| `README.md` | NEW -- project readme |
| `LICENSE` | NEW -- license file |
| `SECURITY.md` | NEW -- security reporting instructions |
| `.golangci.yml` | NEW -- linter config (match jr's errcheck exclusions) |
| `.gitignore` | MODIFY -- add dist/, website/, coverage.out, node_modules patterns |

**Build order dependency:** None. Pure documentation/config.

---

### 15. Makefile Expansion (MODIFY)

**Current cf Makefile targets:** `generate`, `build`, `install`, `test`, `clean`

**New targets to add (matching jr):** `lint`, `spec-update`, `docs-generate`, `docs-dev`, `docs-build`, `docs`

```makefile
SPEC_URL := https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json

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

---

### 16. go.mod Dependency Changes

**Current cf deps:** gojq, libopenapi, cobra (3 direct deps)

**Jr additional deps over cf:** `tidwall/pretty`, `yaml.v3`, `pflag` (direct)

**Recommendation for cf:**
- **Add `spf13/pflag`** as direct dependency -- needed by `cmd/gendocs/main.go` for flag introspection via `pflag.Flag` type
- **Do NOT add `tidwall/pretty`** -- cf uses `json.Indent()` from stdlib for pretty-printing (established in client.WriteOutput). Keep consistency.
- **Do NOT add `yaml.v3`** -- keep templates in JSON format (avoids new dependency, cf templates are simpler than jr templates)

**Final go.mod direct deps:** gojq, libopenapi, cobra, pflag (4 direct deps)

---

## Architecture Diagram: New Components

```
                           cmd/
                            |
  root.go (MODIFY)          |
    |-- configureCmd         |
    |-- rawCmd               |
    |-- batchCmd             |
    |-- schemaCmd (MODIFY)   |--- schema_cmd.go (MODIFY: aggregate all SchemaOps)
    |-- pagesCmd             |
    |-- spacesCmd            |
    |-- searchCmd            |
    |-- watchCmd             |--- watch_schema.go (NEW)
    |-- avatarCmd            |
    |-- templatesCmd (MODIFY)|--- templates.go (MODIFY: add show, create)
    |                        |--- templates_schema.go (NEW)
    |-- NEW: diffCmd         |--- diff.go (NEW)
    |                        |--- diff_schema.go (NEW)
    |-- NEW: workflowCmd     |--- workflow.go (NEW)
    |                        |--- workflow_schema.go (NEW)
    |-- NEW: presetCmd       |--- preset.go (NEW)
    |-- NEW: exportCmd       |--- export.go (NEW)
    |                        |--- export_schema.go (NEW)
    |-- NEW: gendocs/main.go |
    |
  internal/
    |-- audit/
    |-- avatar/
    |-- cache/
    |-- client/
    |-- config/
    |-- errors/
    |-- jq/
    |-- oauth2/
    |-- policy/
    |-- template/ (MODIFY: add embed.FS builtins)
    |      |-- builtin/ (NEW: embedded JSON templates)
    |-- NEW: jsonutil/
    |-- NEW: preset/
    |-- NEW: duration/
    |
  NEW: .github/workflows/   (7 workflow files)
  NEW: .goreleaser.yml
  NEW: Dockerfile, Dockerfile.goreleaser, .dockerignore
  NEW: website/              (VitePress docs)
  NEW: npm/                  (npm scaffold)
  NEW: python/               (Python scaffold)
  MODIFY: Makefile           (add lint/docs targets)
  MODIFY: .gitignore         (add new patterns)
  MODIFY: go.mod             (add pflag direct)
  NEW: README.md, LICENSE, SECURITY.md, .golangci.yml
```

---

## Data Flow Changes

### Preset Resolution (Modified Flow)

**Current:**
```
PersistentPreRunE -> rawProfile.Presets[preset] -> jqFilter
                     (not found -> error)
```

**New (three-tier lookup):**
```
PersistentPreRunE -> rawProfile.Presets[preset]    (1. profile override -- highest priority)
                  -> preset.Lookup(preset)          (2. user file + builtin fallback)
                  -> error if not found anywhere
                  -> jqFilter
```

Profile presets take priority over `internal/preset` builtins. This matches jr's behavior where user presets override builtins.

### Schema Discovery (Modified Flow)

**Current:**
```
schema_cmd.go -> generated.AllSchemaOps() -> output
```

**New:**
```
schema_cmd.go -> generated.AllSchemaOps()
             -> append HandWrittenSchemaOps()   (workflow)
             -> append DiffSchemaOps()           (diff)
             -> append ExportSchemaOps()          (export)
             -> append TemplateSchemaOps()        (templates)
             -> append WatchSchemaOps()           (watch)
             -> output
```

This is the exact pattern jr uses in its `schema_cmd.go` (lines 30-34).

### Template Loading (Modified Flow)

**Current:**
```
template.Load(name) -> userDir/{name}.json -> Template
                       (not found -> error)
```

**New (user override + embedded fallback):**
```
template.Load(name) -> userDir/{name}.json     (user override -- check first)
                    -> embed.FS builtin/{name}.json (fallback to embedded)
                    -> error if not found anywhere
```

### Documentation Generation Flow (NEW)

```
make docs-generate
  -> go run ./cmd/gendocs/... website
     -> cmd.RootCommand() -- get full command tree
     -> buildSchemaLookup() -- aggregate all SchemaOps
     -> walkCommands() -- extract resource/verb/flag info
     -> render per-resource markdown -> website/commands/*.md
     -> render index page -> website/commands/index.md
     -> render error codes -> website/guide/error-codes.md
     -> generate sidebar JSON -> website/.vitepress/sidebar-commands.json

make docs-build
  -> docs-generate (above)
  -> cd website && npx vitepress build
     -> reads .vitepress/config.ts (imports sidebar-commands.json)
     -> renders static site -> website/.vitepress/dist/
```

---

## Patterns to Follow

### Pattern 1: Hand-Written Schema Registration
**What:** Every hand-written command gets a companion `*_schema.go` file that returns `[]generated.SchemaOp`.
**When:** Any new command that should be discoverable via `cf schema`.
**Example:**
```go
// cmd/diff_schema.go
package cmd

import "github.com/sofq/confluence-cli/cmd/generated"

func DiffSchemaOps() []generated.SchemaOp {
    return []generated.SchemaOp{
        {
            Resource: "diff",
            Verb:     "diff",
            Method:   "GET",
            Path:     "/pages/{id}/versions",
            Summary:  "Show version history for a page as structured JSON",
            HasBody:  false,
            Flags: []generated.SchemaFlag{
                {Name: "id", Required: true, Type: "string", Description: "page ID", In: "custom"},
                {Name: "since", Required: false, Type: "string", Description: "filter versions since duration or date", In: "custom"},
            },
        },
    }
}
```

### Pattern 2: Workflow Subcommand Structure
**What:** Parent command with verb subcommands, each handling one operation.
**When:** Building workflow commands (move, copy, publish, etc.).
**Example:**
```go
var workflowCmd = &cobra.Command{
    Use:   "workflow",
    Short: "High-level workflow commands (move, copy, publish, archive)",
}

var workflowMoveCmd = &cobra.Command{
    Use:   "move",
    Short: "Move a page to a different space or parent",
    RunE:  runWorkflowMove,
}

func init() {
    workflowMoveCmd.Flags().String("id", "", "page ID (required)")
    _ = workflowMoveCmd.MarkFlagRequired("id")
    workflowMoveCmd.Flags().String("to-space-id", "", "target space ID")
    workflowMoveCmd.Flags().String("to-parent-id", "", "target parent page ID")
    workflowCmd.AddCommand(workflowMoveCmd)
}
```

### Pattern 3: Built-in + User Override via embed.FS
**What:** Embed defaults, allow user-level overrides via config directory.
**When:** Presets and templates.
**Example:**
```go
//go:embed builtin/*.json
var embeddedFS embed.FS

func Load(name string) (*Template, error) {
    // Try user dir first (override)
    userPath := filepath.Join(Dir(), name+".json")
    if data, err := os.ReadFile(userPath); err == nil {
        return parseTemplate(data)
    }
    // Fall back to embedded
    data, err := fs.ReadFile(embeddedFS, "builtin/"+name+".json")
    if err != nil {
        return nil, fmt.Errorf("template %q not found", name)
    }
    return parseTemplate(data)
}
```

### Pattern 4: Multi-Step Workflow via Fetch()
**What:** Commands that need intermediate API calls before the final operation use `client.Fetch()` to get raw bytes, then process and make the final call.
**When:** Any command that needs to read-then-write (move, update, copy, archive).
**Example:** Already established in `fetchPageVersion()` + `doPageUpdate()` pattern in `cmd/pages.go`.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Inline Logic Instead of Internal Package
**What:** Putting parsing/business logic directly in cmd files instead of internal packages.
**Why bad:** Untestable without the full CLI entrypoint, duplicated across cmd files.
**Instead:** Extract to internal packages. Duration parsing goes in internal/duration, not inline in diff.go.

### Anti-Pattern 2: Adding Dependencies for Marginal Benefit
**What:** Adding `tidwall/pretty` or `yaml.v3` when stdlib alternatives exist.
**Why bad:** cf has maintained minimal dependencies through v1.0 and v1.1 (only 3 direct deps). Adding deps for features already covered by stdlib (json.Indent for pretty, JSON for templates) breaks the project's zero-unnecessary-deps stance.
**Instead:** Use stdlib. Only add dependencies when there is no stdlib equivalent.

### Anti-Pattern 3: Modifying Generated Code
**What:** Hand-editing files in `cmd/generated/`.
**Why bad:** Gets overwritten on `make generate`. The `mergeCommand()` pattern exists precisely to avoid this.
**Instead:** Use `mergeCommand()` for overriding generated commands, or add new commands via `rootCmd.AddCommand()`.

### Anti-Pattern 4: Skipping Schema Registration
**What:** Adding a command but not creating its `*_schema.go` file.
**Why bad:** Agents cannot discover the command via `cf schema`. Batch operations cannot find it. Documentation generation misses it.
**Instead:** Every public command gets a schema file. Schema registration is mandatory for all hand-written commands.

### Anti-Pattern 5: Coupling Documentation to Implementation
**What:** Writing guide documentation that references specific API response shapes or version numbers.
**Why bad:** Guide pages are hand-written and not auto-updated. When API shapes change, docs become stale.
**Instead:** Guide pages explain concepts and workflows. Command reference pages (auto-generated by gendocs) handle the specifics.

---

## Complete File Inventory

### New Files (37 total)

**Internal packages (6 files):**
- `internal/jsonutil/jsonutil.go`
- `internal/jsonutil/jsonutil_test.go`
- `internal/duration/duration.go`
- `internal/duration/duration_test.go`
- `internal/preset/preset.go`
- `internal/preset/preset_test.go`

**Embedded templates (4 files):**
- `internal/template/builtin/meeting-notes.json`
- `internal/template/builtin/decision-record.json`
- `internal/template/builtin/project-update.json`
- `internal/template/builtin/blank.json`

**Command files (10 files):**
- `cmd/diff.go`
- `cmd/diff_schema.go`
- `cmd/workflow.go`
- `cmd/workflow_schema.go`
- `cmd/preset.go`
- `cmd/export.go`
- `cmd/export_schema.go`
- `cmd/templates_schema.go`
- `cmd/watch_schema.go`
- `cmd/gendocs/main.go`

**Infrastructure (4 files):**
- `.goreleaser.yml`
- `Dockerfile`
- `Dockerfile.goreleaser`
- `.dockerignore`

**GitHub Actions (7 files):**
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `.github/workflows/docs.yml`
- `.github/workflows/security.yml`
- `.github/workflows/spec-drift.yml`
- `.github/workflows/spec-auto-release.yml`
- `.github/workflows/dependabot-auto-merge.yml`

**Distribution scaffolds (5 files):**
- `npm/package.json`
- `npm/install.js`
- `npm/README.md`
- `python/pyproject.toml`
- `python/confluence_cf/__init__.py`

**VitePress site (3+ files, plus generated):**
- `website/package.json`
- `website/.vitepress/config.ts`
- `website/index.md`
- `website/guide/getting-started.md` (and 5 more guide pages)

**Root files (4 files):**
- `README.md`
- `LICENSE`
- `SECURITY.md`
- `.golangci.yml`

### Modified Files (8 total)
- `cmd/root.go` -- register new commands, extend preset resolution
- `cmd/schema_cmd.go` -- aggregate all schema ops
- `cmd/templates.go` -- add show/create subcommands
- `internal/template/template.go` -- add embed.FS builtins
- `Makefile` -- add lint/docs/spec-update targets
- `.gitignore` -- add new patterns
- `go.mod` -- add pflag direct dep
- `cmd/export_test.go` -- add test helpers for new commands if needed

---

## Suggested Build Order

Dependencies drive the order. Items at the same level can be built in parallel.

```
Level 0 (no dependencies -- build first):
  [0a] internal/jsonutil
  [0b] internal/duration
  [0c] .goreleaser.yml + Dockerfile + Dockerfile.goreleaser + .dockerignore
  [0d] .golangci.yml
  [0e] .gitignore updates
  [0f] README.md, LICENSE, SECURITY.md

Level 1 (depends on Level 0 utilities):
  [1a] internal/preset
  [1b] internal/template modification (add embed.FS builtins)
  [1c] cmd/diff.go + cmd/diff_schema.go          (uses duration, jsonutil)
  [1d] cmd/export.go + cmd/export_schema.go       (uses client.Fetch only)

Level 2 (parallel with Level 1 or after):
  [2a] cmd/workflow.go + cmd/workflow_schema.go   (uses jsonutil, client.Fetch)
  [2b] cmd/preset.go                              (uses internal/preset)
  [2c] cmd/templates.go expansion + cmd/templates_schema.go (uses modified template pkg)
  [2d] cmd/watch_schema.go                        (schema for existing watch)

Level 3 (depends on all schema files):
  [3a] cmd/schema_cmd.go modification             (aggregate all *SchemaOps)
  [3b] cmd/root.go modifications                  (register commands, preset resolution)
  [3c] Makefile expansion                         (add lint, spec-update targets)

Level 4 (depends on Level 3 -- full command tree must be registrable):
  [4a] cmd/gendocs/main.go
  [4b] website/ VitePress setup + guide pages
  [4c] Makefile docs targets

Level 5 (depends on GoReleaser archive naming):
  [5a] npm/ scaffold
  [5b] python/ scaffold

Level 6 (depends on everything):
  [6a] .github/workflows/ (all 7 workflows)
```

**Phase ordering rationale:**
- Level 0 items have zero dependencies and establish foundations
- Level 1-2 items are feature code that can largely be built in parallel
- Level 3 is the integration point where all schema ops come together
- Level 4 needs the complete command tree for documentation generation
- Level 5 needs archive naming patterns from GoReleaser
- Level 6 needs all components to exist for CI validation

---

## Scalability Considerations

| Concern | Current (v1.1) | After v1.2 | Notes |
|---------|----------------|------------|-------|
| Schema op count | ~212 generated | ~220+ (generated + hand-written) | Linear array scan is fine at this scale |
| Template count | 0 built-in | ~4 built-in + unlimited user | embed.FS is read-only, no perf concern |
| Preset count | 0 built-in | ~4 built-in + profile + user file | Three-tier lookup is O(1) per preset |
| CI workflow count | 0 | 7 workflows | Standard GitHub Actions limits apply |
| Binary size | ~10MB | ~10-11MB (embed.FS adds minimal) | No significant change |
| Go dependency count | 3 direct | 4 direct (add pflag) | Minimal change |

---

## Sources

All findings based on direct codebase inspection (HIGH confidence):
- cf codebase: `/Users/quan.hoang/quanhh/quanhoang/confluence-cli/`
- jr reference: `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/`
- Key jr files examined: cmd/diff.go, cmd/diff_schema.go, cmd/workflow.go, cmd/workflow_schema.go, cmd/preset.go, cmd/template.go, cmd/template_schema.go, cmd/watch.go, cmd/schema_cmd.go, cmd/root.go, cmd/gendocs/main.go, .goreleaser.yml, .github/workflows/*, internal/jsonutil/, internal/duration/, internal/preset/, internal/changelog/, internal/template/, npm/, python/, website/
