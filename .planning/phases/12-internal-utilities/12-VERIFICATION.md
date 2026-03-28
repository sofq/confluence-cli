---
phase: 12-internal-utilities
verified: 2026-03-28T14:30:00Z
status: passed
score: 13/13 must-haves verified
re_verification: false
---

# Phase 12: Internal Utilities Verification Report

**Phase Goal:** Pure-logic internal packages exist and are fully tested, providing the foundation that all subsequent CLI commands depend on.
**Verified:** 2026-03-28T14:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | MarshalNoEscape serializes Go values to JSON without HTML-escaping &, <, > characters | VERIFIED | `internal/jsonutil/jsonutil.go:14` — `enc.SetEscapeHTML(false)` + `bytes.TrimRight`; TestMarshalNoEscape passes |
| 2 | NewEncoder returns a json.Encoder pre-configured with SetEscapeHTML(false) for streaming to io.Writer | VERIFIED | `internal/jsonutil/jsonutil.go:23-27`; TestNewEncoder + TestNewEncoderNoEscape pass |
| 3 | duration.Parse('2h') returns 2*time.Hour, Parse('1d') returns 24*time.Hour, Parse('1w') returns 168*time.Hour | VERIFIED | `internal/duration/duration.go:37-43`; 18 table-driven test cases cover all units; `go test` exits 0 |
| 4 | duration.Parse('1d 3h') returns 27*time.Hour (compound expressions work) | VERIFIED | `internal/duration/duration_test.go:19`; TestParse/1d_3h passes |
| 5 | duration.Parse('') and Parse('abc') return descriptive errors | VERIFIED | `duration.go:23-27`; TestParseEmptyError asserts "empty" in message, TestParseInvalidError asserts "invalid" |
| 6 | preset.Lookup('brief', profilePresets) returns built-in JQ expression and source 'builtin' when no override exists | VERIFIED | `internal/preset/preset.go:56-77`; TestLookup_BuiltinPresets passes for all 7 built-ins |
| 7 | Profile-level presets override user-level and built-in presets with the same name | VERIFIED | `preset.go:57-60`; TestLookup_ProfileOverridesBuiltin, TestLookup_ProfileOverridesUser, TestLookup_ThreeTierResolution all pass |
| 8 | User-level presets override built-in presets with the same name | VERIFIED | `preset.go:62-69`; TestLookup_UserOverridesBuiltin passes |
| 9 | preset.List(profilePresets) returns JSON array of all presets with source attribution (builtin/user/profile) | VERIFIED | `preset.go:89-124`; TestList_ReturnsAllBuiltinPresets (7 entries, sorted), TestList_IncludesUserPresets, TestList_IncludesProfilePresets, TestList_ProfileOverridesInList all pass |
| 10 | cmd/root.go --preset flag resolves through three-tier chain instead of profile-only lookup | VERIFIED | `cmd/root.go:179` — `preset_pkg.Lookup(preset, rawProfile.Presets)`; no `rawProfile.Presets[preset]` direct map access remains |
| 11 | Preset not found in any tier returns a descriptive error | VERIFIED | `preset.go:76` — `fmt.Errorf("preset %q not found", name)`; TestLookup_NotFound asserts "not found" in message |
| 12 | No file in cmd/ or internal/ contains inline SetEscapeHTML(false) calls outside jsonutil package | VERIFIED | `grep -rn "SetEscapeHTML" cmd/ internal/ --include="*.go" | grep -v jsonutil/jsonutil.go` returns zero results |
| 13 | All existing tests still pass after refactoring | VERIFIED | `go test ./... -count=1` — all 15 packages pass, 0 failures |

**Score:** 13/13 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/jsonutil/jsonutil.go` | MarshalNoEscape and NewEncoder functions | VERIFIED | Exists, 28 lines, exports both functions, `SetEscapeHTML(false)` in each |
| `internal/jsonutil/jsonutil_test.go` | Unit tests for jsonutil package | VERIFIED | Exists, 82 lines (>30), 5 test functions covering no-escape, no-trailing-newline, error, encoder |
| `internal/duration/duration.go` | Parse function returning time.Duration | VERIFIED | Exists, 48 lines, exports `Parse(s string) (time.Duration, error)`, calendar constants confirmed |
| `internal/duration/duration_test.go` | Unit tests for duration package | VERIFIED | Exists, 68 lines (>30), 20 test cases (18 table-driven + 2 error message assertions) |
| `internal/preset/preset.go` | Lookup and List functions with three-tier resolution | VERIFIED | Exists, 124 lines, exports `Lookup` and `List`, 7 built-in presets, `userPresetsPath` var for testability |
| `internal/preset/preset_test.go` | Comprehensive tests for three-tier resolution | VERIFIED | Exists, 464 lines (>100), 18 test functions covering all resolution paths |
| `cmd/root.go` | Three-tier preset resolution via preset.Lookup | VERIFIED | Contains `preset_pkg.Lookup(preset, rawProfile.Presets)` at line 179 |
| `internal/client/client.go` | Refactored to use jsonutil.MarshalNoEscape for all 5 sites | VERIFIED | 5 occurrences of `jsonutil.MarshalNoEscape` at lines 150, 368, 376, 380, 491 |
| `internal/jq/jq.go` | Refactored to use jsonutil.MarshalNoEscape, local function removed | VERIFIED | `marshalNoHTMLEscape` function deleted; 2 call sites use `jsonutil.MarshalNoEscape` |
| `internal/errors/errors.go` | Refactored WriteJSON to use jsonutil.NewEncoder | VERIFIED | Line 68: `_ = jsonutil.NewEncoder(w).Encode(e)` |
| `cmd/schema_cmd.go` | Local marshalNoEscape removed, callers use jsonutil.MarshalNoEscape | VERIFIED | `func marshalNoEscape` deleted; 4 call sites use `jsonutil.MarshalNoEscape` |
| `cmd/watch.go` | Refactored to use jsonutil.NewEncoder for streaming | VERIFIED | Line 84: `enc := jsonutil.NewEncoder(c.Stdout)` |
| `cmd/batch.go` | Refactored to use jsonutil.MarshalNoEscape | VERIFIED | Line 169: `output, _ := jsonutil.MarshalNoEscape(results)` |
| `cmd/root.go` | Refactored help and Execute functions to use jsonutil | VERIFIED | Line 306: `jsonutil.MarshalNoEscape`; Line 330: `jsonutil.NewEncoder(os.Stderr).Encode` |
| `cmd/version.go` | Refactored to use jsonutil.MarshalNoEscape | VERIFIED | Line 13: `jsonutil.MarshalNoEscape(map[string]string{"version": Version})` |
| `cmd/configure.go` | Refactored to use jsonutil.MarshalNoEscape | VERIFIED | 3 call sites at lines 197, 263, 311 |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/jsonutil/jsonutil.go` | `encoding/json` | `json.NewEncoder` with `SetEscapeHTML(false)` | WIRED | `SetEscapeHTML(false)` present in both functions |
| `internal/duration/duration.go` | `time` | returns `time.Duration` values | WIRED | `time.Duration(n) * 24 * time.Hour` (days), `time.Duration(n) * 7 * 24 * time.Hour` (weeks) |
| `cmd/root.go` | `internal/preset/preset.go` | `preset_pkg.Lookup` call in PersistentPreRunE | WIRED | Line 17: import alias; Line 179: `preset_pkg.Lookup(preset, rawProfile.Presets)` |
| `internal/preset/preset.go` | `internal/config/config.go` | Profile.Presets map passed as profilePresets parameter | WIRED | `profilePresets map[string]string` parameter matches `Profile.Presets` field type |
| `internal/client/client.go` | `internal/jsonutil/jsonutil.go` | import and direct call | WIRED | 5 occurrences of `jsonutil.MarshalNoEscape` confirmed |
| `internal/jq/jq.go` | `internal/jsonutil/jsonutil.go` | import replacing local function | WIRED | 2 call sites confirmed, `marshalNoHTMLEscape` deleted |
| `cmd/schema_cmd.go` | `internal/jsonutil/jsonutil.go` | import replacing local marshalNoEscape | WIRED | 4 call sites confirmed, `func marshalNoEscape` deleted |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| UTIL-01 | Plans 01, 03 | JSON output uses `MarshalNoEscape()` to prevent HTML entity corruption in XHTML content | SATISFIED | `internal/jsonutil` package created; all 12+ inline `SetEscapeHTML(false)` sites across 9 files replaced; zero remaining outside `jsonutil/jsonutil.go` |
| UTIL-02 | Plan 01 | Duration parsing supports human-friendly format (2h, 1d, 1w) with calendar time conventions | SATISFIED | `internal/duration.Parse` returns `time.Duration`; 1d=24h, 1w=168h confirmed in code and 20 passing tests |
| UTIL-03 | Plan 02 | Preset resolution follows three-tier lookup: profile > user file > built-in | SATISFIED | `internal/preset.Lookup` implements three-tier chain; `cmd/root.go` wired to use it; 18 tests cover all resolution paths |

All 3 requirements satisfied. No orphaned requirements detected.

---

### Anti-Patterns Found

None. No TODO/FIXME/PLACEHOLDER comments, no stub implementations, no empty returns, no inline `SetEscapeHTML(false)` outside the `jsonutil` package.

---

### Human Verification Required

None required. All observable truths are verifiable programmatically through source code inspection and `go test` results.

---

### Gaps Summary

No gaps. All 13 observable truths verified, all artifacts exist with substantive implementations, all key links confirmed wired, all 3 requirements satisfied, full test suite passes (`go test ./... -count=1` — 15 packages, 0 failures), `go build ./...` and `go vet ./...` both exit 0.

---

_Verified: 2026-03-28T14:30:00Z_
_Verifier: Claude (gsd-verifier)_
