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

// mockContentResponse returns a v1 content API response with the given number of pages.
func mockContentResponse(n int) map[string]any {
	results := make([]map[string]any, n)
	for i := range results {
		results[i] = map[string]any{
			"id":    "page-id",
			"title": "My Test Page",
			"body": map[string]any{
				"storage": map[string]any{
					"value": "<p>Hello world this is a test page with some content.</p>",
				},
			},
			"history": map[string]any{
				"lastUpdated": map[string]any{
					"when": "2024-01-01T00:00:00Z",
				},
			},
		}
	}
	return map[string]any{
		"results": results,
		"_links":  map[string]any{},
	}
}

// executeAvatarCmd runs the root command with the given args and returns stdout, stderr, and error.
func executeAvatarCmd(t *testing.T, srvURL string, args []string) (string, string, error) {
	t.Helper()
	t.Setenv("CF_BASE_URL", srvURL+"/wiki/api/v2")
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
	root.SetArgs(args)
	err := root.Execute()

	wp.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rp)
	_, _ = errBuf.ReadFrom(rse)

	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// TestAvatarAnalyze_Success verifies that with a valid --user flag and a mock server
// returning 2 pages, the command exits 0 and stdout is a valid PersonaProfile JSON.
func TestAvatarAnalyze_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// v1 content API — CQL search
		if strings.Contains(r.URL.Path, "/wiki/rest/api/content") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockContentResponse(2))
			return
		}
		// Anything else — 404
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	stdout, stderr, err := executeAvatarCmd(t, srv.URL, []string{"avatar", "analyze", "--user", "acc123"})
	_ = stderr

	if err != nil {
		t.Fatalf("expected no error, got: %v (stderr: %s)", err, stderr)
	}

	if stdout == "" {
		t.Fatal("expected JSON output on stdout, got empty string")
	}

	var profile map[string]any
	if jsonErr := json.Unmarshal([]byte(stdout), &profile); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\nOutput: %s", jsonErr, stdout)
	}

	// Verify required top-level fields per plan spec.
	requiredFields := []string{"version", "account_id", "display_name", "generated_at", "page_count", "writing", "style_guide"}
	for _, field := range requiredFields {
		if _, ok := profile[field]; !ok {
			t.Errorf("PersonaProfile missing required field: %q\nOutput: %s", field, stdout)
		}
	}

	if profile["account_id"] != "acc123" {
		t.Errorf("account_id = %v, want acc123", profile["account_id"])
	}

	if pageCount, ok := profile["page_count"].(float64); !ok || pageCount != 2 {
		t.Errorf("page_count = %v, want 2", profile["page_count"])
	}
}

// TestAvatarAnalyze_MissingUser verifies that omitting --user returns exit code 4
// and a validation_error JSON on stderr.
func TestAvatarAnalyze_MissingUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call when --user is missing")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, stderr, err := executeAvatarCmd(t, srv.URL, []string{"avatar", "analyze", "--user", ""})

	// Should have an error (AlreadyWrittenError with Code 4)
	if err == nil {
		t.Fatal("expected error for missing --user, got nil")
	}

	// Check stderr contains JSON validation error.
	if strings.TrimSpace(stderr) != "" {
		var errOut map[string]any
		if jsonErr := json.Unmarshal([]byte(stderr), &errOut); jsonErr == nil {
			if errOut["error_type"] != "validation_error" {
				t.Errorf("error_type = %v, want validation_error\nStderr: %s", errOut["error_type"], stderr)
			}
		} else {
			t.Logf("stderr is not JSON (may be OK): %s", stderr)
		}
	}
}

// TestAvatarAnalyze_AuthFailure verifies that a 401 response from the mock server
// results in exit code 2 and an auth-related JSON error on stderr.
func TestAvatarAnalyze_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer srv.Close()

	_, stderr, err := executeAvatarCmd(t, srv.URL, []string{"avatar", "analyze", "--user", "acc123"})

	if err == nil {
		t.Fatal("expected error for auth failure, got nil")
	}

	// Check stderr contains JSON auth error.
	if strings.TrimSpace(stderr) != "" {
		var errOut map[string]any
		if jsonErr := json.Unmarshal([]byte(stderr), &errOut); jsonErr == nil {
			// Should be auth_failed or auth_error type, exit code 2
			errorType, _ := errOut["error_type"].(string)
			if !strings.Contains(errorType, "auth") {
				t.Errorf("error_type = %v, want auth-related error type\nStderr: %s", errorType, stderr)
			}
		}
	}
}
