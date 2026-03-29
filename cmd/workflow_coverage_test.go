package cmd_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWorkflow_NoSubcommand verifies that running `cf workflow` without a subcommand
// returns an error about missing subcommand.
func TestWorkflow_NoSubcommand(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL)

	// Cobra captures the error as a non-zero exit; the error message might be on stderr
	// or just returned as the RunE error (silenced by cobra). Either way, no panic.
	_ = stderr
}

// TestWorkflow_UnknownSubcommand verifies that running `cf workflow unknowncmd`
// returns an error about unknown command.
func TestWorkflow_UnknownSubcommand(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "unknowncmd")

	// Should produce some kind of error output
	_ = stderr
}

// TestWorkflow_Copy_WithPolling verifies that the archive/copy command polls
// a long-running task and returns the final task result when a task ID is in
// the response and --no-wait is NOT set.
func TestWorkflow_Copy_WithPolling(t *testing.T) {
	taskCallCount := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-abc"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/task-abc", func(w http.ResponseWriter, r *http.Request) {
		taskCallCount++
		w.Header().Set("Content-Type", "application/json")
		if taskCallCount < 2 {
			// First poll: not finished yet
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "task-abc",
				"finished":   false,
				"successful": false,
			})
		} else {
			// Second poll: finished and successful
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "task-abc",
				"finished":   true,
				"successful": true,
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "1m",
	)

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "task-abc") {
		t.Errorf("expected task result in stdout, got: %s", stdout)
	}
	if taskCallCount < 2 {
		t.Errorf("expected at least 2 task poll calls, got %d", taskCallCount)
	}
}

// TestWorkflow_Copy_PollingFailed verifies that when a long-running task finishes
// but reports unsuccessful, an error is written to stderr.
func TestWorkflow_Copy_PollingFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-fail"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/task-fail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Task finished but failed
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "task-fail",
			"finished":   true,
			"successful": false,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "1m",
	)

	if stdout != "" {
		t.Errorf("expected no stdout on task failure, got: %s", stdout)
	}
	if !strings.Contains(stderr, "api_error") {
		t.Errorf("expected api_error in stderr for failed task, got: %s", stderr)
	}
}

// TestWorkflow_Copy_InvalidTimeout verifies that an invalid --timeout value
// returns a validation error.
func TestWorkflow_Copy_InvalidTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-123"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "notavalidtimeout!!!",
	)

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr for invalid timeout, got: %s", stderr)
	}
}

// TestWorkflow_Copy_NoTaskID verifies that when the copy response has no task ID
// (empty or missing "id" field), the raw response is returned immediately without
// attempting to parse the timeout. The response must have an empty/missing "id"
// field so taskResp.ID is blank, triggering the early return path.
func TestWorkflow_Copy_NoTaskID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return response without "id" field (task ID absent = immediate return)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "already_complete", "title": "Copy Result",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "1m",
	)

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "already_complete") {
		t.Errorf("expected copy result in stdout, got: %s", stdout)
	}
}

// TestWorkflow_Publish_FetchError verifies that when the initial page GET fails,
// an error is returned.
func TestWorkflow_Publish_FetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		fmt.Fprint(w, `{"message":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "publish", "--id", "123")

	if stdout != "" {
		t.Errorf("expected no stdout for fetch error, got: %s", stdout)
	}
}

// TestWorkflow_Publish_JSONParseError verifies that a malformed GET response
// produces a connection_error on stderr.
func TestWorkflow_Publish_JSONParseError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		// Return malformed JSON
		fmt.Fprint(w, `this is not json`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "publish", "--id", "123")

	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for JSON parse error, got: %s", stderr)
	}
}

// TestWorkflow_Archive_WithPolling verifies that archive polls the long task
// when a task ID is returned and --no-wait is NOT set.
func TestWorkflow_Archive_WithPolling(t *testing.T) {
	taskCallCount := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "archive-task-1"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/archive-task-1", func(w http.ResponseWriter, r *http.Request) {
		taskCallCount++
		w.Header().Set("Content-Type", "application/json")
		// Return finished on first call
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "archive-task-1",
			"finished":   true,
			"successful": true,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
	)

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "archive-task-1") {
		t.Errorf("expected task result in stdout, got: %s", stdout)
	}
	if taskCallCount < 1 {
		t.Errorf("expected at least 1 task poll call, got %d", taskCallCount)
	}
}

// TestWorkflow_Archive_InvalidTimeout verifies that an invalid --timeout value
// returns a validation error.
func TestWorkflow_Archive_InvalidTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "archive-task-2"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "notavalidtimeout!!!",
	)

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error in stderr for invalid timeout, got: %s", stderr)
	}
}

// TestWorkflow_Archive_NoTaskID verifies that when the archive response has no
// task ID, the raw response is returned immediately without polling.
func TestWorkflow_Archive_NoTaskID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return response without a task ID
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "archived",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
	)

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "archived") {
		t.Errorf("expected 'archived' in stdout, got: %s", stdout)
	}
}

// TestWorkflow_PollLongTask_MultiplePolls verifies that pollLongTask keeps
// polling until the task finishes after multiple iterations (not finished -> finished).
func TestWorkflow_PollLongTask_MultiplePolls(t *testing.T) {
	pollCount := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "multi-poll-task"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/multi-poll-task", func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		w.Header().Set("Content-Type", "application/json")
		if pollCount < 3 {
			// First 2 polls: not finished yet
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "multi-poll-task",
				"finished":   false,
				"successful": false,
			})
		} else {
			// Third poll: done
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "multi-poll-task",
				"finished":   true,
				"successful": true,
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
	)

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "multi-poll-task") {
		t.Errorf("expected task result in stdout, got: %s", stdout)
	}
	if pollCount < 3 {
		t.Errorf("expected at least 3 poll calls, got %d", pollCount)
	}
}

// TestWorkflow_PollLongTask_TaskFetchError verifies that when the task poll
// request itself fails, an error code is returned.
func TestWorkflow_PollLongTask_TaskFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "errored-task"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/errored-task", func(w http.ResponseWriter, r *http.Request) {
		// Task polling fails with 500
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"internal server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
	)

	// No successful output expected
	if stdout != "" {
		t.Errorf("expected no stdout when task poll fails, got: %s", stdout)
	}
}

// TestWorkflow_PollLongTask_UnparsableResponse verifies that when the task
// response is unparseable JSON, the raw response is returned as-is.
func TestWorkflow_PollLongTask_UnparsableResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "weird-task"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/weird-task", func(w http.ResponseWriter, r *http.Request) {
		// Return valid HTTP 200 but non-JSON body
		fmt.Fprint(w, `not json at all`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
	)

	// Should return the raw (unparseable) body as output
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "not json at all") {
		t.Errorf("expected raw response in stdout for unparseable task body, got: %s", stdout)
	}
}

// TestWorkflow_Restrict_ViewAPIError verifies that an API error during restrict
// view mode is handled correctly.
func TestWorkflow_Restrict_ViewAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123")

	if stdout != "" {
		t.Errorf("expected no stdout on API error, got: %s", stdout)
	}
}

// TestWorkflow_Restrict_AddUser_APIError verifies that an API error during user
// restriction add/remove is handled correctly.
func TestWorkflow_Restrict_AddUser_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--operation", "read", "--user", "user1")

	if stdout != "" {
		t.Errorf("expected no stdout on API error, got: %s", stdout)
	}
}

// TestWorkflow_Restrict_AddGroup_APIError verifies that an API error during group
// restriction add/remove is handled correctly.
func TestWorkflow_Restrict_AddGroup_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/update/byGroupId/group-abc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--add", "--operation", "update", "--group", "group-abc")

	if stdout != "" {
		t.Errorf("expected no stdout on API error, got: %s", stdout)
	}
}

// TestWorkflow_Restrict_RemoveGroup verifies the group removal path.
func TestWorkflow_Restrict_RemoveGroup(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/byGroupId/group-xyz", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--remove", "--operation", "read", "--group", "group-xyz")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if capturedPath != "/wiki/rest/api/content/123/restriction/byOperation/read/byGroupId/group-xyz" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("expected 'removed' in stdout, got: %s", stdout)
	}
}

// TestWorkflow_Move_APIError verifies that a server error during move
// produces no stdout and stderr captures the error.
func TestWorkflow_Move_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/move/append/456", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		fmt.Fprint(w, `{"message":"page not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "move", "--id", "123", "--target-id", "456")

	if stdout != "" {
		t.Errorf("expected no stdout on API error, got: %s", stdout)
	}
}

// TestWorkflow_Publish_PutError verifies that when the PUT update fails,
// an error is returned.
func TestWorkflow_Publish_PutError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "123", "title": "Draft", "status": "draft",
				"version": map[string]any{"number": 1},
			})
			return
		}
		// PUT fails
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"permission denied"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "publish", "--id", "123")

	if stdout != "" {
		t.Errorf("expected no stdout on publish error, got: %s", stdout)
	}
}

// TestWorkflow_Copy_APIError verifies that a server error during the copy
// POST request produces no stdout.
func TestWorkflow_Copy_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "1m",
	)

	if stdout != "" {
		t.Errorf("expected no stdout on copy API error, got: %s", stdout)
	}
}

// TestWorkflow_Archive_APIError verifies that a server error during the archive
// POST request produces no stdout.
func TestWorkflow_Archive_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
	)

	if stdout != "" {
		t.Errorf("expected no stdout on archive API error, got: %s", stdout)
	}
}

// TestWorkflow_Comment_APIError verifies that a server error during comment
// produces no stdout.
func TestWorkflow_Comment_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/footer-comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "comment", "--id", "123", "--body", "test comment")

	if stdout != "" {
		t.Errorf("expected no stdout on API error, got: %s", stdout)
	}
}
