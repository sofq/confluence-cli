# Phase 12: Internal Utilities - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 12-internal-utilities
**Areas discussed:** Preset three-tier architecture, Built-in preset content, Adoption strategy, Duration conventions

---

## Preset Three-Tier Architecture

### Built-in preset storage

| Option | Description | Selected |
|--------|-------------|----------|
| Embedded Go map | Built-in presets defined as Go map[string]string in source code, compiled into binary | ✓ |
| Embedded JSON file | Built-in presets in JSON file via Go embed directive | |
| External config file | Ship a default presets file alongside the binary | |

**User's choice:** Embedded Go map
**Notes:** Same pattern as jr. No external files needed.

### User preset file location

| Option | Description | Selected |
|--------|-------------|----------|
| ~/.config/cf/presets.json | Same config directory as config.json, mirroring jr's pattern | ✓ |
| Alongside config.json | In the exact same directory as the active config.json file | |

**User's choice:** ~/.config/cf/presets.json
**Notes:** Uses os.UserConfigDir() with fallback, mirrors jr.

### Preset value type

| Option | Description | Selected |
|--------|-------------|----------|
| Pure string (JQ only) | Preset is just a JQ expression string, matches existing config.Profile.Presets | ✓ |
| Struct with JQ + metadata | Preset struct with JQ expression + description field | |

**User's choice:** Pure string (JQ only)
**Notes:** Simpler than jr's Preset struct, matches existing config format.

---

## Built-in Preset Content

### JQ expression design approach

| Option | Description | Selected |
|--------|-------------|----------|
| Claude designs from API shape | Claude examines Confluence v2 API response schemas and designs expressions during planning | ✓ |
| Mirror jr presets | Start from jr's patterns, adapt for Confluence | |
| Define expressions now | User specifies exact JQ expressions during discussion | |

**User's choice:** Claude designs from API shape
**Notes:** 7 presets (brief, titles, agent, tree, meta, search, diff) designed during planning.

---

## Adoption Strategy

### jsonutil adoption scope

| Option | Description | Selected |
|--------|-------------|----------|
| Create + refactor all | Create internal/jsonutil AND update all 12+ existing call sites | ✓ |
| Create package only | Create package but don't touch existing code | |
| Create + refactor cmd/ only | Refactor command files only, leave internal/ packages | |

**User's choice:** Create + refactor all
**Notes:** Full consolidation in one shot.

### Preset wiring

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, wire it in | Update cmd/root.go to use preset.Lookup() with three-tier chain | ✓ |
| Package only, wire later | Create internal/preset but don't change cmd/root.go | |

**User's choice:** Yes, wire it in
**Notes:** Phase is truly "done" when the package is both built and integrated.

---

## Duration Conventions

### Time conventions

| Option | Description | Selected |
|--------|-------------|----------|
| Calendar time | 1d = 24h, 1w = 7d = 168h (per UTIL-02 requirement) | ✓ |
| Work time (like jr) | 1d = 8h, 1w = 5d = 40h | |

**User's choice:** Calendar time
**Notes:** Makes sense for Confluence content monitoring — "changes in last 2 days" = 48 real hours.

### Return type

| Option | Description | Selected |
|--------|-------------|----------|
| time.Duration | Go's standard time.Duration type, integrates with time.Now().Add(-d) | ✓ |
| Seconds as int (like jr) | Returns int seconds, requires conversion at call sites | |

**User's choice:** time.Duration
**Notes:** Per UTIL-02 requirement: "return correct time.Duration values".

### Month unit support

| Option | Description | Selected |
|--------|-------------|----------|
| No, w/d/h/m only | Support weeks, days, hours, minutes only | ✓ |
| Yes, 1M = 30d | Add month support with 30-day approximation | |

**User's choice:** No, w/d/h/m only
**Notes:** Months are ambiguous and UTIL-02 only mentions h/d/w.

---

## Claude's Discretion

- Exact JQ expressions for the 7 built-in presets
- Internal helper functions and error message wording
- Test case selection and organization
- Whether jsonutil also exposes an Encoder helper

## Deferred Ideas

None — discussion stayed within phase scope
