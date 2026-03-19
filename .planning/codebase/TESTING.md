# Testing Patterns

**Analysis Date:** 2026-03-20

## Test Framework

**Runner:**
- Not detected - No Jest, Vitest, or Mocha config found
- Test files: None found (`*.test.js`, `*.spec.js`)

**Assertion Library:**
- Not applicable - No testing infrastructure detected in current codebase

**Current Status:**
- The codebase contains GSD agent definitions (markdown files) and hook scripts (Node.js)
- Hook scripts in `.claude/hooks/` are production code without automated tests
- Testing approach: Manual validation and integration testing via GSD orchestrator

**Run Commands:**
- No standard test commands configured
- Manual testing: Run hooks directly with node or through Claude Code hook system

## Test File Organization

**Current Structure:**
- No dedicated test directory
- No test files co-located with source
- Testing occurs through end-to-end GSD phase execution

**Future Pattern (when testing is added):**
- Recommended: `.test.js` suffix co-located with source
- Location: `hooks/gsd-*.test.js` next to `hooks/gsd-*.js`

## Test Structure

**Current Testing Approach:**
GSD codebase uses integration testing through:
1. Phase-based execution via `/gsd:*` commands
2. Orchestrator validation (gsd-plan-checker, gsd-verifier)
3. Manual verification of hook behavior (statusline, context-monitor, update-check)

**When Hook Tests Should Be Added:**

Suggested test structure for future testing:
```javascript
// hooks/gsd-statusline.test.js - hypothetical pattern

describe('gsd-statusline hook', () => {
  describe('context usage calculation', () => {
    it('should normalize remaining context to usable range', () => {
      // Setup: context data with 50% remaining
      // Assert: usable context calculated correctly considering 16.5% buffer
    });

    it('should show progress bar with correct color', () => {
      // Setup: various usage percentages
      // Assert: ANSI color codes match thresholds (green < 50%, yellow < 65%, etc.)
    });
  });

  describe('task extraction from todos', () => {
    it('should find in-progress task from todo file', () => {
      // Setup: Mock todo JSON with in_progress status
      // Assert: Task name extracted correctly
    });
  });
});
```

## Mocking

**Current Approach:**
- No mocking framework (Jest/Sinon) in use
- Testing done through actual file system operations

**If Testing Were to Be Added:**

Recommended mocking strategy:
```javascript
// Pattern 1: Mock fs module for file operations
jest.mock('fs');
fs.existsSync.mockReturnValue(true);
fs.readFileSync.mockReturnValue('test content');

// Pattern 2: Mock child_process for spawned operations
jest.mock('child_process');
const { spawn } = require('child_process');
spawn.mockImplementation(() => ({ unref: jest.fn() }));

// Pattern 3: Mock environment variables
process.env.CLAUDE_CONFIG_DIR = '/tmp/test-config';
```

**What to Mock:**
- File system operations: `fs.readFileSync`, `fs.writeFileSync`, `fs.existsSync`
- Child process spawning: `spawn()` calls (e.g., background update checks)
- Environment variables: `process.env.*`
- External commands: `execSync('npm view ...')`

**What NOT to Mock:**
- Path operations: `path.join()`, `path.dirname()` (keep real)
- JSON parsing/stringifying: Keep real (format validation matters)
- Timeout operations: Use jest.useFakeTimers() instead of mocking

## Fixtures and Test Data

**Current Status:**
- No fixtures or test data files
- Settings stored in `.claude/settings.json` and `.claude/settings.local.json` (production use only)

**If Testing Were to Be Added:**

Test fixture pattern:
```javascript
// hooks/__fixtures__/sampleData.js

const validMetrics = {
  session_id: 'test-session-123',
  remaining_percentage: 45,
  used_pct: 55,
  timestamp: Math.floor(Date.now() / 1000)
};

const lowContextMetrics = {
  session_id: 'test-session-456',
  remaining_percentage: 20,
  used_pct: 80,
  timestamp: Math.floor(Date.now() / 1000)
};

module.exports = { validMetrics, lowContextMetrics };
```

**Location for future tests:**
- Fixtures: `hooks/__fixtures__/` directory
- Sample configs: `hooks/__fixtures__/sample-config.json`
- Sample state files: `hooks/__fixtures__/sample-state.json`

## Coverage

**Requirements:** Not enforced - No test infrastructure in place

**Current Testing:**
- Manual validation through GSD orchestrator execution
- Automated validation via hook checkers (gsd-plan-checker, gsd-verifier)
- Integration testing through phase execution

**Future Coverage Target:**
If testing framework is added, prioritize:
1. Context calculation logic (gsd-statusline.js) - 80% coverage
2. Hook version detection (gsd-check-update.js) - 85% coverage
3. Context warning thresholds (gsd-context-monitor.js) - 90% coverage

**View Coverage (if implemented):**
```bash
jest --coverage
jest --coverage --watch
# or
npm test -- --coverage
```

## Test Types

**Unit Tests (when implemented):**
- Scope: Individual functions and pure logic
- Example targets:
  - `detectConfigDir()` in gsd-check-update.js
  - Context usage calculation in gsd-statusline.js
  - Threshold logic in gsd-context-monitor.js
- Approach: Mock file system and environment

**Integration Tests:**
- Scope: Hook behavior with real file system
- Current method: Manual testing + GSD orchestrator execution
- Example: Running full hook with actual stdin/stdout
- Executed through: `/gsd:execute-phase` with hook-dependent tasks

**E2E Tests:**
- Framework: Not in use
- Would be handled by GSD orchestrator verification
- Current equivalent: gsd-verifier agent validates hook behavior in context

## Common Patterns for Future Testing

**Async Testing:**
```javascript
// If async code is added (currently all sync in hooks)
describe('async operations', () => {
  it('should handle stdin timeout', async () => {
    jest.useFakeTimers();
    // Setup stdin stream
    setTimeout(() => process.exit(0), 3000);
    // Assert exit called
    expect(process.exit).toHaveBeenCalledWith(0);
    jest.runAllTimers();
  });
});
```

**Error Testing:**
```javascript
// Testing error handling patterns (currently silent)
describe('error handling', () => {
  it('should silently fail on corrupted JSON', () => {
    fs.readFileSync.mockReturnValue('invalid json {');
    // Run hook with malformed data
    // Assert: no exception thrown, process exits gracefully
  });

  it('should ignore missing metrics file', () => {
    fs.existsSync.mockReturnValue(false);
    // Run context monitor hook
    // Assert: exits with code 0, no error logged
  });
});
```

## Testing Philosophy for This Codebase

**Validated Through Execution:**
The GSD codebase validates hooks through actual execution in phases:
1. Hooks are tested when `/gsd:execute-phase` runs
2. Hook failures prevent phase completion
3. Verification stage (gsd-verifier) validates hook outputs

**Manual Testing Current Approach:**
```bash
# Test statusline hook manually
echo '{"model":{"display_name":"Claude"},"context_window":{"remaining_percentage":50}}' | \
  node .claude/hooks/gsd-statusline.js

# Test context monitor manually
echo '{"session_id":"test-123","cwd":"$(pwd)"}' | \
  node .claude/hooks/gsd-context-monitor.js

# Test update check hook
node .claude/hooks/gsd-check-update.js &
# Check cache output after 2 seconds
cat ~/.claude/cache/gsd-update-check.json
```

**Why Current Approach Works:**
- Hooks are simple, focused scripts (100-160 lines each)
- Logic is straightforward (file I/O, JSON parsing, environment detection)
- Failures are caught by orchestrator validation
- Integration testing through GSD phases catches most issues

---

*Testing analysis: 2026-03-20*
