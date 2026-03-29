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
	"time"

	"github.com/sofq/confluence-cli/cmd"
)

// recentTimestamp returns a timestamp string in the format expected by
// Confluence, set to the given number of minutes ago. This ensures test
// timestamps fall within the 48-hour prune window used by pollAndEmit.
func recentTimestamp(minutesAgo int) string {
	return time.Now().UTC().Add(-time.Duration(minutesAgo) * time.Minute).Format("2006-01-02T15:04:05.000Z")
}

// makeWatchSearchResponse builds a v1 search response JSON with the given results.
func makeWatchSearchResponse(results []map[string]any) []byte {
	resp := map[string]any{
		"results": results,
		"_links":  map[string]any{},
	}
	b, _ := json.Marshal(resp)
	return b
}

// makeWatchResult builds a single v1 search result entry.
func makeWatchResult(id, typ, title, spaceKey string, spaceID int, when, modifier string) map[string]any {
	return map[string]any{
		"content": map[string]any{
			"id":    id,
			"type":  typ,
			"title": title,
			"space": map[string]any{
				"id":  spaceID,
				"key": spaceKey,
			},
			"version": map[string]any{
				"when": when,
				"by": map[string]any{
					"displayName": modifier,
				},
			},
		},
		"lastModified": when,
	}
}

// runWatchCommand executes `cf watch` with the given args against the test server,
// using a short interval and capturing stdout/stderr. The command runs until the
// server handler closes, which should be controlled by the test.
func runWatchCommand(t *testing.T, srvURL string, extraArgs ...string) (stdout string, stderr string) {
	t.Helper()

	t.Setenv("CF_BASE_URL", srvURL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	args := append([]string{"watch"}, extraArgs...)
	root.SetArgs(args)
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

// TestWatch_PollAndEmit_TwoResults verifies that pollAndEmit with 2 search results
// emits 2 NDJSON change events to stdout.
func TestWatch_PollAndEmit_TwoResults(t *testing.T) {
	ts1 := recentTimestamp(30) // 30 min ago
	ts2 := recentTimestamp(20) // 20 min ago
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		w.Header().Set("Content-Type", "application/json")
		if pollCount == 1 {
			results := []map[string]any{
				makeWatchResult("101", "page", "Page One", "ENG", 10, ts1, "Alice"),
				makeWatchResult("102", "blogpost", "Blog Two", "ENG", 10, ts2, "Bob"),
			}
			_, _ = w.Write(makeWatchSearchResponse(results))
		} else {
			// Return empty on subsequent polls to avoid infinite loop;
			// The test uses --max-polls=1 to stop after one poll.
			_, _ = w.Write(makeWatchSearchResponse(nil))
		}
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// Filter out shutdown line
	var changeLines []string
	for _, line := range lines {
		if strings.Contains(line, `"type":"change"`) {
			changeLines = append(changeLines, line)
		}
	}
	if len(changeLines) != 2 {
		t.Fatalf("expected 2 change events, got %d\nOutput:\n%s", len(changeLines), stdout)
	}

	// Verify first event structure
	var event map[string]string
	if err := json.Unmarshal([]byte(changeLines[0]), &event); err != nil {
		t.Fatalf("failed to parse change event: %v", err)
	}
	if event["type"] != "change" {
		t.Errorf("expected type=change, got %s", event["type"])
	}
	if event["id"] != "101" {
		t.Errorf("expected id=101, got %s", event["id"])
	}
	if event["contentType"] != "page" {
		t.Errorf("expected contentType=page, got %s", event["contentType"])
	}
	if event["title"] != "Page One" {
		t.Errorf("expected title=Page One, got %s", event["title"])
	}
	if event["modifier"] != "Alice" {
		t.Errorf("expected modifier=Alice, got %s", event["modifier"])
	}
}

// TestWatch_Dedup_SameResults verifies that when the same results appear on a
// second poll, nothing new is emitted (dedup via seen map).
func TestWatch_Dedup_SameResults(t *testing.T) {
	ts := recentTimestamp(15) // 15 min ago — within 48h prune window
	result := makeWatchResult("201", "page", "Stable Page", "ENG", 10, ts, "Alice")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "2", "--interval", "100ms")

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var changeLines []string
	for _, line := range lines {
		if strings.Contains(line, `"type":"change"`) {
			changeLines = append(changeLines, line)
		}
	}
	// Only 1 change event despite 2 polls (dedup)
	if len(changeLines) != 1 {
		t.Fatalf("expected 1 change event (dedup), got %d\nOutput:\n%s", len(changeLines), stdout)
	}
}

// TestWatch_Dedup_UpdatedVersion verifies that when a result has a newer version.when
// on the second poll, exactly 1 new change event is emitted.
func TestWatch_Dedup_UpdatedVersion(t *testing.T) {
	tsOld := recentTimestamp(30) // 30 min ago
	tsNew := recentTimestamp(10) // 10 min ago
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		w.Header().Set("Content-Type", "application/json")
		if pollCount == 1 {
			result := makeWatchResult("301", "page", "Evolving Page", "ENG", 10, tsOld, "Alice")
			_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
		} else {
			// Same content, newer version.when
			result := makeWatchResult("301", "page", "Evolving Page", "ENG", 10, tsNew, "Bob")
			_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
		}
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "2", "--interval", "100ms")

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var changeLines []string
	for _, line := range lines {
		if strings.Contains(line, `"type":"change"`) {
			changeLines = append(changeLines, line)
		}
	}
	if len(changeLines) != 2 {
		t.Fatalf("expected 2 change events (initial + update), got %d\nOutput:\n%s", len(changeLines), stdout)
	}
	// Second event should have the new modifier
	var event map[string]string
	_ = json.Unmarshal([]byte(changeLines[1]), &event)
	if event["modifier"] != "Bob" {
		t.Errorf("expected second event modifier=Bob, got %s", event["modifier"])
	}
}

// TestWatch_HTTPError_ContinuesPolling verifies that an HTTP error writes JSON
// error to stderr and polling continues (doesn't crash).
func TestWatch_HTTPError_ContinuesPolling(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		if pollCount == 1 {
			// First poll: HTTP 500 error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"message":"server error"}`)
		} else {
			// Second poll: success
			w.Header().Set("Content-Type", "application/json")
			result := makeWatchResult("401", "page", "After Error", "ENG", 10, recentTimestamp(5), "Alice")
			_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
		}
	}))
	defer srv.Close()

	stdout, stderr := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "2", "--interval", "100ms")

	// Should have error on stderr
	if !strings.Contains(stderr, "server_error") && !strings.Contains(stderr, "error") {
		t.Errorf("expected error on stderr, got: %s", stderr)
	}

	// Should still emit change event from second poll
	if !strings.Contains(stdout, `"type":"change"`) {
		t.Errorf("expected change event after error recovery, got stdout:\n%s", stdout)
	}
}

// TestWatch_Shutdown_EmitsShutdownEvent verifies that the shutdown path emits
// {"type":"shutdown"} as the final NDJSON line.
func TestWatch_Shutdown_EmitsShutdownEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeWatchSearchResponse(nil))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	if !strings.Contains(stdout, `"type":"shutdown"`) {
		t.Errorf("expected shutdown event in output, got:\n%s", stdout)
	}
}

// TestWatch_EmptyCQL_Rejects verifies that --cql "" is rejected with ExitValidation.
func TestWatch_EmptyCQL_Rejects(t *testing.T) {
	_, stderr := runWatchCommand(t, "http://unused", "--cql", "")

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error on stderr for empty CQL, got:\n%s", stderr)
	}
}

// TestWatch_BuildWatchCQL verifies CQL construction with lastModified filter.
func TestWatch_BuildWatchCQL(t *testing.T) {
	// We test indirectly by checking the CQL sent to the server
	var capturedCQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCQL = r.URL.Query().Get("cql")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeWatchSearchResponse(nil))
	}))
	defer srv.Close()

	runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	// CQL should contain the user query wrapped in parens, AND lastModified >=, ORDER BY
	if !strings.Contains(capturedCQL, "(space = ENG)") {
		t.Errorf("expected CQL to contain (space = ENG), got: %s", capturedCQL)
	}
	if !strings.Contains(capturedCQL, "lastModified >=") {
		t.Errorf("expected CQL to contain lastModified >=, got: %s", capturedCQL)
	}
	if !strings.Contains(capturedCQL, "ORDER BY lastModified DESC") {
		t.Errorf("expected CQL to contain ORDER BY lastModified DESC, got: %s", capturedCQL)
	}
}
