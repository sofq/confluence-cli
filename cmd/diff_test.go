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

// runDiffCommand executes `cf diff` with the given args against the test server,
// capturing stdout and stderr. Uses setupTemplateEnv for config setup.
func runDiffCommand(t *testing.T, srvURL string, args ...string) (stdout string, stderr string) {
	t.Helper()
	cmd.ResetRootPersistentFlags()
	setupTemplateEnv(t, srvURL, nil)

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	// Reset diff subcommand flags to avoid contamination between tests.
	// Cobra retains parsed flag values on the global command singleton.
	for _, sub := range root.Commands() {
		if sub.Name() == "diff" {
			sub.ResetFlags()
			sub.Flags().String("id", "", "page ID to compare versions (required)")
			sub.Flags().String("since", "", "filter changes since duration (e.g. 2h, 1d) or ISO date (e.g. 2026-01-01)")
			sub.Flags().Int("from", 0, "start version number for explicit comparison")
			sub.Flags().Int("to", 0, "end version number for explicit comparison")
			break
		}
	}
	// Also reset --dry-run persistent flag from rootCmd to avoid contamination.
	_ = root.PersistentFlags().Set("dry-run", "false")
	root.SetArgs(append([]string{"diff"}, args...))
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

// dummyServer returns a no-op httptest server for tests that need a config URL
// but should not receive actual API calls.
func dummyServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
}

func TestDiff_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runDiffCommand(t, srv.URL, "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestDiff_SinceWithFromTo(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runDiffCommand(t, srv.URL, "--id", "123", "--since", "2h", "--from", "3")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "cannot use --since with --from/--to") {
		t.Errorf("expected mutual exclusivity error, got: %s", stderr)
	}
}

func TestDiff_DryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server in dry-run mode")
	}))
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "123", "--dry-run")

	if !strings.Contains(stdout, `"method"`) {
		t.Errorf("expected method in dry-run output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"url"`) {
		t.Errorf("expected url in dry-run output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "would fetch") {
		t.Errorf("expected 'would fetch' note in output, got: %s", stdout)
	}
}

func TestDiff_DefaultMode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"number": 2, "authorId": "user-2", "createdAt": "2026-03-15T00:00:00Z", "message": "update"},
				{"number": 1, "authorId": "user-1", "createdAt": "2026-03-01T00:00:00Z", "message": "initial"},
			},
			"_links": map[string]string{},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		version := r.URL.Query().Get("version")
		switch version {
		case "1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{
					"storage": map[string]any{"value": "<p>Hello</p>"},
				},
			})
		case "2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{
					"storage": map[string]any{"value": "<p>Hello World</p>"},
				},
			})
		default:
			w.WriteHeader(400)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "123")

	if !strings.Contains(stdout, `"pageId"`) {
		t.Errorf("expected pageId in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"diffs"`) {
		t.Errorf("expected diffs in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"linesAdded"`) {
		t.Errorf("expected linesAdded in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"linesRemoved"`) {
		t.Errorf("expected linesRemoved in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"authorId"`) {
		t.Errorf("expected authorId in output, got: %s", stdout)
	}
}

func TestDiff_SinceMode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"number": 3, "authorId": "user-3", "createdAt": "2026-03-28T10:00:00Z", "message": "third"},
				{"number": 2, "authorId": "user-2", "createdAt": "2026-03-28T08:00:00Z", "message": "second"},
				{"number": 1, "authorId": "user-1", "createdAt": "2026-01-01T00:00:00Z", "message": "first"},
			},
			"_links": map[string]string{},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		version := r.URL.Query().Get("version")
		switch version {
		case "2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{"storage": map[string]any{"value": "<p>V2</p>"}},
			})
		case "3":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{"storage": map[string]any{"value": "<p>V3</p>"}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{"storage": map[string]any{"value": "<p>V1</p>"}},
			})
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// --since with ISO date that includes v2 and v3 but not v1
	stdout, _ := runDiffCommand(t, srv.URL, "--id", "123", "--since", "2026-03-28")

	if !strings.Contains(stdout, `"pageId"`) {
		t.Errorf("expected pageId in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"since"`) {
		t.Errorf("expected since field in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"diffs"`) {
		t.Errorf("expected diffs in output, got: %s", stdout)
	}
}

func TestDiff_FromToMode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		version := r.URL.Query().Get("version")
		switch version {
		case "3":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{"storage": map[string]any{"value": "<p>Version 3</p>"}},
			})
		case "5":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Test",
				"body": map[string]any{"storage": map[string]any{"value": "<p>Version 5</p>\n<p>Extra line</p>"}},
			})
		default:
			w.WriteHeader(400)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "123", "--from", "3", "--to", "5")

	if !strings.Contains(stdout, `"pageId"`) {
		t.Errorf("expected pageId in output, got: %s", stdout)
	}

	// Verify the output has a single diff entry.
	var result struct {
		Diffs []struct {
			From struct{ Number int } `json:"from"`
			To   struct{ Number int } `json:"to"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nstdout: %s", err, stdout)
	}
	if len(result.Diffs) != 1 {
		t.Fatalf("expected 1 diff entry, got %d", len(result.Diffs))
	}
	if result.Diffs[0].From.Number != 3 {
		t.Errorf("expected from version 3, got %d", result.Diffs[0].From.Number)
	}
	if result.Diffs[0].To.Number != 5 {
		t.Errorf("expected to version 5, got %d", result.Diffs[0].To.Number)
	}
}

func TestDiff_EmptySinceRange(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// All versions are old -- outside any reasonable --since range.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"number": 1, "authorId": "user-1", "createdAt": "2020-01-01T00:00:00Z", "message": "old"},
			},
			"_links": map[string]string{},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "123", "--since", "2026-03-28")

	if !strings.Contains(stdout, `"diffs":[]`) && !strings.Contains(stdout, `"diffs": []`) {
		t.Errorf("expected empty diffs array, got: %s", stdout)
	}
}

func TestDiff_BodyUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"number": 2, "authorId": "user-2", "createdAt": "2026-03-15T00:00:00Z", "message": "update"},
				{"number": 1, "authorId": "user-1", "createdAt": "2026-03-01T00:00:00Z", "message": "initial"},
			},
			"_links": map[string]string{},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return page with empty body.storage.value (body unavailable).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "123", "title": "Test",
			"body": map[string]any{
				"storage": map[string]any{"value": ""},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "123")

	if !strings.Contains(stdout, `"note"`) {
		t.Errorf("expected note field when body unavailable, got: %s", stdout)
	}
	// Stats should be omitted (omitempty) or null when body unavailable.
	var result struct {
		Diffs []struct {
			Stats *struct{} `json:"stats"`
			Note  string    `json:"note"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nstdout: %s", err, stdout)
	}
	if len(result.Diffs) == 0 {
		t.Fatal("expected at least one diff entry")
	}
	if result.Diffs[0].Stats != nil {
		t.Error("expected stats to be omitted when body unavailable")
	}
	if result.Diffs[0].Note == "" {
		t.Error("expected non-empty note when body unavailable")
	}
}
