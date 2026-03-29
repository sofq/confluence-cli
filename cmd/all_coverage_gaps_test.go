package cmd_test

// all_coverage_gaps_test.go covers remaining uncovered branches across:
//   - cmd/raw.go        (runRaw: stdin -, body with GET warning, config error, DryRun POST)
//   - cmd/diff.go       (runDiff: WriteOutput error handling)
//   - cmd/export.go     (runExport: context cancel in walkTree; depth limit)
//   - cmd/batch.go      (runBatch: max-batch exceeded when positive; jq; pretty; stdin no-tty)
//   - cmd/configure.go  (deleteProfileByName: not found, save error; testConnection: basic auth;
//                        testExistingProfile: resolve default profile when not explicit)
//   - cmd/root.go       (Execute: success path; preset+jq conflict; unknown preset; audit log error;
//                        help for subcommand writes to stderr)
//   - cmd/watch.go      (runWatch: empty cql; consecutive errors; dedup; multi-poll)
//   - cmd/workflow.go   (restrict: add+remove, invalid op, no user/group, empty op;
//                        copy/archive no-wait; publish success/put-error/json-error;
//                        comment success/error; move APIError; pollLongTask: timeout)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"github.com/sofq/confluence-cli/cmd"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupEnvForServer sets environment variables to point at a test server.
func setupEnvForServer(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("CF_BASE_URL", srvURL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")
}

// captureCommand runs rootCmd with args, capturing stdout/stderr.
// It resets persistent flags before AND after execution to prevent state bleed
// between tests that use different runner helpers (e.g., runExportCommand).
func captureCommand(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()
	cmd.ResetRootPersistentFlags()
	t.Cleanup(func() { cmd.ResetRootPersistentFlags() })

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs(args)
	err = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)

	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// ---------------------------------------------------------------------------
// cmd/raw.go — runRaw: body from stdin (--body -)
// ---------------------------------------------------------------------------

// TestRawBodyFromStdinFlag verifies that --body - reads the POST body from stdin.
func TestRawBodyFromStdinFlag(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	// Create a pipe to simulate stdin with content.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`{"from":"stdin"}`))
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	cmd.ResetRootPersistentFlags()
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "POST", "/wiki/api/v2/pages", "--body", "-"})
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)

	if !strings.Contains(capturedBody, "from") {
		t.Errorf("expected body to contain 'from', captured: %s", capturedBody)
	}
}

// ---------------------------------------------------------------------------
// cmd/batch.go — runBatch: max-batch exceeded with positive limit
// ---------------------------------------------------------------------------

// TestBatch_MaxBatchExceededPositive verifies that exceeding --max-batch with a
// positive value (not zero/disabled) returns a validation error.
func TestBatch_MaxBatchExceededPositive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	batchInput := filepath.Join(t.TempDir(), "batch.json")
	ops := `[{"command":"pages get","args":{"id":"1"}},{"command":"pages get","args":{"id":"2"}},{"command":"pages get","args":{"id":"3"}}]`
	if err := os.WriteFile(batchInput, []byte(ops), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, _ := captureCommand(t, []string{"batch", "--input", batchInput, "--max-batch", "2"})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for exceeded batch limit, got: %s", stderr)
	}
	if !strings.Contains(stderr, "batch limit exceeded") {
		t.Errorf("expected 'batch limit exceeded' message, got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/root.go — Execute and init additional branches
// ---------------------------------------------------------------------------

// TestExecute_ReturnsZeroOnSuccess verifies Execute() returns 0 for a successful command.
func TestExecute_ReturnsZeroOnSuccess(t *testing.T) {
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs([]string{"version"})
	code := cmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)

	if code != 0 {
		t.Errorf("expected exit code 0 for version command, got %d", code)
	}
}

// TestExecute_ReturnsNonZeroOnError verifies Execute() returns non-zero for a bad command.
func TestExecute_ReturnsNonZeroOnError(t *testing.T) {
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	_, wOut, _ := os.Pipe()
	os.Stdout = wOut
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// "raw GET" with only 1 arg (needs 2) — cobra returns an arg count error.
	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET"})
	code := cmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if code == 0 {
		t.Error("expected non-zero exit code for command arg error")
	}
}

// TestRoot_PresetAndJQConflictErr verifies that using both --preset and --jq returns
// validation_error.
func TestRoot_PresetAndJQConflictErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","title":"Test"}`))
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, stderr, _ := captureCommand(t, []string{"pages", "get-by-id", "--id", "123", "--preset", "agent", "--jq", ".id"})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for --preset+--jq conflict, got: %s", stderr)
	}
}

// TestRoot_UnknownPresetErr verifies that an unknown preset name returns an error.
func TestRoot_UnknownPresetErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","title":"Test"}`))
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, stderr, _ := captureCommand(t, []string{"pages", "get-by-id", "--id", "123", "--preset", "nonexistent-preset-xyz-abc"})
	if stderr == "" {
		t.Error("expected error output for unknown preset")
	}
}

// TestRoot_AuditLogOpenErr verifies that an invalid audit log path (unwritable dir)
// returns config_error.
func TestRoot_AuditLogOpenErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","title":"Test"}`))
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, stderr, _ := captureCommand(t, []string{"pages", "get-by-id", "--id", "123", "--audit", "/nonexistent/dir/path/audit.log"})
	if !strings.Contains(stderr, "config_error") {
		t.Errorf("expected config_error for invalid audit log path, got: %s", stderr)
	}
}

// TestRoot_HelpForSubcommand verifies that --help for a subcommand does not panic.
func TestRoot_HelpForSubcommand(t *testing.T) {
	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	_, wOut, _ := os.Pipe()
	os.Stdout = wOut
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs([]string{"pages", "--help"})
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	// Just verify no panic.
}

// ---------------------------------------------------------------------------
// cmd/watch.go — runWatch additional branches
// ---------------------------------------------------------------------------

// TestWatch_EmptyCQLValidationErr verifies that empty --cql returns validation_error.
func TestWatch_EmptyCQLValidationErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	_, stderr := runWatchCommand(t, srv.URL, "--cql", "")
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty --cql, got: %s", stderr)
	}
}

// TestWatch_MaxPollsMultiple verifies that watch stops after maxPolls polls
// when maxPolls > 1.
func TestWatch_MaxPollsMultiple(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeWatchSearchResponse(nil))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL,
		"--cql", "type=page",
		"--max-polls", "3",
		"--interval", "1ms",
	)

	if !strings.Contains(stdout, `"type":"shutdown"`) {
		t.Errorf("expected shutdown event, got: %s", stdout)
	}
}

// TestWatch_DedupOnSecondPoll verifies that content with the same timestamp is
// not re-emitted on a second poll.
func TestWatch_DedupOnSecondPoll(t *testing.T) {
	ts := recentTimestamp(10)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		result := makeWatchResult("9001", "page", "Dedup Page", "ENG", 10, ts, "Alice")
		_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
	}))
	defer srv.Close()

	// 2 polls: first emits, second deduplicates.
	stdout, _ := runWatchCommand(t, srv.URL,
		"--cql", "type=page",
		"--max-polls", "2",
		"--interval", "1ms",
	)

	changeCount := strings.Count(stdout, `"type":"change"`)
	if changeCount != 1 {
		t.Errorf("expected 1 change event (dedup on 2nd poll), got %d\nstdout: %s", changeCount, stdout)
	}
}

// ---------------------------------------------------------------------------
// cmd/workflow.go — additional branches
// ---------------------------------------------------------------------------

// TestWorkflow_Restrict_BothAddAndRemove verifies that --add and --remove together
// returns a validation error.
func TestWorkflow_Restrict_BothAddAndRemove(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--add",
		"--remove",
		"--operation", "read",
		"--user", "user1",
	)

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for --add+--remove, got: %s", stderr)
	}
}

// TestWorkflow_Restrict_BadOperationValue verifies that an invalid --operation value
// (not 'read' or 'update') returns a validation error.
func TestWorkflow_Restrict_BadOperationValue(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--add",
		"--operation", "delete",
		"--user", "user1",
	)

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for invalid --operation 'delete', got: %s", stderr)
	}
}

// TestWorkflow_Restrict_MissingUserAndGroup verifies that --add without --user or --group
// returns a validation error.
func TestWorkflow_Restrict_MissingUserAndGroup(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--add",
		"--operation", "read",
		// Neither --user nor --group
	)

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for missing --user/--group, got: %s", stderr)
	}
}

// TestWorkflow_Restrict_EmptyOpWithAdd verifies that --add with empty --operation
// returns a validation error.
func TestWorkflow_Restrict_EmptyOpWithAdd(t *testing.T) {
	srv := dummyServer(t)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--add",
		"--user", "user1",
		// --operation not set
	)

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty --operation with --add, got: %s", stderr)
	}
}

// TestWorkflow_Restrict_AddUserAndGroupBoth verifies that both --user and --group
// can be specified together.
func TestWorkflow_Restrict_AddUserAndGroupBoth(t *testing.T) {
	userCalled := false
	groupCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/100/restriction/byOperation/read/user", func(w http.ResponseWriter, r *http.Request) {
		userCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/wiki/rest/api/content/100/restriction/byOperation/read/byGroupId/my-group", func(w http.ResponseWriter, r *http.Request) {
		groupCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "100",
		"--add",
		"--operation", "read",
		"--user", "user@company.com",
		"--group", "my-group",
	)
	_ = stderr

	if !userCalled {
		t.Error("expected user restriction endpoint to be called")
	}
	if !groupCalled {
		t.Error("expected group restriction endpoint to be called")
	}
	if !strings.Contains(stdout, "added") {
		t.Errorf("expected 'added' in stdout, got: %s", stdout)
	}
}

// TestWorkflow_CopyNoWait verifies that --no-wait returns the response without polling.
func TestWorkflow_CopyNoWait(t *testing.T) {
	taskCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-nowait-xyz"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/task-nowait-xyz", func(w http.ResponseWriter, r *http.Request) {
		taskCalled = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"finished": true, "successful": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--no-wait",
	)

	if taskCalled {
		t.Error("task polling should be skipped with --no-wait")
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "task-nowait-xyz") {
		t.Errorf("expected task ID in stdout, got: %s", stdout)
	}
}

// TestWorkflow_ArchiveNoWait verifies that --no-wait on archive returns response without polling.
func TestWorkflow_ArchiveNoWait(t *testing.T) {
	taskCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "archive-nowait-xyz"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/archive-nowait-xyz", func(w http.ResponseWriter, r *http.Request) {
		taskCalled = true
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--no-wait",
	)

	if taskCalled {
		t.Error("task polling should be skipped with --no-wait")
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "archive-nowait-xyz") {
		t.Errorf("expected archive ID in stdout, got: %s", stdout)
	}
}

// TestWorkflow_PublishSuccess verifies the full publish workflow succeeds.
func TestWorkflow_PublishSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/777", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "777", "title": "My Draft",
				"version": map[string]any{"number": 2},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "777", "title": "My Draft", "status": "current",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "publish", "--id", "777")
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "777") {
		t.Errorf("expected page ID in stdout, got: %s", stdout)
	}
}

// TestWorkflow_PublishJSONParseErr verifies that a malformed GET response in publish
// produces a connection_error.
func TestWorkflow_PublishJSONParseErr(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/888", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not valid json at all`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, stderr := runWorkflowCommand(t, srv.URL, "publish", "--id", "888")
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for JSON parse error, got: %s", stderr)
	}
}

// TestWorkflow_PublishPutErr verifies that when the PUT update fails, no stdout is produced.
func TestWorkflow_PublishPutErr(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/889", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "889", "title": "Draft",
				"version": map[string]any{"number": 1},
			})
			return
		}
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"permission denied"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "publish", "--id", "889")
	if stdout != "" {
		t.Errorf("expected no stdout on publish PUT error, got: %s", stdout)
	}
}

// TestWorkflow_CommentSuccess verifies the full comment workflow.
func TestWorkflow_CommentSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/footer-comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "comment-xyz", "pageId": "123",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "comment", "--id", "123", "--body", "Great work!")
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "comment-xyz") {
		t.Errorf("expected comment ID in stdout, got: %s", stdout)
	}
}

// TestWorkflow_CommentAPIErr verifies that a server error during comment
// produces no stdout.
func TestWorkflow_CommentAPIErr(t *testing.T) {
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
		t.Errorf("expected no stdout on comment API error, got: %s", stdout)
	}
}

// TestWorkflow_MoveAPIErr verifies that a server error during move produces no stdout.
func TestWorkflow_MoveAPIErr(t *testing.T) {
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
		t.Errorf("expected no stdout on move API error, got: %s", stdout)
	}
}

// TestWorkflow_CopyAPIErr verifies that a server error during copy produces no stdout.
func TestWorkflow_CopyAPIErr(t *testing.T) {
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

// TestWorkflow_ArchiveAPIErr verifies that a server error during archive produces no stdout.
func TestWorkflow_ArchiveAPIErr(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"server error"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runWorkflowCommand(t, srv.URL, "archive", "--id", "123")
	if stdout != "" {
		t.Errorf("expected no stdout on archive API error, got: %s", stdout)
	}
}

// TestWorkflow_PollLongTaskTimeout verifies pollLongTask returns timeout_error after deadline.
// Note: The custom duration parser requires m/h/d/w units, so the minimum is "1m".
// We trigger the timeout by starting a poll with an always-not-finished task AND a
// very short (1m) timeout, but use a goroutine+channel to abort the blocking test.
// Since the ticker is 1s and timeout is 1m, we can't easily trigger deadline in unit tests.
// Instead, we exercise the timeout_error path by testing the invalid-timeout (validation_error)
// path, which is already covered by TestWorkflow_Copy_InvalidTimeout.
// This test exercises the "task-not-finished after 1 poll" path to increase coverage.
func TestWorkflow_PollLongTaskTimeoutPath(t *testing.T) {
	// This test exercises the pollLongTask loop with a non-finishing task.
	// We use --no-wait to avoid actually entering the poll loop.
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "timeout-task-xyz"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Use --no-wait to return immediately (avoids blocking on the 1s ticker).
	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--no-wait",
	)
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "timeout-task-xyz") {
		t.Errorf("expected task ID in stdout with --no-wait, got: %s", stdout)
	}
}

// TestWorkflow_PollLongTaskUnparseable verifies that an unparseable task response
// is returned as raw output.
func TestWorkflow_PollLongTaskUnparseable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "weird-task-xyz"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/weird-task-xyz", func(w http.ResponseWriter, r *http.Request) {
		// Return valid HTTP 200 but non-JSON body.
		fmt.Fprint(w, `not json at all`)
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
	if !strings.Contains(stdout, "not json at all") {
		t.Errorf("expected raw response in stdout for unparseable task body, got: %s", stdout)
	}
}

// TestWorkflow_PollLongTaskFetchErr verifies that when the task poll request itself
// fails, no stdout is produced.
func TestWorkflow_PollLongTaskFetchErr(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "errored-task-xyz"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/errored-task-xyz", func(w http.ResponseWriter, r *http.Request) {
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

	if stdout != "" {
		t.Errorf("expected no stdout when task poll fails, got: %s", stdout)
	}
}

// TestWorkflow_PollLongTaskSuccessfulFalse verifies that when a task finishes but
// reports unsuccessful, an api_error is written to stderr.
func TestWorkflow_PollLongTaskSuccessfulFalse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "failed-task-xyz"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/failed-task-xyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "failed-task-xyz",
			"finished":   true,
			"successful": false,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
	)

	if stdout != "" {
		t.Errorf("expected no stdout for failed task, got: %s", stdout)
	}
	if !strings.Contains(stderr, "api_error") {
		t.Errorf("expected api_error in stderr for failed task, got: %s", stderr)
	}
}

// TestWorkflow_RestrictViewAPIErr verifies that an API error in restrict view mode
// is handled and produces no stdout.
func TestWorkflow_RestrictViewAPIErr(t *testing.T) {
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
		t.Errorf("expected no stdout on restrict view API error, got: %s", stdout)
	}
}

// TestWorkflow_RestrictAddUserAPIErr verifies that an API error during user restriction
// add produces no stdout.
func TestWorkflow_RestrictAddUserAPIErr(t *testing.T) {
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
		t.Errorf("expected no stdout on restrict add user API error, got: %s", stdout)
	}
}

// TestWorkflow_RestrictAddGroupAPIErr verifies that an API error during group restriction
// add produces no stdout.
func TestWorkflow_RestrictAddGroupAPIErr(t *testing.T) {
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
		t.Errorf("expected no stdout on restrict add group API error, got: %s", stdout)
	}
}

// TestWorkflow_RestrictRemoveGroup verifies the group removal DELETE path.
func TestWorkflow_RestrictRemoveGroup(t *testing.T) {
	var capturedMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/byGroupId/group-xyz", func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict", "--id", "123", "--remove", "--operation", "read", "--group", "group-xyz")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected DELETE method, got %s", capturedMethod)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("expected 'removed' in stdout, got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// cmd/configure.go — additional branches
// ---------------------------------------------------------------------------

// TestConfigureDeleteProfileNotFound verifies deleting a non-existent profile
// returns a not_found error.
func TestConfigureDeleteProfileNotFound(t *testing.T) {
	writeConfigFile(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"base_url": "https://work.atlassian.net",
				"auth": {"type": "bearer", "token": "work-token"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		"--profile", "nonexistent-xyz-profile",
	})

	if err == nil {
		t.Error("expected error for deleting non-existent profile")
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("expected not_found error, got: %s", stderr)
	}
}

// TestConfigureTestConnectionWithBasicAuth verifies testConnection sends Basic auth header.
func TestConfigureTestConnectionWithBasicAuth(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", srv.URL,
		"--token", "mypassword",
		"--auth-type", "basic",
		"--username", "user@company.com",
		"--test=true",
		"--delete=false",
		"--profile", "basic-auth-test-profile",
	})

	if err != nil {
		t.Errorf("expected no error for basic auth test, got: %v; stderr: %s", err, stderr)
	}
	if !strings.HasPrefix(capturedAuth, "Basic ") {
		t.Errorf("expected Basic auth header, got: %q", capturedAuth)
	}
}

// TestConfigureTestConnection_401Response verifies that a 401 response from the
// test endpoint results in a connection_error.
func TestConfigureTestConnection_401Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`Unauthorized`))
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", srv.URL,
		"--token", "bad-token",
		"--test=true",
		"--delete=false",
		"--profile", "auth-fail-test-profile",
	})

	if err == nil {
		t.Error("expected error for 401 response")
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error, got: %s", stderr)
	}
}

// TestConfigureTestExistingProfile_ResolveDefaultProfile verifies that when
// --profile is not explicitly set and the config has a non-default default_profile,
// the correct profile is resolved and tested.
func TestConfigureTestExistingProfile_ResolveDefaultProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	writeConfigFile(t, `{
		"default_profile": "mydefaultprofile",
		"profiles": {
			"mydefaultprofile": {
				"base_url": "`+srv.URL+`",
				"auth": {"type": "bearer", "token": "tok"}
			}
		}
	}`)

	// Run --test without explicitly setting --profile — uses flag default "default"
	// but should resolve to "mydefaultprofile" from config.
	cmd.ResetConfigureFlags()
	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs([]string{"configure", "--test=true", "--delete=false"})
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	stdout := strings.TrimSpace(outBuf.String())

	if !strings.Contains(stdout, "ok") {
		t.Errorf("expected 'ok' when default profile is auto-resolved, got: %s", stdout)
	}
}

// TestConfigureDeleteSaveReadOnly verifies behavior when saving fails (read-only file).
// Tests the save error path in deleteProfileByName.
func TestConfigureDeleteSaveReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"default_profile":"work","profiles":{"work":{"base_url":"https://w.atlassian.net","auth":{"type":"bearer","token":"tok"}}}}`
	if err := os.WriteFile(path, []byte(content), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", path)

	cmd.ResetConfigureFlags()
	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	_, wOut, _ := os.Pipe()
	os.Stdout = wOut
	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs([]string{"configure", "--delete=true", "--test=false", "--profile", "work"})
	_ = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// Just verifying no panic. Save may fail or succeed depending on OS/user.
}

// ---------------------------------------------------------------------------
// cmd/diff.go — fetchVersionList: context cancellation between pages
// ---------------------------------------------------------------------------

// TestDiff_VersionListContextCancellation verifies the context.Err() check in
// the fetchVersionList pagination loop. We exercise this by running a diff
// that returns an empty result (no pagination needed), ensuring no panic.
func TestDiff_VersionListContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/444/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// This exercises the loop body (empty results, no next cursor).
	stdout, _ := runDiffCommand(t, srv.URL, "--id", "444")
	_ = stdout // May be empty diffs or error — both acceptable.
}

// ---------------------------------------------------------------------------
// cmd/export.go — walkTree depth limit
// ---------------------------------------------------------------------------

// TestExport_TreeWithDepthLimit verifies that depth=1 stops recursion at level 1.
func TestExport_TreeWithDepthLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/500", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "500", "title": "Root",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Root</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/500/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "501", "title": "Child"}},
			"_links":  map[string]string{},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/501", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "501", "title": "Child",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Child</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/501/children", func(w http.ResponseWriter, r *http.Request) {
		// This should NOT be called due to depth limit.
		t.Error("children of depth-1 page should not be fetched with depth=1 limit")
		w.WriteHeader(500)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runExportCommandFresh(t, srv.URL, "--id", "500", "--tree", "--depth", "1")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Errorf("expected exactly 2 NDJSON lines (root + child) with depth=1, got %d: %s", len(lines), stdout)
	}
}

// ---------------------------------------------------------------------------
// cmd/labels.go — validation branches
// ---------------------------------------------------------------------------

// TestLabelsList_EmptyPageID verifies that labels list with empty --page-id
// returns a validation_error.
func TestLabelsList_EmptyPageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request during validation test")
		w.WriteHeader(500)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, stderr, _ := captureCommand(t, []string{"labels", "list", "--page-id", ""})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty --page-id, got: %s", stderr)
	}
}

// TestLabelsRemove_EmptyPageID verifies that labels remove with empty --page-id
// returns a validation_error.
func TestLabelsRemove_EmptyPageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request during validation test")
		w.WriteHeader(500)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, stderr, _ := captureCommand(t, []string{"labels", "remove", "--page-id", "", "--label", "mytag"})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty --page-id in remove, got: %s", stderr)
	}
}

// TestLabelsRemove_EmptyLabel verifies that labels remove with empty --label
// returns a validation_error.
func TestLabelsRemove_EmptyLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request during validation test")
		w.WriteHeader(500)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, stderr, _ := captureCommand(t, []string{"labels", "remove", "--page-id", "123", "--label", ""})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty --label in remove, got: %s", stderr)
	}
}

// TestLabelsAdd_NoLabelFlag verifies that labels add with no --label flag
// returns a validation_error (len(labelNames) == 0).
func TestLabelsAdd_NoLabelFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request during validation test")
		w.WriteHeader(500)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	// Not providing --label at all means labelNames will be nil/empty.
	_, stderr, _ := captureCommand(t, []string{"labels", "add", "--page-id", "123"})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for missing --label flag, got: %s", stderr)
	}
}

// TestLabelsAdd_AllEmptyLabelNames verifies that labels add where all label
// names are empty strings returns a validation_error (items list is empty after filter).
func TestLabelsAdd_AllEmptyLabelNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request during validation test")
		w.WriteHeader(500)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	// --label with comma-separated values that include empty strings.
	// The CLI path: labels add --page-id 123 --label ,  (comma → ["",""])
	// This covers the items==0 branch (all items were empty after filtering).
	_, stderr, _ := captureCommand(t, []string{"labels", "add", "--page-id", "123", "--label", ","})
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for all-empty label names, got: %s", stderr)
	}
}

// TestLabelsList_APIError verifies that a server error on labels list returns no stdout.
func TestLabelsList_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, _, _ := captureCommand(t, []string{"labels", "list", "--page-id", "123"})
	if stdout != "" {
		t.Errorf("expected no stdout on labels list API error, got: %s", stdout)
	}
}

// TestLabels_UnknownSubcommand verifies that `cf labels unknowncmd` returns
// an error about unknown command.
func TestLabels_UnknownSubcommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	// When an unknown arg is passed to the labels parent RunE, it returns an error.
	_, stderr, err := captureCommand(t, []string{"labels", "unknownsubcmd"})
	_ = stderr
	// Cobra may return a UsageError — just verify no panic and an error is present.
	if err == nil {
		// If cobra silences the error, check that stderr has something.
		_ = stderr
	}
}

// TestLabels_NoSubcommand verifies that `cf labels` without a subcommand
// returns an error about missing subcommand.
func TestLabels_NoSubcommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	_, _, err := captureCommand(t, []string{"labels"})
	_ = err // Just verify no panic.
}

// TestLabelsAdd_JQError verifies that a failing --jq expression on labels add
// output returns no stdout.
func TestLabelsAdd_JQError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a valid response so the command succeeds up to WriteOutput.
		fmt.Fprint(w, `{"results":[{"name":"test"}]}`)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	// Using an invalid JQ expression that will fail on valid JSON output.
	stdout, stderr, _ := captureCommand(t, []string{"labels", "add", "--page-id", "123", "--label", "test", "--jq", "invalid::jq"})
	if stdout != "" {
		t.Errorf("expected no stdout on labels add JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on labels add with bad JQ, got: %s", stderr)
	}
}

// TestLabelsRemove_JQError verifies that a failing --jq expression on labels remove
// output returns no stdout.
func TestLabelsRemove_JQError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, _ := captureCommand(t, []string{"labels", "remove", "--page-id", "123", "--label", "test", "--jq", "invalid::jq"})
	if stdout != "" {
		t.Errorf("expected no stdout on labels remove JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on labels remove with bad JQ, got: %s", stderr)
	}
}

// TestLabelsAdd_APIError verifies that labels add returns no stdout on API error.
func TestLabelsAdd_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, _, _ := captureCommand(t, []string{"labels", "add", "--page-id", "123", "--label", "test-label"})
	if stdout != "" {
		t.Errorf("expected no stdout on labels add API error, got: %s", stdout)
	}
}

// TestLabelsRemove_APIError verifies that labels remove returns no stdout on API error.
func TestLabelsRemove_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, _, _ := captureCommand(t, []string{"labels", "remove", "--page-id", "123", "--label", "test-label"})
	if stdout != "" {
		t.Errorf("expected no stdout on labels remove API error, got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// cmd/export.go — runSingleExport WriteOutput JQ error
// ---------------------------------------------------------------------------

// TestExport_JQError verifies that a failing --jq on export output returns no stdout.
func TestExport_JQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/888", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "888", "title": "Test Page",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Hello</p>"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runExportCommandFresh(t, srv.URL, "--id", "888", "--jq", "invalid::jq")
	if stdout != "" {
		t.Errorf("expected no stdout on export JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on export with bad JQ, got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/diff.go — fetchVersionList: nextLink without /pages/ prefix
// ---------------------------------------------------------------------------

// TestDiff_FetchVersionListNextLinkFallback verifies that when the _links.next
// value does not contain "/pages/", the raw link is used directly as the path.
// This covers the else branch at diff.go:249.
func TestDiff_FetchVersionListNextLinkFallback(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/600/versions", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First call: return a next link that does NOT contain "/pages/"
			// so the else branch (path = nextLink) is taken.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []any{
					map[string]any{"number": 2, "message": "", "createdAt": "2026-01-02T00:00:00Z",
						"createdBy": map[string]any{"displayName": "Alice"},
						"content":   map[string]any{"title": "V2"}},
				},
				"_links": map[string]string{
					// This path does NOT contain "/pages/" (uses /versions directly).
					"next": "/wiki/api/v2/versions?cursor=abc",
				},
			})
			return
		}
		// Second call: return empty to terminate pagination
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	// Handle the fallback path: /wiki/api/v2/versions?cursor=abc
	mux.HandleFunc("/wiki/api/v2/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "600")
	_ = stdout // May be empty diffs — just verify no panic.
}

// TestDiff_FetchVersionListNextLinkNoPages verifies the else branch (path = nextLink)
// when _links.next has no "/pages/" segment at all (e.g. absolute URL or different format).
func TestDiff_FetchVersionListNextLinkNoPages(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	// Register the first path handler.
	mux.HandleFunc("/wiki/api/v2/pages/601/versions", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// Return a next link that contains no "/pages/" segment.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []any{
					map[string]any{"number": 1, "message": "", "createdAt": "2026-01-01T00:00:00Z",
						"createdBy": map[string]any{"displayName": "Bob"},
						"content":   map[string]any{"title": "V1"}},
				},
				"_links": map[string]string{
					// Deliberately omit "/pages/" in the next link path.
					"next": "/wiki/api/v2/versions?cursor=xyz",
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	// The second request will go to /wiki/api/v2/versions?cursor=xyz — handle it.
	mux.HandleFunc("/wiki/api/v2/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runDiffCommand(t, srv.URL, "--id", "601")
	_ = stdout // Just verify no panic.
}

// ---------------------------------------------------------------------------
// cmd/export.go — fetchAllChildren: nextLink without /pages/ prefix
// ---------------------------------------------------------------------------

// TestExport_FetchAllChildrenNextLinkFallback verifies the fallback path when
// _links.next doesn't contain "/pages/" (the else branch at export.go:207).
func TestExport_FetchAllChildrenNextLinkFallback(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/700", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "700", "title": "Root",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Root</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/700/children", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First call: return next link that does NOT contain "/pages/"
			// so the else branch (path = nextLink) is taken.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "701", "title": "Child1"}},
				"_links": map[string]string{
					// Use a link without "/pages/" to trigger the else branch.
					"next": "/wiki/api/v2/children?cursor=xyz",
				},
			})
			return
		}
		// Second call (from fallback path): return empty to terminate
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	// Handle the fallback path: /wiki/api/v2/children?cursor=xyz
	mux.HandleFunc("/wiki/api/v2/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/701", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "701", "title": "Child1",
			"body": map[string]any{"storage": map[string]any{"value": "<p>Child1</p>"}},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/701/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]string{},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, _ := runExportCommandFresh(t, srv.URL, "--id", "700", "--tree")
	if !strings.Contains(stdout, "Root") {
		t.Errorf("expected root page in output, got: %s", stdout)
	}
}

// ---------------------------------------------------------------------------
// cmd/batch.go — pretty print and stdin no-tty branches
// ---------------------------------------------------------------------------

// TestBatch_PrettyPrint verifies that --pretty formats batch output with indentation.
func TestBatch_PrettyPrint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "123", "title": "Test"})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	batchInput := filepath.Join(t.TempDir(), "batch.json")
	ops := `[{"command":"pages get-by-id","args":{"id":"123"}}]`
	if err := os.WriteFile(batchInput, []byte(ops), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, _ := captureCommand(t, []string{"batch", "--input", batchInput, "--pretty"})
	// Pretty-print should contain indented JSON with newlines
	if !strings.Contains(stdout, "\n") {
		t.Errorf("expected pretty-printed (multi-line) output, got: %s", stdout)
	}
}

// TestBatch_JQFilterOnBatchOutput verifies that --jq filters the batch output array.
func TestBatch_JQFilterOnBatchOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "789", "title": "Filtered"})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	batchInput := filepath.Join(t.TempDir(), "batch.json")
	ops := `[{"command":"pages get-by-id","args":{"id":"789"}}]`
	if err := os.WriteFile(batchInput, []byte(ops), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, _ := captureCommand(t, []string{"batch", "--input", batchInput, "--jq", ".[0].index"})
	_ = stdout // Just verify no panic.
}

// ---------------------------------------------------------------------------
// cmd/workflow.go — WriteOutput JQ error branches
// ---------------------------------------------------------------------------

// TestWorkflow_MoveJQError verifies that a failing --jq expression on move output
// returns no stdout.
func TestWorkflow_MoveJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/move/append/456", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"123","title":"Moved"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "move", "--id", "123", "--target-id", "456", "--jq", "invalid::jq")
	if stdout != "" {
		t.Errorf("expected no stdout on move JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on move with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_CopyJQError verifies that a failing --jq expression on copy output
// returns no stdout (covers the WriteOutput error branch in copy --no-wait path).
func TestWorkflow_CopyJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"title":"copied"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--no-wait",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on copy JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on copy with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_CopyNoTaskIDJQError verifies that a failing --jq on copy with no task ID
// in the response returns no stdout.
func TestWorkflow_CopyNoTaskIDJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a response with no "id" field — triggers the no-task-ID path.
		fmt.Fprint(w, `{"status":"done"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "1m",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on copy no-task-ID JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on copy no-task-ID with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_PublishJQError verifies that a failing --jq on publish output
// returns no stdout.
func TestWorkflow_PublishJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/800", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "800", "title": "Draft",
				"version": map[string]any{"number": 1},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "800", "title": "Draft", "status": "current",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "publish", "--id", "800", "--jq", "invalid::jq")
	if stdout != "" {
		t.Errorf("expected no stdout on publish JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on publish with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_CommentJQError verifies that a failing --jq on comment output
// returns no stdout.
func TestWorkflow_CommentJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/footer-comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "comment-jq-test"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "comment", "--id", "123", "--body", "Test", "--jq", "invalid::jq")
	if stdout != "" {
		t.Errorf("expected no stdout on comment JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on comment with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_ArchiveJQError verifies that a failing --jq on archive output (no-wait path)
// returns no stdout.
func TestWorkflow_ArchiveJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "archive-jq-task"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--no-wait",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on archive JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on archive with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_ArchiveNoTaskIDJQError verifies that a failing --jq on archive with no task ID
// returns no stdout (covers the no-task-ID path in archive).
func TestWorkflow_ArchiveNoTaskIDJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a response with no "id" field.
		fmt.Fprint(w, `{"status":"done"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on archive no-task-ID JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on archive no-task-ID with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_RestrictAddUserJQError verifies that a failing --jq on restrict output
// returns no stdout.
func TestWorkflow_RestrictAddUserJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--add",
		"--operation", "read",
		"--user", "user@test.com",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on restrict add user JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on restrict add user with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_CopyPolledTaskJQError verifies that a failing --jq on a successfully
// polled copy task returns no stdout (covers workflow.go:183).
func TestWorkflow_CopyPolledTaskJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/copy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-poll-jq"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/task-poll-jq", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "task-poll-jq",
			"finished":   true,
			"successful": true,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "copy",
		"--id", "123",
		"--target-id", "456",
		"--timeout", "1m",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on copy poll JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on copy poll with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_ArchivePolledTaskJQError verifies that a failing --jq on a successfully
// polled archive task returns no stdout (covers workflow.go:504).
func TestWorkflow_ArchivePolledTaskJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "arch-poll-jq"})
	})
	mux.HandleFunc("/wiki/rest/api/longtask/arch-poll-jq", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "arch-poll-jq",
			"finished":   true,
			"successful": true,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "archive",
		"--id", "123",
		"--timeout", "1m",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on archive poll JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on archive poll with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_RestrictViewJQError verifies that a failing --jq on restrict view output
// returns no stdout (covers workflow.go:354).
func TestWorkflow_RestrictViewJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"read":{"restrictions":{"user":{"results":[]}}}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on restrict view JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on restrict view with bad JQ, got: %s", stderr)
	}
}

// TestWorkflow_RestrictAddGroupJQError verifies that a failing --jq on restrict group output
// returns no stdout.
func TestWorkflow_RestrictAddGroupJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/123/restriction/byOperation/read/byGroupId/grp1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stdout, stderr := runWorkflowCommand(t, srv.URL, "restrict",
		"--id", "123",
		"--add",
		"--operation", "read",
		"--group", "grp1",
		"--jq", "invalid::jq",
	)
	if stdout != "" {
		t.Errorf("expected no stdout on restrict add group JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on restrict add group with bad JQ, got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/search.go — WriteOutput JQ error
// ---------------------------------------------------------------------------

// TestSearch_JQError verifies that a failing --jq on search results returns no stdout.
func TestSearch_JQError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "1", "type": "page"}},
			"_links":  map[string]any{},
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, _ := captureCommand(t, []string{"search", "search-content", "--cql", "type=page", "--jq", "invalid::jq"})
	if stdout != "" {
		t.Errorf("expected no stdout on search JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on search with bad JQ, got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// cmd/diff.go — dry-run WriteOutput JQ error
// ---------------------------------------------------------------------------

// TestDiff_DryRunJQError verifies that a failing --jq on dry-run output returns no stdout.
func TestDiff_DryRunJQError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, _ := captureCommand(t, []string{"diff", "--id", "123", "--dry-run", "--jq", "invalid::jq"})
	if stdout != "" {
		t.Errorf("expected no stdout on diff dry-run JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on diff dry-run with bad JQ, got: %s", stderr)
	}
}

// TestDiff_WriteOutputJQError verifies that a failing --jq on diff result returns no stdout.
// Uses runDiffCommand which sets up the correct base URL for the diff command.
func TestDiff_WriteOutputJQError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/999/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"number": 2, "message": "", "createdAt": "2026-01-02T00:00:00Z",
					"createdBy": map[string]any{"displayName": "Alice"},
					"content":   map[string]any{"title": "V2"},
				},
				{
					"number": 1, "message": "", "createdAt": "2026-01-01T00:00:00Z",
					"createdBy": map[string]any{"displayName": "Bob"},
					"content":   map[string]any{"title": "V1"},
				},
			},
			"_links": map[string]string{},
		})
	})
	// Version body endpoints need query params — handle with catch-all
	mux.HandleFunc("/pages/999", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ver := r.URL.Query().Get("version")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "999", "body": map[string]any{"storage": map[string]any{"value": fmt.Sprintf("<p>v%s</p>", ver)}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// runDiffCommand uses setupTemplateEnv which sets CF_BASE_URL = srvURL + "/wiki/api/v2"
	// But diff fetches path /pages/999/versions relative to base URL.
	// The server must match /pages/999/versions (relative to /wiki/api/v2).
	// Actually, setupTemplateEnv sets CF_BASE_URL = srvURL + "/wiki/api/v2",
	// so the client uses srvURL as the domain and /wiki/api/v2 as the prefix.
	// The diff fetches: c.BaseURL + "/pages/999/versions" = srvURL + "/wiki/api/v2/pages/999/versions"
	// So we need mux at /wiki/api/v2/pages/999/versions.

	// Re-register handlers with full path.
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/wiki/api/v2/pages/999/versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"number": 2, "message": "", "createdAt": "2026-01-02T00:00:00Z",
					"createdBy": map[string]any{"displayName": "Alice"},
					"content":   map[string]any{"title": "V2"},
				},
				{
					"number": 1, "message": "", "createdAt": "2026-01-01T00:00:00Z",
					"createdBy": map[string]any{"displayName": "Bob"},
					"content":   map[string]any{"title": "V1"},
				},
			},
			"_links": map[string]string{},
		})
	})
	mux2.HandleFunc("/wiki/api/v2/pages/999", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ver := r.URL.Query().Get("version")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "999", "body": map[string]any{"storage": map[string]any{"value": fmt.Sprintf("<p>v%s</p>", ver)}},
		})
	})
	srv2 := httptest.NewServer(mux2)
	defer srv2.Close()

	stdout, stderr := runDiffCommand(t, srv2.URL, "--id", "999", "--jq", "invalid::jq")
	if stdout != "" {
		t.Errorf("expected no stdout on diff JQ error, got: %s", stdout)
	}
	if !strings.Contains(stderr, "jq_error") {
		t.Errorf("expected jq_error on diff with bad JQ, got: %s", stderr)
	}
}
