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

// TestLabelsAdd_SendsV1Body verifies that the add subcommand POSTs to the v1
// label path with array of {prefix:"global",name:...} items.
func TestLabelsAdd_SendsV1Body(t *testing.T) {
	var capturedPath string
	var capturedBody bytes.Buffer

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if _, err := capturedBody.ReadFrom(r.Body); err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"prefix": "global", "name": "mytag"}},
		})
	}))
	defer srv.Close()

	// BaseURL must include /wiki/api/v2 so searchV1Domain extracts the domain correctly.
	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"labels", "add", "--page-id", "page-10", "--label", "mytag"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// v1 path: /wiki/rest/api/content/{pageId}/label
	if capturedPath != "/wiki/rest/api/content/page-10/label" {
		t.Errorf("expected v1 path /wiki/rest/api/content/page-10/label, got %q", capturedPath)
	}

	// Body must be an array of {prefix,name} objects
	var items []map[string]any
	if err := json.Unmarshal(capturedBody.Bytes(), &items); err != nil {
		t.Fatalf("body is not a JSON array: %v\nBody: %s", err, capturedBody.String())
	}
	if len(items) == 0 {
		t.Fatal("expected at least one label item in body")
	}
	if items[0]["prefix"] != "global" {
		t.Errorf("expected prefix=%q, got %v", "global", items[0]["prefix"])
	}
	if items[0]["name"] != "mytag" {
		t.Errorf("expected name=%q, got %v", "mytag", items[0]["name"])
	}
}

// TestLabelsAdd_MissingPageID verifies that --page-id="" returns a validation error.
func TestLabelsAdd_MissingPageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
		w.WriteHeader(http.StatusOK)
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
	root.SetArgs([]string{"labels", "add", "--page-id", "", "--label", "mytag"})
	err := root.Execute()

	wse.Close()
	wso.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(rse)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if err == nil {
		t.Error("expected an error for missing --page-id, got nil")
	}

	if stderrOutput != "" {
		var errOut map[string]any
		if jsonErr := json.Unmarshal([]byte(stderrOutput), &errOut); jsonErr == nil {
			if errOut["error_type"] != "validation_error" {
				t.Errorf("error_type = %v, want validation_error", errOut["error_type"])
			}
		}
	}
}

// TestLabelsAdd_MissingLabel verifies that an empty --label returns a validation error.
// This test uses a client-injection approach to avoid cobra singleton flag state issues.
func TestLabelsAdd_MissingLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use the exported helper to test the validation logic directly.
	// labels_add validates that len(items) == 0 after filtering empty label names.
	// We verify this through LabelAddValidation.
	exitCode := cmd.LabelsAddValidation("page-123", nil)
	if exitCode == 0 {
		t.Error("expected non-zero exit code for empty labels, got 0")
	}
}

// TestLabelsRemove_SendsDeleteToV1 verifies that the remove subcommand sends
// DELETE to the v1 path with ?name= query parameter.
func TestLabelsRemove_SendsDeleteToV1(t *testing.T) {
	var capturedPath string
	var capturedQuery string
	var capturedMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"labels", "remove", "--page-id", "page-10", "--label", "mytag"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if capturedMethod != "DELETE" {
		t.Errorf("expected method DELETE, got %q", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/page-10/label" {
		t.Errorf("expected v1 path /wiki/rest/api/content/page-10/label, got %q", capturedPath)
	}
	if !strings.Contains(capturedQuery, "name=mytag") {
		t.Errorf("expected name=mytag in query, got %q", capturedQuery)
	}
}

// TestLabelsRemove_OutputsConfirmation verifies that remove outputs
// JSON with status:"removed" and the label name.
func TestLabelsRemove_OutputsConfirmation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("CF_BASE_URL", srv.URL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	rout, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"labels", "remove", "--page-id", "page-10", "--label", "removeme"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rout)
	output := strings.TrimSpace(outBuf.String())

	if output == "" {
		t.Fatal("expected JSON output from labels remove, got nothing")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}
	if result["status"] != "removed" {
		t.Errorf("expected status=%q, got %v", "removed", result["status"])
	}
	if result["label"] != "removeme" {
		t.Errorf("expected label=%q, got %v", "removeme", result["label"])
	}
}

// TestLabelsList_CallsCorrectPath verifies that the list subcommand sends
// GET to /pages/{pageId}/labels (v2 path).
func TestLabelsList_CallsCorrectPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]any{},
		})
	}))
	defer srv.Close()

	// labels_list uses v2 path via c.Do, so CF_BASE_URL should NOT include /wiki/api/v2
	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"labels", "list", "--page-id", "page-55"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if capturedPath != "/pages/page-55/labels" {
		t.Errorf("expected path /pages/page-55/labels, got %q", capturedPath)
	}
}
