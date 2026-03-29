package oauth2

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// errorReader is an io.ReadCloser that returns an error on the first Read call.
type errorReader struct{}

func (errorReader) Read(p []byte) (int, error) { return 0, errors.New("simulated read error") }
func (errorReader) Close() error               { return nil }

// errorBodyTransport is a custom http.RoundTripper that responds with HTTP 200
// but a body that always errors on Read.
type errorBodyTransport struct{}

func (errorBodyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       errorReader{},
	}, nil
}

func TestClientCredentialsCachedToken(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, "cached")

	// Pre-save a valid token
	cached := &Token{
		AccessToken: "cached-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now(),
	}
	if err := store.Save(cached); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Should return cached token without hitting network
	tok, err := ClientCredentials("id", "secret", "", store)
	if err != nil {
		t.Fatalf("ClientCredentials failed: %v", err)
	}
	if tok.AccessToken != "cached-token" {
		t.Errorf("AccessToken = %q, want cached-token", tok.AccessToken)
	}
}

func TestClientCredentialsFetchesNewToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.FormValue("grant_type"); got != "client_credentials" {
			t.Errorf("grant_type = %q, want client_credentials", got)
		}
		if got := r.FormValue("client_id"); got != "my-id" {
			t.Errorf("client_id = %q, want my-id", got)
		}
		if got := r.FormValue("client_secret"); got != "my-secret" {
			t.Errorf("client_secret = %q, want my-secret", got)
		}
		if got := r.FormValue("scope"); got != "read:confluence" {
			t.Errorf("scope = %q, want read:confluence", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	// Override token endpoint
	old := tokenEndpoint
	tokenEndpoint = srv.URL
	defer func() { tokenEndpoint = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "fresh")

	tok, err := ClientCredentials("my-id", "my-secret", "read:confluence", store)
	if err != nil {
		t.Fatalf("ClientCredentials failed: %v", err)
	}
	if tok.AccessToken != "new-token" {
		t.Errorf("AccessToken = %q, want new-token", tok.AccessToken)
	}

	// Verify token was saved to store
	loaded := store.Load()
	if loaded == nil {
		t.Fatal("token was not saved to store")
	}
	if loaded.AccessToken != "new-token" {
		t.Errorf("saved AccessToken = %q, want new-token", loaded.AccessToken)
	}
}

func TestClientCredentialsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()

	old := tokenEndpoint
	tokenEndpoint = srv.URL
	defer func() { tokenEndpoint = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "error")

	_, err := ClientCredentials("bad-id", "bad-secret", "", store)
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "HTTP 401") {
		t.Errorf("error = %q, should contain 'HTTP 401'", got)
	}
}

func TestClientCredentialsExpiredCacheFetchesNew(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "refreshed-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	old := tokenEndpoint
	tokenEndpoint = srv.URL
	defer func() { tokenEndpoint = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "expired")

	// Save an expired token
	expired := &Token{
		AccessToken: "old-token",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now().Add(-4000 * time.Second),
	}
	if err := store.Save(expired); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tok, err := ClientCredentials("id", "secret", "", store)
	if err != nil {
		t.Fatalf("ClientCredentials failed: %v", err)
	}
	if tok.AccessToken != "refreshed-token" {
		t.Errorf("AccessToken = %q, want refreshed-token", tok.AccessToken)
	}
}

func TestClientCredentialsNetworkError(t *testing.T) {
	// Point tokenEndpoint at a port that is not listening.
	old := tokenEndpoint
	tokenEndpoint = "http://127.0.0.1:1" // port 1 is reserved/refused
	defer func() { tokenEndpoint = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "neterr")

	_, err := ClientCredentials("id", "secret", "", store)
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "token request failed") {
		t.Errorf("error = %q, should contain 'token request failed'", err.Error())
	}
}

func TestClientCredentialsInvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`not-valid-json`))
	}))
	defer srv.Close()

	old := tokenEndpoint
	tokenEndpoint = srv.URL
	defer func() { tokenEndpoint = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "badjson")

	_, err := ClientCredentials("id", "secret", "", store)
	if err == nil {
		t.Fatal("expected JSON decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decoding token response") {
		t.Errorf("error = %q, should contain 'decoding token response'", err.Error())
	}
}

func TestClientCredentialsBodyReadError(t *testing.T) {
	// Point tokenEndpoint at a local server so the HTTP POST connects, then
	// override the default transport to return a body that errors on read.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`ignored`))
	}))
	defer srv.Close()

	old := tokenEndpoint
	tokenEndpoint = srv.URL
	defer func() { tokenEndpoint = old }()

	// Replace default transport so the response body returns a read error.
	origTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = errorBodyTransport{}
	defer func() { http.DefaultClient.Transport = origTransport }()

	dir := t.TempDir()
	store := NewFileStore(dir, "bodyreaderr")

	_, err := ClientCredentials("id", "secret", "", store)
	if err == nil {
		t.Fatal("expected body read error, got nil")
	}
	if !strings.Contains(err.Error(), "reading token response") {
		t.Errorf("error = %q, should contain 'reading token response'", err.Error())
	}
}

