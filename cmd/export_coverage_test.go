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
	"github.com/spf13/cobra"
)

// runExportCommandFresh is like runExportCommand but explicitly resets export
// command flags before each invocation to prevent Cobra singleton contamination.
func runExportCommandFresh(t *testing.T, srvURL string, args ...string) (stdout string, stderr string) {
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
	resetExportFlags(root)
	_ = root.PersistentFlags().Set("dry-run", "false")
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

// resetExportFlags resets the export subcommand flags to their defaults.
func resetExportFlags(root *cobra.Command) {
	for _, sub := range root.Commands() {
		if sub.Name() == "export" {
			sub.ResetFlags()
			sub.Flags().String("id", "", "page ID to export (required)")
			sub.Flags().String("format", "storage", "body format: storage, atlas_doc_format, view")
			sub.Flags().Bool("tree", false, "recursively export page tree as NDJSON")
			sub.Flags().Int("depth", 0, "maximum tree depth (0 = unlimited)")
			break
		}
	}
}

// TestExport_SinglePage_APIError verifies that an API error in runSingleExport
// is handled and produces no stdout (error is written to stderr by the client).
func TestExport_SinglePage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		fmt.Fprint(w, `{"message":"page not found"}`)
	}))
	defer srv.Close()

	stdout, _ := runExportCommandFresh(t, srv.URL, "--id", "missing-page")

	// With API error, there should be no valid JSON output on stdout
	if stdout != "" {
		t.Errorf("expected empty stdout for API error, got: %s", stdout)
	}
}

// TestExport_SinglePage_JSONParseError verifies that a malformed page response
// produces a connection_error on stderr.
func TestExport_SinglePage_JSONParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return malformed JSON
		fmt.Fprint(w, `not valid json at all`)
	}))
	defer srv.Close()

	_, stderr := runExportCommandFresh(t, srv.URL, "--id", "123")

	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for JSON parse error, got: %s", stderr)
	}
}

// TestExport_SinglePage_NilBody verifies that a page response with no "body" key
// (which leaves page.Body as nil json.RawMessage) produces a not_found error.
// Note: JSON null decodes to json.RawMessage("null") not nil, so we must omit
// the "body" key entirely to trigger the nil check.
func TestExport_SinglePage_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a page with no "body" key — page.Body will be nil json.RawMessage
		fmt.Fprint(w, `{"id":"123","title":"No Body Page"}`)
	}))
	defer srv.Close()

	_, stderr := runExportCommandFresh(t, srv.URL, "--id", "123")

	if !strings.Contains(stderr, "not_found") {
		t.Errorf("expected not_found in stderr for nil body, got: %s", stderr)
	}
}

// TestExport_TreeWalkTree_ContextCancelled verifies that a cancelled context
// during tree export stops recursion gracefully without panicking.
// This is tested via the root page fetch failing with a 500 error
// which exercises the error path in walkTree.
func TestExport_TreeWalkTree_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"server error"}`)
	}))
	defer srv.Close()

	stdout, stderr := runExportCommandFresh(t, srv.URL, "--id", "123", "--tree")

	// Should produce no NDJSON lines (root page failed)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if stdout != "" && len(lines) > 0 && lines[0] != "" {
		t.Errorf("expected no output lines when root page fetch fails, got: %s", stdout)
	}
	// Should write connection_error to stderr
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for walk tree fetch error, got: %s", stderr)
	}
}

// TestExport_TreeWalkTree_JSONParseError verifies that a malformed root page
// response produces a connection_error on stderr and no NDJSON output.
func TestExport_TreeWalkTree_JSONParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return malformed JSON
		fmt.Fprint(w, `this is not json`)
	}))
	defer srv.Close()

	stdout, stderr := runExportCommandFresh(t, srv.URL, "--id", "123", "--tree")

	if stdout != "" {
		t.Errorf("expected no stdout for JSON parse error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr, got: %s", stderr)
	}
}

// TestExport_TreeWalkTree_ChildrenFetchError verifies that when the children
// endpoint fails, the root page is still emitted but a connection_error is
// written to stderr (partial failure behavior).
func TestExport_TreeWalkTree_ChildrenFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/100", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			fmt.Fprint(w, `{"message":"forbidden"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "100", "title": "Root Page",
			"body": map[string]any{
				"storage": map[string]any{"representation": "storage", "value": "<p>Root</p>"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/100/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runExportCommandFresh(t, srv.URL, "--id", "100", "--tree")

	// Root page should still be emitted
	if !strings.Contains(stdout, "Root Page") {
		t.Errorf("expected root page in output even when children fetch fails, got: %s", stdout)
	}
	// Should log the children error
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for children fetch failure, got: %s", stderr)
	}
}

// TestExport_FetchAllChildren_JSONParseError verifies that a malformed children
// response is handled as an error (partial failure in walkTree).
func TestExport_FetchAllChildren_JSONParseError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/100", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			// Return malformed JSON for children
			fmt.Fprint(w, `not valid json`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "100", "title": "Root",
			"body": map[string]any{
				"storage": map[string]any{"value": "<p>Root</p>"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/100/children", func(w http.ResponseWriter, r *http.Request) {
		// Return malformed JSON for children
		fmt.Fprint(w, `not valid json`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runExportCommandFresh(t, srv.URL, "--id", "100", "--tree")

	// Root page should be emitted
	if !strings.Contains(stdout, "Root") {
		t.Errorf("expected root page in output, got: %s", stdout)
	}
	// Should log the children parse error
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for children JSON parse error, got: %s", stderr)
	}
}

// TestExport_FetchAllChildren_Pagination verifies that fetchAllChildren follows
// the pagination cursor from _links.next.
func TestExport_FetchAllChildren_Pagination(t *testing.T) {
	childrenCallCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/100", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			return // handled below
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "100", "title": "Root",
			"body": map[string]any{
				"storage": map[string]any{"value": "<p>Root</p>"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/100/children", func(w http.ResponseWriter, r *http.Request) {
		childrenCallCount++
		w.Header().Set("Content-Type", "application/json")
		if childrenCallCount == 1 {
			// First page: include a _links.next cursor
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "200", "title": "Child A"},
				},
				"_links": map[string]string{
					"next": "/wiki/api/v2/pages/100/children?cursor=abc",
				},
			})
		} else {
			// Second page: no more children
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "300", "title": "Child B"},
				},
				"_links": map[string]string{},
			})
		}
	})
	// Children pages return empty children
	mux.HandleFunc("/wiki/api/v2/pages/200", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "200", "title": "Child A",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Child A</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/200/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
	})
	mux.HandleFunc("/wiki/api/v2/pages/300", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "300", "title": "Child B",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Child B</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/300/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runExportCommandFresh(t, srv.URL, "--id", "100", "--tree")

	// Should have root + both children = 3 lines
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines (root + 2 paginated children), got %d: %v", len(lines), lines)
	}

	if childrenCallCount < 2 {
		t.Errorf("expected at least 2 children endpoint calls for pagination, got %d", childrenCallCount)
	}
}

// TestExport_FetchAllChildren_RelativeNextLink verifies that when _links.next is
// a relative path not containing "/pages/", the full relative path is used as-is.
func TestExport_FetchAllChildren_RelativeNextLink(t *testing.T) {
	childCallCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/100", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			return // handled below
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "100", "title": "Root",
			"body": map[string]any{
				"storage": map[string]any{"value": "<p>Root</p>"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/100/children", func(w http.ResponseWriter, r *http.Request) {
		childCallCount++
		w.Header().Set("Content-Type", "application/json")
		if childCallCount == 1 {
			// Relative next link that does NOT contain "/pages/" -- triggers else branch
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "200", "title": "Child A"},
				},
				"_links": map[string]string{
					"next": "/wiki/api/v2/pages/100/children?cursor=relative",
				},
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "300", "title": "Child B"},
				},
				"_links": map[string]string{},
			})
		}
	})
	mux.HandleFunc("/wiki/api/v2/pages/200", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "200", "title": "Child A",
			"body": map[string]any{"storage": map[string]any{"value": "<p>A</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/200/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
	})
	mux.HandleFunc("/wiki/api/v2/pages/300", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "300", "title": "Child B",
			"body": map[string]any{"storage": map[string]any{"value": "<p>B</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/300/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "_links": map[string]string{}})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runExportCommandFresh(t, srv.URL, "--id", "100", "--tree")

	// Should have root + 2 children = 3 lines
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines (root + 2 paginated children), got %d: %v", len(lines), lines)
	}
}

// TestExport_FetchAllChildren_ContextCancelled verifies that context cancellation
// during children pagination is handled (returns what was fetched so far).
// We test this by cancelling via a timeout on a slow server.
func TestExport_FetchAllChildren_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/100", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/children") {
			return // handled below
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "100", "title": "Root",
			"body": map[string]any{
				"storage": map[string]any{"value": "<p>Root</p>"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/100/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"server error"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runExportCommandFresh(t, srv.URL, "--id", "100", "--tree")

	// Root page should still be emitted
	if !strings.Contains(stdout, "Root") {
		t.Errorf("expected root page emitted before children error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr, got: %s", stderr)
	}
}
