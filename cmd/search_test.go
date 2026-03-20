package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

// TestRunSearch_SinglePage verifies that a single-page search result is output
// as a flat JSON array.
func TestRunSearch_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The search command calls /wiki/rest/api/search
		if !strings.Contains(r.URL.Path, "/wiki/rest/api/search") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "1", "type": "page"}},
			"_links":  map[string]any{},
		})
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	rp, wp, _ := os.Pipe()
	os.Stdout = wp

	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"search", "--cql", "type=page"})
	_ = root.Execute()

	wp.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rp)
	output := strings.TrimSpace(outBuf.String())

	if output == "" {
		t.Fatal("expected output from search, got nothing")
	}

	var results []json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not a JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

// TestRunSearch_TwoPages verifies that when two pages of results are returned,
// they are merged into a single flat JSON array.
func TestRunSearch_TwoPages(t *testing.T) {
	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			// First page: includes a next link
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "1"}},
				"_links":  map[string]any{"next": "/wiki/rest/api/search?cursor=abc&cql=type%3Dpage"},
			})
		} else {
			// Second page: no next link
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "2"}},
				"_links":  map[string]any{},
			})
		}
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	rp, wp, _ := os.Pipe()
	os.Stdout = wp

	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"search", "--cql", "type=page"})
	_ = root.Execute()

	wp.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rp)
	output := strings.TrimSpace(outBuf.String())

	var results []json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not a JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 merged results, got %d\nOutput: %s", len(results), output)
	}
}

// TestRunSearch_CursorTooLong verifies that when the next URL exceeds 4000 chars,
// pagination stops and a warning is written to stderr.
func TestRunSearch_CursorTooLong(t *testing.T) {
	// Build a very long cursor value
	longCursor := strings.Repeat("x", 4100)
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		// Always return a next link with a very long cursor
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "1"}},
			"_links":  map[string]any{"next": "/wiki/rest/api/search?cursor=" + longCursor},
		})
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	rp, wp, _ := os.Pipe()
	os.Stdout = wp

	oldStderr := os.Stderr
	rse, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"search", "--cql", "type=page"})
	_ = root.Execute()

	wp.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rp)

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(rse)
	stderrOutput := stderrBuf.String()

	// Only one request should have been made (cursor guard fires before second request)
	if requestCount != 1 {
		t.Errorf("expected 1 request (cursor guard), got %d", requestCount)
	}

	if !strings.Contains(stderrOutput, "cursor URL too long") && !strings.Contains(stderrOutput, "warning") {
		t.Errorf("expected cursor-too-long warning in stderr, got: %q", stderrOutput)
	}
}

// TestRunSearch_MissingCQL verifies that calling search without --cql returns ExitValidation.
func TestRunSearch_MissingCQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStderr := os.Stderr
	rse, wse, _ := os.Pipe()
	os.Stderr = wse

	oldStdout := os.Stdout
	_, wso, _ := os.Pipe()
	os.Stdout = wso

	root := cmd.RootCommand()
	root.SetArgs([]string{"search", "--cql", ""}) // empty --cql should trigger validation error
	err := root.Execute()

	wse.Close()
	wso.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(rse)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if err == nil {
		t.Error("expected error for missing --cql, got nil")
	}

	if stderrOutput != "" {
		var errOut map[string]any
		if jsonErr := json.Unmarshal([]byte(stderrOutput), &errOut); jsonErr == nil {
			if errOut["error_type"] != "validation_error" {
				t.Errorf("error_type = %v, want validation_error", errOut["error_type"])
			}
		}
	}
}

// TestSearchV1Domain verifies the domain extraction helper.
func TestSearchV1Domain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.atlassian.net/wiki/api/v2", "https://example.atlassian.net"},
		{"https://example.atlassian.net/wiki/rest/api/v1", "https://example.atlassian.net"},
		{"https://no-wiki-path.example.com", "https://no-wiki-path.example.com"},
	}
	for _, tt := range tests {
		got := cmd.SearchV1Domain(tt.input)
		if got != tt.want {
			t.Errorf("SearchV1Domain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

