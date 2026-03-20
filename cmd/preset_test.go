package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
	"github.com/sofq/confluence-cli/internal/config"
)

// setupPresetConfig writes a config file with presets and sets env vars.
func setupPresetConfig(t *testing.T, srvURL string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				BaseURL: srvURL + "/wiki/api/v2",
				Auth: config.AuthConfig{
					Type:  "bearer",
					Token: "test-token",
				},
				Presets: map[string]string{
					"brief":  ".title",
					"titles": ".results[].title",
				},
			},
		},
	}
	if err := config.SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	t.Setenv("CF_CONFIG_PATH", cfgPath)
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")

	return cfgPath
}

func TestPresetResolvesJQFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "42",
			"title": "Hello World",
		})
	}))
	defer srv.Close()
	setupPresetConfig(t, srv.URL)

	stdout, _ := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"pages", "get", "42", "--preset", "brief"})
		_ = root.Execute()
	})

	// The "brief" preset is ".title", so output should be the title string.
	stdout = strings.TrimSpace(stdout)
	if !strings.Contains(stdout, "Hello World") {
		t.Errorf("expected preset to filter output to title, got stdout: %s", stdout)
	}
}

func TestPresetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call for nonexistent preset")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setupPresetConfig(t, srv.URL)

	_, stderrOut := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"pages", "get", "42", "--preset", "nonexistent"})
		err := root.Execute()
		if err == nil {
			t.Error("expected error for nonexistent preset, got nil")
		}
	})

	if !strings.Contains(stderrOut, `preset \"nonexistent\" not found`) && !strings.Contains(stderrOut, `preset "nonexistent" not found`) {
		t.Errorf("expected 'not found' error in stderr, got: %s", stderrOut)
	}
	if !strings.Contains(stderrOut, "config_error") {
		t.Errorf("expected config_error type in stderr, got: %s", stderrOut)
	}
}

func TestPresetConflictsWithJQ(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP call for preset+jq conflict")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setupPresetConfig(t, srv.URL)

	_, stderrOut := captureOutput(t, func() {
		root := cmd.RootCommand()
		root.SetArgs([]string{"pages", "get", "42", "--preset", "brief", "--jq", ".foo"})
		err := root.Execute()
		if err == nil {
			t.Error("expected error for --preset + --jq conflict, got nil")
		}
	})

	if !strings.Contains(stderrOut, "cannot use --preset and --jq together") {
		t.Errorf("expected conflict error in stderr, got: %s", stderrOut)
	}
	if !strings.Contains(stderrOut, "validation_error") {
		t.Errorf("expected validation_error type in stderr, got: %s", stderrOut)
	}
}

func TestPresetEmptyStringDoesNotInterfere(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "42",
			"title": "Hello World",
		})
	}))
	defer srv.Close()
	setupPresetConfig(t, srv.URL)

	stdout, _ := captureOutput(t, func() {
		root := cmd.RootCommand()
		// Explicitly pass --preset "" and --jq "" to reset Cobra singleton flag state.
		root.SetArgs([]string{"pages", "get", "42", "--preset", "", "--jq", ""})
		_ = root.Execute()
	})

	stdout = strings.TrimSpace(stdout)
	// With empty --preset and no --jq, we get the full JSON object.
	if !strings.Contains(stdout, "42") {
		t.Errorf("expected command to succeed with empty --preset and return page data, got stdout: %s", stdout)
	}
}
