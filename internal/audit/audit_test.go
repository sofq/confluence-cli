package audit_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sofq/confluence-cli/internal/audit"
)

func TestNewLogger_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "audit.log")

	l, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer l.Close()

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file to be created, stat failed: %v", err)
	}
}

func TestLog_WritesNDJSONLine(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	l, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	l.Log(audit.Entry{
		Profile:   "myprofile",
		Operation: "pages:get",
		Method:    "GET",
		Path:      "/pages/123",
		Status:    200,
		Exit:      0,
	})
	l.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v — line: %q", err, lines[0])
	}

	// Check required fields.
	if entry["ts"] == nil || entry["ts"] == "" {
		t.Error("expected 'ts' field to be set")
	}
	if entry["profile"] != "myprofile" {
		t.Errorf("expected profile=myprofile, got %v", entry["profile"])
	}
	if entry["op"] != "pages:get" {
		t.Errorf("expected op=pages:get, got %v", entry["op"])
	}
	if entry["method"] != "GET" {
		t.Errorf("expected method=GET, got %v", entry["method"])
	}
	if entry["path"] != "/pages/123" {
		t.Errorf("expected path=/pages/123, got %v", entry["path"])
	}
}

func TestLog_ConcurrentWritesDontCorruptFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	l, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer l.Close()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			l.Log(audit.Entry{
				Operation: "pages:list",
				Method:    "GET",
				Path:      "/pages",
				Status:    200,
			})
		}()
	}
	wg.Wait()
	l.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("corrupt NDJSON line %d: %v — line: %q", count+1, err, line)
		}
		count++
	}
	if count != goroutines {
		t.Fatalf("expected %d log lines, got %d", goroutines, count)
	}
}

func TestNilLogger_Log_IsNoOp(t *testing.T) {
	var l *audit.Logger
	// Should not panic.
	l.Log(audit.Entry{Operation: "test"})
}

func TestNilLogger_Close_IsNoOp(t *testing.T) {
	var l *audit.Logger
	// Should not panic.
	l.Close()
}

func TestClose_MakesSubsequentLog_NoOp(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	l, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	l.Log(audit.Entry{Operation: "before-close"})
	l.Close()
	// After close, Log should be a no-op, not panic.
	l.Log(audit.Entry{Operation: "after-close"})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line (before close only), got %d", len(lines))
	}
}

func TestDefaultPath_EndsWithCfAuditLog(t *testing.T) {
	path := audit.DefaultPath()
	if !strings.HasSuffix(path, filepath.Join("cf", "audit.log")) {
		t.Errorf("DefaultPath() = %q; want path ending in cf/audit.log", path)
	}
}
