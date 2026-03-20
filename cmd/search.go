package cmd

import (
	"encoding/json"
	"fmt"
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

	// c.BaseURL is the domain only (e.g. "https://example.atlassian.net").
	// c.Fetch() builds: fullURL = c.BaseURL + path.
	// So passing "/wiki/rest/api/search?cql=..." gives the correct v1 URL.
	q := url.Values{}
	q.Set("cql", cqlQuery)
	q.Set("limit", "25")
	initialPath := "/wiki/rest/api/search?" + q.Encode()

	var allResults []json.RawMessage

	nextPath := initialPath
	for {
		// SRCH-03: guard against excessively long cursor URLs (e.g. from Atlassian cursor bloat).
		if len(nextPath) > 4000 {
			fmt.Fprintf(c.Stderr, `{"type":"warning","message":"search cursor URL too long (%d chars); stopping pagination early"}`+"\n", len(nextPath))
			break
		}

		body, code := c.Fetch(cmd.Context(), "GET", nextPath, nil)
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

		// Build next path from _links.next.
		// _links.next may be an absolute URL (e.g. "https://domain/wiki/rest/api/search?cursor=...")
		// or a relative path (e.g. "/wiki/rest/api/search?cursor=...").
		// Since c.BaseURL is the domain only, we need just the path+query portion.
		nextLink := page.Links.Next
		if strings.HasPrefix(nextLink, "http") {
			parsed, err := url.Parse(nextLink)
			if err != nil {
				break
			}
			// RequestURI returns path+query, e.g. "/wiki/rest/api/search?cursor=..."
			nextPath = parsed.RequestURI()
		} else {
			nextPath = nextLink
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
