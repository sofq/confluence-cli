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

// TestCommentsCreate_SendsCorrectBody verifies that the create subcommand sends
// a POST to /footer-comments with the correct pageId and body.representation fields.
func TestCommentsCreate_SendsCorrectBody(t *testing.T) {
	var capturedPath string
	var capturedBody bytes.Buffer

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if _, err := capturedBody.ReadFrom(r.Body); err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "com-1", "pageId": "page-42",
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
	_, wo, _ := os.Pipe()
	os.Stdout = wo
	oldStderr := os.Stderr
	_, wse, _ := os.Pipe()
	os.Stderr = wse

	root := cmd.RootCommand()
	root.SetArgs([]string{"comments", "create", "--page-id", "page-42", "--body", "<p>My comment</p>"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if capturedPath != "/footer-comments" {
		t.Errorf("expected path /footer-comments, got %q", capturedPath)
	}

	var body map[string]any
	if err := json.Unmarshal(capturedBody.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\nBody: %s", err, capturedBody.String())
	}
	if body["pageId"] != "page-42" {
		t.Errorf("expected pageId=%q, got %v", "page-42", body["pageId"])
	}
	bodyField, ok := body["body"].(map[string]any)
	if !ok {
		t.Fatalf("body.body is not a map: %v", body["body"])
	}
	if bodyField["representation"] != "storage" {
		t.Errorf("expected body.representation=%q, got %v", "storage", bodyField["representation"])
	}
}

// TestCommentsCreate_ValidationErrors verifies that missing --page-id or
// --body results in a validation error without an HTTP call.
func TestCommentsCreate_ValidationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing page-id", []string{"comments", "create", "--body", "<p>hi</p>", "--page-id", ""}},
		{"missing body", []string{"comments", "create", "--page-id", "123", "--body", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CF_BASE_URL", srv.URL)
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
			root.SetArgs(tt.args)
			err := root.Execute()

			wse.Close()
			wso.Close()
			os.Stderr = oldStderr
			os.Stdout = oldStdout

			var stderrBuf bytes.Buffer
			_, _ = stderrBuf.ReadFrom(rse)
			stderrOutput := strings.TrimSpace(stderrBuf.String())

			if err == nil {
				t.Errorf("expected an error for %q, got nil", tt.name)
			}

			if stderrOutput != "" {
				var errOut map[string]any
				if jsonErr := json.Unmarshal([]byte(stderrOutput), &errOut); jsonErr == nil {
					if errOut["error_type"] != "validation_error" {
						t.Errorf("error_type = %v, want validation_error", errOut["error_type"])
					}
				}
			}
		})
	}
}

// TestCommentsList_CallsCorrectPath verifies that the list subcommand calls
// GET /pages/{pageId}/footer-comments.
func TestCommentsList_CallsCorrectPath(t *testing.T) {
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
	root.SetArgs([]string{"comments", "list", "--page-id", "page-99"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if capturedPath != "/pages/page-99/footer-comments" {
		t.Errorf("expected path /pages/page-99/footer-comments, got %q", capturedPath)
	}
}

// TestCommentsDelete_CallsCorrectPath verifies that the delete subcommand sends
// DELETE to /footer-comments/{commentId}.
func TestCommentsDelete_CallsCorrectPath(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

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
	root.SetArgs([]string{"comments", "delete", "--comment-id", "com-123"})
	_ = root.Execute()

	wo.Close()
	wse.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if capturedPath != "/footer-comments/com-123" {
		t.Errorf("expected path /footer-comments/com-123, got %q", capturedPath)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected method DELETE, got %q", capturedMethod)
	}
}

