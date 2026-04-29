# Batch Command Reference

Batch dispatches commands by `"resource verb"` strings matching `cf schema` output. Hand-written commands outside the schema (`cf search`, `cf watch`, `cf raw`, `cf configure`) are NOT available in batch — call them directly.

## Command Name Mapping

| CLI command | Batch `"command"` string |
|---|---|
| `cf pages get` (list pages) | `"pages get"` |
| `cf pages get-by-id` (single page) | `"pages get-by-id"` |
| `cf pages create` | `"pages create"` |
| `cf pages update` | `"pages update"` |
| `cf pages delete` | `"pages delete"` |
| `cf spaces get` | `"spaces get"` |
| `cf spaces get-by-id` | `"spaces get-by-id"` |
| `cf blogposts get-blog-posts` | `"blogposts get-blog-posts"` |
| `cf blogposts get-blog-post-by-id` | `"blogposts get-blog-post-by-id"` |
| `cf workflow move` | `"workflow move"` |
| `cf workflow copy` | `"workflow copy"` |
| `cf workflow archive` | `"workflow archive"` |
| `cf workflow publish` | `"workflow publish"` |
| `cf workflow comment` | `"workflow comment"` |
| `cf diff` | `"diff diff"` |
| `cf export` | `"export export"` |

Note: `"diff diff"` and `"export export"` are correct — the resource and verb share the same name.

## Examples

```bash
echo '[
  {"command": "pages get-by-id", "args": {"id": "12345"}, "jq": ".title"},
  {"command": "workflow comment", "args": {"id": "12345", "body": "Reviewed"}},
  {"command": "diff diff", "args": {"id": "12345", "since": "2h"}}
]' | cf batch

# Or read from a file
cf batch --input ops.json
```
