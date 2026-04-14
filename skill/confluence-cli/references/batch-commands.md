# Batch Command Reference

## Command Name Mapping

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

## Examples

```bash
echo '[
  {"command": "pages get", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "workflow comment", "args": {"id": "12345", "body": "Reviewed"}},
  {"command": "diff diff", "args": {"id": "12345", "since": "2h"}}
]' | cf batch

# Or read from a file
cf batch --input ops.json
```
