---
phase: 18-documentation-site
plan: 02
subsystem: docs
tags: [vitepress, documentation, guides, markdown, confluence]

# Dependency graph
requires:
  - phase: 18-documentation-site/01
    provides: VitePress site scaffold and configuration
provides:
  - 4 core guide pages: getting-started, filtering, discovery, templates
  - User-facing install/config/first-commands documentation
  - Template system documentation with variable reference
affects: [18-documentation-site/03, website]

# Tech tracking
tech-stack:
  added: []
  patterns: [vitepress-guide-pages, code-group-install-tabs, v-pre-go-template-guard]

key-files:
  created:
    - website/guide/getting-started.md
    - website/guide/filtering.md
    - website/guide/discovery.md
    - website/guide/templates.md
  modified: []

key-decisions:
  - "Adapted jr guide structure exactly, replacing all Jira-specific content with Confluence equivalents"
  - "Used 8,000 tokens (not 10,000) as baseline for Confluence page size in filtering guide"
  - "Included workflow commands section in getting-started to showcase cf-specific capabilities"

patterns-established:
  - "Guide page structure: title, intro, sections with code-group/code blocks, tips/info boxes, next-steps links"
  - "v-pre wrapper required for any Go template syntax ({{.variable}}) in VitePress markdown"

requirements-completed: [DOCS-04]

# Metrics
duration: 3min
completed: 2026-03-29
---

# Phase 18 Plan 02: Guide Pages Summary

**4 core guide pages (getting-started, filtering, discovery, templates) adapted from jr reference with Confluence-specific content, cf commands, CQL queries, and storage format references**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-28T18:22:26Z
- **Completed:** 2026-03-28T18:25:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Getting-started guide with 6 install methods, 3 auth types, env vars, named profiles, security settings, first commands, and workflow section
- Filtering guide covering --preset (7 presets), --fields, --jq with before/after token comparison and cache tip
- Discovery guide documenting 4 cf schema modes with 242-command stat and agent tip
- Templates guide with 6 built-in templates table, variable usage, from-page creation, YAML format with v-pre guard, and batch usage

## Task Commits

Each task was committed atomically:

1. **Task 1: Create getting-started and filtering guide pages** - `c97235f` (feat)
2. **Task 2: Create discovery and templates guide pages** - `e5b2c55` (feat)

## Files Created/Modified
- `website/guide/getting-started.md` - Install, configure, first commands, workflow commands, next steps (223 lines)
- `website/guide/filtering.md` - Presets, fields, jq, combined filtering, cache (61 lines)
- `website/guide/discovery.md` - Four cf schema discovery modes (33 lines)
- `website/guide/templates.md` - Built-in templates, variables, creation, file format, batch (157 lines)

## Decisions Made
- Adapted jr guide structure exactly, replacing all Jira-specific content with Confluence equivalents
- Used 8,000 tokens as Confluence page baseline (vs 10,000 for Jira issues) in filtering examples
- Included full workflow commands section in getting-started with move, copy, publish, comment, archive, restrict
- Templates guide uses Confluence-specific built-in templates (meeting-notes, decision, runbook, retrospective, adr, blank) instead of jr issue templates

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 4 guide pages ready for VitePress sidebar integration
- Cross-links between pages established (getting-started links to filtering, discovery, templates)
- Ready for plan 03 (remaining guide pages: global-flags, agent-integration, and any others)

## Self-Check: PASSED

All 4 guide files exist. Both task commits verified. SUMMARY.md created.

---
*Phase: 18-documentation-site*
*Completed: 2026-03-29*
