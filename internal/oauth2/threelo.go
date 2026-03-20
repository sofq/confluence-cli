package oauth2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Package-level overrides for testing.
var (
	tokenEndpointThreeLO = TokenURL
	resourcesEndpoint    = ResourcesURL
	openBrowserFunc      = openBrowser
	callbackTimeout      = 5 * time.Minute
)

// generateCodeVerifier creates a PKCE code verifier: 32 random bytes,
// base64url-encoded without padding (43 characters).
func generateCodeVerifier() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// s256Challenge computes the S256 code challenge for a PKCE verifier.
func s256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// generateState returns a random hex string for the OAuth2 state parameter.
func generateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(u string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	default:
		return exec.Command("xdg-open", u).Start()
	}
}

// refreshToken exchanges a refresh token for a new access token.
func refreshToken(clientID, clientSecret, refreshTok string, store *FileStore) (*Token, error) {
	// Load current token to preserve CloudID.
	oldToken := store.Load()

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshTok},
	}

	resp, err := http.Post(tokenEndpointThreeLO, "application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("refresh token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading refresh response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("refresh token failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decoding refresh response: %w", err)
	}
	token.ObtainedAt = time.Now()

	// Preserve CloudID from old token.
	if oldToken != nil && oldToken.CloudID != "" {
		token.CloudID = oldToken.CloudID
	}

	_ = store.Save(&token)
	return &token, nil
}

// discoverCloudID calls the accessible-resources endpoint to find the
// Confluence cloud site ID.
func discoverCloudID(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", resourcesEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("creating resources request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resources request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading resources response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("resources request failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sites []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(body, &sites); err != nil {
		return "", fmt.Errorf("decoding resources response: %w", err)
	}

	switch len(sites) {
	case 0:
		return "", fmt.Errorf("no accessible Confluence sites found for this token")
	case 1:
		return sites[0].ID, nil
	default:
		var parts []string
		for _, s := range sites {
			parts = append(parts, fmt.Sprintf("%s (%s)", s.Name, s.ID))
		}
		return "", fmt.Errorf("multiple sites found: %s; specify --cloud-id", strings.Join(parts, ", "))
	}
}

// waitForCallback starts an HTTP server on the given listener and waits
// for the OAuth2 authorization callback. It returns the authorization code
// or an error if the callback times out or contains an error.
func waitForCallback(listener net.Listener, expectedState string, timeout time.Duration) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		if s := r.URL.Query().Get("state"); s != expectedState {
			errCh <- fmt.Errorf("state mismatch: expected %s, got %s", expectedState, s)
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			errCh <- fmt.Errorf("authorization denied: %s: %s", errMsg, desc)
			fmt.Fprint(w, "Authorization denied. You may close this window.")
			return
		}
		code := r.URL.Query().Get("code")
		fmt.Fprint(w, "Authorization successful! You may close this window.")
		codeCh <- code
	})

	go srv.Serve(listener)
	defer srv.Close()

	select {
	case code := <-codeCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("authorization timed out after %v; no callback received", timeout)
	}
}

// exchangeCode exchanges an authorization code for tokens.
func exchangeCode(clientID, clientSecret, code, redirectURI, verifier string) (*Token, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	resp, err := http.Post(tokenEndpointThreeLO, "application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading exchange response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decoding exchange response: %w", err)
	}
	token.ObtainedAt = time.Now()

	return &token, nil
}

// ThreeLO performs the OAuth2 authorization code (3LO) flow with PKCE.
// It checks for a cached unexpired token, tries refresh if available,
// and falls back to the full browser-based authorization flow.
func ThreeLO(clientID, clientSecret, scopes, cloudID string, store *FileStore) (*Token, error) {
	// 1. Check store for unexpired token.
	if cached := store.Load(); cached != nil && !cached.Expired(60*time.Second) {
		return cached, nil
	}

	// 2. Try refresh if refresh token exists.
	if cached := store.Load(); cached != nil && cached.RefreshToken != "" {
		tok, err := refreshToken(clientID, clientSecret, cached.RefreshToken, store)
		if err == nil {
			return tok, nil
		}
		// Refresh failed (e.g., expired refresh token) -- fall through to full flow.
	}

	// 3. Full browser flow.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	verifier := generateCodeVerifier()
	challenge := s256Challenge(verifier)
	state := generateState()

	// Ensure offline_access is included in scopes.
	fullScopes := scopes
	if !strings.Contains(fullScopes, "offline_access") {
		fullScopes = fullScopes + " offline_access"
	}
	fullScopes = strings.TrimSpace(fullScopes)

	authURL := fmt.Sprintf("%s?audience=%s&client_id=%s&scope=%s&redirect_uri=%s&state=%s&response_type=code&prompt=consent&code_challenge=%s&code_challenge_method=S256",
		AuthorizationURL, "api.atlassian.com", clientID,
		url.QueryEscape(fullScopes),
		url.QueryEscape(redirectURI), state, challenge)

	fmt.Fprintf(os.Stderr, "Opening browser for authorization...\nIf browser does not open, visit:\n%s\n", authURL)
	_ = openBrowserFunc(authURL)

	// Wait for callback.
	code, err := waitForCallback(listener, state, callbackTimeout)
	if err != nil {
		return nil, err
	}

	// Exchange code for token.
	token, err := exchangeCode(clientID, clientSecret, code, redirectURI, verifier)
	if err != nil {
		return nil, err
	}

	// Discover cloudID if not provided.
	if cloudID == "" {
		discovered, err := discoverCloudID(token.AccessToken)
		if err != nil {
			return nil, err
		}
		cloudID = discovered
	}
	token.CloudID = cloudID

	_ = store.Save(token)
	return token, nil
}
