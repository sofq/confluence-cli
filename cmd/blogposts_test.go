package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
)

// testBlogpostsAuth returns a minimal bearer AuthConfig for use in blog post tests.
func testBlogpostsAuth() config.AuthConfig {
	return config.AuthConfig{Type: "bearer", Token: "test-token"}
}

// makeTestBlogpostsClient creates a Client pointed at the given test server.
func makeTestBlogpostsClient(srv *httptest.Server) *client.Client {
	return &client.Client{
		BaseURL:    srv.URL,
		Auth:       testBlogpostsAuth(),
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
}

// TestFetchBlogpostVersion_Success verifies that FetchBlogpostVersion returns the correct
// version number from a successful API response.
func TestFetchBlogpostVersion_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/blogposts/42") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "42",
			"title": "Test",
			"version": map[string]any{
				"number": 5,
			},
		})
	}))
	defer srv.Close()

	c := makeTestBlogpostsClient(srv)
	ver, code := cmd.FetchBlogpostVersion(context.Background(), c, "42")
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if ver != 5 {
		t.Fatalf("expected version 5, got %d", ver)
	}
}

// TestFetchBlogpostVersion_NotFound verifies that a 404 response returns (0, non-zero code).
func TestFetchBlogpostVersion_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_type": "not_found",
			"message":    "blog post not found",
		})
	}))
	defer srv.Close()

	c := makeTestBlogpostsClient(srv)
	ver, code := cmd.FetchBlogpostVersion(context.Background(), c, "nonexistent")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for 404, got ExitOK")
	}
	if ver != 0 {
		t.Fatalf("expected version 0 on error, got %d", ver)
	}
}

// TestDoBlogpostUpdate_SendsCorrectBody verifies that DoBlogpostUpdate sends the correct
// JSON body in the PUT request with all required fields.
func TestDoBlogpostUpdate_SendsCorrectBody(t *testing.T) {
	var capturedBody bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if _, err := capturedBody.ReadFrom(r.Body); err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "99", "status": "current"})
	}))
	defer srv.Close()

	c := makeTestBlogpostsClient(srv)
	code := cmd.DoBlogpostUpdate(context.Background(), c, "99", "My Title", "<p>content</p>", 7)
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}

	var body map[string]any
	if err := json.Unmarshal(capturedBody.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\nBody: %s", err, capturedBody.String())
	}
	if body["id"] != "99" {
		t.Errorf("expected id=%q, got %v", "99", body["id"])
	}
	if body["status"] != "current" {
		t.Errorf("expected status=%q, got %v", "current", body["status"])
	}
	if body["title"] != "My Title" {
		t.Errorf("expected title=%q, got %v", "My Title", body["title"])
	}
	bodyField, ok := body["body"].(map[string]any)
	if !ok {
		t.Fatalf("body.body is not a map: %v", body["body"])
	}
	if bodyField["representation"] != "storage" {
		t.Errorf("expected body.representation=%q, got %v", "storage", bodyField["representation"])
	}
	if bodyField["value"] != "<p>content</p>" {
		t.Errorf("expected body.value=%q, got %v", "<p>content</p>", bodyField["value"])
	}
	versionField, ok := body["version"].(map[string]any)
	if !ok {
		t.Fatalf("body.version is not a map: %v", body["version"])
	}
	// JSON numbers are decoded as float64
	if versionField["number"] != float64(7) {
		t.Errorf("expected version.number=7, got %v", versionField["number"])
	}
}

// TestBlogpostsWorkflowUpdate_RetryOn409 verifies that when the first PUT returns 409,
// a second version fetch + PUT is made and the final result is ExitOK.
func TestBlogpostsWorkflowUpdate_RetryOn409(t *testing.T) {
	var getCount int64
	var putCount int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/blogposts/"):
			n := atomic.AddInt64(&getCount, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "77", "title": "Blog Post",
				"version": map[string]any{"number": int(n) * 3},
			})
		case r.Method == "PUT":
			n := atomic.AddInt64(&putCount, 1)
			if n == 1 {
				// First PUT: return 409 conflict
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error_type": "conflict"})
				return
			}
			// Second PUT: success
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "77", "status": "current"})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := makeTestBlogpostsClient(srv)
	ctx := context.Background()

	// Step 1: Fetch current version
	ver1, code := cmd.FetchBlogpostVersion(ctx, c, "77")
	if code != cferrors.ExitOK {
		t.Fatalf("initial GET failed: %d", code)
	}

	// Step 2: First update attempt (should fail with 409)
	code = cmd.DoBlogpostUpdate(ctx, c, "77", "Test Title", "<p>body content</p>", ver1+1)
	if code != cferrors.ExitConflict {
		t.Fatalf("expected ExitConflict on first PUT, got %d", code)
	}

	// Step 3: Retry -- fetch version again
	ver2, code := cmd.FetchBlogpostVersion(ctx, c, "77")
	if code != cferrors.ExitOK {
		t.Fatalf("retry GET failed: %d", code)
	}

	// Step 4: Second update attempt (should succeed)
	code = cmd.DoBlogpostUpdate(ctx, c, "77", "Test Title", "<p>body content</p>", ver2+1)
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK on retry PUT, got %d", code)
	}

	// Verify exactly 2 GETs were made (one before first PUT, one before retry)
	if n := atomic.LoadInt64(&getCount); n != 2 {
		t.Errorf("expected 2 GET requests (version fetches), got %d", n)
	}
}

// TestBlogpostsWorkflowGetByID_InjectsBodyFormat verifies that the get-blog-post-by-id command
// always sends body-format=storage as a query parameter.
func TestBlogpostsWorkflowGetByID_InjectsBodyFormat(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "55", "title": "Test",
			"version": map[string]any{"number": 1},
		})
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"blogposts", "get-blog-post-by-id", "--id", "55"})
	_ = root.Execute()

	w.Close()
	os.Stdout = oldStdout

	if !strings.Contains(capturedQuery, "body-format=storage") {
		t.Errorf("expected body-format=storage in query, got: %q", capturedQuery)
	}
}

// TestBlogpostsWorkflowCreate_ValidationError verifies that calling create-blog-post with a
// missing --space-id returns an error (ExitValidation) without panicking.
func TestBlogpostsWorkflowCreate_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called
		t.Error("unexpected HTTP call during validation test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStderr := os.Stderr
	r, we, _ := os.Pipe()
	os.Stderr = we

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	// space-id intentionally omitted -- should trigger ExitValidation
	root.SetArgs([]string{"blogposts", "create-blog-post", "--title", "Test", "--body", "<p>hi</p>"})
	err := root.Execute()

	we.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if err == nil {
		t.Error("expected an error for missing --space-id, got nil")
	}

	if stderrOutput != "" {
		var errOut map[string]any
		if jsonErr := json.Unmarshal([]byte(stderrOutput), &errOut); jsonErr == nil {
			errType, _ := errOut["error_type"].(string)
			if errType != "validation_error" {
				t.Errorf("error_type = %q, want validation_error", errType)
			}
		}
	}
}
