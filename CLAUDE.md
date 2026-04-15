# cf — Agent-Friendly Confluence CLI

## Quick Start
```
cf configure --base-url https://yoursite.atlassian.net --token YOUR_API_TOKEN
cf configure --test                        # test saved credentials
cf configure --test --profile work         # test a specific profile
cf configure --profile myprofile --delete  # remove a profile
```

## Key Patterns

- **All output is JSON** on stdout. Errors are JSON on stderr.
- **Exit codes are semantic**: 0=ok, 1=error, 2=auth, 3=not_found, 4=validation, 5=rate_limited, 6=conflict, 7=server
- **Use `--preset`** for common field sets: `cf pages get --id 12345 --preset agent` (presets: agent, brief, titles, meta, tree, search, diff)
- **Use `--jq`** to reduce output tokens: `cf pages get --id 12345 --jq '{id: .id, title: .title}'`
- **Use `--fields`** to limit Confluence response fields: `cf pages get --id 12345 --fields id,title,status`
- **Use `cf batch`** to run multiple operations in one call
- **Use `cf raw`** for any API endpoint not covered by generated commands
- **Content is XHTML** (Confluence storage format), not Markdown

## Common Operations

```bash
# Get page
cf pages get --id 12345

# Search content with CQL
cf search search-content --cql "space = DEV AND type = page" --jq '.results[] | {id, title}'

# Create page (content in Confluence storage format — XHTML)
cf pages create --space-id 123456 --title "New Page" --body "<p>Content here</p>"

# Update page (auto-increments version number)
cf pages update --id 12345 --title "Updated" --body "<p>New content</p>"

# Delete page
cf pages delete --id 12345

# List spaces
cf spaces get --jq '.results[] | {id, key: .key, name: .name}'

# Blog posts
cf blogposts create-blog-post --space-id 123456 --title "Sprint Recap" --body "<p>What we shipped</p>"
cf blogposts get-blog-posts --jq '.results[] | {id, title}'

# Comments
cf workflow comment --id 12345 --body "Reviewed and approved"

# Labels
cf labels add --page-id 12345 --label "reviewed"
cf labels remove --page-id 12345 --label "draft"

# Attachments
cf attachments upload --page-id 12345 --file ./diagram.png
cf attachments list --page-id 12345

# Workflow commands
cf workflow move --id 12345 --target 67890 --position append
cf workflow copy --id 12345 --target 67890 --title "Copy of Runbook"
cf workflow archive --id 12345
cf workflow restrict --id 12345 --user "john@company.com" --operation read

# Diff — structured version comparison
cf diff --id 12345                           # all changes
cf diff --id 12345 --since 2h               # changes in last 2 hours
cf diff --id 12345 --since 2026-01-01       # changes since date

# Export — page content extraction
cf export --id 12345                         # single page body
cf export --id 12345 --tree                  # page + all descendants
cf export --id 12345 --format storage        # raw storage format

# Raw API call (path is relative to base URL; POST/PUT/PATCH require --body)
cf raw GET /pages/12345
cf raw POST /pages --body '{"spaceId":"123","title":"New"}'
echo '{"spaceId":"123"}' | cf raw POST /pages --body -  # stdin

# Batch operations
echo '[{"command":"pages get","args":{"id":"12345"},"jq":".title"},{"command":"pages get","args":{"id":"67890"},"jq":".title"}]' | cf batch

# Watch for changes (NDJSON stream — always use --max-polls in automated contexts)
cf watch --cql "space = DEV" --interval 30s --max-polls 50

```

## Discovery

```bash
cf schema                     # resource → verbs mapping (default)
cf schema --list              # all resource names only
cf schema pages               # all operations for 'pages'
cf schema pages get           # full schema with flags for one operation
cf preset list                # list available output presets
```

## Batch Command Names

In `cf batch`, use `"resource verb"` strings. Notable: `"diff diff"` (not just `"diff"`), `"export export"` (not just `"export"`).

```bash
echo '[
  {"command": "pages get", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "workflow comment", "args": {"id": "12345", "body": "Reviewed"}},
  {"command": "diff diff", "args": {"id": "12345", "since": "2h"}}
]' | cf batch
```

Batch exit code = highest-severity code from all operations.

## Global Flags

| Flag | Description |
|------|-------------|
| `--preset <name>` | named output preset (agent, brief, titles, meta, tree, search, diff) |
| `--jq <expr>` | jq filter on response |
| `--fields <list>` | comma-separated fields to return (GET only) |
| `--cache <duration>` | cache GET responses (e.g. 5m, 1h) |
| `--pretty` | pretty-print JSON |
| `--no-paginate` | disable auto-pagination |
| `--dry-run` | show request without executing |
| `--verbose` | log HTTP details to stderr (JSON) |
| `--timeout <duration>` | HTTP request timeout (default 30s) |
| `--profile <name>` | use named config profile |
| `--audit <path>` | NDJSON audit log file path |
| `--max-batch <N>` | max operations per batch (default 50, batch command only) |

## Security

### Operation Policy (per profile)
Restrict which operations a profile can execute:

```json
{
  "profiles": {
    "agent": {
      "base_url": "...",
      "auth": {"type": "basic", "token": "..."},
      "allowed_operations": ["pages get", "search *", "workflow *"]
    },
    "readonly": {
      "base_url": "...",
      "auth": {"type": "basic", "token": "..."},
      "denied_operations": ["* delete*", "workflow *", "raw *"]
    }
  }
}
```

Rules:
- Use `allowed_operations` OR `denied_operations`, not both
- Patterns use glob matching: `*` matches any sequence
- `allowed_operations`: implicit deny-all, only matching ops run
- `denied_operations`: implicit allow-all, only matching ops blocked

### Batch Limits
Default max batch size is 50. Override with `--max-batch N`.

### Audit Logging
Enable per-invocation with `--audit <path>`. Logs NDJSON to the specified file.

## Development

```bash
make generate    # regenerate commands from OpenAPI spec
make build       # build binary
make test        # run tests
make lint        # run golangci-lint
```
