# Tija Phase 2 (Quality) — Handoff Document

**Date:** 2026-10-22  
**Branch:** `feat/phase-2-quality`  
**Plan Reference:** `docs/superpowers/plans/2026-10-22-tija-phase-2-quality.md`  
**Spec Reference:** `docs/superpowers/specs/2026-10-22-tija-phases-design.md`

---

## 1. Executive Summary

Phase 2 ("Quality") has been implemented in its entirety following the implementation plan without introducing any external Go dependencies beyond the initial project set (`github.com/spf13/cobra`, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `gopkg.in/yaml.v3`).

Every task was developed with strict test-driven development (TDD failing tests first), reviewed by dedicated subagents prior to committing, and committed with the exact message format required by the plan.

---

## 2. Commit Log

The following commits form the Phase 2 implementation in `feat/phase-2-quality`:

1. `e77ffae feat(sys): stream combined process output and add git head/status paths`
2. `7359a31 feat(manifest): validate quality entries and record package.json scripts`
3. `451d44d feat(quality): add tool adapters and effective configuration resolution`
4. `67a2aa5 feat(quality): parse test reports and persist quality session`
5. `3168623 feat(engine): run quality steps with timeouts, logs and session results`
6. `731fecc refactor(doctor): check the tools of the effective quality configuration`
7. `f07ad6a feat(render): render quality reports and JSON error envelopes`
8. `edc3c3f feat(cli): add check, lint, format and test commands`
9. `093d92f feat(tui): add check and tests with live progress, warning mascot and scrolling`
10. `f15082b fix(status): treat configured and remote default branches as base branches`
11. `613ae16 docs: document quality commands`
12. `427d162 docs: add phase 2 quality implementation plan`

---

## 3. Key Design Decisions

### 3.1 Local Binary Resolution & Windows Shims
- **Precedence:** `node_modules/.bin/` is checked prior to searching system `PATH`.
- **Windows First-Class:** On Windows (`runtime.GOOS == "windows"`), `LookupBin` automatically looks for `.cmd` and `.exe` extensions (e.g. `biome.cmd`, `vitest.cmd`), ensuring npm/bun/pnpm/yarn shims function seamlessly in native Windows shells.
- **Zero Auto-Install:** If a tool is missing, Tija never attempts to run package managers automatically. Instead, it marks the step as `unavailable` and suggests an actionable hint (e.g., `run: bun install`, `install the .NET SDK: https://dot.net`).

### 3.2 Biome Grouping Optimization
- When consecutive steps use `biome` for `lint` and `format`, they are grouped into a single invocation:
  - `CheckMode`: `biome ci .`
  - `FixMode`: `biome check --write .`
- The results are mapped back to individual steps in `.tija/session/quality.json`.

### 3.3 Test Report Parsers
- **TRX Parser (`ParseTRX`):** Sums test counters (`total`, `passed`, `failed`, `skipped`) across all `*.trx` files found in the target directory (supporting .NET multi-project solutions). `notExecuted` is counted as `skipped`.
- **Jest/Vitest Parser (`ParseJestJSON`):** Reads `numTotalTests`, `numPassedTests`, `numFailedTests`, `numPendingTests` (plus `numTodoTests` into `skipped`).
- Both parsers use standard library `encoding/xml` and `encoding/json` with zero external dependencies.

### 3.4 Session Persistence & Uninitialized Repository Rule
- Results are merged by `Kind` into `.tija/session/quality.json`.
- Writes are performed atomically via temporary files and rename.
- **Uninitialized Repos:** In an uninitialized repository (no `.tija/manifest.yaml`), `tija check` runs using detection but never writes `.tija/` or logs to disk, and warns: `"not initialized — results are not saved (run: tija init)"`.
- **Fix Mode:** Fix commands (`tija format`, `tija lint --fix`) calculate changed files using SHA-256 hashes of `git status` porcelain paths before and after execution; they intentionally do not save quality session files because fixing code does not verify that checks pass.

### 3.5 Deterministic Exit Code Contract
- `0`: All steps passed (or conforming status).
- `1`: Check failure (test failure, lint violation, unformatted files, or `status --strict` nonconformance).
- `2`: Usage or environment error (missing/unavailable tool, invalid flag, not a git repository).
- Under `--json`, when an error occurs, the error is written to `stdout` using a compact `{"error":{"code":2,"message":"..."}}` envelope with an empty `stderr`.

### 3.6 Configurable & Remote Base Branches
- `DefaultBases` includes: `"main"`, `"master"`, `"dev"`, `"develop"`, `"desarrollo"`.
- `manifest.Git.BaseBranches` allows teams to declare custom base branches (e.g. `"trunk"`).
- `origin/HEAD` (`repo.DefaultRemoteBranch`) is dynamically detected and treated as a base branch.
- Base branches are never classified as nonconforming, and rename suggestions never suggest deleting published base branches on remote.

### 3.7 TUI Architecture & Mascot
- New `screenQuality` screen displays live step progress driven by `engine.RunQuality`'s `onEvent` callback over a channel and `tea.Cmd`.
- An animated spinner (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) ticks every 120ms during in-flight steps.
- New `Warning` mascot state displays an expressive 16x16 pixel cat sprite (8 terminal rows) with an exclamation mark in Sand (`#FED7AA`).
- Result screens implement line wrapping and vertical scroll slicing (`height - 6`) with keyboard navigation (`↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End`).

---

## 4. Verification & Validation Summary

### 4.1 Static Analysis
```sh
go vet ./...
# Result: 0 issues
```

### 4.2 Full Unit & Integration Test Suite
```sh
go test -count=1 ./...
# Result: PASS across all 15 packages
```
Packages tested:
- `cmd/tija`
- `internal/agents`
- `internal/assets`
- `internal/branch`
- `internal/cli`
- `internal/detect`
- `internal/engine`
- `internal/gitx`
- `internal/manifest`
- `internal/quality`
- `internal/render`
- `internal/sys`
- `internal/textdiff`
- `internal/tui`

### 4.3 End-to-End CLI Verification on Real Repository
- `go run ./cmd/tija check --json`
  - Output: Valid JSON report, exit code 2 (uninitialized repository, no tools configured).
- `go run ./cmd/tija doctor`
  - Output: Environment analysis, detected GitHub CLI, agents (`claude`, `codex`, `gemini`), manifest check.
- `go run ./cmd/tija status`
  - Output: Correctly classified branch `feat/phase-2-quality`, 10 commits ahead of `main`, exit code 0.

---

## 5. Review & Pending Items for Phase 3

1. **Go Toolchain Adapter (Future expansion):**
   - The current adapters support Biome, ESLint, Prettier, .NET, Vitest, Jest, and script fallbacks. While Go projects currently fallback to scripts or pass cleanly through doctor, a native `go test` and `golangci-lint` adapter can be added in a future phase using the exact same `quality.Adapter` interface.
2. **Merging to Main:**
   - The `feat/phase-2-quality` branch is ready for merge into `main` via PR.
