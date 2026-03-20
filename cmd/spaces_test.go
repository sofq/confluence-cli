package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
)

// testSpacesAuth returns a minimal bearer AuthConfig for use in spaces tests.
func testSpacesAuth() config.AuthConfig {
	return config.AuthConfig{Type: "bearer", Token: "test-token"}
}

// makeTestSpacesClient creates a Client pointed at the given test server.
func makeTestSpacesClient(srv *httptest.Server) *client.Client {
	return &client.Client{
		BaseURL:    srv.URL,
		Auth:       testSpacesAuth(),
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
}

// TestResolveSpaceID_NumericPassThrough verifies that numeric strings are
// returned unchanged without making an API call.
func TestResolveSpaceID_NumericPassThrough(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := makeTestSpacesClient(srv)
	id, code := cmd.ResolveSpaceID(context.Background(), c, "123")
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if id != "123" {
		t.Fatalf("expected id=%q, got %q", "123", id)
	}
	if called {
		t.Fatal("resolveSpaceID must not call the API for numeric IDs")
	}
}

// TestResolveSpaceID_AlphaKeyResolvesViaAPI verifies that alpha keys trigger
// GET /spaces?keys=<KEY> and the resolved numeric ID is returned.
func TestResolveSpaceID_AlphaKeyResolvesViaAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("keys") != "ENG" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"results": []map[string]any{
				{"id": "456"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := makeTestSpacesClient(srv)
	id, code := cmd.ResolveSpaceID(context.Background(), c, "ENG")
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if id != "456" {
		t.Fatalf("expected id=%q, got %q", "456", id)
	}
}

// TestResolveSpaceID_NotFound verifies that a key not found in the API
// returns ExitNotFound and an empty ID.
func TestResolveSpaceID_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"results": []any{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := makeTestSpacesClient(srv)
	id, code := cmd.ResolveSpaceID(context.Background(), c, "NONEXISTENT")
	if code != cferrors.ExitNotFound {
		t.Fatalf("expected ExitNotFound, got %d", code)
	}
	if id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
}

// TestSpacesListCommandExists verifies that "cf spaces get" runs without panic.
func TestSpacesListCommandExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{"results":[],"_links":{}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"spaces", "get"})
	_ = root.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
}

// TestSpacesGetByIDCommandExists verifies that "cf spaces get-by-id --id 123"
// runs without panic.
func TestSpacesGetByIDCommandExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{"id":"123","key":"ENG"}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	rPipe, wPipe, _ := os.Pipe()
	os.Stdout = wPipe

	root := cmd.RootCommand()
	root.SetArgs([]string{"spaces", "get-by-id", "--id", "123"})
	_ = root.Execute()

	wPipe.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(rPipe)
	_ = buf.String()
}
