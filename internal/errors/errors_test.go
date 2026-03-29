package errors_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	cferrors "github.com/sofq/confluence-cli/internal/errors"
)

func TestExitCodeFromStatus(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{200, cferrors.ExitOK},
		{201, cferrors.ExitOK},
		{204, cferrors.ExitOK},
		{401, cferrors.ExitAuth},
		{403, cferrors.ExitAuth},
		{404, cferrors.ExitNotFound},
		{410, cferrors.ExitNotFound},
		{400, cferrors.ExitValidation},
		{422, cferrors.ExitValidation},
		{429, cferrors.ExitRateLimit},
		{409, cferrors.ExitConflict},
		{500, cferrors.ExitServer},
		{503, cferrors.ExitServer},
	}

	for _, tc := range cases {
		got := cferrors.ExitCodeFromStatus(tc.status)
		if got != tc.want {
			t.Errorf("ExitCodeFromStatus(%d) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestExitCodeConstants(t *testing.T) {
	if cferrors.ExitOK != 0 {
		t.Errorf("ExitOK should be 0, got %d", cferrors.ExitOK)
	}
	if cferrors.ExitError != 1 {
		t.Errorf("ExitError should be 1, got %d", cferrors.ExitError)
	}
	if cferrors.ExitAuth != 2 {
		t.Errorf("ExitAuth should be 2, got %d", cferrors.ExitAuth)
	}
	if cferrors.ExitNotFound != 3 {
		t.Errorf("ExitNotFound should be 3, got %d", cferrors.ExitNotFound)
	}
	if cferrors.ExitValidation != 4 {
		t.Errorf("ExitValidation should be 4, got %d", cferrors.ExitValidation)
	}
	if cferrors.ExitRateLimit != 5 {
		t.Errorf("ExitRateLimit should be 5, got %d", cferrors.ExitRateLimit)
	}
	if cferrors.ExitConflict != 6 {
		t.Errorf("ExitConflict should be 6, got %d", cferrors.ExitConflict)
	}
	if cferrors.ExitServer != 7 {
		t.Errorf("ExitServer should be 7, got %d", cferrors.ExitServer)
	}
}

func TestNewFromHTTP(t *testing.T) {
	t.Run("404 sets not_found error type", func(t *testing.T) {
		err := cferrors.NewFromHTTP(404, "not found", "GET", "/path", nil)
		if err.ErrorType != "not_found" {
			t.Errorf("ErrorType = %q, want %q", err.ErrorType, "not_found")
		}
		if err.Status != 404 {
			t.Errorf("Status = %d, want 404", err.Status)
		}
		if err.Request == nil || err.Request.Method != "GET" {
			t.Error("Request.Method not set")
		}
	})

	t.Run("HTML body is sanitized", func(t *testing.T) {
		htmlBody := "<!DOCTYPE html><html><body>Internal Server Error</body></html>"
		err := cferrors.NewFromHTTP(401, htmlBody, "GET", "/path", nil)
		if strings.Contains(err.Message, "<") {
			t.Errorf("Message should not contain '<', got: %s", err.Message)
		}
	})

	t.Run("HTML error page not_found", func(t *testing.T) {
		htmlBody := "<html><head><title>Not Found</title></head></html>"
		err := cferrors.NewFromHTTP(404, htmlBody, "GET", "/wiki", nil)
		if strings.Contains(err.Message, "<") {
			t.Errorf("Message contains HTML, got: %s", err.Message)
		}
	})

	t.Run("Retry-After header parsed", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{"Retry-After": []string{"30"}},
		}
		err := cferrors.NewFromHTTP(429, "rate limited", "GET", "/path", resp)
		if err.RetryAfter == nil || *err.RetryAfter != 30 {
			t.Errorf("RetryAfter = %v, want 30", err.RetryAfter)
		}
	})

	t.Run("nil resp no panic", func(t *testing.T) {
		err := cferrors.NewFromHTTP(500, "server error", "POST", "/api", nil)
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if err.RetryAfter != nil {
			t.Error("RetryAfter should be nil when resp is nil")
		}
	})
}

func TestAlreadyWrittenError(t *testing.T) {
	err := &cferrors.AlreadyWrittenError{Code: 3}
	if err.Error() != "error already written" {
		t.Errorf("Error() = %q, want %q", err.Error(), "error already written")
	}
	if err.Code != 3 {
		t.Errorf("Code = %d, want 3", err.Code)
	}
}

func TestAPIErrorWriteJSON(t *testing.T) {
	apiErr := &cferrors.APIError{
		ErrorType: "not_found",
		Status:    404,
		Message:   "resource not found",
	}

	var buf bytes.Buffer
	apiErr.WriteJSON(&buf)

	var decoded map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("WriteJSON output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if _, ok := decoded["error_type"]; !ok {
		t.Errorf("JSON output missing 'error_type' key, got: %s", buf.String())
	}
}

func TestAPIErrorExitCode(t *testing.T) {
	cases := []struct {
		status   int
		wantCode int
	}{
		{401, cferrors.ExitAuth},
		{404, cferrors.ExitNotFound},
		{500, cferrors.ExitServer},
	}
	for _, tc := range cases {
		apiErr := &cferrors.APIError{Status: tc.status}
		if got := apiErr.ExitCode(); got != tc.wantCode {
			t.Errorf("APIError{Status: %d}.ExitCode() = %d, want %d", tc.status, got, tc.wantCode)
		}
	}
}

func TestAPIErrorError(t *testing.T) {
	t.Run("without hint", func(t *testing.T) {
		apiErr := &cferrors.APIError{
			ErrorType: "not_found",
			Status:    404,
			Message:   "page does not exist",
		}
		got := apiErr.Error()
		want := "not_found (status 404): page does not exist"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("with hint", func(t *testing.T) {
		apiErr := &cferrors.APIError{
			ErrorType: "auth_failed",
			Status:    401,
			Message:   "unauthorized",
			Hint:      "check your token",
		}
		got := apiErr.Error()
		want := "auth_failed (status 401): unauthorized \u2014 check your token"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

func TestAPIErrorWriteStderr(t *testing.T) {
	// WriteStderr writes to os.Stderr; we just verify it does not panic.
	apiErr := &cferrors.APIError{
		ErrorType: "server_error",
		Status:    500,
		Message:   "internal server error",
	}
	// This should not panic or return an error.
	apiErr.WriteStderr()
}

func TestExitCodeFromStatusGenericBranches(t *testing.T) {
	cases := []struct {
		status int
		want   int
		label  string
	}{
		// generic 4xx (not one of the named codes)
		{418, cferrors.ExitValidation, "generic 4xx"},
		// default branch: status that doesn't match any range (e.g. 0)
		{0, cferrors.ExitError, "default (0)"},
		// another default: 1xx
		{100, cferrors.ExitError, "default (100)"},
	}
	for _, tc := range cases {
		got := cferrors.ExitCodeFromStatus(tc.status)
		if got != tc.want {
			t.Errorf("ExitCodeFromStatus(%d) [%s] = %d, want %d", tc.status, tc.label, got, tc.want)
		}
	}
}

func TestErrorTypeFromStatus(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{401, "auth_failed"},
		{403, "auth_failed"},
		{404, "not_found"},
		{400, "validation_error"},
		{422, "validation_error"},
		{429, "rate_limited"},
		{409, "conflict"},
		{410, "gone"},
		// generic 4xx
		{418, "client_error"},
		// generic 5xx
		{500, "server_error"},
		{503, "server_error"},
		// default (no match)
		{0, "connection_error"},
		{100, "connection_error"},
	}
	for _, tc := range cases {
		got := cferrors.ErrorTypeFromStatus(tc.status)
		if got != tc.want {
			t.Errorf("ErrorTypeFromStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestHintFromStatus(t *testing.T) {
	cases := []struct {
		status int
		empty  bool
		label  string
	}{
		{401, false, "401 hint present"},
		{403, false, "403 hint present"},
		{429, false, "429 hint present"},
		{404, true, "404 no hint"},
		{500, true, "500 no hint"},
	}
	for _, tc := range cases {
		got := cferrors.HintFromStatus(tc.status)
		if tc.empty && got != "" {
			t.Errorf("HintFromStatus(%d) [%s] = %q, want empty string", tc.status, tc.label, got)
		}
		if !tc.empty && got == "" {
			t.Errorf("HintFromStatus(%d) [%s] returned empty string, want non-empty hint", tc.status, tc.label)
		}
	}
}
