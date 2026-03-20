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
