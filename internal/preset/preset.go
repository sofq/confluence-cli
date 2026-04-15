package preset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// builtinPresets contains the default presets shipped with cf.
// Values are JQ expression strings applied to Confluence v2 API JSON responses.
var builtinPresets = map[string]string{
	"brief":  `if has("results") then .results[] | {id, title, status} else {id, title, status} end`,
	"titles": `if has("results") then .results[] | .title else .title end`,
	"agent":  `if has("results") then .results[] | {id, title, status, spaceId, version: .version.number, _links} else {id, title, status, spaceId, version: .version.number, _links} end`,
	"tree":   `if has("results") then .results[] | {id, title, parentId, childPosition: .position} else {id, title, parentId, childPosition: .position} end`,
	"meta":   `. | {id, title, status, version: .version, createdAt, authorId: .authorId, spaceId}`,
	"search": `.results[] | {content: .content.id, title: .content.title, excerpt: .excerpt, url: .url}`,
	"diff":   `. | {id, title, version: .version.number, body}`,
}

// userPresetsPath returns the path to the user-defined presets config file.
// It is a var so tests can override it.
var userPresetsPath = func() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "cf", "presets.json")
	}
	return filepath.Join(dir, "cf", "presets.json")
}

// loadUserPresets reads user-defined presets from disk.
// Returns (nil, nil) if the file doesn't exist.
func loadUserPresets() (map[string]string, error) {
	data, err := os.ReadFile(userPresetsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var presets map[string]string
	if err := json.Unmarshal(data, &presets); err != nil {
		return nil, err
	}
	return presets, nil
}

// Lookup resolves a preset name through the three-tier chain:
// profile config (highest) > user preset file > built-in (lowest).
// Returns (expression, source, error). Source is "profile", "user", or "builtin".
// Returns an error if the preset is not found in any tier or if the user file is malformed.
func Lookup(name string, profilePresets map[string]string) (string, string, error) {
	// Tier 1: profile config (highest priority).
	if expr, ok := profilePresets[name]; ok {
		return expr, "profile", nil
	}

	// Tier 2: user preset file.
	user, err := loadUserPresets()
	if err != nil {
		return "", "", fmt.Errorf("reading user presets: %w", err)
	}
	if expr, ok := user[name]; ok {
		return expr, "user", nil
	}

	// Tier 3: built-in (lowest priority).
	if expr, ok := builtinPresets[name]; ok {
		return expr, "builtin", nil
	}

	return "", "", fmt.Errorf("preset %q not found", name)
}

// presetEntry is used for listing presets with their source.
type presetEntry struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Source     string `json:"source"`
}

// List returns all available presets merged from all three tiers as JSON bytes.
// Higher tiers override lower tiers for the same name.
// profilePresets is the profile-level presets map (may be nil).
func List(profilePresets map[string]string) ([]byte, error) {
	merged := make(map[string]presetEntry)

	// Start with built-in presets (lowest priority).
	for name, expr := range builtinPresets {
		merged[name] = presetEntry{Name: name, Expression: expr, Source: "builtin"}
	}

	// Overlay user-defined presets.
	user, err := loadUserPresets()
	if err != nil {
		return nil, err
	}
	for name, expr := range user {
		merged[name] = presetEntry{Name: name, Expression: expr, Source: "user"}
	}

	// Overlay profile presets (highest priority).
	for name, expr := range profilePresets {
		merged[name] = presetEntry{Name: name, Expression: expr, Source: "profile"}
	}

	// Sort by name for stable output.
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]presetEntry, 0, len(names))
	for _, name := range names {
		result = append(result, merged[name])
	}

	return json.Marshal(result)
}
