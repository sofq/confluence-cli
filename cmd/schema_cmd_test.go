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

	// Must be valid JSON (object mapping resource -> verbs)
	var out interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("schema --compact output is not valid JSON: %v\nOutput: %q", err, output)
	}
}

func TestSchemaIncludesHandWrittenOps(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--list", "--compact=false"})
	err := root.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("schema --list returned error: %v", err)
	}

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	var resources []string
	if err := json.Unmarshal([]byte(output), &resources); err != nil {
		t.Fatalf("schema --list output is not a valid JSON string array: %v", err)
	}

	expected := []string{"diff", "workflow", "export", "preset", "templates"}
	resourceSet := make(map[string]bool, len(resources))
	for _, r := range resources {
		resourceSet[r] = true
	}
	for _, want := range expected {
		if !resourceSet[want] {
			t.Errorf("schema --list missing hand-written resource %q; got: %v", want, resources)
		}
	}
}

func TestSchemaWorkflowListsSixVerbs(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := cmd.RootCommand()
	root.SetArgs([]string{"schema", "--list=false", "--compact=false", "workflow"})
	err := root.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("schema workflow returned error: %v", err)
	}

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)
	output := strings.TrimSpace(stdoutBuf.String())

	type schemaOp struct {
		Resource string `json:"resource"`
		Verb     string `json:"verb"`
	}
	var ops []schemaOp
	if err := json.Unmarshal([]byte(output), &ops); err != nil {
		t.Fatalf("schema workflow output is not a valid JSON array: %v", err)
	}

	if len(ops) != 6 {
		t.Fatalf("expected 6 workflow operations, got %d", len(ops))
	}

	expectedVerbs := map[string]bool{
		"move": false, "copy": false, "publish": false,
		"comment": false, "restrict": false, "archive": false,
	}
	for _, op := range ops {
		if op.Resource != "workflow" {
			t.Errorf("expected resource 'workflow', got %q", op.Resource)
		}
		if _, ok := expectedVerbs[op.Verb]; ok {
			expectedVerbs[op.Verb] = true
		} else {
			t.Errorf("unexpected workflow verb: %q", op.Verb)
		}
	}
	for verb, found := range expectedVerbs {
		if !found {
			t.Errorf("missing workflow verb: %q", verb)
		}
	}
}

func TestSchemaCompactIncludesHandWritten(t *testing.T) {
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

	var compact map[string][]string
	if err := json.Unmarshal([]byte(output), &compact); err != nil {
		t.Fatalf("schema --compact output is not a valid JSON object: %v", err)
	}

	expected := []string{"diff", "workflow", "export", "preset", "templates"}
	for _, want := range expected {
		if _, ok := compact[want]; !ok {
			t.Errorf("schema --compact missing hand-written resource %q", want)
		}
	}
}
