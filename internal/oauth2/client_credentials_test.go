package oauth2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		w.Write([]byte(`{"error":"invalid_client"}`))
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
	if got := err.Error(); !contains(got, "HTTP 401") {
		t.Errorf("error = %q, should contain 'HTTP 401'", got)
	}
}

func TestClientCredentialsExpiredCacheFetchesNew(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
