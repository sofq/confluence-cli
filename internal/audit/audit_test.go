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

func TestNewLogger_ErrorOnUnwritablePath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere; cannot test permission error")
	}
	// Use a path inside a read-only directory to trigger OpenFile error.
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer os.Chmod(dir, 0o700) //nolint:errcheck

	logPath := filepath.Join(dir, "subdir", "audit.log")
	_, err := audit.NewLogger(logPath)
	// MkdirAll cannot create subdir inside read-only dir — should return error.
	if err == nil {
		t.Fatal("expected error when parent directory is read-only, got nil")
	}
}

func TestNewLogger_ErrorOnUnwritableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere; cannot test permission error")
	}
	// Create a directory where the log file path should be — OpenFile on a dir fails.
	dir := t.TempDir()
	// Create a directory at the exact path where the log file should be created.
	logPath := filepath.Join(dir, "audit.log")
	if err := os.Mkdir(logPath, 0o700); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	_, err := audit.NewLogger(logPath)
	if err == nil {
		t.Fatal("expected error when log path is a directory, got nil")
	}
}

func TestDefaultPath_FallbackWhenNoConfigDir(t *testing.T) {
	// os.UserConfigDir() fails when $HOME is unset (both macOS and Linux).
	// Unsetting HOME forces the fallback path: ~/.config/cf/audit.log.
	original, hadHome := os.LookupEnv("HOME")
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if hadHome {
			os.Setenv("HOME", original)
		} else {
			os.Unsetenv("HOME")
		}
		if originalXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})

	// Unset HOME so UserConfigDir returns an error, triggering the fallback.
	os.Unsetenv("HOME")
	os.Unsetenv("XDG_CONFIG_HOME")

	path := audit.DefaultPath()
	// With HOME unset, os.UserHomeDir() also fails; the fallback uses an empty
	// home, so path will be ".config/cf/audit.log" or similar — just verify suffix.
	if !strings.HasSuffix(path, filepath.Join("cf", "audit.log")) {
		t.Errorf("DefaultPath() = %q; want path ending in cf/audit.log", path)
	}
}
