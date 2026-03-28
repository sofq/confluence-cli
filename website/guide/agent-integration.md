# Agent Integration

`cf` is designed from the ground up for AI agents and LLM-powered tools. Every command returns structured JSON on stdout, errors as JSON on stderr, and semantic exit codes --- so agents can parse, branch, and retry reliably.

## Why cf for Agents

| Property | How cf handles it |
|----------|-------------------|
| **Output format** | JSON on stdout, always. Errors on stderr, always. No mixed output. |
| **Error handling** | Semantic exit codes (0-7) --- branch without parsing error messages |
| **Token efficiency** | `--preset`, `--fields` + `--jq` reduce 8,000-token responses to ~50 tokens |
| **Workflow commands** | `cf workflow` commands accept simple flags --- no JSON body construction needed |
| **Discovery** | `cf schema` for runtime command discovery --- no hardcoding needed |
| **Batch operations** | `cf batch` runs multiple commands in a single process |
| **Security controls** | Per-profile operation allowlist/denylist, batch limits, audit logging |
| **242 commands** | Auto-generated from Confluence's OpenAPI spec, always up to date |

## Runtime Discovery

Agents should use `cf schema` to discover commands dynamically rather than hardcoding command names:

```bash
# Get all resources (compact overview)
cf schema

# List just resource names
cf schema --list

# See all verbs for a resource
cf schema pages

# Get full flag reference for one operation
cf schema pages get
```

::: tip
Always use `cf schema` to discover the exact command name and flags before running an unfamiliar operation. Command names come from Confluence's API and can be verbose (e.g., `search-content`).
:::

### Discovery flow for agents

```
1. cf schema --list          -> pick the right resource
2. cf schema <resource>      -> pick the right verb
3. cf schema <resource> <verb> -> see required flags
4. cf <resource> <verb> --flags...  -> execute
```

## Workflow Commands

`cf workflow` provides high-level commands that accept simple flags instead of raw JSON bodies. These handle common Confluence operations like moving pages, managing comments, and controlling page lifecycle.

```bash
# Move a page to a new parent
cf workflow move --id 12345 --target 67890 --position append

# Copy a page under a new parent
cf workflow copy --id 12345 --target 67890 --title "Copy of Runbook"

# Publish a draft page
cf workflow publish --id 12345

# Add a comment (plain text, auto-converted to storage format)
cf workflow comment --id 12345 --body "Reviewed and approved"

# Archive a page
cf workflow archive --id 12345

# View page restrictions
cf workflow restrict --id 12345 --operation read
```

::: tip
Workflow commands save significant tokens compared to constructing raw JSON bodies. For example, `cf workflow move` replaces a multi-step lookup-and-PUT sequence with a single flag-based command.
:::

## Watch for Changes

`cf watch` polls Confluence on an interval and emits change events as NDJSON (one JSON object per line). This enables autonomous agents to monitor Confluence without repeated manual searches.

```bash
# Poll a CQL query every 30 seconds
cf watch --cql "space = DEV AND type = page" --interval 30s

# Watch an entire space for changes every minute
cf watch --cql "space = DEV" --interval 1m --preset agent

# Stop after N events
cf watch --cql "space = DEV" --max-events 10
```

Event types:
- `initial` --- emitted on first poll for all matching content
- `created` --- new content appeared in results
- `updated` --- content's `lastModified` timestamp changed
- `removed` --- content no longer matches the query

Each event is one JSON line: `{"event":"updated","id":"12345","title":"Deploy Runbook","version":5,"when":"2026-03-28T10:15:00Z"}`

Use Ctrl-C (SIGINT) to stop gracefully, or use `--max-events` to stop after a fixed number of events.

::: warning
Always use `--max-events` when calling from an automated/agent context --- agents cannot send Ctrl-C to stop the stream.
:::

## Token Efficiency

Confluence responses can be enormous. A single page response can consume ~8,000 tokens. Several mechanisms solve this:

### Output presets

Use `--preset` to select a predefined set of fields without remembering `--fields` + `--jq` combos:

```bash
# Agent-friendly preset: key fields for AI consumption
cf pages get --id 12345 --preset agent

# Brief preset: compact summary for scanning
cf pages get --id 12345 --preset brief

# Titles only: just page titles
cf spaces list --preset titles

# List all available presets
cf preset list
```

### Manual field filtering

For custom field combinations, use `--fields` and `--jq`:

```bash
# --fields: server-side filtering (tell Confluence what to return)
# --jq: client-side filtering (reshape the JSON locally)

# Without filtering: ~8,000 tokens
cf pages get --id 12345

# With filtering: ~50 tokens
cf pages get --id 12345 \
  --fields id,title,status \
  --jq '{id: .id, title: .title, status: .status}'
```

::: warning
Always use `--preset` or `--fields` + `--jq` to minimize output. A single unfiltered page can consume 8,000+ tokens of an agent's context window.
:::

See [Filtering & Presets](./filtering) for the full guide on reducing token usage.

### Caching repeated reads

Use `--cache` to avoid redundant API calls for stable data:

```bash
# Cache space list for 5 minutes
cf spaces list --cache 5m --jq '[.results[].key]'
```

## Batch Operations

When an agent needs multiple Confluence calls, use `cf batch` to run them in a single process invocation:

```bash
echo '[
  {"command": "pages get", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "pages get", "args": {"id": "67890"}, "jq": ".title"},
  {"command": "spaces list", "args": {}, "jq": "[.results[].key]"}
]' | cf batch
```

Output is an array of results:

```json
[
  {"index": 0, "exit_code": 0, "data": "Deploy Runbook"},
  {"index": 1, "exit_code": 0, "data": "Architecture Overview"},
  {"index": 2, "exit_code": 0, "data": ["DEV", "OPS", "TEAM"]}
]
```

Each result includes `index`, `exit_code`, and either `data` (on success) or `error` (on failure). This avoids spawning N subprocesses.

## Error Handling for Agents

Errors are structured JSON on stderr. Branch on exit codes, not error messages:

| Exit Code | Error Type | Meaning | Agent Action |
|-----------|-----------|---------|--------------|
| 0 | --- | Success | Parse stdout as JSON |
| 1 | `connection_error` | Network/unknown error | Check connectivity, retry |
| 2 | `auth_failed` | Auth failed (401/403) | Check token/credentials |
| 3 | `not_found` | Not found (404) | Verify page ID/resource ID |
| 3 | `gone` | Resource gone (410) | Resource was deleted; do not retry |
| 4 | `validation_error` | Bad request (400/422) | Fix the request payload |
| 4 | `client_error` | Other 4xx errors | Check request parameters |
| 5 | `rate_limited` | Rate limited (429) | Wait `retry_after` seconds, then retry |
| 6 | `conflict` | Conflict (409) | Fetch latest and retry |
| 7 | `server_error` | Server error (5xx) | Retry with backoff |

Example error response:

```json
{
  "error_type": "not_found",
  "status": 404,
  "message": "Page Does Not Exist",
  "request": {"method": "GET", "path": "/wiki/api/v2/pages/99999"}
}
```

Error JSON may also include `hint` (actionable recovery text) and `retry_after` (integer seconds for rate limits):

```json
{
  "error_type": "rate_limited",
  "status": 429,
  "message": "Rate limit exceeded",
  "hint": "You are being rate limited. Wait before retrying.",
  "retry_after": 30,
  "request": {"method": "GET", "path": "/wiki/api/v2/pages"}
}
```

::: tip
For exit code 5 (rate limited), the error JSON includes a `retry_after` field with the number of seconds to wait.
:::

## Skill Setup

`cf` ships with a skill file that teaches AI agents everything on this page automatically. The skill follows the [Agent Skills](https://agentskills.io) open standard, supported by 30+ tools.

See the **[Skill Setup Guide](/guide/skill-setup)** for installation instructions for Claude Code, Cursor, VS Code Copilot, OpenAI Codex, Gemini CLI, Goose, Roo Code, and more.

## Troubleshooting

### "command not found"

`cf` is not installed. Install via:

```bash
brew install sofq/tap/cf          # Homebrew
go install github.com/sofq/confluence-cli@latest  # Go
```

### Exit code 2 (auth)

Token expired or misconfigured. Verify with:

```bash
cf configure --test
```

If this fails, generate a new API token at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens).

### Unknown command

Command names are auto-generated from Confluence's API and can be verbose. Use `cf schema` to find the right name:

```bash
cf schema --list          # find the resource
cf schema <resource>      # find the verb
```

Or use `cf raw` as an escape hatch for any API endpoint.

### Large responses filling context

::: danger
Always use `--preset` or `--fields` + `--jq` to minimize output. A single unfiltered page can consume 8,000+ tokens of an agent's context window.
:::

### Dry-run before mutations

Use `--dry-run` to preview what `cf` will send without making the API call:

```bash
cf pages create --spaceId 123456 --title "Test" --body "<p>Hello</p>" --dry-run
```
