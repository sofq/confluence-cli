package oauth2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// tokenEndpoint is the URL used for token requests. It defaults to TokenURL
// and can be overridden in tests.
var tokenEndpoint = TokenURL

// ClientCredentials performs an OAuth2 client credentials (2LO) token fetch.
// It first checks the store for an unexpired cached token. If none is found,
// it requests a new token from the Atlassian token endpoint.
func ClientCredentials(clientID, clientSecret, scopes string, store *FileStore) (*Token, error) {
	// Check for cached unexpired token.
	if cached := store.Load(); cached != nil && !cached.Expired(60*time.Second) {
		return cached, nil
	}

	// Build form data.
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	if scopes != "" {
		data.Set("scope", scopes)
	}

	resp, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode())) // #nosec G107 -- tokenEndpoint is derived from the trusted TokenURL constant, not user input
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token request failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	token.ObtainedAt = time.Now()

	// Best-effort save to cache.
	_ = store.Save(&token)

	return &token, nil
}
