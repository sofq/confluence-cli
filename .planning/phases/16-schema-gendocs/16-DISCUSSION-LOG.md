# Phase 16: Schema + Gendocs - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 16-schema-gendocs
**Areas discussed:** Schema registration pattern, Schema aggregation, Gendocs binary scope, Command coverage
**Mode:** Auto (--auto flag — all areas auto-selected, recommended defaults chosen)

---

## Schema Registration Pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Individual *_schema.go files per command group | Separate files mirroring jr pattern: diff_schema.go, workflow_schema.go, etc. | :heavy_check_mark: |
| Single hand_written_schema.go file | All hand-written ops in one file | |
| Inline in each command file | Schema ops defined alongside command code | |

**User's choice:** Individual *_schema.go files per command group (auto-selected recommended default)
**Notes:** Matches jr architecture. Each file is self-contained and easy to maintain. 5 files covering 11 total ops.

---

## Schema Aggregation

| Option | Description | Selected |
|--------|-------------|----------|
| AllOps() helper appending hand-written to generated | schema_cmd.go calls generated.AllSchemaOps() + each *SchemaOps() function | :heavy_check_mark: |
| Registry pattern with init() auto-registration | Each schema file registers itself during init() | |
| Generated code includes hand-written ops | Modify code generator to embed hand-written ops | |

**User's choice:** AllOps() helper appending hand-written to generated (auto-selected recommended default)
**Notes:** Simplest approach. Explicit aggregation in schema_cmd.go — easy to see all sources at a glance.

---

## Gendocs Binary Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror jr's gendocs exactly | Standalone binary generating per-resource Markdown + sidebar.json + error codes | :heavy_check_mark: |
| Cobra doc generation plugin | Use built-in cobra/doc package | |
| Custom with extra features | Add usage examples, interactive API explorer | |

**User's choice:** Mirror jr's gendocs exactly (auto-selected recommended default)
**Notes:** Proven pattern. Generates VitePress-compatible output that Phase 18 (Documentation Site) will consume directly.

---

## Command Coverage

| Option | Description | Selected |
|--------|-------------|----------|
| All hand-written commands from Phases 13-15 | diff (1), workflow (6), export (1), preset (1), templates (2) = 11 ops | :heavy_check_mark: |
| Only new Phase 15 commands | Just workflow subcommands | |
| All commands including Phase 1-11 hand-written | Broader coverage including search, comments, labels, etc. | |

**User's choice:** All hand-written commands from Phases 13-15 (auto-selected recommended default)
**Notes:** Phases 1-11 hand-written commands (search, comments, labels, pages, spaces, etc.) already have generated schema entries from the OpenAPI spec. Only Phases 13-15 introduced pure hand-written commands without generated counterparts.

---

## Claude's Discretion

- Markdown template formatting within gendocs
- Test approach for schema files
- Sidebar structure (flat alphabetical vs grouped)
- Error codes table format

## Deferred Ideas

None — discussion stayed within phase scope
