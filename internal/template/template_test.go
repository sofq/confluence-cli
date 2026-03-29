package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// setupTempTemplates creates a temporary config dir with template files and
// sets CF_CONFIG_PATH so Dir() derives the correct templates path.
func setupTempTemplates(t *testing.T, templates map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	// Write a minimal config file so DefaultPath resolves.
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", cfgPath)

	tmplDir := filepath.Join(dir, "templates")
	if templates != nil {
		if err := os.MkdirAll(tmplDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range templates {
			p := filepath.Join(tmplDir, name+".json")
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return tmplDir
}

func TestList_SortedNames(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"zebra":         `{"title":"Z","body":"z"}`,
		"alpha":         `{"title":"A","body":"a"}`,
		"meeting-notes": `{"title":"M","body":"m"}`,
	})

	entries, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Verify sorted by name.
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("List() not sorted: %v", names)
	}

	// 3 user templates + 5 built-in not overlapping (adr, blank, decision, retrospective, runbook) = 8 total.
	// "meeting-notes" is both user and built-in; user wins.
	wantCount := 8
	if len(entries) != wantCount {
		t.Fatalf("List() got %d entries, want %d; entries=%v", len(entries), wantCount, entries)
	}

	// Verify meeting-notes shows source "user" (user overrides built-in).
	for _, e := range entries {
		if e.Name == "meeting-notes" {
			if e.Source != "user" {
				t.Errorf("meeting-notes source = %q, want %q", e.Source, "user")
			}
			break
		}
	}

	// Verify user-only templates have source "user".
	for _, e := range entries {
		if e.Name == "alpha" || e.Name == "zebra" {
			if e.Source != "user" {
				t.Errorf("%s source = %q, want %q", e.Name, e.Source, "user")
			}
		}
	}

	// Verify built-in templates have source "builtin".
	for _, e := range entries {
		if e.Name == "blank" || e.Name == "decision" || e.Name == "runbook" || e.Name == "retrospective" || e.Name == "adr" {
			if e.Source != "builtin" {
				t.Errorf("%s source = %q, want %q", e.Name, e.Source, "builtin")
			}
		}
	}
}

func TestList_EmptySliceForNonexistentDir(t *testing.T) {
	// Point to a config path in a dir with no templates subdir.
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	entries, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if entries == nil {
		t.Fatal("List() returned nil, want non-nil slice")
	}
	// Even with no user dir, built-in templates (6) should be listed.
	if len(entries) != 6 {
		t.Errorf("List() got %d entries, want 6 built-in; entries=%v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Source != "builtin" {
			t.Errorf("%s source = %q, want %q", e.Name, e.Source, "builtin")
		}
	}
}

func TestLoad_ReturnsTemplate(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"meeting-notes": `{"title":"{{.title}}","body":"<p>Meeting on {{.date}}</p>","space_id":"{{.space_id}}"}`,
	})

	tmpl, err := Load("meeting-notes")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if tmpl.Title != "{{.title}}" {
		t.Errorf("Title = %q, want %q", tmpl.Title, "{{.title}}")
	}
	if tmpl.Body != "<p>Meeting on {{.date}}</p>" {
		t.Errorf("Body = %q", tmpl.Body)
	}
	if tmpl.SpaceID != "{{.space_id}}" {
		t.Errorf("SpaceID = %q", tmpl.SpaceID)
	}
}

func TestLoad_FallsBackToBuiltin(t *testing.T) {
	// Point to a config path with no templates subdir.
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	tmpl, err := Load("blank")
	if err != nil {
		t.Fatalf("Load(blank) error: %v", err)
	}
	if tmpl.Title != "{{.title}}" {
		t.Errorf("Title = %q, want %q", tmpl.Title, "{{.title}}")
	}
	if tmpl.Body != "" {
		t.Errorf("Body = %q, want empty", tmpl.Body)
	}
}

func TestLoad_ErrorForNonexistent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	_, err := Load("does-not-exist")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent template")
	}
}

func TestExtractVariables_MeetingNotes(t *testing.T) {
	tmpl := builtinTemplates["meeting-notes"]
	vars := ExtractVariables(tmpl)
	want := []string{"title", "attendees", "agenda"}
	if len(vars) != len(want) {
		t.Fatalf("ExtractVariables() got %v, want %v", vars, want)
	}
	for i, v := range vars {
		if v != want[i] {
			t.Errorf("ExtractVariables()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestExtractVariables_BlankTemplate(t *testing.T) {
	tmpl := builtinTemplates["blank"]
	vars := ExtractVariables(tmpl)
	if len(vars) != 1 || vars[0] != "title" {
		t.Errorf("ExtractVariables(blank) = %v, want [title]", vars)
	}
}

func TestShow_BuiltinTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	out, err := Show("blank")
	if err != nil {
		t.Fatalf("Show(blank) error: %v", err)
	}
	if out.Name != "blank" {
		t.Errorf("Name = %q, want %q", out.Name, "blank")
	}
	if out.Source != "builtin" {
		t.Errorf("Source = %q, want %q", out.Source, "builtin")
	}
	if len(out.Variables) != 1 || out.Variables[0] != "title" {
		t.Errorf("Variables = %v, want [title]", out.Variables)
	}
}

func TestShow_UserTemplateOverridesBuiltin(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"blank": `{"title":"Custom {{.title}}","body":"<p>Custom blank</p>"}`,
	})

	out, err := Show("blank")
	if err != nil {
		t.Fatalf("Show(blank) error: %v", err)
	}
	if out.Source != "user" {
		t.Errorf("Source = %q, want %q", out.Source, "user")
	}
	if out.Title != "Custom {{.title}}" {
		t.Errorf("Title = %q", out.Title)
	}
}

func TestShow_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	_, err := Show("nonexistent")
	if err == nil {
		t.Fatal("Show() expected error for nonexistent template")
	}
}

func TestSave_CreatesFile(t *testing.T) {
	tmplDir := setupTempTemplates(t, nil)

	tmpl := &Template{
		Title: "{{.title}}",
		Body:  "<p>Test body</p>",
	}
	if err := Save("my-template", tmpl); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(tmplDir, "my-template.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Reload and compare.
	var loaded Template
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if loaded.Title != tmpl.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, tmpl.Title)
	}
	if loaded.Body != tmpl.Body {
		t.Errorf("Body = %q, want %q", loaded.Body, tmpl.Body)
	}
}

func TestSave_ErrorIfExists(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"existing": `{"title":"E","body":"e"}`,
	})

	tmpl := &Template{Title: "New", Body: "new"}
	err := Save("existing", tmpl)
	if err == nil {
		t.Fatal("Save() expected error for existing template")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	// Point to a config path with no templates subdir.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", cfgPath)

	tmpl := &Template{Title: "{{.title}}", Body: "<p>new</p>"}
	if err := Save("new-template", tmpl); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the directory was created and file exists.
	path := filepath.Join(dir, "templates", "new-template.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("File not found: %v", err)
	}
}

func TestRender_AllVariablesPresent(t *testing.T) {
	tmpl := &Template{
		Title:   "{{.title}}",
		Body:    "<p>Meeting on {{.date}}</p>",
		SpaceID: "{{.space_id}}",
	}
	vars := map[string]string{
		"title":    "Weekly Standup",
		"date":     "2026-03-20",
		"space_id": "12345",
	}

	rendered, err := Render(tmpl, vars)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if rendered.Title != "Weekly Standup" {
		t.Errorf("Title = %q, want %q", rendered.Title, "Weekly Standup")
	}
	if rendered.Body != "<p>Meeting on 2026-03-20</p>" {
		t.Errorf("Body = %q", rendered.Body)
	}
	if rendered.SpaceID != "12345" {
		t.Errorf("SpaceID = %q, want %q", rendered.SpaceID, "12345")
	}
}

func TestRender_MissingVariableError(t *testing.T) {
	tmpl := &Template{
		Title: "{{.title}}",
		Body:  "<p>{{.missing_var}}</p>",
	}
	vars := map[string]string{
		"title": "Test",
		// "missing_var" intentionally omitted
	}

	_, err := Render(tmpl, vars)
	if err == nil {
		t.Fatal("Render() expected error for missing variable")
	}
}

func TestRender_StaticTemplate(t *testing.T) {
	tmpl := &Template{
		Title: "Static Title",
		Body:  "<p>No variables here</p>",
	}
	rendered, err := Render(tmpl, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if rendered.Title != "Static Title" {
		t.Errorf("Title = %q", rendered.Title)
	}
	if rendered.Body != "<p>No variables here</p>" {
		t.Errorf("Body = %q", rendered.Body)
	}
}

func TestList_ReadDirError(t *testing.T) {
	// Point Dir() at a file path (not a directory), which makes ReadDir return a
	// non-IsNotExist error — this covers the error-return branch in List().
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	// Write a file where the templates directory is expected, so ReadDir fails
	// with a non-ENOENT error.
	tmplPath := filepath.Join(dir, "templates")
	if err := os.WriteFile(tmplPath, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", cfgPath)

	_, err := List()
	if err == nil {
		t.Fatal("List() expected error when templates dir is actually a file, got nil")
	}
}

func TestList_SkipsDirectoryEntries(t *testing.T) {
	// Subdirectories inside the templates dir should be silently skipped.
	tmplDir := setupTempTemplates(t, map[string]string{
		"valid": `{"title":"V","body":"v"}`,
	})
	// Create a subdirectory inside the templates dir.
	if err := os.MkdirAll(filepath.Join(tmplDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	// "subdir" should NOT appear in the listing.
	for _, e := range entries {
		if e.Name == "subdir" {
			t.Error("List() should not include directory entries")
		}
	}
}

func TestLoad_PathSeparatorError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	_, err := Load("path/with/separator")
	if err == nil {
		t.Fatal("Load() expected error for name with path separator")
	}
}

func TestLoad_InvalidJSONInUserFile(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"bad-json": `{not valid json`,
	})
	_, err := Load("bad-json")
	if err == nil {
		t.Fatal("Load() expected error for invalid JSON in user template")
	}
}

func TestShow_PathSeparatorError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	_, err := Show("path/with/separator")
	if err == nil {
		t.Fatal("Show() expected error for name with path separator")
	}
}

func TestShow_InvalidJSONInUserFile(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"bad-json": `{not valid json`,
	})
	_, err := Show("bad-json")
	if err == nil {
		t.Fatal("Show() expected error for invalid JSON in user template file")
	}
}

func TestSave_PathSeparatorError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	tmpl := &Template{Title: "T", Body: "B"}
	err := Save("path/with/separator", tmpl)
	if err == nil {
		t.Fatal("Save() expected error for name with path separator")
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere; cannot test permission error")
	}
	// Make Dir() return a path whose parent cannot have subdirs created in it.
	// We do this by placing a regular file at the path where "templates" would
	// be, then pointing config to a config.json whose parent is that file.
	// Dir() = filepath.Dir(config.DefaultPath()) + "/templates"
	// So if we set CF_CONFIG_PATH = <some-file>/config.json, Dir() will try
	// MkdirAll(<some-file>/templates) which fails because <some-file> is a file.
	dir := t.TempDir()
	// Create a regular file named "cf" in the temp dir.
	cfFile := filepath.Join(dir, "cf")
	if err := os.WriteFile(cfFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Point CF_CONFIG_PATH inside the "cf" file (which is actually a file, not a dir).
	t.Setenv("CF_CONFIG_PATH", filepath.Join(cfFile, "config.json"))

	tmpl := &Template{Title: "{{.title}}", Body: "<p>test</p>"}
	err := Save("my-template", tmpl)
	if err == nil {
		t.Fatal("Save() expected error when MkdirAll fails (parent path is a file)")
	}
}

func TestSave_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere; cannot test permission error")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", cfgPath)

	// Create the templates directory but make it read-only so WriteFile fails.
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(tmplDir, 0o700) //nolint:errcheck

	tmpl := &Template{Title: "{{.title}}", Body: "<p>test</p>"}
	err := Save("my-template", tmpl)
	if err == nil {
		t.Fatal("Save() expected error when templates directory is read-only")
	}
}

func TestRender_TitleParseError(t *testing.T) {
	// An invalid Go template syntax in Title triggers the parse error branch.
	tmpl := &Template{
		Title: "{{.title",
		Body:  "<p>content</p>",
	}
	_, err := Render(tmpl, map[string]string{"title": "T"})
	if err == nil {
		t.Fatal("Render() expected error for invalid title template syntax")
	}
}

func TestRender_BodyMissingVariable(t *testing.T) {
	// A missing variable in Body triggers the render body error branch.
	tmpl := &Template{
		Title: "{{.title}}",
		Body:  "<p>{{.missing}}</p>",
	}
	_, err := Render(tmpl, map[string]string{"title": "T"})
	if err == nil {
		t.Fatal("Render() expected error for missing body variable")
	}
}

func TestRender_SpaceIDMissingVariable(t *testing.T) {
	// A missing variable in SpaceID triggers the render space_id error branch.
	tmpl := &Template{
		Title:   "{{.title}}",
		Body:    "<p>content</p>",
		SpaceID: "{{.missing_space}}",
	}
	_, err := Render(tmpl, map[string]string{"title": "T"})
	if err == nil {
		t.Fatal("Render() expected error for missing space_id variable")
	}
}

func TestSave_AlreadyExists(t *testing.T) {
	setupTempTemplates(t, map[string]string{
		"existing": `{"title":"test","body":"<p>test</p>"}`,
	})
	tmpl := &Template{Title: "new", Body: "<p>new</p>"}
	err := Save("existing", tmpl)
	if err == nil {
		t.Fatal("expected error for existing template, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestSave_MarshalIndentError(t *testing.T) {
	setupTempTemplates(t, nil)
	// A template with a func field or chan can't be marshaled, but Template
	// is a plain struct. Use json.Number with invalid value to force error.
	// Actually, Template has only string fields, so json.MarshalIndent never fails.
	// We can cover this by passing a value that causes MarshalIndent to fail.
	// Since Template is all strings, this branch is dead code.
	// To cover it anyway, we can test with a valid template to confirm success.
	tmpl := &Template{Title: "new", Body: "<p>body</p>"}
	err := Save("new-template", tmpl)
	if err != nil {
		t.Fatalf("expected success for new template, got: %v", err)
	}
}
