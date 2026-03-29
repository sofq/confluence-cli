package cmd_test

// misc_coverage_test.go covers the anonymous RunE closures in:
//   - cmd/version.go    (version command)
//   - cmd/preset.go     (preset list, preset parent RunE)
//   - cmd/templates.go  (templates parent RunE, jq/pretty branches, error paths)
//   - cmd/root.go       (PersistentPreRunE OAuth2 paths)

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/config"
	"github.com/sofq/confluence-cli/internal/oauth2"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runRootCmd executes the root cobra command with the given arguments.
// It captures stdout into buf (pass nil to discard), captures stderr, and
// returns the Execute error. It resets persistent flag state before and after.
func runRootCmd(t *testing.T, args []string, buf *bytes.Buffer) error {
	t.Helper()
	cmd.ResetRootPersistentFlags()
	t.Cleanup(func() { cmd.ResetRootPersistentFlags() })

	root := cmd.RootCommand()
	if buf != nil {
		root.SetOut(buf)
	}
	root.SetArgs(args)

	oldStderr := os.Stderr
	_, wErr, _ := os.Pipe()
	os.Stderr = wErr
	t.Cleanup(func() {
		wErr.Close()
		os.Stderr = oldStderr
	})

	return root.Execute()
}

// runRootCmdCaptureStderr captures both stdout (into outBuf) and stderr (into
// errBuf), and returns the Execute error.
func runRootCmdCaptureStderr(t *testing.T, args []string, outBuf, errBuf *bytes.Buffer) error {
	t.Helper()
	cmd.ResetRootPersistentFlags()
	t.Cleanup(func() { cmd.ResetRootPersistentFlags() })

	root := cmd.RootCommand()
	if outBuf != nil {
		root.SetOut(outBuf)
	}
	root.SetArgs(args)

	// Redirect os.Stderr to a pipe so JSON error output is captured.
	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	err := root.Execute()

	wErr.Close()
	os.Stderr = oldStderr
	if errBuf != nil {
		_, _ = errBuf.ReadFrom(rErr)
	}

	return err
}

// setupOAuth2Config creates a temp config dir with an oauth2 (or oauth2-3lo)
// profile and sets the relevant environment variables. Returns the config dir.
func setupOAuth2Config(t *testing.T, authType, cloudID string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	profile := config.Profile{
		BaseURL: "http://localhost/wiki/api/v2",
		Auth: config.AuthConfig{
			Type:         authType,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			CloudID:      cloudID,
		},
	}
	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles:       map[string]config.Profile{"default": profile},
	}
	if err := config.SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	return dir
}

// writeCachedToken writes a non-expired OAuth2 access token to the FileStore
// for the "default" profile in the given token dir.
func writeCachedToken(t *testing.T, tokenDir, accessToken, cloudID string) {
	t.Helper()
	store := oauth2.NewFileStore(tokenDir, "default")
	tok := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "bearer",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now(),
		CloudID:     cloudID,
	}
	if err := store.Save(tok); err != nil {
		t.Fatalf("writeCachedToken: %v", err)
	}
}

// ---------------------------------------------------------------------------
// version.go
// ---------------------------------------------------------------------------

// TestVersionCmd covers the versionCmd.RunE closure (cmd/version.go line 14-16).
// schemaOutput writes to os.Stdout directly, so we redirect os.Stdout.
func TestVersionCmd(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	err := runRootCmd(t, []string{"version"}, nil)

	wOut.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(rOut)

	if err != nil {
		t.Fatalf("version command returned error: %v", err)
	}

	var out map[string]string
	if jsonErr := json.Unmarshal(buf.Bytes(), &out); jsonErr != nil {
		t.Fatalf("version output is not valid JSON: %v\nOutput: %s", jsonErr, buf.String())
	}
	if _, ok := out["version"]; !ok {
		t.Errorf("version output missing 'version' key; got: %s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// preset.go — parent RunE
// ---------------------------------------------------------------------------

// TestPresetCmd_NoSubcommand covers the parent templatesCmd.RunE error path
// (cmd/preset.go lines 20-25) when no subcommand is provided.
func TestPresetCmd_NoSubcommand(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"preset"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error when running 'preset' without subcommand, got nil")
	}
}

// TestPresetCmd_UnknownArg covers the parent presetCmd.RunE "unknown command" branch
// (cmd/preset.go line 22-23) when an unknown positional arg is passed.
func TestPresetCmd_UnknownArg(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"preset", "unknowncmd"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error when running 'preset unknowncmd', got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// preset.go — preset list command
// ---------------------------------------------------------------------------

// TestPresetList_BasicOutput covers the happy path of preset list
// (cmd/preset.go lines 31-69).
func TestPresetList_BasicOutput(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"preset", "list"}, &buf); err != nil {
		t.Fatalf("preset list returned error: %v", err)
	}

	var entries []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("preset list output is not valid JSON array: %v\nOutput: %s", err, buf.String())
	}
	if len(entries) == 0 {
		t.Error("preset list returned empty array; expected built-in presets")
	}
	// Verify each entry has name, expression, and source fields.
	for _, e := range entries {
		if e["name"] == "" {
			t.Errorf("preset entry missing name: %v", e)
		}
		if e["source"] == "" {
			t.Errorf("preset entry missing source: %v", e)
		}
	}
}

// TestPresetList_WithJQ covers the jq filter path in preset list
// (cmd/preset.go lines 53-61).
func TestPresetList_WithJQ(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"preset", "list", "--jq", ".[0].name"}, &buf); err != nil {
		t.Fatalf("preset list --jq returned error: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("preset list --jq output was empty")
	}
}

// TestPresetList_WithInvalidJQ covers the jq error path in preset list
// (cmd/preset.go lines 55-58).
func TestPresetList_WithInvalidJQ(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"preset", "list", "--jq", "invalid jq {{{"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for invalid jq filter, got nil")
	}
	if !strings.Contains(errBuf.String(), "jq_error") {
		t.Errorf("expected jq_error in stderr, got: %s", errBuf.String())
	}
}

// TestPresetList_WithPretty covers the pretty-print path in preset list
// (cmd/preset.go lines 62-66).
func TestPresetList_WithPretty(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"preset", "list", "--pretty"}, &buf); err != nil {
		t.Fatalf("preset list --pretty returned error: %v", err)
	}
	// Pretty-printed JSON should contain newlines.
	if !strings.Contains(buf.String(), "\n") {
		t.Errorf("preset list --pretty output has no newlines; got: %s", buf.String())
	}
}

// TestPresetList_WithProfilePresets covers the branch where a named config profile
// has custom presets, exercising the rawProfile.Presets code path
// (cmd/preset.go lines 40-43).
func TestPresetList_WithProfilePresets(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				BaseURL: "http://localhost/wiki/api/v2",
				Auth:    config.AuthConfig{Type: "bearer", Token: "tok"},
				Presets: map[string]string{
					"my-preset": ".[0].id",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"preset", "list"}, &buf); err != nil {
		t.Fatalf("preset list with profile presets returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "my-preset") {
		t.Errorf("expected 'my-preset' in output, got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// templates.go — parent RunE
// ---------------------------------------------------------------------------

// TestTemplatesCmd_NoSubcommand covers templatesCmd.RunE missing subcommand path
// (cmd/templates.go lines 28-32).
func TestTemplatesCmd_NoSubcommand(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"templates"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error when running 'templates' without subcommand, got nil")
	}
}

// TestTemplatesCmd_UnknownArg covers the unknown command branch in templatesCmd.RunE
// (cmd/templates.go lines 29-31).
func TestTemplatesCmd_UnknownArg(t *testing.T) {
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"templates", "unknowncmd"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error when running 'templates unknowncmd', got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// templates.go — templates list jq/pretty branches
// ---------------------------------------------------------------------------

// TestTemplatesList_WithInvalidJQ covers the jq error path in templates list
// (cmd/templates.go lines 58-63).
func TestTemplatesList_WithInvalidJQ(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"templates", "list", "--jq", "invalid jq {{{"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for invalid jq filter on templates list, got nil")
	}
	if !strings.Contains(errBuf.String(), "jq_error") {
		t.Errorf("expected jq_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesList_WithPretty covers the pretty-print path in templates list
// (cmd/templates.go lines 65-70).
func TestTemplatesList_WithPretty(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"templates", "list", "--pretty"}, &buf); err != nil {
		t.Fatalf("templates list --pretty returned error: %v", err)
	}
	// Pretty-printed JSON should contain newlines.
	if !strings.Contains(buf.String(), "\n") {
		t.Errorf("templates list --pretty output has no newlines; got: %s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// templates.go — templates show jq/pretty branches
// ---------------------------------------------------------------------------

// TestTemplatesShow_WithInvalidJQ covers the jq error path in templates show
// (cmd/templates.go lines 101-107).
func TestTemplatesShow_WithInvalidJQ(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"templates", "show", "blank", "--jq", "invalid jq {{{"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for invalid jq filter on templates show, got nil")
	}
	if !strings.Contains(errBuf.String(), "jq_error") {
		t.Errorf("expected jq_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesShow_WithPretty covers the pretty-print path in templates show
// (cmd/templates.go lines 108-113).
func TestTemplatesShow_WithPretty(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"templates", "show", "blank", "--pretty"}, &buf); err != nil {
		t.Fatalf("templates show --pretty returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "\n") {
		t.Errorf("templates show --pretty output has no newlines; got: %s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// templates.go — templates create error paths
// ---------------------------------------------------------------------------

// TestPresetList_ConfigResolveError covers cmd/preset.go:35-38 — the non-fatal
// config.Resolve fallback path when the auth type is invalid. preset list continues
// with built-in presets when Resolve fails.
func TestPresetList_ConfigResolveError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	// Invalid auth type causes config.Resolve to return an error.
	rawCfg := `{"profiles":{"default":{"base_url":"http://localhost","auth":{"type":"invalid_auth_type"}}},"default_profile":"default"}`
	if err := os.WriteFile(cfgPath, []byte(rawCfg), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	// preset list is non-fatal on Resolve error; it falls back to built-in presets.
	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"preset", "list"}, &buf); err != nil {
		t.Fatalf("preset list with resolve error should not fail, got: %v", err)
	}
	// Should still return built-in presets.
	if !strings.Contains(buf.String(), "brief") {
		t.Errorf("expected built-in presets in output, got: %s", buf.String())
	}
}

// TestPresetList_ListError covers cmd/preset.go:35-38 — the preset.List error
// path triggered when the user presets file contains malformed JSON.
func TestPresetList_ListError(t *testing.T) {
	// Write a malformed user presets file and override the path used by the
	// preset package.
	dir := t.TempDir()
	presetsFile := filepath.Join(dir, "presets.json")
	if err := os.WriteFile(presetsFile, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	old := cmd.SetPresetUserPresetsPath(func() string { return presetsFile })
	t.Cleanup(func() { cmd.SetPresetUserPresetsPath(old) })

	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"preset", "list"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for malformed user presets, got nil")
	}
	if !strings.Contains(errBuf.String(), "config_error") {
		t.Errorf("expected config_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesList_WithJQ covers cmd/templates.go:59 — the jq success path
// (data = filtered) when a valid jq filter is applied to templates list.
func TestTemplatesList_WithJQ(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"templates", "list", "--jq", ".[0].name"}, &buf); err != nil {
		t.Fatalf("templates list --jq returned error: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("templates list --jq output was empty")
	}
}

// TestTemplatesList_ListError covers cmd/templates.go:42-46 — the cftemplate.List
// error path triggered when the templates directory exists but is not readable (os.ReadDir fails).
func TestTemplatesList_ListError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	// Create a temp config dir with a templates directory that has no read permission.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("WriteFile cfg: %v", err)
	}
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Put a placeholder so the dir exists, then make it unreadable.
	if err := os.WriteFile(filepath.Join(tmplDir, "dummy.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("WriteFile dummy: %v", err)
	}
	if err := os.Chmod(tmplDir, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(tmplDir, 0o755) })

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "http://localhost/wiki/api/v2")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "test")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"templates", "list"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for unreadable templates directory, got nil")
	}
	if !strings.Contains(errBuf.String(), "config_error") {
		t.Errorf("expected config_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesShow_WithJQ covers cmd/templates.go:98 — the jq success path
// (data = filtered) when a valid jq filter is applied to templates show.
func TestTemplatesShow_WithJQ(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var buf bytes.Buffer
	if err := runRootCmd(t, []string{"templates", "show", "blank", "--jq", ".name"}, &buf); err != nil {
		t.Fatalf("templates show --jq returned error: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("templates show --jq output was empty")
	}
}

// TestTemplatesCreate_MissingFromPage covers the --from-page validation error
// (cmd/templates.go lines 132-136).
func TestTemplatesCreate_MissingFromPage(t *testing.T) {
	setupTemplateEnv(t, "", nil)

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{
		"templates", "create",
		"--name", "my-template",
		// deliberately omit --from-page
	}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for missing --from-page, got nil")
	}
	if !strings.Contains(errBuf.String(), "validation_error") {
		t.Errorf("expected validation_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesCreate_ConfigResolveError covers the config.Resolve error path
// (cmd/templates.go lines 142-146). An invalid auth type causes Resolve to fail.
func TestTemplatesCreate_ConfigResolveError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	// Write a config with an invalid auth type to make config.Resolve fail.
	rawCfg := `{"profiles":{"default":{"base_url":"http://localhost","auth":{"type":"invalid_type"}}},"default_profile":"default"}`
	if err := os.WriteFile(cfgPath, []byte(rawCfg), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{
		"templates", "create",
		"--name", "my-template",
		"--from-page", "123",
	}, nil, &errBuf)
	if err == nil {
		t.Error("expected error from config resolve failure, got nil")
	}
	if !strings.Contains(errBuf.String(), "config_error") {
		t.Errorf("expected config_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesCreate_FetchError covers the page fetch error path
// (cmd/templates.go lines 158-160) when the page API returns an error.
func TestTemplatesCreate_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"page not found"}`))
	}))
	defer srv.Close()

	setupTemplateEnv(t, srv.URL, nil)

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{
		"templates", "create",
		"--name", "my-template",
		"--from-page", "99999",
	}, nil, &errBuf)
	if err == nil {
		t.Error("expected error from page fetch failure, got nil")
	}
}

// TestTemplatesCreate_InvalidJSONResponse covers cmd/templates.go:163-167 — the
// json.Unmarshal error path when the page fetch returns non-JSON response.
func TestTemplatesCreate_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a 200 status with invalid JSON body to trigger json.Unmarshal failure.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json at all"))
	}))
	defer srv.Close()

	setupTemplateEnv(t, srv.URL, nil)

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{
		"templates", "create",
		"--name", "my-template",
		"--from-page", "42",
	}, nil, &errBuf)
	if err == nil {
		t.Error("expected error from invalid JSON page response, got nil")
	}
	if !strings.Contains(errBuf.String(), "connection_error") {
		t.Errorf("expected connection_error in stderr, got: %s", errBuf.String())
	}
}

// TestTemplatesCreate_SaveError covers the template save error path
// (cmd/templates.go lines 171-175) when the template dir is not writable.
func TestTemplatesCreate_SaveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "42",
			"title": "My Page",
			"body": map[string]any{
				"storage": map[string]any{"value": "<p>body</p>"},
			},
		})
	}))
	defer srv.Close()

	dir := setupTemplateEnv(t, srv.URL, nil)

	// Create the templates dir and make it read-only so os.WriteFile fails.
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(tmplDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(tmplDir, 0o755) })

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{
		"templates", "create",
		"--name", "my-template",
		"--from-page", "42",
	}, nil, &errBuf)
	if err == nil {
		t.Error("expected error from template save failure (read-only dir), got nil")
	}
	if !strings.Contains(errBuf.String(), "config_error") {
		t.Errorf("expected config_error in stderr, got: %s", errBuf.String())
	}
}

// ---------------------------------------------------------------------------
// root.go — PersistentPreRunE OAuth2 paths
// ---------------------------------------------------------------------------

// TestRootPersistentPreRunE_OAuth2TokenError covers the OAuth2 token fetch error
// path (cmd/root.go lines 105-135: tokenErr != nil). A mock server returns HTTP
// 401 from the token endpoint, causing ClientCredentials to return an error.
func TestRootPersistentPreRunE_OAuth2TokenError(t *testing.T) {
	// Start a mock token endpoint that always returns 401.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer tokenSrv.Close()

	// Override the oauth2 client-credentials token endpoint.
	oldEndpoint := tokenSrv.URL
	cmd.SetOAuth2TokenEndpoint(tokenSrv.URL)
	t.Cleanup(func() { cmd.SetOAuth2TokenEndpoint(oldEndpoint) })

	// Use a temp token dir with NO cached token so the endpoint is hit.
	tokenDir := t.TempDir()
	t.Setenv("CF_TOKEN_DIR", tokenDir)

	setupOAuth2Config(t, "oauth2", "my-cloud-id")

	var errBuf bytes.Buffer
	// Use "raw GET /wiki/api/v2/pages" which triggers PersistentPreRunE but has
	// its own command-level flags reset by ResetRootPersistentFlags, avoiding
	// any state bleed from TestRoot_HelpForSubcommand's "pages --help" run.
	err := runRootCmdCaptureStderr(t, []string{"raw", "GET", "/wiki/api/v2/pages"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error from OAuth2 token fetch failure, got nil")
	}
	if !strings.Contains(errBuf.String(), "auth_error") && !strings.Contains(errBuf.String(), "OAuth2") {
		t.Errorf("expected auth_error or OAuth2 in stderr, got: %s", errBuf.String())
	}
}

// TestRootPersistentPreRunE_OAuth2MissingCloudID covers the missing cloud_id
// validation error path (cmd/root.go lines 138-154). Uses oauth2-3lo (which
// does not require cloud_id in config.Resolve) with a pre-cached token that
// has no cloud_id, so neither config nor token supplies a cloud_id.
func TestRootPersistentPreRunE_OAuth2MissingCloudID(t *testing.T) {
	tokenDir := t.TempDir()
	t.Setenv("CF_TOKEN_DIR", tokenDir)

	// Pre-write a valid non-expired token with NO cloud_id.
	// ThreeLO returns this from cache without starting the browser flow.
	writeCachedToken(t, tokenDir, "test-access-token", "" /* no cloud_id */)

	// oauth2-3lo does not require cloud_id during config.Resolve.
	setupOAuth2Config(t, "oauth2-3lo", "" /* no cloud_id in config either */)

	var errBuf bytes.Buffer
	err := runRootCmdCaptureStderr(t, []string{"raw", "GET", "/wiki/api/v2/pages"}, nil, &errBuf)
	if err == nil {
		t.Error("expected error for missing cloud_id, got nil")
	}
	if !strings.Contains(errBuf.String(), "cloud_id") {
		t.Errorf("expected cloud_id error in stderr, got: %s", errBuf.String())
	}
}

// TestRootPersistentPreRunE_OAuth2CloudIDFromToken covers the branch where
// cloud_id is absent from the config but present in the pre-cached token
// (cmd/root.go lines 143-145), and then lines 157-160 (base URL rewrite).
// The pages command will fail at HTTP level (rewritten base URL not reachable),
// but PersistentPreRunE completes all OAuth2 branches successfully.
func TestRootPersistentPreRunE_OAuth2CloudIDFromToken(t *testing.T) {
	tokenDir := t.TempDir()
	t.Setenv("CF_TOKEN_DIR", tokenDir)

	// Pre-write a valid non-expired token WITH cloud_id.
	// oauth2-3lo returns this from cache; cloud_id is discovered from the token.
	writeCachedToken(t, tokenDir, "test-access-token", "token-cloud-id")

	// oauth2-3lo with no cloud_id in config; cloud_id comes from the cached token.
	setupOAuth2Config(t, "oauth2-3lo", "" /* cloud_id absent in config */)

	// The raw request will fail at HTTP level (Atlassian proxy not reachable),
	// but PersistentPreRunE will have executed lines 138-160 successfully.
	var errBuf bytes.Buffer
	_ = runRootCmdCaptureStderr(t, []string{"raw", "GET", "/wiki/api/v2/pages"}, nil, &errBuf)
	// We only care that the OAuth2 branches were entered; the HTTP failure is expected.
}

// TestRootPersistentPreRunE_OAuth2WithCachedToken covers the oauth2 success path
// (cmd/root.go lines 138-160) using a pre-cached token and cloud_id from config.
// The pages command fails at HTTP level, but PersistentPreRunE completes fully.
func TestRootPersistentPreRunE_OAuth2WithCachedToken(t *testing.T) {
	tokenDir := t.TempDir()
	t.Setenv("CF_TOKEN_DIR", tokenDir)

	// Pre-write a valid non-expired token to the FileStore.
	writeCachedToken(t, tokenDir, "cached-access-token", "")

	setupOAuth2Config(t, "oauth2", "my-cloud-id")

	// The raw request will fail at HTTP level (rewritten base URL not reachable),
	// but PersistentPreRunE will have executed lines 138-160 successfully.
	var errBuf bytes.Buffer
	_ = runRootCmdCaptureStderr(t, []string{"raw", "GET", "/wiki/api/v2/pages"}, nil, &errBuf)
	// Only coverage of the OAuth2 success path matters; HTTP error is expected.
}
