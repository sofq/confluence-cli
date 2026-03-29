package cmd_test

// coverage_gaps_test.go covers previously uncovered branches in:
//   - cmd/blogposts.go    (fetchBlogpostVersion)
//   - cmd/custom_content.go (fetchCustomContentMeta)
//   - cmd/labels.go       (fetchV1WithBody)
//   - cmd/pages.go        (fetchPageVersion)
//   - cmd/search.go       (fetchV1, runSearch)
//   - cmd/spaces.go       (resolveSpaceID)

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeMinimalClient creates a bare-minimum client with the given base URL.
// Stdout/Stderr use strings.Builder so output doesn't reach os.Stdout/Stderr.
func makeMinimalClient(baseURL string, httpClient *http.Client) *client.Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &client.Client{
		BaseURL:    baseURL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: httpClient,
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
}

// newCobraCmd returns a bare *cobra.Command with a background context and the
// given client injected, ready to be passed into FetchV1 / FetchV1WithBody.
func newCobraCmd(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{}
	ctx := client.NewContext(context.Background(), c)
	cmd.SetContext(ctx)
	return cmd
}

// dialRefused returns a URL that will refuse connections immediately.
func dialRefusedURL(t *testing.T) string {
	t.Helper()
	// Listen on a random port then close it so connecting to it gets refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr
}

// ---------------------------------------------------------------------------
// blogposts.go — fetchBlogpostVersion
// ---------------------------------------------------------------------------

// TestFetchBlogpostVersion_InvalidJSON covers the branch where the API returns
// a 200 OK with non-JSON body, triggering the json.Unmarshal error path.
func TestFetchBlogpostVersion_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-valid-json"))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "tok"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
	ver, code := cmd.FetchBlogpostVersion(context.Background(), c, "99")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for invalid JSON, got ExitOK")
	}
	if ver != 0 {
		t.Errorf("expected version=0 on parse error, got %d", ver)
	}
}

// ---------------------------------------------------------------------------
// custom_content.go — fetchCustomContentMeta
// ---------------------------------------------------------------------------

// TestFetchCustomContentMeta_InvalidJSON covers the branch where the API returns
// a 200 OK with non-JSON body, triggering the json.Unmarshal error path.
func TestFetchCustomContentMeta_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-valid-json"))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "tok"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
	meta, code := cmd.FetchCustomContentMeta(context.Background(), c, "77")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for invalid JSON, got ExitOK")
	}
	if meta.Version != 0 || meta.Type != "" {
		t.Errorf("expected zero meta on parse error, got %+v", meta)
	}
}

// ---------------------------------------------------------------------------
// pages.go — fetchPageVersion
// ---------------------------------------------------------------------------

// TestFetchPageVersion_InvalidJSON covers the branch where the API returns
// a 200 OK with non-JSON body, triggering the json.Unmarshal error path.
func TestFetchPageVersion_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-valid-json"))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "tok"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
	ver, code := cmd.FetchPageVersion(context.Background(), c, "42")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for invalid JSON, got ExitOK")
	}
	if ver != 0 {
		t.Errorf("expected version=0 on parse error, got %d", ver)
	}
}

// ---------------------------------------------------------------------------
// labels.go — fetchV1WithBody
// ---------------------------------------------------------------------------

// TestFetchV1WithBody_InvalidURL covers the branch where http.NewRequestWithContext
// fails due to an invalid URL (contains a control character).
func TestFetchV1WithBody_InvalidURL(t *testing.T) {
	c := makeMinimalClient("http://localhost", nil)
	cobraCmd := newCobraCmd(c)

	// "\x00" in the URL causes NewRequestWithContext to fail.
	_, code := cmd.FetchV1WithBody(cobraCmd, c, "POST", "http://localhost/\x00invalid", bytes.NewReader([]byte(`[]`)))
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for invalid URL, got ExitOK")
	}
}

// TestFetchV1WithBody_HTTPClientError covers the branch where the HTTP client
// itself fails (connection refused), not an HTTP-level error.
func TestFetchV1WithBody_HTTPClientError(t *testing.T) {
	refusedURL := dialRefusedURL(t)

	c := makeMinimalClient(refusedURL+"/wiki/api/v2", nil)
	cobraCmd := newCobraCmd(c)

	_, code := cmd.FetchV1WithBody(cobraCmd, c, "POST", refusedURL+"/wiki/rest/api/content/123/label", bytes.NewReader([]byte(`[]`)))
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code from connection refused, got ExitOK")
	}
}

// TestFetchV1WithBody_HTTP400 covers the branch where the server responds with a 4xx status.
func TestFetchV1WithBody_HTTP400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer srv.Close()

	c := makeMinimalClient(srv.URL+"/wiki/api/v2", srv.Client())
	cobraCmd := newCobraCmd(c)

	_, code := cmd.FetchV1WithBody(cobraCmd, c, "POST", srv.URL+"/wiki/rest/api/content/123/label", bytes.NewReader([]byte(`[]`)))
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for 400 response, got ExitOK")
	}
}

// TestFetchV1WithBody_HTTP204NoContent covers the 204 No Content branch that
// should return an empty JSON object body.
func TestFetchV1WithBody_HTTP204NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := makeMinimalClient(srv.URL+"/wiki/api/v2", srv.Client())
	cobraCmd := newCobraCmd(c)

	body, code := cmd.FetchV1WithBody(cobraCmd, c, "DELETE", srv.URL+"/wiki/rest/api/content/123/label", nil)
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK for 204, got %d", code)
	}
	if string(body) != "{}" {
		t.Errorf("expected '{}' for 204 response, got %q", string(body))
	}
}

// TestFetchV1WithBody_Success covers the happy path returning a non-empty JSON body.
func TestFetchV1WithBody_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := makeMinimalClient(srv.URL+"/wiki/api/v2", srv.Client())
	cobraCmd := newCobraCmd(c)

	body, code := cmd.FetchV1WithBody(cobraCmd, c, "POST", srv.URL+"/wiki/rest/api/content/123/label", bytes.NewReader([]byte(`[]`)))
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if len(body) == 0 {
		t.Error("expected non-empty response body")
	}
}

// ---------------------------------------------------------------------------
// search.go — fetchV1
// ---------------------------------------------------------------------------

// TestFetchV1_InvalidURL covers the branch where http.NewRequestWithContext
// fails due to an invalid URL (contains a control character).
func TestFetchV1_InvalidURL(t *testing.T) {
	c := makeMinimalClient("http://localhost", nil)
	cobraCmd := newCobraCmd(c)

	// "\x00" in a URL causes NewRequestWithContext to fail.
	_, code := cmd.FetchV1(cobraCmd, c, "http://localhost/\x00invalid")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for invalid URL, got ExitOK")
	}
}

// TestFetchV1_HTTPClientError covers the branch where c.HTTPClient.Do fails.
func TestFetchV1_HTTPClientError(t *testing.T) {
	refusedURL := dialRefusedURL(t)

	c := makeMinimalClient(refusedURL+"/wiki/api/v2", nil)
	cobraCmd := newCobraCmd(c)

	_, code := cmd.FetchV1(cobraCmd, c, refusedURL+"/wiki/rest/api/search?cql=type=page")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code from connection refused, got ExitOK")
	}
}

// TestFetchV1_HTTP400 covers the branch where the server responds with a 4xx status.
func TestFetchV1_HTTP400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	c := makeMinimalClient(srv.URL+"/wiki/api/v2", srv.Client())
	cobraCmd := newCobraCmd(c)

	_, code := cmd.FetchV1(cobraCmd, c, srv.URL+"/wiki/rest/api/search?cql=type=page")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for 401, got ExitOK")
	}
}

// TestFetchV1_Success covers the happy path.
func TestFetchV1_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"_links":{}}`))
	}))
	defer srv.Close()

	c := makeMinimalClient(srv.URL+"/wiki/api/v2", srv.Client())
	cobraCmd := newCobraCmd(c)

	body, code := cmd.FetchV1(cobraCmd, c, srv.URL+"/wiki/rest/api/search?cql=type=page")
	if code != cferrors.ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

// ---------------------------------------------------------------------------
// search.go — runSearch
// ---------------------------------------------------------------------------

// TestRunSearch_AbsoluteNextLink covers the branch where _links.next is an
// absolute URL (starts with "http"). The URL must be the mock server's own URL.
func TestRunSearch_AbsoluteNextLink(t *testing.T) {
	var callCount int
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// Return an absolute URL next link pointing to this same server.
			nextURL := srv.URL + "/wiki/rest/api/search?cursor=abc&cql=type%3Dpage"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "1"}},
				"_links":  map[string]any{"next": nextURL},
			})
		} else {
			// Second page: no next link.
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
		t.Errorf("expected 2 results (absolute URL pagination), got %d\nOutput: %s", len(results), output)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

// TestRunSearch_InvalidJSONResponse covers the json.Unmarshal error branch in
// runSearch when the API returns valid HTTP 200 but non-JSON body.
func TestRunSearch_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-valid-json"))
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
	root.SetArgs([]string{"search", "--cql", "type=page"})
	err := root.Execute()

	wse.Close()
	wso.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(rse)
	stderrOutput := stderrBuf.String()

	if err == nil {
		t.Error("expected error from non-JSON response, got nil")
	}
	if !strings.Contains(stderrOutput, "connection_error") && !strings.Contains(stderrOutput, "error") {
		t.Errorf("expected error in stderr, got: %q", stderrOutput)
	}
}

// TestRunSearch_FetchV1Error covers the branch where fetchV1 fails mid-pagination
// (the first request returns an error). The search command should propagate the error.
func TestRunSearch_FetchV1Error(t *testing.T) {
	// Start a server then close it immediately so the first fetch fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse
	oldStdout := os.Stdout
	_, wso, _ := os.Pipe()
	os.Stdout = wso

	root := cmd.RootCommand()
	root.SetArgs([]string{"search", "--cql", "type=page"})
	err := root.Execute()

	wse.Close()
	wso.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error from 500 response, got nil")
	}
}

// ---------------------------------------------------------------------------
// spaces.go — resolveSpaceID
// ---------------------------------------------------------------------------

// TestResolveSpaceID_InvalidJSONResponse covers the branch where the /spaces API
// returns 200 but invalid JSON, causing the json.Unmarshal or empty results check
// to return ExitNotFound.
func TestResolveSpaceID_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return invalid JSON to trigger the json.Unmarshal error branch.
		_, _ = w.Write([]byte("not-valid-json"))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "tok"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
	id, code := cmd.ResolveSpaceID(context.Background(), c, "ENG")
	if code != cferrors.ExitNotFound {
		t.Fatalf("expected ExitNotFound for invalid JSON, got %d", code)
	}
	if id != "" {
		t.Errorf("expected empty id on error, got %q", id)
	}
}

// TestResolveSpaceID_APIFetchError covers the branch where the /spaces API
// itself returns an HTTP error (e.g. 401), causing Fetch to return non-OK.
func TestResolveSpaceID_APIFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "tok"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
	id, code := cmd.ResolveSpaceID(context.Background(), c, "ENG")
	if code == cferrors.ExitOK {
		t.Fatal("expected non-zero exit code for 401, got ExitOK")
	}
	if id != "" {
		t.Errorf("expected empty id on error, got %q", id)
	}
}

// TestResolveSpaceID_NotFoundViaDirectCall covers the resolveSpaceID function
// directly when the API returns empty results (no alpha key match), expecting
// ExitNotFound to be returned.
func TestResolveSpaceID_NotFoundViaDirectCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "tok"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}
	id, code := cmd.ResolveSpaceID(context.Background(), c, "NOKEY")
	if code != cferrors.ExitNotFound {
		t.Fatalf("expected ExitNotFound, got %d", code)
	}
	if id != "" {
		t.Errorf("expected empty id, got %q", id)
	}
}

// ---------------------------------------------------------------------------
// search.go — runSearch direct (no client in context)
// ---------------------------------------------------------------------------

// TestRunSearch_NoClientInContext covers the branch where client.FromContext
// returns an error. The RunSearch export calls runSearch directly with a
// cobra command whose context has no client.
func TestRunSearch_NoClientInContext(t *testing.T) {
	cobraCmd := &cobra.Command{}
	cobraCmd.SetContext(context.Background())
	cobraCmd.Flags().String("cql", "type=page", "CQL query")

	err := cmd.RunSearch(cobraCmd, nil)
	if err == nil {
		t.Fatal("expected error when no client in context, got nil")
	}
}

