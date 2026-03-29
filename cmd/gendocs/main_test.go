package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/sofq/confluence-cli/cmd/generated"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// ---- extractFlags coverage ----

// TestExtractFlagsSkipsHelp verifies that the --help flag injected by cobra
// is silently skipped and never appears in the returned slice.
func TestExtractFlagsSkipsHelp(t *testing.T) {
	c := &cobra.Command{Use: "test", Short: "test cmd"}
	// cobra automatically adds a --help local flag; initialise flags so it exists.
	c.InitDefaultHelpFlag()

	flags := extractFlags(c)
	for _, f := range flags {
		if f.Name == "help" {
			t.Error("extractFlags should not include the 'help' flag")
		}
	}
}

// TestExtractFlagsRequiredAnnotation verifies that a flag marked as required
// via cobra.BashCompOneRequiredFlag annotation is reflected in the returned
// flagInfo.Required field.
func TestExtractFlagsRequiredAnnotation(t *testing.T) {
	c := &cobra.Command{Use: "test", Short: "test cmd"}
	c.Flags().String("my-flag", "", "a required flag")
	if err := c.MarkFlagRequired("my-flag"); err != nil {
		t.Fatalf("MarkFlagRequired: %v", err)
	}

	flags := extractFlags(c)
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	if !flags[0].Required {
		t.Error("expected Required=true for flag marked as required")
	}
}

// TestExtractFlagsIgnoresNonRequiredAnnotation verifies that a flag whose
// BashCompOneRequiredFlag annotation is absent has Required=false.
func TestExtractFlagsOptionalFlag(t *testing.T) {
	c := &cobra.Command{Use: "test", Short: "test cmd"}
	c.Flags().String("opt", "default", "an optional flag")

	flags := extractFlags(c)
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	if flags[0].Required {
		t.Error("expected Required=false for flag with no required annotation")
	}
	if flags[0].Default != "default" {
		t.Errorf("expected Default=%q, got %q", "default", flags[0].Default)
	}
}

// TestExtractFlagsBashCompAnnotationNotTrue verifies that a flag whose
// BashCompOneRequiredFlag annotation has a value other than "true" is not
// treated as required.
func TestExtractFlagsBashCompAnnotationNotTrue(t *testing.T) {
	c := &cobra.Command{Use: "test", Short: "test cmd"}
	c.Flags().String("annotated", "", "flag with non-true annotation")
	f := c.Flags().Lookup("annotated")
	if f.Annotations == nil {
		f.Annotations = make(map[string][]string)
	}
	f.Annotations[cobra.BashCompOneRequiredFlag] = []string{"false"}

	flags := extractFlags(c)
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d: %v", len(flags), flags)
	}
	if flags[0].Required {
		t.Error("expected Required=false when annotation value is 'false'")
	}
}

// TestExtractFlagsLocalFlagsOnly verifies that inherited (persistent parent)
// flags are not included — extractFlags uses c.LocalFlags().
func TestExtractFlagsLocalFlagsOnly(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	parent.PersistentFlags().String("inherited", "", "inherited flag")

	child := &cobra.Command{Use: "child", Short: "child cmd"}
	child.Flags().String("local", "", "local flag")
	parent.AddCommand(child)

	flags := extractFlags(child)
	// Should contain only "local", not "inherited".
	for _, f := range flags {
		if f.Name == "inherited" {
			t.Error("extractFlags should not include inherited persistent flags")
		}
	}
	found := false
	for _, f := range flags {
		if f.Name == "local" {
			found = true
		}
	}
	if !found {
		t.Error("extractFlags should include the local flag")
	}
}

// ---- walkCommands coverage ----

// TestWalkCommandsFiltersHiddenHelpCompletion verifies that hidden commands,
// commands named "help", and commands named "completion" are all skipped.
func TestWalkCommandsFiltersHiddenHelpCompletion(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	// Hidden top-level command.
	hiddenCmd := &cobra.Command{Use: "hidden-resource", Short: "hidden", Hidden: true}
	root.AddCommand(hiddenCmd)

	// Named "help".
	helpCmd := &cobra.Command{Use: "help", Short: "help"}
	root.AddCommand(helpCmd)

	// Named "completion".
	completionCmd := &cobra.Command{Use: "completion", Short: "completion"}
	root.AddCommand(completionCmd)

	// A visible command with a hidden child and a completion child — those
	// children should be filtered, leaving visible list empty → SingleVerb=true.
	visible := &cobra.Command{Use: "myresource", Short: "my resource"}
	hiddenChild := &cobra.Command{Use: "hidden-verb", Short: "hidden verb", Hidden: true}
	helpChild := &cobra.Command{Use: "help", Short: "help"}
	completionChild := &cobra.Command{Use: "completion", Short: "completion"}
	realChild := &cobra.Command{Use: "do-thing", Short: "do thing"}
	visible.AddCommand(hiddenChild, helpChild, completionChild, realChild)
	root.AddCommand(visible)

	schema := map[schemaKey]generated.SchemaOp{}
	pages := walkCommands(root, schema)

	// Only "myresource" should appear — hidden, help, completion are filtered.
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d: %v", len(pages), pages)
	}
	if pages[0].Resource != "myresource" {
		t.Errorf("expected resource 'myresource', got %q", pages[0].Resource)
	}
	// realChild is the only visible child, so SingleVerb should be false and
	// verbs list should contain exactly "do-thing".
	if pages[0].SingleVerb {
		t.Error("expected SingleVerb=false because there is one visible child")
	}
	if len(pages[0].Verbs) != 1 || pages[0].Verbs[0].Name != "do-thing" {
		t.Errorf("expected verb 'do-thing', got %v", pages[0].Verbs)
	}
}

// TestWalkCommandsSingleVerbLeaf verifies that a top-level command with no
// visible children is treated as a SingleVerb leaf.
func TestWalkCommandsSingleVerbLeaf(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	leaf := &cobra.Command{Use: "configure", Short: "configure the CLI"}
	root.AddCommand(leaf)

	schema := map[schemaKey]generated.SchemaOp{}
	pages := walkCommands(root, schema)

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if !pages[0].SingleVerb {
		t.Error("expected SingleVerb=true for leaf command")
	}
}

// ---- renderTemplate coverage ----

// TestRenderTemplateParseError verifies that a template string with a syntax
// error propagates a parse error.
func TestRenderTemplateGendocsParseError(t *testing.T) {
	_, err := renderTemplate("bad", "{{ invalid", nil)
	if err == nil {
		t.Fatal("expected parse error from renderTemplate, got nil")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("error should mention 'parse template', got: %v", err)
	}
}

// TestRenderTemplateExecuteError verifies that a template execution failure
// (accessing a missing field on the wrong type) returns an execute error.
func TestRenderTemplateGendocsExecuteError(t *testing.T) {
	// Template accesses .Missing on a string — will fail at execute time.
	_, err := renderTemplate("exec", "{{ .Missing }}", "not a struct")
	if err == nil {
		t.Fatal("expected execute error from renderTemplate, got nil")
	}
	if !strings.Contains(err.Error(), "execute template") {
		t.Errorf("error should mention 'execute template', got: %v", err)
	}
}

// ---- writeFile coverage ----

// TestWriteFileMkdirAllError verifies that writeFile returns an error when
// the parent path is a regular file (MkdirAll cannot create a dir there).
func TestWriteFileMkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a regular file that blocks directory creation.
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o444); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Attempt to write to a path whose parent would need to be created inside
	// the file — MkdirAll fails because blocker is a file, not a dir.
	err := writeFile(filepath.Join(blocker, "subdir", "file.md"), []byte("content"))
	if err == nil {
		t.Fatal("expected MkdirAll error, got nil")
	}
}

// withBrokenTmplFuncs temporarily replaces the package-level tmplFuncs with a
// FuncMap that's missing the named functions so templates that reference them
// fail to parse. It restores the original FuncMap via t.Cleanup.
func withBrokenTmplFuncs(t *testing.T, omit ...string) {
	t.Helper()
	orig := tmplFuncs
	t.Cleanup(func() { tmplFuncs = orig })
	broken := make(template.FuncMap, len(orig))
	omitSet := make(map[string]bool, len(omit))
	for _, k := range omit {
		omitSet[k] = true
	}
	for k, v := range orig {
		if !omitSet[k] {
			broken[k] = v
		}
	}
	tmplFuncs = broken
}

// TestRunRenderResourcePageError verifies that run returns an error when the
// resource page template fails to render (forced by removing the "lower" func
// that resourcePageTmpl references).
func TestRunRenderResourcePageError(t *testing.T) {
	withBrokenTmplFuncs(t, "lower")
	tmpDir := t.TempDir()
	err := run(tmpDir)
	if err == nil {
		t.Fatal("expected render error for resource page, got nil")
	}
	if !strings.Contains(err.Error(), "render") {
		t.Logf("run error (acceptable): %v", err)
	}
}

// TestRunRenderIndexPageError verifies that run returns an error when the
// index template fails to render (forced by removing the "verbList" func that
// indexPageTmpl references). Resource pages must succeed first, so "lower" and
// "escapePipe" are kept intact.
func TestRunRenderIndexPageError(t *testing.T) {
	withBrokenTmplFuncs(t, "verbList")
	tmpDir := t.TempDir()
	err := run(tmpDir)
	if err == nil {
		t.Fatal("expected render error for index page, got nil")
	}
	if !strings.Contains(err.Error(), "render") && !strings.Contains(err.Error(), "index") {
		t.Logf("run error (acceptable): %v", err)
	}
}

// ---- run error branches ----

// TestRunWriteResourceFileError verifies that run returns an error when the
// commandsDir cannot be written (it is a file, not a directory).
func TestRunWriteResourceFileError(t *testing.T) {
	tmpDir := t.TempDir()
	// Make commandsDir a regular file so writeFile (via MkdirAll) fails.
	commandsDir := filepath.Join(tmpDir, "commands")
	if err := os.WriteFile(commandsDir, []byte("blocker"), 0o444); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err := run(tmpDir)
	if err == nil {
		t.Fatal("expected error when commandsDir is a file, got nil")
	}
}

// TestRunWriteIndexFileError verifies that run returns an error when the index
// page cannot be written (commandsDir is read-only after resource pages).
func TestRunWriteIndexFileError(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Pre-populate resource pages so run() writes them successfully.
	// Then make commandsDir read-only so index.md write fails.
	// But run() writes resource pages first — we need them to succeed then fail on index.
	// Trick: make commandsDir read-only AFTER resource pages are written via a
	// blocking file named "index.md" inside commandsDir (makes WriteFile fail).
	indexBlocker := filepath.Join(commandsDir, "index.md")
	if err := os.MkdirAll(indexBlocker, 0o755); err != nil {
		// index.md is a directory — WriteFile into it fails.
		t.Fatalf("MkdirAll for index blocker: %v", err)
	}
	err := run(tmpDir)
	if err == nil {
		t.Fatal("expected error when index.md cannot be written, got nil")
	}
	if !strings.Contains(err.Error(), "write") && !strings.Contains(err.Error(), "index") {
		t.Logf("run error (acceptable): %v", err)
	}
}

// TestRunWriteSidebarFileError verifies that run returns an error when the
// sidebar JSON file cannot be written.
func TestRunWriteSidebarFileError(t *testing.T) {
	tmpDir := t.TempDir()
	// Block .vitepress directory creation by making it a regular file.
	vitepressDir := filepath.Join(tmpDir, ".vitepress")
	if err := os.WriteFile(vitepressDir, []byte("blocker"), 0o444); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err := run(tmpDir)
	if err == nil {
		t.Fatal("expected error when .vitepress is a file, got nil")
	}
}

// TestRunWriteErrorCodesFileError verifies that run returns an error when the
// guide/error-codes.md file cannot be written.
func TestRunWriteErrorCodesFileError(t *testing.T) {
	tmpDir := t.TempDir()
	// Block guide directory creation by making it a regular file.
	guideDir := filepath.Join(tmpDir, "guide")
	if err := os.WriteFile(guideDir, []byte("blocker"), 0o444); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err := run(tmpDir)
	if err == nil {
		t.Fatal("expected error when guide dir is a file, got nil")
	}
}

// ---- main coverage ----

// mainTestOutputDir is populated by TestMain immediately before main() is
// invoked so TestMainSuccess can assert the side-effects.
var mainTestOutputDir string

// TestMain invokes main() exactly once (before running any test) so that
// flag.String("output") is not registered twice — a second registration would
// panic. Tests that need the result read mainTestOutputDir.
func TestMain(m *testing.M) {
	// Create a temporary directory that survives for the lifetime of the process.
	dir, err := os.MkdirTemp("", "gendocs-main-test-")
	if err != nil {
		panic("TestMain: os.MkdirTemp: " + err.Error())
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	// Override os.Args so main()'s flag.Parse() picks up --output=<dir>.
	origArgs := os.Args
	os.Args = []string{origArgs[0], "--output=" + dir}
	mainTestOutputDir = dir
	main()
	os.Args = origArgs

	// Do NOT call os.Exit — return instead so that coverage data is flushed
	// correctly by the testing framework. If tests fail, the framework handles
	// the non-zero exit via its own os.Exit call.
	code := m.Run()
	os.RemoveAll(dir) //nolint:errcheck
	os.Exit(code)
}

// TestMainSuccess verifies that main() wrote the expected output files.
func TestMainSuccess(t *testing.T) {
	if _, err := os.Stat(filepath.Join(mainTestOutputDir, "commands", "index.md")); err != nil {
		t.Errorf("commands/index.md not created by main(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(mainTestOutputDir, ".vitepress", "sidebar-commands.json")); err != nil {
		t.Errorf("sidebar-commands.json not created by main(): %v", err)
	}
}

// Ensure the pflag import is used (it is needed for TestExtractFlagsLocalFlagsOnly).
var _ = pflag.Flag{}
