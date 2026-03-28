package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

var rawCmd = &cobra.Command{
	Use:   "raw <method> <path>",
	Short: "Execute a raw Confluence API call",
	Args:  cobra.ExactArgs(2),
	RunE:  runRaw,
}

func init() {
	f := rawCmd.Flags()
	f.String("body", "", "request body as a JSON string or @filename")
	f.StringArray("query", nil, "query parameters as key=value (repeatable)")
}

// validHTTPMethods is the set of allowed HTTP methods.
var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// methodsWithBody lists HTTP methods that typically require a request body.
var methodsWithBody = map[string]bool{
	"POST": true, "PUT": true, "PATCH": true,
}

func runRaw(cmd *cobra.Command, args []string) error {
	method := strings.ToUpper(args[0])
	path := args[1]

	// Validate HTTP method client-side.
	if !validHTTPMethods[method] {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   fmt.Sprintf("invalid HTTP method %q; must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS", method),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	c, err := client.FromContext(cmd.Context())
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Message:   err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	// Build query values.
	queryPairs, _ := cmd.Flags().GetStringArray("query")
	q := url.Values{}
	for _, pair := range queryPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   fmt.Sprintf("invalid --query %q: expected key=value format", pair),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		q.Add(parts[0], parts[1])
	}

	// Resolve body reader.
	var bodyReader io.Reader

	bodyFlag, _ := cmd.Flags().GetString("body")
	switch {
	case bodyFlag == "-":
		// Explicit stdin: --body -
		bodyReader = os.Stdin
	case bodyFlag != "" && strings.HasPrefix(bodyFlag, "@"):
		filename := bodyFlag[1:]
		if filename == "" {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "--body @<filename> requires a filename after @",
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		f, err := os.Open(filename)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Message:   "cannot open body file: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		defer f.Close()
		bodyReader = f
	case bodyFlag != "":
		bodyReader = strings.NewReader(bodyFlag)
	default:
		// Don't auto-read stdin for raw commands — require explicit --body - instead.
		// This prevents hanging when no body is piped.
	}

	// Strip /wiki/api/v2 prefix from path if present, since client.Do already
	// prepends BaseURL (which includes /wiki/api/v2). This prevents double-prefixing
	// when users provide the full API path (e.g. /wiki/api/v2/spaces).
	for _, prefix := range []string{"/wiki/api/v2", "wiki/api/v2"} {
		if strings.HasPrefix(path, prefix) {
			path = path[len(prefix):]
			break
		}
	}

	// Warn if --body is used with GET/HEAD/DELETE/OPTIONS.
	if bodyFlag != "" && !methodsWithBody[method] {
		warnMsg := map[string]any{
			"type":    "warning",
			"message": fmt.Sprintf("--body is ignored for %s requests", method),
		}
		warnJSON, _ := json.Marshal(warnMsg)
		fmt.Fprintf(c.Stderr, "%s\n", warnJSON)
		bodyReader = nil
	}

	// If method needs a body but none was provided, error instead of hanging on stdin.
	// Skip this check in dry-run mode to match generated command behavior.
	if methodsWithBody[method] && bodyReader == nil && !c.DryRun {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   fmt.Sprintf("%s request requires a body; use --body '{...}' or pipe JSON to stdin", method),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	code := c.Do(cmd.Context(), method, path, q, bodyReader)
	if code != cferrors.ExitOK {
		return &cferrors.AlreadyWrittenError{Code: code}
	}
	return nil
}
