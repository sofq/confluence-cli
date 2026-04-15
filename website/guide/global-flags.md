# Global Flags

All persistent flags listed here can be used with any `cf` command. They control authentication, output formatting, caching, and request behavior.

## Summary

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--profile` | `-p` | `string` | `""` | Config profile to use |
| `--base-url` | | `string` | `""` | Confluence base URL (overrides config) |
| `--auth-type` | | `string` | `""` | Auth type: `basic`, `bearer`, or `oauth2` (overrides config) |
| `--auth-user` | | `string` | `""` | Username for basic auth (overrides config) |
| `--auth-token` | | `string` | `""` | API token or bearer token (overrides config) |
| `--preset` | | `string` | `""` | Named output preset (`agent`, `brief`, `titles`, `tree`, `meta`, `search`, `diff`) |
| `--jq` | | `string` | `""` | jq filter expression applied to the response |
| `--fields` | | `string` | `""` | Comma-separated list of fields to return (GET only) |
| `--cache` | | `duration` | `0` | Cache GET responses for this duration |
| `--pretty` | | `bool` | `false` | Pretty-print JSON output |
| `--no-paginate` | | `bool` | `false` | Disable automatic pagination |
| `--verbose` | | `bool` | `false` | Log HTTP request/response details to stderr |
| `--dry-run` | | `bool` | `false` | Print the request as JSON without executing it |
| `--timeout` | | `duration` | `30s` | HTTP request timeout |
| `--audit` | | `bool` | `false` | Enable audit logging for this invocation |
| `--audit-file` | | `string` | `""` | Path to audit log file (implies `--audit`) |

## Detailed reference

---

### `--profile` / `-p`

**Type:** `string`
**Default:** `""` (uses default profile)

Select a named configuration profile. Profiles let you switch between multiple Confluence instances without modifying environment variables.

```bash
# Use the "work" profile for this command
cf pages get --profile work --id 12345

# Short form
cf pages get -p work --id 12345
```

Profiles are created with `cf configure --profile <name>`. See [Getting Started](./getting-started#named-profiles) for setup instructions.

---

### `--base-url`

**Type:** `string`
**Default:** `""` (uses value from config or `CF_BASE_URL` env var)

Override the Confluence instance URL for a single command. Useful for one-off requests to a different instance.

```bash
cf pages get --base-url https://other.atlassian.net --id 12345
```

---

### `--auth-type`

**Type:** `string`
**Default:** `""` (uses value from config)

Override the authentication type for a single command. Accepted values: `basic`, `bearer`, `oauth2`.

::: info
`oauth2` works as a runtime override but cannot be used with `cf configure` --- OAuth2 profiles must be configured manually in `~/.config/cf/config.json` since they require `client_id`, `client_secret`, and `token_url`.
:::

```bash
cf raw GET /pages --auth-type bearer --auth-token MY_PAT
```

---

### `--auth-user`

**Type:** `string`
**Default:** `""` (uses value from config or `CF_AUTH_USER` env var)

Override the username for basic authentication. This is your Atlassian account email.

```bash
cf pages get --id 12345 --auth-user other@company.com --auth-token TOKEN
```

---

### `--auth-token`

**Type:** `string`
**Default:** `""` (uses value from config or `CF_AUTH_TOKEN` env var)

Override the API token or bearer token for a single command.

```bash
cf raw GET /pages --auth-token NEW_TOKEN
```

::: warning
Avoid passing tokens directly on the command line in shared environments. Prefer environment variables (`CF_AUTH_TOKEN`) or config file profiles for sensitive credentials.
:::

---

### `--preset`

**Type:** `string`
**Default:** `""` (no preset)

Select a named output preset that expands to a predefined set of `--fields`. Presets provide a shorthand for common field combinations, avoiding the need to remember or type out long `--fields` lists.

| Preset | Purpose |
|--------|---------|
| `agent` | Key fields for AI agent consumption |
| `brief` | Compact summary for quick scans |
| `titles` | Page/content titles only |
| `tree` | Hierarchical page tree view |
| `meta` | Metadata: dates, author, version |
| `search` | Search result essentials |
| `diff` | Version comparison fields |

```bash
# Quick agent-friendly view
cf pages get --id 12345 --preset agent

# Brief summary for scanning
cf pages get --id 12345 --preset brief

# List available presets with their field sets
cf preset list
```

If `--preset` is used together with `--fields` or `--jq`, the explicit flags take precedence.

---

### `--jq`

**Type:** `string`
**Default:** `""` (no filtering)

Apply a [jq](https://jqlang.github.io/jq/) filter expression to the JSON response. This is client-side filtering --- it runs after the response is received.

```bash
# Extract just the id and title
cf pages get --id 12345 --jq '{id: .id, title: .title}'

# Get all page titles from a search
cf search search-content \
  --cql "space = DEV AND type = page" \
  --jq '[.results[] | {id, title}]'

# Count results
cf search search-content \
  --cql "space = DEV" \
  --jq '.totalSize'
```

::: tip
Combine `--jq` with `--fields` for maximum token efficiency. `--fields` reduces the Confluence response at the server, and `--jq` shapes it into exactly what you need.
:::

---

### `--fields`

**Type:** `string`
**Default:** `""` (returns all fields)

Comma-separated list of fields to include in the response. This is server-side filtering --- Confluence only returns the requested fields, reducing response size and API load.

Only applies to GET requests.

```bash
# Return only id, title, and status
cf pages get --id 12345 --fields id,title,status

# Combine with --jq for minimal output
cf pages get --id 12345 \
  --fields id,title \
  --jq '{id: .id, title: .title}'
```

---

### `--cache`

**Type:** `duration`
**Default:** `0` (caching disabled)

Cache GET responses locally for the specified duration. Cached responses are returned without making an API call. Accepts Go duration strings: `5m`, `1h`, `30s`, etc.

Only applies to GET requests.

```bash
# Cache space list for 5 minutes
cf spaces get --cache 5m --jq '[.results[].key]'

# Cache for 1 hour
cf pages get --id 12345 --cache 1h --fields id,title
```

::: tip
Caching is especially useful for data that changes infrequently, like space lists, content types, and label metadata. This avoids redundant API calls and reduces rate limit consumption.
:::

---

### `--pretty`

**Type:** `bool`
**Default:** `false`

Pretty-print the JSON output with indentation. Useful for human inspection; agents should generally leave this off to save tokens.

```bash
cf pages get --id 12345 --pretty
```

---

### `--no-paginate`

**Type:** `bool`
**Default:** `false`

Disable automatic pagination. By default, `cf` fetches all pages for paginated endpoints and merges them into a single response. Use this flag when you only need the first page or want to handle pagination yourself.

```bash
# Only get the first page of results
cf spaces get --no-paginate
```

---

### `--verbose`

**Type:** `bool`
**Default:** `false`

Log HTTP request and response details to stderr as JSON. Stdout remains clean JSON output. Useful for debugging API interactions.

```bash
cf pages get --id 12345 --verbose
# stderr shows: {"method":"GET","url":"https://yoursite.atlassian.net/wiki/api/v2/pages/12345","status":200,"duration":"123ms"}
# stdout shows: the page JSON
```

---

### `--dry-run`

**Type:** `bool`
**Default:** `false`

Print the HTTP request that would be made as JSON, without actually executing it. Useful for debugging request payloads, especially for POST/PUT operations.

```bash
cf pages create --space-id 123456 --title "Test" --body "<p>Hello</p>" --dry-run
# Outputs the request details without calling Confluence
```

---

### `--timeout`

**Type:** `duration`
**Default:** `30s`

Set the HTTP request timeout. Accepts Go duration strings: `10s`, `1m`, `2m30s`, etc.

```bash
# Increase timeout for slow responses
cf search search-content \
  --cql "space = DEV AND type = page" \
  --timeout 2m

# Short timeout for quick checks
cf raw GET /spaces --timeout 5s
```

---

### `--audit`

**Type:** `bool`
**Default:** `false`

Enable audit logging for this invocation. Writes a JSONL entry per operation to the audit log file (`~/.config/cf/audit.log` by default). Can also be enabled per-profile with `"audit_log": true` in the config file.

```bash
cf pages get --id 12345 --audit
```

---

### `--audit-file`

**Type:** `string`
**Default:** `""` (uses `~/.config/cf/audit.log`)

Path to the audit log file. Implies `--audit`. Useful for directing audit logs to a specific location.

```bash
cf pages get --id 12345 --audit-file /var/log/cf-audit.log
```

---

## Configuration resolution order

Flags override environment variables, which override the config file:

```
CLI flags  >  Environment variables  >  Config file (profile)
```

For example, if your config file sets `base-url` to `https://a.atlassian.net` but you pass `--base-url https://b.atlassian.net`, the flag value wins.
