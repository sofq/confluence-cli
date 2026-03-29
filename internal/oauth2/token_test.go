package oauth2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTokenExpired(t *testing.T) {
	t.Run("not expired within margin", func(t *testing.T) {
		tok := &Token{
			AccessToken: "abc",
			ExpiresIn:   3600,
			ObtainedAt:  time.Now(),
		}
		if tok.Expired(60 * time.Second) {
			t.Error("token should not be expired")
		}
	})

	t.Run("expired past margin", func(t *testing.T) {
		tok := &Token{
			AccessToken: "abc",
			ExpiresIn:   3600,
			ObtainedAt:  time.Now().Add(-3600 * time.Second),
		}
		if !tok.Expired(60 * time.Second) {
			t.Error("token should be expired")
		}
	})

	t.Run("expired within margin", func(t *testing.T) {
		tok := &Token{
			AccessToken: "abc",
			ExpiresIn:   3600,
			ObtainedAt:  time.Now().Add(-3550 * time.Second),
		}
		if !tok.Expired(60 * time.Second) {
			t.Error("token should be expired when within margin")
		}
	})
}

func TestFileStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, "test-profile")

	tok := &Token{
		AccessToken: "my-access-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now().Truncate(time.Second),
	}

	if err := store.Save(tok); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(store.path())
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	loaded := store.Load()
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.AccessToken != "my-access-token" {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, "my-access-token")
	}
	if loaded.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", loaded.ExpiresIn)
	}
}

func TestFileStoreLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, "nonexistent")

	if tok := store.Load(); tok != nil {
		t.Errorf("Load should return nil for non-existent file, got %+v", tok)
	}
}

func TestFileStoreSaveCreatesDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "tokens")
	store := NewFileStore(dir, "test")

	tok := &Token{
		AccessToken: "abc",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now(),
	}
	if err := store.Save(tok); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check directory permissions
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat dir failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir permissions = %o, want 0700", perm)
	}
}

func TestFileStoreSaveAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, "atomic-test")

	tok := &Token{
		AccessToken: "first",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now(),
	}
	if err := store.Save(tok); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Save again -- the file should still be valid JSON (atomic replace)
	tok2 := &Token{
		AccessToken: "second",
		ExpiresIn:   7200,
		ObtainedAt:  time.Now(),
	}
	if err := store.Save(tok2); err != nil {
		t.Fatalf("Save (second) failed: %v", err)
	}

	data, err := os.ReadFile(store.path())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var loaded Token
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if loaded.AccessToken != "second" {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, "second")
	}
}

func TestFileStoreLoadCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir, "corrupt")

	// Write corrupt JSON directly to the token file path.
	if err := os.WriteFile(store.path(), []byte(`{not valid json`), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Load should return nil for corrupt JSON.
	if tok := store.Load(); tok != nil {
		t.Errorf("Load should return nil for corrupt JSON, got %+v", tok)
	}
}

func TestFileStoreSaveWriteError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based write failure test not applicable on Windows")
	}

	base := t.TempDir()
	dir := filepath.Join(base, "tokens")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	// Make the directory read-only so WriteFile fails.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()

	store := NewFileStore(dir, "readonly")
	tok := &Token{
		AccessToken: "abc",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now(),
	}
	if err := store.Save(tok); err == nil {
		t.Error("expected write error for read-only directory, got nil")
	}
}

func TestFileStoreSaveMkdirAllError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based mkdir failure test not applicable on Windows")
	}

	base := t.TempDir()
	// Make base read-only so MkdirAll cannot create the subdirectory.
	if err := os.Chmod(base, 0o500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer func() { _ = os.Chmod(base, 0o700) }()

	// The store dir does not yet exist under the read-only base.
	dir := filepath.Join(base, "newsubdir")
	store := NewFileStore(dir, "profile")
	tok := &Token{
		AccessToken: "abc",
		ExpiresIn:   3600,
		ObtainedAt:  time.Now(),
	}
	if err := store.Save(tok); err == nil {
		t.Error("expected MkdirAll error for read-only parent, got nil")
	}
}
