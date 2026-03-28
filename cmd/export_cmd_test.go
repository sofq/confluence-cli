package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

// runExportCommand executes `cf export` with the given args against the test server,
// capturing stdout and stderr. Uses setupTemplateEnv for config setup.
func runExportCommand(t *testing.T, srvURL string, args ...string) (stdout string, stderr string) {
	t.Helper()
	setupTemplateEnv(t, srvURL, nil)

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs(append([]string{"export"}, args...))
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)

	return outBuf.String(), errBuf.String()
}

func TestExport_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correct path and query params.
		if r.URL.Query().Get("body-format") != "storage" {
			t.Errorf("expected body-format=storage, got %q", r.URL.Query().Get("body-format"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "123",
			"title": "Test Page",
			"body": map[string]any{
				"storage": map[string]any{
					"representation": "storage",
					"value":          "<p>Hello</p>",
				},
			},
		})
	}))
	defer srv.Close()

	stdout, _ := runExportCommand(t, srv.URL, "--id", "123")

	if !strings.Contains(stdout, "storage") {
		t.Errorf("expected body output to contain 'storage', got: %s", stdout)
	}
	// Body content may have HTML entities escaped (\u003cp\u003e) by Go's JSON encoder.
	if !strings.Contains(stdout, "Hello") {
		t.Errorf("expected body output to contain page content, got: %s", stdout)
	}
}

func TestExport_ViewFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("body-format") != "view" {
			t.Errorf("expected body-format=view, got %q", r.URL.Query().Get("body-format"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "123", "title": "Test",
			"body": map[string]any{
				"view": map[string]any{
					"representation": "view",
					"value":          "<div>Rendered</div>",
				},
			},
		})
	}))
	defer srv.Close()

	stdout, _ := runExportCommand(t, srv.URL, "--id", "123", "--format", "view")

	if !strings.Contains(stdout, "view") {
		t.Errorf("expected view format in output, got: %s", stdout)
	}
}

func TestExport_MissingID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	}))
	defer srv.Close()

	_, stderr := runExportCommand(t, srv.URL, "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
}

func TestExport_Tree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/pages/100") && !strings.Contains(path, "/children"):
			// Root page
			json.NewEncoder(w).Encode(map[string]any{
				"id": "100", "title": "Root",
				"body": map[string]any{
					"storage": map[string]any{"representation": "storage", "value": "<p>Root</p>"},
				},
			})
		case strings.HasSuffix(path, "/pages/100/children"):
			// Children of root
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "200", "title": "Child A"},
					{"id": "300", "title": "Child B"},
				},
				"_links": map[string]string{},
			})
		case strings.HasSuffix(path, "/pages/200") && !strings.Contains(path, "/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "200", "title": "Child A",
				"body": map[string]any{
					"storage": map[string]any{"representation": "storage", "value": "<p>A</p>"},
				},
			})
		case strings.HasSuffix(path, "/pages/200/children"):
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
		case strings.HasSuffix(path, "/pages/300") && !strings.Contains(path, "/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "300", "title": "Child B",
				"body": map[string]any{
					"storage": map[string]any{"representation": "storage", "value": "<p>B</p>"},
				},
			})
		case strings.HasSuffix(path, "/pages/300/children"):
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
		default:
			t.Errorf("unexpected path: %s", path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	stdout, _ := runExportCommand(t, srv.URL, "--id", "100", "--tree")

	// Parse NDJSON lines.
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines, got %d: %v", len(lines), lines)
	}

	// Verify first line is root (depth 0).
	var first struct {
		ID    string `json:"id"`
		Depth int    `json:"depth"`
	}
	json.Unmarshal([]byte(lines[0]), &first)
	if first.ID != "100" || first.Depth != 0 {
		t.Errorf("first line: id=%q depth=%d, want id=100 depth=0", first.ID, first.Depth)
	}

	// Verify children have depth 1.
	var second struct {
		ID       string `json:"id"`
		ParentID string `json:"parentId"`
		Depth    int    `json:"depth"`
	}
	json.Unmarshal([]byte(lines[1]), &second)
	if second.Depth != 1 {
		t.Errorf("second line depth = %d, want 1", second.Depth)
	}
	if second.ParentID != "100" {
		t.Errorf("second line parentId = %q, want %q", second.ParentID, "100")
	}
}

func TestExport_TreeDepthLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/pages/100") && !strings.Contains(path, "/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "100", "title": "Root",
				"body": map[string]any{"storage": map[string]any{"representation": "storage", "value": "<p>Root</p>"}},
			})
		case strings.HasSuffix(path, "/pages/100/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "200", "title": "Child"}},
				"_links":  map[string]string{},
			})
		case strings.HasSuffix(path, "/pages/200") && !strings.Contains(path, "/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "200", "title": "Child",
				"body": map[string]any{"storage": map[string]any{"representation": "storage", "value": "<p>Child</p>"}},
			})
		case strings.HasSuffix(path, "/pages/200/children"):
			// Should NOT be called when depth=1, but if it is, return a grandchild
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "300", "title": "Grandchild"}},
				"_links":  map[string]string{},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	stdout, _ := runExportCommand(t, srv.URL, "--id", "100", "--tree", "--depth", "1")

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// depth=1 means root (depth 0) + children (depth 1) but NOT grandchildren
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines with depth=1, got %d: %v", len(lines), lines)
	}
}

func TestExport_TreePartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/pages/100") && !strings.Contains(path, "/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "100", "title": "Root",
				"body": map[string]any{"storage": map[string]any{"representation": "storage", "value": "<p>Root</p>"}},
			})
		case strings.HasSuffix(path, "/pages/100/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "200", "title": "Accessible"},
					{"id": "403page", "title": "Forbidden"},
				},
				"_links": map[string]string{},
			})
		case strings.HasSuffix(path, "/pages/200") && !strings.Contains(path, "/children"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "200", "title": "Accessible",
				"body": map[string]any{"storage": map[string]any{"value": "<p>OK</p>"}},
			})
		case strings.HasSuffix(path, "/pages/200/children"):
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
		case strings.HasSuffix(path, "/pages/403page"):
			w.WriteHeader(403)
			fmt.Fprintf(w, `{"message":"forbidden"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	stdout, _ := runExportCommand(t, srv.URL, "--id", "100", "--tree")

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// Root + accessible child = 2 lines (forbidden child skipped)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 NDJSON lines, got %d: %v", len(lines), lines)
	}
}
