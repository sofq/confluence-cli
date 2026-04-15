package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPreset_AgentOnSingleItem verifies that the built-in "agent" preset
// works on single-item responses (e.g., pages get-by-id) where there is no
// .results[] wrapper.
func TestPreset_AgentOnSingleItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "42",
			"title":   "Test Page",
			"status":  "current",
			"spaceId": "100",
			"version": map[string]any{"number": 3},
			"_links":  map[string]any{"webui": "/pages/42"},
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, err := captureCommand(t, []string{
		"pages", "get-by-id", "--id", "42", "--preset", "agent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}

	// Should produce a valid JSON object with the preset fields.
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nstdout: %s", err, stdout)
	}

	// Verify key fields are present.
	if result["id"] != "42" {
		t.Errorf("expected id=42, got %v", result["id"])
	}
	if result["title"] != "Test Page" {
		t.Errorf("expected title=Test Page, got %v", result["title"])
	}
	if result["status"] != "current" {
		t.Errorf("expected status=current, got %v", result["status"])
	}
}

// TestPreset_AgentOnListResponse verifies that the "agent" preset still works
// on list responses (e.g., pages get) that have a .results[] wrapper.
func TestPreset_AgentOnListResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":      "1",
					"title":   "Page One",
					"status":  "current",
					"spaceId": "100",
					"version": map[string]any{"number": 1},
					"_links":  map[string]any{"webui": "/pages/1"},
				},
				{
					"id":      "2",
					"title":   "Page Two",
					"status":  "current",
					"spaceId": "100",
					"version": map[string]any{"number": 2},
					"_links":  map[string]any{"webui": "/pages/2"},
				},
			},
			"_links": map[string]any{},
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, err := captureCommand(t, []string{
		"pages", "get", "--preset", "agent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}

	// The agent preset on a list should produce a JSON array.
	var results []map[string]any
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("expected valid JSON array, got parse error: %v\nstdout: %s", err, stdout)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0]["title"] != "Page One" {
		t.Errorf("expected first title=Page One, got %v", results[0]["title"])
	}
}

// TestPreset_BriefOnSingleItem verifies the "brief" preset on single-item responses.
func TestPreset_BriefOnSingleItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "42",
			"title":   "Test Page",
			"status":  "current",
			"spaceId": "100",
			"version": map[string]any{"number": 3},
			"body":    map[string]any{"storage": map[string]any{"value": "<p>lots of content</p>"}},
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, err := captureCommand(t, []string{
		"pages", "get-by-id", "--id", "42", "--preset", "brief",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nstdout: %s", err, stdout)
	}

	// Brief should include id, title, status but NOT body or version.
	if result["id"] != "42" {
		t.Errorf("expected id=42, got %v", result["id"])
	}
	if result["title"] != "Test Page" {
		t.Errorf("expected title=Test Page, got %v", result["title"])
	}
	if _, hasBody := result["body"]; hasBody {
		t.Error("brief preset should not include body")
	}
}

// TestPreset_MetaOnSingleItem verifies the "meta" preset on single-item responses.
func TestPreset_MetaOnSingleItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        "42",
			"title":     "Test Page",
			"status":    "current",
			"spaceId":   "100",
			"authorId":  "user-abc",
			"createdAt": "2026-01-01T00:00:00Z",
			"version":   map[string]any{"number": 3},
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, err := captureCommand(t, []string{
		"pages", "get-by-id", "--id", "42", "--preset", "meta",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nstdout: %s", err, stdout)
	}

	if result["authorId"] != "user-abc" {
		t.Errorf("expected authorId=user-abc, got %v", result["authorId"])
	}
	if result["spaceId"] != "100" {
		t.Errorf("expected spaceId=100, got %v", result["spaceId"])
	}
}

// TestPreset_TitlesOnSingleItem verifies the "titles" preset on a single-item response.
func TestPreset_TitlesOnSingleItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "42",
			"title": "My Page Title",
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, err := captureCommand(t, []string{
		"pages", "get-by-id", "--id", "42", "--preset", "titles",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}

	stdout = strings.TrimSpace(stdout)
	if !strings.Contains(stdout, "My Page Title") {
		t.Errorf("expected output to contain title, got: %s", stdout)
	}
}

// TestPreset_StatusIsStringNotObject verifies that presets work when
// the Confluence v2 API returns status as a plain string ("current")
// rather than an object ({current: "current"}).
func TestPreset_StatusIsStringNotObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Confluence v2 API returns status as a plain string.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "42",
			"title":  "Test",
			"status": "current", // string, not object
		})
	}))
	defer srv.Close()
	setupEnvForServer(t, srv.URL)

	stdout, stderr, err := captureCommand(t, []string{
		"pages", "get-by-id", "--id", "42", "--preset", "brief",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}

	// Should not produce a jq error.
	if strings.Contains(stderr, "jq_error") {
		t.Errorf("preset should handle string status without jq error, stderr: %s", stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nstdout: %s", err, stdout)
	}
	if result["status"] != "current" {
		t.Errorf("expected status=current, got %v", result["status"])
	}
}
