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

// testCustomContentAuth returns a minimal bearer AuthConfig for custom content tests.
func testCustomContentAuth() config.AuthConfig {
	return config.AuthConfig{Type: "bearer", Token: "test-token"}
}

// makeTestCustomContentClient creates a Client pointed at the given test server.
func makeTestCustomContentClient(srv *httptest.Server) *client.Client {
	return &client.Client{
		BaseURL:    srv.URL,
		Auth:       testCustomContentAuth(),
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
}

// TestFetchCustomContentMeta_Success verifies that FetchCustomContentMeta
// returns the correct version number and type from a successful API response.
func TestFetchCustomContentMeta_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/custom-content/42") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "42",
			"title": "Test",
			"type":  "ac:app:custom-type",
			"version": map[string]any{
				"number": 5,
			},
		})
	}))
	defer srv.Close()

	c := makeTestCustomContentClient(srv)
	meta, code := cmd.FetchCustomContentMeta(context.Background(), c, "42")
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if meta.Version != 5 {
		t.Fatalf("expected version 5, got %d", meta.Version)
	}
	if meta.Type != "ac:app:custom-type" {
		t.Fatalf("expected type ac:app:custom-type, got %q", meta.Type)
	}
}

// TestFetchCustomContentMeta_NotFound verifies that a 404 response returns (zero meta, non-zero code).
func TestFetchCustomContentMeta_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_type": "not_found",
			"message":    "custom content not found",
		})
	}))
	defer srv.Close()

	c := makeTestCustomContentClient(srv)
	meta, code := cmd.FetchCustomContentMeta(context.Background(), c, "nonexistent")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for 404, got ExitOK")
	}
	if meta.Version != 0 {
		t.Fatalf("expected version 0 on error, got %d", meta.Version)
	}
}

// TestCustomContentList_RequiresType verifies that get-custom-content-by-type
// returns a validation error when --type is not provided.
func TestCustomContentList_RequiresType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// --type intentionally omitted
	root.SetArgs([]string{"custom-content", "get-custom-content-by-type"})
	err := root.Execute()

	we.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if err == nil {
		t.Error("expected an error for missing --type, got nil")
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

// TestCustomContentCreate_RequiresType verifies that create-custom-content
// returns a validation error when --type is not provided.
func TestCustomContentCreate_RequiresType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// --type intentionally omitted, but other required flags present
	root.SetArgs([]string{"custom-content", "create-custom-content", "--space-id", "123", "--title", "Test", "--body", "<p>hi</p>"})
	err := root.Execute()

	we.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if err == nil {
		t.Error("expected an error for missing --type, got nil")
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

// TestCustomContentUpdate_409Retry verifies the version fetch + update + 409 + refetch + succeed flow.
func TestCustomContentUpdate_409Retry(t *testing.T) {
	var getCount int64
	var putCount int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/custom-content/"):
			n := atomic.AddInt64(&getCount, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "77", "title": "Custom Content", "type": "ac:app:test-type",
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

	c := makeTestCustomContentClient(srv)
	ctx := context.Background()

	// Step 1: Fetch current version and type
	meta1, code := cmd.FetchCustomContentMeta(ctx, c, "77")
	if code != cferrors.ExitOK {
		t.Fatalf("initial GET failed: %d", code)
	}

	// Step 2: First update attempt (should fail with 409)
	code = cmd.DoCustomContentUpdate(ctx, c, "77", meta1.Type, "Test Title", "<p>body content</p>", meta1.Version+1)
	if code != cferrors.ExitConflict {
		t.Fatalf("expected ExitConflict on first PUT, got %d", code)
	}

	// Step 3: Retry -- fetch version again
	meta2, code := cmd.FetchCustomContentMeta(ctx, c, "77")
	if code != cferrors.ExitOK {
		t.Fatalf("retry GET failed: %d", code)
	}

	// Step 4: Second update attempt (should succeed)
	code = cmd.DoCustomContentUpdate(ctx, c, "77", meta2.Type, "Test Title", "<p>body content</p>", meta2.Version+1)
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK on retry PUT, got %d", code)
	}

	// Verify exactly 2 GETs were made
	if n := atomic.LoadInt64(&getCount); n != 2 {
		t.Errorf("expected 2 GET requests (version fetches), got %d", n)
	}
}
