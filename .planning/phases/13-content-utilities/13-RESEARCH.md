# Phase 13: Content Utilities - Research

**Researched:** 2026-03-28
**Domain:** CLI command layer -- wiring Phase 12 internal packages into user-facing Cobra commands (preset list, template management, page export)
**Confidence:** HIGH

## Summary

Phase 13 is a pure command-layer phase. The hard work -- preset three-tier resolution, template loading/rendering, JQ filtering, NDJSON streaming -- already exists in internal packages from Phase 12 and earlier. This phase creates five groups of CLI commands: `preset list`, `templates show`, `templates create --from-page`, `export`, and `export --tree`. It also extends the template package with built-in templates (mirroring the built-in preset pattern) and refactors `templates list` to return source-attributed entries.

The codebase has strong, consistent patterns for all of these. The jr (Jira CLI) reference implementation provides near-identical `preset list` and `template show/create` commands. The watch command provides an established NDJSON streaming pattern using `jsonutil.NewEncoder`. The pages workflow commands demonstrate the `body-format` query parameter wiring for the Confluence v2 API. No new Go dependencies are needed.

**Primary recommendation:** Mirror existing patterns exactly -- jr's preset/template commands for command structure, watch.go's NDJSON encoder for tree export, and pages.go's body-format handling for single-page export. The only substantial new code is the recursive tree walker for `export --tree`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Built-in templates stored as an embedded Go `map[string]*Template` in `internal/template/template.go` source code -- same pattern as built-in presets in `internal/preset/preset.go`
- **D-02:** 6 built-in templates: blank, meeting-notes, decision, runbook, retrospective, adr -- bodies are Confluence storage format (XHTML) structural skeletons with headings, sections, and `{{.variable}}` placeholders
- **D-03:** Built-in templates merged with user templates in `templates list` output -- same merge pattern as presets (built-in lowest priority, user overrides). Each entry includes name + source tag (builtin/user)
- **D-04:** Refactor `template.List()` to return `[]templateEntry` (name, source) JSON like `preset.List()` -- built-in map checked first, user dir overlays with higher priority. Minimal struct: name + source (no body in list output)
- **D-05:** `cf templates show <name>` outputs the full Template struct as JSON (title, body, space_id). For built-in templates, serialize from embedded map. For user templates, read the file. Same format either way
- **D-06:** Show output includes a `variables` array extracted by parsing the body for `{{.varName}}` patterns -- agents can discover required vars without parsing XHTML
- **D-07:** `cf templates create --from-page <id> --name <name>` fetches the page via v2 API, saves the storage-format body as-is into a template JSON file. User manually adds `{{.variable}}` placeholders later by editing
- **D-08:** Captures title (as title template string) and body only. SpaceID left empty so template is reusable across spaces
- **D-09:** Template file saved to user templates directory (`~/.config/cf/templates/<name>.json`). Requires `--name` flag to specify template name
- **D-10:** `cf export --id <pageId>` outputs page body in requested format. `--format` flag supports all three Confluence v2 body formats: storage (default), atlas_doc_format, view. Passed as `body-format` query param
- **D-11:** `cf export --id <pageId> --tree` recursively exports page tree as NDJSON using v2 child pages API. Depth-first traversal. Each line is a JSON object with id, title, parentId, depth, and body in requested format
- **D-12:** `--depth` flag controls recursion depth (0 = unlimited, default unlimited). Lets agents control tree scope
- **D-13:** Tree export handles partial failures with skip + stderr warning -- inaccessible pages logged as APIError JSON to stderr, NDJSON stream continues. Agents see which pages failed
- **D-14:** `cf preset list` (singular parent, matches jr exactly). `presetCmd` parent with `presetListCmd` child registered to root
- **D-15:** Output from `preset.List()` flows through standard `--jq` and `--pretty` pipeline, consistent with all other commands
- **D-16:** Passes current profile's presets to `preset.List()` so output reflects all three tiers (builtin, user, profile) -- shows what `--preset` would actually resolve to

### Claude's Discretion
- Exact XHTML content for each of the 6 built-in template bodies (structural skeletons appropriate to each template's purpose)
- JQ expression for built-in presets already defined in Phase 12 -- no changes needed
- Internal helper functions, error message wording, test case selection
- Template variable extraction implementation details (regex vs template parse)
- NDJSON line field ordering and any additional metadata fields beyond id/title/parentId/depth/body

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CONT-01 | User can list all available presets (built-in + user) via `preset list` | Mirror jr `cmd/preset.go` pattern; `preset.List(profilePresets)` returns JSON bytes; wire through `--jq`/`--pretty` pipeline |
| CONT-02 | CLI ships 7 built-in presets (brief, titles, agent, tree, meta, search, diff) | Already complete in `internal/preset/preset.go` (Phase 12). No work needed -- verify via `preset list` output |
| CONT-03 | CLI ships 6 built-in templates (blank, meeting-notes, decision, runbook, retrospective, adr) | Add `builtinTemplates` map to `internal/template/template.go` mirroring `builtinPresets` pattern; XHTML skeletons with `{{.variable}}` placeholders |
| CONT-04 | User can inspect a template definition via `templates show <name>` | New `templates show` subcommand; `template.Show(name)` loads from builtin map or user dir; includes extracted variables array |
| CONT-05 | User can create a template from an existing page via `templates create --from-page` | New `templates create` subcommand; fetches page via `c.Fetch()` with `body-format=storage`; saves to user templates dir |
| CONT-06 | User can export page body in requested format via `export` command | New `cmd/export.go`; GET `/pages/{id}` with `body-format` query param; extract body field from response |
| CONT-07 | User can recursively export a page tree as NDJSON via `export --tree` | Depth-first tree walker using GET `/pages/{id}/children` endpoint; NDJSON via `jsonutil.NewEncoder`; partial failure handling per D-13 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | (existing) | CLI command framework | Already in project; all commands use it |
| `github.com/itchyny/gojq` | (existing) | JQ filtering for `--jq`/`--preset` pipeline | Already in project via `internal/jq` |
| `text/template` (stdlib) | Go stdlib | Template variable extraction via regex | Already used in `internal/template/template.go` |
| `regexp` (stdlib) | Go stdlib | Extract `{{.varName}}` patterns from template bodies | Stdlib; simpler than template.Parse for variable listing |
| `encoding/json` (stdlib) | Go stdlib | JSON marshaling, NDJSON streaming | Already used throughout project |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/preset` | (existing) | `List()` for preset list command | Preset list command |
| `internal/template` | (existing) | Template management: `List()`, `Load()`, `Render()`, `Dir()` | Template commands |
| `internal/jsonutil` | (existing) | `MarshalNoEscape()`, `NewEncoder()` | All JSON output, NDJSON streaming |
| `internal/errors` | (existing) | `APIError`, `AlreadyWrittenError`, exit codes | Error handling in all commands |
| `internal/client` | (existing) | `Fetch()`, `WriteOutput()`, `Do()` | API calls for export and template create from page |

**No new dependencies required.** All functionality builds on existing packages.

## Architecture Patterns

### Recommended Project Structure
```
cmd/
  preset.go          # NEW: presetCmd parent + presetListCmd child
  export.go          # NEW: exportCmd with --tree flag
  templates.go       # MODIFIED: add show, create subcommands; refactor list
  root.go            # MODIFIED: register presetCmd, exportCmd
internal/
  template/
    template.go      # MODIFIED: add builtinTemplates, Show(), Save(), refactor List()
    builtin.go       # NEW: built-in template definitions (keeps template.go clean)
```

### Pattern 1: Preset List Command (mirror jr)
**What:** `cf preset list` returns JSON array through `--jq`/`--pretty` pipeline
**When to use:** CONT-01
**Example:**
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/preset.go
var presetCmd = &cobra.Command{
    Use:   "preset",
    Short: "Manage output presets",
}

var presetListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all available output presets",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Load profile presets from config -- same pattern as root.go line 179
        profileName, _ := cmd.Flags().GetString("profile")
        resolved, err := config.Resolve(config.DefaultPath(), profileName, &config.FlagOverrides{})
        // ... error handling ...
        var rawProfile config.Profile
        if cfg, loadErr := config.LoadFrom(config.DefaultPath()); loadErr == nil {
            rawProfile = cfg.Profiles[resolved.ProfileName]
        }

        data, err := preset_pkg.List(rawProfile.Presets)
        // ... error handling ...

        // Apply --jq and --pretty manually (same as jr pattern)
        jqFilter, _ := cmd.Flags().GetString("jq")
        prettyFlag, _ := cmd.Flags().GetBool("pretty")
        if jqFilter != "" {
            data, err = jq.Apply(data, jqFilter)
            // ... error handling ...
        }
        if prettyFlag {
            var out bytes.Buffer
            if err := json.Indent(&out, data, "", "  "); err == nil {
                data = out.Bytes()
            }
        }
        fmt.Fprintf(os.Stdout, "%s\n", strings.TrimRight(string(data), "\n"))
        return nil
    },
}
```

**Key detail:** `preset list` must NOT go through `PersistentPreRunE` (no client needed). The `preset` command must be added to `skipClientCommands` in `root.go` OR registered in a way that skips client injection. Current approach: `templatesCmd` is already in `skipClientCommands` -- do the same for `presetCmd`.

### Pattern 2: Built-in Templates Map (mirror built-in presets)
**What:** Embedded Go map of Template structs
**When to use:** CONT-03
**Example:**
```go
// Source: internal/preset/preset.go (adapted for templates)
var builtinTemplates = map[string]*Template{
    "blank": {
        Title: "{{.title}}",
        Body:  "",
    },
    "meeting-notes": {
        Title: "{{.title}}",
        Body:  `<h2>Attendees</h2><p>{{.attendees}}</p><h2>Agenda</h2><p>{{.agenda}}</p><h2>Notes</h2><p></p><h2>Action Items</h2><p></p>`,
    },
    // ... etc
}
```

### Pattern 3: Template Variable Extraction (regex)
**What:** Parse `{{.varName}}` patterns from template body to discover required variables
**When to use:** CONT-04 (templates show output)
**Example:**
```go
import "regexp"

var varPattern = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

func extractVariables(tmpl *Template) []string {
    seen := make(map[string]bool)
    var vars []string
    for _, matches := range varPattern.FindAllStringSubmatch(tmpl.Title+tmpl.Body+tmpl.SpaceID, -1) {
        name := matches[1]
        if !seen[name] {
            seen[name] = true
            vars = append(vars, name)
        }
    }
    return vars
}
```

**Why regex over template.Parse:** The `text/template` parser does not expose its AST variable names directly. Regex is simpler, deterministic, and sufficient for the `{{.varName}}` pattern. Phase 12 decisions explicitly left this to Claude's discretion.

### Pattern 4: NDJSON Tree Export (mirror watch.go)
**What:** Depth-first recursive traversal writing one JSON line per page
**When to use:** CONT-07
**Example:**
```go
// Source: cmd/watch.go line 84 (NDJSON encoder pattern)
enc := jsonutil.NewEncoder(c.Stdout)

type exportEntry struct {
    ID       string `json:"id"`
    Title    string `json:"title"`
    ParentID string `json:"parentId"`
    Depth    int    `json:"depth"`
    Body     any    `json:"body"`
}

func walkTree(ctx context.Context, c *client.Client, pageID, parentID string,
    depth, maxDepth int, format string, enc *json.Encoder) {
    // 1. Fetch page with body-format
    body, code := c.Fetch(ctx, "GET",
        fmt.Sprintf("/pages/%s?body-format=%s", url.PathEscape(pageID), url.QueryEscape(format)), nil)
    if code != cferrors.ExitOK {
        // Partial failure: log to stderr, continue
        return
    }
    // 2. Extract fields, emit NDJSON line
    _ = enc.Encode(entry)
    // 3. Fetch children via /pages/{id}/children
    // 4. Recurse for each child (depth-first)
}
```

### Pattern 5: Single-Page Export (extract body from page response)
**What:** Fetch page with body-format, extract only the body field
**When to use:** CONT-06
**Example:**
```go
// v2 response when body-format=storage:
// { "id": "123", "body": { "storage": { "representation": "storage", "value": "<p>...</p>" } }, ... }
// Extract .body.{format} from response
var page struct {
    Body map[string]json.RawMessage `json:"body"`
}
_ = json.Unmarshal(respBody, &page)
bodyContent := page.Body[format] // "storage", "view", or "atlas_doc_format"
```

**Critical detail:** The `--format` flag maps to `body-format` query parameter. The response nests the body under `.body.{format_name}`. Valid values from the OpenAPI spec's `BodySingle` schema: `storage`, `atlas_doc_format`, `view`.

### Pattern 6: Template Create from Page (mirror jr template create --from)
**What:** Fetch page, extract title+body, save as template JSON file
**When to use:** CONT-05
**Example:**
```go
// Source: jr cmd/template.go runTemplateCreate (adapted for cf)
// 1. Fetch page: c.Fetch(ctx, "GET", "/pages/{id}?body-format=storage", nil)
// 2. Parse response for title + body.storage.value
// 3. Build Template{Title: page.Title, Body: storageValue, SpaceID: ""}
// 4. Marshal to JSON and write to template.Dir()/{name}.json
```

### Anti-Patterns to Avoid
- **Do NOT use `c.Do()` for export:** `c.Do()` writes full response to stdout. Export needs to extract the body field only. Use `c.Fetch()` which returns raw bytes.
- **Do NOT build a custom pagination loop for children:** The `c.Fetch()` method does not paginate automatically. For tree export, you must handle cursor pagination manually for the children endpoint (check `_links.next`).
- **Do NOT add `preset` to `skipClientCommands` map literally:** The `preset` command parent is `presetCmd`, but its child `list` does need profile resolution (not API client). Handle by resolving config directly in the command RunE, not via PersistentPreRunE. Same pattern as jr's preset list.
- **Do NOT put built-in template XHTML bodies inline in template.go:** Create a separate `builtin.go` file in `internal/template/` to keep template.go readable.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JQ filtering on preset list | Custom output filtering | `jq.Apply()` + `--jq`/`--pretty` flags | Already works; jr does this exactly |
| JSON serialization without HTML escaping | `encoding/json` directly | `jsonutil.MarshalNoEscape()` / `jsonutil.NewEncoder()` | Template bodies contain XHTML with `<`, `>`, `&` |
| Template variable discovery | Template AST walking | `regexp.MustCompile(\{\{\s*\.(\w+)\s*\}\})` | Simpler, covers all cases in our format |
| NDJSON line encoding | Manual string concatenation | `jsonutil.NewEncoder(w).Encode(entry)` | Handles escaping, newlines; established pattern |
| Page tree cursor pagination | Full pagination library | Manual next-link following in walkTree | Only needed in tree walker; simple loop |

**Key insight:** Every internal utility this phase needs already exists. The only new algorithm is the depth-first tree walker, and even that is a straightforward recursive function using existing `c.Fetch()`.

## Common Pitfalls

### Pitfall 1: PersistentPreRunE Runs for Preset List
**What goes wrong:** `preset list` triggers client injection in PersistentPreRunE, which requires base_url/token, but `preset list` is a local config-only operation.
**Why it happens:** PersistentPreRunE runs for ALL commands unless skipped.
**How to avoid:** Add `"preset"` to the `skipClientCommands` map in `cmd/root.go`. This is the same pattern used for `"templates"`, `"configure"`, `"schema"`, etc.
**Warning signs:** `preset list` fails with "base_url is not set" when no profile is configured.

### Pitfall 2: HTML Escaping in Template Bodies
**What goes wrong:** XHTML template bodies have `<h2>`, `<p>`, `&` converted to `\u003ch2\u003e`, `\u003cp\u003e`, `\u0026` in JSON output.
**Why it happens:** Go's `encoding/json` escapes HTML by default.
**How to avoid:** Use `jsonutil.MarshalNoEscape()` or `jsonutil.NewEncoder()` for ALL JSON output containing template bodies. This is already the project convention.
**Warning signs:** Template show output has `\u003c` instead of `<`.

### Pitfall 3: Children Endpoint Returns No Body
**What goes wrong:** Tree export calls `/pages/{id}/children` expecting body content, gets only id/title/spaceId/childPosition.
**Why it happens:** The v2 children endpoint returns `ChildPage` objects (id, status, title, spaceId, childPosition) -- no body field. The `body-format` query param is NOT supported on the children endpoint.
**How to avoid:** Use children endpoint only to discover child IDs. Then fetch each child individually via `GET /pages/{id}?body-format={format}` to get the body. This is the correct two-step approach.
**Warning signs:** Body field is null/empty in NDJSON output.

### Pitfall 4: Template List Refactoring Breaks Existing Tests
**What goes wrong:** Current `templates list` returns `[]string` (just names). Refactoring to `[]templateEntry` (name+source) changes the JSON output format, breaking `cmd/templates_test.go`.
**Why it happens:** Tests assert `json.Unmarshal(buf.Bytes(), &names)` where `names` is `[]string`.
**How to avoid:** Update tests to expect `[]templateEntry` format. Also update the `resolveTemplate` helper in `cmd/templates.go` which calls `template.Load()` -- it needs to check built-in templates too.
**Warning signs:** `TestTemplatesList_WithTemplates` fails with JSON unmarshal error.

### Pitfall 5: Export --tree Pagination for Children
**What goes wrong:** Pages with many children (>25) only export the first page of children, missing the rest.
**Why it happens:** Confluence v2 children endpoint paginates at 25 results by default. `c.Fetch()` does NOT auto-paginate (only `c.Do()` does).
**How to avoid:** In the tree walker, after fetching children, check for `_links.next` in the response and follow it. Parse the cursor-paginated envelope manually (same structure as `cursorPage` in `client.go`).
**Warning signs:** Large page trees are truncated to 25 children per level.

### Pitfall 6: Preset List Needs Profile Resolution Without Full Client
**What goes wrong:** `preset list` needs profile presets (from config) but `PersistentPreRunE` is skipped. How does it get profile presets?
**Why it happens:** The profile resolution logic is in PersistentPreRunE, which is skipped for the preset command.
**How to avoid:** Resolve config directly in the preset list RunE. Call `config.Resolve()` and `config.LoadFrom()` inline, just like jr does. Only need the presets map, not the full client.
**Warning signs:** Profile presets not appearing in `preset list` output.

## Code Examples

### Preset List Command (verified pattern from jr)
```go
// Source: /Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/preset.go
// jr passes no profile presets -- cf needs to resolve them from config.
// Key difference: cf has three-tier (profile > user > builtin), jr has two-tier.
var presetListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all available output presets",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Resolve profile to get profile-level presets.
        profileName, _ := cmd.Flags().GetString("profile")
        resolved, err := config.Resolve(config.DefaultPath(), profileName, &config.FlagOverrides{})
        if err != nil {
            // Non-fatal: list built-in presets only if config fails.
            resolved = &config.ResolvedConfig{}
        }
        var rawProfile config.Profile
        if cfg, loadErr := config.LoadFrom(config.DefaultPath()); loadErr == nil {
            rawProfile = cfg.Profiles[resolved.ProfileName]
        }

        data, err := preset_pkg.List(rawProfile.Presets)
        if err != nil {
            apiErr := &cferrors.APIError{ErrorType: "config_error", Message: "failed to list presets: " + err.Error()}
            apiErr.WriteJSON(os.Stderr)
            return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
        }

        jqFilter, _ := cmd.Flags().GetString("jq")
        prettyFlag, _ := cmd.Flags().GetBool("pretty")
        if jqFilter != "" {
            filtered, err := jq.Apply(data, jqFilter)
            if err != nil {
                apiErr := &cferrors.APIError{ErrorType: "jq_error", Message: "jq: " + err.Error()}
                apiErr.WriteJSON(os.Stderr)
                return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
            }
            data = filtered
        }
        if prettyFlag {
            var out bytes.Buffer
            if jsonErr := json.Indent(&out, data, "", "  "); jsonErr == nil {
                data = out.Bytes()
            }
        }
        fmt.Fprintf(os.Stdout, "%s\n", strings.TrimRight(string(data), "\n"))
        return nil
    },
}
```

### Template Show Output Structure
```json
{
  "name": "meeting-notes",
  "title": "{{.title}}",
  "body": "<h2>Attendees</h2><p>{{.attendees}}</p>...",
  "space_id": "",
  "source": "builtin",
  "variables": ["title", "attendees", "agenda"]
}
```

### Built-in Template Example (meeting-notes)
```go
// internal/template/builtin.go
"meeting-notes": {
    Title: "{{.title}}",
    Body:  `<h2>Attendees</h2><p>{{.attendees}}</p><h2>Agenda</h2><p>{{.agenda}}</p><h2>Notes</h2><p></p><h2>Action Items</h2><ul><li></li></ul>`,
},
```

### Export NDJSON Line Format
```json
{"id":"123","title":"Parent Page","parentId":"","depth":0,"body":{"storage":{"representation":"storage","value":"<p>Content</p>"}}}
{"id":"456","title":"Child Page","parentId":"123","depth":1,"body":{"storage":{"representation":"storage","value":"<p>Child content</p>"}}}
```

### Tree Walker Children Pagination
```go
// Fetch all children of a page, handling cursor pagination.
func fetchAllChildren(ctx context.Context, c *client.Client, pageID string) ([]childInfo, error) {
    var all []childInfo
    path := fmt.Sprintf("/pages/%s/children?limit=25", url.PathEscape(pageID))
    for path != "" {
        body, code := c.Fetch(ctx, "GET", path, nil)
        if code != cferrors.ExitOK {
            return all, fmt.Errorf("fetch children failed")
        }
        var page struct {
            Results []childInfo           `json:"results"`
            Links   struct{ Next string } `json:"_links"`
        }
        _ = json.Unmarshal(body, &page)
        all = append(all, page.Results...)
        path = page.Links.Next
        // Strip domain prefix if present
    }
    return all, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `templates list` returns `[]string` | Returns `[]templateEntry` with name+source | Phase 13 | Existing tests need updating |
| Templates only from user files | Built-in + user merge (three-tier for presets) | Phase 13 | New users get templates out of the box |
| No body export command | `cf export` with format selection | Phase 13 | Agents can extract page content directly |

## Open Questions

1. **Template Save function**
   - What we know: jr has `tmpl.Save(t, overwrite)` that handles file writing and conflict detection.
   - What's unclear: cf's `internal/template` does not yet have a `Save()` function.
   - Recommendation: Add `Save(name string, tmpl *Template) error` to `internal/template/template.go`. Write JSON to `Dir()/{name}.json`. Return error if file exists (no `--overwrite` flag in cf's decision set).

2. **Export command registration and client requirement**
   - What we know: Export needs an API client (it fetches pages). Preset list does NOT need a client.
   - What's unclear: Whether `export` should be in `skipClientCommands`.
   - Recommendation: `export` should NOT be in `skipClientCommands` -- it requires authenticated API calls. Register as `rootCmd.AddCommand(exportCmd)`. Use `client.FromContext()` in RunE.

3. **Tree export body field: raw JSON vs extracted value**
   - What we know: D-11 says "body in requested format". The page response nests body as `{"storage": {"representation": "storage", "value": "<p>...</p>"}}`.
   - What's unclear: Should NDJSON include the full body object `{"storage": {...}}` or just the value string?
   - Recommendation: Include the full body object as-is from the API response (preserves format metadata). This matches how `pages get-by-id` returns it.

## Sources

### Primary (HIGH confidence)
- `internal/preset/preset.go` -- Complete preset package with `List()`, `Lookup()`, `builtinPresets` map, three-tier resolution
- `internal/template/template.go` -- Template package with `List()`, `Load()`, `Render()`, `Dir()`
- `cmd/templates.go` -- Current templates command with `templates list` and `resolveTemplate()`
- `cmd/root.go` -- Root command with `skipClientCommands`, `--preset`/`--jq` flag wiring, profile resolution
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/preset.go` -- jr preset list command (reference implementation)
- `/Users/quan.hoang/quanhh/quanhoang/jira-cli-v2/cmd/template.go` -- jr template show/create commands (reference implementation)
- `cmd/watch.go` -- NDJSON streaming pattern with `jsonutil.NewEncoder()`
- `cmd/pages.go` -- Pages workflow commands with `body-format` handling, `c.Fetch()` usage
- `spec/confluence-v2.json` -- OpenAPI spec confirming: ChildPage schema has no body field; BodySingle has storage/atlas_doc_format/view; children endpoint at `/pages/{id}/children`

### Secondary (MEDIUM confidence)
- `cmd/generated/pages.go` -- Generated children endpoint confirms `get-child` uses path `/pages/{id}/children` with cursor/limit/sort params
- `internal/client/client.go` -- `Fetch()` returns raw bytes (no auto-pagination), `Do()` auto-paginates, `WriteOutput()` applies JQ + pretty

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- All packages already exist in the project; zero new dependencies
- Architecture: HIGH -- jr reference implementation provides exact patterns; watch.go provides NDJSON pattern; all integration points verified in source code
- Pitfalls: HIGH -- Verified children endpoint has no body via OpenAPI spec; HTML escaping issue is a known project pattern; PersistentPreRunE skip pattern confirmed in root.go

**Research date:** 2026-03-28
**Valid until:** 2026-04-28 (stable -- internal project patterns, no external dependency changes)
