package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/config"
)

func TestConfigureSavesProfile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	root := cmd.RootCommand()
	root.SetArgs([]string{
		"configure",
		"--base-url", "https://example.atlassian.net",
		"--token", "my-api-token",
		"--auth-type", "bearer",
		"--profile", "test-profile",
	})

	// Suppress stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := root.Execute()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("configure returned error: %v", err)
	}

	// Verify the config was saved
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	profile, ok := cfg.Profiles["test-profile"]
	if !ok {
		t.Fatal("profile 'test-profile' not found in saved config")
	}

	if profile.BaseURL != "https://example.atlassian.net" {
		t.Errorf("BaseURL = %q, want %q", profile.BaseURL, "https://example.atlassian.net")
	}
	if profile.Auth.Token != "my-api-token" {
		t.Errorf("Token = %q, want %q", profile.Auth.Token, "my-api-token")
	}
}

func TestConfigureStripTrailingSlash(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	root := cmd.RootCommand()
	root.SetArgs([]string{
		"configure",
		"--base-url", "https://example.atlassian.net///",
		"--token", "token123",
		"--profile", "default",
	})

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	_ = root.Execute()
	w.Close()
	os.Stdout = oldStdout

	cfg, _ := config.LoadFrom(configPath)
	profile, ok := cfg.Profiles["default"]
	if !ok {
		t.Fatal("default profile not found")
	}
	if strings.HasSuffix(profile.BaseURL, "/") {
		t.Errorf("BaseURL %q should not have trailing slash", profile.BaseURL)
	}
}

func TestConfigureEmptyBaseURLReturnsValidationError(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	// Suppress stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	root := cmd.RootCommand()
	root.SetArgs([]string{
		"configure",
		"--base-url", "",
		"--token", "token",
	})
	err := root.Execute()

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := stderrBuf.String()

	// Should have error (AlreadyWrittenError)
	if err == nil {
		t.Error("configure with empty --base-url should return an error")
	}

	// stderr should contain JSON error
	if stderrOutput != "" {
		var errOut map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(stderrOutput)), &errOut); jsonErr == nil {
			if errType, ok := errOut["error_type"]; ok && errType != "validation_error" {
				t.Errorf("error_type = %v, want validation_error", errType)
			}
		}
	}
}

func TestConfigureDeleteWithoutProfileReturnsValidationError(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	// Suppress stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	root := cmd.RootCommand()
	root.SetArgs([]string{
		"configure",
		"--delete",
	})
	err := root.Execute()

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)

	if err == nil {
		t.Error("configure --delete without --profile should return error")
	}
}

func TestConfigureDeleteNonExistentProfileReturnsNotFound(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CF_CONFIG_PATH", configPath)

	// Suppress stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	root := cmd.RootCommand()
	root.SetArgs([]string{
		"configure",
		"--delete",
		"--profile", "nonexistent-profile",
	})
	_ = root.Execute()

	w.Close()
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	if stderrOutput == "" {
		t.Error("Expected error output for non-existent profile delete")
		return
	}

	var errOut map[string]interface{}
	if err := json.Unmarshal([]byte(stderrOutput), &errOut); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nOutput: %s", err, stderrOutput)
	}

	if errOut["error_type"] != "not_found" {
		t.Errorf("error_type = %v, want not_found", errOut["error_type"])
	}
}
