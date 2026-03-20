---
phase: 10-output-presets-and-templates
verified: 2026-03-20T14:30:00Z
status: gaps_found
score: 8/9 must-haves verified
re_verification: false
gaps:
  - truth: "All project tests continue to pass (no regressions)"
    status: failed
    reason: "TestPresetEmptyStringDoesNotInterfere contaminates subsequent tests by using the package-level cobra singleton with --jq and --preset flags, leaving flag state that corrupts unrelated tests (TestRawGETWithQueryParams, TestSchemaListReturnsJSONArray, TestRunSearch_SinglePage, and 11 other pre-existing tests now fail when run together)."
    artifacts:
      - path: "cmd/preset_test.go"
        issue: "TestPresetEmptyStringDoesNotInterfere calls root.SetArgs with --jq .title on the global cobra singleton (RootCommand() returns package-level rootCmd), leaving the JQ filter set for subsequent tests that share the same command tree."
    missing:
      - "Isolate TestPresetEmptyStringDoesNotInterfere from the global cobra singleton: either use a fresh cobra tree (via a constructor that returns a new command), or reset the --jq and --preset flag changed-state after the test, or remove the --jq .title argument from this specific test (the test only needs to verify --preset '' does not interfere, not that --jq works simultaneously)."
human_verification:
  - test: "Verify cf --preset <name> with a real config file"
    expected: "Output is JQ-filtered using the named preset expression"
    why_human: "End-to-end test against actual config file and Confluence API not covered by httptest"
  - test: "Verify cf templates list with actual templates directory"
    expected: "JSON array of template filenames without .json extension, sorted"
    why_human: "OS-specific directory path (Library/Application Support on macOS) behavior"
---

# Phase 10: Output Presets and Templates Verification Report

**Phase Goal:** Users can save and reuse output formatting configurations and create content from reusable templates with variable substitution.
**Verified:** 2026-03-20T14:30:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can define named presets in profile config with JQ expressions | VERIFIED | `Profile.Presets map[string]string` field with `json:"presets,omitempty"` tag exists in `internal/config/config.go:36`; roundtrip test passes |
| 2 | User can apply --preset <name> to any command and get filtered output | VERIFIED | `--preset` flag registered in `cmd/root.go:265`; resolution in PersistentPreRunE at lines 171-192 sets `jqFilter = expr`; `TestPresetResolvesJQFilter` passes |
| 3 | Using --preset and --jq simultaneously produces an error | VERIFIED | Mutual exclusion enforced at `cmd/root.go:172-179`; `TestPresetConflictsWithJQ` passes with `validation_error` JSON |
| 4 | Using a nonexistent preset name produces a clear error | VERIFIED | Error at `cmd/root.go:183-190` lists available presets; `TestPresetNotFound` passes with `config_error` JSON |
| 5 | User can list available templates from the templates directory | VERIFIED | `cf templates list` implemented in `cmd/templates.go:27-43`; calls `cftemplate.List()`; `TestTemplatesList_WithTemplates` and `TestTemplatesList_EmptyDir` pass |
| 6 | User can create a page from a template with variable substitution | VERIFIED | `--template` and `--var` flags on pages create; `resolveTemplate` called at `cmd/pages.go:143`; `TestPagesCreate_WithTemplate` passes |
| 7 | User can create a blog post from a template with variable substitution | VERIFIED | Identical pattern in `cmd/blogposts.go:142`; `TestBlogpostsCreate_WithTemplate` passes |
| 8 | Missing template variables produce a clear error | VERIFIED | `text/template.Option("missingkey=error")` in `internal/template/template.go:84`; `TestRender_MissingVariableError` passes |
| 9 | Nonexistent template name produces a clear error | VERIFIED | `resolveTemplate` writes `config_error` JSON to stderr; `TestLoad_ErrorForNonexistent` and `TestBlogpostsCreate_WithTemplate` cover this path |

**Score:** 9/9 truths verified for phase goal

**However, a test isolation regression is present** (see Gaps section).

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | Presets map field on Profile struct | VERIFIED | Line 36: `Presets map[string]string \`json:"presets,omitempty"\`` |
| `cmd/root.go` | --preset flag registration and resolution in PersistentPreRunE | VERIFIED | Flag at line 265; resolution block at lines 171-192; `availablePresets` helper at lines 328-339 |
| `cmd/preset_test.go` | Tests for preset resolution and error cases | VERIFIED | 4 tests: resolve, not-found, conflict, empty-string — all pass when run individually; isolation bug when run in full suite |
| `internal/template/template.go` | Template loading, listing, rendering with Go text/template | VERIFIED | Exports `Dir`, `Template`, `RenderedTemplate`, `List`, `Load`, `Render`; all 7 unit tests pass |
| `cmd/templates.go` | cf templates list command and resolveTemplate helper | VERIFIED | `templatesCmd` + `templates_list` + `resolveTemplate` all present and functional |
| `cmd/pages.go` | --template and --var flags on pages create | VERIFIED | Flags at lines 308-309; template resolution at lines 133-155 via `resolveTemplate` |
| `cmd/blogposts.go` | --template and --var flags on blogposts create | VERIFIED | Flags at lines 302-303; template resolution at lines 132-154 via `resolveTemplate` |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/root.go` | `internal/config/config.go` | Reads `rawProfile.Presets[name]` to get JQ expression | WIRED | `cmd/root.go:181`: `expr, ok := rawProfile.Presets[preset]` |
| `cmd/root.go` | `client.Client.JQFilter` | Sets JQFilter to resolved preset expression | WIRED | `cmd/root.go:190`: `jqFilter = expr` (then JQFilter set on Client construction) |
| `cmd/pages.go` | `internal/template/template.go` | Load + Render via resolveTemplate | WIRED | `cmd/pages.go:143`: `resolveTemplate(templateName, varFlags)` which calls `cftemplate.Load` + `cftemplate.Render` |
| `cmd/templates.go` | `internal/template/template.go` | List to enumerate available templates | WIRED | `cmd/templates.go:31`: `cftemplate.List()` |
| `internal/template/template.go` | `text/template` | Go stdlib template execution with map[string]string data | WIRED | Line 11: `"text/template"` import; line 84: `template.New(name).Option("missingkey=error").Parse(text)` |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| PRST-01 | 10-01-PLAN.md | User can define named output presets in profile config | SATISFIED | `Profile.Presets map[string]string` field; roundtrip test passes |
| PRST-02 | 10-01-PLAN.md | User can apply a preset to any command output via `--preset <name>` | SATISFIED | `--preset` persistent flag on root; JQ resolution in PersistentPreRunE |
| TMPL-01 | 10-02-PLAN.md | User can list available content templates | SATISFIED | `cf templates list` outputs sorted JSON array from templates directory |
| TMPL-02 | 10-02-PLAN.md | User can create content from a template with variable substitution | SATISFIED | `--template` and `--var` flags on both `pages create` and `blogposts create-blog-post` |

All 4 requirement IDs from both PLAN frontmatter entries are accounted for and satisfied. No orphaned requirements found in REQUIREMENTS.md for Phase 10.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/preset_test.go` | 139 | `root.SetArgs([]string{"pages", "get", "42", "--jq", ".title", "--preset", ""})` on global cobra singleton | Blocker | Leaves `--jq .title` flag "changed" state on the global pages command tree, corrupting 14 subsequent tests when the full `./cmd/` suite runs. Tests that fail: `TestRawGETWithQueryParams`, `TestVersionSubcommandOutputsJSON`, `TestSchemaListReturnsJSONArray`, `TestSchemaNoArgsReturnsValidJSON`, `TestSchemaOutputToStdout`, `TestRunSearch_SinglePage`, `TestRunSearch_TwoPages`, `TestBatch_PartialFailure`, `TestBatch_MultiOpSuccess`, `TestBlogpostsWorkflowGetByID_InjectsBodyFormat`, `TestCommentsList_CallsCorrectPath`, `TestCommentsDelete_CallsCorrectPath`, `TestLabelsList_CallsCorrectPath`, `TestPagesWorkflowGetByID_InjectsBodyFormat` |

---

### Human Verification Required

#### 1. Preset with real config file

**Test:** Create a config with `presets: {"titles": ".results[].title"}` in profile, then run `cf pages list --preset titles`
**Expected:** Output is a JSON array of title strings, not the full page objects
**Why human:** Httptest verifies filtering occurs, but real Confluence response structure and actual JQ behavior on live data is not covered

#### 2. Template with real templates directory

**Test:** Create `~/.config/cf/templates/meeting-notes.json` with `{"title":"{{.title}}","body":"<p>{{.date}}</p>"}`, run `cf templates list`, then `cf pages create --template meeting-notes --var "title=Test" --var "date=2026-03-20" --space-id SPACE`
**Expected:** `templates list` returns `["meeting-notes"]`; page created with rendered title and body
**Why human:** OS-specific config directory path (macOS uses `Library/Application Support`) behavior only exercisable on real filesystem

---

### Gaps Summary

The phase goal is functionally achieved — all 4 requirements (PRST-01, PRST-02, TMPL-01, TMPL-02) have complete, wired implementations backed by passing unit and integration tests. The build is clean (`go build ./...` and `go vet ./...` succeed).

**One blocker gap exists:** `TestPresetEmptyStringDoesNotInterfere` in `cmd/preset_test.go` uses `--jq .title` alongside `--preset ""` on the package-level cobra singleton (`RootCommand()` returns `rootCmd`). This leaves the `--jq` flag in a "changed" state on the global command tree, causing 14 other unrelated tests to receive incorrect output or fail entirely when `go test ./cmd/ -count=1` runs the full package.

The failing tests were verified to PASS at the pre-phase-10 commit (`fdef939`) and FAIL after phase 10 is applied — confirming this is a regression introduced in this phase.

**Fix:** Remove `--jq .title` from `TestPresetEmptyStringDoesNotInterfere`. The test's stated purpose is to verify that an empty `--preset ""` does not conflict with `--jq`. This can be verified by passing `--jq .title` without `--preset ""`, or by simply verifying the command succeeds without the `--preset` flag set at all. The `--jq` flag is incidental to what the test is verifying.

---

_Verified: 2026-03-20T14:30:00Z_
_Verifier: Claude (gsd-verifier)_
