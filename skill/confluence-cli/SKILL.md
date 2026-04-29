---
name: confluence-cli
description: Use when the user mentions Confluence, Atlassian wiki, or knowledge base pages, or asks to create, search, export, update, or manage Confluence pages, spaces, blog posts, comments, labels, or attachments. Also trigger on `cf` commands appearing in shell or codebase, and on phrases like "check the wiki", "publish to Confluence", "update the docs on Confluence", or "search our knowledge base".
---

# cf — Confluence Cloud CLI for AI Agents

`cf` is a Confluence Cloud CLI designed for AI agents. Every command returns structured JSON on stdout, errors as JSON on stderr, and semantic exit codes — so you can parse, branch, and retry reliably.

**Requirements:** `cf` binary on PATH. Install via `npm i -g confluence-cf`, `pip install confluence-cf`, `brew install sofq/tap/cf` (macOS/Linux), `scoop install cf` (Windows), `go install github.com/sofq/confluence-cli@latest`, or grab a prebuilt binary from [Releases](https://github.com/sofq/confluence-cli/releases).

**Sections:** [Setup](#setup) · [Discovering Commands](#discovering-commands) · [Common Operations](#common-operations) · [Token Efficiency](#token-efficiency) · [Batch Operations](#batch-operations) · [Error Handling](#error-handling) · [Global Flags](#global-flags) · [Common Mistakes](#common-mistakes) · [When NOT to Use](#when-not-to-use) · [Common Agent Patterns](#common-agent-patterns)

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
cf pages get-by-id --profile work --id 12345
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
cf schema pages get-by-id     # full schema with all flags for one operation
```

Always use `cf schema` to discover the exact command name and flags before running an unfamiliar operation.

## Common Operations

### Pages
```bash
# Fetch one page by ID (use get-by-id, not get)
cf pages get-by-id --id 12345

# List pages in a space (get accepts filters: --space-id, --title, --id as comma-separated list)
cf pages get --space-id 123456 --jq '.results[] | {id, title}'

cf pages create --space-id 123456 --title "Deploy Runbook" \
  --body "<h1>Steps</h1><p>Follow these steps...</p>"
cf pages update --id 12345 \
  --title "Deploy Runbook v2" --body "<h1>Updated Steps</h1>"
cf pages delete --id 12345
```

`cf pages get` lists pages and returns a `{results: [...]}` envelope. `cf pages get-by-id` returns a single page object. Picking the wrong one is the most common mistake. Content uses Confluence storage format (XHTML, not Markdown).

### Search with CQL
`cf search` outputs a flat JSON array of v1 search hits — no `.results` envelope. Each hit has nested `.content.id`, `.content.title`, plus `.title`, `.excerpt`, `.url`.

```bash
cf search \
  --cql "space = DEV AND type = page AND lastModified > now('-7d')" \
  --jq '.[] | {id: .content.id, title: .content.title}'
```

### Spaces
```bash
cf spaces get --jq '.results[] | {id, key: .key, name: .name}'
cf spaces get-by-id --id 123456
```

### Blog posts
```bash
cf blogposts create-blog-post --space-id 123456 --title "Sprint Recap" --body "<p>What we shipped...</p>"
cf blogposts get-blog-posts --jq '.results[] | {id, title}'
cf blogposts get-blog-post-by-id --id 99999
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

Events: `initial`, `created`, `updated`, `removed`. Always pass `--max-polls` in agent contexts — agents cannot send Ctrl-C to stop the stream. The flag is hidden from `--help` (it's marked test-only) but is stable and required for safe automation.

### Raw API call (escape hatch)
```bash
cf raw GET /pages/12345
cf raw POST /pages --body '{"spaceId":"123","title":"New Page"}'
echo '{"spaceId":"123"}' | cf raw POST /pages --body -
```

POST/PUT/PATCH require `--body`. Without it, `cf raw` will error instead of hanging on stdin.

## Token Efficiency

Confluence responses can be large (8K+ tokens for a single page). Always minimize output:

```bash
# --preset: named preset for common field combinations
cf pages get-by-id --id 12345 --preset agent    # id, title, status, spaceId
cf pages get-by-id --id 12345 --preset brief    # id, title, status

# --fields: server-side field filtering
cf pages get-by-id --id 12345 --fields id,title,status

# --jq: client-side JSON filtering
cf pages get-by-id --id 12345 --jq '{id: .id, title: .title}'

# Combine for maximum efficiency (~50 tokens vs ~8,000)
cf pages get-by-id --id 12345 --fields id,title --jq '{id: .id, title: .title}'

# Cache read-heavy data
cf spaces get --cache 5m --jq '[.results[].key]'
```

Always use `--preset` or `--fields` + `--jq`. Run `cf preset list` for available presets. See `references/presets.md` for the full preset table.

## Batch Operations

Run multiple Confluence calls in a single process. Batch only dispatches commands listed by `cf schema` — `cf search`, `cf watch`, `cf raw`, `cf configure` are NOT available in batch.

```bash
echo '[
  {"command": "pages get-by-id", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "pages get-by-id", "args": {"id": "67890"}, "jq": ".title"},
  {"command": "spaces get", "args": {}, "jq": "[.results[].key]"}
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

## Common Mistakes

- **Using `cf pages get --id X` to fetch a single page.** That command lists pages and returns a `{results: [...]}` envelope. Use `cf pages get-by-id --id X` for a single object.
- **Sending Markdown.** Bodies must be Confluence storage format (XHTML), e.g. `<h1>Title</h1><p>Body</p>`.
- **Using `--fields` on non-GET operations.** It's silently ignored.
- **Calling `cf raw POST` without `--body`.** It errors rather than reading stdin by default; pass `--body -` to read stdin.
- **Running `cf watch` without `--max-polls` in automation.** The stream never exits on its own.
- **Mixing `allowed_operations` and `denied_operations` in one profile.** Use one or the other.
- **Calling `cf search` from `cf batch`.** Search isn't in the batch dispatch map; run it as a separate step and feed IDs into a batch.
- **Assuming `cf search` returns a `.results` envelope.** It returns a flat array of merged v1 hits.

## When NOT to Use

- **Jira issues, Trello, or Bitbucket** — different products, different CLIs.
- **Confluence Server / Data Center** — `cf` targets Confluence Cloud only.
- **Local Markdown wikis or static-site docs** — no API to call.

## Common Agent Patterns

### Check-then-act: update if exists, create if not
```bash
cf pages get-by-id --id 12345 --preset agent
# Exit 0 → update; Exit 3 → create
```

### Bulk page export
```bash
# Step 1: Find page IDs (search returns a flat array; .[] not .results[])
IDS=$(cf search \
  --cql "space = DEV AND type = page" \
  --jq '[.[].content.id]')

# Step 2: Build batch payload from IDs and execute
echo "$IDS" | jq '[.[] | {command: "export export", args: {id: .}}]' | cf batch
```

### Create page with children
```bash
# Step 1: Create parent
PARENT=$(cf pages create --space-id 123456 --title "Project Docs" \
  --body "<p>Root</p>" --jq '.id' | tr -d '"')

# Step 2: Build batch payload safely with jq -n
jq -n --arg pid "$PARENT" --arg sid "123456" '[
  {command:"pages create", args:{"space-id":$sid, "parent-id":$pid, title:"Getting Started", body:"<p>Setup</p>"}},
  {command:"pages create", args:{"space-id":$sid, "parent-id":$pid, title:"Architecture", body:"<p>Design</p>"}}
]' | cf batch
```

### Validate before executing
```bash
cf pages create --space-id 123456 --title "Test" --body "<p>test</p>" --dry-run
```
