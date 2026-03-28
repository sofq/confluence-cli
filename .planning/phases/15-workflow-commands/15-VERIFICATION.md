---
phase: 15-workflow-commands
verified: 2026-03-28T16:30:00Z
status: passed
score: 7/7 must-haves verified
re_verification: false
---

# Phase 15: Workflow Commands Verification Report

**Phase Goal:** Users can perform content lifecycle operations (move, copy, publish, comment, restrict, archive) through dedicated workflow subcommands.
**Verified:** 2026-03-28T16:30:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths (from PLAN 15-01 must_haves)

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | `cf workflow move --id X --target-id Y` calls v1 move endpoint and outputs JSON response | VERIFIED | `runWorkflowMove` calls `fetchV1WithBody(cmd, c, "PUT", domain+"/wiki/rest/api/content/{id}/move/append/{targetId}", nil)`; `TestWorkflow_Move_Success` passes confirming PUT method + correct path + JSON stdout |
| 2 | `cf workflow copy --id X --target-id Y` calls v1 copy endpoint with copy flags and polls long task | VERIFIED | `runWorkflowCopy` posts to `/wiki/rest/api/content/{id}/copy` with `copyRequestBody` struct; `--no-wait` path returns immediately; polling via `pollLongTask`; `TestWorkflow_Copy_NoWait` passes |
| 3 | `cf workflow publish --id X` fetches current page, bumps version, PUTs status=current via v2 | VERIFIED | `runWorkflowPublish` calls `c.Fetch GET /pages/{id}`, increments `version.Number+1`, PUTs with `status:"current"`; `TestWorkflow_Publish_Success` verifies version=2 and status="current" in PUT body |
| 4 | `cf workflow comment --id X --body text` wraps text in `<p>` tags and POSTs to `/footer-comments` via v2 | VERIFIED | `storageBody := "<p>" + bodyText + "</p>"` then `c.Fetch(ctx, "POST", "/footer-comments", ...)` using `createCommentBody`; `TestWorkflow_Comment_Success` verifies path, pageId, and `<p>Hello World</p>` body value |
| 5 | `cf workflow restrict --id X` GETs current restrictions from v1 API | VERIFIED | View mode (no --add/--remove) calls `fetchV1WithBody(... "GET", domain+"/wiki/rest/api/content/{id}/restriction", nil)`; `TestWorkflow_Restrict_View` confirms GET to correct path |
| 6 | `cf workflow restrict --id X --add --operation read --user U` PUTs individual restriction via v1 | VERIFIED | Add mode builds URL `/wiki/rest/api/content/{id}/restriction/byOperation/{op}/user?accountId={user}` and calls `fetchV1WithBody(... http.MethodPut, ...)`; `TestWorkflow_Restrict_AddUser` confirms PUT + accountId query param + "added" stdout |
| 7 | `cf workflow archive --id X` POSTs to v1 content/archive endpoint | VERIFIED | `runWorkflowArchive` marshals `archiveRequest{Pages: []archivePage{{ID: id}}}` and POSTs to `/wiki/rest/api/content/archive`; `TestWorkflow_Archive_Success` confirms POST method, path, and `pages:[{id:"123"}]` body shape |

**Score: 7/7 truths verified**

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/workflow.go` | workflowCmd parent + move/copy/publish/comment/restrict/archive subcommands + pollLongTask helper; min 300 lines | VERIFIED | 598 lines; all 6 subcommands present; `pollLongTask` implemented; `var workflowCmd`, all 6 `var workflow_*` command vars confirmed |
| `cmd/root.go` | `rootCmd.AddCommand(workflowCmd)` registration | VERIFIED | Line 302: `rootCmd.AddCommand(workflowCmd)  // Phase 15: workflow lifecycle commands` |
| `cmd/workflow_test.go` | Tests for all six workflow subcommands; min 200 lines | VERIFIED | 663 lines; 22 tests — 13 validation + 8 integration + 1 extra group-add test |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/workflow.go` | `internal/client` | `client.FromContext`, `client.SearchV1Domain`, `c.Fetch`, `c.WriteOutput` | WIRED | 26 usages confirmed across all 6 subcommands and pollLongTask |
| `cmd/workflow.go` | `cmd/labels.go` | `fetchV1WithBody` (same package) | WIRED | Called 7 times: move, copy, restrict (view/add/remove/group), archive, pollLongTask |
| `cmd/workflow.go` | `internal/duration` | `duration.Parse` for `--timeout` flag | WIRED | Lines 172, 493 — used in both copy and archive async paths |
| `cmd/root.go` | `cmd/workflow.go` | `rootCmd.AddCommand(workflowCmd)` | WIRED | Line 302 of root.go; `go run . workflow --help` confirms all 6 subcommands listed |
| `cmd/workflow_test.go` | `cmd/workflow.go` | `root.SetArgs([]string{"workflow"}, ...)` via `cmd.RootCommand().Execute()` | WIRED | Line 34; all 22 tests exercise commands through root execution |
| `cmd/workflow_test.go` | `cmd/templates_test.go` | `setupTemplateEnv` test helper | WIRED | Line 21; reused in every test via `runWorkflowCommand` helper |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| WKFL-01 | 15-01, 15-02 | User can move a page to a different parent or space via `workflow move` | SATISFIED | `runWorkflowMove` + `TestWorkflow_Move_Success` — PUT to v1 move endpoint confirmed |
| WKFL-02 | 15-01, 15-02 | User can copy a page with options (attachments, permissions, labels) via `workflow copy` | SATISFIED | `runWorkflowCopy` + `TestWorkflow_Copy_NoWait` — POST with copyAttachments/copyLabels flags in body confirmed |
| WKFL-03 | 15-01, 15-02 | User can publish a draft page via `workflow publish` | SATISFIED | `runWorkflowPublish` GET + PUT with status="current" + `TestWorkflow_Publish_Success` confirms version bump |
| WKFL-04 | 15-01, 15-02 | User can add a plain-text comment to a page via `workflow comment` | SATISFIED | `runWorkflowComment` wraps text in `<p>` tags, POSTs to `/footer-comments`; `TestWorkflow_Comment_Success` verifies body structure |
| WKFL-05 | 15-01, 15-02 | User can view, add, and remove page restrictions via `workflow restrict` | SATISFIED | Three-mode restrict: view (GET), add (PUT), remove (DELETE); 6 validation tests + 3 integration tests confirm all modes |
| WKFL-06 | 15-01, 15-02 | User can archive pages via `workflow archive` | SATISFIED | `runWorkflowArchive` POSTs `{pages:[{id}]}` to v1 archive endpoint; `TestWorkflow_Archive_Success` confirms body shape |

No orphaned requirements. All 6 WKFL-* requirements claimed in both plan frontmatters are fully implemented and tested.

### Anti-Patterns Found

None. Scan of `cmd/workflow.go` and `cmd/workflow_test.go` found no TODO/FIXME/placeholder comments, no empty implementations, no stub returns.

### Human Verification Required

None. All observable behaviors are verified programmatically through:
- `go build ./...` — confirms no compilation errors
- `go run . workflow --help` — confirms all 6 subcommands listed with correct short descriptions
- `go test ./cmd/ -run TestWorkflow -count=1` — 22/22 tests pass
- `go test ./cmd/ -count=1` — full test suite passes (no regressions)
- `go vet ./cmd/` — no vet warnings

---

## Summary

Phase 15 goal is fully achieved. All six workflow subcommands (`move`, `copy`, `publish`, `comment`, `restrict`, `archive`) exist in `cmd/workflow.go` (598 lines), are registered in `cmd/root.go`, and are covered by 22 passing tests in `cmd/workflow_test.go` (663 lines).

Key implementation details confirmed by code inspection:
- `move`: PUT to v1 `/wiki/rest/api/content/{id}/move/append/{targetId}`
- `copy`: POST to v1 `/wiki/rest/api/content/{id}/copy` with async poll via `pollLongTask`
- `publish`: v2 GET page + v2 PUT with `status:"current"` and incremented version
- `comment`: v2 POST to `/footer-comments` with `<p>`-wrapped body in storage representation
- `restrict`: three-mode v1 command — GET `/restriction`, PUT/DELETE `/restriction/byOperation/{op}/user` and `/byGroupId/{group}`
- `archive`: POST to v1 `/wiki/rest/api/content/archive` with `{pages:[{id}]}` body, async poll via `pollLongTask`

All WKFL-01 through WKFL-06 requirements satisfied. Project compiles cleanly. No regressions in existing test suite.

---

_Verified: 2026-03-28T16:30:00Z_
_Verifier: Claude (gsd-verifier)_
