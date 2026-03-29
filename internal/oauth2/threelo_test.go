package oauth2

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestThreeLOCachedToken(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, "cached3lo")

	cached := &Token{
		AccessToken:  "cached-3lo-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "refresh-abc",
		ObtainedAt:   time.Now(),
		CloudID:      "cloud-123",
	}
	if err := store.Save(cached); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tok, err := ThreeLO("id", "secret", "read:confluence", "cloud-123", store)
	if err != nil {
		t.Fatalf("ThreeLO failed: %v", err)
	}
	if tok.AccessToken != "cached-3lo-token" {
		t.Errorf("AccessToken = %q, want cached-3lo-token", tok.AccessToken)
	}
}

func TestThreeLORefreshSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.FormValue("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", got)
		}
		if got := r.FormValue("refresh_token"); got != "old-refresh" {
			t.Errorf("refresh_token = %q, want old-refresh", got)
		}
		if got := r.FormValue("client_id"); got != "my-id" {
			t.Errorf("client_id = %q, want my-id", got)
		}
		if got := r.FormValue("client_secret"); got != "my-secret" {
			t.Errorf("client_secret = %q, want my-secret", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "refreshed-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "new-refresh",
		})
	}))
	defer srv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = srv.URL
	defer func() { tokenEndpointThreeLO = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "refresh")

	// Save an expired token with a refresh token
	expired := &Token{
		AccessToken:  "expired-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "old-refresh",
		ObtainedAt:   time.Now().Add(-4000 * time.Second),
		CloudID:      "cloud-456",
	}
	if err := store.Save(expired); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tok, err := ThreeLO("my-id", "my-secret", "read:confluence", "", store)
	if err != nil {
		t.Fatalf("ThreeLO failed: %v", err)
	}
	if tok.AccessToken != "refreshed-token" {
		t.Errorf("AccessToken = %q, want refreshed-token", tok.AccessToken)
	}
	if tok.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want new-refresh", tok.RefreshToken)
	}
	// CloudID should be preserved from old token
	if tok.CloudID != "cloud-456" {
		t.Errorf("CloudID = %q, want cloud-456", tok.CloudID)
	}

	// Verify saved to store
	loaded := store.Load()
	if loaded == nil {
		t.Fatal("token was not saved to store")
	}
	if loaded.AccessToken != "refreshed-token" {
		t.Errorf("saved AccessToken = %q, want refreshed-token", loaded.AccessToken)
	}
}

func TestThreeLORefreshFailureFallsThrough(t *testing.T) {
	// Refresh endpoint returns 400 (invalid_grant)
	refreshSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer refreshSrv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = refreshSrv.URL
	defer func() { tokenEndpointThreeLO = old }()

	// Prevent actual browser open
	oldBrowser := openBrowserFunc
	openBrowserFunc = func(url string) error { return nil }
	defer func() { openBrowserFunc = oldBrowser }()

	// Override callbackTimeout for fast test
	oldTimeout := callbackTimeout
	callbackTimeout = 200 * time.Millisecond
	defer func() { callbackTimeout = oldTimeout }()

	dir := t.TempDir()
	store := NewFileStore(dir, "refreshfail")

	expired := &Token{
		AccessToken:  "expired",
		ExpiresIn:    3600,
		RefreshToken: "bad-refresh",
		ObtainedAt:   time.Now().Add(-4000 * time.Second),
	}
	if err := store.Save(expired); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// ThreeLO should fail because refresh fails and no callback comes
	_, err := ThreeLO("id", "secret", "read:confluence", "cloud-123", store)
	if err == nil {
		t.Error("expected error when refresh fails and no callback, got nil")
	}
}

func TestGenerateCodeVerifier(t *testing.T) {
	v := generateCodeVerifier()
	// 32 bytes base64url-encoded = 43 characters
	if len(v) != 43 {
		t.Errorf("verifier length = %d, want 43", len(v))
	}

	// Should be different each time
	v2 := generateCodeVerifier()
	if v == v2 {
		t.Error("two verifiers should not be identical")
	}
}

func TestS256Challenge(t *testing.T) {
	challenge := s256Challenge("test")
	if challenge == "" {
		t.Error("challenge should not be empty")
	}
	// Verify it's valid base64url (no + / = characters)
	for _, c := range challenge {
		if c == '+' || c == '/' || c == '=' {
			t.Errorf("challenge contains non-base64url character: %c", c)
		}
	}
}

func TestExchangeCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.FormValue("grant_type"); got != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", got)
		}
		if got := r.FormValue("code"); got != "auth-code-xyz" {
			t.Errorf("code = %q, want auth-code-xyz", got)
		}
		if got := r.FormValue("code_verifier"); got != "my-verifier" {
			t.Errorf("code_verifier = %q, want my-verifier", got)
		}
		if got := r.FormValue("redirect_uri"); got != "http://localhost:9999/callback" {
			t.Errorf("redirect_uri = %q, want http://localhost:9999/callback", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "exchanged-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "exchange-refresh",
		})
	}))
	defer srv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = srv.URL
	defer func() { tokenEndpointThreeLO = old }()

	tok, err := exchangeCode("cid", "csecret", "auth-code-xyz", "http://localhost:9999/callback", "my-verifier")
	if err != nil {
		t.Fatalf("exchangeCode failed: %v", err)
	}
	if tok.AccessToken != "exchanged-token" {
		t.Errorf("AccessToken = %q, want exchanged-token", tok.AccessToken)
	}
	if tok.RefreshToken != "exchange-refresh" {
		t.Errorf("RefreshToken = %q, want exchange-refresh", tok.RefreshToken)
	}
}

func TestDiscoverCloudIDSingleSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "site-id-1", "name": "My Site", "url": "https://mysite.atlassian.net"},
		})
	}))
	defer srv.Close()

	old := resourcesEndpoint
	resourcesEndpoint = srv.URL
	defer func() { resourcesEndpoint = old }()

	id, err := discoverCloudID("test-token")
	if err != nil {
		t.Fatalf("discoverCloudID failed: %v", err)
	}
	if id != "site-id-1" {
		t.Errorf("cloudID = %q, want site-id-1", id)
	}
}

func TestDiscoverCloudIDMultipleSites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "site-1", "name": "Site One", "url": "https://one.atlassian.net"},
			{"id": "site-2", "name": "Site Two", "url": "https://two.atlassian.net"},
		})
	}))
	defer srv.Close()

	old := resourcesEndpoint
	resourcesEndpoint = srv.URL
	defer func() { resourcesEndpoint = old }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected error for multiple sites, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "Site One") || !strings.Contains(errMsg, "Site Two") {
		t.Errorf("error should list site names, got: %s", errMsg)
	}
}

func TestDiscoverCloudIDZeroSites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer srv.Close()

	old := resourcesEndpoint
	resourcesEndpoint = srv.URL
	defer func() { resourcesEndpoint = old }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected error for zero sites, got nil")
	}
	if !strings.Contains(err.Error(), "no accessible") {
		t.Errorf("error = %q, should mention 'no accessible'", err.Error())
	}
}

func TestCallbackTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	start := time.Now()
	_, err = waitForCallback(listener, "expected-state", 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, should mention 'timed out'", err.Error())
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took %v, expected ~100ms", elapsed)
	}
}

func TestCallbackStateMismatch(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		time.Sleep(50 * time.Millisecond)
		//nolint:errcheck
		_, _ = http.Get(fmt.Sprintf("http://localhost:%d/callback?state=wrong&code=abc", port))
	}()

	_, err = waitForCallback(listener, "expected-state", 5*time.Second)
	if err == nil {
		t.Fatal("expected state mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("error = %q, should mention 'state mismatch'", err.Error())
	}
}
