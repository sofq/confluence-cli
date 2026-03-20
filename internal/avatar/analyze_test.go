package avatar_test

import (
	"testing"

	"github.com/sofq/confluence-cli/internal/avatar"
)

func TestAnalyzeWriting_Nil(t *testing.T) {
	result := avatar.AnalyzeWriting(nil)
	if result.AvgLengthWords != 0 {
		t.Errorf("expected AvgLengthWords=0, got %f", result.AvgLengthWords)
	}
	if result.MedianLengthWords != 0 {
		t.Errorf("expected MedianLengthWords=0, got %f", result.MedianLengthWords)
	}
}

func TestAnalyzeWriting_SingleText(t *testing.T) {
	result := avatar.AnalyzeWriting([]string{"Hello world"})
	if result.AvgLengthWords != 2.0 {
		t.Errorf("expected AvgLengthWords=2.0, got %f", result.AvgLengthWords)
	}
}

func TestAnalyzeWriting_BulletsRatio(t *testing.T) {
	texts := []string{
		"- item one\n- item two",
		"* bullet here",
		"plain text no bullets",
	}
	result := avatar.AnalyzeWriting(texts)
	// 2 out of 3 have bullets: 0.666...
	expected := 2.0 / 3.0
	if result.Formatting.UsesBullets < expected-0.01 || result.Formatting.UsesBullets > expected+0.01 {
		t.Errorf("expected UsesBullets≈%f, got %f", expected, result.Formatting.UsesBullets)
	}
}

func TestAnalyzeWriting_Headings(t *testing.T) {
	texts := []string{
		"## Overview\nsome content here",
		"no headings",
	}
	result := avatar.AnalyzeWriting(texts)
	if result.Formatting.UsesHeadings <= 0 {
		t.Errorf("expected UsesHeadings>0, got %f", result.Formatting.UsesHeadings)
	}
}

func TestAnalyzeWriting_CodeBlocks(t *testing.T) {
	texts := []string{
		"Here is some code: ```func main() {}```",
		"no code",
	}
	result := avatar.AnalyzeWriting(texts)
	if result.Formatting.UsesCodeBlocks <= 0 {
		t.Errorf("expected UsesCodeBlocks>0, got %f", result.Formatting.UsesCodeBlocks)
	}
}

func TestAnalyzeWriting_FirstPerson(t *testing.T) {
	texts := []string{"I think this is a good idea."}
	result := avatar.AnalyzeWriting(texts)
	if result.ToneSignals.FirstPersonRatio <= 0 {
		t.Errorf("expected FirstPersonRatio>0, got %f", result.ToneSignals.FirstPersonRatio)
	}
}

func TestExtractCommonPhrases(t *testing.T) {
	texts := []string{
		"hello world foo bar",
		"hello world bar baz",
	}
	result := avatar.AnalyzeWriting(texts)
	// "hello world" should appear as a common phrase (present in both texts)
	found := false
	for _, p := range result.Vocabulary.CommonPhrases {
		if p == "hello world" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'hello world' in common phrases, got: %v", result.Vocabulary.CommonPhrases)
	}
}

func TestAnalyzeWriting_LengthDist(t *testing.T) {
	// Short: <=100 words, Long: >=500 words, Medium: 101-499 words
	shortText := "brief"
	mediumText := ""
	for i := 0; i < 150; i++ {
		mediumText += "word "
	}

	texts := []string{shortText, mediumText}
	result := avatar.AnalyzeWriting(texts)

	// 1 short (1 word), 1 medium (150 words)
	if result.LengthDist.ShortPct != 0.5 {
		t.Errorf("expected ShortPct=0.5, got %f", result.LengthDist.ShortPct)
	}
	if result.LengthDist.MediumPct != 0.5 {
		t.Errorf("expected MediumPct=0.5, got %f", result.LengthDist.MediumPct)
	}
}

func TestAnalyzeWriting_StructurePatterns(t *testing.T) {
	// "overview" keyword in >20% of pages should appear in patterns.
	texts := make([]string, 5)
	texts[0] = "overview of the project"
	texts[1] = "overview section here"
	texts[2] = "unrelated content"
	texts[3] = "other stuff"
	texts[4] = "more things"
	result := avatar.AnalyzeWriting(texts)

	// 2/5 = 40% have "overview", above 20% threshold
	found := false
	for _, p := range result.StructurePatterns {
		if p == "overview" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'overview' in structure patterns, got: %v", result.StructurePatterns)
	}
}

func TestAnalyzeWriting_Tables(t *testing.T) {
	texts := []string{
		"| Col1 | Col2 |\n| val1 | val2 |",
		"plain text",
	}
	result := avatar.AnalyzeWriting(texts)
	if result.Formatting.UsesTables <= 0 {
		t.Errorf("expected UsesTables>0, got %f", result.Formatting.UsesTables)
	}
}
