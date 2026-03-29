package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/spf13/cobra"
)

// runWorkflowCommand executes `cf workflow <args>` against the test server,
// capturing stdout and stderr. Uses setupTemplateEnv for config setup.
func runWorkflowCommand(t *testing.T, srvURL string, args ...string) (stdout string, stderr string) {
	t.Helper()
	setupTemplateEnv(t, srvURL, nil)

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	resetWorkflowFlags(root)
	_ = root.PersistentFlags().Set("dry-run", "false")
	root.SetArgs(append([]string{"workflow"}, args...))
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)

	return outBuf.String(), errBuf.String()
}

// resetWorkflowFlags resets all workflow subcommand flags to prevent Cobra
// singleton contamination between tests.
func resetWorkflowFlags(root *cobra.Command) {
	for _, sub := range root.Commands() {
		if sub.Name() != "workflow" {
			continue
		}
		for _, wfSub := range sub.Commands() {
			wfSub.ResetFlags()
			switch wfSub.Name() {
			case "move":
				wfSub.Flags().String("id", "", "page ID to move (required)")
				wfSub.Flags().String("target-id", "", "target parent page ID (required)")
			case "copy":
				wfSub.Flags().String("id", "", "page ID to copy (required)")
				wfSub.Flags().String("target-id", "", "target parent page ID (required)")
				wfSub.Flags().String("title", "", "title for the copied page")
				wfSub.Flags().Bool("copy-attachments", false, "include attachments in copy")
				wfSub.Flags().Bool("copy-labels", false, "include labels in copy")
				wfSub.Flags().Bool("copy-permissions", false, "include permissions in copy")
				wfSub.Flags().Bool("no-wait", false, "return immediately without polling")
				wfSub.Flags().String("timeout", "60s", "timeout for async operation (e.g. 30s, 2m)")
			case "publish":
				wfSub.Flags().String("id", "", "page ID to publish (required)")
			case "comment":
				wfSub.Flags().String("id", "", "page ID to comment on (required)")
				wfSub.Flags().String("body", "", "comment text (required)")
			case "restrict":
				wfSub.Flags().String("id", "", "page ID to manage restrictions (required)")
				wfSub.Flags().Bool("add", false, "add a restriction")
				wfSub.Flags().Bool("remove", false, "remove a restriction")
				wfSub.Flags().String("operation", "", "restriction operation: read or update")
				wfSub.Flags().String("user", "", "user account ID")
				wfSub.Flags().String("group", "", "group name")
			case "archive":
				wfSub.Flags().String("id", "", "page ID to archive (required)")
				wfSub.Flags().Bool("no-wait", false, "return immediately without polling")
				wfSub.Flags().String("timeout", "60s", "timeout for async operation (e.g. 30s, 2m)")
			}
		}
		break
	}
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

func TestWorkflow_Move_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "move", "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Move_MissingTargetID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "move", "--id", "123", "--target-id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--target-id must not be empty") {
		t.Errorf("expected '--target-id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Copy_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "copy", "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Copy_MissingTargetID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "copy", "--id", "123", "--target-id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--target-id must not be empty") {
		t.Errorf("expected '--target-id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Publish_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "publish", "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Comment_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "comment", "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Comment_MissingBody(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "comment", "--id", "123", "--body", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--body must not be empty") {
		t.Errorf("expected '--body must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Restrict_AddRemoveMutualExclusion(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--remove", "--operation", "read", "--user", "u1")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Restrict_AddMissingOperation(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--user", "u1")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--operation") {
		t.Errorf("expected '--operation' error in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Restrict_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Restrict_InvalidOperation(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--operation", "delete", "--user", "u1")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "'read' or 'update'") {
		t.Errorf("expected operation validation error in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Restrict_AddMissingUserAndGroup(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--operation", "read")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--user or --group") {
		t.Errorf("expected user/group required error in stderr, got: %s", stderr)
	}
}

func TestWorkflow_Archive_MissingID(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "archive", "--id", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--id must not be empty") {
		t.Errorf("expected '--id must not be empty' in stderr, got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// API integration tests with mock HTTP servers
// ---------------------------------------------------------------------------

func TestWorkflow_Move_Success(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/move/append/456", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "123", "title": "Moved Page", "status": "current",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "move", "--id", "123", "--target-id", "456")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/123/move/append/456" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if !strings.Contains(stdout, "Moved Page") {
		t.Errorf("expected 'Moved Page' in stdout, got: %s", stdout)
	}
}

func TestWorkflow_Copy_NoWait(t *testing.T) {
	var capturedBody bytes.Buffer
	var capturedMethod string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		_, _ = io.Copy(&capturedBody, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-1"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--title", "Copied Page",
		"--copy-attachments",
		"--copy-labels",
		"--no-wait",
	)

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if !strings.Contains(stdout, "task-1") {
		t.Errorf("expected 'task-1' in stdout, got: %s", stdout)
	}

	// Verify request body shape.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody.Bytes(), &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v\nBody: %s", err, capturedBody.String())
	}
	if reqBody["copyAttachments"] != true {
		t.Errorf("expected copyAttachments=true, got %v", reqBody["copyAttachments"])
	}
	if reqBody["copyLabels"] != true {
		t.Errorf("expected copyLabels=true, got %v", reqBody["copyLabels"])
	}
	if reqBody["pageTitle"] != "Copied Page" {
		t.Errorf("expected pageTitle='Copied Page', got %v", reqBody["pageTitle"])
	}
	dest, ok := reqBody["destination"].(map[string]any)
	if !ok {
		t.Fatal("expected destination object in request body")
	}
	if dest["type"] != "parent_page" {
		t.Errorf("expected destination.type='parent_page', got %v", dest["type"])
	}
	if dest["value"] != "456" {
		t.Errorf("expected destination.value='456', got %v", dest["value"])
	}
}

func TestWorkflow_Publish_Success(t *testing.T) {
	var putCalled bool
	var putBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Draft Page", "status": "draft",
				"version": map[string]any{"number": 1},
			})
			return
		}
		if r.Method == "PUT" {
			putCalled = true
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Draft Page", "status": "current",
				"version": map[string]any{"number": 2},
			})
			return
		}
		w.WriteHeader(405)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "publish", "--id", "123")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !putCalled {
		t.Error("expected PUT to be called for publish")
	}
	if !strings.Contains(stdout, "current") {
		t.Errorf("expected 'current' status in stdout, got: %s", stdout)
	}

	// Verify version was incremented.
	if putBody != nil {
		ver, ok := putBody["version"].(map[string]any)
		if !ok {
			t.Fatal("expected version in PUT body")
		}
		// JSON numbers decode as float64.
		if ver["number"] != float64(2) {
			t.Errorf("expected version.number=2, got %v", ver["number"])
		}
		if putBody["status"] != "current" {
			t.Errorf("expected status='current', got %v", putBody["status"])
		}
	}
}

func TestWorkflow_Comment_Success(t *testing.T) {
	var capturedBody bytes.Buffer
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/footer-comments", func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = io.Copy(&capturedBody, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "comment-1", "pageId": "123",
			"body": map[string]any{
				"value":          "<p>Hello World</p>",
				"representation": "storage",
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "comment", "--id", "123", "--body", "Hello World")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedPath != "/wiki/api/v2/footer-comments" {
		t.Errorf("expected path /wiki/api/v2/footer-comments, got %s", capturedPath)
	}
	if !strings.Contains(stdout, "comment-1") {
		t.Errorf("expected 'comment-1' in stdout, got: %s", stdout)
	}

	// Verify request body has correct structure.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody.Bytes(), &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v\nBody: %s", err, capturedBody.String())
	}
	if reqBody["pageId"] != "123" {
		t.Errorf("expected pageId='123', got %v", reqBody["pageId"])
	}
	body, ok := reqBody["body"].(map[string]any)
	if !ok {
		t.Fatal("expected body object in request body")
	}
	if body["representation"] != "storage" {
		t.Errorf("expected representation='storage', got %v", body["representation"])
	}
	if body["value"] != "<p>Hello World</p>" {
		t.Errorf("expected value='<p>Hello World</p>', got %v", body["value"])
	}
}

func TestWorkflow_Restrict_View(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"operation": "read", "restrictions": map[string]any{"user": map[string]any{"results": []any{}}}},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "GET" {
		t.Errorf("expected GET, got %s", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/123/restriction" {
		t.Errorf("expected restriction path, got %s", capturedPath)
	}
	if !strings.Contains(stdout, "results") {
		t.Errorf("expected 'results' in stdout, got: %s", stdout)
	}
}

func TestWorkflow_Restrict_AddUser(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/user", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--operation", "read", "--user", "user1")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/123/restriction/byOperation/read/user" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if !strings.Contains(capturedQuery, "accountId=user1") {
		t.Errorf("expected accountId=user1 in query, got: %s", capturedQuery)
	}
	if !strings.Contains(stdout, "added") {
		t.Errorf("expected 'added' in stdout, got: %s", stdout)
	}
}

func TestWorkflow_Restrict_RemoveUser(t *testing.T) {
	var capturedMethod string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/user", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--remove", "--operation", "read", "--user", "user1")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("expected 'removed' in stdout, got: %s", stdout)
	}
}

func TestWorkflow_Restrict_AddGroup(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/update/byGroupId/group-abc", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--operation", "update", "--group", "group-abc")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/123/restriction/byOperation/update/byGroupId/group-abc" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if !strings.Contains(stdout, "added") {
		t.Errorf("expected 'added' in stdout, got: %s", stdout)
	}
}

func TestWorkflow_Archive_Success(t *testing.T) {
	var capturedBody bytes.Buffer
	var capturedMethod string
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		_, _ = io.Copy(&capturedBody, r.Body)
		w.Header().Set("Content-Type", "application/json")
		// Return a completed response (no task ID -- immediate success).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive", "--id", "123", "--no-wait")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/archive" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if !strings.Contains(stdout, "completed") {
		t.Errorf("expected 'completed' in stdout, got: %s", stdout)
	}

	// Verify request body has pages array.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody.Bytes(), &reqBody); err != nil {
		t.Fatalf("failed to parse request body: %v\nBody: %s", err, capturedBody.String())
	}
	pages, ok := reqBody["pages"].([]any)
	if !ok {
		t.Fatal("expected pages array in request body")
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	page, ok := pages[0].(map[string]any)
	if !ok {
		t.Fatal("expected page object in pages array")
	}
	if page["id"] != "123" {
		t.Errorf("expected page id='123', got %v", page["id"])
	}
}
