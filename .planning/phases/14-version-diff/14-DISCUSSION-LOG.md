# Phase 14: Version Diff - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 14-version-diff
**Areas discussed:** Diff output structure, --since multi-version scope, Body retrieval strategy, Edge case behavior

---

## Diff Output Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Metadata + line stats | Version metadata for both versions + change statistics. No body content — agents use pages get-by-id --version N | ✓ |
| Metadata + line stats + diff hunks | Everything above PLUS a changes array with line-level diff hunks | |
| Full bodies + metadata | Both versions' full body content alongside metadata, no diff computation | |

**User's choice:** Metadata + line stats
**Notes:** Clean output, agents fetch full bodies separately if needed

### Follow-up: Line stats computation

| Option | Description | Selected |
|--------|-------------|----------|
| Split on newlines | Treat storage format as plain text, split on \n, simple line-level diff | ✓ |
| You decide | Claude picks based on API response structure | |

**User's choice:** Split on newlines
**Notes:** Fast, no dependencies, good enough for stats on XHTML

---

## --since Multi-Version Scope

| Option | Description | Selected |
|--------|-------------|----------|
| All pairwise diffs | Array of diffs between adjacent versions (v3→v4, v4→v5) | ✓ |
| First-to-last only | Single diff from oldest to newest version in range | |
| You decide | Claude picks based on API response and agent needs | |

**User's choice:** All pairwise diffs
**Notes:** Agents see full change timeline, mirrors jr's approach

### Follow-up: Output consistency

| Option | Description | Selected |
|--------|-------------|----------|
| Always diffs array | Consistent shape, even for single diff (single-element array) | ✓ |
| Flat for single, array for multi | Different shapes per case | |
| You decide | Claude picks based on --jq/--preset pipeline | |

**User's choice:** Always diffs array
**Notes:** Agents can always expect the same JSON structure

### Follow-up: --since format support

| Option | Description | Selected |
|--------|-------------|----------|
| Durations + ISO dates | Mirror jr: try ISO date first, fall back to duration.Parse() | ✓ |
| Durations only | Human-friendly durations only per DIFF-02 | |

**User's choice:** Durations + ISO dates
**Notes:** Mirrors jr's parseSince() pattern

---

## Body Retrieval Strategy

**[auto] Claude selected recommended approach**

| Option | Description | Selected |
|--------|-------------|----------|
| v2 API with version param | Use GET /pages/{id}?version=N&body-format=storage for historical bodies | ✓ |
| Metadata-only (no body) | Skip body retrieval entirely, no line stats | |
| v1 API fallback | Use v1 API if v2 doesn't return historical bodies | |

**User's choice:** [auto] v2 API with version param
**Notes:** Fall back to metadata-only if v2 doesn't return body (needs live validation). No v1 fallback — maintain v2-primary approach.

---

## Edge Case Behavior

**[auto] Claude selected recommended approach**

| Scenario | Behavior |
|----------|----------|
| Single version | from: null, stats show all lines as added |
| Empty --since range | diffs: [] empty array, not an error |
| --from equals --to | Zero stats, not an error |
| Page not found / no permission | Standard APIError JSON to stderr |

**User's choice:** [auto] All edge cases return structured JSON, never error on valid input
**Notes:** Consistent with existing error patterns across all cf commands

---

## Claude's Discretion

- Internal package structure (likely `internal/diff/`)
- Exact diff algorithm implementation
- Version metadata field names matching v2 API response
- Test case selection
- --from/--to vs --since mutual exclusivity

## Deferred Ideas

None — discussion stayed within phase scope
