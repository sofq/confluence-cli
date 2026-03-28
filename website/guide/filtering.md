# Filtering & Presets

Confluence API responses are large. A single page can be 8,000+ tokens of JSON. `cf` provides several ways to cut that down dramatically.

## `--preset` --- named output presets

The `--preset` flag selects a predefined combination of fields, so you don't need to remember which fields to request:

```bash
# Get key fields for agent use
cf pages get --id 12345 --preset agent

# Get a brief summary
cf pages get --id 12345 --preset brief
```

Available presets: `agent`, `brief`, `titles`, `tree`, `meta`, `search`, `diff`. Run `cf preset list` to see all presets and what fields they include.

## `--fields` --- server-side filtering

The `--fields` flag tells Confluence to return only the fields you ask for. This reduces what comes over the wire:

```bash
cf pages get --id 12345 --fields id,title,status
```

## `--jq` --- client-side filtering

The `--jq` flag applies a [jq](https://jqlang.github.io/jq/) expression to the response, reshaping or extracting data:

```bash
cf pages get --id 12345 --jq '{id: .id, title: .title, status: .status}'
```

## Combine both for maximum efficiency

Use `--fields` and `--jq` together to minimize tokens at every stage:

**Before** (no filtering) --- ~8,000 tokens:
```bash
cf pages get --id 12345
```

**After** (both flags) --- ~50 tokens:
```bash
cf pages get --id 12345 \
  --fields id,title \
  --jq '{id: .id, title: .title}'
```

::: tip
Always use `--fields` and `--jq` together. `--fields` reduces what Confluence sends back (saving bandwidth and API quota), while `--jq` shapes the output into exactly the structure you need.
:::

## Cache read-heavy data

For data that changes infrequently (like space lists), use `--cache` to avoid redundant API calls:

```bash
cf spaces list --cache 5m --jq '[.results[].key]'
```
