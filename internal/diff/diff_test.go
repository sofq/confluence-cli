package diff

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

func TestParseSince_Durations(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		{"2h", fixedNow.Add(-2 * time.Hour)},
		{"1d", fixedNow.Add(-24 * time.Hour)},
		{"1w", fixedNow.Add(-168 * time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSince(tt.input, fixedNow)
			if err != nil {
				t.Fatalf("ParseSince(%q) unexpected error: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseSince(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSince_ISODates(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "date only",
			input: "2026-01-15",
			want:  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "datetime without timezone",
			input: "2026-01-15T10:30:00",
			want:  time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339",
			input: "2026-01-15T10:30:00Z",
			want:  time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSince(tt.input, fixedNow)
			if err != nil {
				t.Fatalf("ParseSince(%q) unexpected error: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseSince(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSince_InvalidInput(t *testing.T) {
	_, err := ParseSince("garbage", fixedNow)
	if err == nil {
		t.Fatal("ParseSince(\"garbage\") expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --since") {
		t.Errorf("error should contain 'invalid --since', got: %v", err)
	}
}

func TestParseSince_ZeroNow(t *testing.T) {
	// With zero Now, should use time.Now() internally -- just verify no panic.
	got, err := ParseSince("2h", time.Time{})
	if err != nil {
		t.Fatalf("ParseSince with zero Now: unexpected error: %v", err)
	}
	if got.IsZero() {
		t.Error("ParseSince with zero Now returned zero time")
	}
}

func TestLineStats(t *testing.T) {
	tests := []struct {
		name    string
		oldBody string
		newBody string
		want    Stats
	}{
		{
			name:    "identical content",
			oldBody: "a\nb\nc",
			newBody: "a\nb\nc",
			want:    Stats{LinesAdded: 0, LinesRemoved: 0},
		},
		{
			name:    "one line changed",
			oldBody: "a\nb",
			newBody: "a\nc",
			want:    Stats{LinesAdded: 1, LinesRemoved: 1},
		},
		{
			name:    "empty old to new content",
			oldBody: "",
			newBody: "a\nb\nc",
			want:    Stats{LinesAdded: 3, LinesRemoved: 1},
		},
		{
			name:    "content to empty new",
			oldBody: "a\nb\nc",
			newBody: "",
			want:    Stats{LinesAdded: 1, LinesRemoved: 3},
		},
		{
			name:    "duplicate lines",
			oldBody: "a\na\nb",
			newBody: "a\nc\nc",
			want:    Stats{LinesAdded: 2, LinesRemoved: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LineStats(tt.oldBody, tt.newBody)
			if got != tt.want {
				t.Errorf("LineStats(%q, %q) = %+v, want %+v", tt.oldBody, tt.newBody, got, tt.want)
			}
		})
	}
}

func TestCompare_TwoVersions(t *testing.T) {
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 1, AuthorID: "user1", CreatedAt: "2026-01-01T00:00:00Z", Message: "first"},
			Body:          "line1\nline2",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 2, AuthorID: "user2", CreatedAt: "2026-01-02T00:00:00Z", Message: "second"},
			Body:          "line1\nline3",
			BodyAvailable: true,
		},
	}

	result, err := Compare("12345", versions, Options{})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	if result.PageID != "12345" {
		t.Errorf("PageID = %q, want %q", result.PageID, "12345")
	}
	if len(result.Diffs) != 1 {
		t.Fatalf("len(Diffs) = %d, want 1", len(result.Diffs))
	}

	d := result.Diffs[0]
	if d.From == nil || d.From.Number != 1 {
		t.Errorf("From.Number = %v, want 1", d.From)
	}
	if d.To == nil || d.To.Number != 2 {
		t.Errorf("To.Number = %v, want 2", d.To)
	}
	if d.Stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if d.Stats.LinesAdded != 1 || d.Stats.LinesRemoved != 1 {
		t.Errorf("Stats = %+v, want {LinesAdded:1, LinesRemoved:1}", d.Stats)
	}
}

func TestCompare_SingleVersion(t *testing.T) {
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 1, AuthorID: "user1", CreatedAt: "2026-01-01T00:00:00Z", Message: "initial"},
			Body:          "line1\nline2\nline3",
			BodyAvailable: true,
		},
	}

	result, err := Compare("12345", versions, Options{})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	if len(result.Diffs) != 1 {
		t.Fatalf("len(Diffs) = %d, want 1", len(result.Diffs))
	}

	d := result.Diffs[0]
	if d.From != nil {
		t.Errorf("From should be nil for single version, got %+v", d.From)
	}
	if d.To == nil || d.To.Number != 1 {
		t.Errorf("To.Number = %v, want 1", d.To)
	}
	if d.Stats == nil {
		t.Fatal("Stats should not be nil for single version with body")
	}
	// All lines added: "line1", "line2", "line3" = 3 added, empty string removed = 1
	if d.Stats.LinesAdded != 3 || d.Stats.LinesRemoved != 1 {
		t.Errorf("Stats = %+v, want {LinesAdded:3, LinesRemoved:1}", d.Stats)
	}
}

func TestCompare_EmptyBody(t *testing.T) {
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 1, AuthorID: "user1", CreatedAt: "2026-01-01T00:00:00Z", Message: "first"},
			Body:          "content",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 2, AuthorID: "user2", CreatedAt: "2026-01-02T00:00:00Z", Message: "second"},
			Body:          "",
			BodyAvailable: false,
		},
	}

	result, err := Compare("12345", versions, Options{})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	if len(result.Diffs) != 1 {
		t.Fatalf("len(Diffs) = %d, want 1", len(result.Diffs))
	}

	d := result.Diffs[0]
	if d.Stats != nil {
		t.Errorf("Stats should be nil when body unavailable, got %+v", d.Stats)
	}
	if d.Note == "" {
		t.Error("Note should be set when body unavailable")
	}
	if !strings.Contains(d.Note, "2") {
		t.Errorf("Note should mention version number 2, got %q", d.Note)
	}
}

func TestCompare_NonNilDiffsSlice(t *testing.T) {
	result, err := Compare("12345", []VersionInput{}, Options{})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	// Diffs must be non-nil empty slice for JSON [] not null.
	if result.Diffs == nil {
		t.Fatal("Diffs should be non-nil empty slice, got nil")
	}

	out, _ := json.Marshal(result)
	if !strings.Contains(string(out), `"diffs":[]`) {
		t.Errorf("JSON should contain \"diffs\":[], got %s", string(out))
	}
}

func TestCompare_SinceFiltering(t *testing.T) {
	// fixedNow = 2026-03-15T12:00:00Z
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 1, AuthorID: "user1", CreatedAt: "2026-03-10T00:00:00Z", Message: "old"},
			Body:          "old",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 2, AuthorID: "user2", CreatedAt: "2026-03-14T00:00:00Z", Message: "recent"},
			Body:          "recent",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 3, AuthorID: "user3", CreatedAt: "2026-03-15T10:00:00Z", Message: "latest"},
			Body:          "latest",
			BodyAvailable: true,
		},
	}

	result, err := Compare("12345", versions, Options{Since: "2d", Now: fixedNow})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	if result.Since != "2d" {
		t.Errorf("Since = %q, want %q", result.Since, "2d")
	}

	// Cutoff = 2026-03-13T12:00:00Z, so versions 2 and 3 are within range.
	// That gives us one pairwise diff: v2 -> v3.
	if len(result.Diffs) != 1 {
		t.Fatalf("len(Diffs) = %d, want 1 (v2->v3)", len(result.Diffs))
	}

	if result.Diffs[0].From.Number != 2 || result.Diffs[0].To.Number != 3 {
		t.Errorf("Diff pair = v%d->v%d, want v2->v3", result.Diffs[0].From.Number, result.Diffs[0].To.Number)
	}
}

func TestCompare_FromTo(t *testing.T) {
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 1, AuthorID: "user1", CreatedAt: "2026-01-01T00:00:00Z", Message: "first"},
			Body:          "a\nb",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 2, AuthorID: "user2", CreatedAt: "2026-01-02T00:00:00Z", Message: "second"},
			Body:          "a\nc",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 3, AuthorID: "user3", CreatedAt: "2026-01-03T00:00:00Z", Message: "third"},
			Body:          "a\nd",
			BodyAvailable: true,
		},
	}

	result, err := Compare("12345", versions, Options{From: 1, To: 3})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	if len(result.Diffs) != 1 {
		t.Fatalf("len(Diffs) = %d, want 1", len(result.Diffs))
	}

	d := result.Diffs[0]
	if d.From.Number != 1 || d.To.Number != 3 {
		t.Errorf("From/To = v%d->v%d, want v1->v3", d.From.Number, d.To.Number)
	}
	// "a\nb" vs "a\nd" -> 1 added, 1 removed
	if d.Stats.LinesAdded != 1 || d.Stats.LinesRemoved != 1 {
		t.Errorf("Stats = %+v, want {LinesAdded:1, LinesRemoved:1}", d.Stats)
	}
}

func TestCompare_FromEqualsTo(t *testing.T) {
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 2, AuthorID: "user1", CreatedAt: "2026-01-01T00:00:00Z", Message: "same"},
			Body:          "content",
			BodyAvailable: true,
		},
	}

	result, err := Compare("12345", versions, Options{From: 2, To: 2})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	if len(result.Diffs) != 1 {
		t.Fatalf("len(Diffs) = %d, want 1", len(result.Diffs))
	}

	d := result.Diffs[0]
	if d.Stats == nil {
		t.Fatal("Stats should not be nil for from==to")
	}
	if d.Stats.LinesAdded != 0 || d.Stats.LinesRemoved != 0 {
		t.Errorf("Stats = %+v, want {LinesAdded:0, LinesRemoved:0}", d.Stats)
	}
}

func TestCompare_MutualExclusivity(t *testing.T) {
	_, err := Compare("12345", nil, Options{Since: "2h", From: 1, To: 2})
	if err == nil {
		t.Fatal("expected error for --since with --from/--to, got nil")
	}
	if !strings.Contains(err.Error(), "cannot use --since with --from/--to") {
		t.Errorf("error = %q, want to contain 'cannot use --since with --from/--to'", err.Error())
	}
}

func TestCompare_MultipleAdjacentPairs(t *testing.T) {
	versions := []VersionInput{
		{
			Meta:          VersionMeta{Number: 1, AuthorID: "user1", CreatedAt: "2026-01-01T00:00:00Z", Message: "v1"},
			Body:          "a",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 2, AuthorID: "user2", CreatedAt: "2026-01-02T00:00:00Z", Message: "v2"},
			Body:          "b",
			BodyAvailable: true,
		},
		{
			Meta:          VersionMeta{Number: 3, AuthorID: "user3", CreatedAt: "2026-01-03T00:00:00Z", Message: "v3"},
			Body:          "c",
			BodyAvailable: true,
		},
	}

	result, err := Compare("12345", versions, Options{})
	if err != nil {
		t.Fatalf("Compare unexpected error: %v", err)
	}

	// 3 versions = 2 adjacent pairs: v1->v2, v2->v3
	if len(result.Diffs) != 2 {
		t.Fatalf("len(Diffs) = %d, want 2", len(result.Diffs))
	}

	if result.Diffs[0].From.Number != 1 || result.Diffs[0].To.Number != 2 {
		t.Errorf("Diffs[0] = v%d->v%d, want v1->v2", result.Diffs[0].From.Number, result.Diffs[0].To.Number)
	}
	if result.Diffs[1].From.Number != 2 || result.Diffs[1].To.Number != 3 {
		t.Errorf("Diffs[1] = v%d->v%d, want v2->v3", result.Diffs[1].From.Number, result.Diffs[1].To.Number)
	}
}
