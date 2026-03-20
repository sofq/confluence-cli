package avatar

import (
	"bytes"
	"sort"
	"text/template"
	"time"
)

// styleGuideTmplSrc is the Go template for generating StyleGuide.Writing prose.
const styleGuideTmplSrc = `{{.DisplayName}} writes {{.LengthDesc}} pages — typically {{.MedianWords}} words.{{if .HeadingHigh}} Frequently uses headings and structured sections.{{end}}{{if .BulletHigh}} Uses bullet points for lists.{{end}}{{if .CodeHigh}} Includes code blocks for technical content.{{end}}`

// styleGuideData is the template data for StyleGuide.Writing.
type styleGuideData struct {
	DisplayName string
	LengthDesc  string
	MedianWords int
	HeadingHigh bool
	BulletHigh  bool
	CodeHigh    bool
}

var styleGuideTmpl = template.Must(template.New("style_guide").Parse(styleGuideTmplSrc))

// pageLengthDescription returns a human-readable description of page length.
func pageLengthDescription(median float64) string {
	switch {
	case median <= 100:
		return "short"
	case median >= 500:
		return "long"
	default:
		return "medium-length"
	}
}

// BuildProfile composes a PersonaProfile from a user's Confluence pages.
// It calls AnalyzeWriting on the page bodies, generates a StyleGuide prose
// description, and picks up to 3 representative page examples.
func BuildProfile(accountID, displayName string, pages []PageRecord) *PersonaProfile {
	// Extract body strings for analysis.
	bodies := make([]string, len(pages))
	for i, p := range pages {
		bodies[i] = p.Body
	}

	writing := AnalyzeWriting(bodies)

	// Generate StyleGuide.Writing prose.
	name := displayName
	if name == "" {
		name = accountID
	}
	data := styleGuideData{
		DisplayName: name,
		LengthDesc:  pageLengthDescription(writing.MedianLengthWords),
		MedianWords: int(writing.MedianLengthWords),
		HeadingHigh: writing.Formatting.UsesHeadings > 0.4,
		BulletHigh:  writing.Formatting.UsesBullets > 0.4,
		CodeHigh:    writing.Formatting.UsesCodeBlocks > 0.2,
	}
	var buf bytes.Buffer
	_ = styleGuideTmpl.Execute(&buf, data)
	styleGuideWriting := buf.String()

	// Select up to 3 examples: longest pages by word count, trim body to 300 chars.
	examples := selectExamples(pages, 3)

	return &PersonaProfile{
		Version:     "1",
		AccountID:   accountID,
		DisplayName: displayName,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		PageCount:   len(pages),
		Writing:     writing,
		StyleGuide: StyleGuide{
			Writing: styleGuideWriting,
		},
		Examples: examples,
	}
}

// selectExamples picks the top n pages by word count and trims their body
// to 300 characters (appending "..." if truncated).
func selectExamples(pages []PageRecord, n int) []PageExample {
	if len(pages) == 0 {
		return nil
	}

	// Sort pages by word count descending.
	sorted := make([]PageRecord, len(pages))
	copy(sorted, pages)
	sort.Slice(sorted, func(i, j int) bool {
		return wordCount(sorted[i].Body) > wordCount(sorted[j].Body)
	})

	const maxChars = 300
	var examples []PageExample
	for i := 0; i < n && i < len(sorted); i++ {
		p := sorted[i]
		text := p.Body
		if len(text) > maxChars {
			text = text[:maxChars] + "..."
		}
		examples = append(examples, PageExample{
			Title: p.Title,
			Text:  text,
		})
	}
	return examples
}
