package diff

import (
	"fmt"
	"strings"
	"time"

	"github.com/sofq/confluence-cli/internal/duration"
)

// VersionMeta holds metadata for a single page version.
type VersionMeta struct {
	Number    int    `json:"number"`
	AuthorID  string `json:"authorId"`
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message"`
}

// Stats holds line-level change statistics.
type Stats struct {
	LinesAdded   int `json:"linesAdded"`
	LinesRemoved int `json:"linesRemoved"`
}

// DiffEntry represents a single version-to-version comparison.
type DiffEntry struct {
	From  *VersionMeta `json:"from"`
	To    *VersionMeta `json:"to"`
	Stats *Stats       `json:"stats,omitempty"`
	Note  string       `json:"note,omitempty"`
}

// Result is the top-level output of the diff command.
type Result struct {
	PageID string      `json:"pageId"`
	Since  string      `json:"since,omitempty"`
	Diffs  []DiffEntry `json:"diffs"`
}

// Options controls diff behavior.
type Options struct {
	Since string
	From  int
	To    int
	Now   time.Time
}

// VersionInput holds version data passed into Compare.
type VersionInput struct {
	Meta          VersionMeta
	Body          string
	BodyAvailable bool
}

// ParseSince parses a --since value as either an ISO date or a human duration.
// It tries ISO date formats first (RFC3339, datetime, date-only), then falls
// back to duration.Parse for human durations (2h, 1d, 1w).
func ParseSince(s string, now time.Time) (time.Time, error) {
	// Try ISO date formats first (per pitfall 6).
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	// Try duration format via cf's duration parser.
	dur, err := duration.Parse(s)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid --since value %q: expected duration (e.g. 2h, 1d) or date (e.g. 2026-01-01)", s)
	}
	if now.IsZero() {
		now = time.Now()
	}
	// cf's duration.Parse returns time.Duration directly -- no * time.Second.
	return now.Add(-dur), nil
}

// LineStats computes linesAdded and linesRemoved by comparing line frequency
// sets between old and new body content. Bodies are treated as plain text
// split on newline (per D-04).
func LineStats(oldBody, newBody string) Stats {
	oldLines := strings.Split(oldBody, "\n")
	newLines := strings.Split(newBody, "\n")

	oldSet := make(map[string]int)
	for _, line := range oldLines {
		oldSet[line]++
	}

	newSet := make(map[string]int)
	for _, line := range newLines {
		newSet[line]++
	}

	removed := 0
	for line, count := range oldSet {
		newCount := newSet[line]
		if newCount < count {
			removed += count - newCount
		}
	}

	added := 0
	for line, count := range newSet {
		oldCount := oldSet[line]
		if oldCount < count {
			added += count - oldCount
		}
	}

	return Stats{LinesAdded: added, LinesRemoved: removed}
}

// Compare produces a Result with pairwise DiffEntry items for the given
// versions. Versions should be sorted oldest-first (ascending by number).
func Compare(pageID string, versions []VersionInput, opts Options) (*Result, error) {
	// Mutual exclusivity check.
	if opts.Since != "" && (opts.From != 0 || opts.To != 0) {
		return nil, fmt.Errorf("cannot use --since with --from/--to")
	}

	result := &Result{
		PageID: pageID,
		Diffs:  []DiffEntry{}, // non-nil for JSON [] not null (D-12, pitfall 4)
	}

	// Handle --since filtering.
	if opts.Since != "" {
		now := opts.Now
		cutoff, err := ParseSince(opts.Since, now)
		if err != nil {
			return nil, err
		}
		result.Since = opts.Since

		var filtered []VersionInput
		for _, v := range versions {
			t, err := time.Parse(time.RFC3339, v.Meta.CreatedAt)
			if err != nil {
				continue
			}
			if !t.Before(cutoff) {
				filtered = append(filtered, v)
			}
		}
		versions = filtered
	}

	// Handle --from/--to explicit version comparison.
	if opts.From != 0 || opts.To != 0 {
		// from == to: return zero stats (D-13).
		if opts.From == opts.To {
			var target *VersionInput
			for i := range versions {
				if versions[i].Meta.Number == opts.From {
					target = &versions[i]
					break
				}
			}
			if target != nil {
				meta := target.Meta
				entry := DiffEntry{
					From:  &meta,
					To:    &meta,
					Stats: &Stats{LinesAdded: 0, LinesRemoved: 0},
				}
				result.Diffs = append(result.Diffs, entry)
			}
			return result, nil
		}

		// Find the two specified versions.
		var fromV, toV *VersionInput
		for i := range versions {
			if versions[i].Meta.Number == opts.From {
				fromV = &versions[i]
			}
			if versions[i].Meta.Number == opts.To {
				toV = &versions[i]
			}
		}
		if fromV != nil && toV != nil {
			result.Diffs = append(result.Diffs, buildDiffEntry(fromV, toV))
		}
		return result, nil
	}

	// Handle empty input.
	if len(versions) == 0 {
		return result, nil
	}

	// Handle single version (D-11): from=nil, all lines as added.
	if len(versions) == 1 {
		v := &versions[0]
		toMeta := v.Meta
		entry := DiffEntry{
			From: nil,
			To:   &toMeta,
		}
		if v.BodyAvailable {
			stats := LineStats("", v.Body)
			entry.Stats = &stats
		} else {
			entry.Note = fmt.Sprintf("body not available for version %d", v.Meta.Number)
		}
		result.Diffs = append(result.Diffs, entry)
		return result, nil
	}

	// Default mode: compare adjacent pairs (oldest to newest).
	for i := 0; i < len(versions)-1; i++ {
		result.Diffs = append(result.Diffs, buildDiffEntry(&versions[i], &versions[i+1]))
	}

	return result, nil
}

// buildDiffEntry creates a DiffEntry for a pair of versions.
func buildDiffEntry(from, to *VersionInput) DiffEntry {
	fromMeta := from.Meta
	toMeta := to.Meta
	entry := DiffEntry{
		From: &fromMeta,
		To:   &toMeta,
	}

	if !from.BodyAvailable {
		entry.Note = fmt.Sprintf("body not available for version %d", from.Meta.Number)
		return entry
	}
	if !to.BodyAvailable {
		entry.Note = fmt.Sprintf("body not available for version %d", to.Meta.Number)
		return entry
	}

	stats := LineStats(from.Body, to.Body)
	entry.Stats = &stats
	return entry
}
