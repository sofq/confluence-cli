# Preset Reference

## Built-in Presets

| Preset | Fields |
|--------|--------|
| `brief` | id, title, status |
| `titles` | id, title only |
| `agent` | id, title, status, spaceId — optimized for AI agents |
| `tree` | hierarchical page tree view |
| `meta` | id, title, version, created/modified dates |
| `search` | search result fields |
| `diff` | version comparison fields |

## Usage

```bash
cf pages get --id 12345 --preset agent
cf pages get --id 12345 --preset brief
cf preset list                           # list all available presets
```

## Custom Presets

User-defined presets can include a `jq` filter. Store them in profile config or `~/.config/cf/presets.json`.
