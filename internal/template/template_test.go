package template

import (
	"os"
	"path/filepath"
	"sort"
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

	names, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("List() not sorted: %v", names)
	}
	want := []string{"alpha", "meeting-notes", "zebra"}
	if len(names) != len(want) {
		t.Fatalf("List() got %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("List()[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestList_EmptySliceForNonexistentDir(t *testing.T) {
	// Point to a config path in a dir with no templates subdir.
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	names, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if names == nil {
		t.Fatal("List() returned nil, want empty slice")
	}
	if len(names) != 0 {
		t.Errorf("List() got %v, want empty slice", names)
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

func TestLoad_ErrorForNonexistent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CF_CONFIG_PATH", filepath.Join(dir, "config.json"))

	_, err := Load("does-not-exist")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent template")
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
