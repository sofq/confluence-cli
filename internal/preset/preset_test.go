package preset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookup_BuiltinPresets(t *testing.T) {
	tests := []struct {
		name      string
		wantFound bool
	}{
		{"brief", true},
		{"titles", true},
		{"agent", true},
		{"tree", true},
		{"meta", true},
		{"search", true},
		{"diff", true},
		{"nonexistent", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, source, err := Lookup(tt.name, nil)
			if tt.wantFound {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if expr == "" {
					t.Errorf("Lookup(%q) returned empty expression", tt.name)
				}
				if source != "builtin" {
					t.Errorf("Lookup(%q) source = %q, want %q", tt.name, source, "builtin")
				}
			} else {
				if err == nil {
					t.Errorf("Lookup(%q) expected error, got nil", tt.name)
				}
			}
		})
	}
}

func TestLookup_BuiltinPresetExpressions(t *testing.T) {
	expr, source, err := Lookup("brief", nil)
	if err != nil {
		t.Fatal(err)
	}
	if source != "builtin" {
		t.Errorf("source = %q, want %q", source, "builtin")
	}
	if !strings.Contains(expr, ".results[]") {
		t.Errorf("brief expression should contain .results[], got %q", expr)
	}
}

func TestLookup_ProfileOverridesBuiltin(t *testing.T) {
	profilePresets := map[string]string{
		"brief": ".title",
	}
	expr, source, err := Lookup("brief", profilePresets)
	if err != nil {
		t.Fatal(err)
	}
	if expr != ".title" {
		t.Errorf("expression = %q, want %q", expr, ".title")
	}
	if source != "profile" {
		t.Errorf("source = %q, want %q", source, "profile")
	}
}

func TestLookup_UserOverridesBuiltin(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")

	userPresets := map[string]string{
		"brief": `.results[] | .title`,
	}
	data, _ := json.Marshal(userPresets)
	if err := os.WriteFile(presetsFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	expr, source, err := Lookup("brief", nil)
	if err != nil {
		t.Fatal(err)
	}
	if expr != `.results[] | .title` {
		t.Errorf("expected user override expression, got: %s", expr)
	}
	if source != "user" {
		t.Errorf("source = %q, want %q", source, "user")
	}
}

func TestLookup_ProfileOverridesUser(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")

	userPresets := map[string]string{
		"brief": `.results[] | .title`,
	}
	data, _ := json.Marshal(userPresets)
	if err := os.WriteFile(presetsFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	profilePresets := map[string]string{
		"brief": ".id",
	}
	expr, source, err := Lookup("brief", profilePresets)
	if err != nil {
		t.Fatal(err)
	}
	if expr != ".id" {
		t.Errorf("expected profile override, got: %s", expr)
	}
	if source != "profile" {
		t.Errorf("source = %q, want %q", source, "profile")
	}
}

func TestLookup_ThreeTierResolution(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")

	userPresets := map[string]string{
		"brief":  `.results[] | .title`,
		"custom": ".custom_user",
	}
	data, _ := json.Marshal(userPresets)
	if err := os.WriteFile(presetsFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	profilePresets := map[string]string{
		"brief": ".profile_override",
	}

	// Profile wins over user and builtin.
	expr, source, err := Lookup("brief", profilePresets)
	if err != nil {
		t.Fatal(err)
	}
	if expr != ".profile_override" {
		t.Errorf("expected profile to win, got: %s", expr)
	}
	if source != "profile" {
		t.Errorf("source = %q, want %q", source, "profile")
	}

	// User preset not in profile resolves as "user".
	expr, source, err = Lookup("custom", profilePresets)
	if err != nil {
		t.Fatal(err)
	}
	if expr != ".custom_user" {
		t.Errorf("expected user preset, got: %s", expr)
	}
	if source != "user" {
		t.Errorf("source = %q, want %q", source, "user")
	}

	// Builtin preset not in profile or user resolves as "builtin".
	_, source, err = Lookup("titles", profilePresets)
	if err != nil {
		t.Fatal(err)
	}
	if source != "builtin" {
		t.Errorf("source = %q, want %q", source, "builtin")
	}
}

func TestLookup_UserOnlyPreset(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")

	userPresets := map[string]string{
		"custom": ".my_custom",
	}
	data, _ := json.Marshal(userPresets)
	if err := os.WriteFile(presetsFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	expr, source, err := Lookup("custom", nil)
	if err != nil {
		t.Fatal(err)
	}
	if expr != ".my_custom" {
		t.Errorf("expression = %q, want %q", expr, ".my_custom")
	}
	if source != "user" {
		t.Errorf("source = %q, want %q", source, "user")
	}
}

func TestLookup_ProfileOnlyPreset(t *testing.T) {
	profilePresets := map[string]string{
		"custom": ".profile_custom",
	}
	expr, source, err := Lookup("custom", profilePresets)
	if err != nil {
		t.Fatal(err)
	}
	if expr != ".profile_custom" {
		t.Errorf("expression = %q, want %q", expr, ".profile_custom")
	}
	if source != "profile" {
		t.Errorf("source = %q, want %q", source, "profile")
	}
}

func TestLookup_MalformedUserPresets(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")
	if err := os.WriteFile(presetsFile, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	_, _, err := Lookup("agent", nil)
	if err == nil {
		t.Error("expected error for malformed presets file, got nil")
	}
}

func TestLookup_EmptyUserPresetsFile(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")
	if err := os.WriteFile(presetsFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	// Built-in presets should still work with an empty user file.
	expr, source, err := Lookup("agent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if expr == "" {
		t.Error("agent preset has empty expression")
	}
	if source != "builtin" {
		t.Errorf("source = %q, want %q", source, "builtin")
	}
}

func TestLookup_NotFound(t *testing.T) {
	_, _, err := Lookup("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should contain 'not found', got: %v", err)
	}
}

func TestList_ReturnsAllBuiltinPresets(t *testing.T) {
	data, err := List(nil)
	if err != nil {
		t.Fatal(err)
	}

	var entries []struct {
		Name       string `json:"name"`
		Expression string `json:"expression"`
		Source     string `json:"source"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to parse list output: %v", err)
	}

	if len(entries) != 7 {
		t.Errorf("expected 7 presets, got %d", len(entries))
	}

	// Verify all are builtin.
	for _, e := range entries {
		if e.Source != "builtin" {
			t.Errorf("preset %q has source %q, want %q", e.Name, e.Source, "builtin")
		}
	}

	// Verify sorted order.
	for i := 1; i < len(entries); i++ {
		if entries[i].Name < entries[i-1].Name {
			t.Errorf("presets not sorted: %q comes after %q", entries[i].Name, entries[i-1].Name)
		}
	}
}

func TestList_IncludesUserPresets(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")

	userPresets := map[string]string{
		"mypreset": ".my_expression",
	}
	data, _ := json.Marshal(userPresets)
	if err := os.WriteFile(presetsFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	listData, err := List(nil)
	if err != nil {
		t.Fatal(err)
	}

	var entries []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(listData, &entries); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range entries {
		if e.Name == "mypreset" && e.Source == "user" {
			found = true
		}
	}
	if !found {
		t.Error("user preset 'mypreset' not found in list output")
	}
}

func TestList_IncludesProfilePresets(t *testing.T) {
	profilePresets := map[string]string{
		"custom": ".id",
	}

	listData, err := List(profilePresets)
	if err != nil {
		t.Fatal(err)
	}

	var entries []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(listData, &entries); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range entries {
		if e.Name == "custom" && e.Source == "profile" {
			found = true
		}
	}
	if !found {
		t.Error("profile preset 'custom' not found in list output")
	}
}

func TestList_ProfileOverridesInList(t *testing.T) {
	profilePresets := map[string]string{
		"brief": ".profile_brief",
	}

	listData, err := List(profilePresets)
	if err != nil {
		t.Fatal(err)
	}

	var entries []struct {
		Name       string `json:"name"`
		Expression string `json:"expression"`
		Source     string `json:"source"`
	}
	if err := json.Unmarshal(listData, &entries); err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if e.Name == "brief" {
			if e.Source != "profile" {
				t.Errorf("brief source = %q, want %q", e.Source, "profile")
			}
			if e.Expression != ".profile_brief" {
				t.Errorf("brief expression = %q, want %q", e.Expression, ".profile_brief")
			}
			return
		}
	}
	t.Error("brief preset not found in list output")
}

func TestList_MalformedUserPresets(t *testing.T) {
	tmpDir := t.TempDir()
	presetsFile := filepath.Join(tmpDir, "presets.json")
	if err := os.WriteFile(presetsFile, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := userPresetsPath
	userPresetsPath = func() string { return presetsFile }
	t.Cleanup(func() { userPresetsPath = orig })

	_, err := List(nil)
	if err == nil {
		t.Error("expected error for malformed presets file, got nil")
	}
}

func TestUserPresetsPath_Default(t *testing.T) {
	p := userPresetsPath()
	if p == "" {
		t.Error("default userPresetsPath() returned empty string")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got %q", p)
	}
	if filepath.Base(p) != "presets.json" {
		t.Errorf("expected filename 'presets.json', got %q", filepath.Base(p))
	}
}

func TestLoadUserPresets_NonExistentError(t *testing.T) {
	tmpDir := t.TempDir()
	// tmpDir itself is a directory -- ReadFile on a directory returns an error
	// that is NOT os.ErrNotExist on all major platforms.
	orig := userPresetsPath
	userPresetsPath = func() string { return tmpDir }
	t.Cleanup(func() { userPresetsPath = orig })

	_, _, err := Lookup("agent", nil)
	if err == nil {
		t.Fatal("expected error when presets path points to a directory, got nil")
	}
}

// TestUserPresetsPathFallback covers the error branch inside the default
// userPresetsPath implementation (lines 28-30 of preset.go) where
// os.UserConfigDir() fails and the function falls back to os.UserHomeDir().
// On Unix, os.UserConfigDir() fails when both HOME and XDG_CONFIG_HOME are
// unset; on macOS it specifically needs HOME.
// TestSetUserPresetsPath covers the SetUserPresetsPath export in testing_export.go.
func TestSetUserPresetsPath(t *testing.T) {
	var called bool
	old := SetUserPresetsPath(func() string {
		called = true
		return "/test/path/presets.json"
	})
	defer func() { SetUserPresetsPath(old) }()

	path := userPresetsPath()
	if !called {
		t.Error("SetUserPresetsPath: injected function was not stored")
	}
	if path != "/test/path/presets.json" {
		t.Errorf("userPresetsPath() = %q, want /test/path/presets.json", path)
	}
}

func TestUserPresetsPathFallback(t *testing.T) {
	if os.Getenv("HOME") == "" {
		t.Skip("HOME already unset; cannot produce meaningful fallback test")
	}

	// Restore the package-level var to the original implementation before
	// overriding env vars, so we execute the real default function body.
	// Any earlier test that replaced userPresetsPath will have restored it via
	// t.Cleanup, so we capture the current value (which is the original) and
	// put it back when done.
	origFn := userPresetsPath
	t.Cleanup(func() { userPresetsPath = origFn })

	// Unset the env vars that os.UserConfigDir() reads on Unix / macOS.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Call the default implementation directly (origFn still holds the
	// original closure; userPresetsPath has not been replaced yet).
	path := origFn()

	// The fallback path ends with the expected suffix regardless of the
	// exact home directory value (which may be empty).
	if !strings.HasSuffix(path, filepath.Join(".config", "cf", "presets.json")) {
		t.Errorf("fallback path = %q; want suffix %q", path, filepath.Join(".config", "cf", "presets.json"))
	}
}
