# Architecture

**Analysis Date:** 2026-03-20

## Pattern Overview

**Overall:** Modular CLI Orchestration Framework with Agent-Driven Task Execution

**Key Characteristics:**
- Distributed agent architecture with specialized roles (planner, executor, researcher, verifier)
- Central CLI entrypoint routing user input to appropriate workflows
- Phase-based project management system with state persistence
- Markdown-driven planning documents as executable prompts
- Git-integrated commit lifecycle with state tracking

## Layers

**CLI Router & Dispatcher Layer:**
- Purpose: Accept user input and route to appropriate GSD command
- Location: `.claude/get-shit-done/workflows/do.md` (dispatcher)
- Contains: Input validation, intent matching, command routing logic
- Depends on: State loader, project detection
- Used by: User shell, orchestrator commands

**Workflow Orchestration Layer:**
- Purpose: Coordinate multi-agent workflows with checkpoints and state transitions
- Location: `.claude/get-shit-done/workflows/*.md` (e.g., `plan-phase.md`, `execute-phase.md`)
- Contains: Workflow definitions, agent sequencing, initialization, checkpoint handling
- Depends on: gsd-tools CLI, state management, phase/roadmap systems
- Used by: User input routing, autonomous execution flows

**Agent Execution Layer:**
- Purpose: Specialized Claude agents that perform domain-specific work
- Location: `.claude/agents/gsd-*.md` (15+ agent definitions)
- Contains: Agent roles (planner, executor, researcher, verifier, debugger, etc.), task execution protocols, checkpoint handling
- Depends on: Project context (CLAUDE.md), codebase docs, plan/state files
- Used by: Orchestration workflows

**Core Library Layer:**
- Purpose: Shared utilities for state, config, phase, and file management
- Location: `.claude/get-shit-done/bin/lib/*.cjs` (14 CommonJS modules)
- Contains: File I/O, git operations, config loading, phase/milestone tracking, frontmatter parsing
- Depends on: Node.js, fs, path, child_process modules
- Used by: gsd-tools CLI, orchestration workflows

**CLI Tools Layer:**
- Purpose: Command-line interface exposing library functionality
- Location: `.claude/get-shit-done/bin/gsd-tools.cjs` (single entry point)
- Contains: Command parsing, argument handling, JSON output, large payload streaming
- Depends on: Core library, Node.js runtime
- Used by: Workflows, agents (for atomic operations like state load, phase lookup, git commits)

**Planning & Metadata Layer:**
- Purpose: Structured documents that define project state and phases
- Location: `.planning/` directory in user's project
- Contains: STATE.md (current progress), ROADMAP.md (phase definitions), REQUIREMENTS.md, phase directories with PLAN.md/SUMMARY.md
- Depends on: Nothing — read-only from agent perspective
- Used by: All orchestration and agents for context

## Data Flow

**Phase-Based Planning & Execution:**

1. **Phase Planning**
   - User requests planning: `/gsd:plan-phase <phase>`
   - Orchestrator initializes via `gsd-tools init plan-phase`
   - Phase researcher (if needed) analyzes scope and creates DISCOVERY.md
   - Planner creates PLAN.md with tasks, dependencies, and success criteria
   - Plan checker validates structure and feasibility
   - Plans committed to git (if enabled)

2. **Phase Execution**
   - User requests execution: `/gsd:execute-phase <phase>`
   - Executor reads PLAN.md, parses tasks, and executes each task sequentially
   - For each task: execute code → verify criteria → create atomic commit
   - Handle deviations (auto-fix bugs, add missing critical features, resolve blocking issues)
   - Track completion time, deviations, and commit hashes
   - Generate SUMMARY.md with outcomes and patterns established
   - Update STATE.md with phase completion

3. **Verification & Iteration**
   - Plan checker verifies SUMMARY.md against PLAN.md
   - If gaps detected: create gap-closure plans via `/gsd:plan-phase --gaps`
   - Verifier produces VERIFICATION.md report
   - Loop until all criteria met

**State Management:**

- **Current State:** `STATE.md` tracks current phase, progress percentage, blockers, completed milestones
- **Phase Index:** `ROADMAP.md` lists all phases with status (pending, in-progress, complete)
- **Plans & Summaries:** Each phase directory contains PLAN.md (specification) and SUMMARY.md (results)
- **Config:** `.planning/config.json` stores model profiles, branching strategy, enabled features
- **Todos:** `.planning/todos/` tracks pending and completed work items
- **Git Integration:** Planning commits reference files changed, phase/plan summaries

## Key Abstractions

**Phase:**
- Purpose: Atomic unit of work with clear scope, dependencies, and deliverables
- Examples: `1`, `2.1`, `3a` (integer, decimal, letter-suffix numbering)
- Pattern: Phase directories named `{padded_number}-{slug}` containing PLAN.md and SUMMARY.md
- Properties: number, name, goal, status, dependencies, requirements, patterns

**Plan:**
- Purpose: Executable specification for a phase with breakdown into tasks
- Examples: `.planning/phases/1-setup/PLAN.md`
- Pattern: Markdown with frontmatter (phase, name, type, autonomous, wave) + objective + context + tasks + success criteria
- Task types: `auto` (fully autonomous), `checkpoint:*` (pause for user decision), `tdd` (test-driven)
- Structure: Multiple plans per phase (wave-based parallelization)

**Summary:**
- Purpose: Outcome document recording what was built, decisions, patterns, and deviations
- Examples: `.planning/phases/1-setup/SUMMARY.md`
- Pattern: Markdown with frontmatter matching plan structure + what was built + patterns established + decisions + deviations
- Consumed by: Plan checker, verifier, history digest

**Workflow:**
- Purpose: Orchestration script coordinating agents and tools for a user intent
- Examples: `.claude/get-shit-done/workflows/plan-phase.md`
- Pattern: Markdown with steps, command invocations, agent spawning, checkpoint logic
- Used by: User commands routed through dispatcher

**Agent:**
- Purpose: Claude instance with specialized role and knowledge
- Examples: `gsd-planner`, `gsd-executor`, `gsd-debugger`
- Pattern: Markdown with role definition, process steps, decision rules, tool usage
- Properties: name, color, tools available (Read, Write, Edit, Bash, Grep, Glob, WebFetch)

## Entry Points

**User Shell Dispatcher:**
- Location: `/gsd:do` (via orchestrator or Claude Code slash command)
- Triggers: User natural language input
- Responsibilities: Route intent to appropriate GSD command (plan-phase, execute-phase, debug, research-phase, etc.)

**Phase Planning Workflow:**
- Location: `.claude/get-shit-done/workflows/plan-phase.md`
- Triggers: `/gsd:plan-phase <phase>` command
- Responsibilities: Initialize research if needed, spawn planner, run plan checker, commit docs

**Phase Execution Workflow:**
- Location: `.claude/get-shit-done/workflows/execute-phase.md`
- Triggers: `/gsd:execute-phase <phase>` command
- Responsibilities: Load plan, execute tasks with checkpoint handling, create summary, update state

**Codebase Mapping:**
- Location: `.claude/get-shit-done/workflows/map-codebase.md`
- Triggers: `/gsd:map-codebase <focus>` command (focus: tech, arch, quality, concerns)
- Responsibilities: Analyze codebase, write ARCHITECTURE.md, STACK.md, TESTING.md, etc. to `.planning/codebase/`

**Progress Reporting:**
- Location: `.claude/get-shit-done/workflows/progress.md`
- Triggers: `/gsd:progress` command
- Responsibilities: Load state, roadmap, phases; render progress bar and summary

## Error Handling

**Strategy:** Multi-level error recovery with user decision gates for architectural changes

**Patterns:**

- **Rule 1: Auto-fix bugs** — Executor automatically fixes broken code without asking (wrong queries, logic errors, type errors, crashes)

- **Rule 2: Auto-add missing critical features** — Executor automatically adds missing error handling, validation, auth, indexes, logging

- **Rule 3: Auto-fix blocking issues** — Executor automatically resolves missing dependencies, broken imports, type errors, env vars, DB connection errors

- **Rule 4: Ask about architectural changes** — Executor pauses at checkpoints for user decision on major changes (new DB table, schema migration, new service layer, library switching, breaking API changes)

- **Validation Errors:** State/roadmap inconsistencies caught by `validate consistency` and `validate health` commands with optional repair

- **Plan Failures:** Plan checker identifies gaps (incomplete task coverage, missing success criteria verification) and routes to gap-closure planning loop

## Cross-Cutting Concerns

**Logging:** Bash command output captured and logged to phase directories; executor tracks all commits with hashes

**Validation:**
- Frontmatter schema validation on plan/summary files (required fields: phase, name, type, autonomous, wave)
- Phase numbering validation (decimal phases must be children of parent, archived phases filtered correctly)
- Roadmap/disk sync validation (phase dirs match ROADMAP.md entries)

**Authentication:**
- Git operations authenticated via git credential system
- No credential storage in `.planning/` (user's home git config handles auth)
- Agent tasks can pause with authentication gates (e.g., "GitHub API rate limit" → pause for user to authenticate)

**State Persistence:**
- All changes to STATE.md atomic via `gsd-tools state update` (prevents race conditions)
- WAITING.json signals support async checkpoint protocol (executor pauses, workflow can be resumed later)
- Git commits create audit trail of all planning and execution decisions

---

*Architecture analysis: 2026-03-20*
