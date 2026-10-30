---
name: thermo-review
description: Comprehensive security and correctness audit of diff changes.
source:
  project: thermos
  author: Cursor
  repo: https://github.com/cursor/plugins
  path: thermos/skills/thermo-nuclear-review/SKILL.md
  commit: 1ba91fc25c0977741dd0e267849e6234b037ed0a
  license: MIT
  copyright: "Copyright (c) 2026 Cursor"
adapted:
  by: Tija
  date: 2026-10-29
  changes:
    - Removed Cursor subagent orchestration directives (disable-model-invocation, subagent_type, run_in_background).
    - Generalized PR comments inspection to gh CLI performed strictly after independent audit.
    - Mapped output to Tija finding schema (file, line, severity Critical/High/Medium/Low, rule, suggestion).
    - Preserved diff-only scope and over-reporting calibration guidelines.
---

> Adaptado de thermos de Cursor (https://github.com/cursor/plugins, licencia MIT). Los cambios respecto del original están listados en el frontmatter.

# thermo-review

Comprehensive security, correctness, and reliability audit of code changes in a branch or pull request diff.

## Scope

- **Diff only:** ONLY report issues related to code that is being ADDED or MODIFIED in this branch/PR diff.
- **Untouched code:** DO NOT report pre-existing vulnerabilities or issues in surrounding code that is not being modified.

---

## Guidelines

### Breaking Functionality
Trace potential side effects across packages and modules. Simple changes in one layer often cause subtle breakage elsewhere. Trace end-to-end data flows affected by modified interfaces or contracts.

### Breaking Developer Experience (Devex)
Catch changes that disrupt developers' ability to run, build, or test code locally:
- Modifying how secrets are read or where they are stored.
- Adding or modifying required environment variables without defaults or documentation.
- Remapping ports, network bindings, or service endpoints.
- Introducing required prerequisite scripts for normal workflows.

### Feature Gate Leaks
Ensure that features intended to be guarded behind feature flags or internal-only checks do not inadvertently leak into public routes, exposed APIs, or unauthenticated contexts.

### Intended Breakage
If a high-risk change is clearly the intended goal of the PR (e.g. deprecating an old endpoint, removing a safeguard intentionally) and the scope is strictly constrained, do not create unnecessary noise. Report only if unintended collateral damage or malicious risk is suspected.

### Over-reporting Guidelines
Calibrate severity honestly. Marking non-critical issues as High or Critical creates alert fatigue and destroys reviewer trust. Be thorough and confirm issues end-to-end before reporting.

---

## Critical Rules

1. **No unfinished research:** Never report speculative issues that could be verified directly by inspecting accessible codebase files.
2. **Independent audit first:** If a PR is open on GitHub, any inspection of comments via `gh pr view --comments` must occur strictly **after** completing the independent audit to preserve fresh review perspective.

---

## Finding Schema

Findings must follow Tija's structured audit schema:

- `file`: Path to the affected file.
- `line`: Line number in the diff.
- `severity`: `Critical` | `High` | `Medium` | `Low`
- `rule`: Category of standard (`breaking-functionality`, `devex-regression`, `feature-leak`, `security-vulnerability`).
- `suggestion`: Concise, actionable remediation.
