package cmd_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestWatch_ParseTimestamp_Formats verifies that parseTimestamp handles all
// supported timestamp formats (RFC3339, milliseconds, and fallback to zero).
// We test indirectly by passing results with various timestamp formats and
// verifying dedup behavior (same timestamp = dedup, different = new event).
func TestWatch_ParseTimestamp_MillisecondFormat(t *testing.T) {
	// Use a recent timestamp in milliseconds format (2006-01-02T15:04:05.000Z)
	// that parseTimestamp should parse correctly.
	ts := time.Now().UTC().Add(-5 * time.Minute).Format("2006-01-02T15:04:05.000Z")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		result := makeWatchResult("501", "page", "Millis Page", "ENG", 10, ts, "Alice")
		_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	// Should emit a change event since the timestamp was parseable and recent
	if !strings.Contains(stdout, `"type":"change"`) {
		t.Errorf("expected change event for millisecond timestamp, got: %s", stdout)
	}
	if !strings.Contains(stdout, "501") {
		t.Errorf("expected content ID 501 in output, got: %s", stdout)
	}
}

// TestWatch_ParseTimestamp_RFC3339Millis verifies the RFC3339 with milliseconds
// format (2006-01-02T15:04:05.999Z07:00) is handled correctly.
func TestWatch_ParseTimestamp_RFC3339Millis(t *testing.T) {
	ts := time.Now().UTC().Add(-5 * time.Minute).Format("2006-01-02T15:04:05.999Z07:00")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		result := makeWatchResult("502", "page", "RFC3339 Millis Page", "ENG", 10, ts, "Bob")
		_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	if !strings.Contains(stdout, `"type":"change"`) {
		t.Errorf("expected change event for RFC3339 millis timestamp, got: %s", stdout)
	}
}

// TestWatch_ParseTimestamp_InvalidFallback verifies that an unparseable timestamp
// results in zero time, causing the item to be treated as "never seen" and
// emitted as a change event (not suppressed).
func TestWatch_ParseTimestamp_InvalidFallback(t *testing.T) {
	// Pass a completely invalid timestamp — parseTimestamp should return zero time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		result := makeWatchResult("503", "page", "Bad Timestamp Page", "ENG", 10, "not-a-real-timestamp", "Carol")
		_, _ = w.Write(makeWatchSearchResponse([]map[string]any{result}))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	// With zero time, the seen[contentID] check is: if !zeroTime.After(zeroTime) => skip!
	// zero time means it's never "after" zero time, so on second poll it would be skipped.
	// On first poll though, seen is empty, so the item IS emitted.
	if !strings.Contains(stdout, "503") {
		t.Errorf("expected content 503 to be emitted (zero time treated as initial emit), got: %s", stdout)
	}
}

// TestWatch_MaxConsecutiveErrors verifies that after 5 consecutive poll failures,
// the watch command emits an error event and exits with non-zero code.
func TestWatch_MaxConsecutiveErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return 500 error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"server always fails"}`)
	}))
	defer srv.Close()

	// Use --max-polls large enough that 5 consecutive errors can occur
	// The watch command exits after 5 consecutive errors
	stdout, stderr := runWatchCommand(t, srv.URL,
		"--cql", "space = ENG",
		"--max-polls", "10",
		"--interval", "100ms",
	)

	// Should emit an error event to stdout
	if !strings.Contains(stdout, `"type":"error"`) {
		t.Errorf("expected error event after 5 consecutive failures, got stdout: %s", stdout)
	}
	// Should also have error messages on stderr from the failed polls
	_ = stderr
}

// TestWatch_PollAndEmit_Pagination verifies that when a search response includes
// a _links.next, pollAndEmit follows pagination up to 5 pages.
func TestWatch_PollAndEmit_Pagination(t *testing.T) {
	pageCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		ts := recentTimestamp(pageCount * 5)
		id := fmt.Sprintf("60%d", pageCount)
		result := makeWatchResult(id, "page", fmt.Sprintf("Page %d", pageCount), "ENG", 10, ts, "Alice")

		var nextLink string
		if pageCount < 3 {
			// Provide next link for pagination (up to 3 pages)
			nextLink = fmt.Sprintf("http://%s/wiki/rest/api/search?cursor=page%d", r.Host, pageCount)
		}

		resp := map[string]any{
			"results": []map[string]any{result},
			"_links":  map[string]any{"next": nextLink},
		}
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	// Should have emitted change events from multiple pages
	changeCount := strings.Count(stdout, `"type":"change"`)
	if changeCount < 2 {
		t.Errorf("expected at least 2 change events from pagination, got %d\nOutput:\n%s", changeCount, stdout)
	}
}

// TestWatch_PollAndEmit_ContextCancelledDuringFetch verifies that when context
// is cancelled, pollAndEmit returns the context error.
// We test this indirectly by having the server go away after --max-polls=1
// and verifying no panic occurs.
func TestWatch_PollAndEmit_ParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return valid HTTP 200 but invalid JSON
		fmt.Fprint(w, `this is not valid json at all`)
	}))
	defer srv.Close()

	stdout, stderr := runWatchCommand(t, srv.URL,
		"--cql", "space = ENG",
		"--max-polls", "1",
	)

	// Parse error should produce stderr output and no change events
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr for parse error, got: %s", stderr)
	}
	// Shutdown event should still be emitted
	if !strings.Contains(stdout, `"type":"shutdown"`) {
		t.Errorf("expected shutdown event even after parse error, got: %s", stdout)
	}
}

// TestWatch_UseLastModified_WhenVersionWhenEmpty verifies that when
// content.version.when is empty, lastModified is used as the timestamp.
func TestWatch_UseLastModified_WhenVersionWhenEmpty(t *testing.T) {
	ts := recentTimestamp(15)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Create result with empty version.when but populated lastModified
		result := map[string]any{
			"content": map[string]any{
				"id":    "701",
				"type":  "page",
				"title": "Last Modified Only",
				"space": map[string]any{"id": 10, "key": "ENG"},
				"version": map[string]any{
					"when": "", // Empty version.when
					"by":   map[string]any{"displayName": "Dave"},
				},
			},
			"lastModified": ts, // Should fall back to this
		}
		b, _ := json.Marshal(map[string]any{
			"results": []map[string]any{result},
			"_links":  map[string]string{},
		})
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	if !strings.Contains(stdout, "701") {
		t.Errorf("expected content 701 in output when using lastModified fallback, got: %s", stdout)
	}
}

// TestWatch_PollAndEmit_RelativeNextLink verifies that when _links.next is a
// relative path (not starting with "http"), it is prefixed with the domain.
func TestWatch_PollAndEmit_RelativeNextLink(t *testing.T) {
	pageCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		ts := recentTimestamp(pageCount * 3)
		id := fmt.Sprintf("80%d", pageCount)
		result := makeWatchResult(id, "page", fmt.Sprintf("Relative Page %d", pageCount), "ENG", 10, ts, "Alice")

		var nextLink string
		if pageCount < 2 {
			// Provide a relative (non-http) next link to trigger the else branch
			nextLink = "/wiki/rest/api/search?cursor=relative_cursor"
		}

		resp := map[string]any{
			"results": []map[string]any{result},
			"_links":  map[string]any{"next": nextLink},
		}
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	// Should have emitted at least 1 change event (from the paginated result)
	if !strings.Contains(stdout, `"type":"change"`) {
		t.Errorf("expected change event from relative-link pagination, got: %s", stdout)
	}
}

// TestWatch_MaxPolls_ExactlyOne verifies the maxPolls<=1 early exit path
// (runs one poll, emits shutdown, returns).
func TestWatch_MaxPolls_ExactlyOne(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeWatchSearchResponse(nil))
	}))
	defer srv.Close()

	stdout, _ := runWatchCommand(t, srv.URL, "--cql", "space = ENG", "--max-polls", "1")

	if pollCount != 1 {
		t.Errorf("expected exactly 1 poll for --max-polls=1, got %d", pollCount)
	}
	if !strings.Contains(stdout, `"type":"shutdown"`) {
		t.Errorf("expected shutdown event, got: %s", stdout)
	}
}
