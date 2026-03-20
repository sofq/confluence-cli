package avatar

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/client"
)

// reHTMLTag matches any HTML/XML tag.
var reHTMLTag = regexp.MustCompile(`<[^>]+>`)

// reCDATA matches CDATA sections, capturing the inner content.
var reCDATA = regexp.MustCompile(`<!\[CDATA\[(.*?)]]>`)

// StripStorageHTML strips HTML/XML tags from Confluence storage format content,
// decodes HTML entities, and collapses whitespace to return clean plain text.
func StripStorageHTML(s string) string {
	if s == "" {
		return ""
	}
	// Replace CDATA sections with their text content before stripping tags.
	s = reCDATA.ReplaceAllString(s, "$1")
	// Strip all HTML/XML tags.
	s = reHTMLTag.ReplaceAllString(s, " ")
	// Decode HTML entities.
	s = html.UnescapeString(s)
	// Collapse whitespace.
	return strings.Join(strings.Fields(s), " ")
}

// contentPage is the Confluence v1 content API response structure.
type contentPage struct {
	Results []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		History struct {
			LastUpdated struct {
				When string `json:"when"`
			} `json:"lastUpdated"`
		} `json:"history"`
	} `json:"results"`
	Links struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// FetchUserPages fetches Confluence pages created by accountID.
// It uses the v1 content API with CQL search and returns up to 200 pages.
// The Body field of each PageRecord contains plain text (HTML stripped).
func FetchUserPages(c *client.Client, accountID string) ([]PageRecord, error) {
	cql := fmt.Sprintf(`creator = "%s" AND type = page ORDER BY lastModified DESC`,
		escapeCQLString(accountID))

	domain := client.SearchV1Domain(c.BaseURL)

	q := url.Values{}
	q.Set("cql", cql)
	q.Set("limit", "50")
	q.Set("expand", "body.storage,version,history.lastUpdated")
	nextURL := domain + "/wiki/rest/api/content?" + q.Encode()

	var records []PageRecord
	const maxPages = 200

	for nextURL != "" && len(records) < maxPages {
		body, err := fetchContentV1(context.TODO(), c, nextURL)
		if err != nil {
			return nil, err
		}

		var page contentPage
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("avatar: failed to parse content response: %w", err)
		}

		for _, r := range page.Results {
			plainText := StripStorageHTML(r.Body.Storage.Value)
			lastMod := parseWhen(r.History.LastUpdated.When)
			records = append(records, PageRecord{
				ID:           r.ID,
				Title:        r.Title,
				Body:         plainText,
				LastModified: lastMod,
			})
		}

		// Follow pagination via _links.next.
		nextLink := page.Links.Next
		if nextLink == "" {
			break
		}
		if strings.HasPrefix(nextLink, "http") {
			nextURL = nextLink
		} else {
			nextURL = domain + nextLink
		}
	}

	return records, nil
}

// fetchContentV1 performs a single GET request against a v1 content URL.
// It applies auth from c and returns the raw response body or an error.
func fetchContentV1(ctx context.Context, c *client.Client, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("avatar: failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if err := c.ApplyAuth(req); err != nil {
		return nil, fmt.Errorf("avatar: failed to apply auth: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("avatar: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("avatar: failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("avatar: HTTP %d from %s: %s", resp.StatusCode, fullURL, strings.TrimSpace(string(body)))
	}

	return body, nil
}

// escapeCQLString escapes a value for interpolation inside a CQL double-quoted string.
func escapeCQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// parseWhen parses a Confluence "when" timestamp (RFC3339 or with milliseconds).
// Returns zero time on parse failure.
func parseWhen(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.999Z07:00",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
