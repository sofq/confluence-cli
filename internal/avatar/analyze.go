package avatar

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Compiled regexes (package-level for efficiency)
// ---------------------------------------------------------------------------

var (
	rePageBullets    = regexp.MustCompile(`(?m)^[\s]*[-*•]\s`)
	rePageHeadings   = regexp.MustCompile(`(?m)^#{1,6}\s`)
	rePageCodeBlocks = regexp.MustCompile("(?s)```.*?```")
	rePageTables     = regexp.MustCompile(`(?m)^\|`)
	rePageQuestion   = regexp.MustCompile(`\?`)
	rePageExclam     = regexp.MustCompile(`!`)
	rePageFirstPerson = regexp.MustCompile(`(?i)\b(I|I'm|I'll|I've|I'd|my|me|mine)\b`)
	rePageImperative = regexp.MustCompile(`(?i)^(fix|add|update|remove|check|deploy|merge|test|run|set|move)\b`)
)

// rePageEmoji matches common emoji Unicode ranges.
var rePageEmoji = regexp.MustCompile(`[\x{1F300}-\x{1F9FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`)

// stopWords is a small set of English stop words to exclude from jargon extraction.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "as": true, "is": true,
	"was": true, "are": true, "were": true, "be": true, "been": true, "has": true,
	"have": true, "had": true, "do": true, "does": true, "did": true, "will": true,
	"would": true, "should": true, "could": true, "may": true, "might": true,
	"can": true, "this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "we": true, "our": true, "you": true, "your": true,
	"they": true, "their": true, "not": true, "also": true, "just": true,
	"so": true, "if": true, "then": true, "when": true, "there": true,
	"what": true, "which": true, "who": true, "how": true, "all": true,
}

// structureKeywords lists section keywords checked for structure patterns.
// If a keyword appears in >20% of pages, it is added to StructurePatterns.
var structureKeywords = []string{
	"overview", "background", "prerequisites", "steps", "conclusion", "summary",
}

// AnalyzeWriting analyses plain-text page bodies and returns aggregate WritingAnalysis.
// Returns zero-value WritingAnalysis for nil/empty input.
func AnalyzeWriting(bodies []string) WritingAnalysis {
	if len(bodies) == 0 {
		return WritingAnalysis{}
	}

	n := float64(len(bodies))

	// Word counts.
	counts := make([]int, len(bodies))
	for i, b := range bodies {
		counts[i] = wordCount(b)
	}

	// Average word count.
	sum := 0
	for _, c := range counts {
		sum += c
	}
	avg := float64(sum) / n

	// Median word count.
	sorted := make([]int, len(counts))
	copy(sorted, counts)
	sort.Ints(sorted)
	var median float64
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		median = float64(sorted[mid-1]+sorted[mid]) / 2.0
	} else {
		median = float64(sorted[mid])
	}

	// Length distribution. Short <= 100 words, Long >= 500 words.
	var short, long, medium int
	for _, c := range counts {
		switch {
		case c <= 100:
			short++
		case c >= 500:
			long++
		default:
			medium++
		}
	}
	dist := LengthDist{
		ShortPct:  float64(short) / n,
		MediumPct: float64(medium) / n,
		LongPct:   float64(long) / n,
	}

	// Formatting ratios.
	var bullets, headings, codeBlocks, emoji, tables float64
	for _, b := range bodies {
		if rePageBullets.MatchString(b) {
			bullets++
		}
		if rePageHeadings.MatchString(b) {
			headings++
		}
		if rePageCodeBlocks.MatchString(b) {
			codeBlocks++
		}
		if rePageEmoji.MatchString(b) {
			emoji++
		}
		if rePageTables.MatchString(b) {
			tables++
		}
	}
	formatting := FormattingStats{
		UsesBullets:    bullets / n,
		UsesHeadings:   headings / n,
		UsesCodeBlocks: codeBlocks / n,
		UsesEmoji:      emoji / n,
		UsesTables:     tables / n,
	}

	// Tone signals — computed per sentence.
	var totalSentences, questions, exclamations, firstPerson, imperative float64
	for _, b := range bodies {
		for _, s := range splitPageSentences(b) {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			totalSentences++
			if rePageQuestion.MatchString(s) {
				questions++
			}
			if rePageExclam.MatchString(s) {
				exclamations++
			}
			if rePageFirstPerson.MatchString(s) {
				firstPerson++
			}
			if rePageImperative.MatchString(s) {
				imperative++
			}
		}
	}
	tone := ToneSignals{}
	if totalSentences > 0 {
		tone.QuestionRatio = questions / totalSentences
		tone.ExclamationRatio = exclamations / totalSentences
		tone.FirstPersonRatio = firstPerson / totalSentences
		tone.ImperativeRatio = imperative / totalSentences
	}

	// Vocabulary.
	vocab := VocabularyStats{
		CommonPhrases: extractCommonPhrases(bodies, 20),
		Jargon:        extractJargon(bodies),
	}

	// Structure patterns: detect keywords that appear in >20% of pages.
	threshold := n * 0.2
	patternCounts := make(map[string]int)
	for _, b := range bodies {
		lower := strings.ToLower(b)
		for _, kw := range structureKeywords {
			if strings.Contains(lower, kw) {
				patternCounts[kw]++
			}
		}
	}
	var patterns []string
	for _, kw := range structureKeywords {
		if float64(patternCounts[kw]) > threshold {
			patterns = append(patterns, kw)
		}
	}

	return WritingAnalysis{
		AvgLengthWords:    avg,
		MedianLengthWords: median,
		LengthDist:        dist,
		Formatting:        formatting,
		Vocabulary:        vocab,
		ToneSignals:       tone,
		StructurePatterns: patterns,
	}
}

// ---------------------------------------------------------------------------
// Unexported helpers
// ---------------------------------------------------------------------------

// wordCount counts words using strings.Fields.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

// splitPageSentences splits text into sentences on punctuation or newlines.
func splitPageSentences(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		start := 0
		for i := 0; i < len(line); i++ {
			ch := line[i]
			if (ch == '.' || ch == '?' || ch == '!') && i+1 < len(line) && line[i+1] == ' ' {
				chunk := strings.TrimSpace(line[start : i+1])
				if chunk != "" {
					result = append(result, chunk)
				}
				start = i + 2
			}
		}
		if start < len(line) {
			chunk := strings.TrimSpace(line[start:])
			if chunk != "" {
				result = append(result, chunk)
			}
		}
	}
	return result
}

// extractCommonPhrases extracts 2-gram and 3-gram phrases that appear in at
// least 2 texts. Each phrase is counted at most once per text.
// Returns up to maxPhrases results sorted by frequency descending.
func extractCommonPhrases(texts []string, maxPhrases int) []string {
	freq := make(map[string]int)

	for _, t := range texts {
		clean := strings.Map(func(r rune) rune {
			if unicode.IsPunct(r) {
				return ' '
			}
			return unicode.ToLower(r)
		}, t)
		words := strings.Fields(clean)

		seen := make(map[string]bool)
		for i := 0; i < len(words); i++ {
			if i+1 < len(words) {
				seen[words[i]+" "+words[i+1]] = true
			}
			if i+2 < len(words) {
				seen[words[i]+" "+words[i+1]+" "+words[i+2]] = true
			}
		}
		for ng := range seen {
			freq[ng]++
		}
	}

	type entry struct {
		phrase string
		count  int
	}
	var candidates []entry
	for phrase, cnt := range freq {
		if cnt >= 2 {
			candidates = append(candidates, entry{phrase, cnt})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count > candidates[j].count
		}
		return candidates[i].phrase < candidates[j].phrase
	})

	var result []string
	for i, e := range candidates {
		if i >= maxPhrases {
			break
		}
		result = append(result, e.phrase)
	}
	return result
}

// extractJargon finds frequent non-stopword terms (>3 chars, >=3 occurrences)
// and returns the top 10.
func extractJargon(texts []string) []string {
	freq := make(map[string]int)
	for _, t := range texts {
		clean := strings.Map(func(r rune) rune {
			if unicode.IsPunct(r) {
				return ' '
			}
			return unicode.ToLower(r)
		}, t)
		words := strings.Fields(clean)
		seen := make(map[string]bool)
		for _, w := range words {
			if len(w) <= 3 || stopWords[w] {
				continue
			}
			seen[w] = true
		}
		for w := range seen {
			freq[w]++
		}
	}

	type entry struct {
		word  string
		count int
	}
	var candidates []entry
	for word, cnt := range freq {
		if cnt >= 3 {
			candidates = append(candidates, entry{word, cnt})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count > candidates[j].count
		}
		return candidates[i].word < candidates[j].word
	})

	var result []string
	for i, e := range candidates {
		if i >= 10 {
			break
		}
		result = append(result, e.word)
	}
	return result
}
