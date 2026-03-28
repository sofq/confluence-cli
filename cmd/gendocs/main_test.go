package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGeneratesExpectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := run(tmpDir); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	// Check key files exist.
	required := []string{
		filepath.Join(tmpDir, "commands", "index.md"),
		filepath.Join(tmpDir, ".vitepress", "sidebar-commands.json"),
		filepath.Join(tmpDir, "guide", "error-codes.md"),
	}
	for _, f := range required {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Check at least 10 .md files in commands/.
	entries, err := os.ReadDir(filepath.Join(tmpDir, "commands"))
	if err != nil {
		t.Fatalf("cannot read commands dir: %v", err)
	}
	mdCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdCount++
		}
	}
	if mdCount < 10 {
		t.Errorf("expected at least 10 .md files in commands/, got %d", mdCount)
	}
}

func TestSidebarJSONIsValid(t *testing.T) {
	tmpDir := t.TempDir()
	if err := run(tmpDir); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".vitepress", "sidebar-commands.json"))
	if err != nil {
		t.Fatalf("cannot read sidebar JSON: %v", err)
	}

	var entries []sidebarEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("invalid sidebar JSON: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("sidebar JSON has zero entries")
	}

	// Check sorted alphabetically.
	for i := 0; i < len(entries)-1; i++ {
		if entries[i].Text > entries[i+1].Text {
			t.Errorf("sidebar not sorted: %q > %q at index %d", entries[i].Text, entries[i+1].Text, i)
		}
	}

	// Check each entry has valid fields.
	for _, e := range entries {
		if e.Text == "" {
			t.Error("sidebar entry has empty Text")
		}
		if !strings.HasPrefix(e.Link, "/commands/") {
			t.Errorf("sidebar entry %q link %q does not start with /commands/", e.Text, e.Link)
		}
	}
}

func TestCommandPagesContainHandWrittenCommands(t *testing.T) {
	tmpDir := t.TempDir()
	if err := run(tmpDir); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	// Check hand-written command pages exist.
	handWritten := []string{"diff.md", "workflow.md", "export.md", "preset.md", "templates.md"}
	for _, name := range handWritten {
		path := filepath.Join(tmpDir, "commands", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected hand-written command page %s to exist", path)
		}
	}

	// Check workflow.md contains all 6 subcommand headings.
	workflowData, err := os.ReadFile(filepath.Join(tmpDir, "commands", "workflow.md"))
	if err != nil {
		t.Fatalf("cannot read workflow.md: %v", err)
	}
	content := string(workflowData)
	subcommands := []string{"## move", "## copy", "## publish", "## comment", "## restrict", "## archive"}
	for _, heading := range subcommands {
		if !strings.Contains(content, heading) {
			t.Errorf("workflow.md missing heading %q", heading)
		}
	}
}

func TestErrorCodesPageContainsAllCodes(t *testing.T) {
	tmpDir := t.TempDir()
	if err := run(tmpDir); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "guide", "error-codes.md"))
	if err != nil {
		t.Fatalf("cannot read error-codes.md: %v", err)
	}
	content := string(data)

	// Check heading.
	if !strings.Contains(content, "Error Codes") {
		t.Error("error-codes.md missing 'Error Codes' heading")
	}

	// Check all 8 exit code names present.
	names := []string{"OK", "Error", "Auth", "NotFound", "Validation", "RateLimit", "Conflict", "Server"}
	for _, name := range names {
		if !strings.Contains(content, name) {
			t.Errorf("error-codes.md missing exit code name %q", name)
		}
	}

	// Check Exit Codes section.
	if !strings.Contains(content, "Exit Codes") {
		t.Error("error-codes.md missing 'Exit Codes' section")
	}

	// Pitfall 6: must NOT contain ExitTimeout.
	if strings.Contains(content, "ExitTimeout") {
		t.Error("error-codes.md should NOT contain ExitTimeout (cf does not have it)")
	}
}

func TestBuildSchemaLookupIncludesHandWritten(t *testing.T) {
	lookup := buildSchemaLookup()

	checks := []struct {
		resource, verb string
	}{
		{"diff", "diff"},
		{"workflow", "move"},
		{"templates", "show"},
	}

	for _, c := range checks {
		key := schemaKey{c.resource, c.verb}
		if _, ok := lookup[key]; !ok {
			t.Errorf("buildSchemaLookup() missing key {%q, %q}", c.resource, c.verb)
		}
	}
}

func TestStalePageCleanup(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a stale page that should be cleaned up.
	commandsDir := filepath.Join(tmpDir, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("cannot create commands dir: %v", err)
	}
	stalePath := filepath.Join(commandsDir, "stale-old-resource.md")
	if err := os.WriteFile(stalePath, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("cannot create stale file: %v", err)
	}

	// Run generation.
	if err := run(tmpDir); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	// Stale file should be removed.
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("stale-old-resource.md should have been removed but still exists")
	}

	// A real generated file should exist.
	diffPath := filepath.Join(commandsDir, "diff.md")
	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		t.Error("diff.md should exist after generation")
	}
}
