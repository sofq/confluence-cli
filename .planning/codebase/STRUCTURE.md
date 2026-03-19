# Codebase Structure

**Analysis Date:** 2026-03-20

## Directory Layout

```
confluence-cli/
├── .claude/                                    # GSD framework and agent definitions
│   ├── agents/                                 # 15+ specialized Claude agents
│   │   ├── gsd-codebase-mapper.md              # Codebase analysis agent
│   │   ├── gsd-executor.md                     # Phase execution agent
│   │   ├── gsd-planner.md                      # Phase planning agent
│   │   ├── gsd-plan-checker.md                 # Plan quality verification
│   │   ├── gsd-phase-researcher.md             # Technical research agent
│   │   ├── gsd-verifier.md                     # Verification & UAT agent
│   │   ├── gsd-debugger.md                     # Bug investigation agent
│   │   ├── gsd-roadmapper.md                   # Roadmap planning agent
│   │   ├── gsd-project-researcher.md           # Project-scope research
│   │   ├── gsd-integration-checker.md          # External integration validation
│   │   └── [other agents]                      # UI auditor, user profiler, etc.
│   ├── commands/                               # GSD command definitions
│   │   └── gsd/                                # /gsd:* command handlers
│   ├── hooks/                                  # Lifecycle hooks
│   │   ├── gsd-check-update.js                 # Check for framework updates
│   │   ├── gsd-statusline.js                   # Status line rendering
│   │   └── gsd-context-monitor.js              # Context usage tracking
│   ├── get-shit-done/                          # Core framework
│   │   ├── bin/                                # Executable and utilities
│   │   │   ├── gsd-tools.cjs                   # Main CLI tool (~1K lines)
│   │   │   └── lib/                            # Shared utility modules
│   │   │       ├── core.cjs                    # Path helpers, config, git ops (~550 lines)
│   │   │       ├── commands.cjs                # Utility commands (~600 lines)
│   │   │       ├── state.cjs                   # STATE.md CRUD (~750 lines)
│   │   │       ├── phase.cjs                   # Phase lifecycle ops (~850 lines)
│   │   │       ├── init.cjs                    # Project initialization (~700 lines)
│   │   │       ├── roadmap.cjs                 # ROADMAP.md operations (~400 lines)
│   │   │       ├── milestone.cjs               # Milestone management (~240 lines)
│   │   │       ├── frontmatter.cjs             # YAML frontmatter parsing (~300 lines)
│   │   │       ├── verify.cjs                  # Verification suite (~850 lines)
│   │   │       ├── profile-output.cjs          # User profiling output (~1000 lines)
│   │   │       ├── profile-pipeline.cjs        # Profiling workflow (~430 lines)
│   │   │       ├── config.cjs                  # Config management (~250 lines)
│   │   │       ├── template.cjs                # Template filling (~180 lines)
│   │   │       ├── model-profiles.cjs          # Model definitions (~80 lines)
│   │   │       └── [more modules]              # State, phase, etc.
│   │   ├── workflows/                          # Orchestration workflows (50+ files)
│   │   │   ├── do.md                           # User intent dispatcher
│   │   │   ├── plan-phase.md                   # Phase planning workflow
│   │   │   ├── execute-phase.md                # Phase execution workflow
│   │   │   ├── map-codebase.md                 # Codebase mapping workflow
│   │   │   ├── new-project.md                  # Project initialization
│   │   │   ├── research-phase.md               # Technical research workflow
│   │   │   ├── discuss-phase.md                # User decisions workflow
│   │   │   ├── execute-plan.md                 # Plan execution
│   │   │   ├── autonomous.md                   # Full project automation
│   │   │   ├── verify-work.md                  # Verification workflow
│   │   │   ├── quick.md                        # Quick single-task execution
│   │   │   ├── add-phase.md                    # Add roadmap phase
│   │   │   ├── complete-milestone.md           # Milestone completion
│   │   │   ├── progress.md                     # Progress reporting
│   │   │   └── [30+ more workflows]            # health, cleanup, debug, etc.
│   │   ├── references/                         # System documentation
│   │   │   ├── checkpoints.md                  # Checkpoint protocol spec
│   │   │   ├── continuation-format.md          # Resumable execution format
│   │   │   ├── decimal-phase-calculation.md    # Phase numbering rules
│   │   │   ├── git-integration.md              # Git integration details
│   │   │   ├── model-profile-resolution.md     # Model selection logic
│   │   │   ├── phase-argument-parsing.md       # Phase number parsing
│   │   │   ├── planning-config.md              # Config schema
│   │   │   ├── tdd.md                          # TDD execution pattern
│   │   │   ├── verification-patterns.md        # Verification approach
│   │   │   ├── user-profiling.md               # User profiling system
│   │   │   └── [more references]               # UI brand, questioning, etc.
│   │   ├── templates/                          # Document templates
│   │   │   ├── codebase/                       # Codebase analysis templates
│   │   │   │   ├── architecture.md             # ARCHITECTURE.md template
│   │   │   │   ├── structure.md                # STRUCTURE.md template
│   │   │   │   ├── conventions.md              # CONVENTIONS.md template
│   │   │   │   ├── testing.md                  # TESTING.md template
│   │   │   │   ├── stack.md                    # STACK.md template
│   │   │   │   ├── integrations.md             # INTEGRATIONS.md template
│   │   │   │   ├── concerns.md                 # CONCERNS.md template
│   │   │   │   └── (more templates)
│   │   │   ├── milestone.md                    # Milestone template
│   │   │   ├── phase-prompt.md                 # Phase prompt template
│   │   │   ├── PLAN.md                         # Plan template
│   │   │   ├── SUMMARY.md                      # Summary template
│   │   │   ├── CONTEXT.md                      # Context template
│   │   │   ├── VERIFICATION.md                 # Verification template
│   │   │   ├── UAT.md                          # User acceptance test template
│   │   │   └── [20+ more templates]            # discovery, debug, state, etc.
│   │   └── VERSION                             # Framework version (e.g., "1.26.0")
│   ├── settings.json                           # GSD hooks configuration
│   ├── settings.local.json                     # Local overrides
│   ├── gsd-file-manifest.json                  # File manifest with checksums (~1K entries)
│   └── package.json                            # CommonJS marker ({"type": "commonjs"})
├── .planning/                                  # Project planning root (created by user)
│   ├── codebase/                               # Codebase analysis docs
│   │   ├── ARCHITECTURE.md                     # Code organization patterns
│   │   ├── STRUCTURE.md                        # File structure guide
│   │   ├── CONVENTIONS.md                      # Coding conventions
│   │   ├── TESTING.md                          # Test patterns
│   │   ├── STACK.md                            # Technology stack
│   │   ├── INTEGRATIONS.md                     # External integrations
│   │   └── CONCERNS.md                         # Technical debt & issues
│   ├── config.json                             # Project config (model profile, features)
│   ├── STATE.md                                # Current project state
│   ├── ROADMAP.md                              # Phase definitions and status
│   ├── REQUIREMENTS.md                         # Project requirements (REQ-* IDs)
│   ├── phases/                                 # Phase directories
│   │   ├── 1-setup/                            # Phase 1: Setup
│   │   │   ├── PLAN.md                         # Phase execution plan
│   │   │   ├── SUMMARY.md                      # Phase completion summary
│   │   │   ├── CONTEXT.md                      # User decisions & vision
│   │   │   └── VERIFICATION.md                 # Phase verification results
│   │   ├── 2-core-features/
│   │   ├── 2.1-refactor/                       # Decimal phase (child of 2)
│   │   └── [phase directories]
│   ├── milestones/                             # Archived phases
│   │   ├── v1.0-phases/                        # Milestone v1.0 phases
│   │   ├── MILESTONES.md                       # Milestone index
│   │   └── [milestone directories]
│   ├── todos/                                  # Todo tracking
│   │   ├── pending/                            # Pending todos
│   │   └── completed/                          # Completed todos
│   └── WAITING.json                            # Async checkpoint signal (optional)
├── .git/                                       # Git repository
├── CLAUDE.md                                   # Project instructions (optional)
└── [user codebase files]                       # Application code being built
```

## Directory Purposes

**.claude/agents/:**
- Purpose: Specialized Claude agent definitions for different roles
- Contains: Agent role specs, process flows, tool usage, decision rules
- Key files: 15+ agent markdown files, each 5-40KB
- Pattern: Each agent defines a single role (planner, executor, researcher, etc.)

**.claude/get-shit-done/bin/:**
- Purpose: Core CLI and shared utilities
- Contains: gsd-tools.cjs (entry point) and 14 library modules (.cjs files)
- Key files: `core.cjs` (path/config helpers), `state.cjs` (STATE.md ops), `phase.cjs` (phase lifecycle)
- Pattern: CommonJS modules with functions exported at module.exports

**.claude/get-shit-done/workflows/:**
- Purpose: Orchestration workflows that coordinate agents and tools
- Contains: 50+ workflow markdown files (one per GSD command/operation)
- Key files: `do.md` (dispatcher), `plan-phase.md`, `execute-phase.md`, `map-codebase.md`
- Pattern: Each workflow contains a process section with numbered steps, bash invocations, agent spawning

**.claude/get-shit-done/references/:**
- Purpose: System documentation and specifications
- Contains: 15+ reference markdown files covering protocols, formats, schemas
- Key files: `checkpoints.md`, `continuation-format.md`, `planning-config.md`, `git-integration.md`
- Pattern: Detailed specifications for framework internals

**.claude/get-shit-done/templates/:**
- Purpose: Template documents and scaffolding
- Contains: 40+ template files for projects, phases, plans, summaries, verification
- Key files: `codebase/*.md` (ARCHITECTURE.md, STRUCTURE.md, TESTING.md, etc.), `PLAN.md`, `SUMMARY.md`, `CONTEXT.md`
- Pattern: Markdown with `[Placeholder text]` for template filling

**.planning/:**
- Purpose: Project state and deliverables (created by user running `/gsd:new-project`)
- Contains: Config, state, roadmap, phases, todos, milestones
- Key files: `config.json` (project settings), `STATE.md` (current progress), `ROADMAP.md` (phase list)
- Pattern: Hierarchical with phases as subdirectories

**.planning/phases/:**
- Purpose: Container for individual phase work
- Contains: Per-phase directories named `{padded_number}-{slug}` (e.g., `01-setup`, `02-core-features`, `02.1-refactor`)
- Key files: `PLAN.md` (spec), `SUMMARY.md` (results), `CONTEXT.md` (decisions), `VERIFICATION.md` (validation)
- Pattern: One directory per phase, multiple PLAN/SUMMARY files supported for wave-based execution

## Key File Locations

**Entry Points:**
- `.claude/get-shit-done/bin/gsd-tools.cjs` — Main CLI utility (all atomic commands)
- `.claude/get-shit-done/workflows/do.md` — User intent dispatcher
- `.claude/agents/gsd-executor.md` — Phase execution agent (spawned by execute-phase)
- `.claude/agents/gsd-planner.md` — Phase planning agent (spawned by plan-phase)

**Configuration:**
- `.claude/settings.json` — GSD hook configuration
- `.planning/config.json` — Project-level settings (model profile, branching strategy, features)
- `.planning/STATE.md` — Current project state (phase, progress, blockers)

**Core Logic:**
- `.claude/get-shit-done/bin/lib/core.cjs` — Shared utilities (path normalization, config loading, git ops)
- `.claude/get-shit-done/bin/lib/state.cjs` — STATE.md CRUD operations
- `.claude/get-shit-done/bin/lib/phase.cjs` — Phase lifecycle operations (list, create, complete, remove)
- `.claude/get-shit-done/bin/lib/verify.cjs` — Plan and summary verification

**Workflows & Orchestration:**
- `.claude/get-shit-done/workflows/plan-phase.md` — Workflow: plan a phase
- `.claude/get-shit-done/workflows/execute-phase.md` — Workflow: execute a phase
- `.claude/get-shit-done/workflows/map-codebase.md` — Workflow: analyze codebase
- `.claude/get-shit-done/workflows/new-project.md` — Workflow: initialize project

**Planning & State:**
- `.planning/ROADMAP.md` — Phase definitions (name, goal, status, requirements)
- `.planning/STATE.md` — Current phase, progress %, blockers, completed milestones
- `.planning/phases/{phase}/PLAN.md` — Phase execution specification
- `.planning/phases/{phase}/SUMMARY.md` — Phase completion results and patterns

## Naming Conventions

**Files:**
- Agent definitions: `gsd-{role}.md` (e.g., `gsd-executor.md`, `gsd-planner.md`)
- Workflows: `{operation}.md` (e.g., `plan-phase.md`, `execute-phase.md`)
- Plan files: `PLAN.md` or `{wave}-PLAN.md` (e.g., `1-PLAN.md` for wave 1)
- Summary files: `SUMMARY.md` or `{wave}-SUMMARY.md`
- Phase directories: `{padded_phase}-{slug}` (e.g., `01-setup`, `02.1-refactor`)
- Config files: `.cjs` for CommonJS modules, `.md` for documentation/workflows

**Directories:**
- Phases: `.planning/phases/` with subdirectories per phase
- Milestones: `.planning/milestones/` with subdirectories per milestone
- Todos: `.planning/todos/pending/` and `.planning/todos/completed/`
- Codebase analysis: `.planning/codebase/` with ARCHITECTURE.md, STRUCTURE.md, etc.

**Phase Numbering:**
- Integer phases: `1`, `2`, `3` (main phases)
- Decimal phases: `2.1`, `2.2` (sub-phases under parent 2)
- Letter suffix: `3a`, `3b` (variant phases)
- Padded in directories: `01`, `02`, `02.1` (zero-padded for sorting)

## Where to Add New Code

**New Feature Phase:**
- Primary code: Create phase directory `.planning/phases/{phase}-{slug}/` with PLAN.md
- Tests: If applicable, add test tasks to PLAN.md with `tdd="true"` flag
- Documentation: Add CONTEXT.md for user decisions, VERIFICATION.md for validation

**New Agent:**
- Implementation: Create `.claude/agents/gsd-{role}.md` with role spec, process, decision rules
- Entry point: Define how it's spawned (from which workflow)
- Tools: Specify required tools (Read, Write, Edit, Bash, Grep, Glob)

**New Workflow:**
- Orchestration: Create `.claude/get-shit-done/workflows/{name}.md` with process steps
- Agent spawning: Define which agents to spawn and in what sequence
- State management: Use `gsd-tools state *` commands for STATE.md updates

**New gsd-tools Command:**
- CLI handler: Add function to appropriate module in `.claude/get-shit-done/bin/lib/*.cjs`
- Entry point: Add command dispatch in `.claude/get-shit-done/bin/gsd-tools.cjs`
- Export: Add function to module.exports
- Documentation: Add command to gsd-tools.cjs header comments

**Utilities & Helpers:**
- Shared helpers: `.claude/get-shit-done/bin/lib/core.cjs` for common patterns (path ops, git, config)
- Module-specific: Add to specific module if specialized (state ops to state.cjs, phase ops to phase.cjs)

## Special Directories

**.claude/:**
- Purpose: GSD framework source code (do not modify unless extending framework)
- Generated: No (checked into git)
- Committed: Yes (framework distribution)
- Permissions: Read-only for framework users (modify only when extending)

**.planning/:**
- Purpose: Project planning and state (created and managed by GSD)
- Generated: Yes (created by `/gsd:new-project`)
- Committed: Yes (planning docs and state tracked in git)
- Permissions: Read/write (user modifies STATE.md, CONTEXT.md via workflows)

**.planning/codebase/:**
- Purpose: Codebase analysis documents (written by gsd-codebase-mapper)
- Generated: Yes (created by `/gsd:map-codebase`)
- Committed: Yes (reference documents for planning/execution)
- Permissions: Read for executors, write-only by mapper

**.planning/phases/:**
- Purpose: Phase execution directories
- Generated: Yes (created by `/gsd:add-phase` or plan-phase workflow)
- Committed: Yes (plans and summaries tracked)
- Permissions: Read for all agents, write by executor (SUMMARY.md)

**.planning/milestones/:**
- Purpose: Archived phases from completed milestones
- Generated: Yes (created by `/gsd:complete-milestone`)
- Committed: Yes (milestone history tracked)
- Permissions: Read-only (archive)

**.planning/todos/:**
- Purpose: Todo tracking (pending and completed)
- Generated: Yes (created by `/gsd:add-todo`)
- Committed: Yes (todo history tracked)
- Permissions: Read/write by todo operations

---

*Structure analysis: 2026-03-20*
