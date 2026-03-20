package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/internal/config"
)

func TestDefaultPath(t *testing.T) {
	t.Run("contains cf directory segment", func(t *testing.T) {
		path := config.DefaultPath()
		// Should contain "cf" as a directory segment (not "jr" from jira-cli)
		if !strings.Contains(path, "cf") {
			t.Errorf("DefaultPath() = %q, should contain 'cf'", path)
		}
		if strings.Contains(path, "jr") {
			t.Errorf("DefaultPath() = %q, should not contain 'jr'", path)
		}
	})

	t.Run("CF_CONFIG_PATH env var overrides default", func(t *testing.T) {
		customPath := "/tmp/test-config.json"
		t.Setenv("CF_CONFIG_PATH", customPath)
		got := config.DefaultPath()
		if got != customPath {
			t.Errorf("DefaultPath() = %q, want %q", got, customPath)
		}
	})
}

func TestLoadFromNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom non-existent path should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFrom non-existent path should return empty Config, got nil")
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	original := &config.Config{
		DefaultProfile: "prod",
		Profiles: map[string]config.Profile{
			"prod": {
				BaseURL: "https://example.atlassian.net",
				Auth: config.AuthConfig{
					Type:     "basic",
					Username: "user@example.com",
					Token:    "my-secret-token",
				},
			},
		},
	}

	if err := config.SaveTo(original, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	if loaded.DefaultProfile != original.DefaultProfile {
		t.Errorf("DefaultProfile = %q, want %q", loaded.DefaultProfile, original.DefaultProfile)
	}

	prod, ok := loaded.Profiles["prod"]
	if !ok {
		t.Fatal("profile 'prod' not found after roundtrip")
	}
	if prod.BaseURL != "https://example.atlassian.net" {
		t.Errorf("BaseURL = %q, want %q", prod.BaseURL, "https://example.atlassian.net")
	}
	if prod.Auth.Username != "user@example.com" {
		t.Errorf("Auth.Username = %q, want %q", prod.Auth.Username, "user@example.com")
	}
	if prod.Auth.Token != "my-secret-token" {
		t.Errorf("Auth.Token = %q, want %q", prod.Auth.Token, "my-secret-token")
	}
}

func TestResolveWithEnvBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_BASE_URL", "https://env.atlassian.net")
	// Clear other env vars that might interfere
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")

	resolved, err := config.Resolve(path, "", nil)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.BaseURL != "https://env.atlassian.net" {
		t.Errorf("BaseURL = %q, want %q", resolved.BaseURL, "https://env.atlassian.net")
	}
}

func TestCFProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write a config with a "staging" profile
	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				BaseURL: "https://default.atlassian.net",
				Auth:    config.AuthConfig{Type: "basic"},
			},
			"staging": {
				BaseURL: "https://staging.atlassian.net",
				Auth:    config.AuthConfig{Type: "bearer", Token: "staging-token"},
			},
		},
	}
	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Run("CF_PROFILE selects staging profile", func(t *testing.T) {
		t.Setenv("CF_PROFILE", "staging")
		t.Setenv("CF_BASE_URL", "")
		t.Setenv("CF_AUTH_TYPE", "")
		t.Setenv("CF_AUTH_TOKEN", "")
		t.Setenv("CF_AUTH_USER", "")

		resolved, err := config.Resolve(path, "", nil)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}
		if resolved.BaseURL != "https://staging.atlassian.net" {
			t.Errorf("BaseURL = %q, want %q", resolved.BaseURL, "https://staging.atlassian.net")
		}
		if resolved.ProfileName != "staging" {
			t.Errorf("ProfileName = %q, want %q", resolved.ProfileName, "staging")
		}
	})

	t.Run("--profile flag overrides CF_PROFILE env var", func(t *testing.T) {
		t.Setenv("CF_PROFILE", "staging")
		t.Setenv("CF_BASE_URL", "")
		t.Setenv("CF_AUTH_TYPE", "")
		t.Setenv("CF_AUTH_TOKEN", "")
		t.Setenv("CF_AUTH_USER", "")

		// Explicit profileName arg overrides CF_PROFILE
		resolved, err := config.Resolve(path, "default", nil)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}
		if resolved.BaseURL != "https://default.atlassian.net" {
			t.Errorf("BaseURL = %q, want %q", resolved.BaseURL, "https://default.atlassian.net")
		}
	})
}

func TestResolveNonExistentExplicitProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")

	_, err := config.Resolve(path, "nonexistent-profile", nil)
	if err == nil {
		t.Error("Expected error for non-existent explicit profile, got nil")
	}
}

func TestResolveFlagsPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write config with a profile
	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				BaseURL: "https://file.atlassian.net",
				Auth:    config.AuthConfig{Type: "basic", Token: "file-token"},
			},
		},
	}
	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_BASE_URL", "https://env.atlassian.net")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")

	flags := &config.FlagOverrides{
		BaseURL: "https://flag.atlassian.net",
		Token:   "flag-token",
	}

	resolved, err := config.Resolve(path, "", flags)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Flags should win over env and file
	if resolved.BaseURL != "https://flag.atlassian.net" {
		t.Errorf("BaseURL = %q, want flags to win", resolved.BaseURL)
	}
	if resolved.Auth.Token != "flag-token" {
		t.Errorf("Token = %q, want flag-token", resolved.Auth.Token)
	}
}

func TestResolveTrimsTrailingSlash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_BASE_URL", "https://example.atlassian.net///")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")

	resolved, err := config.Resolve(path, "", nil)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if strings.HasSuffix(resolved.BaseURL, "/") {
		t.Errorf("BaseURL %q should not have trailing slash", resolved.BaseURL)
	}
}

func TestResolveEmptyBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")

	resolved, err := config.Resolve(path, "", nil)
	if err != nil {
		t.Fatalf("Resolve should not error on missing config, got: %v", err)
	}
	if resolved.BaseURL != "" {
		t.Errorf("BaseURL should be empty for unconfigured instance, got: %q", resolved.BaseURL)
	}
}

func TestValidAuthTypeOAuth2(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"oauth2", true},
		{"oauth2-3lo", true},
		{"OAuth2", true},
		{"basic", true},
		{"bearer", true},
		{"invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := config.ValidAuthType(tt.input); got != tt.want {
				t.Errorf("ValidAuthType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveOAuth2FromProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		DefaultProfile: "oauth",
		Profiles: map[string]config.Profile{
			"oauth": {
				BaseURL: "https://mysite.atlassian.net",
				Auth: config.AuthConfig{
					Type:         "oauth2",
					ClientID:     "file-client-id",
					ClientSecret: "file-client-secret",
					CloudID:      "file-cloud-id",
					Scopes:       "read:confluence-content.all",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	resolved, err := config.Resolve(path, "", nil)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.Auth.ClientID != "file-client-id" {
		t.Errorf("ClientID = %q, want %q", resolved.Auth.ClientID, "file-client-id")
	}
	if resolved.Auth.ClientSecret != "file-client-secret" {
		t.Errorf("ClientSecret = %q, want %q", resolved.Auth.ClientSecret, "file-client-secret")
	}
	if resolved.Auth.CloudID != "file-cloud-id" {
		t.Errorf("CloudID = %q, want %q", resolved.Auth.CloudID, "file-cloud-id")
	}
	if resolved.Auth.Scopes != "read:confluence-content.all" {
		t.Errorf("Scopes = %q, want %q", resolved.Auth.Scopes, "read:confluence-content.all")
	}
}

func TestResolveOAuth2EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		DefaultProfile: "oauth",
		Profiles: map[string]config.Profile{
			"oauth": {
				BaseURL: "https://mysite.atlassian.net",
				Auth: config.AuthConfig{
					Type:         "oauth2",
					ClientID:     "file-client-id",
					ClientSecret: "file-client-secret",
					CloudID:      "file-cloud-id",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "env-client-id")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "env-client-secret")
	t.Setenv("CF_AUTH_CLOUD_ID", "env-cloud-id")

	resolved, err := config.Resolve(path, "", nil)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.Auth.ClientID != "env-client-id" {
		t.Errorf("ClientID = %q, want env override %q", resolved.Auth.ClientID, "env-client-id")
	}
	if resolved.Auth.ClientSecret != "env-client-secret" {
		t.Errorf("ClientSecret = %q, want env override", resolved.Auth.ClientSecret)
	}
	if resolved.Auth.CloudID != "env-cloud-id" {
		t.Errorf("CloudID = %q, want env override", resolved.Auth.CloudID)
	}
}

func TestResolveOAuth2FlagOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		DefaultProfile: "oauth",
		Profiles: map[string]config.Profile{
			"oauth": {
				BaseURL: "https://mysite.atlassian.net",
				Auth: config.AuthConfig{
					Type:         "oauth2",
					ClientID:     "file-client-id",
					ClientSecret: "file-client-secret",
					CloudID:      "file-cloud-id",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "env-client-id")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	flags := &config.FlagOverrides{
		ClientID:     "flag-client-id",
		ClientSecret: "flag-client-secret",
		CloudID:      "flag-cloud-id",
	}

	resolved, err := config.Resolve(path, "", flags)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.Auth.ClientID != "flag-client-id" {
		t.Errorf("ClientID = %q, want flag override %q", resolved.Auth.ClientID, "flag-client-id")
	}
	if resolved.Auth.ClientSecret != "flag-client-secret" {
		t.Errorf("ClientSecret = %q, want flag override", resolved.Auth.ClientSecret)
	}
	if resolved.Auth.CloudID != "flag-cloud-id" {
		t.Errorf("CloudID = %q, want flag override", resolved.Auth.CloudID)
	}
}

func TestResolveOAuth2MissingClientID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		DefaultProfile: "oauth",
		Profiles: map[string]config.Profile{
			"oauth": {
				BaseURL: "https://mysite.atlassian.net",
				Auth: config.AuthConfig{
					Type:         "oauth2",
					ClientSecret: "secret",
					CloudID:      "cloud",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_AUTH_CLIENT_ID", "")
	t.Setenv("CF_AUTH_CLIENT_SECRET", "")
	t.Setenv("CF_AUTH_CLOUD_ID", "")

	_, err := config.Resolve(path, "", nil)
	if err == nil {
		t.Fatal("expected error for missing client_id, got nil")
	}
	if !strings.Contains(err.Error(), "client_id") {
		t.Errorf("error should mention client_id, got: %v", err)
	}
}

func TestTokenDir(t *testing.T) {
	t.Setenv("CF_TOKEN_DIR", "")

	dir := config.TokenDir()
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, "Library/Application Support/cf/tokens") {
			t.Errorf("TokenDir() on darwin = %q, want Library/Application Support/cf/tokens", dir)
		}
	default:
		if !strings.Contains(dir, ".config/cf/tokens") {
			t.Errorf("TokenDir() = %q, want .config/cf/tokens", dir)
		}
	}
}

func TestTokenDirEnvOverride(t *testing.T) {
	t.Setenv("CF_TOKEN_DIR", "/tmp/custom-tokens")
	dir := config.TokenDir()
	if dir != "/tmp/custom-tokens" {
		t.Errorf("TokenDir() = %q, want /tmp/custom-tokens", dir)
	}
}

func TestOAuth2RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	original := &config.Config{
		DefaultProfile: "oauth",
		Profiles: map[string]config.Profile{
			"oauth": {
				BaseURL: "https://mysite.atlassian.net",
				Auth: config.AuthConfig{
					Type:         "oauth2",
					ClientID:     "my-client-id",
					ClientSecret: "my-secret",
					Scopes:       "read:confluence-content.all",
					CloudID:      "my-cloud-id",
				},
			},
		},
	}
	if err := config.SaveTo(original, path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	p := loaded.Profiles["oauth"]
	if p.Auth.ClientID != "my-client-id" {
		t.Errorf("ClientID = %q, want %q", p.Auth.ClientID, "my-client-id")
	}
	if p.Auth.CloudID != "my-cloud-id" {
		t.Errorf("CloudID = %q, want %q", p.Auth.CloudID, "my-cloud-id")
	}
}
