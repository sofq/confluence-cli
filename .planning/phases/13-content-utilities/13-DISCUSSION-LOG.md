# Phase 13: Content Utilities - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 13-content-utilities
**Areas discussed:** Built-in template definitions, From-page template creation, Export command design, Preset list command, Templates show command, Template list refactoring, Export depth limit, Command registration, Error handling patterns, Template variable documentation

---

## Built-in Template Definitions

### Storage approach

| Option | Description | Selected |
|--------|-------------|----------|
| Embedded Go map | Same pattern as built-in presets — map[string]*Template in source code. Consistent, no embed directive. | ✓ |
| embed.FS with JSON files | Actual .json files in internal/template/builtin/ embedded via //go:embed. Easier to read/edit XHTML. | |
| You decide | Claude picks the approach | |

**User's choice:** Embedded Go map
**Notes:** Keeps consistency with presets pattern

### Body depth

| Option | Description | Selected |
|--------|-------------|----------|
| Structural skeleton | Headings, sections, placeholder text with {{.variable}} placeholders | ✓ |
| Minimal scaffold | Just essential structure — few headings, empty sections | |
| You decide | Claude designs per template | |

**User's choice:** Structural skeleton
**Notes:** None

### Discovery

| Option | Description | Selected |
|--------|-------------|----------|
| Merged list with source tag | Same pattern as presets — all templates with source field. User overrides built-in. | ✓ |
| Separate commands | templates list shows user only, --all shows built-in too | |
| You decide | Claude picks | |

**User's choice:** Merged list with source tag
**Notes:** None

---

## From-page Template Creation

### Body capture

| Option | Description | Selected |
|--------|-------------|----------|
| Raw body save | Fetch page, save storage-format body as-is. User adds {{.variable}} later. | ✓ |
| Interactive variable detection | Scan body for variable candidates, prompt user to mark them. | |
| You decide | Claude picks simplest approach | |

**User's choice:** Raw body save
**Notes:** Simple, predictable

### Metadata

| Option | Description | Selected |
|--------|-------------|----------|
| Title + body only | Save just title and body. SpaceID left empty for reusability. | ✓ |
| Title + body + labels | Also capture page labels for auto-apply on creation. | |
| You decide | Claude decides | |

**User's choice:** Title + body only
**Notes:** None

### Save path

| Option | Description | Selected |
|--------|-------------|----------|
| User templates dir | Save to ~/.config/cf/templates/<name>.json. Requires --name flag. | ✓ |
| Current directory | Save to ./<name>.json. User moves manually. | |
| You decide | Claude picks | |

**User's choice:** User templates dir
**Notes:** None

---

## Export Command Design

### Formats

| Option | Description | Selected |
|--------|-------------|----------|
| All three | storage (default), atlas_doc_format, view — pass as body-format query param | ✓ |
| Storage only | Only raw storage format. Matches project principle. | |
| You decide | Claude decides | |

**User's choice:** All three
**Notes:** None

### Tree walk

| Option | Description | Selected |
|--------|-------------|----------|
| v2 children API | Use v2 get child pages endpoint recursively. Depth-first, emit NDJSON. | ✓ |
| CQL search | Use CQL ancestor query. Faster but loses tree structure. | |
| You decide | Claude picks | |

**User's choice:** v2 children API
**Notes:** Already generated in cmd/generated/pages.go

### NDJSON shape

| Option | Description | Selected |
|--------|-------------|----------|
| Full page body + metadata | Each line: id, title, parentId, depth, body in requested format | ✓ |
| Metadata only, body on demand | Lines contain id/title/parentId/depth. Body requires separate call. | |
| You decide | Claude designs | |

**User's choice:** Full page body + metadata
**Notes:** Agents get everything in one stream

---

## Preset List Command

### Pipeline

| Option | Description | Selected |
|--------|-------------|----------|
| Standard pipeline | Output through --jq and --pretty flags. Consistent with all commands. | ✓ |
| Direct output only | Print JSON array directly. | |
| You decide | Claude follows jr pattern | |

**User's choice:** Standard pipeline
**Notes:** None

### Profile tier

| Option | Description | Selected |
|--------|-------------|----------|
| All three tiers | Pass current profile presets to preset.List(). Shows actual resolution. | ✓ |
| Built-in + user only | Skip profile presets. Simpler but incomplete. | |
| You decide | Claude picks | |

**User's choice:** All three tiers
**Notes:** None

---

## Templates Show Command

### Output

| Option | Description | Selected |
|--------|-------------|----------|
| Full template JSON | Template struct as JSON (title, body, space_id). Same format for built-in and user. | ✓ |
| Annotated output | JSON with extra fields: detected variables, source, file path. | |
| You decide | Claude picks | |

**User's choice:** Full template JSON
**Notes:** None

---

## Template List Refactoring

### Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror preset pattern | Change template.List() to return []templateEntry (name, source) JSON. Built-in + user merged. | ✓ |
| Keep names, add --verbose | Default stays as name array, --verbose shows source. Less breaking. | |
| You decide | Claude mirrors preset pattern | |

**User's choice:** Mirror preset pattern
**Notes:** None

---

## Export Depth Limit

### Depth flag

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, unlimited default | --depth flag (0 = unlimited, default). Agents control recursion. | ✓ |
| Yes, sensible default | Default depth of 10 to prevent runaway recursion. | |
| No depth flag | Always export full tree. | |

**User's choice:** Yes, unlimited default
**Notes:** None

---

## Command Registration

### Preset command name

| Option | Description | Selected |
|--------|-------------|----------|
| Singular: cf preset list | Matches jr exactly. presetCmd parent with presetListCmd child. | ✓ |
| Plural: cf presets list | Match cf templates (plural) pattern. More grammatically consistent. | |
| You decide | Claude mirrors jr | |

**User's choice:** Singular: cf preset list
**Notes:** None

---

## Error Handling

### Tree export partial failures

| Option | Description | Selected |
|--------|-------------|----------|
| Skip + stderr warning | Log error as APIError JSON to stderr, skip page, continue. NDJSON uninterrupted. | ✓ |
| Fail fast | Stop entire export on first error. | |
| You decide | Claude picks | |

**User's choice:** Skip + stderr warning
**Notes:** Agents see which pages failed via stderr

---

## Template Variable Documentation

### Variable listing in show

| Option | Description | Selected |
|--------|-------------|----------|
| Add variables field | Parse body for {{.varName}} patterns, include variables array in show output. | ✓ |
| Body only, no extraction | Just output raw template JSON. Variables implicit in body. | |
| You decide | Claude decides | |

**User's choice:** Add variables field
**Notes:** Agents can discover required vars without parsing XHTML

---

## Claude's Discretion

- Exact XHTML content for each of the 6 built-in template bodies
- Internal helper functions and error message wording
- Template variable extraction implementation details
- NDJSON line field ordering
- Test case selection and organization

## Deferred Ideas

None — discussion stayed within phase scope
