package avatar_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sofq/confluence-cli/internal/avatar"
)

func TestBuildProfile_ZeroPages(t *testing.T) {
	profile := avatar.BuildProfile("acc123", "Alice", nil)
	if profile.PageCount != 0 {
		t.Errorf("expected PageCount=0, got %d", profile.PageCount)
	}
	if profile.AccountID != "acc123" {
		t.Errorf("expected AccountID=acc123, got %s", profile.AccountID)
	}
	if profile.DisplayName != "Alice" {
		t.Errorf("expected DisplayName=Alice, got %s", profile.DisplayName)
	}
	if profile.Version != "1" {
		t.Errorf("expected Version=1, got %s", profile.Version)
	}
	// GeneratedAt should be parseable as RFC3339
	if _, err := time.Parse(time.RFC3339, profile.GeneratedAt); err != nil {
		t.Errorf("GeneratedAt %q is not valid RFC3339: %v", profile.GeneratedAt, err)
	}
	// StyleGuide.Writing should be non-empty even with 0 pages
	if profile.StyleGuide.Writing == "" {
		t.Error("expected non-empty StyleGuide.Writing for 0 pages")
	}
}

func TestBuildProfile_OnePage(t *testing.T) {
	pages := []avatar.PageRecord{
		{ID: "1", Title: "Test Page", Body: "Hello world this is content", LastModified: time.Now()},
	}
	profile := avatar.BuildProfile("acc123", "Bob", pages)
	if profile.PageCount != 1 {
		t.Errorf("expected PageCount=1, got %d", profile.PageCount)
	}
}

func TestBuildProfile_FivePages(t *testing.T) {
	pages := make([]avatar.PageRecord, 5)
	for i := range pages {
		body := ""
		for j := 0; j < (i+1)*50; j++ {
			body += "word "
		}
		pages[i] = avatar.PageRecord{
			ID:           "page" + string(rune('0'+i)),
			Title:        "Page " + string(rune('A'+i)),
			Body:         body,
			LastModified: time.Now(),
		}
	}
	profile := avatar.BuildProfile("acc456", "Carol", pages)

	if profile.PageCount != 5 {
		t.Errorf("expected PageCount=5, got %d", profile.PageCount)
	}

	// Examples should be at most 3
	if len(profile.Examples) > 3 {
		t.Errorf("expected at most 3 examples, got %d", len(profile.Examples))
	}
	// Each example text should be at most 303 chars (300 + "...")
	for i, ex := range profile.Examples {
		if len(ex.Text) > 303 {
			t.Errorf("examples[%d].Text length %d exceeds 303", i, len(ex.Text))
		}
	}
}

func TestBuildProfile_JSONMarshallable(t *testing.T) {
	pages := []avatar.PageRecord{
		{ID: "1", Title: "Page", Body: "content here", LastModified: time.Now()},
	}
	profile := avatar.BuildProfile("acc789", "Dave", pages)

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("marshaled JSON is empty")
	}

	// Should contain required top-level fields
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	requiredFields := []string{"version", "account_id", "display_name", "generated_at", "page_count", "writing", "style_guide"}
	for _, f := range requiredFields {
		if _, ok := m[f]; !ok {
			t.Errorf("missing field %q in marshaled JSON", f)
		}
	}
}

func TestBuildProfile_ExamplesTrimmed(t *testing.T) {
	longBody := ""
	for i := 0; i < 400; i++ {
		longBody += "x"
	}
	pages := []avatar.PageRecord{
		{ID: "1", Title: "Long Page", Body: longBody, LastModified: time.Now()},
	}
	profile := avatar.BuildProfile("acc", "Eve", pages)

	if len(profile.Examples) > 0 {
		ex := profile.Examples[0]
		if len(ex.Text) > 303 { // 300 + "..."
			t.Errorf("example text too long: %d chars", len(ex.Text))
		}
		if len(longBody) > 300 && len(ex.Text) != 303 {
			t.Errorf("expected trimmed example to be 303 chars (300 + ...), got %d", len(ex.Text))
		}
	}
}
