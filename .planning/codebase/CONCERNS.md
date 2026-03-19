# Codebase Concerns

**Analysis Date:** 2026-03-20

## Tech Debt

**Context Window Management Complexity:**
- Issue: Context usage tracking relies on ephemeral temp files (`.tmp/claude-ctx-{session_id}.json`, `.tmp/claude-ctx-{session_id}-warned.json`) that exist only during sessions, with no persistent state. Metrics are written by statusline hook and read by context-monitor hook - a fragile file-based IPC mechanism.
- Files: `.claude/hooks/gsd-statusline.js` (lines 36-51), `.claude/hooks/gsd-context-monitor.js` (lines 62-76)
- Impact: Race conditions possible between hook executions; context warnings may fail silently on permission errors; no audit trail of context exhaustion events; switching sessions without cleanup risks stale warnings
- Fix approach: Replace temp file IPC with structured logging to `.planning/logs/context-usage.log`; implement session cleanup on start; add metrics persistence to `.planning/STATE.md`

**Hook Version Tracking:**
- Issue: Stale hook detection relies on string matching `// gsd-hook-version: X.Y.Z` in file headers. Version mismatch detection is best-effort with no automated update mechanism, only advisory warnings in statusline.
- Files: `.claude/hooks/gsd-check-update.js` (lines 73-90), `.claude/hooks/gsd-statusline.js` (lines 103-105)
- Impact: Users see warnings about stale hooks but have no automated fix path; mismatched hook versions may cause silent failures; update checks happen once per session with 10-second network timeout
- Fix approach: Implement `/gsd:update-hooks` command that auto-patches stale hooks; add pre-flight version check at agent spawn time; move hook version checking to gsd-tools.cjs

**Silent Failure Recovery:**
- Issue: All three hooks use try-catch patterns with `process.exit(0)` to silently fail (e.g., lines 152-155 in gsd-context-monitor.js, lines 116-118 in gsd-statusline.js). Failures are never logged, making debugging hook failures impossible.
- Files: `.claude/hooks/gsd-check-update.js` (line 65, 90), `.claude/hooks/gsd-context-monitor.js` (line 152-155), `.claude/hooks/gsd-statusline.js` (line 116-118)
- Impact: Hook failures are invisible; administrators have no way to diagnose which hook failed or why; ephemeral temp files may accumulate if cleanup fails
- Fix approach: Add optional debug logging to `.planning/logs/hook-errors.log` (enabled via `.planning/config.json` setting); expose hook errors in statusline as diagnostic warnings

**Unbounded Agent File Growth:**
- Issue: Each agent definition is large (16-43KB), and `gsd-executor.md` exceeds 19KB with extensive nested documentation. No size limits or documentation compression strategy exists.
- Files: `.claude/agents/gsd-executor.md` (19KB), `.claude/agents/gsd-planner.md` (43KB), `.claude/agents/gsd-plan-checker.md` (24KB)
- Impact: Large agent files consume context budget; loading multiple agents simultaneously risks context exhaustion; no agent modularization reduces reusability
- Fix approach: Modularize agent docs - extract shared patterns to `.claude/get-shit-done/references/` and `@`-reference them; split gsd-executor.md sections into separate doc files; implement agent template inheritance

## Known Bugs

**Context Monitor JSON Parse Silent Failure:**
- Symptoms: If `.tmp/claude-ctx-{session_id}.json` is corrupted or partially written, `JSON.parse()` at line 70 in gsd-context-monitor.js throws but is caught silently
- Files: `.claude/hooks/gsd-context-monitor.js` (lines 69-76)
- Trigger: Statusline writes metrics while context-monitor reads; race condition if both fire simultaneously
- Workaround: Restart Claude Code session to reset temp files

**Update Check npm Timeout Hanging:**
- Symptoms: `npm view get-shit-done-cc version` at line 95 in gsd-check-update.js sometimes hangs past 10-second timeout on slow networks
- Files: `.claude/hooks/gsd-check-update.js` (lines 93-96)
- Trigger: When user has poor network connectivity or npm registry is slow
- Workaround: Set `CLAUDE_CONFIG_DIR` to disable update checks for offline use

## Security Considerations

**Temp File Permissions in /tmp:**
- Risk: Session metrics and warning state written to `/tmp/claude-ctx-{session}.json` are world-readable if umask is permissive. Multi-user systems could leak context usage patterns.
- Files: `.claude/hooks/gsd-statusline.js` (line 47), `.claude/hooks/gsd-context-monitor.js` (line 117)
- Current mitigation: Temp files auto-delete at session end (if hook completes), but no explicit file mode is set
- Recommendations: Explicitly set 0600 file mode on temp files; consider using `~/.claude/runtime/` instead of `/tmp/`; document that shared machines should use private shell sessions

**Config File Injection in gsd-context-monitor:**
- Risk: `.planning/config.json` is read and parsed without validation. Malicious config could be injected to disable security warnings or context monitoring.
- Files: `.claude/hooks/gsd-context-monitor.js` (lines 50-60)
- Current mitigation: JSON parse errors are caught silently, preventing code execution
- Recommendations: Validate config schema before trusting `hooks.context_warnings`; log config changes to audit log; restrict write access to `.planning/config.json`

**Subprocess Spawning in gsd-check-update:**
- Risk: `child_process.spawn()` at line 45 in gsd-check-update.js executes user-controlled `cwd` paths. If working directory is untrusted, this could execute malicious code.
- Files: `.claude/hooks/gsd-check-update.js` (lines 45-107)
- Current mitigation: Spawned process only reads VERSION files and runs `npm view`, no shell interpretation
- Recommendations: Validate that `projectVersionFile` and `globalVersionFile` exist before reading; use `child_process.execFile()` instead of `spawn()` with `-e` code injection risk

## Performance Bottlenecks

**Synchronous File I/O in Hooks:**
- Problem: All hooks use synchronous `fs.readFileSync()`, `fs.writeFileSync()`, which block the main thread and delay tool execution display
- Files: `.claude/hooks/gsd-statusline.js` (multiple), `.claude/hooks/gsd-context-monitor.js` (multiple), `.claude/hooks/gsd-check-update.js` (lines 58-64)
- Cause: Hooks must complete before next tool can execute; blocking I/O is simplest but slowest
- Improvement path: Convert statusline hook to async using `fs.promises`; batch context metrics (write every 5 calls instead of every call); cache version file reads for session duration

**npm Update Check Every Session:**
- Problem: `npm view get-shit-done-cc version` makes a network call once per session, blocking for up to 10 seconds if npm is slow
- Files: `.claude/hooks/gsd-check-update.js` (line 95)
- Cause: Update check is synchronous and must complete before agent can be used
- Improvement path: Move update check to background after initial load; cache latest version for 24 hours (not 1 session); make statusline update check async so it doesn't block session start

**Config File Re-reads:**
- Problem: `.planning/config.json` is read every single time context-monitor hook runs (after every tool use), even if config hasn't changed
- Files: `.claude/hooks/gsd-context-monitor.js` (lines 50-60)
- Cause: No caching of config parsing results
- Improvement path: Cache config in memory with file mtime watch; only re-read if mtime changes; add config validation on first load with cached errors

## Fragile Areas

**Hook Version Header Regex:**
- Files: `.claude/hooks/gsd-check-update.js` (line 77)
- Why fragile: Regex `/\\/\\/ gsd-hook-version:\\s*(.+)/` expects exact format in file header. Manual edits or reformatting will break version detection. No test coverage for regex.
- Safe modification: Use dedicated `VERSION` marker file instead of string matching; parse hook metadata from JSON header
- Test coverage: No automated tests for hook version detection; no fixtures with malformed headers

**Temp File Race Conditions:**
- Files: `.claude/hooks/gsd-statusline.js` (writes), `.claude/hooks/gsd-context-monitor.js` (reads)
- Why fragile: Two separate hooks write/read same temp file without locking. High concurrency could cause partial writes to be read as valid JSON.
- Safe modification: Use atomic file operations (write to temp then rename); add file locking with `npm:proper-lockfile`; or consolidate to single source of truth
- Test coverage: No concurrency tests; no simulation of simultaneous hook execution

**Agent Documentation as Executable Context:**
- Files: All `.claude/agents/*.md` files are read as-is and provided to Claude models
- Why fragile: Agent docs contain inline code snippets, file paths, and instructions that must be syntactically correct. A typo in a file path or bash command will fail at execution time with no validation.
- Safe modification: Add linter that validates:
  - All `@file:` references exist
  - All bash command examples are syntactically valid
  - All file paths in docs match actual structure
- Test coverage: No automated validation of agent docs; breaking changes discovered only when agent is spawned

## Scaling Limits

**Session Metrics Accumulation:**
- Current capacity: Temp files are per-session, auto-delete on session end. Up to 3 temp files per active session.
- Limit: No explicit cleanup for orphaned temp files (e.g., killed processes). Over weeks, hundreds of stale files could accumulate in `/tmp/`.
- Scaling path: Implement `.planning/cleanup` command that removes temp files older than 7 days; add mtime check to context-monitor to skip stale metrics

**Agent Documentation Load:**
- Current capacity: All agent docs loaded into context at agent spawn time. 16 agents × avg 20KB = ~320KB of agent documentation.
- Limit: At 50-70% context usage, loading additional agents risks context exhaustion. No way to load only required agents.
- Scaling path: Implement selective agent loading (load only agents for current workflow); split agent docs into minimal spec + detailed references; use `@` syntax for external docs

**Hook Execution Queue:**
- Current capacity: PostToolUse hooks execute sequentially after each tool. With multiple hooks, overhead compounds.
- Limit: If 3+ hooks run after every tool, and tools are frequent, overhead could reach 5+ seconds per tool in worst case (10s timeout each, but parallelizable).
- Scaling path: Parallelize hook execution using `Promise.all()`; consolidate hooks into single binary with modular components; add hook performance metrics

## Dependencies at Risk

**npm Package Update Checks:**
- Risk: Update check calls `npm view get-shit-done-cc version` every session with 10-second timeout. If npm registry is down, sessions are delayed.
- Files: `.claude/hooks/gsd-check-update.js` (line 95)
- Impact: Unavailable npm registry blocks session start
- Migration plan: Add fallback to cached version if npm check fails; implement offline mode flag to disable checks; use GitHub releases API as fallback to npm registry

**Stale Hook Detection Logic:**
- Risk: Hook version headers are manually maintained strings (`// gsd-hook-version: X.Y.Z`). Version drift between VERSION file and hook headers will cause false positives/negatives.
- Files: `.claude/hooks/gsd-check-update.js` (lines 73-90)
- Impact: Users may see warnings about stale hooks that are actually up-to-date; updates may be skipped if versions appear to match
- Migration plan: Source version from single VERSION file at runtime; embed version hash in hook instead of string; implement hook signature verification

## Missing Critical Features

**Hook Health Monitoring:**
- Problem: No way to verify hooks are installed and functioning. Users receive context warnings only if context-monitor hook is working - if it's broken, users never know.
- Blocks: Debugging hook failures; detecting broken installations; ensuring monitoring is active

**Update Rollback Mechanism:**
- Problem: No way to rollback to previous GSD version if update breaks workflow. Only warning is "stale hooks" message.
- Blocks: Safe updates for production workflows; recovery from bad releases; version pinning

**Persistent Error Logs:**
- Problem: All hook errors are silent. No error history, audit trail, or diagnostic logs exist.
- Blocks: Debugging production issues; monitoring system health; post-incident analysis

**Context Usage Audit Trail:**
- Problem: No persistent record of when context reached critical levels, which agents consumed most context, or context usage trends.
- Blocks: Optimizing agent designs; understanding session patterns; capacity planning

## Test Coverage Gaps

**Hook Integration Tests:**
- What's not tested: End-to-end flow of statusline writing metrics → context-monitor reading and warning. No tests for concurrent hook execution or temp file race conditions.
- Files: `.claude/hooks/` directory (no test directory exists)
- Risk: Race conditions, json parse failures, or file permission issues in production are never caught
- Priority: High - hooks run in every session

**Hook Version Detection:**
- What's not tested: Regex parsing of version headers; behavior with malformed headers; version comparison logic
- Files: `.claude/hooks/gsd-check-update.js` (lines 73-90)
- Risk: Version detection silently fails on unexpected formats; stale hooks go undetected
- Priority: High - affects update notifications

**Agent Documentation Validation:**
- What's not tested: Syntactic validity of bash snippets; existence of `@file:` references; correctness of file paths in agent docs
- Files: All `.claude/agents/*.md` files
- Risk: Typos in instructions or file paths discovered only at execution time; broken workflows cascade to end users
- Priority: High - affects all agent executions

**Config File Schema Validation:**
- What's not tested: `.planning/config.json` structure; handling of invalid config values
- Files: `.claude/hooks/gsd-context-monitor.js` (lines 50-60)
- Risk: Invalid config silently ignored; features may be disabled without user awareness
- Priority: Medium - impacts only users with custom config

---

*Concerns audit: 2026-03-20*
