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

func setupRawTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Echo back method and path as JSON
		fmt.Fprintf(w, `{"method":%q,"path":%q,"query":%q}`, r.Method, r.URL.Path, r.URL.RawQuery)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestRawInvalidMethodReturnsValidationError(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "FOO", "/wiki/api/v2/pages"})
	root.Execute()

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if stderrOutput == "" {
		t.Fatal("Expected error output for invalid method")
	}

	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}

	if errOut["error_type"] != "validation_error" {
		t.Errorf("error_type = %v, want validation_error", errOut["error_type"])
	}
}

func TestRawGETCallsServer(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages"})
	root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if output == "" {
		t.Fatal("Expected output from raw GET, got nothing")
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("raw GET output is not valid JSON: %v\nOutput: %s", err, output)
	}
}

func TestRawPOSTWithoutBodyReturnsValidationError(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "POST", "/wiki/api/v2/pages"})
	root.Execute()

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if stderrOutput == "" {
		t.Fatal("Expected error for POST without body")
	}

	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}

	if errOut["error_type"] != "validation_error" {
		t.Errorf("error_type = %v, want validation_error", errOut["error_type"])
	}
}

func TestRawGETWithQueryParams(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages", "--query", "limit=5"})
	root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if !strings.Contains(output, "limit") {
		t.Errorf("Expected query param 'limit' in output, got: %s", output)
	}
}
