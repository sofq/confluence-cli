# Coding Conventions

**Analysis Date:** 2026-03-20

## Naming Patterns

**Files:**
- Kebab-case for JavaScript/Node files: `gsd-check-update.js`, `gsd-statusline.js`, `gsd-context-monitor.js`
- Kebab-case for agent/command markdown files: `gsd-codebase-mapper.md`, `gsd-executor.md`, `gsd-planner.md`
- Version tracking in hook headers: First line comments include hook version (e.g., `// gsd-hook-version: 1.26.0`)

**Functions:**
- camelCase for function declarations and variables: `detectConfigDir()`, `stdinTimeout`, `isGsdActive`
- Descriptive names indicating purpose: `detectConfigDir()`, `readMetricsFromBridge()`, `buildAdvisoryWarning()`

**Variables:**
- camelCase for all variable names: `configDir`, `cacheFile`, `sessionId`, `remaining`, `usableRemaining`
- Constants in UPPER_SNAKE_CASE: `WARNING_THRESHOLD`, `CRITICAL_THRESHOLD`, `STALE_SECONDS`, `DEBOUNCE_CALLS`
- Prefix naming conventions for state tracking: `warnData`, `metricsPath`, `bridgePath` (domain + purpose)

**Types:**
- JSON objects defined inline: `{ recursive: true }`, `{ encoding: 'utf8', timeout: 10000, windowsHide: true }`
- Array variables plural: `hookFiles`, `staleHooks`, `files`
- Boolean variables with `is` prefix: `isGsdActive`, `isCritical`, `firstWarn`

## Code Style

**Formatting:**
- Spaces for indentation: 2 spaces per level (seen in all .js files)
- Semicolons: Required at end of statements
- Line length: Practical wrap around ~80-100 characters, with string continuation allowed

**Linting:**
- ESLint expected (hook configuration mentions `eslint --fix` in PostToolUse hooks)
- Config location: `.eslintrc*` files expected in project root or .claude directory
- Auto-fix on write operations: PostToolUse hooks run `npx eslint --fix $FILE 2>/dev/null || true` for Write/Edit tools

**Quote Style:**
- Single quotes `'` preferred for strings: `'utf8'`, `'path'`, `'fs'`
- Double quotes for JSON: `{ "type": "command" }`
- Template literals with backticks for interpolation: `` `${variable}` ``

## Import Organization

**Order:**
1. Node.js built-in modules (fs, path, os, child_process, etc.)
2. Conditionally imported modules based on availability
3. Local file/config imports

**Examples from codebase:**
```javascript
// Pattern 1 - Standard imports
const fs = require('fs');
const path = require('path');
const os = require('os');
const { spawn } = require('child_process');

// Pattern 2 - Destructured imports
const { execSync } = require('child_process');
```

**Path Aliases:**
- No aliases detected; uses relative and absolute paths directly
- Environment variable support: `process.env.CLAUDE_CONFIG_DIR`, `process.env.GEMINI_API_KEY`

## Error Handling

**Patterns:**
- Try-catch blocks for file I/O and JSON parsing: Used extensively in hook files
- Silent failures with comments: `} catch (e) {}` with preceding comment explaining why silent
- Graceful degradation: Exit silently (process.exit(0)) on non-critical errors rather than throwing
- Timeout guards for I/O operations: `const stdinTimeout = setTimeout(() => process.exit(0), timeout)`

**Error Recovery:**
- Parse errors treated as non-blocking: Files with corrupted JSON get default values
- Fallback values on errors: `const installed = '0.0.0'` as fallback when version read fails
- Config file missing is not an error: Conditional checks (`if (fs.existsSync(...))`)

**Examples:**
```javascript
// Silent catch with explanation
try {
  if (fs.existsSync(projectVersionFile)) {
    installed = fs.readFileSync(projectVersionFile, 'utf8').trim();
    configDir = path.dirname(path.dirname(projectVersionFile));
  }
} catch (e) {}  // Silent: version detection is optional

// Timeout guard
const stdinTimeout = setTimeout(() => process.exit(0), 3000);
process.stdin.on('end', () => {
  clearTimeout(stdinTimeout);
  // ... process stdin
});

// Graceful degradation on config error
try {
  const config = JSON.parse(fs.readFileSync(configPath, 'utf8'));
  if (config.hooks?.context_warnings === false) {
    process.exit(0);
  }
} catch (e) {
  // Ignore config parse errors - hook execution continues
}
```

## Logging

**Framework:** console and process.stdout only (no external logging libraries)

**Patterns:**
- console.log() for general output: Not used in hook files (silent-first approach)
- process.stdout.write() for direct output: Used for statusline display and hook output
- No debug logging visible in production code

**When to Log:**
- Status/progress information via process.stdout.write()
- Never log to stdout during normal hook execution (only JSON output for hooks)
- File writing for persistent state instead of logging

**Example:**
```javascript
// Hook output format - structured JSON
const output = {
  hookSpecificOutput: {
    hookEventName: process.env.GEMINI_API_KEY ? "AfterTool" : "PostToolUse",
    additionalContext: message
  }
};
process.stdout.write(JSON.stringify(output));
```

## Comments

**When to Comment:**
- Header comments explaining file purpose: `// gsd-hook-version: 1.26.0` followed by description
- Inline comments for non-obvious logic: Explaining why silent catch blocks exist, why thresholds are set
- Comments explaining external system integration: References to issue numbers (#775, #884) and specific behaviors

**Format:**
- Single-line comments with `//` for most explanations
- Multi-line comments with `//` repeated (no `/* */` blocks in hook files)
- Issue references in comments: `// See #775` for traceability

**Examples:**
```javascript
// Timeout guard: if stdin doesn't close within 3s (e.g. pipe issues on
// Windows/Git Bash), exit silently instead of hanging. See #775.
const stdinTimeout = setTimeout(() => process.exit(0), 3000);

// Debounce: check if we warned recently
const warnPath = path.join(tmpDir, `claude-ctx-${sessionId}-warned.json`);

// Detect if GSD is active (has .planning/STATE.md in working directory)
const isGsdActive = fs.existsSync(path.join(cwd, '.planning', 'STATE.md'));
```

## Function Design

**Size:** Functions 20-100 lines typical; longer functions broken into stages with comment markers

**Parameters:**
- Small parameter counts preferred (1-3 parameters typical)
- Object parameters for configuration: `fs.mkdirSync(cacheDir, { recursive: true })`
- Destructuring in parameter lists where appropriate

**Return Values:**
- Explicit return statements; no implicit undefined returns
- Boolean returns for existence checks: `fs.existsSync()`
- JSON object returns: `{ recursive: true }`

**Example - Well-structured function:**
```javascript
// gsd-check-update.js - detectConfigDir function
function detectConfigDir(baseDir) {
  // Check env override first (supports multi-account setups)
  const envDir = process.env.CLAUDE_CONFIG_DIR;
  if (envDir && fs.existsSync(path.join(envDir, 'get-shit-done', 'VERSION'))) {
    return envDir;
  }
  for (const dir of ['.config/opencode', '.opencode', '.gemini', '.claude']) {
    if (fs.existsSync(path.join(baseDir, dir, 'get-shit-done', 'VERSION'))) {
      return path.join(baseDir, dir);
    }
  }
  return envDir || path.join(baseDir, '.claude');
}
```

## Module Design

**Exports:**
- CommonJS (`module.exports`) not used in hook files; hooks are script-based
- Functional/procedural style preferred over OOP in Node hooks
- Direct execution model: scripts run completely on invocation

**Barrel Files:**
- Not applicable to hook structure; each hook is independent
- Markdown agents are standalone documents, not imported/exported

**Architectural Pattern:**
- Hook pattern: File-based scripts invoked by external system (Claude Code)
- Data exchange via JSON over stdin/stdout
- File-based state persistence: JSON files in cache directories and /tmp

## Code Quality Standards

**Defensive Programming:**
- Existence checks before file operations: `if (fs.existsSync(...))`
- Try-catch around all I/O operations
- Null/undefined checks: `if (remaining != null)` for optional values
- Optional chaining: `data.model?.display_name || 'Claude'`

**Robustness:**
- Non-blocking error handling: Errors exit silently rather than propagate
- Timeout guards on all stdin/async operations
- Graceful degradation: Missing configs don't break execution

**Examples:**
```javascript
// Defensive: Check existence before reading
if (fs.existsSync(metricsPath)) {
  const metrics = JSON.parse(fs.readFileSync(metricsPath, 'utf8'));
  // process metrics
}

// Defensive: Optional chaining for nested access
const model = data.model?.display_name || 'Claude';
const remaining = data.context_window?.remaining_percentage;

// Defensive: Safe fallback on error
if (warnData.callsSinceWarn < DEBOUNCE_CALLS && !severityEscalated) {
  fs.writeFileSync(warnPath, JSON.stringify(warnData));
  process.exit(0);
}
```

---

*Convention analysis: 2026-03-20*
