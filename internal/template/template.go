package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/sofq/confluence-cli/internal/config"
)

// Dir returns the OS-appropriate templates directory path.
// Derives from config.DefaultPath() parent directory + "templates".
func Dir() string {
	return filepath.Join(filepath.Dir(config.DefaultPath()), "templates")
}

// Template is the raw template file structure.
type Template struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	SpaceID string `json:"space_id,omitempty"`
}

// RenderedTemplate holds the result after variable substitution.
type RenderedTemplate struct {
	Title   string
	Body    string
	SpaceID string
}

// TemplateEntry represents a template in list output with source attribution.
type TemplateEntry struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// ShowOutput represents the full detail output for a single template.
type ShowOutput struct {
	Name      string   `json:"name"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	SpaceID   string   `json:"space_id,omitempty"`
	Source    string   `json:"source"`
	Variables []string `json:"variables"`
}

// varPattern matches Go template variable references like {{.varName}}.
var varPattern = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

// List returns sorted template entries with source attribution.
// Built-in templates are included; user templates from Dir() overlay built-ins.
// Returns only built-in templates if the user directory does not exist.
func List() ([]TemplateEntry, error) {
	merged := make(map[string]TemplateEntry)

	// Start with built-in templates (lowest priority).
	for name := range builtinTemplates {
		merged[name] = TemplateEntry{Name: name, Source: "builtin"}
	}

	// Overlay user templates from Dir() (user overrides built-in for same name).
	entries, err := os.ReadDir(Dir())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			n := strings.TrimSuffix(name, ".json")
			merged[n] = TemplateEntry{Name: n, Source: "user"}
		}
	}

	// Sort by name for stable output.
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]TemplateEntry, 0, len(names))
	for _, name := range names {
		result = append(result, merged[name])
	}
	return result, nil
}

// Load reads and parses the template file for the given name.
// Checks user directory first, then falls back to built-in templates.
func Load(name string) (*Template, error) {
	if strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("template name must not contain path separators")
	}
	path := filepath.Join(Dir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fall back to built-in templates.
			if t, ok := builtinTemplates[name]; ok {
				return t, nil
			}
		}
		return nil, fmt.Errorf("template %q not found: %w", name, err)
	}
	var tmpl Template
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("template %q: invalid JSON: %w", name, err)
	}
	return &tmpl, nil
}

// Show returns the full detail output for a template by name.
// Checks built-in templates first, then user directory via Load().
func Show(name string) (*ShowOutput, error) {
	if strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("template name must not contain path separators")
	}

	var tmpl *Template
	var source string

	// Check user directory first (higher priority).
	path := filepath.Join(Dir(), name+".json")
	data, err := os.ReadFile(path)
	if err == nil {
		var t Template
		if jsonErr := json.Unmarshal(data, &t); jsonErr != nil {
			return nil, fmt.Errorf("template %q: invalid JSON: %w", name, jsonErr)
		}
		tmpl = &t
		source = "user"
	} else if os.IsNotExist(err) {
		// Fall back to built-in.
		if t, ok := builtinTemplates[name]; ok {
			tmpl = t
			source = "builtin"
		}
	}

	if tmpl == nil {
		return nil, fmt.Errorf("template %q not found", name)
	}

	return &ShowOutput{
		Name:      name,
		Title:     tmpl.Title,
		Body:      tmpl.Body,
		SpaceID:   tmpl.SpaceID,
		Source:    source,
		Variables: ExtractVariables(tmpl),
	}, nil
}

// Save writes a template to the user templates directory.
// Creates the directory if it does not exist.
// Returns an error if a file with the same name already exists.
func Save(name string, tmpl *Template) error {
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("template name must not contain path separators")
	}

	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create templates directory: %w", err)
	}

	path := filepath.Join(dir, name+".json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("template %q already exists", name)
	}

	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write template: %w", err)
	}
	return nil
}

// ExtractVariables parses a template's Title, Body, and SpaceID for
// {{.varName}} patterns and returns the unique variable names in order
// of first appearance.
func ExtractVariables(tmpl *Template) []string {
	seen := make(map[string]bool)
	var vars []string
	combined := tmpl.Title + tmpl.Body + tmpl.SpaceID
	for _, matches := range varPattern.FindAllStringSubmatch(combined, -1) {
		name := matches[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}

// Render executes Go text/template substitution on the template's Title, Body,
// and SpaceID using vars as the data map. Uses Option("missingkey=error") so
// missing vars produce an error instead of silent empty strings.
func Render(tmpl *Template, vars map[string]string) (*RenderedTemplate, error) {
	render := func(name, text string) (string, error) {
		t, err := template.New(name).Option("missingkey=error").Parse(text)
		if err != nil {
			return "", fmt.Errorf("parse %s template: %w", name, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, vars); err != nil {
			return "", fmt.Errorf("render %s: %w", name, err)
		}
		return buf.String(), nil
	}

	title, err := render("title", tmpl.Title)
	if err != nil {
		return nil, err
	}
	body, err := render("body", tmpl.Body)
	if err != nil {
		return nil, err
	}
	spaceID, err := render("space_id", tmpl.SpaceID)
	if err != nil {
		return nil, err
	}

	return &RenderedTemplate{
		Title:   title,
		Body:    body,
		SpaceID: spaceID,
	}, nil
}
