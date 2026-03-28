<p align="center">
  <h1 align="center">cf</h1>
  <p align="center"><strong>The Confluence CLI that speaks JSON — built for AI agents</strong></p>
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/confluence-cf"><img src="https://img.shields.io/npm/v/confluence-cf?style=for-the-badge&logo=npm&logoColor=white&color=CB3837" alt="npm"></a>
  <a href="https://pypi.org/project/confluence-cf"><img src="https://img.shields.io/pypi/v/confluence-cf?style=for-the-badge&logo=pypi&logoColor=white&color=3775A9" alt="PyPI"></a>
  <a href="https://github.com/sofq/confluence-cli/releases"><img src="https://img.shields.io/github/v/release/sofq/confluence-cli?style=for-the-badge&logo=github&logoColor=white&color=181717" alt="GitHub Release"></a>
  <a href="https://github.com/sofq/confluence-cli/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/sofq/confluence-cli/ci.yml?style=for-the-badge&logo=githubactions&logoColor=white&label=CI" alt="CI"></a>
  <a href="https://codecov.io/gh/sofq/confluence-cli"><img src="https://img.shields.io/codecov/c/github/sofq/confluence-cli?style=for-the-badge&logo=codecov&logoColor=white" alt="codecov"></a>
  <a href="https://github.com/sofq/confluence-cli/security"><img src="https://img.shields.io/github/actions/workflow/status/sofq/confluence-cli/security.yml?style=for-the-badge&logo=shieldsdotio&logoColor=white&label=Security" alt="Security"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue?style=for-the-badge" alt="License"></a>
</p>

<br>

> Pure JSON stdout. Structured errors on stderr. Semantic exit codes. 200+ auto-generated commands from the Confluence OpenAPI spec. Zero prompts, zero interactivity — just pipe and parse.

---

## Install

```bash
brew install sofq/tap/cf          # macOS / Linux
npm install -g confluence-cf       # Node
pip install confluence-cf          # Python
scoop bucket add sofq https://github.com/sofq/scoop-bucket && scoop install cf  # Windows
go install github.com/sofq/confluence-cli@latest                                # Go
```

## Quick start

```bash
cf configure --base-url https://yoursite.atlassian.net --token YOUR_API_TOKEN
cf pages get --id 12345 --preset agent
```

## Why agents love cf

### Self-describing — no hardcoded command lists

```bash
cf schema                  # all resources and verbs
cf schema pages get        # full flags for one operation
```

### Token-efficient — 8K tokens to 50

```bash
cf pages get --id 12345 \
  --fields id,title,status --jq '{id: .id, title: .title, status: .status}'
```

`--preset`, `--fields`, and `--jq` stack so agents only consume what they need.

### CQL search — powerful Confluence queries

```bash
cf search search-content \
  --cql "space = DEV AND type = page AND lastModified > now('-7d')" \
  --jq '.results[] | {id, title}'
```

### Page management — create, update, diff

```bash
cf pages create --spaceId 123456 --title "Deploy Runbook" \
  --body "<h1>Steps</h1><p>Follow these steps...</p>"

cf pages update --id 12345 --version-number 3 \
  --title "Deploy Runbook v2" --body "<h1>Updated Steps</h1>"
```

### Workflow commands — move, copy, publish, archive

```bash
cf workflow move --id 12345 --target 67890 --position append
cf workflow copy --id 12345 --target 67890 --title "Copy of Runbook"
cf workflow archive --id 12345
cf workflow comment --id 12345 --body "Reviewed and approved"
```

### Watch — real-time content monitoring

```bash
cf watch --cql "space = DEV" --interval 30s --max-events 50
```

```json
{"event":"updated","id":"12345","title":"Deploy Runbook","version":5,"when":"2026-03-28T10:15:00Z"}
{"event":"created","id":"67890","title":"New RFC","version":1,"when":"2026-03-28T10:16:30Z"}
```

Events: `initial`, `created`, `updated`, `removed`.

### Templates — structured page creation

```bash
cf templates list
cf pages create --template meeting-notes --var title="Q1 Review" --var date="2026-03-28"
cf templates create my-template --from 12345
```

Built-in: `meeting-notes`, `decision`, `retrospective`, `runbook`, `adr`, `rfc`.

### Diff — structured version comparison

```bash
cf diff --id 12345 --since 2h
```

```json
{"version_from":3,"version_to":5,"title_changed":true,"body":{"added":12,"removed":4,"changed":8}}
```

### Export — page and tree extraction

```bash
cf export --id 12345                   # single page
cf export --id 12345 --tree            # page + all descendants
cf export --id 12345 --format storage  # raw Confluence storage format
```

### Batch — N operations, one process

```bash
echo '[
  {"command":"pages get","args":{"id":"12345"},"jq":".title"},
  {"command":"pages get","args":{"id":"67890"},"jq":".title"}
]' | cf batch
```

### Error contract — predictable exit codes

```json
{"error_type":"rate_limited","status":429,"retry_after":30}
```

| Exit | Meaning       | Agent action        |
|------|---------------|---------------------|
| 0    | OK            | Parse stdout        |
| 1    | General error | Check stderr        |
| 2    | Auth failed   | Re-authenticate     |
| 3    | Not found     | Check page ID       |
| 4    | Validation    | Fix input           |
| 5    | Rate limited  | Wait `retry_after`  |
| 6    | Permission    | Check access        |

### Raw escape hatch

```bash
cf raw --method GET --path "/wiki/api/v2/pages/12345"
cf raw --method POST --path "/wiki/api/v2/pages" --body '{"spaceId":"123","title":"New Page"}'
```

## Agent integration

### Claude Code skill (included)

```bash
cp -r skill/confluence-cli ~/.claude/skills/    # global
```

### Any agent

Add to your agent's instructions:

```
Use `cf` for all Confluence operations. Output is always JSON.
Use `cf schema` to discover available commands.
Use `--jq` or `--preset agent` to minimize token usage.
Errors go to stderr with semantic exit codes.
```

## Security

**Operation policies** — restrict per profile with glob patterns:

```json
{"allowed_operations": ["pages get", "search *", "workflow *"]}
```

**Audit logging** — NDJSON to `~/.config/cf/audit.log` via `--audit` flag or per-profile config.

**Batch limits** — default 50, override with `--max-batch N`.

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## Development

```bash
make build          # Build binary
make test           # Run all tests
make lint           # Run golangci-lint
make generate       # Regenerate commands from OpenAPI spec
make spec-update    # Download latest Confluence OpenAPI spec
make docs-dev       # Serve documentation locally
```

| Target            | Description                                   |
|-------------------|-----------------------------------------------|
| `make build`      | Build the `cf` binary                         |
| `make test`       | Run all tests with race detection             |
| `make lint`       | Run golangci-lint                             |
| `make generate`   | Regenerate commands from Confluence OpenAPI    |
| `make spec-update`| Download latest Confluence OpenAPI spec        |
| `make docs-dev`   | Serve VitePress documentation locally          |
| `make docs-build` | Build static documentation site               |
| `make clean`      | Remove build artifacts                        |

## License

[Apache 2.0](LICENSE)
