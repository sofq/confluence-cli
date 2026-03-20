// Package avatar contains types and logic for the avatar feature, which
// analyses a user's Confluence writing activity to generate a writing profile.
package avatar

import "time"

// PageRecord is a single Confluence page as returned by FetchUserPages.
type PageRecord struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`         // plain text (HTML stripped)
	LastModified time.Time `json:"last_modified"`
}

// PersonaProfile is the top-level JSON document output by cf avatar analyze.
type PersonaProfile struct {
	Version     string          `json:"version"`
	AccountID   string          `json:"account_id"`
	DisplayName string          `json:"display_name"`
	GeneratedAt string          `json:"generated_at"` // RFC3339
	PageCount   int             `json:"page_count"`
	Writing     WritingAnalysis `json:"writing"`
	StyleGuide  StyleGuide      `json:"style_guide"`
	Examples    []PageExample   `json:"examples,omitempty"`
}

// StyleGuide holds prose guidance sentences for writing style.
type StyleGuide struct {
	Writing string `json:"writing"`
}

// WritingAnalysis aggregates statistics derived from page bodies.
type WritingAnalysis struct {
	AvgLengthWords    float64         `json:"avg_length_words"`
	MedianLengthWords float64         `json:"median_length_words"`
	LengthDist        LengthDist      `json:"length_dist"`
	Formatting        FormattingStats `json:"formatting"`
	Vocabulary        VocabularyStats `json:"vocabulary"`
	ToneSignals       ToneSignals     `json:"tone_signals"`
	StructurePatterns []string        `json:"structure_patterns"`
}

// LengthDist breaks down the percentage of short/medium/long texts.
// Short: <=100 words, Long: >=500 words.
type LengthDist struct {
	ShortPct  float64 `json:"short_pct"`
	MediumPct float64 `json:"medium_pct"`
	LongPct   float64 `json:"long_pct"`
}

// FormattingStats records the fraction of pages using each formatting element.
type FormattingStats struct {
	UsesBullets    float64 `json:"uses_bullets"`
	UsesHeadings   float64 `json:"uses_headings"`
	UsesCodeBlocks float64 `json:"uses_code_blocks"`
	UsesEmoji      float64 `json:"uses_emoji"`
	UsesTables     float64 `json:"uses_tables"`
}

// VocabularyStats captures repeated phrases and idiomatic language.
type VocabularyStats struct {
	CommonPhrases []string `json:"common_phrases"`
	Jargon        []string `json:"jargon"`
}

// ToneSignals measures stylistic ratios.
type ToneSignals struct {
	QuestionRatio    float64 `json:"question_ratio"`
	ExclamationRatio float64 `json:"exclamation_ratio"`
	FirstPersonRatio float64 `json:"first_person_ratio"`
	ImperativeRatio  float64 `json:"imperative_ratio"`
}

// PageExample is a representative excerpt included in the profile.
type PageExample struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
