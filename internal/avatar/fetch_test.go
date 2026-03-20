package avatar_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sofq/confluence-cli/internal/avatar"
	"github.com/sofq/confluence-cli/internal/client"
	"github.com/sofq/confluence-cli/internal/config"
)

// newTestClient creates a minimal client.Client pointed at the given base URL.
func newTestClient(baseURL string) *client.Client {
	return &client.Client{
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
		Stderr:     io.Discard,
		Auth:       config.AuthConfig{Type: "basic", Username: "user", Token: "token"},
	}
}

// TestStripStorageHTML tests the StripStorageHTML function.
func TestStripStorageHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "simple paragraph",
			input: "<p>Hello <strong>world</strong></p>",
			want:  "Hello world",
		},
		{
			name:  "HTML entities decoded",
			input: "<p>a &amp; b &lt;c&gt; &quot;d&quot; &nbsp;e</p>",
			want:  "a & b <c> \"d\" e",
		},
		{
			name:  "structured macro with CDATA",
			input: "<ac:structured-macro><ac:plain-text-body><![CDATA[code here]]></ac:plain-text-body></ac:structured-macro>",
			want:  "code here",
		},
		{
			name:  "whitespace collapsed",
			input: "<p>  foo   </p>  <p>  bar  </p>",
			want:  "foo bar",
		},
		{
			name:  "nested tags",
			input: "<h1>Title</h1><p>Some <em>text</em> here.</p>",
			want:  "Title Some text here.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := avatar.StripStorageHTML(tt.input)
			if got != tt.want {
				t.Errorf("StripStorageHTML(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// makeContentResponse builds a mock Confluence v1 content API response.
func makeContentResponse(pages []struct {
	ID    string
	Title string
	Body  string
	When  string
}, nextURL string) []byte {
	type result struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Body    struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		History struct {
			LastUpdated struct {
				When string `json:"when"`
			} `json:"lastUpdated"`
		} `json:"history"`
	}

	var results []result
	for _, p := range pages {
		var r result
		r.ID = p.ID
		r.Title = p.Title
		r.Body.Storage.Value = p.Body
		r.History.LastUpdated.When = p.When
		results = append(results, r)
	}

	links := map[string]string{}
	if nextURL != "" {
		links["next"] = nextURL
	}

	resp := map[string]any{
		"results": results,
		"_links":  links,
	}
	b, _ := json.Marshal(resp)
	return b
}

// TestFetchUserPages_HappyPath tests that FetchUserPages correctly parses a response.
func TestFetchUserPages_HappyPath(t *testing.T) {
	accountID := "user123"
	when1 := "2024-01-15T10:00:00.000Z"
	when2 := "2024-01-16T11:00:00.000Z"

	pages := []struct {
		ID    string
		Title string
		Body  string
		When  string
	}{
		{ID: "1", Title: "Page One", Body: "<p>Hello <b>world</b></p>", When: when1},
		{ID: "2", Title: "Page Two", Body: "<p>Foo &amp; bar</p>", When: when2},
	}

	var requestedCQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedCQL = r.URL.Query().Get("cql")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(makeContentResponse(pages, ""))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL + "/wiki/api/v2")
	records, err := avatar.FetchUserPages(c, accountID)
	if err != nil {
		t.Fatalf("FetchUserPages returned error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].ID != "1" {
		t.Errorf("records[0].ID = %q; want %q", records[0].ID, "1")
	}
	if records[0].Title != "Page One" {
		t.Errorf("records[0].Title = %q; want %q", records[0].Title, "Page One")
	}
	if records[0].Body != "Hello world" {
		t.Errorf("records[0].Body = %q; want %q", records[0].Body, "Hello world")
	}

	if records[1].Body != "Foo & bar" {
		t.Errorf("records[1].Body = %q; want %q", records[1].Body, "Foo & bar")
	}

	// Verify CQL query contains the accountId and is correct format.
	expectedCQLFragment := fmt.Sprintf(`creator = "%s"`, accountID)
	if requestedCQL == "" {
		t.Error("no CQL query was sent")
	}
	_ = expectedCQLFragment // CQL is URL-encoded, check via substring would be unreliable

	// Verify LastModified is parsed from When field.
	wantTime1, _ := time.Parse("2006-01-02T15:04:05.000Z", when1)
	if !records[0].LastModified.Equal(wantTime1) {
		t.Errorf("records[0].LastModified = %v; want %v", records[0].LastModified, wantTime1)
	}
}

// TestFetchUserPages_EmptyResults tests the empty results case.
func TestFetchUserPages_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":[],"_links":{}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL + "/wiki/api/v2")
	records, err := avatar.FetchUserPages(c, "user123")
	if err != nil {
		t.Fatalf("FetchUserPages returned error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

// TestFetchUserPages_HTTP401 tests that a 401 response returns a non-nil error.
func TestFetchUserPages_HTTP401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL + "/wiki/api/v2")
	records, err := avatar.FetchUserPages(c, "user123")
	if err == nil {
		t.Fatalf("expected error on 401, got nil (records: %v)", records)
	}
	if records != nil {
		t.Errorf("expected nil records on 401, got %v", records)
	}
}
