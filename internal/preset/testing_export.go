package preset

// SetUserPresetsPath overrides the userPresetsPath function for tests.
// Returns the previous function so callers can restore it with defer.
func SetUserPresetsPath(f func() string) func() string {
	old := userPresetsPath
	userPresetsPath = f
	return old
}
