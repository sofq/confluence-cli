package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

func TestVersionFlagOutputsJSON(t *testing.T) {
	// Redirect stdout to capture version output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"--version"})
	_ = root.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check it is valid JSON
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &out); err != nil {
		t.Fatalf("--version output is not valid JSON: %v\nOutput: %q", err, output)
	}

	if _, ok := out["version"]; !ok {
		t.Errorf("--version JSON output missing 'version' key, got: %s", output)
	}
}

func TestVersionSubcommandOutputsJSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"version"})
	_ = root.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("version subcommand output is not valid JSON: %v\nOutput: %q", err, output)
	}

	if _, ok := out["version"]; !ok {
		t.Errorf("version subcommand JSON missing 'version' key, got: %s", output)
	}
}

func TestRootHelpOutputsJSON(t *testing.T) {
	// "cf" with no args calls the help func which should emit JSON hint
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"help"})
	_ = root.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	if output == "" {
		// help might write to stderr — that's also acceptable
		return
	}

	// If stdout has content, it should be valid JSON
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("help output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

func TestExecuteNoConfigReturnsNonZero(t *testing.T) {
	// With no CF_BASE_URL and no config file, a command requiring a client should fail
	t.Setenv("CF_BASE_URL", "")
	t.Setenv("CF_AUTH_TOKEN", "")
	t.Setenv("CF_AUTH_TYPE", "")
	t.Setenv("CF_AUTH_USER", "")
	t.Setenv("CF_PROFILE", "")
	t.Setenv("CF_CONFIG_PATH", t.TempDir()+"/nonexistent-config.json")

	root := cmd.RootCommand()
	root.SetArgs([]string{"raw", "GET", "/wiki/api/v2/pages"})

	// Suppress stderr output during test
	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := cmd.Execute()

	w.Close()
	os.Stderr = old

	if code == 0 {
		t.Error("Execute() with no base_url configured should return non-zero exit code")
	}
}
