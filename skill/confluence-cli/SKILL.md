---
name: confluence-cli
description: "Confluence Cloud CLI (`cf`) — interact with Confluence pages, spaces, blog posts, comments, labels, attachments, and any Confluence Cloud API operation through structured JSON output and semantic exit codes. Use this skill whenever the user asks about Confluence, Atlassian wiki, knowledge base pages, or any wiki/documentation management that involves Confluence — creating pages, searching content, exporting page trees, managing spaces, or automating workflows. Also trigger when you see `cf` commands in the codebase, or the user says things like 'check the wiki', 'update the docs on Confluence', 'publish to Confluence', or 'search our knowledge base'. Even casual mentions of Confluence or Atlassian wiki operations should trigger this skill."
compatibility:
  tools: ["Bash"]
  requirements: ["cf CLI binary (brew install sofq/tap/cf)"]
---

# cf — Confluence Cloud CLI for AI Agents

`cf` is a Confluence Cloud CLI designed for AI agents. Every command returns structured JSON on stdout, errors as JSON on stderr, and semantic exit codes — so you can parse, branch, and retry reliably.

**Sections:** [Setup](#setup) · [Discovering Commands](#discovering-commands) · [Common Operations](#common-operations) · [Token Efficiency](#token-efficiency) · [Batch Operations](#batch-operations) · [Error Handling](#error-handling) · [Global Flags](#global-flags) · [Common Agent Patterns](#common-agent-patterns)

**Reference files** (read when needed):
- `references/presets.md` — Full preset reference table and custom preset config
- `references/batch-commands.md` — Batch command name mapping table and examples
- `references/security.md` — Operation policies, batch limits, audit logging
- `references/troubleshooting.md` — Installation, auth errors, content format notes

## Setup

If `cf` is not configured yet, help the user set it up:

```bash
cf version                    # check if installed
cf configure --base-url https://yoursite.atlassian.net --token YOUR_API_TOKEN
```

**Auth types:** `basic` (default, username + API token), `bearer`, `oauth2` (client credentials 2LO), `oauth2-3lo` (browser flow with PKCE)

**Config resolution order:** CLI flags > config file (`~/.config/cf/config.json`)

```bash
# Named profiles for multiple Confluence instances
cf configure --base-url https://work.atlassian.net --token TOKEN --profile work
cf pages get --profile work --id 12345
cf configure --profile work --delete   # remove a profile
```

```bash
# Validate credentials before saving (or test an existing profile)
cf configure --base-url https://yoursite.atlassian.net --token TOKEN --test
cf configure --test                     # test the default profile's saved credentials
cf configure --test --profile work      # test a specific profile
```

If you get exit code 2 (auth error), the token is likely expired or wrong. Ask the user to generate a new API token at https://id.atlassian.com/manage-profile/security/api-tokens.

## Discovering Commands

`cf` has 200+ commands auto-generated from Confluence's OpenAPI spec. Discover at runtime:

```bash
cf schema                     # resource → verbs mapping (most useful overview)
cf schema --list              # all resource names only (pages, spaces, comments, ...)
cf schema pages               # all operations for a resource
cf schema pages get           # full schema with all flags for one operation
```

Always use `cf schema` to discover the exact command name and flags before running an unfamiliar operation.

## Common Operations

### Pages
```bash
cf pages get --id 12345
cf pages create --spaceId 123456 --title "Deploy Runbook" \
  --body "<h1>Steps</h1><p>Follow these steps...</p>"
cf pages update --id 12345 --version-number 3 \
  --title "Deploy Runbook v2" --body "<h1>Updated Steps</h1>"
cf pages delete --id 12345
```

Content uses Confluence storage format (XHTML, not Markdown).

### Search with CQL
```bash
cf search search-content \
  --cql "space = DEV AND type = page AND lastModified > now('-7d')" \
  --jq '.results[] | {id, title}'
```

### Spaces
```bash
cf spaces list --jq '.results[] | {id, key: .key, name: .name}'
```

### Blog posts
```bash
cf blogposts create --spaceId 123456 --title "Sprint Recap" --body "<p>What we shipped...</p>"
cf blogposts list --jq '.results[] | {id, title}'
```

### Comments, Labels, Attachments
```bash
cf workflow comment --id 12345 --body "Reviewed and approved"
cf labels add --page-id 12345 --label "reviewed"
cf labels remove --page-id 12345 --label "draft"
cf attachments upload --page-id 12345 --file ./diagram.png
cf attachments list --page-id 12345 --jq '.results[] | {id, title}'
```

### Workflow commands
```bash
cf workflow move --id 12345 --target 67890 --position append
cf workflow copy --id 12345 --target 67890 --title "Copy of Runbook"
cf workflow publish --id 12345
cf workflow archive --id 12345
cf workflow restrict --id 12345 --user "john@company.com" --operation read
```

### Diff — version comparison
```bash
cf diff --id 12345                       # all changes
cf diff --id 12345 --since 2h           # changes in last 2 hours
cf diff --id 12345 --since 2026-01-01   # changes since date
```

### Export — page content extraction
```bash
cf export --id 12345                   # single page body
cf export --id 12345 --tree            # page + all descendants
cf export --id 12345 --format storage  # raw Confluence storage format
```

### Watch for changes (NDJSON stream)
```bash
cf watch --cql "space = DEV" --interval 30s --max-polls 50
```

Events: `initial`, `created`, `updated`, `removed`. Always use `--max-polls` in automated contexts — agents cannot send Ctrl-C to stop the stream.

### Raw API call (escape hatch)
```bash
cf raw GET /wiki/api/v2/pages/12345
cf raw POST /wiki/api/v2/pages --body '{"spaceId":"123","title":"New Page"}'
echo '{"spaceId":"123"}' | cf raw POST /wiki/api/v2/pages --body -
```

POST/PUT/PATCH require `--body`. Without it, `cf raw` will error instead of hanging on stdin.

## Token Efficiency

Confluence responses can be large (8K+ tokens for a single page). Always minimize output:

```bash
# --preset: named preset for common field combinations
cf pages get --id 12345 --preset agent    # id, title, status, spaceId
cf pages get --id 12345 --preset brief    # id, title, status

# --fields: server-side field filtering
cf pages get --id 12345 --fields id,title,status

# --jq: client-side JSON filtering
cf pages get --id 12345 --jq '{id: .id, title: .title}'

# Combine for maximum efficiency (~50 tokens vs ~8,000)
cf pages get --id 12345 --fields id,title --jq '{id: .id, title: .title}'

# Cache read-heavy data
cf spaces list --cache 5m --jq '[.results[].key]'
```

Always use `--preset` or `--fields` + `--jq`. Run `cf preset list` for available presets. See `references/presets.md` for the full preset table.

## Batch Operations

Run multiple Confluence calls in a single process:

```bash
echo '[
  {"command": "pages get", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "pages get", "args": {"id": "67890"}, "jq": ".title"},
  {"command": "spaces list", "args": {}, "jq": "[.results[].key]"}
]' | cf batch
```

Batch exit code is the highest-severity code from all operations. See `references/batch-commands.md` for the full command name mapping table.

## Error Handling

Errors are structured JSON on stderr. Branch on `exit_code`:

| Exit code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Parse stdout as JSON |
| 1 | Network/unknown error | Check connectivity, retry with backoff |
| 2 | Auth failed (401/403) | Check token/credentials |
| 3 | Not found (404) | Verify resource ID — do not retry |
| 4 | Bad request (400/422) | Fix the request payload — do not retry |
| 5 | Rate limited (429) | Wait `retry_after` seconds from stderr JSON, then retry |
| 6 | Conflict (409) | Resource conflict — resolve and retry |
| 7 | Server error (5xx) | Confluence server issue — retry with backoff |

For rate limits (exit 5), parse `retry_after` from stderr JSON and wait. For connection errors (exit 1), retry with exponential backoff (1s, 2s, 4s, max 3 retries). Do not retry exit codes 3 or 4.

## Global Flags

| Flag | Description |
|------|-------------|
| `--preset <name>` | named output preset |
| `--jq <expr>` | jq filter on response |
| `--fields <list>` | comma-separated fields to return (GET only) |
| `--cache <duration>` | cache GET responses (e.g. `5m`, `1h`) |
| `--pretty` | pretty-print JSON output |
| `--no-paginate` | disable automatic pagination |
| `--dry-run` | show the request without executing it |
| `--verbose` | log HTTP details to stderr (JSON) |
| `--timeout <duration>` | HTTP request timeout (default 30s) |
| `--profile <name>` | use a named config profile |
| `--audit <path>` | NDJSON audit log file path |

## Common Agent Patterns

### Check-then-act: update if exists, create if not
```bash
# Try to get the page; check exit code
cf pages get --id 12345 --preset agent
# Exit 0 → update; Exit 3 → create
```

### Bulk page export
```bash
# Step 1: Find pages
cf search search-content \
  --cql "space = DEV AND type = page" \
  --jq '[.results[].content.id]'

# Step 2: Build batch payload from IDs and execute
echo '[
  {"command": "export export", "args": {"id": "12345"}},
  {"command": "export export", "args": {"id": "67890"}}
]' | cf batch
```

### Create page with children
```bash
# Step 1: Create parent
PARENT=$(cf pages create --spaceId 123456 --title "Project Docs" \
  --body "<p>Root</p>" --jq '.id')

# Step 2: Create children via batch
echo "[
  {\"command\": \"pages create\", \"args\": {\"spaceId\": \"123456\", \"parentId\": \"$PARENT\", \"title\": \"Getting Started\", \"body\": \"<p>Setup</p>\"}},
  {\"command\": \"pages create\", \"args\": {\"spaceId\": \"123456\", \"parentId\": \"$PARENT\", \"title\": \"Architecture\", \"body\": \"<p>Design</p>\"}}
]" | cf batch
```

### Validate before executing
```bash
cf pages create --spaceId 123456 --title "Test" --body "<p>test</p>" --dry-run
```
