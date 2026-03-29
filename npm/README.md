# confluence-cf

**Confluence CLI built for AI agents** — pure JSON output, semantic exit codes, 200+ auto-generated commands, and built-in jq filtering.

Give your AI agent (Claude Code, Cursor, Copilot, or custom bots) reliable, token-efficient access to Confluence Cloud.

## Install

```bash
npm install -g confluence-cf
```

## Why cf?

```bash
# Full Confluence response: ~8,000 tokens
cf pages get --id 12345

# With cf's filtering: ~50 tokens
cf pages get --id 12345 --fields id,title --jq '{id: .id, title: .title}'
```

- **All output is JSON** — stdout for data, stderr for errors, always
- **Semantic exit codes** — 0=ok, 2=auth, 3=not_found, 5=rate_limited — agents can branch without parsing
- **200+ commands** from the official Confluence OpenAPI spec, synced daily
- **Batch operations** — N API calls in one process via `cf batch`
- **Self-describing** — `cf schema` lets agents discover commands at runtime
- **Workflow helpers** — `cf workflow move`, `cf workflow copy`, `cf workflow archive`

## Quick start

```bash
# Configure
cf configure --base-url https://yoursite.atlassian.net --token YOUR_API_TOKEN

# Search pages
cf search search-content --cql "space = DEV AND type = page" \
  --jq '.results[] | {id, title}'

# Export a page tree
cf export --id 12345 --tree

# Batch operations
echo '[
  {"command":"pages get","args":{"id":"12345"},"jq":".title"},
  {"command":"pages get","args":{"id":"67890"},"jq":".title"}
]' | cf batch
```

## Also available via

- **Homebrew**: `brew install sofq/tap/cf`
- **pip**: `pip install confluence-cf`
- **Scoop**: `scoop bucket add sofq https://github.com/sofq/scoop-bucket && scoop install cf`
- **Docker**: `docker run --rm ghcr.io/sofq/cf version`
- **Go**: `go install github.com/sofq/confluence-cli@latest`

## Documentation

Full docs, Claude Code skill, and source at [github.com/sofq/confluence-cli](https://github.com/sofq/confluence-cli).
