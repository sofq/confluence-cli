# Phase 15: Workflow Commands - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 15-workflow-commands
**Areas discussed:** API strategy, Async operation handling, Comment convenience model, Restriction API approach, Copy option flags
**Mode:** Auto (--auto flag — all areas auto-selected, recommended defaults chosen)

---

## API Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| v2 where available, v1 for gaps | Use v2 API for move/publish/archive/comment, v1 for copy/restrict where no v2 endpoint exists | :heavy_check_mark: |
| v2 only (skip operations without v2 support) | Limit to only v2 API operations, skip copy and restrict | |
| v1 only for all workflow operations | Use v1 content API for consistency across all operations | |

**User's choice:** v2 where available, v1 for gaps (auto-selected recommended default)
**Notes:** Consistent with project constraint (v2 primary, v1 for gaps). Move/publish/archive have v2 equivalents via page update/archive endpoint. Copy and restrict require v1 content API.

---

## Async Operation Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Block and poll by default with --no-wait flag | Wait for completion by default, offer --no-wait for immediate return | :heavy_check_mark: |
| Always async with task ID return | Never block, always return task ID | |
| Configurable default via config | Let user set default behavior in config | |

**User's choice:** Block and poll by default with --no-wait flag (auto-selected recommended default)
**Notes:** Agents typically want the final result. --no-wait provides escape hatch for agents that manage their own polling.

---

## Comment Convenience Model

| Option | Description | Selected |
|--------|-------------|----------|
| Plain text input with auto-conversion to storage format | Accept --body as plain text, wrap in <p> tags | :heavy_check_mark: |
| Raw storage format input | Require --body as Confluence storage format XHTML | |
| Both modes with --raw flag | Default to plain text, --raw flag for storage format input | |

**User's choice:** Plain text input with auto-conversion to storage format (auto-selected recommended default)
**Notes:** Convenience is the purpose of workflow commands. Agents needing raw format use `cf comments create` directly.

---

## Restriction API Approach

| Option | Description | Selected |
|--------|-------------|----------|
| v1 restrictions API with operation-based flags | --add/--remove flags with --operation read|update and --user/--group | :heavy_check_mark: |
| Simplified allow/deny model | Abstract restrictions to simpler allow/deny permissions | |
| JSON body input for full control | Accept restrictions as raw JSON body | |

**User's choice:** v1 restrictions API with operation-based flags (auto-selected recommended default)
**Notes:** Maps directly to Confluence's restriction model. View mode (no flags) shows current restrictions. Explicit --add/--remove with --operation for modifications.

---

## Copy Option Flags

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror Confluence UI options as boolean flags | --copy-attachments, --copy-labels, --copy-permissions as booleans | :heavy_check_mark: |
| Single --include flag with comma values | --include attachments,labels,permissions | |
| Copy everything by default with --skip flags | --skip-attachments, --skip-labels, --skip-permissions | |

**User's choice:** Mirror Confluence UI options as boolean flags (auto-selected recommended default)
**Notes:** Explicit opt-in per copy option. Default false for all — safe defaults, agents specify what they want.

---

## Claude's Discretion

- Internal code organization (single workflow.go file or split per subcommand)
- Exact v1 API request/response shapes (validated during research)
- Whether move is truly async or synchronous via v2 PUT
- Test case selection and organization
- Error message wording

## Deferred Ideas

- WKFL-07 (restore previous version) — future milestone
- WKFL-08 (bulk move) — future milestone
