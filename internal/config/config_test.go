package config_test

import (
	"path/filepath"
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
