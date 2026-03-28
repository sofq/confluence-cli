# Getting Started

This guide walks you through installing `cf`, configuring it for your Confluence Cloud instance, and running your first commands.

## Installation

::: code-group

```bash [Homebrew]
brew install sofq/tap/cf
```

```bash [npm]
npm install -g confluence-cf
```

```bash [pip / uv]
pip install confluence-cf
# or
uv tool install confluence-cf
```

```bash [Go]
go install github.com/sofq/confluence-cli@latest
```

```bash [Scoop (Windows)]
scoop bucket add sofq https://github.com/sofq/scoop-bucket
scoop install cf
```

```bash [Docker]
docker run --rm ghcr.io/sofq/cf version
```

:::

You can also download a pre-built binary from [GitHub Releases](https://github.com/sofq/confluence-cli/releases).

Verify the installation:

```bash
cf version
# {"version":"0.x.x"}
```

## Configuration

### Basic auth (default)

The most common setup uses your Atlassian email and an API token. Generate a token at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens), then run:

```bash
cf configure \
  --base-url https://yoursite.atlassian.net \
  --token YOUR_API_TOKEN \
  --username your@email.com
```

This writes a config file to `~/.config/cf/config.json`.

To validate your credentials before saving:

```bash
cf configure \
  --base-url https://yoursite.atlassian.net \
  --token YOUR_API_TOKEN \
  --username your@email.com \
  --test
```

To test an already-saved profile:

```bash
cf configure --test                  # test default profile
cf configure --test --profile work   # test a specific profile
```

### Bearer token

For personal access tokens or service accounts that use bearer auth:

```bash
cf configure \
  --base-url https://yoursite.atlassian.net \
  --auth-type bearer \
  --token YOUR_BEARER_TOKEN
```

### OAuth2

OAuth2 requires fields (`client_id`, `client_secret`, `token_url`) that cannot be set via CLI flags, so you must configure it manually in the config file (`~/.config/cf/config.json`):

```json
{
  "profiles": {
    "default": {
      "base_url": "https://yoursite.atlassian.net",
      "auth_type": "oauth2",
      "client_id": "YOUR_CLIENT_ID",
      "client_secret": "YOUR_CLIENT_SECRET",
      "token_url": "https://auth.atlassian.com/oauth/token"
    }
  }
}
```

You can still use `--auth-type oauth2` as a runtime flag override for individual commands.

### Environment variables

Environment variables are ideal for CI pipelines and containerized agents. They override the config file:

```bash
export CF_BASE_URL=https://yoursite.atlassian.net
export CF_AUTH_TOKEN=your-api-token
export CF_AUTH_USER=your@email.com   # optional for bearer/oauth2
```

### Named profiles

If you work with multiple Confluence instances, use named profiles:

```bash
# Create a "work" profile
cf configure --base-url https://work.atlassian.net --token TOKEN_A --profile work

# Create a "personal" profile
cf configure --base-url https://personal.atlassian.net --token TOKEN_B --profile personal

# Use a specific profile
cf pages get --profile work --id 12345

# Delete a profile you no longer need
cf configure --profile work --delete
```

::: tip Configuration resolution order
CLI flags take the highest priority, followed by environment variables, followed by the config file. This means you can override any config file setting with a flag or env var on a per-command basis.
:::

### Security settings

Profiles can include operation restrictions and audit logging. Edit `~/.config/cf/config.json` directly:

```json
{
  "profiles": {
    "agent": {
      "base_url": "https://yoursite.atlassian.net",
      "auth": {"type": "basic", "username": "...", "token": "..."},
      "allowed_operations": ["pages get", "search *", "workflow *"],
      "audit_log": true
    }
  }
}
```

- `allowed_operations` / `denied_operations` --- glob patterns restricting which commands the profile can run (use one or the other, not both)
- `audit_log` --- write a JSONL entry per operation to `~/.config/cf/audit.log`

See [Global Flags](./global-flags) for `--audit` and `--audit-file` flags.

## Your first commands

### Get a page

```bash
cf pages get --id 12345
```

This returns the full JSON representation of the page on stdout.

### Search with CQL

```bash
cf search search-content \
  --cql "space = DEV AND type = page AND lastModified > now('-7d')"
```

::: info
Command names are auto-generated from Confluence's OpenAPI spec, so they can be verbose. Use `cf schema` to discover the exact name you need (see [Discovering Commands](./discovery)).
:::

### List spaces

```bash
cf spaces list
```

## Workflow commands

`cf workflow` provides high-level commands that accept simple flags instead of raw JSON. These handle common Confluence operations like moving pages, managing comments, and controlling page lifecycle.

```bash
# Move a page to a new parent
cf workflow move --id 12345 --target 67890 --position append

# Copy a page
cf workflow copy --id 12345 --target 67890 --title "Copy of Runbook"

# Publish a draft
cf workflow publish --id 12345

# Add a comment (plain text, auto-converted to storage format)
cf workflow comment --id 12345 --body "Reviewed and approved"

# Archive a page
cf workflow archive --id 12345

# View page restrictions
cf workflow restrict --id 12345 --operation read
```

See the full [workflow command reference](/commands/workflow) for all flags and options.

## Next steps

- [Filtering & Presets](./filtering) --- cut 8,000-token responses down to ~50 tokens
- [Discovering Commands](./discovery) --- explore 242 commands with `cf schema`
- [Templates](./templates) --- create pages from predefined patterns with variables
- [Global Flags](./global-flags) --- full reference for all persistent flags
- [Agent Integration](./agent-integration) --- how AI agents can use `cf` effectively
