package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// List returns sorted names of available templates (filename without .json extension).
// Returns empty slice if directory does not exist.
func List() ([]string, error) {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			names = append(names, strings.TrimSuffix(name, ".json"))
		}
	}
	sort.Strings(names)
	if names == nil {
		names = []string{}
	}
	return names, nil
}

// Load reads and parses the template file for the given name.
// Looks for {Dir()}/{name}.json.
func Load(name string) (*Template, error) {
	if strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("template name must not contain path separators")
	}
	path := filepath.Join(Dir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", name, err)
	}
	var tmpl Template
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("template %q: invalid JSON: %w", name, err)
	}
	return &tmpl, nil
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
