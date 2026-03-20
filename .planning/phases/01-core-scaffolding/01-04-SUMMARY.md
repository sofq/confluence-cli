---
phase: 01-core-scaffolding
plan: "04"
subsystem: testing
tags: [tests, tdd, go-test, internal, client, cmd]
dependency_graph:
  requires: [01-01, 01-02, 01-03]
  provides: [automated-verification-baseline, INFRA-coverage]
  affects: [all-future-phases]
tech_stack:
  added: []
  patterns: [table-driven-tests, httptest-server, t-setenv, t-tempdir, external-test-package]
key_files:
  created:
    - internal/errors/errors_test.go
    - internal/config/config_test.go
    - internal/jq/jq_test.go
    - internal/cache/cache_test.go
    - internal/client/client_test.go
    - cmd/root_test.go
    - cmd/configure_test.go
    - cmd/raw_test.go
    - cmd/schema_cmd_test.go
  modified: []
decisions:
  - "External test packages (_test suffix) used for all test files to prevent testing internals and ensure public API coverage"
  - "Cache tests use unique URL-based keys (incorporating t.Name()) to avoid cross-test cache pollution from sync.Once Dir()"
  - "Cobra command state between tests handled by passing explicit --profile flags rather than relying on flag defaults"
  - "os.Pipe() used to capture stdout/stderr for cmd-level tests since cobra writes to os.Stdout/os.Stderr directly"
metrics:
  duration: "5 minutes"
  completed_date: "2026-03-20"
  tasks_completed: 2
  files_created: 9
---

# Phase 01 Plan 04: Test Suite Summary

**One-liner:** Nine test files covering all Phase 1 packages with httptest servers, TDD patterns, and race-detector-clean execution.

## What Was Built

Full automated test suite for Phase 1, establishing the verification baseline required by all 13 INFRA requirements. Tests run GREEN with no data races.

### Task 1: Internal Package Tests

**internal/errors/errors_test.go** — 6 test functions:
- `TestExitCodeFromStatus`: 13 cases covering all HTTP status → exit code mappings (401→2, 403→2, 404→3, 422→4, 429→5, 409→6, 500→7, 200→0)
- `TestExitCodeConstants`: validates all 8 exit code constant values
- `TestNewFromHTTP`: 5 subtests covering error type, HTML sanitization, Retry-After header parsing, nil resp
- `TestAlreadyWrittenError`: sentinel error string and Code field
- `TestAPIErrorWriteJSON`: valid JSON with `error_type` key
- `TestAPIErrorExitCode`: exit code delegation from status

**internal/config/config_test.go** — 8 test functions:
- `TestDefaultPath`: CF_CONFIG_PATH env var, contains "cf" not "jr"
- `TestLoadFromNonExistent`: returns empty Config, nil error
- `TestSaveAndLoadRoundtrip`: full roundtrip preserves all fields
- `TestResolveWithEnvBaseURL`: CF_BASE_URL env var applied
- `TestCFProfile`: CF_PROFILE selects staging, --profile flag overrides CF_PROFILE
- `TestResolveNonExistentExplicitProfile`: explicit missing profile returns error
- `TestResolveFlagsPriority`: flags > env > file priority order verified
- `TestResolveTrimsTrailingSlash`: trailing slashes stripped from BaseURL
- `TestResolveEmptyBaseURL`: missing config returns empty BaseURL (not error)

**internal/jq/jq_test.go** — 1 test function with 6 subtests:
- `TestApply`: simple field, array iteration, empty passthrough, invalid filter error, invalid JSON error, nested field, multi-result array

**internal/cache/cache_test.go** — 4 test functions:
- `TestKeyUniqueness`: different auth contexts, methods, URLs all produce unique keys; hex encoding verified
- `TestGetSetRoundtrip`: Set → Get with 5min TTL returns original data
- `TestGetExpiredTTL`: negative TTL returns nil, false
- `TestGetNonExistent`: non-existent key returns nil, false

### Task 2: Client and Command Tests

**internal/client/client_test.go** — 8 test functions:
- `TestApplyAuthBasic`: base64 decodes to user:token format
- `TestApplyAuthBearer`: sets Authorization: Bearer <token>
- `TestDryRun`: server not called, JSON output with method+url
- `TestVerboseLogFalse`: nothing written to stderr
- `TestVerboseLogTrue`: valid JSON written to stderr
- `TestWriteOutputWithJQFilter`: `.id` on `{"id":42}` outputs `42`
- `TestWriteOutputWithInvalidJQFilter`: returns ExitValidation
- `TestCursorPagination`: httptest server serves 2 pages via `_links.next`, merged output contains id:1 and id:2, 2 HTTP requests made
- `TestCacheResponse`: second identical GET hits cache (only 1 HTTP request)
- `TestDoHTTPErrorReturnsExitCode`: 5 subtests for 401→2, 404→3, 422→4, 429→5, 500→7

**cmd/root_test.go** — 4 test functions:
- `TestVersionFlagOutputsJSON`: `--version` writes valid JSON with `version` key to stdout
- `TestVersionSubcommandOutputsJSON`: `version` subcommand writes valid JSON
- `TestRootHelpOutputsJSON`: help output is valid JSON when written to stdout
- `TestExecuteNoConfigReturnsNonZero`: no CF_BASE_URL + no config returns non-zero

**cmd/configure_test.go** — 5 test functions:
- `TestConfigureSavesProfile`: writes bearer profile to CF_CONFIG_PATH temp file
- `TestConfigureStripTrailingSlash`: trailing slashes stripped from stored BaseURL
- `TestConfigureEmptyBaseURLReturnsValidationError`: validation error for empty --base-url
- `TestConfigureDeleteWithoutProfileReturnsValidationError`: --delete without --profile errors
- `TestConfigureDeleteNonExistentProfileReturnsNotFound`: not_found error for unknown profile

**cmd/raw_test.go** — 4 test functions:
- `TestRawInvalidMethodReturnsValidationError`: "FOO" method yields validation_error JSON
- `TestRawGETCallsServer`: successful GET returns valid JSON from test server
- `TestRawPOSTWithoutBodyReturnsValidationError`: POST without --body in non-dry-run errors
- `TestRawGETWithQueryParams`: --query limit=5 passes to server

**cmd/schema_cmd_test.go** — 4 test functions:
- `TestSchemaListReturnsJSONArray`: --list returns parseable JSON array
- `TestSchemaNoArgsReturnsValidJSON`: no args returns valid JSON
- `TestSchemaOutputToStdout`: stdout has content, stderr is empty
- `TestSchemaCompactReturnsJSONObject`: --compact returns valid JSON object

## Verification Results

```
go test ./... -count=1          → ALL PASS (7 packages with tests)
go test ./... -count=1 -race    → ALL PASS (no data races)
go test ./internal/... -v       → 30 PASS cases
```

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written with these minor adaptations:

1. **Cache test strategy**: Tests call `cache.Key()`, `cache.Set()`, `cache.Get()` directly with unique URL-based keys (incorporating `t.Name()`) rather than overriding `Dir()` via `sync.Once` reset. This avoids the once-initialized cache dir while still achieving full coverage.

2. **Cobra test state isolation**: `--profile` flag explicitly passed in all configure tests to prevent Cobra's retained flag state from causing cross-test interference after the first test sets a non-default profile.

## INFRA Requirements Coverage

| Requirement | Test(s) |
|-------------|---------|
| INFRA-01 (JSON-only stdout) | TestVersionFlagOutputsJSON, TestSchemaNoArgsReturnsValidJSON |
| INFRA-02 (Semantic exit codes) | TestExitCodeFromStatus, TestDoHTTPErrorReturnsExitCode |
| INFRA-03 (JQ filtering) | TestWriteOutputWithJQFilter, TestWriteOutputWithInvalidJQFilter |
| INFRA-04 (Config file) | TestSaveAndLoadRoundtrip, TestConfigureSavesProfile |
| INFRA-05 (Auth: basic/bearer) | TestApplyAuthBasic, TestApplyAuthBearer |
| INFRA-06 (Env var overrides) | TestResolveWithEnvBaseURL, TestCFProfile, TestResolveFlagsPriority |
| INFRA-07 (CF_PROFILE) | TestCFProfile |
| INFRA-08 (Cursor pagination) | TestCursorPagination |
| INFRA-09 (Cache) | TestCacheResponse, TestGetSetRoundtrip, TestGetExpiredTTL |
| INFRA-10 (Dry-run) | TestDryRun |
| INFRA-11 (Verbose logging) | TestVerboseLogTrue, TestVerboseLogFalse |
| INFRA-12 (schema command) | TestSchemaListReturnsJSONArray, TestSchemaCompactReturnsJSONObject |
| INFRA-13 (configure command) | TestConfigureSavesProfile, TestConfigureDeleteNonExistentProfileReturnsNotFound |

## Self-Check: PASSED

Files created:
- FOUND: internal/errors/errors_test.go
- FOUND: internal/config/config_test.go
- FOUND: internal/jq/jq_test.go
- FOUND: internal/cache/cache_test.go
- FOUND: internal/client/client_test.go
- FOUND: cmd/root_test.go
- FOUND: cmd/configure_test.go
- FOUND: cmd/raw_test.go
- FOUND: cmd/schema_cmd_test.go

Commits:
- FOUND: 4627b05 (Task 1: internal package tests)
- FOUND: 4fe0ba7 (Task 2: client and command tests)
