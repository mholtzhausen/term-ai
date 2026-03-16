---
name: version-bump
description: Bump the version of this project by editing the VERSION variable in the Makefile. Handles patch/build bumps, minor bumps, and major bumps with correct semver label rules. USE FOR: "bump version", "bump minor", "bump major", "release", "increment version", "new version". DO NOT USE FOR: release tagging, git operations, or changelog management.
---

# Version Bump Skill

## Overview

Modifies the `VERSION` variable at the top of `Makefile` to produce the next semver version.  
Version format: `v[major].[minor].[patch][-label]` (e.g. `v1.2.3`, `v1.2.3-alpha`, `v0.9.3-rc2`).

---

## Rules

### Label Preservation
- A pre-release label (`-alpha`, `-beta`, `-rc1`, `-rc2`, etc.) is **preserved as-is** on every bump *unless* the user explicitly requests it be added or removed.
- Only a **minor bump** or **major bump** strips all labels automatically.

### Bump Types

| Trigger phrase | Action |
|---|---|
| "bump version" / "bump" / "bump build" / "bump patch" | Increment **patch** number; keep label unchanged |
| "bump minor" / "minor bump" | Increment **minor**, reset patch → `0`, **remove all labels** |
| "bump major" / "major bump" | Increment **major**, reset minor → `0`, reset patch → `0`, **remove all labels** |
| "add label `<x>`" | Append `-<x>` to current version (do not change numbers) |
| "remove label" / "strip label" | Remove the label from current version (do not change numbers) |

---

## Step-by-Step Workflow

1. **Read** `Makefile` (top of file) to find the current `VERSION` value.
2. **Parse** the version: extract `major`, `minor`, `patch`, and optional `label`.
3. **Apply** the correct bump rule from the table above.
4. **Write** the new version back to `Makefile`, replacing only the `VERSION=` line.
5. **Confirm** by showing the user: `v<old> → v<new>`.

---

## Examples

Given current version `v0.9.3-alpha`:

| User says | Result |
|---|---|
| "bump version" | `v0.9.4-alpha` |
| "bump minor" | `v0.10.0` |
| "bump major" | `v1.0.0` |
| "add label rc1" | `v0.9.3-rc1` |
| "remove label" | `v0.9.3` |

Given current version `v1.2.0`:

| User says | Result |
|---|---|
| "bump" | `v1.2.1` |
| "bump minor" | `v1.3.0` |
| "bump major" | `v2.0.0` |
| "add label beta" | `v1.2.0-beta` |

---

## Constraints

- The `VERSION` value **must always** start with `v` (required for Go `-ldflags` injection).
- Do **not** modify anything else in `Makefile`.
- Do **not** create git tags, commits, or changelogs — version file edit only.
- If the bump type is ambiguous, default to **patch bump** and note the assumption.
