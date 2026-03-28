package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/audit"
	"github.com/sofq/confluence-cli/internal/cache"
	"github.com/sofq/confluence-cli/internal/config"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jq"
	"github.com/sofq/confluence-cli/internal/policy"
	"github.com/spf13/cobra"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// Client is the core HTTP client for cf. It wraps net/http with auth,
// pagination, jq filtering, and structured error output.
type Client struct {
	BaseURL     string
	Auth        config.AuthConfig
	HTTPClient  *http.Client
	Stdout      io.Writer     // JSON responses go here
	Stderr      io.Writer     // structured errors go here
	JQFilter    string        // --jq filter expression
	Paginate    bool          // auto-paginate GET responses
	DryRun      bool          // output request as JSON, don't execute
	Verbose     bool          // log request/response to stderr
	Pretty      bool          // pretty-print JSON
	Fields      string        // --fields comma-separated field names for GET
	CacheTTL    time.Duration // --cache duration; 0 means no caching
	Policy      *policy.Policy // nil = unrestricted
	AuditLogger *audit.Logger  // nil = no logging
	Profile     string         // active profile name (for audit entries)
	Operation   string         // operation name (for audit entries, set by batch)
}

// NewContext stores the client in the given context and returns the new context.
func NewContext(ctx context.Context, c *Client) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext retrieves the Client stored in ctx. Returns an error if no
// client was stored.
func FromContext(ctx context.Context) (*Client, error) {
	c, ok := ctx.Value(contextKey{}).(*Client)
	if !ok || c == nil {
		return nil, fmt.Errorf("client: no client found in context")
	}
	return c, nil
}

// QueryFromFlags extracts only the flags from names that have actually been
// changed by the user (i.e. are "set" in the flag set) and returns them as
// url.Values.
func QueryFromFlags(cmd *cobra.Command, names ...string) url.Values {
	q := url.Values{}
	for _, name := range names {
		f := cmd.Flags().Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		q.Set(name, f.Value.String())
	}
	return q
}

// ApplyAuth sets authentication headers on the request according to the
// client's Auth configuration. Returns an error if authentication setup fails.
func (c *Client) ApplyAuth(req *http.Request) error {
	switch strings.ToLower(c.Auth.Type) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.Auth.Token)
	default: // "basic" and any unrecognized type default to basic auth
		req.SetBasicAuth(c.Auth.Username, c.Auth.Token)
	}
	return nil
}

// Do executes an HTTP request and returns an exit code. It constructs the URL
// from BaseURL+path+query, applies auth, handles pagination, jq filtering, and
// pretty-printing. Errors are written as structured JSON to Stderr.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body io.Reader) int {
	// Append --fields as query param for GET requests.
	if c.Fields != "" && method == "GET" {
		if query == nil {
			query = url.Values{}
		}
		query.Set("fields", c.Fields)
	}

	// Ensure path separator between BaseURL and path.
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	rawURL := c.BaseURL + path
	if len(query) > 0 {
		if strings.Contains(rawURL, "?") {
			rawURL = rawURL + "&" + query.Encode()
		} else {
			rawURL = rawURL + "?" + query.Encode()
		}
	}

	// Determine operation name for policy enforcement and audit logging.
	operationName := c.Operation
	if operationName == "" {
		operationName = fmt.Sprintf("%s %s", method, path)
	}

	// Policy enforcement — BEFORE DryRun check so policy blocks even dry-run.
	if err := c.Policy.Check(operationName); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "policy_denied",
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return cferrors.ExitValidation
	}

	// DryRun: emit the request as JSON and return immediately.
	if c.DryRun {
		dryOut := map[string]any{
			"method": method,
			"url":    rawURL,
		}
		if body != nil {
			bodyBytes, err := io.ReadAll(body)
			if err == nil && len(bodyBytes) > 0 {
				var parsed any
				if json.Unmarshal(bodyBytes, &parsed) == nil {
					dryOut["body"] = parsed
				} else {
					dryOut["body"] = string(bodyBytes)
				}
			}
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(dryOut)
		exitCode := c.WriteOutput(bytes.TrimRight(buf.Bytes(), "\n"))
		c.AuditLogger.Log(audit.Entry{
			Profile:   c.Profile,
			Operation: operationName,
			Method:    method,
			Path:      path,
			Status:    0,
			Exit:      cferrors.ExitOK,
			DryRun:    true,
		})
		return exitCode
	}

	// Pagination only for GET requests.
	if c.Paginate && method == "GET" {
		return c.doWithPagination(ctx, method, rawURL, path, query)
	}

	return c.doOnce(ctx, method, rawURL, path, operationName, body)
}

// doOnce performs a single HTTP request and writes the response to Stdout.
func (c *Client) doOnce(ctx context.Context, method, rawURL, path, operationName string, body io.Reader) int {
	// Cache check for GET requests.
	var cacheKey string
	if c.CacheTTL > 0 && method == "GET" {
		cacheKey = cache.Key(method, rawURL, c.cacheAuthContext())
		if data, ok := cache.Get(cacheKey, c.CacheTTL); ok {
			return c.WriteOutput(data)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return cferrors.ExitError
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.ApplyAuth(req); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "auth_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return cferrors.ExitAuth
	}

	c.VerboseLog(map[string]any{"type": "request", "method": method, "url": rawURL})

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return cferrors.ExitError
	}
	defer resp.Body.Close()

	c.VerboseLog(map[string]any{"type": "response", "status": resp.StatusCode})

	// Capture status code for audit logging before body is read.
	statusCode := resp.StatusCode

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   "reading response body: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return cferrors.ExitError
	}

	// HTTP error (>=400): write structured error to stderr.
	if resp.StatusCode >= 400 {
		apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(respBody)), method, path, resp)
		apiErr.WriteJSON(c.Stderr)
		exitCode := apiErr.ExitCode()
		c.AuditLogger.Log(audit.Entry{
			Profile:   c.Profile,
			Operation: operationName,
			Method:    method,
			Path:      path,
			Status:    statusCode,
			Exit:      exitCode,
		})
		return exitCode
	}

	// HTTP 204 No Content — emit empty JSON object to maintain JSON-only contract.
	if len(respBody) == 0 || resp.StatusCode == http.StatusNoContent {
		respBody = []byte("{}")
	}

	// Cache successful GET responses.
	if cacheKey != "" {
		if err := cache.Set(cacheKey, respBody); err != nil {
			c.VerboseLog(map[string]any{"type": "warning", "message": "cache write failed: " + err.Error()})
		}
	}

	exitCode := c.WriteOutput(respBody)
	c.AuditLogger.Log(audit.Entry{
		Profile:   c.Profile,
		Operation: operationName,
		Method:    method,
		Path:      path,
		Status:    statusCode,
		Exit:      exitCode,
	})
	return exitCode
}

// cursorPage represents a Confluence v2 paginated response envelope.
type cursorPage struct {
	Results []json.RawMessage `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// detectCursorPagination returns true when body is a Confluence cursor-paginated envelope.
func detectCursorPagination(body []byte) bool {
	var probe struct {
		Results json.RawMessage `json:"results"`
		Links   json.RawMessage `json:"_links"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	return probe.Results != nil && probe.Links != nil
}

// doWithPagination fetches all pages of a Confluence cursor-paginated response,
// merges the results arrays, and writes the combined envelope to Stdout.
// Non-paginated responses are passed through unchanged.
func (c *Client) doWithPagination(ctx context.Context, method, firstURL, path string, query url.Values) int {
	// Check cache before fetching pages.
	var cacheKey string
	if c.CacheTTL > 0 {
		cacheKey = cache.Key(method, firstURL, c.cacheAuthContext())
		if data, ok := cache.Get(cacheKey, c.CacheTTL); ok {
			return c.WriteOutput(data)
		}
	}

	firstBody, code := c.fetchPage(ctx, method, firstURL, path)
	if code != cferrors.ExitOK {
		return code
	}

	if !detectCursorPagination(firstBody) {
		if cacheKey != "" {
			if err := cache.Set(cacheKey, firstBody); err != nil {
				c.VerboseLog(map[string]any{"type": "warning", "message": "cache write failed: " + err.Error()})
			}
		}
		return c.WriteOutput(firstBody)
	}

	return c.doCursorPagination(ctx, method, path, firstBody, cacheKey)
}

// doCursorPagination follows _links.next URLs until exhausted, accumulating
// all results[] entries, then writes a merged envelope to Stdout.
func (c *Client) doCursorPagination(ctx context.Context, method, path string, firstBody []byte, cacheKey string) int {
	var firstPage cursorPage
	if err := json.Unmarshal(firstBody, &firstPage); err != nil {
		return c.WriteOutput(firstBody)
	}

	allResults := append([]json.RawMessage{}, firstPage.Results...)
	nextLink := firstPage.Links.Next

	for nextLink != "" {
		// nextLink is a path relative to the domain (e.g. /wiki/api/v2/pages?cursor=xxx&limit=25).
		// Strip any domain prefix if present, then append to BaseURL.
		nextPath := nextLink
		if idx := strings.Index(nextLink, "/wiki/"); idx >= 0 {
			nextPath = nextLink[idx:]
		}
		// Strip the BaseURL suffix (e.g. /wiki/api/v2) to get the domain,
		// then prepend it so we don't double the /wiki/api/v2 prefix.
		domain := SearchV1Domain(c.BaseURL)
		nextURL := domain + nextPath

		body, code := c.fetchPage(ctx, method, nextURL, path)
		if code != cferrors.ExitOK {
			return code
		}

		var nextPage cursorPage
		if err := json.Unmarshal(body, &nextPage); err != nil {
			break
		}
		allResults = append(allResults, nextPage.Results...)
		nextLink = nextPage.Links.Next
	}

	// Reconstruct the envelope with merged results.
	var envelope map[string]json.RawMessage
	_ = json.Unmarshal(firstBody, &envelope)

	var resBuf bytes.Buffer
	resEnc := json.NewEncoder(&resBuf)
	resEnc.SetEscapeHTML(false)
	_ = resEnc.Encode(allResults)
	envelope["results"] = json.RawMessage(bytes.TrimRight(resBuf.Bytes(), "\n"))

	// Remove _links.next from merged result — pagination is complete.
	var links map[string]json.RawMessage
	if linksRaw, ok := envelope["_links"]; ok {
		_ = json.Unmarshal(linksRaw, &links)
		delete(links, "next")
		var linksBuf bytes.Buffer
		linksEnc := json.NewEncoder(&linksBuf)
		linksEnc.SetEscapeHTML(false)
		_ = linksEnc.Encode(links)
		envelope["_links"] = json.RawMessage(bytes.TrimRight(linksBuf.Bytes(), "\n"))
	}

	var resultBuf bytes.Buffer
	enc := json.NewEncoder(&resultBuf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(envelope)
	result := bytes.TrimRight(resultBuf.Bytes(), "\n")

	if cacheKey != "" {
		if err := cache.Set(cacheKey, result); err != nil {
			c.VerboseLog(map[string]any{"type": "warning", "message": "cache write failed: " + err.Error()})
		}
	}

	return c.WriteOutput(result)
}

// fetchPage performs a single GET request and returns the response body.
func (c *Client) fetchPage(ctx context.Context, method, rawURL, path string) ([]byte, int) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	req.Header.Set("Accept", "application/json")
	if err := c.ApplyAuth(req); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "auth_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitAuth
	}

	c.VerboseLog(map[string]any{"type": "request", "method": method, "url": rawURL})

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	defer resp.Body.Close()

	c.VerboseLog(map[string]any{"type": "response", "status": resp.StatusCode})

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Status:    0,
			Message:   "reading response body: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}

	if resp.StatusCode >= 400 {
		apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(body)), method, path, resp)
		apiErr.WriteJSON(c.Stderr)
		return nil, apiErr.ExitCode()
	}

	return body, cferrors.ExitOK
}

// cacheAuthContext returns a string that uniquely identifies the auth configuration,
// so that different profiles/credentials produce different cache keys for the same URL.
// The token is hashed to avoid keeping raw credentials in memory longer than necessary.
func (c *Client) cacheAuthContext() string {
	h := sha256.Sum256([]byte(c.Auth.Token))
	tokenHash := hex.EncodeToString(h[:8]) // 8 bytes is enough to distinguish tokens
	return c.BaseURL + "\x00" + c.Auth.Type + "\x00" + c.Auth.Username + "\x00" + tokenHash
}

// VerboseLog writes a structured JSON log entry to stderr when verbose mode is enabled.
func (c *Client) VerboseLog(fields map[string]any) {
	if !c.Verbose {
		return
	}
	data, _ := json.Marshal(fields)
	fmt.Fprintf(c.Stderr, "%s\n", data)
}

// Fetch performs an HTTP request and returns the raw response body and an exit code.
// Unlike Do, it does not write to Stdout — callers handle the body themselves.
// This is used by workflow commands and batch operations that need the raw response.
func (c *Client) Fetch(ctx context.Context, method, path string, body io.Reader) ([]byte, int) {
	fullURL := c.BaseURL + path

	// DryRun: emit the request as JSON and return immediately.
	if c.DryRun {
		dryOut := map[string]any{
			"method": method,
			"url":    fullURL,
		}
		if body != nil {
			bodyBytes, err := io.ReadAll(body)
			if err == nil && len(bodyBytes) > 0 {
				var parsed any
				if json.Unmarshal(bodyBytes, &parsed) == nil {
					dryOut["body"] = parsed
				} else {
					dryOut["body"] = string(bodyBytes)
				}
			}
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(dryOut)
		return bytes.TrimRight(buf.Bytes(), "\n"), cferrors.ExitOK
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   "failed to create request: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.ApplyAuth(req); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "auth_error",
			Message:   err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitAuth
	}

	c.VerboseLog(map[string]any{"type": "request", "method": method, "url": fullURL})

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	defer resp.Body.Close()

	c.VerboseLog(map[string]any{"type": "response", "status": resp.StatusCode})

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "connection_error",
			Message:   "reading response body: " + err.Error(),
		}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	// Determine operation name for audit logging.
	operationName := c.Operation
	if operationName == "" {
		operationName = fmt.Sprintf("%s %s", method, path)
	}

	if resp.StatusCode >= 400 {
		apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(respBody)), method, path, resp)
		apiErr.WriteJSON(c.Stderr)
		exitCode := apiErr.ExitCode()
		c.AuditLogger.Log(audit.Entry{
			Profile:   c.Profile,
			Operation: operationName,
			Method:    method,
			Path:      path,
			Status:    resp.StatusCode,
			Exit:      exitCode,
		})
		return nil, exitCode
	}

	c.AuditLogger.Log(audit.Entry{
		Profile:   c.Profile,
		Operation: operationName,
		Method:    method,
		Path:      path,
		Status:    resp.StatusCode,
		Exit:      cferrors.ExitOK,
	})
	return respBody, cferrors.ExitOK
}

// WriteOutput applies optional JQ filtering and pretty-printing, then writes
// the final JSON bytes to Stdout. Returns an exit code.
func (c *Client) WriteOutput(data []byte) int {
	// Apply JQ filter.
	if c.JQFilter != "" {
		filtered, err := jq.Apply(data, c.JQFilter)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "jq_error",
				Status:    0,
				Message:   "jq: " + err.Error(),
			}
			apiErr.WriteJSON(c.Stderr)
			return cferrors.ExitValidation
		}
		data = filtered
	}

	// Pretty-print if requested.
	if c.Pretty {
		var out bytes.Buffer
		if err := json.Indent(&out, data, "", "  "); err == nil {
			data = out.Bytes()
		}
	}

	fmt.Fprintf(c.Stdout, "%s\n", strings.TrimRight(string(data), "\n"))
	return cferrors.ExitOK
}

// SearchV1Domain extracts the scheme+host from a BaseURL.
// BaseURL is "https://domain/wiki/api/v2" in production, so we split on "/wiki/"
// to get just "https://domain".
func SearchV1Domain(baseURL string) string {
	if idx := strings.Index(baseURL, "/wiki/"); idx > 0 {
		return baseURL[:idx]
	}
	return baseURL
}
