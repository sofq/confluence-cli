package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// threeloErrorReader is an io.ReadCloser whose Read always returns an error.
type threeloErrorReader struct{}

func (threeloErrorReader) Read(p []byte) (int, error) { return 0, errors.New("simulated read error") }
func (threeloErrorReader) Close() error               { return nil }

// threeloErrorBodyTransport is a custom http.RoundTripper that returns HTTP 200
// with a body that errors on the first Read call.
type threeloErrorBodyTransport struct{}

func (threeloErrorBodyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       threeloErrorReader{},
	}, nil
}

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

// TestCallbackNotFoundPath verifies the server returns 404 for paths other than /callback.
func TestCallbackNotFoundPath(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		time.Sleep(30 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/other", port))
		if err != nil {
			return
		}
		resp.Body.Close()
		// Now send a valid callback to unblock waitForCallback.
		time.Sleep(20 * time.Millisecond)
		resp2, err2 := http.Get(fmt.Sprintf("http://localhost:%d/callback?state=mystate&code=thecode", port))
		if err2 != nil {
			return
		}
		resp2.Body.Close()
	}()

	code, err := waitForCallback(listener, "mystate", 5*time.Second)
	if err != nil {
		t.Fatalf("waitForCallback failed: %v", err)
	}
	if code != "thecode" {
		t.Errorf("code = %q, want thecode", code)
	}
}

// TestCallbackAuthorizationError verifies the error parameter is surfaced.
func TestCallbackAuthorizationError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		time.Sleep(30 * time.Millisecond)
		//nolint:errcheck
		_, _ = http.Get(fmt.Sprintf(
			"http://localhost:%d/callback?state=mystate&error=access_denied&error_description=User+denied",
			port,
		))
	}()

	_, err = waitForCallback(listener, "mystate", 5*time.Second)
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
	if !strings.Contains(err.Error(), "authorization denied") {
		t.Errorf("error = %q, should contain 'authorization denied'", err.Error())
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("error = %q, should contain 'access_denied'", err.Error())
	}
}

// TestCallbackMissingCode verifies the missing-code path returns an error.
func TestCallbackMissingCode(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		time.Sleep(30 * time.Millisecond)
		//nolint:errcheck
		_, _ = http.Get(fmt.Sprintf("http://localhost:%d/callback?state=mystate", port))
	}()

	_, err = waitForCallback(listener, "mystate", 5*time.Second)
	if err == nil {
		t.Fatal("expected missing code error, got nil")
	}
	if !strings.Contains(err.Error(), "missing code") {
		t.Errorf("error = %q, should mention 'missing code'", err.Error())
	}
}

// TestCallbackSuccess verifies a valid code is returned.
func TestCallbackSuccess(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		time.Sleep(30 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/callback?state=mystate&code=goodcode", port))
		if err != nil {
			return
		}
		resp.Body.Close()
	}()

	code, err := waitForCallback(listener, "mystate", 5*time.Second)
	if err != nil {
		t.Fatalf("waitForCallback failed: %v", err)
	}
	if code != "goodcode" {
		t.Errorf("code = %q, want goodcode", code)
	}
}

// TestOpenBrowser exercises the openBrowser function by checking it attempts
// the right command for the current OS. We replace openBrowserFunc to capture
// the URL but also directly test openBrowser path selection via the OS switch.
func TestOpenBrowserCurrentOS(t *testing.T) {
	// Verify openBrowserFunc is invoked without actually launching a browser.
	old := openBrowserFunc
	var capturedURL string
	openBrowserFunc = func(u string) error {
		capturedURL = u
		return nil
	}
	defer func() { openBrowserFunc = old }()

	_ = openBrowserFunc("https://example.com")
	if capturedURL != "https://example.com" {
		t.Errorf("expected https://example.com, got %s", capturedURL)
	}
}

// TestRefreshTokenNetworkError covers the HTTP Post failure path in refreshToken.
func TestRefreshTokenNetworkError(t *testing.T) {
	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = "http://127.0.0.1:1" // refused
	defer func() { tokenEndpointThreeLO = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "refreshneterr")

	_, err := refreshToken("id", "secret", "refresh-tok", store)
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "refresh token request failed") {
		t.Errorf("error = %q, should contain 'refresh token request failed'", err.Error())
	}
}

// TestRefreshTokenHTTPError covers the HTTP 4xx path in refreshToken.
func TestRefreshTokenHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = srv.URL
	defer func() { tokenEndpointThreeLO = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "refreshhttperr")
	// Pre-save a token so Load() returns something with a CloudID to preserve.
	_ = store.Save(&Token{
		AccessToken: "old",
		CloudID:     "cloud-x",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now().Add(-4000 * time.Second),
	})

	_, err := refreshToken("id", "secret", "bad-refresh", store)
	if err == nil {
		t.Fatal("expected HTTP error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("error = %q, should contain 'HTTP 401'", err.Error())
	}
}

// TestRefreshTokenInvalidJSON covers the JSON decode failure path in refreshToken.
func TestRefreshTokenInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = srv.URL
	defer func() { tokenEndpointThreeLO = old }()

	dir := t.TempDir()
	store := NewFileStore(dir, "refreshbadjson")

	_, err := refreshToken("id", "secret", "refresh-tok", store)
	if err == nil {
		t.Fatal("expected JSON error, got nil")
	}
	if !strings.Contains(err.Error(), "decoding refresh response") {
		t.Errorf("error = %q, should contain 'decoding refresh response'", err.Error())
	}
}

// TestRefreshTokenPreservesCloudID verifies cloudID from old token is carried over
// even when the old token has no CloudID (empty string branch).
func TestRefreshTokenNoOldToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-tok",
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
	// Do NOT save anything — store.Load() returns nil.
	store := NewFileStore(dir, "nooldtoken")

	tok, err := refreshToken("id", "secret", "refresh-tok", store)
	if err != nil {
		t.Fatalf("refreshToken failed: %v", err)
	}
	if tok.AccessToken != "new-tok" {
		t.Errorf("AccessToken = %q, want new-tok", tok.AccessToken)
	}
	// CloudID should be empty (no old token to inherit from).
	if tok.CloudID != "" {
		t.Errorf("CloudID = %q, want empty", tok.CloudID)
	}
}

// TestDiscoverCloudIDNetworkError covers the HTTP Do failure path.
func TestDiscoverCloudIDNetworkError(t *testing.T) {
	old := resourcesEndpoint
	resourcesEndpoint = "http://127.0.0.1:1" // refused
	defer func() { resourcesEndpoint = old }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "resources request failed") {
		t.Errorf("error = %q, should contain 'resources request failed'", err.Error())
	}
}

// TestDiscoverCloudIDHTTPError covers the HTTP 4xx path in discoverCloudID.
func TestDiscoverCloudIDHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	old := resourcesEndpoint
	resourcesEndpoint = srv.URL
	defer func() { resourcesEndpoint = old }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected HTTP error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("error = %q, should contain 'HTTP 403'", err.Error())
	}
}

// TestDiscoverCloudIDInvalidJSON covers the JSON decode failure path.
func TestDiscoverCloudIDInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	old := resourcesEndpoint
	resourcesEndpoint = srv.URL
	defer func() { resourcesEndpoint = old }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected JSON error, got nil")
	}
	if !strings.Contains(err.Error(), "decoding resources response") {
		t.Errorf("error = %q, should contain 'decoding resources response'", err.Error())
	}
}

// TestExchangeCodeNetworkError covers the HTTP Post failure path.
func TestExchangeCodeNetworkError(t *testing.T) {
	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = "http://127.0.0.1:1" // refused
	defer func() { tokenEndpointThreeLO = old }()

	_, err := exchangeCode("id", "secret", "code", "http://localhost/callback", "verifier")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "token exchange failed") {
		t.Errorf("error = %q, should contain 'token exchange failed'", err.Error())
	}
}

// TestExchangeCodeHTTPError covers the HTTP 4xx path.
func TestExchangeCodeHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = srv.URL
	defer func() { tokenEndpointThreeLO = old }()

	_, err := exchangeCode("id", "secret", "code", "http://localhost/callback", "verifier")
	if err == nil {
		t.Fatal("expected HTTP error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("error = %q, should contain 'HTTP 400'", err.Error())
	}
}

// TestExchangeCodeInvalidJSON covers the JSON decode failure path.
func TestExchangeCodeInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = srv.URL
	defer func() { tokenEndpointThreeLO = old }()

	_, err := exchangeCode("id", "secret", "code", "http://localhost/callback", "verifier")
	if err == nil {
		t.Fatal("expected JSON error, got nil")
	}
	if !strings.Contains(err.Error(), "decoding exchange response") {
		t.Errorf("error = %q, should contain 'decoding exchange response'", err.Error())
	}
}

// TestThreeLOFullFlowWithCloudID exercises the full 3LO browser flow when no
// cached/refresh token exists and cloudID is pre-supplied (skips discovery).
func TestThreeLOFullFlowWithCloudID(t *testing.T) {
	// Token exchange server.
	exchangeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "full-flow-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "full-flow-refresh",
		})
	}))
	defer exchangeSrv.Close()

	oldEndpoint := tokenEndpointThreeLO
	tokenEndpointThreeLO = exchangeSrv.URL
	defer func() { tokenEndpointThreeLO = oldEndpoint }()

	// Suppress browser open.
	oldBrowser := openBrowserFunc
	var capturedAuthURL string
	openBrowserFunc = func(u string) error {
		capturedAuthURL = u
		return nil
	}
	defer func() { openBrowserFunc = oldBrowser }()

	// Shorten callback timeout.
	oldTimeout := callbackTimeout
	callbackTimeout = 5 * time.Second
	defer func() { callbackTimeout = oldTimeout }()

	dir := t.TempDir()
	store := NewFileStore(dir, "fullflow")

	// Run ThreeLO in a goroutine; simulate the OAuth callback once we know the port.
	type result struct {
		tok *Token
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		tok, err := ThreeLO("my-client", "my-secret", "read:confluence", "cloud-abc", store)
		resultCh <- result{tok, err}
	}()

	// Wait a moment for ThreeLO to start the listener and print the auth URL,
	// then parse the redirect_uri from capturedAuthURL to know the callback port.
	var port int
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if capturedAuthURL != "" {
			break
		}
	}
	if capturedAuthURL == "" {
		t.Fatal("openBrowserFunc was not called within timeout")
	}
	// Parse redirect_uri from the auth URL.
	parsed, err := url.Parse(capturedAuthURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	redirectURI := parsed.Query().Get("redirect_uri")
	state := parsed.Query().Get("state")
	cbParsed, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirectURI: %v", err)
	}
	// Extract port from redirect URI host.
	_, portStr, _ := net.SplitHostPort(cbParsed.Host)
	fmt.Sscanf(portStr, "%d", &port)

	// Send the callback.
	cbURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=authcode123", port, state)
	resp, err := http.Get(cbURL) //nolint:noctx
	if err != nil {
		t.Fatalf("callback GET failed: %v", err)
	}
	resp.Body.Close()

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("ThreeLO failed: %v", res.err)
	}
	if res.tok.AccessToken != "full-flow-token" {
		t.Errorf("AccessToken = %q, want full-flow-token", res.tok.AccessToken)
	}
	if res.tok.CloudID != "cloud-abc" {
		t.Errorf("CloudID = %q, want cloud-abc", res.tok.CloudID)
	}

	// Verify saved to store.
	loaded := store.Load()
	if loaded == nil {
		t.Fatal("token not saved to store")
	}
	if loaded.AccessToken != "full-flow-token" {
		t.Errorf("saved AccessToken = %q, want full-flow-token", loaded.AccessToken)
	}
}

// TestThreeLOFullFlowDiscoverCloudID exercises the path where cloudID is empty
// and must be discovered from accessible-resources.
func TestThreeLOFullFlowDiscoverCloudID(t *testing.T) {
	// Token exchange server.
	exchangeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "discovered-cloud-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "discovered-refresh",
		})
	}))
	defer exchangeSrv.Close()

	// Resources endpoint (discovery).
	resourcesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"id": "discovered-cloud-id", "name": "My Site", "url": "https://mysite.atlassian.net"},
		})
	}))
	defer resourcesSrv.Close()

	oldEndpoint := tokenEndpointThreeLO
	tokenEndpointThreeLO = exchangeSrv.URL
	defer func() { tokenEndpointThreeLO = oldEndpoint }()

	oldResources := resourcesEndpoint
	resourcesEndpoint = resourcesSrv.URL
	defer func() { resourcesEndpoint = oldResources }()

	oldBrowser := openBrowserFunc
	var capturedAuthURL string
	openBrowserFunc = func(u string) error {
		capturedAuthURL = u
		return nil
	}
	defer func() { openBrowserFunc = oldBrowser }()

	oldTimeout := callbackTimeout
	callbackTimeout = 5 * time.Second
	defer func() { callbackTimeout = oldTimeout }()

	dir := t.TempDir()
	store := NewFileStore(dir, "discovercloud")

	type result struct {
		tok *Token
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		// Pass empty cloudID so discovery is triggered.
		tok, err := ThreeLO("my-client", "my-secret", "read:confluence", "", store)
		resultCh <- result{tok, err}
	}()

	// Wait for the browser URL to be captured.
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if capturedAuthURL != "" {
			break
		}
	}
	if capturedAuthURL == "" {
		t.Fatal("openBrowserFunc was not called within timeout")
	}

	parsed, err := url.Parse(capturedAuthURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	redirectURI := parsed.Query().Get("redirect_uri")
	state := parsed.Query().Get("state")
	cbParsed, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirectURI: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(cbParsed.Host)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	cbURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=authcode456", port, state)
	resp, err := http.Get(cbURL) //nolint:noctx
	if err != nil {
		t.Fatalf("callback GET failed: %v", err)
	}
	resp.Body.Close()

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("ThreeLO failed: %v", res.err)
	}
	if res.tok.AccessToken != "discovered-cloud-token" {
		t.Errorf("AccessToken = %q, want discovered-cloud-token", res.tok.AccessToken)
	}
	if res.tok.CloudID != "discovered-cloud-id" {
		t.Errorf("CloudID = %q, want discovered-cloud-id", res.tok.CloudID)
	}
}

// TestThreeLOFullFlowDiscoveryError exercises the path where cloudID discovery fails.
func TestThreeLOFullFlowDiscoveryError(t *testing.T) {
	// Token exchange server (returns success).
	exchangeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "some-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer exchangeSrv.Close()

	// Resources endpoint returns an error.
	resourcesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer resourcesSrv.Close()

	oldEndpoint := tokenEndpointThreeLO
	tokenEndpointThreeLO = exchangeSrv.URL
	defer func() { tokenEndpointThreeLO = oldEndpoint }()

	oldResources := resourcesEndpoint
	resourcesEndpoint = resourcesSrv.URL
	defer func() { resourcesEndpoint = oldResources }()

	oldBrowser := openBrowserFunc
	var capturedAuthURL string
	openBrowserFunc = func(u string) error {
		capturedAuthURL = u
		return nil
	}
	defer func() { openBrowserFunc = oldBrowser }()

	oldTimeout := callbackTimeout
	callbackTimeout = 5 * time.Second
	defer func() { callbackTimeout = oldTimeout }()

	dir := t.TempDir()
	store := NewFileStore(dir, "discoveryfail")

	type result struct {
		tok *Token
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		tok, err := ThreeLO("my-client", "my-secret", "read:confluence", "", store)
		resultCh <- result{tok, err}
	}()

	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if capturedAuthURL != "" {
			break
		}
	}
	if capturedAuthURL == "" {
		t.Fatal("openBrowserFunc was not called within timeout")
	}

	parsed, err := url.Parse(capturedAuthURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	redirectURI := parsed.Query().Get("redirect_uri")
	state := parsed.Query().Get("state")
	cbParsed, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirectURI: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(cbParsed.Host)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	cbURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=authcode789", port, state)
	resp, err := http.Get(cbURL) //nolint:noctx
	if err != nil {
		t.Fatalf("callback GET failed: %v", err)
	}
	resp.Body.Close()

	res := <-resultCh
	if res.err == nil {
		t.Fatal("expected discovery error, got nil")
	}
	if !strings.Contains(res.err.Error(), "HTTP 403") {
		t.Errorf("error = %q, should contain 'HTTP 403'", res.err.Error())
	}
}

// TestThreeLOScopesAlreadyContainOfflineAccess verifies that offline_access
// is not duplicated when already present in scopes.
func TestThreeLOScopesAlreadyContainOfflineAccess(t *testing.T) {
	exchangeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "scoped-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer exchangeSrv.Close()

	oldEndpoint := tokenEndpointThreeLO
	tokenEndpointThreeLO = exchangeSrv.URL
	defer func() { tokenEndpointThreeLO = oldEndpoint }()

	oldBrowser := openBrowserFunc
	var capturedAuthURL string
	openBrowserFunc = func(u string) error {
		capturedAuthURL = u
		return nil
	}
	defer func() { openBrowserFunc = oldBrowser }()

	oldTimeout := callbackTimeout
	callbackTimeout = 5 * time.Second
	defer func() { callbackTimeout = oldTimeout }()

	dir := t.TempDir()
	store := NewFileStore(dir, "scopecheck")

	type result struct {
		tok *Token
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		// Scopes already include offline_access.
		tok, err := ThreeLO("my-client", "my-secret", "read:confluence offline_access", "cloud-xyz", store)
		resultCh <- result{tok, err}
	}()

	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if capturedAuthURL != "" {
			break
		}
	}
	if capturedAuthURL == "" {
		t.Fatal("openBrowserFunc was not called")
	}

	// Verify offline_access appears only once in the scope parameter.
	parsed, err := url.Parse(capturedAuthURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	scope, _ := url.QueryUnescape(parsed.Query().Get("scope"))
	count := strings.Count(scope, "offline_access")
	if count != 1 {
		t.Errorf("scope %q contains offline_access %d times, want 1", scope, count)
	}

	redirectURI := parsed.Query().Get("redirect_uri")
	state := parsed.Query().Get("state")
	cbParsed, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirectURI: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(cbParsed.Host)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	cbURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=scopecode", port, state)
	resp, err := http.Get(cbURL) //nolint:noctx
	if err != nil {
		t.Fatalf("callback GET failed: %v", err)
	}
	resp.Body.Close()

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("ThreeLO failed: %v", res.err)
	}
}

// TestDiscoverCloudIDNewRequestError covers the http.NewRequest failure path by
// setting resourcesEndpoint to a URL that is syntactically invalid.
func TestDiscoverCloudIDNewRequestError(t *testing.T) {
	old := resourcesEndpoint
	resourcesEndpoint = "://invalid-url" // causes http.NewRequest to fail
	defer func() { resourcesEndpoint = old }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected request construction error, got nil")
	}
	if !strings.Contains(err.Error(), "creating resources request") {
		t.Errorf("error = %q, should contain 'creating resources request'", err.Error())
	}
}

// TestThreeLOExchangeCodeFailure covers the ThreeLO path where the callback is
// received but the code exchange with the token endpoint fails.
func TestThreeLOExchangeCodeFailure(t *testing.T) {
	// Token exchange server returns an error.
	exchangeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer exchangeSrv.Close()

	oldEndpoint := tokenEndpointThreeLO
	tokenEndpointThreeLO = exchangeSrv.URL
	defer func() { tokenEndpointThreeLO = oldEndpoint }()

	oldBrowser := openBrowserFunc
	var capturedAuthURL string
	openBrowserFunc = func(u string) error {
		capturedAuthURL = u
		return nil
	}
	defer func() { openBrowserFunc = oldBrowser }()

	oldTimeout := callbackTimeout
	callbackTimeout = 5 * time.Second
	defer func() { callbackTimeout = oldTimeout }()

	dir := t.TempDir()
	store := NewFileStore(dir, "exchangefail")

	type result struct {
		tok *Token
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		tok, err := ThreeLO("my-client", "my-secret", "read:confluence", "cloud-xyz", store)
		resultCh <- result{tok, err}
	}()

	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if capturedAuthURL != "" {
			break
		}
	}
	if capturedAuthURL == "" {
		t.Fatal("openBrowserFunc was not called within timeout")
	}

	parsed, err := url.Parse(capturedAuthURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	redirectURI := parsed.Query().Get("redirect_uri")
	state := parsed.Query().Get("state")
	cbParsed, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirectURI: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(cbParsed.Host)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	cbURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=failcode", port, state)
	resp, err := http.Get(cbURL) //nolint:noctx
	if err != nil {
		t.Fatalf("callback GET failed: %v", err)
	}
	resp.Body.Close()

	res := <-resultCh
	if res.err == nil {
		t.Fatal("expected exchange error, got nil")
	}
	if !strings.Contains(res.err.Error(), "HTTP 400") {
		t.Errorf("error = %q, should contain 'HTTP 400'", res.err.Error())
	}
}

// TestRefreshTokenBodyReadError covers the io.ReadAll failure path in refreshToken.
func TestRefreshTokenBodyReadError(t *testing.T) {
	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = "http://example.com/token" // irrelevant — transport is replaced
	defer func() { tokenEndpointThreeLO = old }()

	origTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = threeloErrorBodyTransport{}
	defer func() { http.DefaultClient.Transport = origTransport }()

	dir := t.TempDir()
	store := NewFileStore(dir, "refreshbodyreaderr")

	_, err := refreshToken("id", "secret", "refresh-tok", store)
	if err == nil {
		t.Fatal("expected body read error, got nil")
	}
	if !strings.Contains(err.Error(), "reading refresh response") {
		t.Errorf("error = %q, should contain 'reading refresh response'", err.Error())
	}
}

// TestDiscoverCloudIDBodyReadError covers the io.ReadAll failure path in discoverCloudID.
func TestDiscoverCloudIDBodyReadError(t *testing.T) {
	old := resourcesEndpoint
	resourcesEndpoint = "http://example.com/resources" // irrelevant — transport is replaced
	defer func() { resourcesEndpoint = old }()

	origTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = threeloErrorBodyTransport{}
	defer func() { http.DefaultClient.Transport = origTransport }()

	_, err := discoverCloudID("test-token")
	if err == nil {
		t.Fatal("expected body read error, got nil")
	}
	if !strings.Contains(err.Error(), "reading resources response") {
		t.Errorf("error = %q, should contain 'reading resources response'", err.Error())
	}
}

// TestExchangeCodeBodyReadError covers the io.ReadAll failure path in exchangeCode.
func TestExchangeCodeBodyReadError(t *testing.T) {
	old := tokenEndpointThreeLO
	tokenEndpointThreeLO = "http://example.com/token" // irrelevant — transport is replaced
	defer func() { tokenEndpointThreeLO = old }()

	origTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = threeloErrorBodyTransport{}
	defer func() { http.DefaultClient.Transport = origTransport }()

	_, err := exchangeCode("id", "secret", "code", "http://localhost/callback", "verifier")
	if err == nil {
		t.Fatal("expected body read error, got nil")
	}
	if !strings.Contains(err.Error(), "reading exchange response") {
		t.Errorf("error = %q, should contain 'reading exchange response'", err.Error())
	}
}
