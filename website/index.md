---
layout: home
customHero: true

features:
  - title: "8K tokens to 50"
    details: "--fields, --jq, and --preset strip Confluence's verbose responses down to exactly what you need."
  - title: Drop-in Agent Skill
    details: "Ships with SKILL.md. Claude Code, Cursor, Codex, Gemini CLI -- agents learn the CLI in one read."
  - title: CQL Search
    details: "cf search with CQL queries. Filter by space, type, label, last modified. Structured JSON results."
  - title: Workflow Commands
    details: "cf workflow move, copy, publish, archive, comment, restrict. Simple flags, no raw JSON."
  - title: Templates
    details: "cf pages create --template meeting-notes --var title='Q1 Review'. Built-in patterns with variables."
  - title: Version Diff
    details: "cf diff --id 12345 --since 2h. See what changed, structured JSON, not a wall of text."
  - title: Real-time Watch
    details: "cf watch --cql 'space = DEV' --interval 30s. NDJSON stream of content changes."
  - title: Export & Tree
    details: "cf export --id 12345 --tree. Recursively export page trees as NDJSON."
  - title: 242 Commands
    details: "Auto-generated from Confluence's OpenAPI spec. Every endpoint is a CLI command, always in sync."
---

<!-- Why cf section -->
<div class="why-section">
  <h2>Why another Confluence CLI?</h2>
  <div class="why-grid">
    <div class="why-card">
      <div class="why-label">Other CLIs</div>
      <div class="why-content">Human-readable tables. Agents can't parse them. Manual flag lookup. Breaks when Confluence updates APIs.</div>
    </div>
    <div class="why-card highlight">
      <div class="why-label">cf</div>
      <div class="why-content">JSON everywhere. Agents read it natively. Ships with SKILL.md -- agents learn it in one read. Auto-generated from OpenAPI -- never out of date.</div>
    </div>
  </div>
</div>

<!-- Killer demos -->
<div class="terminal-section">
  <h2 class="section-title">See it in action</h2>

  <div class="terminal-window">
    <div class="terminal-header">
      <div class="terminal-dots">
        <span class="dot red"></span>
        <span class="dot yellow"></span>
        <span class="dot green"></span>
      </div>
      <div class="terminal-title">templates -- create pages from patterns</div>
    </div>
    <div class="terminal-body">

```bash
cf pages create --template meeting-notes \
  --spaceId 123456 \
  --var title="Q1 Review" \
  --var date="2026-03-28"
```

  </div>
  </div>

  <div class="terminal-window">
    <div class="terminal-header">
      <div class="terminal-dots">
        <span class="dot red"></span>
        <span class="dot yellow"></span>
        <span class="dot green"></span>
      </div>
      <div class="terminal-title">watch -- real-time NDJSON stream</div>
    </div>
    <div class="terminal-body">

```bash
cf watch --cql "space = DEV AND type = page" --interval 30s
```

```json
{"event":"initial","id":"12345","title":"Deploy Runbook","version":3}
{"event":"updated","id":"12345","title":"Deploy Runbook","version":4}
```

  </div>
  </div>

  <div class="terminal-window">
    <div class="terminal-header">
      <div class="terminal-dots">
        <span class="dot red"></span>
        <span class="dot yellow"></span>
        <span class="dot green"></span>
      </div>
      <div class="terminal-title">diff -- structured version comparison</div>
    </div>
    <div class="terminal-body">

```bash
cf diff --id 12345 --since 2h
```

```json
{"version_from":3,"version_to":5,"title_changed":true,"body":{"added":12,"removed":4}}
```

  </div>
  </div>
</div>
