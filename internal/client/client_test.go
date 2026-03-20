package client_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
)

func newTestClient(ts *httptest.Server, stdout, stderr *bytes.Buffer) *client.Client {
	return &client.Client{
		BaseURL:    ts.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: ts.Client(),
		Stdout:     stdout,
		Stderr:     stderr,
	}
}

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
