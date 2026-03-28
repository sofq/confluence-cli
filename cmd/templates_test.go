package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/config"
)

// setupTemplateEnv creates a temp config dir with optional templates and sets
// CF_CONFIG_PATH. Returns the config dir path.
func setupTemplateEnv(t *testing.T, srvURL string, templates map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	baseURL := ""
	if srvURL != "" {
		baseURL = srvURL + "/wiki/api/v2"
	}

	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				BaseURL: baseURL,
				Auth: config.AuthConfig{
					Type:  "bearer",
					Token: "test-token",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	if templates != nil {
		tmplDir := filepath.Join(dir, "templates")
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

	return dir
}

func TestTemplatesList_WithTemplates(t *testing.T) {
	setupTemplateEnv(t, "", map[string]string{
		"meeting-notes": `{"title":"{{.title}}","body":"<p>Meeting</p>"}`,
		"status-report": `{"title":"Status","body":"<p>Report</p>"}`,
	})

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"templates", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	type entry struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	var entries []entry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, buf.String())
	}
	// 2 user templates + 5 built-in not overlapping (adr, blank, decision, retrospective, runbook) = 7.
	// "meeting-notes" is both user and built-in; user wins.
	if len(entries) != 7 {
		t.Fatalf("got %d templates, want 7; entries=%v", len(entries), entries)
	}
	// Verify user templates have source "user".
	for _, e := range entries {
		if e.Name == "meeting-notes" || e.Name == "status-report" {
			if e.Source != "user" {
				t.Errorf("%s source = %q, want %q", e.Name, e.Source, "user")
			}
		}
	}
}

func TestTemplatesList_EmptyDir(t *testing.T) {
	setupTemplateEnv(t, "", map[string]string{})

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"templates", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	type entry struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	var entries []entry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, buf.String())
	}
	// No user templates, but 6 built-in templates should appear.
	if len(entries) != 6 {
		t.Errorf("got %d entries, want 6 built-in; entries=%v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Source != "builtin" {
			t.Errorf("%s source = %q, want %q", e.Name, e.Source, "builtin")
		}
	}
}

func TestTemplatesShow_Builtin(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"templates", "show", "meeting-notes"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var output struct {
		Name      string   `json:"name"`
		Title     string   `json:"title"`
		Body      string   `json:"body"`
		Source    string   `json:"source"`
		Variables []string `json:"variables"`
	}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("unmarshal: %v (raw: %s)", err, buf.String())
	}
	if output.Name != "meeting-notes" {
		t.Errorf("name = %q, want %q", output.Name, "meeting-notes")
	}
	if output.Source != "builtin" {
		t.Errorf("source = %q, want %q", output.Source, "builtin")
	}
	if len(output.Variables) == 0 {
		t.Error("expected non-empty variables array")
	}
	// Verify XHTML is NOT escaped (no \u003c in output).
	if strings.Contains(buf.String(), "\\u003c") {
		t.Errorf("output contains HTML-escaped characters: %s", buf.String())
	}
}

func TestTemplatesShow_UserTemplate(t *testing.T) {
	setupTemplateEnv(t, "", map[string]string{
		"my-tmpl": `{"title":"{{.title}}","body":"<p>Hello {{.name}}</p>"}`,
	})

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"templates", "show", "my-tmpl"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var output struct {
		Name      string   `json:"name"`
		Title     string   `json:"title"`
		Body      string   `json:"body"`
		Source    string   `json:"source"`
		Variables []string `json:"variables"`
	}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("unmarshal: %v (raw: %s)", err, buf.String())
	}
	if output.Name != "my-tmpl" {
		t.Errorf("name = %q, want %q", output.Name, "my-tmpl")
	}
	if output.Source != "user" {
		t.Errorf("source = %q, want %q", output.Source, "user")
	}
	if len(output.Variables) != 2 {
		t.Errorf("got %d variables, want 2 (title, name)", len(output.Variables))
	}
}

func TestTemplatesShow_NotFound(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := cmd.RootCommand()
	rootCmd.SetArgs([]string{"templates", "show", "nonexistent-template"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)

	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
	if !strings.Contains(stderrBuf.String(), "not_found") {
		t.Errorf("expected not_found error, got: %s", stderrBuf.String())
	}
}

func TestTemplatesCreate_FromPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasPrefix(r.URL.Path, "/wiki/api/v2/pages/") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		// Verify body-format=storage query param.
		if r.URL.Query().Get("body-format") != "storage" {
			t.Errorf("expected body-format=storage, got %q", r.URL.Query().Get("body-format"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "123",
			"title": "My Page Title",
			"body": map[string]any{
				"storage": map[string]any{
					"representation": "storage",
					"value":          "<p>Page content here</p>",
				},
			},
		})
	}))
	defer srv.Close()

	dir := setupTemplateEnv(t, srv.URL, nil)

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{
		"templates", "create",
		"--from-page", "123",
		"--name", "my-template",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Verify output includes created status.
	if !strings.Contains(buf.String(), "created") {
		t.Errorf("expected 'created' in output, got: %s", buf.String())
	}

	// Verify template file was written.
	tmplPath := filepath.Join(dir, "templates", "my-template.json")
	data, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("template file not found at %s: %v", tmplPath, err)
	}
	if !strings.Contains(string(data), "My Page Title") {
		t.Errorf("template file missing title: %s", data)
	}
}

func TestTemplatesCreate_MissingName(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := cmd.RootCommand()
	rootCmd.SetArgs([]string{
		"templates", "create",
		"--from-page", "123",
		"--name", "",
	})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)

	if err == nil {
		t.Fatal("expected error for missing --name")
	}
	if !strings.Contains(stderrBuf.String(), "validation_error") {
		t.Errorf("expected validation_error, got: %s", stderrBuf.String())
	}
}

func TestPagesCreate_WithTemplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/wiki/api/v2/pages" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		// Verify template-rendered content
		if title, _ := body["title"].(string); title != "Weekly Standup" {
			t.Errorf("title = %q, want %q", title, "Weekly Standup")
		}
		bodyObj, _ := body["body"].(map[string]interface{})
		if val, _ := bodyObj["value"].(string); !strings.Contains(val, "2026-03-20") {
			t.Errorf("body value missing date: %q", val)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"id": "123", "title": "Weekly Standup"})
	}))
	defer srv.Close()

	setupTemplateEnv(t, srv.URL, map[string]string{
		"meeting-notes": `{"title":"{{.title}}","body":"<p>Meeting on {{.date}}</p>"}`,
	})

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{
		"pages", "create",
		"--template", "meeting-notes",
		"--var", "title=Weekly Standup",
		"--var", "date=2026-03-20",
		"--space-id", "SPACE1",
		"--body", "",
		"--title", "",
		"--parent-id", "",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
}

// TestPagesCreate_ZZ_TemplateAndBodyConflict is named with ZZ prefix to run last
// in alphabetical order, since cobra package-level commands retain flag state
// across tests (the --body flag set here would leak into subsequent tests).
func TestPagesCreate_ZZ_TemplateAndBodyConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	}))
	defer srv.Close()

	setupTemplateEnv(t, srv.URL, map[string]string{
		"meeting-notes": `{"title":"T","body":"<p>B</p>"}`,
	})

	// Capture stderr to verify JSON error output.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := cmd.RootCommand()
	rootCmd.SetArgs([]string{
		"pages", "create",
		"--template", "meeting-notes",
		"--body", "<p>manual body</p>",
		"--space-id", "SPACE1",
		"--title", "Test",
	})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)

	if err == nil {
		t.Fatal("expected error for --template + --body conflict")
	}
	if !strings.Contains(stderrBuf.String(), "cannot use --template and --body together") {
		t.Errorf("stderr missing expected message: %s", stderrBuf.String())
	}
}

func TestBlogpostsCreate_WithTemplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/wiki/api/v2/blogposts" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if title, _ := body["title"].(string); title != "March Update" {
			t.Errorf("title = %q, want %q", title, "March Update")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"id": "456", "title": "March Update"})
	}))
	defer srv.Close()

	setupTemplateEnv(t, srv.URL, map[string]string{
		"blog-update": `{"title":"{{.title}}","body":"<p>Update for {{.month}}</p>"}`,
	})

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{
		"blogposts", "create-blog-post",
		"--template", "blog-update",
		"--var", "title=March Update",
		"--var", "month=March",
		"--space-id", "SPACE1",
		"--body", "",
		"--title", "",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
}
