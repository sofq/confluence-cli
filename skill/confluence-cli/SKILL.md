---
name: confluence-cli
description: "How to use `cf`, the agent-friendly Confluence CLI, to interact with Confluence Cloud. Use this skill whenever the user asks to work with Confluence pages, spaces, blog posts, comments, labels, attachments, or any Confluence operation — searching content, creating pages, managing spaces, exporting page trees, or automating Confluence workflows. Also use when you see `cf` commands in the codebase or the user mentions Confluence in the context of CLI tooling. Even if the user just says 'check my Confluence pages' or 'update the wiki', this skill applies."
---

# cf — Confluence CLI for AI Agents

`cf` is a Confluence Cloud CLI designed for AI agents. Every command returns structured JSON on stdout, errors as JSON on stderr, and semantic exit codes — so you can parse, branch, and retry reliably.

**Sections:** [Setup](#setup) · [Discovering Commands](#discovering-commands) · [Common Operations](#common-operations) · [Token Efficiency](#token-efficiency) · [Batch Operations](#batch-operations) · [Error Handling](#error-handling) · [Global Flags](#global-flags) · [Common Agent Patterns](#common-agent-patterns) · [Security](#security) · [Troubleshooting](#troubleshooting)

## Setup

If `cf` is not configured yet, help the user set it up:

```bash
# Check if cf is installed
cf version

# Configure with Confluence Cloud credentials
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

`cf` has 200+ commands auto-generated from Confluence's OpenAPI spec. You don't need to memorize them — discover at runtime:

```bash
cf schema                     # resource → verbs mapping (default, most useful overview)
cf schema --list              # all resource names only (pages, spaces, comments, ...)
cf schema pages               # all operations for a resource
cf schema pages get           # full schema with all flags for one operation
```

Always use `cf schema` to discover the exact command name and flags before running an unfamiliar operation.

`cf schema` (no flags) defaults to the compact resource→verbs mapping, which is the most useful starting point.

## Common Operations

### Get a page
```bash
cf pages get --id 12345
```

### Search content with CQL
```bash
cf search search-content \
  --cql "space = DEV AND type = page AND lastModified > now('-7d')" \
  --jq '.results[] | {id, title}'
```

### Create a page
```bash
# Content uses Confluence storage format (XHTML, not Markdown)
cf pages create --spaceId 123456 --title "Deploy Runbook" \
  --body "<h1>Steps</h1><p>Follow these steps...</p>"
```

### Update a page
```bash
cf pages update --id 12345 --version-number 3 \
  --title "Deploy Runbook v2" --body "<h1>Updated Steps</h1>"
```

### Delete a page
```bash
cf pages delete --id 12345
```

### List spaces
```bash
cf spaces list --jq '.results[] | {id, key: .key, name: .name}'
```

### Blog posts
```bash
cf blogposts create --spaceId 123456 --title "Sprint Recap" --body "<p>What we shipped...</p>"
cf blogposts list --jq '.results[] | {id, title}'
```

### Comments
```bash
cf workflow comment --id 12345 --body "Reviewed and approved"
```

### Labels
```bash
cf labels add --page-id 12345 --name "reviewed"
cf labels remove --page-id 12345 --name "draft"
cf labels list --page-id 12345 --jq '.results[].name'
```

### Attachments
```bash
cf attachments upload --page-id 12345 --file ./diagram.png
cf attachments list --page-id 12345 --jq '.results[] | {id, title}'
cf attachments delete --id 67890
```

### Workflow commands
```bash
# Move a page under another page
cf workflow move --id 12345 --target 67890 --position append

# Copy a page (async — waits for completion by default)
cf workflow copy --id 12345 --target 67890 --title "Copy of Runbook"

# Publish a draft
cf workflow publish --id 12345

# Archive a page
cf workflow archive --id 12345

# Restrict access
cf workflow restrict --id 12345 --user "john@company.com" --operation read
```

### Diff — version comparison
```bash
# All changes
cf diff --id 12345

# Changes in last 2 hours
cf diff --id 12345 --since 2h

# Changes since a specific date
cf diff --id 12345 --since 2026-01-01
```

### Export — page content extraction
```bash
cf export --id 12345                   # single page body
cf export --id 12345 --tree            # page + all descendants
cf export --id 12345 --format storage  # raw Confluence storage format
```

### Watch for changes (NDJSON stream)
```bash
# Poll a CQL query and emit events as NDJSON (one JSON object per line)
cf watch --cql "space = DEV" --interval 30s --max-events 50

# Watch a specific space
cf watch --cql "space = DEV AND type = page" --interval 1m --max-events 20
```

Events: `initial` (first poll), `created`, `updated`, `removed`.

**Important:** Always use `--max-events` when calling from an automated/agent context — agents cannot send Ctrl-C (SIGINT) to stop the stream.

### Raw API call (escape hatch)
```bash
# For any endpoint not covered by generated commands
cf raw GET /wiki/api/v2/pages/12345
cf raw POST /wiki/api/v2/pages --body '{"spaceId":"123","title":"New Page"}'
cf raw POST /wiki/api/v2/pages --body @request.json
# Read body from stdin (must use --body - explicitly)
echo '{"spaceId":"123","title":"New"}' | cf raw POST /wiki/api/v2/pages --body -
```

**Note:** POST/PUT/PATCH require `--body`. Without it, `cf raw` will error instead of hanging on stdin.

## Token Efficiency

Confluence responses can be large (8K+ tokens for a single page). Always minimize output:

```bash
# --preset: use a named preset for common field combinations
cf pages get --id 12345 --preset agent    # id, title, status, spaceId
cf pages get --id 12345 --preset brief    # id, title, status
cf pages get --id 12345 --preset meta     # id, title, version, dates

# List all available presets
cf preset list

# --fields: tell Confluence to return only these fields (server-side filtering)
cf pages get --id 12345 --fields id,title,status

# --jq: filter the JSON response (client-side filtering)
cf pages get --id 12345 --jq '{id: .id, title: .title}'

# Combine both for maximum efficiency (~50 tokens vs ~8,000)
cf pages get --id 12345 --fields id,title --jq '{id: .id, title: .title}'

# Cache read-heavy data to avoid redundant API calls
cf spaces list --cache 5m --jq '[.results[].key]'
```

**Always use `--preset` or `--fields` + `--jq`.** `--preset` gives you common field sets with zero effort. `--fields` reduces what Confluence sends back, `--jq` shapes the output into exactly what you need.

### Preset reference

| Preset | Description |
|--------|-------------|
| `brief` | id, title, status |
| `titles` | id, title only |
| `agent` | id, title, status, spaceId — optimized for AI agents |
| `tree` | hierarchical page tree view |
| `meta` | id, title, version, created/modified dates |
| `search` | search result fields |
| `diff` | version comparison fields |

User-defined presets can include a `jq` filter; store them in profile config or `~/.config/cf/presets.json`.

## Batch Operations

When you need multiple Confluence calls, use `cf batch` to run them in a single process:

```bash
echo '[
  {"command": "pages get", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "pages get", "args": {"id": "67890"}, "jq": ".title"},
  {"command": "spaces list", "args": {}, "jq": "[.results[].key]"}
]' | cf batch
```

**Batch exit code:** The process exit code is the highest-severity exit code from all operations. Check individual `exit_code` fields for per-operation status.

### Batch command names

Batch uses `"resource verb"` strings matching `cf schema` output:

| CLI command | Batch `"command"` string |
|---|---|
| `cf pages get` | `"pages get"` |
| `cf pages create` | `"pages create"` |
| `cf pages update` | `"pages update"` |
| `cf workflow move` | `"workflow move"` |
| `cf workflow copy` | `"workflow copy"` |
| `cf workflow archive` | `"workflow archive"` |
| `cf workflow comment` | `"workflow comment"` |
| `cf search search-content` | `"search search-content"` |
| `cf diff` | `"diff diff"` |
| `cf export` | `"export export"` |

Note: `"diff diff"` and `"export export"` are correct — the resource and verb share the same name.

```bash
echo '[
  {"command": "pages get", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "workflow comment", "args": {"id": "12345", "body": "Reviewed"}},
  {"command": "diff diff", "args": {"id": "12345", "since": "2h"}}
]' | cf batch

# Or read from a file
cf batch --input ops.json
```

## Error Handling

Errors are structured JSON on stderr. Branch on `exit_code` and `error_type`:

| Exit code | error_type | Meaning | What to do |
|-----------|-----------|---------|------------|
| 0 | — | Success | Parse stdout as JSON |
| 1 | `connection_error` | Network/unknown error | Check connectivity, retry |
| 2 | `auth_failed` | Auth failed (401/403) | Check token/credentials |
| 3 | `not_found` | Resource not found (404) | Verify page ID / resource ID |
| 4 | `validation_error` | Bad request (400/422) | Fix the request payload |
| 5 | `rate_limited` | Rate limited (429) | Wait `retry_after` seconds, then retry |
| 6 | `permission_denied` | Permission denied | Check access / space permissions |

Error JSON includes optional `hint` (actionable recovery text) and `retry_after` (integer seconds for rate limits):

```json
{
  "error_type": "rate_limited",
  "status": 429,
  "message": "Rate limit exceeded",
  "hint": "You are being rate limited. Wait before retrying.",
  "retry_after": 30,
  "request": {"method": "GET", "path": "/wiki/api/v2/pages/12345"}
}
```

### Retry pattern for agents

For rate limits (exit 5) and general errors (exit 1), retry with backoff:

```
1. Run the command
2. If exit code is 5 (rate_limited):
   - Parse stderr JSON, read "retry_after" (integer seconds)
   - Wait that many seconds, then retry (max 3 retries)
3. If exit code is 1 (connection error):
   - Retry with exponential backoff: wait 1s, 2s, 4s (max 3 retries)
4. If exit code is 3 (not_found) or 4 (validation_error):
   - Do NOT retry — fix the request or report to user
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--preset <name>` | named output preset (agent, brief, titles, meta, tree, search, diff) |
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

# If exit code 0 → page exists, update it
cf pages update --id 12345 --version-number 3 --body "<p>Updated content</p>"

# If exit code 3 (not_found) → create it
cf pages create --spaceId 123456 --title "New Page" --body "<p>Content</p>"
```

### Bulk page export

Search + batch to export all pages in a space:

```bash
# Step 1: Find pages
PAGES=$(cf search search-content \
  --cql "space = DEV AND type = page" \
  --jq '[.results[].content.id]')

# Step 2: Build batch payload and execute
echo '[
  {"command": "export export", "args": {"id": "12345"}},
  {"command": "export export", "args": {"id": "67890"}}
]' | cf batch
```

### Create page with children

```bash
# Step 1: Create the parent page
PARENT=$(cf pages create --spaceId 123456 --title "Project Docs" \
  --body "<p>Project documentation root</p>" --jq '.id')

# Step 2: Create child pages (use batch for efficiency)
echo "[
  {\"command\": \"pages create\", \"args\": {\"spaceId\": \"123456\", \"parentId\": \"$PARENT\", \"title\": \"Getting Started\", \"body\": \"<p>Setup guide</p>\"}},
  {\"command\": \"pages create\", \"args\": {\"spaceId\": \"123456\", \"parentId\": \"$PARENT\", \"title\": \"Architecture\", \"body\": \"<p>System design</p>\"}},
  {\"command\": \"pages create\", \"args\": {\"spaceId\": \"123456\", \"parentId\": \"$PARENT\", \"title\": \"Runbook\", \"body\": \"<p>Operations guide</p>\"}}
]" | cf batch
```

### Validate before executing

Use `--dry-run` to preview requests before making API calls:

```bash
# Preview what would be sent (no API call made)
cf pages create --spaceId 123456 --title "Test" --body "<p>test</p>" --dry-run

# If the output looks correct, run without --dry-run
cf pages create --spaceId 123456 --title "Test" --body "<p>test</p>"
```

## Security

### Operation Policy (per profile)
Restrict which operations a profile can execute:

```json
{
  "profiles": {
    "agent": {
      "allowed_operations": ["pages get", "search *", "workflow *"]
    },
    "readonly": {
      "denied_operations": ["* delete*", "workflow *", "raw *"]
    }
  }
}
```

- Use `allowed_operations` OR `denied_operations`, not both
- Patterns use glob matching: `*` matches any sequence
- `allowed_operations`: implicit deny-all, only matching ops run
- `denied_operations`: implicit allow-all, only matching ops blocked

### Batch Limits
Default max batch size is 50. Override with `--max-batch N`.

### Audit Logging
Enable per-invocation with `--audit <path>`.
Logs NDJSON with: timestamp, user, profile, operation, exit code.

## Pagination

`cf` auto-paginates by default — it fetches all pages and merges results into a single response. Use `--no-paginate` if you only need the first page.

## Troubleshooting

**"command not found"** — `cf` is not installed. Install via:
```bash
brew install sofq/tap/cf          # Homebrew
go install github.com/sofq/confluence-cli@latest  # Go
```

**Exit code 2 (auth)** — Token expired or misconfigured. Test with:
```bash
cf configure --test
```

**Unknown command** — Command names are auto-generated from Confluence's API. Use `cf schema` to find the right name, or use `cf raw` as an escape hatch.

**Large responses filling context** — Always use `--preset` or `--fields` + `--jq` to minimize output.

**Content format** — Confluence uses Atlassian Storage Format (XHTML-based, not Markdown). Pass storage format through as-is. Example: `<h1>Title</h1><p>Content</p>`

**"--dry-run"** — Use this to preview what `cf` will send without making the API call.

---

> **Note:** The project root also contains `CLAUDE.md` with a compressed quick-reference for `cf`. If both files are loaded in context, prefer this SKILL.md as the authoritative reference — `CLAUDE.md` is a contributor-oriented summary.
