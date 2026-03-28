package diff

import (
	"time"
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
func ParseSince(s string, now time.Time) (time.Time, error) {
	panic("not implemented")
}

// LineStats computes linesAdded and linesRemoved by comparing line sets.
func LineStats(oldBody, newBody string) Stats {
	panic("not implemented")
}

// Compare produces a Result with pairwise DiffEntry items for the given versions.
func Compare(pageID string, versions []VersionInput, opts Options) (*Result, error) {
	panic("not implemented")
}
