---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: Workflow, Parity & Release Infrastructure
status: completed
stopped_at: Completed 17-04-PLAN.md
last_updated: "2026-03-28T17:50:00.651Z"
last_activity: 2026-03-28
progress:
  total_phases: 7
  completed_phases: 6
  total_plans: 16
  completed_plans: 16
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-28)

**Core value:** Give AI agents reliable, structured JSON access to Confluence content through a CLI
**Current focus:** Phase 16 — schema-gendocs

## Current Position

Phase: 18
Plan: Not started
Status: Phase 16 complete
Last activity: 2026-03-28

Progress: [██████████] 100% (2/2 plans in phase 16)

## Performance Metrics

**Velocity:**

- Total plans completed: 24 (v1.0: 16, v1.1: 8)
- Average duration: ~5min
- Total execution time: ~2 hours

**By Phase (v1.1):**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 06-oauth2 P01 | 1 | 6min | 6min |
| 06-oauth2 P02 | 1 | 4min | 4min |
| 07-blog-posts P01 | 1 | 3min | 3min |
| 08-attachments P01 | 1 | 3min | 3min |
| 09-custom-content P01 | 1 | 3min | 3min |
| 10-output-presets P01 | 1 | 3min | 3min |
| 10-output-presets P02 | 1 | 7min | 7min |
| 11-watch P01 | 1 | 5min | 5min |

**Recent Trend:**

- Last 5 plans: 3m, 3m, 3m, 7m, 5m
- Trend: Stable

| Phase 12 P01 | 2min | 2 tasks | 4 files |
| Phase 12 P02 | 3min | 2 tasks | 3 files |
| Phase 12 P03 | 5min | 2 tasks | 9 files |
| Phase 13 P01 | 4min | 2 tasks | 6 files |
| Phase 13 P02 | 3min | 2 tasks | 2 files |
| Phase 13 P03 | 3min | 2 tasks | 3 files |
| Phase 14-version-diff P01 | 3min | 1 tasks | 2 files |
| Phase 14-version-diff P02 | 9min | 2 tasks | 3 files |
| Phase 15-workflow-commands P01 | 2min | 2 tasks | 2 files |
| Phase 15-workflow-commands P02 | 2min | 1 tasks | 1 files |
| Phase 16-schema-gendocs P01 | 3min | 2 tasks | 8 files |
| Phase 16-schema-gendocs P02 | 2min | 2 tasks | 2 files |
| Phase 17-02 P02 | 2min | 2 tasks | 5 files |
| Phase 17-04 P04 | 2min | 1 tasks | 1 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [v1.2 roadmap]: Phases 13/14/15 can parallelize after Phase 12 (no mutual dependency)
- [v1.2 research]: v2 historical body retrieval needs live API validation (Phase 14)
- [v1.2 research]: Move endpoint async behavior needs live testing (Phase 15)
- [v1.2 research]: npm/PyPI first-publish must be manual before OIDC workflows work
- [v1.1]: Zero new Go dependencies -- all features use stdlib only
- [v1.1]: OAuth2 token in PersistentPreRunE -- client stays stateless
- [v1.1]: map[string]string for template data -- prevents SSTI
- [Phase 12-01]: Calendar time conventions for duration: 1d=24h, 1w=168h (not Jira work-time)
- [Phase 12-01]: NewEncoder added beyond jr pattern for streaming use cases (errors.go, watch.go)
- [Phase 12-02]: Import alias preset_pkg for preset package in cmd/root.go (local var preset conflicts with package name)
- [Phase 12]: Removed unused encoding/json import from errors.go, bytes from jq.go, both from root.go after jsonutil consolidation
- [Phase 13]: Built-in templates in separate builtin.go file (keeps template.go clean)
- [Phase 13]: User templates override built-in for same name; Save() rejects overwrite
- [Phase 13]: Manual client construction in templates create rather than removing templates from skipClientCommands
- [Phase 13]: Body field as json.RawMessage preserves full API response body including format metadata
- [Phase 14-version-diff]: ParseSince tries ISO date formats before duration.Parse (pitfall 6 avoidance)
- [Phase 14-version-diff]: LineStats uses frequency-map comparison per D-04, not Myers/LCS
- [Phase 14-version-diff]: --since and --from/--to mutually exclusive (validation error)
- [Phase 14-version-diff]: Pre-filter versions by --since cutoff before fetching bodies (avoids unnecessary API calls)
- [Phase 14-version-diff]: Cobra flag reset in test helper for singleton command state isolation
- [Phase 15-workflow-commands]: v1 move endpoint (PUT /content/{id}/move/append/{targetId}) over v2 PUT parentId -- reliable dedicated endpoint
- [Phase 15-workflow-commands]: v1 archive endpoint (POST /content/archive) used -- no v2 equivalent exists
- [Phase 15-workflow-commands]: pollLongTask returns raw body on unmarshal failure -- graceful degradation
- [Phase 15-workflow-commands]: Reused setupTemplateEnv and dummyServer from existing test files; created resetWorkflowFlags for Cobra singleton isolation
- [Phase 16-schema-gendocs]: Per-resource *_schema.go files following jr pattern for hand-written schema op separation
- [Phase 16-schema-gendocs]: Flag types match init() declarations: Int as integer, Bool as boolean
- [Phase 16-schema-gendocs]: Used --output flag instead of positional arg for gendocs CLI
- [Phase 17-02]: Adapted jr reference patterns exactly -- same download/extract logic, platform maps, archive naming
- [Phase 17-02]: npm version 0.1.0 per D-07, Python version 0.0.0 (release workflow sets via sed)
- [Phase 17-04]: Mirrored jr README structure exactly per D-09/D-10 with 12 cf feature showcase sections

### Pending Todos

None yet.

### Blockers/Concerns

- npm OIDC first-publish: manual step required before Phase 17 release workflows work end-to-end
- v2 historical version body retrieval may need v1 fallback (validate in Phase 14 planning)
- Move endpoint async behavior unclear (validate in Phase 15 planning)
- Atlassian rate limit point costs per endpoint not published -- watch interval needs empirical validation

## Session Continuity

Last session: 2026-03-28T17:44:25.035Z
Stopped at: Completed 17-04-PLAN.md
Resume file: None
