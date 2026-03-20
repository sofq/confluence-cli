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
		"meeting-notes":  `{"title":"{{.title}}","body":"<p>Meeting</p>"}`,
		"status-report":  `{"title":"Status","body":"<p>Report</p>"}`,
	})

	rootCmd := cmd.RootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"templates", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var names []string
	if err := json.Unmarshal(buf.Bytes(), &names); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, buf.String())
	}
	if len(names) != 2 {
		t.Fatalf("got %d templates, want 2", len(names))
	}
	if names[0] != "meeting-notes" || names[1] != "status-report" {
		t.Errorf("got %v, want [meeting-notes status-report]", names)
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

	var names []string
	if err := json.Unmarshal(buf.Bytes(), &names); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, buf.String())
	}
	if len(names) != 0 {
		t.Errorf("got %v, want empty array", names)
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
