package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

// helper sets up env vars pointing BaseURL at the given test server.
func setupAttachmentEnv(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("CF_BASE_URL", srvURL+"/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")
}

// captureOutput redirects os.Stdout and os.Stderr, runs fn, and returns captured output.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout := os.Stdout
	rout, wout, _ := os.Pipe()
	os.Stdout = wout

	oldStderr := os.Stderr
	rerr, werr, _ := os.Pipe()
	os.Stderr = werr

	fn()

	wout.Close()
	werr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rout)
	_, _ = errBuf.ReadFrom(rerr)
	return outBuf.String(), errBuf.String()
}

// --- List subcommand tests ---

func TestAttachmentsList_EmptyPageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	_, stderrOut := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "list", "--page-id", ""})
		err := root.Execute()
		if err == nil {
			t.Error("expected error for empty --page-id, got nil")
		}
	})

	if !strings.Contains(stderrOut, "page-id must not be empty") {
		t.Errorf("expected validation error about --page-id, got stderr: %s", stderrOut)
	}
}

func TestAttachmentsList_ValidPageID(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{},
			"_links":  map[string]any{},
		})
	}))
	defer srv.Close()

	// For v2 path via c.Do, BaseURL is used directly (no /wiki/api/v2 prefix needed for path matching).
	t.Setenv("CF_BASE_URL", srv.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/no-config.json")

	captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "list", "--page-id", "12345"})
		_ = root.Execute()
	})

	if capturedMethod != "GET" {
		t.Errorf("expected GET, got %q", capturedMethod)
	}
	if capturedPath != "/pages/12345/attachments" {
		t.Errorf("expected path /pages/12345/attachments, got %q", capturedPath)
	}
}

// --- Upload subcommand tests ---

func TestAttachmentsUpload_EmptyPageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	_, stderrOut := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "upload", "--page-id", "", "--file", "/tmp/x.pdf"})
		err := root.Execute()
		if err == nil {
			t.Error("expected error for empty --page-id")
		}
	})

	if !strings.Contains(stderrOut, "page-id must not be empty") {
		t.Errorf("expected page-id validation error, got stderr: %s", stderrOut)
	}
}

func TestAttachmentsUpload_EmptyFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	_, stderrOut := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "upload", "--page-id", "123", "--file", ""})
		err := root.Execute()
		if err == nil {
			t.Error("expected error for empty --file")
		}
	})

	if !strings.Contains(stderrOut, "file must not be empty") {
		t.Errorf("expected file validation error, got stderr: %s", stderrOut)
	}
}

func TestAttachmentsUpload_NonexistentFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during validation test")
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	_, stderrOut := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "upload", "--page-id", "123", "--file", "/tmp/nonexistent-file-xyz.pdf"})
		err := root.Execute()
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	if !strings.Contains(stderrOut, "cannot open file") {
		t.Errorf("expected 'cannot open file' error, got stderr: %s", stderrOut)
	}
}

func TestAttachmentsUpload_MultipartAndHeaders(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	var capturedHeaders http.Header
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		capturedHeaders = r.Header.Clone()
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"att-1","title":"report.pdf"}]`))
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	// Create a temp file to upload.
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "report.pdf")
	if err := os.WriteFile(filePath, []byte("fake-pdf-content"), 0644); err != nil {
		t.Fatal(err)
	}

	captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "upload", "--page-id", "page-42", "--file", filePath})
		err := root.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Check method
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %q", capturedMethod)
	}

	// Check v1 URL path
	if capturedPath != "/wiki/rest/api/content/page-42/child/attachment" {
		t.Errorf("expected v1 path /wiki/rest/api/content/page-42/child/attachment, got %q", capturedPath)
	}

	// Check X-Atlassian-Token header
	if capturedHeaders.Get("X-Atlassian-Token") != "no-check" {
		t.Errorf("expected X-Atlassian-Token: no-check, got %q", capturedHeaders.Get("X-Atlassian-Token"))
	}

	// Check Content-Type is multipart/form-data with boundary
	ct := capturedHeaders.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("failed to parse Content-Type %q: %v", ct, err)
	}
	if mediaType != "multipart/form-data" {
		t.Errorf("expected multipart/form-data, got %q", mediaType)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("expected boundary in Content-Type, got none")
	}

	// Parse multipart body and check field name and filename
	reader := multipart.NewReader(bytes.NewReader(capturedBody), boundary)
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("failed to read multipart part: %v", err)
	}
	if part.FormName() != "file" {
		t.Errorf("expected form field name 'file', got %q", part.FormName())
	}
	if part.FileName() != "report.pdf" {
		t.Errorf("expected filename 'report.pdf', got %q", part.FileName())
	}
	partData, _ := io.ReadAll(part)
	if string(partData) != "fake-pdf-content" {
		t.Errorf("expected file content 'fake-pdf-content', got %q", string(partData))
	}

	// Check Accept header
	if capturedHeaders.Get("Accept") != "application/json" {
		t.Errorf("expected Accept: application/json, got %q", capturedHeaders.Get("Accept"))
	}
}

func TestAttachmentsUpload_UsesSearchV1Domain(t *testing.T) {
	var capturedHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"att-1"}]`))
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "upload", "--page-id", "p1", "--file", filePath})
		_ = root.Execute()
	})

	// The request should go to the same host as the server (searchV1Domain extracts scheme+host)
	if capturedHost == "" {
		t.Error("expected a request to the server, got none")
	}
}

func TestAttachmentsUpload_DryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call during dry-run test")
	}))
	defer srv.Close()
	setupAttachmentEnv(t, srv.URL)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "report.pdf")
	fileContent := []byte("fake-pdf-content-for-dryrun")
	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		t.Fatal(err)
	}

	stdoutOut, _ := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"attachments", "upload", "--page-id", "p1", "--file", filePath, "--dry-run"})
		err := root.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	stdoutOut = strings.TrimSpace(stdoutOut)
	if stdoutOut == "" {
		t.Fatal("expected dry-run JSON output, got nothing")
	}

	var dryOut map[string]any
	if err := json.Unmarshal([]byte(stdoutOut), &dryOut); err != nil {
		t.Fatalf("dry-run output is not valid JSON: %v\nOutput: %s", err, stdoutOut)
	}

	if dryOut["method"] != "POST" {
		t.Errorf("expected method=POST, got %v", dryOut["method"])
	}
	if url, ok := dryOut["url"].(string); !ok || !strings.Contains(url, "/wiki/rest/api/content/p1/child/attachment") {
		t.Errorf("expected url containing v1 attachment path, got %v", dryOut["url"])
	}
	if dryOut["filename"] != "report.pdf" {
		t.Errorf("expected filename=report.pdf, got %v", dryOut["filename"])
	}
	if size, ok := dryOut["fileSize"].(float64); !ok || int(size) != len(fileContent) {
		t.Errorf("expected fileSize=%d, got %v", len(fileContent), dryOut["fileSize"])
	}
}
