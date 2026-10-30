---
name: thermo-quality
description: Strict maintainability review for abstractions, file size limits, and spaghetti conditions.
source:
  project: thermos
  author: Cursor
  repo: https://github.com/cursor/plugins
  path: thermos/skills/thermo-nuclear-code-quality-review/SKILL.md
  commit: 1ba91fc25c0977741dd0e267849e6234b037ed0a
  license: MIT
  copyright: "Copyright (c) 2026 Cursor"
adapted:
  by: Tija
  date: 2026-10-29
  changes:
    - Removed Cursor subagent orchestration directives (disable-model-invocation, subagent_type, run_in_background).
    - Mapped output to Tija finding schema (file, line, severity Critical/High/Medium/Low, rule, suggestion).
    - Retained core structural principles (code judo, 1000-line ceiling, spaghetti prevention, canonical layer placement).
---

> Adaptado de thermos de Cursor (https://github.com/cursor/plugins, licencia MIT). Los cambios respecto del original están listados en el frontmatter.

# thermo-quality

Strict code quality and maintainability review. Focuses on structural health, abstraction elegance, and long-term codebase maintainability.

---

## Core Review Standards

### 0. Structural Simplification ("Code Judo")
Actively search for opportunities to simplify architecture. Do not stop at cosmetic refactoring. Look for restructurings that eliminate whole branches, helper abstractions, modes, or conditionals while preserving exact behavior. Prefer deleting complexity rather than rearranging it.

### 1. The 1000-Line Ceiling
Do not allow a PR to push a file from under 1,000 lines to over 1,000 lines without exceptional, documented justification. Decompose expanding files into focused helpers, subcomponents, or dedicated modules before merging.

### 2. Prevent Spaghetti Conditionals
Reject ad-hoc, scattered `if` branches or special cases inserted into existing flows. When new variants or behavior arise, encapsulate them cleanly in dedicated abstractions, state machines, policy objects, or strategy handlers rather than tangling shared code paths.

### 3. Clean Design Over "It Works"
Working code is the baseline, not the finish line. Do not accept implementations that add architectural debt. Strongly prefer simplifications that remove moving pieces altogether over refactors that merely redistribute complexity.

### 4. Direct, Maintainable Code Over "Magic"
Flag brittle, ad-hoc, or overly clever mechanisms. Thin abstractions or pass-through wrappers that add indirection without improving clarity should be removed.

### 5. Type and Boundary Cleanliness
Question loose types (`any`, `unknown`, or heavy type assertions) at domain boundaries. Explicit models and shared contracts keep boundaries reliable and self-documenting.

### 6. Canonical Layer Placement
Ensure domain and feature logic resides in the canonical layer. Prevent feature-specific assumptions from leaking into shared infrastructure or general-purpose utilities.

### 7. Clean Orchestration
Flag unnecessary sequential blocking where independent operations can execute concurrently. Require atomic state transitions rather than partial updates that can leave systems inconsistent on failure.

---

## Finding Schema

Findings must follow Tija's structured audit schema:

- `file`: Path to the affected file.
- `line`: Line number in the diff.
- `severity`: `Critical` | `High` | `Medium` | `Low`
- `rule`: Category of rule (`code-judo`, `1000-lines-ceiling`, `spaghetti-condition`, `clean-design`, `type-boundary`, `canonical-layer`, `clean-orchestration`).
- `suggestion`: Concrete structural suggestion.
