---
phase: 05-avatar-analysis
plan: "01"
subsystem: avatar
tags: [avatar, analysis, writing-profile, tdd]
dependency_graph:
  requires: []
  provides:
    - internal/avatar package with types, fetch, analyze, build
  affects:
    - cmd/avatar.go (Phase 5 Plan 02 — will import this package)
tech_stack:
  added: []
  patterns:
    - TDD red-green workflow for all new functions
    - Direct net/http + c.ApplyAuth() for v1 API calls (no c.Fetch() to avoid URL doubling)
    - Package-level compiled regexes for performance
    - text/template for prose generation
key_files:
  created:
    - internal/avatar/types.go
    - internal/avatar/fetch.go
    - internal/avatar/fetch_test.go
    - internal/avatar/analyze.go
    - internal/avatar/analyze_test.go
    - internal/avatar/build.go
    - internal/avatar/build_test.go
  modified: []
decisions:
  - StripStorageHTML uses regexp + html.UnescapeString (no external HTML parser) for minimal dependencies
  - FetchUserPages uses v1 content API (/wiki/rest/api/content) with CQL, not v2 API
  - Length thresholds page-appropriate: short <= 100 words, long >= 500 words (vs 20/80 for comments in jira-cli)
  - StructurePatterns threshold at >20% of pages (not count-based) to normalize across corpus sizes
  - BuildProfile selects examples by longest pages (most representative content)
metrics:
  duration: 4 minutes
  completed_date: "2026-03-20"
  tasks_completed: 2
  files_created: 7
---

# Phase 05 Plan 01: internal/avatar package — types, fetch, analyze, build Summary

**One-liner:** Confluence writing-style analysis engine with CQL page fetching, HTML stripping, statistical analysis, and template-based profile generation.

## What Was Built

The complete `internal/avatar/` package providing all types and logic for Phase 5 avatar analysis:

- **types.go** — `PersonaProfile`, `WritingAnalysis`, `PageRecord`, and all sub-types (`LengthDist`, `FormattingStats`, `VocabularyStats`, `ToneSignals`, `StyleGuide`, `PageExample`)
- **fetch.go** — `FetchUserPages` (CQL search via Confluence v1 content API, paginated up to 200 pages) and `StripStorageHTML` (regex tag stripping + HTML entity decoding + whitespace collapse)
- **analyze.go** — `AnalyzeWriting` (word count statistics, length distribution, formatting ratios, tone signals via sentence splitting, vocabulary n-grams, structure pattern detection)
- **build.go** — `BuildProfile` (composes PersonaProfile from pages, generates `StyleGuide.Writing` via `text/template`, selects top-3 examples trimmed to 300 chars)

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Define types and implement StripStorageHTML + FetchUserPages | f046d9e | types.go, fetch.go, fetch_test.go |
| 2 | Implement AnalyzeWriting and BuildProfile | 0b123a8 | analyze.go, analyze_test.go, build.go, build_test.go |

## Test Coverage

- `TestStripStorageHTML`: 6 subtests (empty, simple HTML, entities, CDATA, whitespace, nested tags)
- `TestFetchUserPages_HappyPath`: 2 pages, verifies ID/Title/Body/LastModified parsing and CQL format
- `TestFetchUserPages_EmptyResults`: verifies empty slice + nil error
- `TestFetchUserPages_HTTP401`: verifies nil records + non-nil error on 401
- `TestAnalyzeWriting_*`: 9 cases covering nil, single text, bullets ratio, headings, code blocks, first person, common phrases, length distribution, structure patterns, tables
- `TestBuildProfile_*`: 5 cases covering 0 pages, 1 page, 5 pages, JSON marshallability, example trimming

All 19+ test cases pass. `go vet ./internal/avatar/...` clean. `go build ./...` passes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test LengthDist threshold value**
- **Found during:** Task 2 GREEN phase
- **Issue:** Test used 60-word "medium" text, but 60 words <= 100-word threshold so it landed in "short" bucket
- **Fix:** Changed test medium text from 60 words to 150 words (correctly falls in 101-499 medium range)
- **Files modified:** internal/avatar/analyze_test.go
- **Commit:** 0b123a8

## Self-Check: PASSED

Files created:
- FOUND: internal/avatar/types.go
- FOUND: internal/avatar/fetch.go
- FOUND: internal/avatar/fetch_test.go
- FOUND: internal/avatar/analyze.go
- FOUND: internal/avatar/analyze_test.go
- FOUND: internal/avatar/build.go
- FOUND: internal/avatar/build_test.go

Commits verified:
- FOUND: f046d9e
- FOUND: 0b123a8
