package cmd_test

// batch_internal_test.go tests internal batch helper functions directly via
// white-box exports in export_test.go.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

// TestParseErrorJSON_ValidSingleJSONObject verifies single valid JSON object is returned as-is.
func TestParseErrorJSON_ValidSingleJSONObject(t *testing.T) {
	input := `{"error_type":"not_found","message":"page not found","status":404}`
	result := cmd.ParseErrorJSON(input)

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", result)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out["error_type"] != "not_found" {
		t.Errorf("error_type: want not_found, got %v", out["error_type"])
	}
}

// TestParseErrorJSON_MultipleJSONLines verifies multiple valid JSON lines are wrapped in an array.
func TestParseErrorJSON_MultipleJSONLines(t *testing.T) {
	line1 := `{"type":"request","method":"GET","url":"/pages"}`
	line2 := `{"type":"response","status":404}`
	input := line1 + "\n" + line2

	result := cmd.ParseErrorJSON(input)

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", result)
	}
	var arr []interface{}
	if err := json.Unmarshal(result, &arr); err != nil {
		t.Fatalf("expected JSON array but got: %s\nerr: %v", result, err)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 elements in array, got %d; result: %s", len(arr), result)
	}
}

// TestParseErrorJSON_MultipleJSONLinesWithInvalidLine verifies that if any line is invalid JSON,
// the output falls back to a plain text wrapper.
func TestParseErrorJSON_MultipleJSONLinesWithInvalidLine(t *testing.T) {
	line1 := `{"error_type":"partial"}`
	line2 := `not valid json at all`
	input := line1 + "\n" + line2

	result := cmd.ParseErrorJSON(input)

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", result)
	}
	// Should fall back to wrapping the whole string as a "message" field
	var out map[string]interface{}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := out["message"]; !ok {
		t.Errorf("expected 'message' key in fallback, got: %v", out)
	}
}

// TestParseErrorJSON_PlainTextWrapped verifies that plain (non-JSON) text is wrapped as message.
func TestParseErrorJSON_PlainTextWrapped(t *testing.T) {
	input := "something went totally wrong"
	result := cmd.ParseErrorJSON(input)

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", result)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out["message"] != input {
		t.Errorf("message: want %q, got %q", input, out["message"])
	}
}

// TestParseErrorJSON_EmptyString verifies that an empty string is handled gracefully.
func TestParseErrorJSON_EmptyString(t *testing.T) {
	result := cmd.ParseErrorJSON("")

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", result)
	}
}

// TestParseErrorJSON_MultipleJSONLinesAllEmpty verifies lines that are all empty/whitespace.
func TestParseErrorJSON_MultipleJSONLinesAllEmpty(t *testing.T) {
	input := "\n\n\n"
	result := cmd.ParseErrorJSON(input)

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", result)
	}
}

// TestStripVerboseLogs_RequestResponseForwarded verifies that request/response type lines
// are removed from the return value (they are forwarded to os.Stderr).
func TestStripVerboseLogs_RequestResponseForwarded(t *testing.T) {
	requestLine := `{"type":"request","method":"GET","url":"/pages"}`
	responseLine := `{"type":"response","status":200,"body":{}}`
	errorLine := `{"error_type":"not_found","message":"page not found"}`

	input := requestLine + "\n" + responseLine + "\n" + errorLine

	result := cmd.StripVerboseLogs(input)

	// The result should only contain the error line, not the request/response lines
	if strings.Contains(result, `"type":"request"`) {
		t.Error("stripVerboseLogs should remove request lines from result")
	}
	if strings.Contains(result, `"type":"response"`) {
		t.Error("stripVerboseLogs should remove response lines from result")
	}
	if !strings.Contains(result, "not_found") {
		t.Errorf("stripVerboseLogs should retain error lines, got: %q", result)
	}
}

// TestStripVerboseLogs_OnlyVerboseLines verifies that output is empty when all lines are verbose.
func TestStripVerboseLogs_OnlyVerboseLines(t *testing.T) {
	requestLine := `{"type":"request","method":"GET","url":"/pages"}`
	responseLine := `{"type":"response","status":200}`

	input := requestLine + "\n" + responseLine

	result := cmd.StripVerboseLogs(input)

	// All lines were verbose, so result should be empty
	if strings.TrimSpace(result) != "" {
		t.Errorf("expected empty result when all lines are verbose, got: %q", result)
	}
}

// TestStripVerboseLogs_NonJSONLine verifies that non-JSON lines are kept as error lines.
func TestStripVerboseLogs_NonJSONLine(t *testing.T) {
	input := "some plain text error\nanother error line"

	result := cmd.StripVerboseLogs(input)

	if !strings.Contains(result, "some plain text error") {
		t.Errorf("expected plain text error lines to be kept, got: %q", result)
	}
}

// TestStripVerboseLogs_EmptyInput verifies empty input returns empty string.
func TestStripVerboseLogs_EmptyInput(t *testing.T) {
	result := cmd.StripVerboseLogs("")

	if result != "" {
		t.Errorf("expected empty result for empty input, got: %q", result)
	}
}

// TestStripVerboseLogs_EmptyLines verifies that empty lines are skipped.
func TestStripVerboseLogs_EmptyLines(t *testing.T) {
	errorLine := `{"error_type":"not_found","message":"not found"}`
	input := "\n" + errorLine + "\n\n"

	result := cmd.StripVerboseLogs(input)

	if !strings.Contains(result, "not_found") {
		t.Errorf("expected error line to be kept, got: %q", result)
	}
}

// TestStripVerboseLogs_JSONWithOtherType verifies that JSON lines with non-request/response
// types are kept as error lines.
func TestStripVerboseLogs_JSONWithOtherType(t *testing.T) {
	// A JSON line with type "warning" (not request/response) should be kept
	warningLine := `{"type":"warning","message":"some warning"}`
	requestLine := `{"type":"request","method":"GET"}`

	input := requestLine + "\n" + warningLine

	result := cmd.StripVerboseLogs(input)

	// warningLine should be kept
	if !strings.Contains(result, "warning") {
		t.Errorf("expected warning line to be kept, got: %q", result)
	}
	// requestLine should be removed
	if strings.Contains(result, "request") {
		t.Errorf("expected request line to be removed, got: %q", result)
	}
}
