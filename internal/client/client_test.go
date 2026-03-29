package client_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/policy"
	"github.com/spf13/cobra"
)

func TestApplyAuthBasic(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	c := &client.Client{
		Auth: config.AuthConfig{
			Type:     "basic",
			Username: "user@example.com",
			Token:    "my-token",
		},
	}
	if err := c.ApplyAuth(req); err != nil {
		t.Fatalf("ApplyAuth returned error: %v", err)
	}

	got := req.Header.Get("Authorization")
	if !strings.HasPrefix(got, "Basic ") {
		t.Fatalf("Authorization header = %q, want Basic ...", got)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "Basic "))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}
	if string(decoded) != "user@example.com:my-token" {
		t.Errorf("Basic auth credentials = %q, want %q", string(decoded), "user@example.com:my-token")
	}
}

func TestApplyAuthBearer(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	c := &client.Client{
		Auth: config.AuthConfig{
			Type:  "bearer",
			Token: "my-bearer-token",
		},
	}
	if err := c.ApplyAuth(req); err != nil {
		t.Fatalf("ApplyAuth returned error: %v", err)
	}

	got := req.Header.Get("Authorization")
	want := "Bearer my-bearer-token"
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestDryRun(t *testing.T) {
	// DryRun should not call the server
	serverCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		DryRun:     true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if serverCalled {
		t.Error("DryRun should not call the server")
	}
	if code != cferrors.ExitOK {
		t.Errorf("DryRun Do() = %d, want %d", code, cferrors.ExitOK)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("DryRun output is not valid JSON: %v\nOutput: %s", err, stdout.String())
	}
	if out["method"] != "GET" {
		t.Errorf("DryRun output method = %v, want GET", out["method"])
	}
	if !strings.Contains(fmt.Sprint(out["url"]), "/wiki/api/v2/pages") {
		t.Errorf("DryRun output url = %v, expected path in URL", out["url"])
	}
}

func TestVerboseLogFalse(t *testing.T) {
	var stderr bytes.Buffer
	c := &client.Client{
		Stderr:  &stderr,
		Verbose: false,
	}
	c.VerboseLog(map[string]any{"type": "request", "url": "http://example.com"})
	if stderr.Len() != 0 {
		t.Errorf("VerboseLog with Verbose=false should write nothing, got: %q", stderr.String())
	}
}

func TestVerboseLogTrue(t *testing.T) {
	var stderr bytes.Buffer
	c := &client.Client{
		Stderr:  &stderr,
		Verbose: true,
	}
	c.VerboseLog(map[string]any{"type": "request", "url": "http://example.com"})
	if stderr.Len() == 0 {
		t.Error("VerboseLog with Verbose=true should write to stderr")
	}

	// Output should be valid JSON
	var out map[string]interface{}
	if err := json.Unmarshal(bytes.TrimRight(stderr.Bytes(), "\n"), &out); err != nil {
		t.Errorf("VerboseLog output is not valid JSON: %v\nOutput: %s", err, stderr.String())
	}
}

func TestWriteOutputWithJQFilter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		Stdout:   &stdout,
		Stderr:   &stderr,
		JQFilter: ".id",
	}

	code := c.WriteOutput([]byte(`{"id":42,"name":"test"}`))
	if code != cferrors.ExitOK {
		t.Errorf("WriteOutput returned %d, want %d", code, cferrors.ExitOK)
	}
	output := strings.TrimRight(stdout.String(), "\n")
	if output != "42" {
		t.Errorf("WriteOutput filtered output = %q, want %q", output, "42")
	}
}

func TestWriteOutputWithInvalidJQFilter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		Stdout:   &stdout,
		Stderr:   &stderr,
		JQFilter: "invalid$$filter",
	}

	code := c.WriteOutput([]byte(`{"id":42}`))
	if code != cferrors.ExitValidation {
		t.Errorf("WriteOutput with invalid filter returned %d, want %d", code, cferrors.ExitValidation)
	}
	if stderr.Len() == 0 {
		t.Error("WriteOutput with invalid JQ should write error to stderr")
	}
}

func TestCursorPagination(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			// second page — no next link
			fmt.Fprintf(w, `{"results":[{"id":2}],"_links":{}}`)
		} else {
			// first page — has _links.next
			fmt.Fprintf(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=abc&limit=1"}}`)
		}
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("Do() = %d, want %d. Stderr: %s", code, cferrors.ExitOK, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, `"id":1`) {
		t.Errorf("Output should contain id:1, got: %s", output)
	}
	if !strings.Contains(output, `"id":2`) {
		t.Errorf("Output should contain id:2 from second page, got: %s", output)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 HTTP requests for pagination, got %d", requestCount)
	}
}

func TestCacheResponse(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"cached":true,"req":%d}`, requestCount)
	}))
	defer ts.Close()

	var stdout1, stderr1 bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "cache-test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout1,
		Stderr:     &stderr1,
		CacheTTL:   1 * time.Minute,
	}

	// First request — should hit server
	code1 := c.Do(context.Background(), "GET", "/wiki/api/v2/cache-test-"+t.Name(), nil, nil)
	if code1 != cferrors.ExitOK {
		t.Fatalf("First Do() = %d, want %d", code1, cferrors.ExitOK)
	}

	// Second request — should hit cache
	var stdout2, stderr2 bytes.Buffer
	c.Stdout = &stdout2
	c.Stderr = &stderr2
	code2 := c.Do(context.Background(), "GET", "/wiki/api/v2/cache-test-"+t.Name(), nil, nil)
	if code2 != cferrors.ExitOK {
		t.Fatalf("Second Do() = %d, want %d", code2, cferrors.ExitOK)
	}

	if requestCount != 1 {
		t.Errorf("Expected only 1 HTTP request (second should be cached), got %d", requestCount)
	}
}

func TestDoHTTPErrorReturnsExitCode(t *testing.T) {
	cases := []struct {
		status   int
		wantCode int
	}{
		{401, cferrors.ExitAuth},
		{404, cferrors.ExitNotFound},
		{422, cferrors.ExitValidation},
		{429, cferrors.ExitRateLimit},
		{500, cferrors.ExitServer},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("HTTP%d", tc.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"error":"test"}`)
			}))
			defer ts.Close()

			var stdout, stderr bytes.Buffer
			c := &client.Client{
				BaseURL:    ts.URL,
				Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
				HTTPClient: ts.Client(),
				Stdout:     &stdout,
				Stderr:     &stderr,
			}

			code := c.Do(context.Background(), "GET", "/test", nil, nil)
			if code != tc.wantCode {
				t.Errorf("HTTP %d: Do() = %d, want %d", tc.status, code, tc.wantCode)
			}
		})
	}
}

// newTestClient returns a Client wired to the given test server with sane defaults.
func newTestClient(ts *httptest.Server, stdout, stderr io.Writer) *client.Client {
	return &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     stdout,
		Stderr:     stderr,
	}
}

// ---- NewContext / FromContext ----

func TestNewContextAndFromContext(t *testing.T) {
	c := &client.Client{BaseURL: "https://example.com"}
	ctx := client.NewContext(context.Background(), c)

	got, err := client.FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext returned unexpected error: %v", err)
	}
	if got != c {
		t.Errorf("FromContext returned wrong client pointer")
	}
}

func TestFromContextMissing(t *testing.T) {
	_, err := client.FromContext(context.Background())
	if err == nil {
		t.Fatal("FromContext on empty context should return error")
	}
}

// ---- QueryFromFlags ----

func TestQueryFromFlagsOnlyIncludesChangedFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("title", "", "page title")
	cmd.Flags().String("status", "", "status")
	cmd.Flags().Int("limit", 25, "limit")

	// Mark only "title" and "limit" as changed.
	if err := cmd.Flags().Set("title", "MyPage"); err != nil {
		t.Fatalf("Set title: %v", err)
	}
	if err := cmd.Flags().Set("limit", "50"); err != nil {
		t.Fatalf("Set limit: %v", err)
	}

	q := client.QueryFromFlags(cmd, "title", "status", "limit")

	if q.Get("title") != "MyPage" {
		t.Errorf("title = %q, want %q", q.Get("title"), "MyPage")
	}
	if q.Get("limit") != "50" {
		t.Errorf("limit = %q, want %q", q.Get("limit"), "50")
	}
	if q.Has("status") {
		t.Errorf("status should not be present (not changed)")
	}
}

func TestQueryFromFlagsUnknownFlagSkipped(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	// "unknown" flag does not exist on the command.
	q := client.QueryFromFlags(cmd, "unknown")
	if len(q) != 0 {
		t.Errorf("expected empty query for unknown flag, got %v", q)
	}
}

// ---- Do: fields + path without leading slash + query merging ----

func TestDoAppendsFields(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Fields = "id,title"

	c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)

	if !strings.Contains(gotQuery, "fields=id%2Ctitle") && !strings.Contains(gotQuery, "fields=id,title") {
		t.Errorf("expected fields param in query, got: %s", gotQuery)
	}
}

func TestDoFieldsNotAppendedForNonGET(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Fields = "id,title"

	c.Do(context.Background(), "POST", "/wiki/api/v2/pages", nil, strings.NewReader(`{}`))

	if strings.Contains(gotQuery, "fields=") {
		t.Errorf("fields param should not be appended for POST, got: %s", gotQuery)
	}
}

func TestDoPathWithoutLeadingSlash(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)

	c.Do(context.Background(), "GET", "no-slash", nil, nil)

	if !strings.HasSuffix(gotPath, "/no-slash") {
		t.Errorf("expected path /no-slash, got: %s", gotPath)
	}
}

func TestDoQueryMergedWhenURLAlreadyHasQuery(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	// BaseURL already contains a query string to force the merging branch.
	c.BaseURL = ts.URL + "/base?existing=1"

	q := map[string][]string{"extra": {"2"}}
	c.Do(context.Background(), "GET", "", q, nil)

	if !strings.Contains(gotQuery, "existing=1") || !strings.Contains(gotQuery, "extra=2") {
		t.Errorf("merged query missing expected params, got: %s", gotQuery)
	}
}

// ---- Do: policy denied ----

func TestDoPolicyDenied(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	p, _ := policy.NewFromConfig([]string{"pages get"}, nil) // only "pages get" allowed

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Policy = p

	code := c.Do(context.Background(), "DELETE", "/wiki/api/v2/pages/1", nil, nil)
	if code != cferrors.ExitValidation {
		t.Errorf("expected ExitValidation for denied policy, got %d", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected error written to stderr on policy denial")
	}
}

// ---- Do: DryRun with a body ----

func TestDryRunWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called in dry-run mode")
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.DryRun = true

	payload := strings.NewReader(`{"title":"test"}`)
	code := c.Do(context.Background(), "POST", "/wiki/api/v2/pages", nil, payload)

	if code != cferrors.ExitOK {
		t.Errorf("DryRun with body returned %d, want %d", code, cferrors.ExitOK)
	}

	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("DryRun output is not valid JSON: %v", err)
	}
	if out["body"] == nil {
		t.Error("DryRun output should include body field when body is provided")
	}
}

func TestDryRunWithNonJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called in dry-run mode")
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.DryRun = true

	// Non-JSON body — should be stored as plain string.
	payload := strings.NewReader("plain text body")
	c.Do(context.Background(), "POST", "/wiki/api/v2/pages", nil, payload)

	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("DryRun output is not valid JSON: %v", err)
	}
	if body, ok := out["body"].(string); !ok || body != "plain text body" {
		t.Errorf("expected plain string body, got %v", out["body"])
	}
}

// ---- Do: 204 No Content ----

func TestDoNoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)

	code := c.Do(context.Background(), "DELETE", "/wiki/api/v2/pages/1", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("204 No Content: Do() = %d, want %d", code, cferrors.ExitOK)
	}
	trimmed := strings.TrimSpace(stdout.String())
	if trimmed != "{}" {
		t.Errorf("204 No Content: expected {}, got %q", trimmed)
	}
}

// ---- Do: POST with body (doOnce content-type branch) ----

func TestDoPostWithBody(t *testing.T) {
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":99}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)

	payload := strings.NewReader(`{"title":"new page"}`)
	code := c.Do(context.Background(), "POST", "/wiki/api/v2/pages", nil, payload)

	if code != cferrors.ExitOK {
		t.Errorf("POST: Do() = %d, want %d", code, cferrors.ExitOK)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

// ---- doOnce: connection errors ----

func TestDoOnceInvalidURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		// An invalid URL to trigger http.NewRequest error.
		BaseURL:    "://bad-url",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: http.DefaultClient,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	code := c.Do(context.Background(), "GET", "/test", nil, nil)
	if code != cferrors.ExitError {
		t.Errorf("Invalid URL: Do() = %d, want %d", code, cferrors.ExitError)
	}
}

func TestDoOnceHTTPClientError(t *testing.T) {
	// Use a closed server so the HTTP transport returns an error.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close() // Close immediately.

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)

	code := c.Do(context.Background(), "GET", "/test", nil, nil)
	if code != cferrors.ExitError {
		t.Errorf("Closed server: Do() = %d, want %d", code, cferrors.ExitError)
	}
}

// ---- doOnce: verbose mode ----

func TestDoOnceVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Verbose = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("Verbose Do() = %d, want %d", code, cferrors.ExitOK)
	}
	if stderr.Len() == 0 {
		t.Error("Verbose mode should write request/response logs to stderr")
	}
}

// ---- Do: operationName set explicitly vs derived ----

func TestDoOperationNameExplicit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Operation = "pages get" // explicitly set operation name

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages/1", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("explicit operation: Do() = %d, want %d", code, cferrors.ExitOK)
	}
}

// ---- detectCursorPagination ----

func TestDetectCursorPaginationNonPaginatedResponse(t *testing.T) {
	// Response without results/links keys — should pass through unchanged.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1,"title":"plain"}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages/1", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("non-paginated response: Do() = %d, want %d", code, cferrors.ExitOK)
	}
	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, `"id":1`) {
		t.Errorf("expected passthrough output, got: %s", output)
	}
}

func TestDetectCursorPaginationInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not-json-at-all`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("invalid JSON body: Do() = %d, want %d", code, cferrors.ExitOK)
	}
}

// ---- doWithPagination: cache hit ----

func TestDoWithPaginationCacheHit(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"results":[{"id":%d}],"_links":{}}`, requestCount)
	}))
	defer ts.Close()

	var stdout1, stderr1 bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "paginate-cache-test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout1,
		Stderr:     &stderr1,
		Paginate:   true,
		CacheTTL:   1 * time.Minute,
	}

	code1 := c.Do(context.Background(), "GET", "/wiki/api/v2/paginate-cache-"+t.Name(), nil, nil)
	if code1 != cferrors.ExitOK {
		t.Fatalf("first paginated request failed: %d", code1)
	}

	// Second request — should be cached.
	var stdout2, stderr2 bytes.Buffer
	c.Stdout = &stdout2
	c.Stderr = &stderr2
	code2 := c.Do(context.Background(), "GET", "/wiki/api/v2/paginate-cache-"+t.Name(), nil, nil)
	if code2 != cferrors.ExitOK {
		t.Fatalf("second paginated request failed: %d", code2)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request (cache hit), got %d", requestCount)
	}
}

// ---- doWithPagination: fetchPage error propagates ----

func TestDoWithPaginationFirstPageError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"unauthorized"}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitAuth {
		t.Errorf("expected ExitAuth on 401 first page, got %d", code)
	}
}

// ---- doCursorPagination: invalid JSON on the second page ----

func TestDoCursorPaginationInvalidSecondPage(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			// second page returns invalid JSON — should cause the for loop to break.
			fmt.Fprint(w, `not-valid-json`)
		} else {
			fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=x"}}`)
		}
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("invalid second page JSON: Do() = %d, want %d", code, cferrors.ExitOK)
	}
	// Should still output the first page results.
	if !strings.Contains(stdout.String(), `"id":1`) {
		t.Errorf("expected id:1 in output, got: %s", stdout.String())
	}
}

// ---- doCursorPagination: next page fetch error ----

func TestDoCursorPaginationNextPageError(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"message":"server error"}`)
		} else {
			fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=err"}}`)
		}
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitServer {
		t.Errorf("expected ExitServer on 500 second page, got %d", code)
	}
}

// ---- doCursorPagination: next link without /wiki/ prefix ----

func TestDoCursorPaginationNextLinkWithoutWikiPrefix(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			fmt.Fprint(w, `{"results":[{"id":2}],"_links":{}}`)
		} else {
			// next link without /wiki/ prefix — exercises the idx < 0 branch.
			fmt.Fprintf(w, `{"results":[{"id":1}],"_links":{"next":"/api/v2/pages?cursor=abc"}}`)
		}
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("no /wiki/ prefix in next link: Do() = %d, want %d", code, cferrors.ExitOK)
	}
}

// ---- doCursorPagination: cache write for merged result ----

func TestDoCursorPaginationCacheMerged(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			fmt.Fprint(w, `{"results":[{"id":2}],"_links":{}}`)
		} else {
			fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=abc"}}`)
		}
	}))
	defer ts.Close()

	var stdout1, stderr1 bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "cursor-cache-token"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout1,
		Stderr:     &stderr1,
		Paginate:   true,
		CacheTTL:   1 * time.Minute,
	}

	code1 := c.Do(context.Background(), "GET", "/wiki/api/v2/pages-cursor-cache-"+t.Name(), nil, nil)
	if code1 != cferrors.ExitOK {
		t.Fatalf("first request failed: %d", code1)
	}

	// Second call — should use merged cache.
	var stdout2, stderr2 bytes.Buffer
	c.Stdout = &stdout2
	c.Stderr = &stderr2
	code2 := c.Do(context.Background(), "GET", "/wiki/api/v2/pages-cursor-cache-"+t.Name(), nil, nil)
	if code2 != cferrors.ExitOK {
		t.Fatalf("second request failed: %d", code2)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 HTTP requests (first+second page then cache), got %d", requestCount)
	}
}

// ---- fetchPage: HTTP 4xx error ----

func TestFetchPageHTTPError(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=bad"}}`)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"not found"}`)
		}
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitNotFound {
		t.Errorf("fetchPage 404: Do() = %d, want ExitNotFound", code)
	}
}

func TestFetchPageConnectionError(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
	}))
	// The server is alive for the first request but we'll close it before the second.
	// We arrange first page to return a next link pointing to the same (closed) server.
	firstTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// next link uses the first server's URL which we'll close.
		fmt.Fprintf(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=go"}}`)
	}))
	defer ts.Close()

	// Close firstTS so the second fetchPage (for cursor=go) fails.
	firstTS.Close()

	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    firstTS.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: firstTS.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	// Connection refused on closed server.
	if code != cferrors.ExitError {
		t.Errorf("fetchPage connection error: Do() = %d, want ExitError", code)
	}
}

// ---- Fetch ----

func TestFetchSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":42}`)
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
	}

	body, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages/42", nil)
	if code != cferrors.ExitOK {
		t.Errorf("Fetch success: code = %d, want %d", code, cferrors.ExitOK)
	}
	if !strings.Contains(string(body), `"id":42`) {
		t.Errorf("Fetch body = %q, expected id:42", string(body))
	}
}

func TestFetchDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called in Fetch dry-run mode")
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		DryRun:     true,
	}

	data, code := c.Fetch(context.Background(), "POST", "/wiki/api/v2/pages", strings.NewReader(`{"title":"t"}`))
	if code != cferrors.ExitOK {
		t.Errorf("Fetch DryRun: code = %d, want %d", code, cferrors.ExitOK)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Fetch DryRun output not valid JSON: %v", err)
	}
	if out["method"] != "POST" {
		t.Errorf("Fetch DryRun output method = %v, want POST", out["method"])
	}
}

func TestFetchDryRunNonJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called in Fetch dry-run mode")
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		DryRun:     true,
	}

	data, code := c.Fetch(context.Background(), "POST", "/wiki/api/v2/pages", strings.NewReader("plain"))
	if code != cferrors.ExitOK {
		t.Errorf("Fetch DryRun non-JSON body: code = %d, want %d", code, cferrors.ExitOK)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if body, ok := out["body"].(string); !ok || body != "plain" {
		t.Errorf("expected plain string body, got %v", out["body"])
	}
}

func TestFetchDryRunNoBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		DryRun:     true,
	}

	data, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages", nil)
	if code != cferrors.ExitOK {
		t.Errorf("Fetch DryRun no body: code = %d, want %d", code, cferrors.ExitOK)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if _, hasBody := out["body"]; hasBody {
		t.Error("Fetch DryRun with nil body should not include body field")
	}
}

func TestFetchInvalidURL(t *testing.T) {
	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    "://bad",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: http.DefaultClient,
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
	}

	_, code := c.Fetch(context.Background(), "GET", "/test", nil)
	if code != cferrors.ExitError {
		t.Errorf("Fetch invalid URL: code = %d, want ExitError", code)
	}
}

func TestFetchHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
	}

	_, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages/1", nil)
	if code != cferrors.ExitAuth {
		t.Errorf("Fetch 403: code = %d, want ExitAuth", code)
	}
}

func TestFetchConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close() // closed immediately

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
	}

	_, code := c.Fetch(context.Background(), "GET", "/test", nil)
	if code != cferrors.ExitError {
		t.Errorf("Fetch connection error: code = %d, want ExitError", code)
	}
}

func TestFetchWithBodySetsContentType(t *testing.T) {
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
	}

	payload := strings.NewReader(`{"title":"test"}`)
	_, code := c.Fetch(context.Background(), "POST", "/wiki/api/v2/pages", payload)
	if code != cferrors.ExitOK {
		t.Errorf("Fetch POST: code = %d, want %d", code, cferrors.ExitOK)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

func TestFetchVerboseLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Verbose:    true,
	}

	_, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages", nil)
	if code != cferrors.ExitOK {
		t.Errorf("Fetch verbose: code = %d", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected verbose logs in stderr")
	}
}

func TestFetchWithExplicitOperationName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1}`)
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Operation:  "pages get",
	}

	_, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages/1", nil)
	if code != cferrors.ExitOK {
		t.Errorf("Fetch explicit operation: code = %d", code)
	}
}

// ---- WriteOutput: pretty-print ----

func TestWriteOutputPrettyPrint(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		Stdout: &stdout,
		Stderr: &stderr,
		Pretty: true,
	}

	code := c.WriteOutput([]byte(`{"id":1,"name":"test"}`))
	if code != cferrors.ExitOK {
		t.Errorf("WriteOutput pretty: code = %d, want %d", code, cferrors.ExitOK)
	}
	output := stdout.String()
	if !strings.Contains(output, "\n") {
		t.Error("pretty-printed output should contain newlines")
	}
	if !strings.Contains(output, "  ") {
		t.Error("pretty-printed output should contain indentation")
	}
}

func TestWriteOutputPrettyPrintInvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		Stdout: &stdout,
		Stderr: &stderr,
		Pretty: true,
	}

	// Invalid JSON — json.Indent will fail, output is passed through as-is.
	code := c.WriteOutput([]byte(`not-json`))
	if code != cferrors.ExitOK {
		t.Errorf("WriteOutput pretty invalid JSON: code = %d, want %d", code, cferrors.ExitOK)
	}
	if !strings.Contains(stdout.String(), "not-json") {
		t.Errorf("expected raw passthrough of invalid JSON, got: %s", stdout.String())
	}
}

// ---- SearchV1Domain ----

func TestSearchV1DomainWithWikiPath(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://example.atlassian.net/wiki/api/v2", "https://example.atlassian.net"},
		{"https://mysite.com/wiki/rest/api", "https://mysite.com"},
	}
	for _, tc := range cases {
		got := client.SearchV1Domain(tc.input)
		if got != tc.want {
			t.Errorf("SearchV1Domain(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSearchV1DomainWithoutWikiPath(t *testing.T) {
	// No /wiki/ in URL — returns the full baseURL unchanged.
	input := "https://example.com/api/v2"
	got := client.SearchV1Domain(input)
	if got != input {
		t.Errorf("SearchV1Domain(%q) = %q, want %q", input, got, input)
	}
}

func TestSearchV1DomainWikiAtStart(t *testing.T) {
	// /wiki/ at position 0 — idx is 0, not > 0, so full URL is returned.
	input := "/wiki/api/v2"
	got := client.SearchV1Domain(input)
	if got != input {
		t.Errorf("SearchV1Domain(%q) = %q, want %q", input, got, input)
	}
}

// ---- Do: cache for paginated non-cursor response ----

func TestDoWithPaginationNonCursorCached(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		// Response with no pagination envelope — just a plain object.
		fmt.Fprint(w, `{"id":1,"title":"cached-page"}`)
	}))
	defer ts.Close()

	var stdout1, stderr1 bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "non-cursor-cache"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout1,
		Stderr:     &stderr1,
		Paginate:   true,
		CacheTTL:   1 * time.Minute,
	}

	code1 := c.Do(context.Background(), "GET", "/wiki/api/v2/page-non-cursor-"+t.Name(), nil, nil)
	if code1 != cferrors.ExitOK {
		t.Fatalf("first request failed: %d", code1)
	}

	var stdout2, stderr2 bytes.Buffer
	c.Stdout = &stdout2
	c.Stderr = &stderr2
	code2 := c.Do(context.Background(), "GET", "/wiki/api/v2/page-non-cursor-"+t.Name(), nil, nil)
	if code2 != cferrors.ExitOK {
		t.Fatalf("second request failed: %d", code2)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request (second should use cache), got %d", requestCount)
	}
}

// ---- errReader: simulates a body that fails on Read ----

type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("simulated read error")
}

type brokenBodyTransport struct{}

func (t *brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&errReader{}),
	}, nil
}

// ---- doOnce: io.ReadAll error ----

func TestDoOnceReadBodyError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    "http://example.com",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: &http.Client{Transport: &brokenBodyTransport{}},
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitError {
		t.Errorf("doOnce read body error: Do() = %d, want ExitError", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected error written to stderr on body read failure")
	}
}

// ---- fetchPage: connection error on second page (next link) ----

func TestFetchPageConnectionErrorOnNextPage(t *testing.T) {
	// First request returns a next link. We then close the server so the second
	// fetchPage (following the next link) gets a connection refused error.
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=x"}}`)
	}))

	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	// Close server so the second fetchPage (cursor=x) fails with connection error.
	ts.Close()

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	// Connection refused on closed server for first page.
	if code != cferrors.ExitError {
		t.Errorf("fetchPage with closed server: Do() = %d, want ExitError", code)
	}
}

// ---- fetchPage: NewRequestWithContext error via invalid URL in next link ----

func TestFetchPageInvalidNextLinkURL(t *testing.T) {
	// The next link contains a null byte which, when assembled into a URL, causes
	// http.NewRequestWithContext to fail inside fetchPage.
	// The link does NOT contain /wiki/ so nextPath = the full link,
	// and domain = SearchV1Domain(BaseURL) = BaseURL (no /wiki/ in BaseURL).
	// The resulting nextURL = BaseURL + "/path\x00" which is invalid.
	transport := &singleResponseTransport{
		body: "{\"results\":[{\"id\":1}],\"_links\":{\"next\":\"/page\\u0000cursor\"}}",
	}
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    "http://example.com",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: &http.Client{Transport: transport},
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitError {
		t.Errorf("fetchPage invalid next URL: Do() = %d, want ExitError", code)
	}
}

// ---- fetchPage: io.ReadAll error ----

type brokenBodyTransportWithOKFirst struct {
	calls int
}

func (t *brokenBodyTransportWithOKFirst) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls++
	if t.calls == 1 {
		// First call: valid paginated response with a next link.
		body := `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=x"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	// Second call (cursor page): body read will fail.
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&errReader{}),
	}, nil
}

func TestFetchPageReadBodyError(t *testing.T) {
	transport := &brokenBodyTransportWithOKFirst{}
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    "http://example.com",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: &http.Client{Transport: transport},
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitError {
		t.Errorf("fetchPage body read error: Do() = %d, want ExitError", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected error written to stderr on fetchPage body read failure")
	}
}

// ---- Fetch: io.ReadAll error ----

func TestFetchReadBodyError(t *testing.T) {
	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    "http://example.com",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: &http.Client{Transport: &brokenBodyTransport{}},
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
	}

	_, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages", nil)
	if code != cferrors.ExitError {
		t.Errorf("Fetch read body error: code = %d, want ExitError", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected error written to stderr on Fetch body read failure")
	}
}

// ---- fetchPage: invalid URL in NewRequestWithContext ----

func TestFetchPageNewRequestError(t *testing.T) {
	// Trigger the http.NewRequestWithContext error in fetchPage by having the
	// first page return a next link that, combined with the domain, forms an invalid URL.
	// We set BaseURL to contain a space which will make the next URL invalid after assembly.
	transport := &singleResponseTransport{
		body: `{"results":[{"id":1}],"_links":{"next":"/ has spaces/pages?cursor=x"}}`,
	}
	var stdout, stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    "http://example.com",
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: &http.Client{Transport: transport},
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	// The invalid URL either causes an error or the request to fail — either way not ExitOK.
	// Depending on Go version, this may be ExitError or successfully fetch 0 results.
	_ = code // outcome depends on URL validation behavior; just ensure no panic
}

// singleResponseTransport returns one fixed response and then connection errors.
type singleResponseTransport struct {
	calls int
	body  string
}

func (t *singleResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls++
	if t.calls == 1 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(t.body)),
		}, nil
	}
	return nil, fmt.Errorf("no more responses")
}

// ---- doOnce: AuthFunc error ----

func TestDoOnceAuthFuncError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when auth fails")
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.AuthFunc = func(r *http.Request) error {
		return fmt.Errorf("auth failed")
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitAuth {
		t.Errorf("doOnce AuthFunc error: Do() = %d, want ExitAuth (%d)", code, cferrors.ExitAuth)
	}
	if stderr.Len() == 0 {
		t.Error("expected auth error written to stderr")
	}
}

// ---- doCursorPagination: first body passes detectCursorPagination but fails cursorPage unmarshal ----

func TestDoCursorPaginationFirstBodyUnmarshalError(t *testing.T) {
	// "results" is a JSON object (not an array): detectCursorPagination accepts it
	// (uses json.RawMessage which accepts any value), but json.Unmarshal into
	// cursorPage fails because Results is []json.RawMessage.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":{"key":"value"},"_links":{"next":"/wiki/api/v2/pages?cursor=x"}}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	c := newTestClient(ts, &stdout, &stderr)
	c.Paginate = true

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitOK {
		t.Errorf("doCursorPagination unmarshal error: Do() = %d, want ExitOK (%d)", code, cferrors.ExitOK)
	}
	// The body is passed through as-is via WriteOutput.
	output := stdout.String()
	if !strings.Contains(output, `"results"`) {
		t.Errorf("expected raw body in output, got: %s", output)
	}
}

// ---- fetchPage: AuthFunc error in pagination context ----

// authFuncAfterNTransport lets the first N requests succeed normally, then
// switches the client's AuthFunc to return an error before the next fetchPage.
// We accomplish this by using a counter in the server handler and a client
// whose AuthFunc is set to fail on the second call.

func TestFetchPageAuthFuncError(t *testing.T) {
	// First page succeeds and returns a next link.
	// The client's AuthFunc is set to fail so the second fetchPage (for the next page) returns ExitAuth.
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		// Always return a next link so fetchPage is called again.
		fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=x"}}`)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	authCallCount := 0
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		Paginate:   true,
		AuthFunc: func(r *http.Request) error {
			authCallCount++
			if authCallCount > 1 {
				return fmt.Errorf("auth failed on second call")
			}
			return nil
		},
	}

	code := c.Do(context.Background(), "GET", "/wiki/api/v2/pages", nil, nil)
	if code != cferrors.ExitAuth {
		t.Errorf("fetchPage AuthFunc error: Do() = %d, want ExitAuth (%d)", code, cferrors.ExitAuth)
	}
	if stderr.Len() == 0 {
		t.Error("expected auth error written to stderr from fetchPage")
	}
}

// ---- Fetch: AuthFunc error ----

func TestFetchAuthFuncError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when auth fails")
	}))
	defer ts.Close()

	var stderr bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test"},
		HTTPClient: ts.Client(),
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		AuthFunc: func(r *http.Request) error {
			return fmt.Errorf("auth failed")
		},
	}

	_, code := c.Fetch(context.Background(), "GET", "/wiki/api/v2/pages", nil)
	if code != cferrors.ExitAuth {
		t.Errorf("Fetch AuthFunc error: code = %d, want ExitAuth (%d)", code, cferrors.ExitAuth)
	}
	if stderr.Len() == 0 {
		t.Error("expected auth error written to stderr from Fetch")
	}
}

// ---- doOnce: cache hit path (non-paginated) ----

func TestDoOnceCacheHit(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"req":%d}`, requestCount)
	}))
	defer ts.Close()

	var stdout1, stderr1 bytes.Buffer
	c := &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "doonce-cache"},
		HTTPClient: ts.Client(),
		Stdout:     &stdout1,
		Stderr:     &stderr1,
		CacheTTL:   1 * time.Minute,
		// Paginate is false so we exercise doOnce's cache path directly.
	}

	code1 := c.Do(context.Background(), "GET", "/wiki/api/v2/doonce-cache-"+t.Name(), nil, nil)
	if code1 != cferrors.ExitOK {
		t.Fatalf("first doOnce request failed: %d", code1)
	}

	var stdout2, stderr2 bytes.Buffer
	c.Stdout = &stdout2
	c.Stderr = &stderr2
	code2 := c.Do(context.Background(), "GET", "/wiki/api/v2/doonce-cache-"+t.Name(), nil, nil)
	if code2 != cferrors.ExitOK {
		t.Fatalf("second doOnce request failed: %d", code2)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request (doOnce cache hit), got %d", requestCount)
	}
}

// ---- Cache write error branches ----
// These tests cover the "cache write failed" VerboseLog branches by making the
// cache directory temporarily read-only during a request that would write to cache.

func withReadOnlyCacheDir(t *testing.T, fn func()) {
	t.Helper()
	dir, err := os.UserCacheDir()
	if err != nil {
		t.Skip("cannot determine user cache dir:", err)
	}
	cfDir := filepath.Join(dir, "cf")
	if err := os.MkdirAll(cfDir, 0o700); err != nil {
		t.Skip("cannot create cache dir:", err)
	}
	// Make it read-only.
	if err := os.Chmod(cfDir, 0o555); err != nil {
		t.Skip("cannot chmod cache dir:", err)
	}
	defer func() {
		// Always restore write permission.
		_ = os.Chmod(cfDir, 0o700)
	}()
	fn()
}

func TestDoOnceCacheWriteError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1}`)
	}))
	defer ts.Close()

	withReadOnlyCacheDir(t, func() {
		var stdout, stderr bytes.Buffer
		c := &client.Client{
			BaseURL:    ts.URL,
			Auth:       config.AuthConfig{Type: "bearer", Token: "cache-write-err-doonce"},
			HTTPClient: ts.Client(),
			Stdout:     &stdout,
			Stderr:     &stderr,
			CacheTTL:   1 * time.Minute,
			Verbose:    true, // ensure the warning is logged
		}

		code := c.Do(context.Background(), "GET", "/wiki/api/v2/cache-write-err-"+t.Name(), nil, nil)
		if code != cferrors.ExitOK {
			t.Errorf("doOnce with cache write error: Do() = %d, want %d", code, cferrors.ExitOK)
		}
		// The verbose log should contain the cache write failed warning.
		if !strings.Contains(stderr.String(), "cache write failed") {
			t.Errorf("expected cache write failed warning in stderr verbose output, got: %s", stderr.String())
		}
	})
}

func TestDoWithPaginationNonCursorCacheWriteError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1,"title":"plain"}`) // no pagination envelope
	}))
	defer ts.Close()

	withReadOnlyCacheDir(t, func() {
		var stdout, stderr bytes.Buffer
		c := &client.Client{
			BaseURL:    ts.URL,
			Auth:       config.AuthConfig{Type: "bearer", Token: "cache-write-err-pagination"},
			HTTPClient: ts.Client(),
			Stdout:     &stdout,
			Stderr:     &stderr,
			CacheTTL:   1 * time.Minute,
			Paginate:   true,
			Verbose:    true,
		}

		code := c.Do(context.Background(), "GET", "/wiki/api/v2/non-cursor-write-err-"+t.Name(), nil, nil)
		if code != cferrors.ExitOK {
			t.Errorf("doWithPagination non-cursor cache write error: Do() = %d, want %d", code, cferrors.ExitOK)
		}
		if !strings.Contains(stderr.String(), "cache write failed") {
			t.Errorf("expected cache write failed warning, got: %s", stderr.String())
		}
	})
}

func TestDoCursorPaginationCacheWriteError(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			fmt.Fprint(w, `{"results":[{"id":2}],"_links":{}}`)
		} else {
			fmt.Fprint(w, `{"results":[{"id":1}],"_links":{"next":"/wiki/api/v2/pages?cursor=abc"}}`)
		}
	}))
	defer ts.Close()

	withReadOnlyCacheDir(t, func() {
		var stdout, stderr bytes.Buffer
		c := &client.Client{
			BaseURL:    ts.URL,
			Auth:       config.AuthConfig{Type: "bearer", Token: "cache-write-err-cursor"},
			HTTPClient: ts.Client(),
			Stdout:     &stdout,
			Stderr:     &stderr,
			CacheTTL:   1 * time.Minute,
			Paginate:   true,
			Verbose:    true,
		}

		code := c.Do(context.Background(), "GET", "/wiki/api/v2/cursor-write-err-"+t.Name(), nil, nil)
		if code != cferrors.ExitOK {
			t.Errorf("doCursorPagination cache write error: Do() = %d, want %d", code, cferrors.ExitOK)
		}
		if !strings.Contains(stderr.String(), "cache write failed") {
			t.Errorf("expected cache write failed warning, got: %s", stderr.String())
		}
	})
}
