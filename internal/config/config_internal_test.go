package config

// Internal tests that need direct access to the unexported goos variable
// in order to exercise OS-specific branches of DefaultPath and TokenDir.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setGOOS sets the package-level goos variable to the given value and
// returns a cleanup function that restores the original.
func setGOOS(t *testing.T, value string) {
	t.Helper()
	original := goos
	goos = value
	t.Cleanup(func() { goos = original })
}

// TestDefaultPathWindowsWithAPPDATA exercises the windows branch of DefaultPath
// when the APPDATA environment variable is set.
func TestDefaultPathWindowsWithAPPDATA(t *testing.T) {
	setGOOS(t, "windows")
	t.Setenv("CF_CONFIG_PATH", "")
	t.Setenv("APPDATA", `C:\Users\test\AppData\Roaming`)

	got := DefaultPath()
	want := filepath.Join(`C:\Users\test\AppData\Roaming`, "cf", "config.json")
	if got != want {
		t.Errorf("DefaultPath() on windows with APPDATA = %q, want %q", got, want)
	}
}

// TestDefaultPathWindowsWithoutAPPDATA exercises the windows fallback branch of
// DefaultPath when APPDATA is not set.
func TestDefaultPathWindowsWithoutAPPDATA(t *testing.T) {
	setGOOS(t, "windows")
	t.Setenv("CF_CONFIG_PATH", "")
	t.Setenv("APPDATA", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := DefaultPath()
	want := filepath.Join(home, "AppData", "Roaming", "cf", "config.json")
	if got != want {
		t.Errorf("DefaultPath() on windows without APPDATA = %q, want %q", got, want)
	}
}

// TestDefaultPathLinux exercises the linux/default branch of DefaultPath.
func TestDefaultPathLinux(t *testing.T) {
	setGOOS(t, "linux")
	t.Setenv("CF_CONFIG_PATH", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := DefaultPath()
	want := filepath.Join(home, ".config", "cf", "config.json")
	if got != want {
		t.Errorf("DefaultPath() on linux = %q, want %q", got, want)
	}
}

// TestTokenDirWindowsWithAPPDATA exercises the windows branch of TokenDir
// when the APPDATA environment variable is set.
func TestTokenDirWindowsWithAPPDATA(t *testing.T) {
	setGOOS(t, "windows")
	t.Setenv("CF_TOKEN_DIR", "")
	t.Setenv("APPDATA", `C:\Users\test\AppData\Roaming`)

	got := TokenDir()
	want := filepath.Join(`C:\Users\test\AppData\Roaming`, "cf", "tokens")
	if got != want {
		t.Errorf("TokenDir() on windows with APPDATA = %q, want %q", got, want)
	}
}

// TestTokenDirWindowsWithoutAPPDATA exercises the windows fallback branch of
// TokenDir when APPDATA is not set.
func TestTokenDirWindowsWithoutAPPDATA(t *testing.T) {
	setGOOS(t, "windows")
	t.Setenv("CF_TOKEN_DIR", "")
	t.Setenv("APPDATA", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := TokenDir()
	want := filepath.Join(home, "AppData", "Roaming", "cf", "tokens")
	if got != want {
		t.Errorf("TokenDir() on windows without APPDATA = %q, want %q", got, want)
	}
}

// TestTokenDirLinux exercises the linux/default branch of TokenDir.
func TestTokenDirLinux(t *testing.T) {
	setGOOS(t, "linux")
	t.Setenv("CF_TOKEN_DIR", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := TokenDir()
	want := filepath.Join(home, ".config", "cf", "tokens")
	if got != want {
		t.Errorf("TokenDir() on linux = %q, want %q", got, want)
	}
}

// TestAvailableProfilesEmpty exercises the "(none)" branch of availableProfiles.
func TestAvailableProfilesEmpty(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	got := availableProfiles(cfg)
	if got != "(none)" {
		t.Errorf("availableProfiles(empty) = %q, want %q", got, "(none)")
	}
}

// TestAvailableProfilesMultiple exercises the join branch of availableProfiles.
func TestAvailableProfilesMultiple(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"prod":    {BaseURL: "https://prod.atlassian.net"},
			"staging": {BaseURL: "https://staging.atlassian.net"},
			"dev":     {BaseURL: "https://dev.atlassian.net"},
		},
	}
	got := availableProfiles(cfg)
	// Should be sorted and comma-separated.
	want := "dev, prod, staging"
	if got != want {
		t.Errorf("availableProfiles(3 profiles) = %q, want %q", got, want)
	}
}

// TestLoadFromInvalidJSON exercises the json.Unmarshal error branch of LoadFrom.
func TestLoadFromInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom with invalid JSON should return error, got nil")
	}
}

// TestLoadFromReadError exercises the non-ErrNotExist error branch of LoadFrom.
func TestLoadFromReadError(t *testing.T) {
	dir := t.TempDir()
	// Create a directory where a file is expected — ReadFile on a directory returns an error.
	path := filepath.Join(dir, "isdir")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom on a directory should return error, got nil")
	}
}

// TestLoadFromNilProfilesPopulated exercises the branch where a valid config
// file has a null or missing profiles field, which must be initialised to an
// empty (non-nil) map.
func TestLoadFromNilProfilesPopulated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Write JSON with an explicit null for profiles.
	if err := os.WriteFile(path, []byte(`{"default_profile":"x","profiles":null}`), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg.Profiles == nil {
		t.Error("Profiles should be non-nil after LoadFrom with null profiles in JSON")
	}
}

// TestSaveToMkdirAllError exercises the error path of SaveTo when the parent
// directory cannot be created (path is under a file, not a directory).
func TestSaveToMkdirAllError(t *testing.T) {
	dir := t.TempDir()
	// Create a plain file at "blocker" so that MkdirAll("blocker/subdir") fails.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	path := filepath.Join(blocker, "subdir", "config.json")
	cfg := &Config{Profiles: map[string]Profile{}}
	err := SaveTo(cfg, path)
	if err == nil {
		t.Fatal("SaveTo should return error when MkdirAll fails, got nil")
	}
}

// TestResolveInvalidAuthType exercises the invalid auth type error branch of Resolve.
func TestResolveInvalidAuthType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "invalid-auth")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	_, err := Resolve(path, "", nil)
	if err == nil {
		t.Fatal("Resolve with invalid auth type should return error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid auth type") {
		t.Errorf("error should mention 'invalid auth type', got: %v", err)
	}
}

// TestResolveAuthTypeFromFlag exercises the flags.AuthType and flags.Username
// branches of Resolve.
func TestResolveAuthTypeFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		DefaultProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				BaseURL: "https://example.atlassian.net",
				Auth:    AuthConfig{Type: "basic", Username: "file-user", Token: "file-token"},
			},
		},
	}
	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	flags := &FlagOverrides{
		AuthType: "bearer",
		Username: "flag-user",
	}
	resolved, err := Resolve(path, "", flags)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.Auth.Type != "bearer" {
		t.Errorf("Auth.Type = %q, want %q", resolved.Auth.Type, "bearer")
	}
	if resolved.Auth.Username != "flag-user" {
		t.Errorf("Auth.Username = %q, want %q", resolved.Auth.Username, "flag-user")
	}
}

// TestResolveEnvAuthTypeAndUser exercises the envAuthType and envUsername
// override branches of Resolve.
func TestResolveEnvAuthTypeAndUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		DefaultProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				BaseURL: "https://example.atlassian.net",
				Auth:    AuthConfig{Type: "basic", Username: "file-user", Token: "file-token"},
			},
		},
	}
	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "bearer")
	t.Setenv("CF_AUTH_TOKEN", "env-token")
	t.Setenv("CF_AUTH_USER", "env-user")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	resolved, err := Resolve(path, "", nil)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.Auth.Type != "bearer" {
		t.Errorf("Auth.Type = %q, want %q", resolved.Auth.Type, "bearer")
	}
	if resolved.Auth.Username != "env-user" {
		t.Errorf("Auth.Username = %q, want %q", resolved.Auth.Username, "env-user")
	}
	if resolved.Auth.Token != "env-token" {
		t.Errorf("Auth.Token = %q, want %q", resolved.Auth.Token, "env-token")
	}
}

// TestResolveOAuth23loMissingClientSecret exercises the missing client_secret
// error branch for oauth2-3lo in Resolve.
func TestResolveOAuth23loMissingClientSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "oauth2-3lo")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "some-client-id")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	_, err := Resolve(path, "", nil)
	if err == nil {
		t.Fatal("Resolve with oauth2-3lo missing client_secret should return error, got nil")
	}
	if !strings.Contains(err.Error(), "client_secret") {
		t.Errorf("error should mention 'client_secret', got: %v", err)
	}
}

// TestResolveOAuth2MissingCloudID exercises the missing cloud_id error branch
// for oauth2 (not oauth2-3lo) in Resolve.
func TestResolveOAuth2MissingCloudID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "oauth2")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "some-client-id")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "some-client-secret")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	_, err := Resolve(path, "", nil)
	if err == nil {
		t.Fatal("Resolve with oauth2 missing cloud_id should return error, got nil")
	}
	if !strings.Contains(err.Error(), "cloud_id") {
		t.Errorf("error should mention 'cloud_id', got: %v", err)
	}
}

// TestResolveLoadFromError exercises the LoadFrom error propagation in Resolve.
func TestResolveLoadFromError(t *testing.T) {
	dir := t.TempDir()
	// Provide a directory path instead of a file path so that ReadFile fails
	// with a non-ErrNotExist error.
	badPath := filepath.Join(dir, "isdir")
	if err := os.Mkdir(badPath, 0o700); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")

	_, err := Resolve(badPath, "", nil)
	if err == nil {
		t.Fatal("Resolve should propagate LoadFrom error, got nil")
	}
}

// TestDefaultPathDarwin exercises the darwin branch of DefaultPath.
func TestDefaultPathDarwin(t *testing.T) {
	setGOOS(t, "darwin")
	t.Setenv("CF_CONFIG_PATH", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := DefaultPath()
	want := filepath.Join(home, "Library", "Application Support", "cf", "config.json")
	if got != want {
		t.Errorf("DefaultPath() on darwin = %q, want %q", got, want)
	}
}

// TestTokenDirDarwin exercises the darwin branch of TokenDir.
func TestTokenDirDarwin(t *testing.T) {
	setGOOS(t, "darwin")
	t.Setenv("CF_TOKEN_DIR", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := TokenDir()
	want := filepath.Join(home, "Library", "Application Support", "cf", "tokens")
	if got != want {
		t.Errorf("TokenDir() on darwin = %q, want %q", got, want)
	}
}
