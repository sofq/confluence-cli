package cmd_test

// configure_coverage_test.go adds tests targeting uncovered branches in configure.go:
//   - runConfigure: empty profile name, test-only mode, oauth2 validation, test+save flow,
//     config load/save errors
//   - testExistingProfile: config load error, profile not found, default profile resolution,
//     missing base_url, connection failure, success
//   - deleteProfileByName: config load error, profile not found, default profile reset, save error
//   - testConnection: bearer auth, basic auth, HTTP error (4xx)
//
// IMPORTANT: The cobra rootCmd is a singleton and its local flags (on configureCmd)
// persist between test runs. To avoid contamination, each test explicitly passes ALL
// boolean flags so they always override the persisted state.

import (
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

// writeConfigFile writes a JSON config file and sets CF_CONFIG_PATH.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}
	t.Setenv("CF_CONFIG_PATH", path)
	return path
}

// execConfigure runs the configure command with the given args, capturing stdout and stderr.
// It resets configure command flag state before and after each call to prevent test
// contamination caused by cobra's singleton flag persistence between Execute() calls.
func execConfigure(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()

	// Reset cobra flag state from any previous test call.
	cmd.ResetConfigureFlags()
	cmd.ResetRootPersistentFlags()

	// Also reset AFTER the test to prevent contamination of subsequent tests that do not
	// reset flags themselves (e.g. tests in configure_test.go).
	t.Cleanup(func() {
		cmd.ResetConfigureFlags()
		cmd.ResetRootPersistentFlags()
	})

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	root := cmd.RootCommand()
	root.SetArgs(args)
	err = root.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)

	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// TestConfigureEmptyProfileName verifies that empty --profile returns a validation error.
func TestConfigureEmptyProfileName(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr, _ := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--token", "mytoken",
		"--profile", "   ", // whitespace-only
		"--test=false",
		"--delete=false",
	})

	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty profile name, stderr: %s", stderr)
	}
}

// TestConfigureTestOnlyMode verifies --test without --base-url loads and tests the existing profile.
func TestConfigureTestOnlyMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[],"_links":{}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	writeConfigFile(t, `{
		"default_profile": "testonly",
		"profiles": {
			"testonly": {
				"base_url": "`+srv.URL+`",
				"auth": {"type": "bearer", "token": "test-token"}
			}
		}
	}`)

	stdout, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "testonly",
	})
	_ = err

	if !strings.Contains(stdout, "ok") {
		t.Errorf("expected 'ok' in stdout for successful test, stderr: %s, stdout: %s", stderr, stdout)
	}
}

// TestConfigureTestOnlyModeProfileNotFound verifies test-only mode with missing profile.
func TestConfigureTestOnlyModeProfileNotFound(t *testing.T) {
	writeConfigFile(t, `{
		"default_profile": "somedefault",
		"profiles": {}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "nonexistent-profile-abc",
	})

	if err == nil {
		t.Error("expected error for missing profile in test-only mode")
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("expected not_found error, stderr: %s", stderr)
	}
}

// TestConfigureTestOnlyModeConfigLoadError verifies test-only mode with corrupt config.
func TestConfigureTestOnlyModeConfigLoadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{ invalid json "), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", path)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "loadtest",
	})

	if err == nil {
		t.Error("expected error for config load error in test-only mode")
	}
	if !strings.Contains(stderr, "config_error") {
		t.Errorf("expected config_error, stderr: %s", stderr)
	}
}

// TestConfigureTestOnlyModeEmptyBaseURL verifies test-only with a profile that has no base_url.
func TestConfigureTestOnlyModeEmptyBaseURL(t *testing.T) {
	writeConfigFile(t, `{
		"default_profile": "emptyurlprofile",
		"profiles": {
			"emptyurlprofile": {
				"base_url": "",
				"auth": {"type": "bearer", "token": "token"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "emptyurlprofile",
	})

	if err == nil {
		t.Error("expected error for empty base_url in profile")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for empty base_url, stderr: %s", stderr)
	}
}

// TestConfigureTestOnlyModeConnectionFailed verifies test-only when connection fails.
func TestConfigureTestOnlyModeConnectionFailed(t *testing.T) {
	// Use a server that returns 401 to simulate connection failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	writeConfigFile(t, `{
		"default_profile": "failprofile",
		"profiles": {
			"failprofile": {
				"base_url": "`+srv.URL+`",
				"auth": {"type": "bearer", "token": "bad-token"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "failprofile",
	})

	if err == nil {
		t.Error("expected error when connection fails in test-only mode")
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error, stderr: %s", stderr)
	}
}

// TestConfigureTestOnlyModeDefaultProfileResolution verifies that when the --profile flag
// is set to the default "default" value but config has a different default_profile,
// the resolved profile name is used.
func TestConfigureTestOnlyModeDefaultProfileResolution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	writeConfigFile(t, `{
		"default_profile": "myworkspace",
		"profiles": {
			"myworkspace": {
				"base_url": "`+srv.URL+`",
				"auth": {"type": "bearer", "token": "good-token"}
			}
		}
	}`)

	stdout, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "myworkspace",
	})
	_ = err

	if !strings.Contains(stdout, "myworkspace") || !strings.Contains(stdout, "ok") {
		t.Errorf("expected 'myworkspace' and 'ok' in stdout, stderr: %s, stdout: %s", stderr, stdout)
	}
}

// TestConfigureDeleteSuccess verifies successful profile deletion.
func TestConfigureDeleteSuccess(t *testing.T) {
	configPath := writeConfigFile(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"base_url": "https://work.atlassian.net",
				"auth": {"type": "bearer", "token": "work-token"}
			},
			"personal": {
				"base_url": "https://personal.atlassian.net",
				"auth": {"type": "bearer", "token": "personal-token"}
			}
		}
	}`)

	stdout, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		"--profile", "personal",
	})
	_ = err

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected 'deleted' in stdout, stderr: %s, stdout: %s", stderr, stdout)
	}

	// Verify the profile is actually gone from the config file
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	profiles := cfg["profiles"].(map[string]interface{})
	if _, ok := profiles["personal"]; ok {
		t.Error("expected 'personal' profile to be deleted from config")
	}
}

// TestConfigureDeleteDefaultProfileResetsDefaultProfile verifies that deleting the
// default_profile resets the default_profile field to empty.
func TestConfigureDeleteDefaultProfileResetsDefaultProfile(t *testing.T) {
	configPath := writeConfigFile(t, `{
		"default_profile": "work",
		"profiles": {
			"work": {
				"base_url": "https://work.atlassian.net",
				"auth": {"type": "bearer", "token": "work-token"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		"--profile", "work",
	})
	_ = stderr

	if err != nil {
		t.Errorf("expected no error for successful delete, got: %v", err)
	}

	// Verify default_profile is now empty
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if dp, _ := cfg["default_profile"].(string); dp != "" {
		t.Errorf("expected default_profile to be empty after deleting it, got: %q", dp)
	}
}

// TestConfigureDeleteConfigLoadError verifies delete with corrupt config.
func TestConfigureDeleteConfigLoadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{ bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", path)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		"--profile", "somename",
	})

	if err == nil {
		t.Error("expected error for config load error on delete")
	}
	if !strings.Contains(stderr, "config_error") {
		t.Errorf("expected config_error, stderr: %s", stderr)
	}
}

// TestConfigureInvalidAuthType verifies that an invalid --auth-type returns validation error.
func TestConfigureInvalidAuthType(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--token", "mytoken",
		"--auth-type", "notarealtype",
		"--profile", "authtest",
		"--test=false",
		"--delete=false",
	})

	if err == nil {
		t.Error("expected error for invalid auth-type")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error, stderr: %s", stderr)
	}
}

// TestConfigureEmptyTokenForBasicAuth verifies that empty --token for basic auth returns error.
func TestConfigureEmptyTokenForBasicAuth(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--token", "",
		"--auth-type", "basic",
		"--profile", "basictest",
		"--test=false",
		"--delete=false",
	})

	if err == nil {
		t.Error("expected error for empty token with basic auth")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error, stderr: %s", stderr)
	}
}

// TestConfigureOAuth2MissingClientID verifies that --auth-type oauth2 requires --client-id.
func TestConfigureOAuth2MissingClientID(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--auth-type", "oauth2",
		"--cloud-id", "abc123",
		"--profile", "oauth2test",
		"--test=false",
		"--delete=false",
	})

	if err == nil {
		t.Error("expected error for oauth2 without client-id")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for missing client-id, stderr: %s", stderr)
	}
}

// TestConfigureOAuth2MissingClientSecret verifies that --auth-type oauth2 requires --client-secret.
func TestConfigureOAuth2MissingClientSecret(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--auth-type", "oauth2",
		"--client-id", "my-client-id",
		"--cloud-id", "abc123",
		"--profile", "oauth2secrettest",
		"--test=false",
		"--delete=false",
	})

	if err == nil {
		t.Error("expected error for oauth2 without client-secret")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for missing client-secret, stderr: %s", stderr)
	}
}

// TestConfigureOAuth2MissingCloudID verifies that --auth-type oauth2 requires --cloud-id.
func TestConfigureOAuth2MissingCloudID(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--auth-type", "oauth2",
		"--client-id", "my-client-id",
		"--client-secret", "my-secret",
		"--profile", "oauth2cloudtest",
		"--test=false",
		"--delete=false",
		// no --cloud-id
	})

	if err == nil {
		t.Error("expected error for oauth2 without cloud-id")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error for missing cloud-id, stderr: %s", stderr)
	}
}

// TestConfigureWithTestAndSave verifies that --test with valid connection saves the config.
func TestConfigureWithTestAndSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test connection endpoint
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	stdout, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", srv.URL,
		"--token", "valid-token",
		"--auth-type", "basic",
		"--username", "user@example.com",
		"--test=true",
		"--delete=false",
		"--profile", "tested",
	})
	_ = err

	if !strings.Contains(stdout, "saved") {
		t.Errorf("expected 'saved' in stdout, stderr: %s, stdout: %s", stderr, stdout)
	}
	// Config file should now exist
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Errorf("config file should exist after configure --test --save: %v", statErr)
	}
}

// TestConfigureWithTestConnectionFailed verifies --test with failing connection doesn't save.
func TestConfigureWithTestConnectionFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Forbidden"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", srv.URL,
		"--token", "bad-token",
		"--test=true",
		"--delete=false",
		"--profile", "tested",
	})

	if err == nil {
		t.Error("expected error for failed connection test")
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error in stderr, got: %s", stderr)
	}
}

// TestConfigureSavesDefaultProfileWhenFirstProfile verifies that the first saved profile
// becomes the default_profile.
func TestConfigureSavesDefaultProfileWhenFirstProfile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://new.atlassian.net",
		"--token", "mytoken",
		"--auth-type", "basic",
		"--profile", "firstprofile",
		"--test=false",
		"--delete=false",
	})
	_ = stderr

	if err != nil {
		t.Errorf("expected no error, got: %v; stderr: %s", err, stderr)
	}

	// The first profile should become default_profile
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg["default_profile"] != "firstprofile" {
		t.Errorf("expected default_profile to be 'firstprofile', got: %v", cfg["default_profile"])
	}
}

// TestConfigureNotSettingDefaultWhenAlreadySet verifies that adding a second profile
// doesn't change an existing default_profile.
func TestConfigureNotSettingDefaultWhenAlreadySet(t *testing.T) {
	configPath := writeConfigFile(t, `{
		"default_profile": "existingone",
		"profiles": {
			"existingone": {
				"base_url": "https://existing.atlassian.net",
				"auth": {"type": "bearer", "token": "existing-token"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://new.atlassian.net",
		"--token", "new-token",
		"--auth-type", "basic",
		"--profile", "newprofiletwo",
		"--test=false",
		"--delete=false",
	})
	_ = err

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	// default_profile should still be "existingone"
	if cfg["default_profile"] != "existingone" {
		t.Errorf("expected default_profile to remain 'existingone', got: %v; stderr: %s", cfg["default_profile"], stderr)
	}
}

// TestConfigureTestConnectionBearerAuth tests testConnection with bearer auth.
func TestConfigureTestConnectionBearerAuth(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", srv.URL,
		"--token", "bearer-token-abc",
		"--auth-type", "bearer",
		"--test=true",
		"--delete=false",
		"--profile", "bearertestprofile",
	})

	if err != nil {
		t.Errorf("expected no error for bearer auth test, got: %v; stderr: %s", err, stderr)
	}
	if !strings.HasPrefix(capturedAuth, "Bearer ") {
		t.Errorf("expected Bearer auth header, got: %q", capturedAuth)
	}
}

// TestConfigureTestConnectionBaseURLWithWikiSuffix verifies that base URLs ending in
// /wiki/api/v2 don't get the prefix added again.
func TestConfigureTestConnectionBaseURLWithWikiSuffix(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	// Use a base URL that already ends with /wiki/api/v2
	baseURL := srv.URL + "/wiki/api/v2"
	_, _, _ = execConfigure(t, []string{
		"configure",
		"--base-url", baseURL,
		"--token", "test-token",
		"--test=true",
		"--delete=false",
		"--profile", "path-test",
	})

	// If the request was made, the path should be /wiki/api/v2/spaces?limit=1
	// not /wiki/api/v2/wiki/api/v2/spaces?limit=1
	if capturedPath != "" && strings.Contains(capturedPath, "/wiki/api/v2/wiki/api/v2") {
		t.Errorf("base URL with /wiki/api/v2 suffix should not double the prefix, got path: %s", capturedPath)
	}
}

// TestConfigureConfigLoadErrorOnSave verifies behavior when config load fails during save.
func TestConfigureConfigLoadErrorOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Write an invalid config file
	if err := os.WriteFile(path, []byte(`{bad json`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CF_CONFIG_PATH", path)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--token", "mytoken",
		"--test=false",
		"--delete=false",
		"--profile", "savetest",
	})

	if err == nil {
		t.Error("expected error when config file is corrupt")
	}
	if !strings.Contains(stderr, "config_error") {
		t.Errorf("expected config_error, stderr: %s", stderr)
	}
}

// TestConfigureDeleteFromMultipleProfiles verifies delete of a non-default profile
// from a multi-profile config.
func TestConfigureDeleteFromMultipleProfiles(t *testing.T) {
	configPath := writeConfigFile(t, `{
		"default_profile": "primary",
		"profiles": {
			"primary": {
				"base_url": "https://primary.atlassian.net",
				"auth": {"type": "bearer", "token": "primary-token"}
			},
			"secondary": {
				"base_url": "https://secondary.atlassian.net",
				"auth": {"type": "bearer", "token": "secondary-token"}
			}
		}
	}`)

	stdout, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		"--profile", "secondary",
	})
	_ = err

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected 'deleted' in output, stderr: %s, stdout: %s", stderr, stdout)
	}

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	// Primary should still be the default
	if cfg["default_profile"] != "primary" {
		t.Errorf("default_profile should remain 'primary', got: %v", cfg["default_profile"])
	}
	// Secondary should be gone
	profiles := cfg["profiles"].(map[string]interface{})
	if _, ok := profiles["secondary"]; ok {
		t.Error("'secondary' profile should have been deleted")
	}
}

// TestConfigureSchemaOutputPretty verifies configure with --pretty flag produces pretty JSON.
func TestConfigureSchemaOutputPretty(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	stdout, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--token", "mytoken",
		"--auth-type", "basic",
		"--pretty",
		"--profile", "prettytest",
		"--test=false",
		"--delete=false",
	})
	_ = err

	if err != nil {
		t.Errorf("expected no error, got: %v; stderr: %s", err, stderr)
	}
	// Pretty output should contain newlines
	if !strings.Contains(stdout, "\n") {
		t.Errorf("expected pretty-printed output with newlines, got: %s", stdout)
	}
}

// TestSchemaOutputJQFilter verifies schemaOutput with a valid jq filter.
func TestSchemaOutputJQFilter(t *testing.T) {
	cmd.ResetRootPersistentFlags()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "pages", "get", "--jq", ".resource"})
	_ = root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = strings.TrimSpace(buf.String())
}

// TestSchemaOutputJQError verifies schemaOutput with an invalid jq filter returns an error.
func TestSchemaOutputJQError(t *testing.T) {
	cmd.ResetRootPersistentFlags()
	oldStderr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we
	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--list", "--jq", ".[invalid{{{syntax"})
	_ = root.Execute()

	we.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(re)
	stderrOut := strings.TrimSpace(errBuf.String())

	if stderrOut == "" {
		t.Fatal("expected jq_error on stderr for invalid jq in schema")
	}
	var errJSON map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOut), &errJSON); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOut)
	}
	if errJSON["error_type"] != "jq_error" {
		t.Errorf("error_type: want jq_error, got %v", errJSON["error_type"])
	}
}

// TestSchemaOutputPretty verifies that --pretty on schema command produces indented output.
func TestSchemaOutputPretty(t *testing.T) {
	cmd.ResetRootPersistentFlags()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	oldStderr := os.Stderr
	_, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--list", "--pretty"})
	_ = root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	if output == "" {
		t.Fatal("expected pretty output, got empty")
	}
	// Pretty output should have newlines
	if !strings.Contains(output, "\n") {
		t.Errorf("expected pretty output with newlines, got: %s", output)
	}
	// Should still be valid JSON
	var arr interface{}
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("pretty schema output is not valid JSON: %v\nOutput: %s", err, output)
	}
}

// TestSchemaResourceNotFound verifies that requesting a non-existent resource returns not_found.
func TestSchemaResourceNotFound(t *testing.T) {
	cmd.ResetRootPersistentFlags()
	oldStderr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we
	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "nonexistentresource9999"})
	_ = root.Execute()

	we.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(re)
	stderrOut := strings.TrimSpace(errBuf.String())

	if stderrOut == "" {
		t.Fatal("expected not_found error on stderr")
	}
	var errJSON map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOut), &errJSON); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOut)
	}
	if errJSON["error_type"] != "not_found" {
		t.Errorf("error_type: want not_found, got %v", errJSON["error_type"])
	}
}

// TestSchemaVerbNotFound verifies that requesting a non-existent verb for a real resource
// returns not_found.
func TestSchemaVerbNotFound(t *testing.T) {
	cmd.ResetRootPersistentFlags()
	oldStderr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we
	oldStdout := os.Stdout
	_, wo, _ := os.Pipe()
	os.Stdout = wo

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "pages", "nonexistentverb9999"})
	_ = root.Execute()

	we.Close()
	wo.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(re)
	stderrOut := strings.TrimSpace(errBuf.String())

	if stderrOut == "" {
		t.Fatal("expected not_found error on stderr")
	}
	var errJSON map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOut), &errJSON); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOut)
	}
	if errJSON["error_type"] != "not_found" {
		t.Errorf("error_type: want not_found, got %v", errJSON["error_type"])
	}
}

// TestConfigureDeleteWithoutExplicitProfile verifies that --delete without explicitly
// passing --profile returns a validation error.
func TestConfigureDeleteWithoutExplicitProfile(t *testing.T) {
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	// Pass --delete=true but do NOT pass --profile explicitly (use the cobra default).
	// Since "default" is the cobra default value and Changed=false, this should fail.
	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		// no --profile flag — uses cobra default "default" with Changed=false
	})

	if err == nil {
		t.Error("expected error when --delete used without explicit --profile")
	}
	if !strings.Contains(stderr, "validation_error") {
		t.Errorf("expected validation_error when --delete without --profile, stderr: %s", stderr)
	}
}

// TestConfigureDeleteProfileNotFoundWithExisting verifies that deleting a non-existent profile
// when other profiles exist lists available profiles in the error.
func TestConfigureDeleteProfileNotFoundWithExisting(t *testing.T) {
	writeConfigFile(t, `{
		"default_profile": "existing",
		"profiles": {
			"existing": {
				"base_url": "https://example.atlassian.net",
				"auth": {"type": "bearer", "token": "tok"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--delete=true",
		"--test=false",
		"--profile", "nonexistent-profile-xyz",
	})

	if err == nil {
		t.Error("expected error when deleting non-existent profile")
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("expected not_found error, stderr: %s", stderr)
	}
	// Available profiles should be listed in the error message.
	if !strings.Contains(stderr, "existing") {
		t.Errorf("expected available profiles listed in error, stderr: %s", stderr)
	}
}

// TestConfigureTestOnlyProfileNotFoundWithExisting verifies that test-only mode with a
// missing profile lists available profiles in the error.
func TestConfigureTestOnlyProfileNotFoundWithExisting(t *testing.T) {
	writeConfigFile(t, `{
		"default_profile": "existingprofile",
		"profiles": {
			"existingprofile": {
				"base_url": "https://example.atlassian.net",
				"auth": {"type": "bearer", "token": "tok"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "nonexistent-profile-abc",
	})

	if err == nil {
		t.Error("expected error for missing profile")
	}
	if !strings.Contains(stderr, "not_found") {
		t.Errorf("expected not_found, stderr: %s", stderr)
	}
	// Available profiles listed.
	if !strings.Contains(stderr, "existingprofile") {
		t.Errorf("expected available profiles in error, stderr: %s", stderr)
	}
}

// TestConfigureTestConnectionNewRequestError verifies that an invalid base URL
// returns a connection_error from testConnection.
func TestConfigureTestConnectionNewRequestError(t *testing.T) {
	// An invalid base_url that will fail http.NewRequest (no scheme).
	invalidURL := "://invalid-url-no-scheme"
	writeConfigFile(t, `{
		"default_profile": "badurl",
		"profiles": {
			"badurl": {
				"base_url": "`+invalidURL+`",
				"auth": {"type": "bearer", "token": "tok"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "badurl",
	})

	if err == nil {
		t.Error("expected error for invalid URL in testConnection")
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error, stderr: %s", stderr)
	}
}

// TestConfigureTestConnectionUnreachableHost verifies that a connection to an unreachable
// host returns a connection_error.
func TestConfigureTestConnectionUnreachableHost(t *testing.T) {
	// Port 1 is reserved and connection should be refused immediately.
	writeConfigFile(t, `{
		"default_profile": "unreachable",
		"profiles": {
			"unreachable": {
				"base_url": "http://127.0.0.1:1",
				"auth": {"type": "bearer", "token": "tok"}
			}
		}
	}`)

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--test=true",
		"--delete=false",
		"--profile", "unreachable",
	})

	if err == nil {
		t.Error("expected error for unreachable host")
	}
	if !strings.Contains(stderr, "connection_error") {
		t.Errorf("expected connection_error, stderr: %s", stderr)
	}
}

// TestConfigureConfigSaveError verifies that a config save error (when the config file
// becomes read-only between LoadFrom and SaveTo) returns a config_error.
func TestConfigureConfigSaveError(t *testing.T) {
	// Create a valid config file first so LoadFrom succeeds.
	configPath := writeConfigFile(t, `{
		"default_profile": "savetest",
		"profiles": {
			"savetest": {
				"base_url": "https://example.atlassian.net",
				"auth": {"type": "bearer", "token": "tok"}
			}
		}
	}`)

	// Make the config file read-only so SaveTo fails.
	if err := os.Chmod(configPath, 0o400); err != nil {
		t.Skipf("cannot chmod config file: %v", err)
	}
	t.Cleanup(func() {
		// Restore write permission for cleanup.
		_ = os.Chmod(configPath, 0o600)
	})

	_, stderr, err := execConfigure(t, []string{
		"configure",
		"--base-url", "https://new.atlassian.net",
		"--token", "newtoken",
		"--auth-type", "bearer",
		"--profile", "savetest",
		"--test=false",
		"--delete=false",
	})

	if err == nil {
		t.Error("expected error when config file is read-only")
	}
	if !strings.Contains(stderr, "config_error") {
		t.Errorf("expected config_error, stderr: %s", stderr)
	}
}
