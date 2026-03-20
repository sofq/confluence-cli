package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/cmd"
)

func TestSchemaListReturnsJSONArray(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--list"})
	err := root.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("schema --list returned error: %v", err)
	}

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if output == "" {
		t.Fatal("schema --list produced no output")
	}

	// Must be a JSON array (may be empty in Phase 1 stub)
	var arr []interface{}
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("schema --list output is not a valid JSON array: %v\nOutput: %q", err, output)
	}
}

func TestSchemaNoArgsReturnsValidJSON(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema"})
	err := root.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("schema returned error: %v", err)
	}

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if output == "" {
		t.Fatal("schema produced no output")
	}

	// Must be valid JSON
	var out interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("schema output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

func TestSchemaOutputToStdout(t *testing.T) {
	// Ensure schema output goes to stdout, not stderr
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldStderr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--list"})
	root.Execute()

	w.Close()
	we.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)

	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(re)

	if stdoutBuf.Len() == 0 {
		t.Error("schema --list should write to stdout")
	}

	// stderr should be empty for successful schema command
	stderrOutput := strings.TrimSpace(stderrBuf.String())
	if stderrOutput != "" {
		t.Errorf("schema --list should not write to stderr, got: %s", stderrOutput)
	}
}

func TestSchemaCompactReturnsJSONObject(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--compact"})
	err := root.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("schema --compact returned error: %v", err)
	}

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	if output == "" {
		t.Fatal("schema --compact produced no output")
	}

	// Must be valid JSON (object mapping resource → verbs)
	var out interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("schema --compact output is not valid JSON: %v\nOutput: %q", err, output)
	}
}
