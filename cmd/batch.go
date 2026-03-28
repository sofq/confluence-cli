package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/sofq/confluence-cli/cmd/generated"
	"github.com/sofq/confluence-cli/internal/client"
	cferrors "github.com/sofq/confluence-cli/internal/errors"
	"github.com/sofq/confluence-cli/internal/jq"
	"github.com/sofq/confluence-cli/internal/jsonutil"
	"github.com/spf13/cobra"
)

// BatchOp represents a single operation in a batch request.
type BatchOp struct {
	Command string            `json:"command"`
	Args    map[string]string `json:"args"`
	JQ      string            `json:"jq,omitempty"`
}

// BatchResult holds the result of a single batch operation.
type BatchResult struct {
	Index    int             `json:"index"`
	ExitCode int             `json:"exit_code"`
	Data     json.RawMessage `json:"data,omitempty"`
	Error    json.RawMessage `json:"error,omitempty"`
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Execute multiple API operations in a single invocation",
	Long: `Execute multiple Confluence API operations from a JSON array.

Input JSON array format:
  [
    {"command": "pages get-by-id", "args": {"id": "123"}, "jq": ".title"},
    {"command": "spaces get", "args": {}}
  ]

Output JSON array format:
  [
    {"index": 0, "exit_code": 0, "data": "My Page"},
    {"index": 1, "exit_code": 0, "data": {...}}
  ]

Use --input to specify a file, or pipe JSON to stdin.`,
	RunE: runBatch,
}

func init() {
	batchCmd.Flags().String("input", "", "path to JSON input file (reads stdin if not set)")
	batchCmd.Flags().Int("max-batch", 50, "maximum number of operations per batch")
	rootCmd.AddCommand(batchCmd)
	// Phase 4: batch command registered in cmd/batch.go init()
}

func runBatch(cmd *cobra.Command, args []string) error {
	// Get the base client from context (configured in PersistentPreRunE).
	baseClient, err := client.FromContext(cmd.Context())
	if err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "config_error",
			Status:    0,
			Message:   err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitError}
	}

	// Read input.
	inputFlag, _ := cmd.Flags().GetString("input")

	var inputData []byte
	if inputFlag != "" {
		inputData, err = os.ReadFile(inputFlag)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Status:    0,
				Message:   "cannot read input file: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
	} else {
		// Read from stdin.
		fi, statErr := os.Stdin.Stat()
		if statErr != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Status:    0,
				Message:   "no input provided: use --input <file> or pipe JSON to stdin",
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		inputData, err = io.ReadAll(os.Stdin)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "validation_error",
				Status:    0,
				Message:   "failed to read stdin: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
	}

	// Parse the batch ops.
	var ops []BatchOp
	if err := json.Unmarshal(inputData, &ops); err != nil {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Status:    0,
			Message:   "invalid JSON input: expected a JSON array of operations; " + err.Error(),
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Reject null/empty input explicitly.
	if ops == nil {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Status:    0,
			Message:   "invalid JSON input: expected a JSON array of operations, got null",
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Enforce batch size limit.
	maxBatch, _ := cmd.Flags().GetInt("max-batch")
	if maxBatch > 0 && len(ops) > maxBatch {
		apiErr := &cferrors.APIError{
			ErrorType: "validation_error",
			Message:   fmt.Sprintf("batch limit exceeded: %d operations, max %d", len(ops), maxBatch),
			Hint:      "Use --max-batch to increase the limit",
		}
		apiErr.WriteJSON(os.Stderr)
		return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
	}

	// Build opMap from generated.AllSchemaOps() — key is "resource verb".
	allOps := generated.AllSchemaOps()
	opMap := make(map[string]generated.SchemaOp, len(allOps))
	for _, op := range allOps {
		key := op.Resource + " " + op.Verb
		opMap[key] = op
	}

	// Execute each operation and collect results.
	results := make([]BatchResult, len(ops))
	ctx := cmd.Context()
	for i, bop := range ops {
		results[i] = executeBatchOp(ctx, baseClient, i, bop, opMap)
	}

	// Write all results as a JSON array to stdout.
	// BatchResult contains only int, string, and json.RawMessage fields —
	// MarshalNoEscape cannot fail.
	output, _ := jsonutil.MarshalNoEscape(results)

	// Apply global --jq filter to the batch output array.
	if baseClient.JQFilter != "" {
		filtered, err := jq.Apply(output, baseClient.JQFilter)
		if err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "jq_error",
				Status:    0,
				Message:   "jq: " + err.Error(),
			}
			apiErr.WriteJSON(os.Stderr)
			return &cferrors.AlreadyWrittenError{Code: cferrors.ExitValidation}
		}
		output = filtered
	}

	// Apply --pretty to the batch output using encoding/json.Indent (not tidwall/pretty).
	if baseClient.Pretty {
		var indented bytes.Buffer
		if err := json.Indent(&indented, output, "", "  "); err == nil {
			output = indented.Bytes()
		}
	}

	fmt.Fprintf(os.Stdout, "%s\n", output)

	// Exit with highest-severity exit code from batch operations.
	maxExit := 0
	for _, r := range results {
		if r.ExitCode > maxExit {
			maxExit = r.ExitCode
		}
	}
	if maxExit != 0 {
		return &cferrors.AlreadyWrittenError{Code: maxExit}
	}

	return nil
}

// executeBatchOp runs a single batch operation and returns its result.
func executeBatchOp(
	ctx context.Context,
	baseClient *client.Client,
	index int,
	bop BatchOp,
	opMap map[string]generated.SchemaOp,
) BatchResult {
	// Look up the schema op.
	schemaOp, ok := opMap[bop.Command]
	if !ok {
		errMsg := fmt.Sprintf("unknown command %q", bop.Command)
		return errorResult(index, cferrors.ExitError, "validation_error", errMsg)
	}

	// Check policy for this operation (explicit check for clean batch error formatting).
	if baseClient.Policy != nil {
		if err := baseClient.Policy.Check(bop.Command); err != nil {
			apiErr := &cferrors.APIError{
				ErrorType: "policy_denied",
				Message:   err.Error(),
			}
			encoded, _ := json.Marshal(apiErr)
			return BatchResult{
				Index:    index,
				ExitCode: cferrors.ExitValidation,
				Error:    json.RawMessage(encoded),
			}
		}
	}

	// Create a per-operation client with captured stdout/stderr.
	var stdoutBuf strings.Builder
	var stderrBuf strings.Builder

	opClient := &client.Client{
		BaseURL:     baseClient.BaseURL,
		Auth:        baseClient.Auth,
		HTTPClient:  baseClient.HTTPClient,
		Stdout:      &stdoutBuf,
		Stderr:      &stderrBuf,
		JQFilter:    bop.JQ,
		Paginate:    baseClient.Paginate,
		DryRun:      baseClient.DryRun,
		Verbose:     baseClient.Verbose,
		Pretty:      false, // Pretty is applied to the batch output array, not per-op.
		Fields:      baseClient.Fields,
		CacheTTL:    baseClient.CacheTTL,
		AuditLogger: baseClient.AuditLogger,
		Profile:     baseClient.Profile,
		Policy:      baseClient.Policy,
		Operation:   bop.Command,
	}

	// Build path by substituting {paramName} placeholders.
	path := schemaOp.Path
	for _, flag := range schemaOp.Flags {
		if flag.In == "path" {
			placeholder := "{" + flag.Name + "}"
			if val, exists := bop.Args[flag.Name]; exists && val != "" {
				path = strings.ReplaceAll(path, placeholder, url.PathEscape(val))
			} else if flag.Required && strings.Contains(path, placeholder) {
				errMsg := fmt.Sprintf("missing required path parameter %q", flag.Name)
				return errorResult(index, cferrors.ExitValidation, "validation_error", errMsg)
			}
		}
	}

	// Build query params from args that match "query" flags in the schema.
	query := url.Values{}
	for _, flag := range schemaOp.Flags {
		if flag.In == "query" {
			if val, exists := bop.Args[flag.Name]; exists {
				query.Set(flag.Name, val)
			}
		}
	}

	// Handle body.
	var body io.Reader
	if bodyStr, exists := bop.Args["body"]; exists && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	exitCode := opClient.Do(ctx, schemaOp.Method, path, query, body)

	return buildBatchResult(index, exitCode, &stdoutBuf, &stderrBuf, baseClient.Verbose)
}

// stripVerboseLogs separates verbose log lines from error lines in captured stderr.
// Verbose lines (with "type":"request"/"response") are forwarded to real stderr.
// Returns only the non-verbose (error) lines joined as a single string.
func stripVerboseLogs(stderrStr string) string {
	var errorLines []string
	for _, line := range strings.Split(strings.TrimSpace(stderrStr), "\n") {
		if line == "" {
			continue
		}
		var parsed map[string]any
		if json.Unmarshal([]byte(line), &parsed) == nil {
			if tp, ok := parsed["type"].(string); ok && (tp == "request" || tp == "response") {
				fmt.Fprintln(os.Stderr, line)
				continue
			}
		}
		errorLines = append(errorLines, line)
	}
	return strings.Join(errorLines, "\n")
}

// parseErrorJSON parses stderr output into a json.RawMessage.
// Handles single JSON objects, multiple JSON lines, and plain text.
func parseErrorJSON(errOutput string) json.RawMessage {
	if json.Valid([]byte(errOutput)) {
		return json.RawMessage(errOutput)
	}
	lines := strings.Split(errOutput, "\n")
	if len(lines) > 1 {
		var jsonLines []json.RawMessage
		allValid := true
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if json.Valid([]byte(line)) {
				jsonLines = append(jsonLines, json.RawMessage(line))
			} else {
				allValid = false
				break
			}
		}
		// If all non-empty lines are valid JSON but the whole string is not
		// (checked above), there must be multiple JSON values.
		if allValid && len(jsonLines) > 0 {
			arrBytes, _ := json.Marshal(jsonLines)
			return json.RawMessage(arrBytes)
		}
	}
	encoded, _ := json.Marshal(map[string]string{"message": errOutput})
	return json.RawMessage(encoded)
}

// buildBatchResult constructs a BatchResult from captured stdout/stderr.
// Verbose log lines (from --verbose) are forwarded to real stderr so they
// are visible to the caller, while structured error JSON is kept in the result.
func buildBatchResult(index, exitCode int, stdoutBuf, stderrBuf *strings.Builder, verbose bool) BatchResult {
	stderrStr := stderrBuf.String()

	if verbose && stderrStr != "" {
		stderrStr = stripVerboseLogs(stderrStr)
	}

	if exitCode != cferrors.ExitOK {
		return BatchResult{
			Index:    index,
			ExitCode: exitCode,
			Error:    parseErrorJSON(strings.TrimSpace(stderrStr)),
		}
	}

	outStr := strings.TrimSpace(stdoutBuf.String())
	var rawData json.RawMessage
	if outStr != "" && json.Valid([]byte(outStr)) {
		rawData = json.RawMessage(outStr)
	} else if outStr != "" {
		encoded, _ := json.Marshal(outStr)
		rawData = json.RawMessage(encoded)
	}

	return BatchResult{
		Index:    index,
		ExitCode: cferrors.ExitOK,
		Data:     rawData,
	}
}

// errorResult constructs a BatchResult for an error condition.
func errorResult(index, exitCode int, errType, message string) BatchResult {
	apiErr := &cferrors.APIError{
		ErrorType: errType,
		Status:    0,
		Message:   message,
	}
	encoded, _ := json.Marshal(apiErr)
	return BatchResult{
		Index:    index,
		ExitCode: exitCode,
		Error:    json.RawMessage(encoded),
	}
}
