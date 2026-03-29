package cmd_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

// writePolicyConfig writes a minimal config.json with the given profile settings
// to dir/config.json and returns the path.
func writePolicyConfig(t *testing.T, dir string, allowedOps, deniedOps []string, auditLog string) string {
	t.Helper()
	type authCfg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	type profileCfg struct {
		BaseURL           string   `json:"base_url"`
		Auth              authCfg  `json:"auth"`
		AllowedOperations []string `json:"allowed_operations,omitempty"`
		DeniedOperations  []string `json:"denied_operations,omitempty"`
		AuditLog          string   `json:"audit_log,omitempty"`
	}
	type cfgFile struct {
		DefaultProfile string                `json:"default_profile"`
		Profiles       map[string]profileCfg `json:"profiles"`
	}

	// base_url is set to a placeholder; tests override with CF_BASE_URL env var.
	cfg := cfgFile{
		DefaultProfile: "default",
		Profiles: map[string]profileCfg{
			"default": {
				BaseURL:           "http://placeholder",
				Auth:              authCfg{Type: "bearer", Token: "test-token"},
				AllowedOperations: allowedOps,
				DeniedOperations:  deniedOps,
				AuditLog:          auditLog,
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("writePolicyConfig: marshal: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writePolicyConfig: write: %v", err)
	}
	return path
}

// captureExecute runs rootCmd with the given args, capturing stdout and stderr
// via OS pipes. Returns captured stdout, stderr strings, and the exit code.
func captureExecute(t *testing.T, args []string) (stdout, stderr string, exitCode int) {
	t.Helper()

	// Capture stdout.
	oldOut := os.Stdout
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("captureExecute: pipe stdout: %v", err)
	}
	os.Stdout = wOut

	// Capture stderr.
	oldErr := os.Stderr
	rErr, wErr, errPipe := os.Pipe()
	if errPipe != nil {
		t.Fatalf("captureExecute: pipe stderr: %v", errPipe)
	}
	os.Stderr = wErr

	t.Cleanup(func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
	})

	root := cmd.RootCommand()
	root.SetArgs(args)
	exitCode = cmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)

	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), exitCode
}

// neverRequestServer returns an httptest.Server that fails the test if any
// request reaches it (used to verify policy blocks before HTTP call).
func neverRequestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected HTTP request reached server: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// okServer returns an httptest.Server that always responds 200 with {"ok":true}.
func okServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(ts.Close)
	return ts
}

// policyTestBaseURL sets CF_BASE_URL to just the server URL (no path suffix).
// The raw command path includes the full API path (e.g. /wiki/api/v2/spaces).
// Operation name fallback: "GET /wiki/api/v2/spaces" — note: path.Match "*" does
// NOT match strings with slashes, so patterns must use exact or per-segment globs.
const rawSpacesPath = "/wiki/api/v2/spaces"
const rawSpacesOp = "GET " + rawSpacesPath // operation name produced by raw command fallback

// TestPolicyAllowListDeniesUnmatchedOperation verifies that an allow-only profile
// blocks an operation not in allowed_operations before making any HTTP request.
func TestPolicyAllowListDeniesUnmatchedOperation(t *testing.T) {
	ts := neverRequestServer(t)
	dir := t.TempDir()
	// Allow only "pages:get"; "GET /wiki/api/v2/spaces" does not match.
	cfgPath := writePolicyConfig(t, dir, []string{"pages:get"}, nil, "")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL) // raw command path is the full /wiki/api/v2/spaces
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	_, stderr, exitCode := captureExecute(t, []string{"raw", "GET", rawSpacesPath, "--cache", "0s"})

	if exitCode != 4 {
		t.Errorf("exit code = %d, want 4 (ExitValidation)", exitCode)
	}

	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderr), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderr)
	}
	if errOut["error_type"] != "policy_denied" {
		t.Errorf("error_type = %v, want policy_denied", errOut["error_type"])
	}
}

// TestPolicyAllowListPermitsMatchingOperation verifies that an allow-only profile
// permits an operation matching allowed_operations and makes the HTTP request.
func TestPolicyAllowListPermitsMatchingOperation(t *testing.T) {
	ts := okServer(t)
	dir := t.TempDir()
	// Allow the exact operation name that the raw command produces.
	cfgPath := writePolicyConfig(t, dir, []string{rawSpacesOp}, nil, "")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	stdout, _, exitCode := captureExecute(t, []string{"raw", "GET", rawSpacesPath, "--no-paginate", "--cache", "0s"})

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if stdout == "" {
		t.Error("expected stdout output, got nothing")
	}
}

// TestPolicyDenyListDeniesMatchingOperation verifies that a deny-list profile
// blocks matching operations before any HTTP request.
func TestPolicyDenyListDeniesMatchingOperation(t *testing.T) {
	ts := neverRequestServer(t)
	dir := t.TempDir()
	// Deny the exact operation name that raw produces.
	cfgPath := writePolicyConfig(t, dir, nil, []string{rawSpacesOp}, "")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	_, stderr, exitCode := captureExecute(t, []string{"raw", "GET", rawSpacesPath, "--cache", "0s"})

	if exitCode != 4 {
		t.Errorf("exit code = %d, want 4 (ExitValidation)", exitCode)
	}

	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderr), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderr)
	}
	if errOut["error_type"] != "policy_denied" {
		t.Errorf("error_type = %v, want policy_denied", errOut["error_type"])
	}
}

// TestPolicyDryRunWithDenyingPolicyExitsCode4 verifies that --dry-run with a
// denying policy still exits with code 4 (policy enforced before DryRun block).
func TestPolicyDryRunWithDenyingPolicyExitsCode4(t *testing.T) {
	ts := neverRequestServer(t)
	dir := t.TempDir()
	cfgPath := writePolicyConfig(t, dir, nil, []string{rawSpacesOp}, "")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	_, stderr, exitCode := captureExecute(t, []string{"raw", "GET", rawSpacesPath, "--dry-run", "--cache", "0s"})

	if exitCode != 4 {
		t.Errorf("exit code = %d, want 4 (ExitValidation)", exitCode)
	}

	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderr), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderr)
	}
	if errOut["error_type"] != "policy_denied" {
		t.Errorf("error_type = %v, want policy_denied", errOut["error_type"])
	}
}

// TestPolicyNoFieldsBehavesNormally verifies that a profile with no
// allowed_operations or denied_operations is unrestricted.
func TestPolicyNoFieldsBehavesNormally(t *testing.T) {
	ts := okServer(t)
	dir := t.TempDir()
	cfgPath := writePolicyConfig(t, dir, nil, nil, "")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	stdout, _, exitCode := captureExecute(t, []string{"raw", "GET", rawSpacesPath, "--no-paginate", "--cache", "0s"})

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if stdout == "" {
		t.Error("expected stdout output, got nothing")
	}
}

// TestAuditLogWritesNDJSONEntry verifies that a successful API call with --audit
// writes exactly one NDJSON line containing method, path, and status.
func TestAuditLogWritesNDJSONEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(ts.Close)

	dir := t.TempDir()
	cfgPath := writePolicyConfig(t, dir, nil, nil, "")
	auditPath := filepath.Join(dir, "audit.log")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	_, _, exitCode := captureExecute(t, []string{
		"raw", "GET", rawSpacesPath,
		"--audit", auditPath,
		"--no-paginate",
		"--cache", "0s",
		"--dry-run=false",
	})

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}

	// Read audit log.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not created: %v", err)
	}

	lines := countNDJSONLines(data)
	if lines != 1 {
		t.Errorf("audit log has %d lines, want 1\nContent: %s", lines, string(data))
	}

	// Parse the single entry.
	firstLine := bytes.TrimRight(data, "\n")
	if idx := bytes.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	var entry map[string]interface{}
	if err := json.Unmarshal(firstLine, &entry); err != nil {
		t.Fatalf("audit entry is not valid JSON: %v\nContent: %s", err, string(data))
	}
	if entry["method"] != "GET" {
		t.Errorf("audit entry method = %v, want GET", entry["method"])
	}
	path, _ := entry["path"].(string)
	if !strings.Contains(path, "/spaces") {
		t.Errorf("audit entry path = %q, want it to contain /spaces", path)
	}
	status, _ := entry["status"].(float64)
	if status != 200 {
		t.Errorf("audit entry status = %v, want 200", entry["status"])
	}
}

// TestAuditLogNoPolicyDeniedEntry verifies that a policy-denied call does not
// write an audit entry (denial happens before the HTTP call and audit logging).
func TestAuditLogNoPolicyDeniedEntry(t *testing.T) {
	ts := neverRequestServer(t)
	dir := t.TempDir()
	cfgPath := writePolicyConfig(t, dir, nil, []string{rawSpacesOp}, "")
	auditPath := filepath.Join(dir, "audit.log")

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", ts.URL)
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test-token")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	_, _, exitCode := captureExecute(t, []string{
		"raw", "GET", rawSpacesPath,
		"--audit", auditPath,
		"--cache", "0s",
	})

	if exitCode != 4 {
		t.Errorf("exit code = %d, want 4", exitCode)
	}

	// Audit file should not exist since the call was denied before audit logger is opened.
	// Note: the audit logger IS opened (before policy check in the command flow),
	// but no entry is written since policy denial happens before HTTP call in Do().
	// The file may exist (opened by NewLogger) but should have zero NDJSON entries.
	if data, err := os.ReadFile(auditPath); err == nil {
		lines := countNDJSONLines(data)
		if lines != 0 {
			t.Errorf("audit log should have 0 entries for policy-denied call, got %d\nContent: %s", lines, string(data))
		}
	}
	// If the file doesn't exist, that's also fine (audit log only opened if auditPath != "").
	// Wait — the --audit flag is passed, so the logger IS opened. File may exist but empty.
}

// countNDJSONLines counts the number of non-empty lines in NDJSON data.
func countNDJSONLines(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			count++
		}
	}
	return count
}
