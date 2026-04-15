package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
)

// TestBatch_DiffPathSubstitution verifies that batch correctly substitutes
// {id} in the diff command's path template (/pages/{id}/versions).
func TestBatch_DiffPathSubstitution(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"number":2,"authorId":"a","createdAt":"2026-01-01T00:00:00Z","message":""}],"_links":{}}`))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}

	ops := []cmd.BatchOp{
		{Command: "diff diff", Args: map[string]string{"id": "12345"}},
	}
	results := cmd.ExecuteBatchOps(c, ops)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The path should have 12345 substituted, not the literal {id}.
	if strings.Contains(gotPath, "{id}") {
		t.Errorf("path contains unsubstituted {id}: %s", gotPath)
	}
	if !strings.Contains(gotPath, "/pages/12345/versions") {
		t.Errorf("expected path to contain /pages/12345/versions, got: %s", gotPath)
	}
}

// TestBatch_ExportPathSubstitution verifies that batch correctly substitutes
// {id} in the export command's path template (/pages/{id}).
func TestBatch_ExportPathSubstitution(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"67890","title":"Test","body":{"storage":{"value":"<p>Hello</p>"}}}`))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}

	ops := []cmd.BatchOp{
		{Command: "export export", Args: map[string]string{"id": "67890"}},
	}
	results := cmd.ExecuteBatchOps(c, ops)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if strings.Contains(gotPath, "{id}") {
		t.Errorf("path contains unsubstituted {id}: %s", gotPath)
	}
	if !strings.Contains(gotPath, "/pages/67890") {
		t.Errorf("expected path to contain /pages/67890, got: %s", gotPath)
	}
}

// TestBatch_WorkflowCommentPathSubstitution verifies that batch correctly
// substitutes {id} in workflow comment's path (/pages/{id}/footer-comments).
func TestBatch_WorkflowCommentPathSubstitution(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"99","status":"current"}`))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}

	ops := []cmd.BatchOp{
		{Command: "workflow comment", Args: map[string]string{"id": "55555", "body": "test comment"}},
	}
	results := cmd.ExecuteBatchOps(c, ops)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if strings.Contains(gotPath, "{id}") {
		t.Errorf("path contains unsubstituted {id}: %s", gotPath)
	}
	if !strings.Contains(gotPath, "/pages/55555/footer-comments") {
		t.Errorf("expected path to contain /pages/55555/footer-comments, got: %s", gotPath)
	}
}

// TestBatch_WorkflowMovePathSubstitution verifies batch substitution of both
// {id} and {targetId} in the move command's path.
func TestBatch_WorkflowMovePathSubstitution(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"111","title":"Moved"}`))
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}

	ops := []cmd.BatchOp{
		{Command: "workflow move", Args: map[string]string{"id": "111", "target-id": "222"}},
	}
	results := cmd.ExecuteBatchOps(c, ops)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if strings.Contains(gotPath, "{id}") || strings.Contains(gotPath, "{targetId}") {
		t.Errorf("path contains unsubstituted placeholders: %s", gotPath)
	}
}

// TestBatch_CustomCommandMissingRequiredPathParam verifies that batch returns
// a validation error when a required path parameter is missing.
func TestBatch_CustomCommandMissingRequiredPathParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request — should fail validation before making a request")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &client.Client{
		BaseURL:    srv.URL,
		Auth:       config.AuthConfig{Type: "bearer", Token: "test-token"},
		HTTPClient: srv.Client(),
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
	}

	ops := []cmd.BatchOp{
		{Command: "diff diff", Args: map[string]string{}}, // missing required "id"
	}
	results := cmd.ExecuteBatchOps(c, ops)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ExitCode == 0 {
		t.Error("expected non-zero exit code for missing required path param")
	}
	if results[0].Error == nil {
		t.Error("expected error in result for missing required path param")
	}

	// Verify error message mentions the missing parameter.
	var errObj map[string]string
	if err := json.Unmarshal(results[0].Error, &errObj); err == nil {
		if !strings.Contains(errObj["message"], "id") {
			t.Errorf("expected error to mention 'id', got: %s", errObj["message"])
		}
	}
}
