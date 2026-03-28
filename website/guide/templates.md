# Templates

Templates let you create Confluence pages from predefined patterns --- no raw storage-format HTML, no remembering field structures. Define your variables, and `cf` handles the rest.

## Built-in Templates

`cf` ships with 6 templates out of the box:

| Template | Required Variables | Optional Variables |
|----------|-------------------|-------------------|
| `blank` | `title` | `body` |
| `meeting-notes` | `title` | `date`, `attendees`, `agenda` |
| `decision` | `title` | `status`, `stakeholders`, `background`, `options`, `outcome` |
| `runbook` | `title` | `description`, `steps`, `rollback` |
| `retrospective` | `title` | `date`, `went-well`, `improve`, `actions` |
| `adr` | `title` | `status`, `context`, `decision`, `consequences` |

## Quick Start

```bash
# List all templates
cf templates list

# See what a template expects
cf templates show meeting-notes

# Create a page from a template
cf pages create --template meeting-notes \
  --spaceId 123456 \
  --var title="Q1 Review" \
  --var date="2026-03-28" \
  --var attendees="Team Alpha"
```

## Using Variables

Pass variables with `--var key=value`. Required variables must be provided --- optional ones are omitted if not set.

```bash
# Minimal --- only required vars
cf pages create --template blank --spaceId 123456 --var title="Quick Note"

# Full --- all vars filled in
cf pages create --template decision \
  --spaceId 123456 \
  --var title="Adopt Redis for caching" \
  --var status="Proposed" \
  --var stakeholders="Platform Team" \
  --var background="Current cache miss rate is 40%" \
  --var options="1. Redis\n2. Memcached\n3. In-process cache" \
  --var outcome="Redis selected for cluster support"
```

## Creating Your Own Templates

### From Scratch

```bash
cf templates create my-template
```

This creates a scaffold YAML file in your templates directory (`~/.config/cf/templates/` on Linux, `~/Library/Application Support/cf/templates/` on macOS). Edit it to define your fields and variables.

### From an Existing Page

Clone the structure of any Confluence page into a reusable template:

```bash
cf templates create prod-runbook --from-page 12345
```

`cf` fetches the page, extracts its fields (title, body), and generates a template with appropriate variables. The title becomes a required variable; everything else becomes optional with defaults based on the original page.

### Overwriting

If a template with the same name already exists:

```bash
cf templates create my-template --overwrite
```

### Template File Format

Templates are YAML files:

<div v-pre>

```yaml
name: prod-runbook
description: Template for production runbooks
variables:
  - name: title
    required: true
    description: Runbook title
  - name: steps
    required: false
    description: Step-by-step procedure
  - name: rollback
    required: false
    description: Rollback instructions
body: "<h1>{{.title}}</h1><h2>Steps</h2><p>{{.steps}}</p>{{if .rollback}}<h2>Rollback</h2><p>{{.rollback}}</p>{{end}}"
```

**Key rules:**
- Body uses Go template syntax (e.g. `{{.variable}}`)
- Use `{{if .var}}...{{end}}` for optional sections --- empty fields are automatically omitted
- Hyphenated variable names use `{{index . "my-var"}}` syntax
- User templates override built-in templates with the same name

</div>

### Template Name Rules

Names must match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`:
- Valid: `my-template`, `task_v2`, `bug123`
- Invalid: `-starts-dash`, `has space`, `template/sub`

## Examples

### Team Documentation

```bash
# Create a meeting notes page
cf pages create --template meeting-notes \
  --spaceId 123456 \
  --var title="Sprint 12 Retrospective" \
  --var date="2026-04-01" \
  --var attendees="Platform Team"

# Create an ADR
cf pages create --template adr \
  --spaceId 123456 \
  --var title="ADR-005: Use event sourcing for audit trail" \
  --var status="Accepted" \
  --var context="Need reliable audit trail for compliance" \
  --var decision="Use event sourcing pattern with append-only store" \
  --var consequences="Higher storage costs, simpler debugging"

# Create a runbook
cf pages create --template runbook \
  --spaceId 123456 \
  --var title="Database Failover Runbook" \
  --var description="Steps for failing over the primary database" \
  --var steps="1. Verify replica health\n2. Promote replica\n3. Update DNS" \
  --var rollback="1. Revert DNS\n2. Re-sync original primary"
```

### Batch Template Usage

Combine templates with `cf batch` for bulk creation. In batch mode, template variables are passed as direct keys in `args`:

```bash
echo '[
  {"command":"pages create","args":{"template":"meeting-notes","spaceId":"123456","title":"Sprint 1 Retro","date":"2026-04-01"}},
  {"command":"pages create","args":{"template":"meeting-notes","spaceId":"123456","title":"Sprint 2 Retro","date":"2026-04-15"}}
]' | cf batch
```
