package duration

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"30m", 30 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"1w", 168 * time.Hour, false},
		{"1d 3h", 27 * time.Hour, false},
		{"2h 30m", 2*time.Hour + 30*time.Minute, false},
		{"1w 2d 3h 15m", 168*time.Hour + 48*time.Hour + 3*time.Hour + 15*time.Minute, false},
		{"", 0, true},
		{"abc", 0, true},
		{"0h", 0, false},
		{"10m", 10 * time.Minute, false},
		{"2h garbage", 0, true},
		{"abc2h", 0, true},
		{"2hx", 0, true},
		{"2hours", 0, true},
		{"hello 2h world", 0, true},
		{"2h 30m extra", 0, true},
		{" 2h ", 2 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEmptyError(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should contain 'empty', got: %v", err)
	}
}

func TestParseInvalidError(t *testing.T) {
	_, err := Parse("abc")
	if err == nil {
		t.Fatal("expected error for invalid string")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should contain 'invalid', got: %v", err)
	}
}
