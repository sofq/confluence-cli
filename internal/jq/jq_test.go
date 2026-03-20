package jq_test

import (
	"strings"
	"testing"

	"github.com/sofq/confluence-cli/internal/jq"
)

func TestApply(t *testing.T) {
	t.Run("simple field selection", func(t *testing.T) {
		input := []byte(`{"a":1}`)
		got, err := jq.Apply(input, ".a")
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		if string(got) != "1" {
			t.Errorf("Apply(.a) = %q, want %q", string(got), "1")
		}
	})

	t.Run("array iteration", func(t *testing.T) {
		input := []byte(`{"results":[{"id":1}]}`)
		got, err := jq.Apply(input, ".results[].id")
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		if string(got) != "1" {
			t.Errorf("Apply(.results[].id) = %q, want %q", string(got), "1")
		}
	})

	t.Run("empty filter returns input unchanged", func(t *testing.T) {
		input := []byte(`{"a":1,"b":2}`)
		got, err := jq.Apply(input, "")
		if err != nil {
			t.Fatalf("Apply with empty filter returned error: %v", err)
		}
		if string(got) != string(input) {
			t.Errorf("Apply with empty filter returned %q, want %q", string(got), string(input))
		}
	})

	t.Run("invalid jq filter returns error", func(t *testing.T) {
		input := []byte(`{"a":1}`)
		_, err := jq.Apply(input, "invalid jq$$")
		if err == nil {
			t.Fatal("Apply with invalid filter should return error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "invalid") {
			t.Errorf("Expected error containing 'invalid', got: %v", err)
		}
	})

	t.Run("invalid JSON input returns error", func(t *testing.T) {
		_, err := jq.Apply([]byte("not json"), ".a")
		if err == nil {
			t.Fatal("Apply with invalid JSON should return error")
		}
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "invalid") && !strings.Contains(errMsg, "json") {
			t.Errorf("Expected error about invalid JSON, got: %v", err)
		}
	})

	t.Run("nested field access", func(t *testing.T) {
		input := []byte(`{"user":{"name":"alice"}}`)
		got, err := jq.Apply(input, ".user.name")
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		// jq returns string with quotes
		if string(got) != `"alice"` {
			t.Errorf("Apply(.user.name) = %q, want %q", string(got), `"alice"`)
		}
	})

	t.Run("multiple results merged into array", func(t *testing.T) {
		input := []byte(`[1,2,3]`)
		got, err := jq.Apply(input, ".[]")
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		// Multiple results: gojq returns them as array
		if !strings.Contains(string(got), "1") || !strings.Contains(string(got), "3") {
			t.Errorf("Apply(.[]) = %q, expected all elements", string(got))
		}
	})
}
