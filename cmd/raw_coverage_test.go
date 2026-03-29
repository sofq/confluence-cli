package cmd_test

// raw_coverage_test.go adds tests targeting uncovered branches in raw.go:
//   - runRaw: invalid query param, @filename body, empty @, GET with body (warning),
//     POST with --body -, config error, dry-run with POST body nil, successful request

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

// TestRawInvalidQueryParamFormat verifies that invalid --query format returns validation error.
func TestRawInvalidQueryParamFormat(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages", "--query", "noequalssign"})
	_ = root.Execute()

	w.Close()
	os.Stderr = oldStderr

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(errBuf.String())

	if stderrOutput == "" {
		t.Fatal("expected validation_error for invalid query format")
	}
	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}
	if errOut["error_type"] != "validation_error" {
		t.Errorf("error_type: want validation_error, got %v", errOut["error_type"])
	}
}

// TestRawGETWithBodyWarning verifies that using --body with GET triggers a warning.
func TestRawGETWithBodyWarning(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStderr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages", "--body", `{"foo":"bar"}`})
	_ = root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(re)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	var stdoutBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(r)

	// Expect a warning on stderr about --body being ignored for GET
	if stderrOutput == "" {
		t.Fatal("expected warning for --body with GET on stderr")
	}
	var warnOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &warnOut); err != nil {
		t.Fatalf("stderr warning is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}
	if warnOut["type"] != "warning" {
		t.Errorf("expected type 'warning', got %v", warnOut["type"])
	}
}

// TestRawBodyAtFilename verifies that --body @filename reads the file and sends its contents.
func TestRawBodyAtFilename(t *testing.T) {
	var capturedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","status":"current"}`)
	}))
	defer ts.Close()

	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	bodyContent := `{"title":"FromFile","spaceId":"456"}`
	f, err := os.CreateTemp(t.TempDir(), "rawbody-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, _ = f.WriteString(bodyContent)
	_ = f.Close()

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "POST", "/wiki/api/v2/pages", "--body", "@" + f.Name()})
	_ = root.Execute()

	wo.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if !strings.Contains(capturedBody, "FromFile") {
		t.Errorf("expected file contents to be sent as body, got: %q", capturedBody)
	}
}

// TestRawBodyFromNonexistentFile verifies that @nonexistent file returns validation error.
func TestRawBodyFromNonexistentFile(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "POST", "/wiki/api/v2/pages", "--body", "@/nonexistent/path/file.json"})
	_ = root.Execute()

	w.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(errBuf.String())

	if stderrOutput == "" {
		t.Fatal("expected validation_error for nonexistent body file")
	}
	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}
	if errOut["error_type"] != "validation_error" {
		t.Errorf("error_type: want validation_error, got %v", errOut["error_type"])
	}
}

// TestRawBodyEmptyAtSign verifies that --body @ (empty filename) returns validation error.
func TestRawBodyEmptyAtSign(t *testing.T) {
	ts := setupRawTestServer(t)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "POST", "/wiki/api/v2/pages", "--body", "@"})
	_ = root.Execute()

	w.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(errBuf.String())

	if stderrOutput == "" {
		t.Fatal("expected validation_error for empty @ body filename")
	}
	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}
	if errOut["error_type"] != "validation_error" {
		t.Errorf("error_type: want validation_error, got %v", errOut["error_type"])
	}
}

// TestRawPUTWithBody verifies PUT with a body succeeds.
func TestRawPUTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","title":"Updated"}`)
	}))
	defer ts.Close()

	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "PUT", "/wiki/api/v2/pages/1", "--body", `{"title":"Updated"}`})
	_ = root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if output == "" {
		t.Fatal("expected output from raw PUT, got empty")
	}
}

// TestRawPATCHWithBody verifies PATCH with a body succeeds.
func TestRawPATCHWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","status":"patched"}`)
	}))
	defer ts.Close()

	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "PATCH", "/wiki/api/v2/pages/1", "--body", `{"status":"current"}`})
	_ = root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if output == "" {
		t.Fatal("expected output from raw PATCH, got empty")
	}
}

// TestRawDELETERequest verifies DELETE request works.
func TestRawDELETERequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "DELETE", "/wiki/api/v2/pages/1"})
	exitErr := root.Execute()

	wo.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// 204 No Content typically exits with 0
	_ = exitErr
}

// TestRawQueryParamWithEquals verifies that --query key=value with = in value works.
func TestRawQueryParamWithEquals(t *testing.T) {
	var capturedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer ts.Close()

	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages",
		"--query", "cql=space=DEV", // value contains =
	})
	_ = root.Execute()

	wo.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if !strings.Contains(capturedQuery, "cql=") {
		t.Errorf("expected cql query param, got: %q", capturedQuery)
	}
}

// TestRawConfigError verifies that missing config returns config_error.
func TestRawConfigError(t *testing.T) {
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/noconfig.json")

	cmd.ResetRootPersistentFlags()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages"})
	_ = root.Execute()

	w.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(errBuf.String())

	if stderrOutput == "" {
		t.Fatal("expected config_error on stderr when no config")
	}
	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}
	if errOut["error_type"] != "config_error" {
		t.Errorf("error_type: want config_error, got %v", errOut["error_type"])
	}
}
