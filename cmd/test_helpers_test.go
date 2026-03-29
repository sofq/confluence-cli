package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sofq/confluence-cli/internal/config"
)

// setupTemplateEnv creates a temp config dir and sets CF_CONFIG_PATH.
// The srvURL parameter, if non-empty, is used as the base URL (with /wiki/api/v2 appended).
// The templates parameter is ignored (retained for call-site compatibility) and will be
// removed in a future cleanup.
func setupTemplateEnv(t *testing.T, srvURL string, templates map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	baseURL := ""
	if srvURL != "" {
		baseURL = srvURL + "/wiki/api/v2"
	}

	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				BaseURL: baseURL,
				Auth: config.AuthConfig{
					Type:  "bearer",
					Token: "test-token",
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

	if len(templates) > 0 {
		tmplDir := filepath.Join(dir, "templates")
		if err := os.MkdirAll(tmplDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range templates {
			p := filepath.Join(tmplDir, name+".json")
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}
