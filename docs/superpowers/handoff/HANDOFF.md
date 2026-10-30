# Tija Phase 3a (Skill Packs, Comment Cleaning & Workflows) — Handoff Document

**Date:** 2026-10-29  
**Branch:** `feat/phase-3a-skill-packs`  
**Plan Reference:** `docs/superpowers/plans/2026-10-29-tija-phase-3a-skill-packs.md`  
**Spec Reference:** `docs/superpowers/specs/2026-10-29-tija-phase-3a-skill-packs-design.md`

---

## 1. Executive Summary

Phase 3a ("Skill packs, limpieza de comentarios y workflows compuestos") has been implemented in its entirety following the implementation plan and design specifications with zero external dependencies added beyond promoting `golang.org/x/sync` from indirect to direct.

The phase successfully establishes:
1. **Third-Party Skill Packs with Strict Attribution:** Embedded and versioned under `.tija/skills/`, complete with original MIT licenses, explicit frontmatter provenance metadata, and top-level `THIRD_PARTY_NOTICES.md` included in release artifacts.
2. **`tija clean comments`:** AST-independent tokenization for JS, TS, JSX, and TSX files. Applies Biome flag semantics (report, `--write` for safe, `--write --unsafe` for risky changes) with a double-safety net: Go token verification (guaranteeing that code tokens remain 100% untouched) and Biome diagnostics differential checking (restoring files if regressions are detected).
3. **Agent Attribution Stripping:** Deterministic regex-based removal of AI signatures (`🤖 Generated with...`, `Co-Authored-By: Claude...`, `Made with Cursor`, etc.) from commit logs feeding PR prompts and from final generated pull request descriptions, alongside style enforcement using the adapted `antislop-copywriting` skill.
4. **In-Process Composed Workflows (`tija run`):** Direct execution of validated Tija commands sequentially without spawning shell interpreters or subshells, featuring the default `pre-pr` workflow (`clean comments --write` → `check` → `pr prepare`).
5. **Interactive TUI Additions:** Main menu additions for `Clean comments` (scoped to JS/TS) and `Workflows`, with live progress reporting and animated cat mascot state transitions.

Every task adhered to strict Test-Driven Development (TDD with failing tests first) and was committed with the exact message format required by the plan.

---

## 2. Commit Log

The following commits form the Phase 3a implementation in `feat/phase-3a-skill-packs`:

1. `33c6c15 docs: add phase 3a skill packs design and implementation plan`
2. `93515ad feat(assets): add attributed skill packs and third-party notices`
3. `75ca827 feat(manifest): add skills, comments and agent attribution settings`
4. `10fa7c7 feat(skills): list skill packs with origin and license`
5. `1243393 feat(comments): add JS/TS/JSX lexer`
6. `d499ff8 feat(comments): extract, protect, plan and verify comment edits`
7. `591d885 feat(quality): read per-file Biome diagnostics for post-edit checks`
8. `7b305f3 feat(comments): classify comments through the agent as JSON`
9. `08ff5b1 feat(engine): clean comment slop with verified edits`
10. `c42fde8 feat(pr): keep agent attribution and AI phrasing out of pull requests`
11. `51cd572 feat(cli): add tija clean comments`
12. `7b515e7 feat(workflows): run composed Tija commands with tija run`
13. `4876c6d feat(tui): add clean comments and workflows screens`

---

## 3. Key Design Decisions

### 3.1 Strict Third-Party Attribution & Notices (Restriction 11)
- **Provenance & Integrity:** All third-party skills (`antislop-code`, `antislop-copywriting`, `thermo-review`, `thermo-quality`) include complete frontmatter detailing `project`, `author`, `repo`, `path`, `commit`, `version`, `license`, and `copyright`.
- **Visible Notice:** The very first line of each markdown body contains the explicit notice:  
  `> Adaptado de <proyecto> de <autor> (<repo>, licencia MIT). Los cambios respecto del original están listados en el frontmatter.`
- **License Co-location:** Original verbatim `LICENSE` files accompany each skill under `.tija/skills/<pack>/LICENSE`.
- **Distribution:** `THIRD_PARTY_NOTICES.md` is positioned at the repository root and bundled into release archives via `.goreleaser.yaml`.
- **Validation in Doctor:** `tija doctor` scans skills with `skills.List` and emits `StatusWarn` (`!`) if any third-party pack is missing its companion `LICENSE` or provenance metadata.

### 3.2 Pure Go Lexer with Uncertainty Bailout
- **Zero cgo / Zero Tree-sitter:** A custom, high-speed lexer (`internal/comments/lexer.go`) handles JavaScript, TypeScript, and JSX without requiring external dependencies or C libraries.
- **Regex vs Division Disambiguation:** Resolves slash ambiguity based on preceding punctuator or keyword context (e.g. `return`, `typeof`, `case`, `await`, etc.).
- **JSX Distinction:** Accurately distinguishes code, string literals, and JSX child text nodes (`<p>// not a comment</p>`). Block comments nested within JSX expression children (`{/* ... */}`) are identified with `InJSXChild = true` and their outer brace spans are recorded.
- **Fail-Safe Principle (`ErrUncertain`):** If the lexer encounters unclosed literals, unbalanced JSX, or unclosed comments, it returns `ErrUncertain`. Tija immediately skips the file rather than guessing.

### 3.3 Two-Tier Safety Net: Token Verification & Biome Diagnostics
Tija will never write an edited source file unless it passes two independent verification gates:
1. **Gate 1 — Token Verification (`VerifyTokens`):** Original and modified sources are lexed, comments are stripped, and JSX text nodes are normalized using React's whitespace rules (trimming and collapsing empty lines). The token streams must match 1:1. If even a single semicolon or token is dropped or shifted, verification fails and the file is untouched on disk.
2. **Gate 2 — Biome Diagnostics (`BiomeDiagnostics` & `Regressions`):** When Biome is configured in the project, Tija executes `biome check --reporter=github --max-diagnostics=none --colors=off` before and after writes. If any new diagnostic categories appear or existing diagnostic counts increase (e.g., an unintended removal of a `biome-ignore`), the file is immediately restored to its original state from memory and marked as `restored`.

### 3.4 Biome Flag Semantics for Code Edits (Restriction 10)
- **No `--remove-all`:** Per design decisions, blanket comment removal is disallowed to prevent accidental loss of business requirements, API contracts, or type definitions.
- **Three-Tier Writing:**
  - `tija clean comments`: Read-only reporting. Exits `1` if safe changes exist, `0` if clean.
  - `tija clean comments --write`: Applies only **safe** changes (pure separators, empty lines, obvious end-markers, and agent-classified high-confidence slop).
  - `tija clean comments --write --unsafe`: Applies both safe and **unsafe** changes (rewrites, commented-out code, low-confidence suggestions).
- **Atomic File Writing:** Edits are written to a temporary file within the same directory and replaced via atomic `os.Rename`. CRLF line endings and UTF-8 BOM are strictly preserved.
- **Untracked File Protection:** Files not tracked by git are skipped with a warning unless `--force` is specified.

### 3.5 Deterministic Agent Attribution Stripping
- **Commit History Hygiene:** In `internal/engine/pr_context.go`, commit message bodies are stripped of agent signatures (`🤖 Generated with...`, `Co-Authored-By: Claude...`, `Made with Cursor`, etc.) before entering PR prompt assembly, preventing the model from regurgitating agent trailers.
- **Draft Stripping:** In `draftPR`, returned draft bodies are stripped of attribution lines before normalization.
- **Governance Check:** `prrules.CheckAttribution` runs on final PR drafts and flags any surviving attribution lines as warnings (`!`) without failing validation.
- **Human Author Protection:** Legitimate co-authors (e.g., `Co-Authored-By: Ana <ana@empresa.com>`) are never touched.

### 3.6 In-Process Composed Workflows (`tija run`)
- Workflows defined under `.tija/workflows/*.yaml` execute directly in-process via fresh Cobra root command instances without spawning shell processes (`sh`, `bash`, `cmd`, `powershell`).
- Only pre-approved Tija commands (`Allowed`) may be called. Arbitrary shell execution and nested `run` calls are rejected with clear validation errors.
- Default `pre-pr.yaml` chains comment cleaning, quality verification, and PR draft preparation:
  ```yaml
  name: pre-pr
  description: Clean comment slop, run quality checks and prepare the PR
  steps:
    - run: clean comments --write
    - run: check
    - run: pr prepare
  ```

### 3.7 Interactive TUI Integration
- **Context-Aware Menu:** `Clean comments` appears only when JavaScript/TypeScript is detected; `Workflows` appears only when at least one valid workflow exists; `Review` is hidden until Phase 4.
- **Mascot Choreography:** Displays `Scanning` during comment extraction and `Thinking` while awaiting LLM classification batches.
- **Safety Prompts:** Pressing `w` applies safe fixes immediately; pressing `u` triggers an explicit confirmation prompt (`Apply unsafe changes too? (y/n)`).

---

## 4. Verification & Validation Summary

### 4.1 Static Analysis
```sh
go vet ./...
# Result: 0 issues across all packages
```

### 4.2 Full Test Suite Execution
```sh
go test ./...
# Result: PASS across all 18 packages
```
Tested packages:
- `cmd/tija`
- `internal/agents`
- `internal/assets`
- `internal/branch`
- `internal/cli`
- `internal/comments`
- `internal/detect`
- `internal/engine`
- `internal/gitx`
- `internal/manifest`
- `internal/prrules`
- `internal/quality`
- `internal/render`
- `internal/skills`
- `internal/sys`
- `internal/testutil`
- `internal/textdiff`
- `internal/tui`
- `internal/workflows`

### 4.3 Binary Footprint Check
```sh
go build -ldflags="-s -w" -o tija.exe ./cmd/tija
Get-Item tija.exe | Select-Object Length
# Result: 7,190,016 bytes (6.85 MB)
```
The final binary size is 6.85 MB, well below the 7.5 MB budget, confirming that zero heavy dependencies were introduced.

---

## 5. Review, Open Questions & Stoppers for Phase 4

### 5.1 Ready Infrastructure for Phase 4 (Thermos Review)
1. **Packs Already Installed:** Both `thermo-review` and `thermo-quality` are fully authored, attributed, embedded in `assets`, and automatically installed by `tija init`.
2. **Review Synthesis Design:** The review workflow can directly consume the `skills.Load` API, the `quality.StepResult` session, and the multi-provider `agents.Generate` runner developed in Phase 3/3a.
3. **Workflow Integration:** The `pre-pr` workflow is designed to easily insert `run: review` ahead of `pr prepare` once Phase 4 is implemented.

### 5.2 Open Questions & Observations
1. **GitHub CLI Auth in CI Workflows:**
   - In environments where GitHub CLI (`gh`) is present but unauthenticated, `tija run pre-pr` will run `clean comments` and `check`, and prepare a PR draft locally (`tija pr prepare`). If `tija pr create` is added to a workflow, CI requires `GH_TOKEN` configured.
2. **Expansion to C# / .NET:**
   - The comment cleaner currently targets JS/TS/JSX/TSX. For C# and .NET solutions, a future iteration can add Roslyn-based comment extraction using the same `PostCheck` and `CleanReport` abstractions.
3. **Biome GitHub Reporter Formatting:**
   - Biome v2.4.4 prints `--reporter=github` cleanly without path escaping bugs. Should Biome adjust this format in future major versions, `ParseBiomeGithub` safely falls back to returning `ok = false`, causing Tija to report that post-check diagnostics were skipped rather than panicking or misreporting.

### 5.3 Stoppers
- **None.** Phase 3a is complete, tested, and ready for review and merge into `main`.
