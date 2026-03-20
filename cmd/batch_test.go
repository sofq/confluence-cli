package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
	"github.com/sofq/confluence-cli/internal/policy"
)

// writeTempBatchInput writes ops JSON to a temp file and returns the path.
func writeTempBatchInput(t *testing.T, ops any) string {
	t.Helper()
	data, err := json.Marshal(ops)
	if err != nil {
		t.Fatalf("marshal batch ops: %v", err)
	}
	f, err := os.CreateTemp(t.TempDir(), "batch-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return f.Name()
}

// TestBatch_ValidSingleOp verifies a single-op batch against a fake server.
func TestBatch_ValidSingleOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/pages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[{"id":"1","title":"Test Page"}],"_links":{}}`)
	}))
	defer srv.Close()

	ops := []map[string]any{
		{"command": "pages get", "args": map[string]string{}},
	}
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW
	oldErr := os.Stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--dry-run=false"})
	root.Execute()

	outW.Close()
	errW.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	var errBuf bytes.Buffer
	errBuf.ReadFrom(errR)

	output := strings.TrimSpace(outBuf.String())
	if output == "" {
		t.Fatalf("expected JSON output, got empty; stderr: %s", errBuf.String())
	}

	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var exitCode int
	if err := json.Unmarshal(results[0]["exit_code"], &exitCode); err != nil {
		t.Fatalf("parse exit_code: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit_code 0, got %d; result: %s", exitCode, output)
	}
	if results[0]["data"] == nil {
		t.Error("expected data field, got nil")
	}
}

// TestBatch_PartialFailure verifies that op[0] succeeds, op[1] fails with 404,
// and the top-level exit code is the max of all per-op exit codes.
func TestBatch_PartialFailure(t *testing.T) {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			// First request: succeed (pages get).
			fmt.Fprint(w, `{"results":[],"_links":{}}`)
		} else {
			// Second request: 404.
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error_type":"not_found","message":"not found"}`)
		}
	}))
	defer srv.Close()

	ops := []map[string]any{
		{"command": "pages get", "args": map[string]string{}},
		{"command": "pages get-by-id", "args": map[string]string{"id": "nonexistent-999"}},
	}
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW
	oldErr := os.Stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--dry-run=false"})
	root.Execute()

	outW.Close()
	errW.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	var errBuf bytes.Buffer
	errBuf.ReadFrom(errR)
	_ = errBuf

	output := strings.TrimSpace(outBuf.String())
	if output == "" {
		t.Fatalf("expected JSON output even with partial failure")
	}

	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var code0, code1 int
	json.Unmarshal(results[0]["exit_code"], &code0)
	json.Unmarshal(results[1]["exit_code"], &code1)

	if code0 != 0 {
		t.Errorf("op[0] exit_code: want 0, got %d", code0)
	}
	// 404 → ExitNotFound = 3
	if code1 != 3 {
		t.Errorf("op[1] exit_code: want 3 (not_found), got %d; result: %s", code1, output)
	}
}

// TestBatch_UnknownCommand verifies that an unknown command returns exit_code:1 per-op.
func TestBatch_UnknownCommand(t *testing.T) {
	ops := []map[string]any{
		{"command": "nonexistent foo", "args": map[string]string{}},
	}
	inputFile := writeTempBatchInput(t, ops)

	// Use a fake server even though no request should be made.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request for unknown command")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--dry-run=false"})
	root.Execute()

	outW.Close()
	os.Stdout = oldOut

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	output := strings.TrimSpace(outBuf.String())

	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	var exitCode int
	json.Unmarshal(results[0]["exit_code"], &exitCode)
	if exitCode != 1 {
		t.Errorf("unknown command: exit_code want 1, got %d", exitCode)
	}
	if results[0]["error"] == nil {
		t.Error("unknown command: expected error field")
	}
}

// TestBatch_InvalidJSON verifies that invalid JSON input exits code 4 with stderr error.
func TestBatch_InvalidJSON(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "batch-invalid-*.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer f.Close()
	f.WriteString("this is not json")
	inputFile := f.Name()

	t.Setenv("CF_BASE_URL", "http://localhost:9")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldErr := os.Stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW
	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--dry-run=false"})
	root.Execute()

	errW.Close()
	outW.Close()
	os.Stderr = oldErr
	os.Stdout = oldOut

	var errBuf, outBuf bytes.Buffer
	errBuf.ReadFrom(errR)
	outBuf.ReadFrom(outR)

	stderrOut := strings.TrimSpace(errBuf.String())
	if stderrOut == "" {
		t.Fatal("expected error on stderr for invalid JSON")
	}
	var errJSON map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOut), &errJSON); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOut)
	}
	if errJSON["error_type"] != "validation_error" {
		t.Errorf("error_type: want validation_error, got %v", errJSON["error_type"])
	}
	// stdout should be empty (no array written)
	if strings.TrimSpace(outBuf.String()) != "" {
		t.Errorf("expected empty stdout for invalid JSON, got: %s", outBuf.String())
	}
}

// TestBatch_MissingRequiredPathParam verifies that a missing required path param
// returns exit_code:4 per-op (pages get-by-id without "id").
func TestBatch_MissingRequiredPathParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request — missing param should be caught before HTTP")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ops := []map[string]any{
		{"command": "pages get-by-id", "args": map[string]string{}}, // no "id"
	}
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--dry-run=false"})
	root.Execute()

	outW.Close()
	os.Stdout = oldOut

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	output := strings.TrimSpace(outBuf.String())

	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nOutput: %s", err, output)
	}
	var exitCode int
	json.Unmarshal(results[0]["exit_code"], &exitCode)
	// ExitValidation = 4
	if exitCode != 4 {
		t.Errorf("missing path param: exit_code want 4, got %d; output: %s", exitCode, output)
	}
}

// TestBatch_MaxBatchExceeded verifies that exceeding --max-batch exits with code 4.
func TestBatch_MaxBatchExceeded(t *testing.T) {
	// Build 3 ops but set max-batch=2
	ops := make([]map[string]any, 3)
	for i := range ops {
		ops[i] = map[string]any{
			"command": "pages get",
			"args":    map[string]string{},
		}
	}
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", "http://localhost:9")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldErr := os.Stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW
	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--max-batch", "2", "--dry-run=false"})
	root.Execute()

	errW.Close()
	outW.Close()
	os.Stderr = oldErr
	os.Stdout = oldOut

	var errBuf bytes.Buffer
	errBuf.ReadFrom(errR)
	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)

	stderrOut := strings.TrimSpace(errBuf.String())
	if stderrOut == "" {
		t.Fatal("expected validation_error on stderr for max-batch exceeded")
	}
	var errJSON map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOut), &errJSON); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOut)
	}
	if errJSON["error_type"] != "validation_error" {
		t.Errorf("error_type: want validation_error, got %v", errJSON["error_type"])
	}
}

// TestBatch_EmptyArray verifies that an empty ops array `[]` returns exit code 0 and outputs `[]`.
func TestBatch_EmptyArray(t *testing.T) {
	ops := []map[string]any{} // empty array
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", "http://localhost:9")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--dry-run=false"})
	err := root.Execute()

	outW.Close()
	os.Stdout = oldOut

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	output := strings.TrimSpace(outBuf.String())

	if err != nil {
		t.Errorf("expected nil error for empty ops, got %v", err)
	}

	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestBatch_PolicyDeny verifies that a batch op denied by policy returns exit_code:4
// and makes no HTTP requests to the server.
// This test bypasses the CLI entrypoint and calls executeBatchOp directly via export.
func TestBatch_PolicyDeny(t *testing.T) {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		t.Error("unexpected HTTP request — policy should deny before any HTTP")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	}))
	defer srv.Close()

	// Create a policy that only allows "spaces *" operations (denies pages).
	pol, err := policy.NewFromConfig([]string{"spaces *"}, nil)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	ops := []cmd.BatchOp{
		{Command: "pages get", Args: map[string]string{}},
	}
	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
		Policy:     pol,
	}

	results := cmd.ExecuteBatchOps(c, ops)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// ExitValidation = 4
	if results[0].ExitCode != 4 {
		t.Errorf("policy denied: exit_code want 4, got %d", results[0].ExitCode)
	}
	if results[0].Error == nil {
		t.Error("expected error field in policy-denied result")
	}
	if atomic.LoadInt32(&requestCount) != 0 {
		t.Errorf("expected 0 HTTP requests, got %d", requestCount)
	}
}

// TestBatch_JQFilter verifies that --jq on the batch command filters the entire output array.
func TestBatch_JQFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	}))
	defer srv.Close()

	ops := []map[string]any{
		{"command": "pages get", "args": map[string]string{}},
	}
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	root.SetArgs([]string{"batch", "--input", inputFile, "--jq", ".[0].exit_code", "--dry-run=false"})
	root.Execute()

	outW.Close()
	os.Stdout = oldOut

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	output := strings.TrimSpace(outBuf.String())

	// .[0].exit_code should return the integer 0
	if output != "0" {
		t.Errorf("jq filter .[0].exit_code: want \"0\", got %q", output)
	}
}

// TestBatch_MultiOpSuccess verifies a multi-op batch where all ops succeed.
func TestBatch_MultiOpSuccess(t *testing.T) {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[],"_links":{}}`)
	}))
	defer srv.Close()

	ops := []map[string]any{
		{"command": "pages get", "args": map[string]string{}},
		{"command": "pages get", "args": map[string]string{}},
	}
	inputFile := writeTempBatchInput(t, ops)

	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	oldOut := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	root := cmd.RootCommand()
	// Pass --jq "" to reset any jq flag state from prior test runs (cobra singleton).
	root.SetArgs([]string{"batch", "--input", inputFile, "--jq", "", "--dry-run=false"})
	root.Execute()

	outW.Close()
	os.Stdout = oldOut

	var outBuf bytes.Buffer
	outBuf.ReadFrom(outR)
	output := strings.TrimSpace(outBuf.String())

	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("output is not valid JSON array: %v\nOutput: %s", err, output)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d; output: %s", len(results), output)
	}
	for i, r := range results {
		var exitCode int
		json.Unmarshal(r["exit_code"], &exitCode)
		if exitCode != 0 {
			t.Errorf("result[%d] exit_code: want 0, got %d", i, exitCode)
		}
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", requestCount)
	}
}
