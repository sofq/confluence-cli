package cache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sofq/confluence-cli/internal/cache"
)

func TestKeyUniqueness(t *testing.T) {
	t.Run("different auth contexts produce different keys", func(t *testing.T) {
		k1 := cache.Key("GET", "http://x.com", "ctx1")
		k2 := cache.Key("GET", "http://x.com", "ctx2")
		if k1 == k2 {
			t.Error("Keys with different auth context should differ")
		}
	})

	t.Run("different methods produce different keys", func(t *testing.T) {
		url := "http://example.com/api"
		k1 := cache.Key("GET", url)
		k2 := cache.Key("POST", url)
		if k1 == k2 {
			t.Error("Keys with different methods should differ")
		}
	})

	t.Run("different URLs produce different keys", func(t *testing.T) {
		k1 := cache.Key("GET", "http://example.com/api/v1")
		k2 := cache.Key("GET", "http://example.com/api/v2")
		if k1 == k2 {
			t.Error("Keys with different URLs should differ")
		}
	})

	t.Run("same inputs produce same key", func(t *testing.T) {
		k1 := cache.Key("GET", "http://example.com", "authctx")
		k2 := cache.Key("GET", "http://example.com", "authctx")
		if k1 != k2 {
			t.Error("Same inputs should produce same key")
		}
	})

	t.Run("key is non-empty hex string", func(t *testing.T) {
		k := cache.Key("GET", "http://example.com")
		if len(k) == 0 {
			t.Error("Key should not be empty")
		}
		for _, c := range k {
			if ('0' > c || c > '9') && ('a' > c || c > 'f') {
				t.Errorf("Key %q contains non-hex character %q", k, c)
			}
		}
	})
}

func TestGetSetRoundtrip(t *testing.T) {
	// Use a unique key to avoid collisions with other tests
	key := cache.Key("GET", "http://test-roundtrip.example.com/unique-"+t.Name())
	data := []byte(`{"id":42,"name":"test"}`)

	if err := cache.Set(key, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, ok := cache.Get(key, 5*time.Minute)
	if !ok {
		t.Fatal("Get returned false after Set")
	}
	if string(got) != string(data) {
		t.Errorf("Get returned %q, want %q", string(got), string(data))
	}
}

func TestGetExpiredTTL(t *testing.T) {
	key := cache.Key("GET", "http://test-expired.example.com/"+t.Name())
	data := []byte(`{"cached":true}`)

	if err := cache.Set(key, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Negative TTL = immediately expired
	got, ok := cache.Get(key, -1*time.Second)
	if ok {
		t.Errorf("Get should return false for negative TTL, got data: %q", string(got))
	}
	if got != nil {
		t.Errorf("Get should return nil data for expired entry, got: %q", string(got))
	}
}

func TestGetNonExistent(t *testing.T) {
	key := cache.Key("GET", "http://nonexistent.example.com/"+t.Name())
	got, ok := cache.Get(key, 5*time.Minute)
	if ok {
		t.Error("Get should return false for non-existent key")
	}
	if got != nil {
		t.Errorf("Get should return nil for non-existent key, got: %q", string(got))
	}
}

func TestGetReadFileError(t *testing.T) {
	// Trigger the os.ReadFile error branch by placing a directory at the cache
	// key path. Stat succeeds (it's a directory with a valid mtime), but
	// os.ReadFile on a directory returns an error.
	key := cache.Key("GET", "http://test-readfile-err.example.com/"+t.Name())
	cacheFilePath := filepath.Join(cache.Dir(), key)

	// Remove any existing entry first.
	_ = os.Remove(cacheFilePath)

	// Create a directory at the exact key path — Stat succeeds but ReadFile fails.
	if err := os.MkdirAll(cacheFilePath, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheFilePath) })

	got, ok := cache.Get(key, 24*time.Hour)
	if ok {
		t.Error("Get should return false when ReadFile fails (key path is a directory)")
	}
	if got != nil {
		t.Errorf("Get should return nil data when ReadFile fails, got: %q", string(got))
	}
}
