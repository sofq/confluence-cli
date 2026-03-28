package duration

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var unitPattern = regexp.MustCompile(`(\d+)\s*(w|d|h|m)`)

// fullPattern validates the entire string is composed only of valid duration tokens
// separated by optional whitespace.
var fullPattern = regexp.MustCompile(`^(\d+\s*(w|d|h|m)\s*)+$`)

// Parse converts a human duration string (e.g. "2h", "1d 3h", "30m") to time.Duration.
// Supported units: w (weeks), d (days), h (hours), m (minutes).
// Calendar conventions: 1d = 24h, 1w = 7d = 168h.
func Parse(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	if !fullPattern.MatchString(s) {
		return 0, fmt.Errorf("invalid duration %q: expected format like 2h, 1d 3h, 30m", s)
	}

	matches := unitPattern.FindAllStringSubmatch(s, -1)

	var total time.Duration
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1]) // regex guarantees digits
		switch m[2] {
		case "w":
			total += time.Duration(n) * 7 * 24 * time.Hour
		case "d":
			total += time.Duration(n) * 24 * time.Hour
		case "h":
			total += time.Duration(n) * time.Hour
		case "m":
			total += time.Duration(n) * time.Minute
		}
	}

	return total, nil
}
