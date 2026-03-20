package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search Confluence content via CQL",
	RunE:  runSearch,
}

// searchV1Domain extracts the scheme+host from c.BaseURL.
// c.BaseURL is "https://domain/wiki/api/v2" in production, so we split on "/wiki/api"
// to get just "https://domain".
func searchV1Domain(baseURL string) string {
	if idx := strings.Index(baseURL, "/wiki/"); idx > 0 {
		return baseURL[:idx]
	}
	return baseURL
}

// fetchV1 performs a single HTTP GET against a v1 URL (full absolute URL).
// It applies auth from c and writes error JSON to c.Stderr on failure.
func fetchV1(cmd *cobra.Command, c *client.Client, fullURL string) ([]byte, int) {
	req, err := http.NewRequestWithContext(cmd.Context(), "GET", fullURL, nil)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to create request: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	req.Header.Set("Accept", "application/json")
	if err := c.ApplyAuth(req); err != nil {
		apiErr := &cferrors.APIError{ErrorType: "auth_error", Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitAuth
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "reading response body: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return nil, cferrors.ExitError
	}

	if resp.StatusCode >= 400 {
		apiErr := cferrors.NewFromHTTP(resp.StatusCode, strings.TrimSpace(string(body)), "GET", fullURL, resp)
		apiErr.WriteJSON(c.Stderr)
		return nil, apiErr.ExitCode()
	}

	return body, cferrors.ExitOK
}

func runSearch(cmd *cobra.Command, args []string) error {
	c, err := client.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	cqlQuery, _ := cmd.Flags().GetString("cql")
	if strings.TrimSpace(cqlQuery) == "" {
		apiErr := &cferrors.APIError{ErrorType: "validation_error", Message: "--cql must not be empty"}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// c.BaseURL is "https://domain/wiki/api/v2" in production.
	// v1 search API is at "https://domain/wiki/rest/api/search".
	// We extract the domain and build the v1 URL directly.
	domain := searchV1Domain(c.BaseURL)

	q := url.Values{}
	q.Set("cql", cqlQuery)
	q.Set("limit", "25")
	nextURL := domain + "/wiki/rest/api/search?" + q.Encode()

	var allResults []json.RawMessage

	for {
		// SRCH-03: guard against excessively long cursor URLs (e.g. from Atlassian cursor bloat).
		if len(nextURL) > 4000 {
			fmt.Fprintf(c.Stderr, `{"type":"warning","message":"search cursor URL too long (%d chars); stopping pagination early"}`+"\n", len(nextURL))
			break
		}

		body, code := fetchV1(cmd, c, nextURL)
		if code != cferrors.ExitOK {
			return &cferrors.AlreadyWrittenError{Code: code}
		}

		// Parse v1 search response envelope.
		var page struct {
			Results []json.RawMessage `json:"results"`
			Links   struct {
				Next string `json:"next"`
			} `json:"_links"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to parse search response: " + err.Error()}
			apiErr.WriteJSON(c.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
		}
		allResults = append(allResults, page.Results...)

		// No next page — done.
		if page.Links.Next == "" {
			break
		}

		// _links.next may be an absolute URL or a relative path.
		// Normalize to an absolute URL using the domain we already have.
		nextLink := page.Links.Next
		if strings.HasPrefix(nextLink, "http") {
			// Already absolute — use as-is.
			nextURL = nextLink
		} else {
			// Relative path — prepend domain.
			nextURL = domain + nextLink
		}
	}

	// Marshal merged results as a flat JSON array.
	merged, err := json.Marshal(allResults)
	if err != nil {
		apiErr := &cferrors.APIError{ErrorType: "connection_error", Message: "failed to marshal search results: " + err.Error()}
		apiErr.WriteJSON(c.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}
	if ec := c.WriteOutput(merged); ec != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: ec}
	}
	return nil
}

func init() {
	searchCmd.Flags().String("cql", "", "CQL query string (required), e.g. \"space = ENG AND type = page\"")
	// searchCmd is registered via rootCmd.AddCommand(searchCmd) in cmd/root.go (Plan 04).
	// Do NOT call rootCmd.AddCommand here.
}
