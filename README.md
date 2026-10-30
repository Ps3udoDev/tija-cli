# Tija

> Your repository. Your rules. Any agent.

Tija is a CLI that prepares a repository to work with AI coding agents: it detects your stack, writes a versioned `.tija/` folder (manifest, rules, templates, workflows, skill packs), verifies the health of your environment and branches, and provides deterministic quality commands (`check`, `lint`, `format`, `test`), safe comment cleaning (`clean comments`), and composed workflows (`run`). Tija never installs tools automatically; it operates deterministically with your local repo tools or binaries on `PATH`.

*Nota: la interfaz está en inglés; el contenido generado usa `content.language` (por defecto `es`).*

## Install

```sh
go install github.com/Ps3udoDev/tija-cli/cmd/tija@latest
```

Or download a binary for Windows, Linux or macOS from the [Releases](https://github.com/Ps3udoDev/tija-cli/releases) page.

## Commands

| Command | What it does |
|---|---|
| `tija init` | Detects the stack and template, creates `.tija/` (including default skill packs and workflows) and updates `.gitignore`. Idempotent. |
| `tija doctor` | Checks git, `gh` (and its login), the agent CLIs on `PATH`, the manifest, effective quality tools, skill packs attribution, workflows, and PR template. |
| `tija status` | Classifies the current branch against naming rules, shows ahead/behind against the PR target, the working tree and the remote. |
| `tija check` | Runs all configured or detected quality checks (build, lint, format, tests). |
| `tija lint` | Runs linter checks. Pass `--fix` to run lint + format in fix mode. |
| `tija format` | Formats source code in fix mode. |
| `tija test` | Runs the test suite and parses test result reports (TRX, Jest/Vitest JSON). |
| `tija clean comments` | Cleans AI-slop comments in JS/TS/JSX with Biome flag semantics (`--write`, `--unsafe`). |
| `tija skills list` | Lists installed and embedded skill packs with origin, license, author, and commit. |
| `tija skills show <name>` | Displays a skill pack's content, origin, and attribution. |
| `tija run <name>` | Runs a composed workflow defined in `.tija/workflows/*.yaml` in-process. |
| `tija run --list` | Lists available workflows. |
| `tija pr prepare` | Drafts the pull request title and body for the current branch (see Pull requests). |
| `tija pr create` | Pushes the branch and opens or updates the pull request with `gh`. |

Flags:

- `--json` machine-readable output (no colors, no TUI). In case of usage or execution errors, outputs a structured `{"error":{"code":2,"message":"..."}}` envelope to stdout.
- `--yes` approves every action that needs consent (Tija never edits existing files outside `.tija/` without it).
- `--no-color` plain text (also honored via `NO_COLOR`, and automatically when stdout is not a TTY).
- `status --strict` exits `1` when the branch is nonconforming or detached.
- Quality flags (`check`, `lint`, `format`, `test`):
  - `--timeout duration` sets the maximum duration for the entire run (default: `10m`).
  - `--verbose` streams combined process output in real time to `stderr` (ignored under `--json`).

Exit codes:
- `0`: All checks passed.
- `1`: Check failure (e.g. linter error, failing tests, unformatted code, pending changes in check/report mode).
- `2`: Usage error or missing environment dependencies (e.g. unavailable tool, syntax error, not a git repository).

## Clean Comments

`tija clean comments [paths...]` detects and removes AI-generated boilerplate, decorative separators, and noisy narration comments while preserving business rules, architecture rationale, and tool directives (`biome-ignore`, `@ts-*`, licenses, JSDoc types).

Flag semantics match Biome:
- `tija clean comments [paths...]`: Report-only mode. Prints findings and exits `1` if safe changes are pending (`0` if clean).
- `--diff`: Displays unified diffs of proposed changes.
- `--write`: Applies only **safe** changes (decorative separators, empty labels, workflow narration, obvious end markers).
- `--write --unsafe`: Applies both safe and **unsafe** changes (rewrites, commented-out code, low-confidence removals).
- `--unsafe` (without `--write`): Expands report and `--diff` to include unsafe changes.
- `--all`: Scans all files matching `comments.include` and `comments.exclude`. (Default scope without paths: files changed against PR target + untracked files).
- `--no-agent`: Runs only deterministic rules without querying AI agent CLIs.
- `--force`: Includes untracked git files (untracked files are skipped by default).
- `--concurrency N`: Sets concurrent agent classification batches (default `2`).
- `--agent-timeout`: Bounding duration for agent batches (default `3m`).

Safety & Verification:
- **Token Verification:** Before saving, Tija tokenizes original and edited source files (discarding comments and normalizing JSX whitespace) and verifies that AST code tokens are 100% identical. Any code change causes an immediate verification failure; the file is never written.
- **Biome Diagnostics Post-check:** When Biome is configured for lint/format, Tija runs `biome check --reporter=github` before and after writes. If any new or increased diagnostics occur, the file is automatically restored to its original state.

## Workflows

Composed workflows automate multi-step procedures directly in-process without spawning subshells or subprocesses.

- Stored in `.tija/workflows/*.yaml` (e.g. `pre-pr.yaml`).
- Default `pre-pr` workflow:
  1. `clean comments --write`
  2. `check`
  3. `pr prepare`
- Only allowed Tija commands can be steps (`check`, `lint`, `format`, `test`, `status`, `doctor`, `clean comments`, `pr prepare`, `pr create`). Arbitrary shell commands and nested `run` invocations are strictly rejected.
- Step execution stops on the first non-zero exit unless `continueOnError: true` is configured for that step.

## Skill Packs & Third-Party Material

Tija embeds and installs modular skill packs under `.tija/skills/<pack>/SKILL.md`:
- `antislop-code`: Comment classification rubrics (adapted from [anti-slop](https://github.com/miqdadbadjuber/anti-slop)).
- `antislop-copywriting`: PR technical writing style guidelines (adapted from [anti-slop](https://github.com/miqdadbadjuber/anti-slop)).
- `thermo-review`: Correctness, bug hunting, and security review rubrics (adapted from [Cursor Thermos](https://github.com/cursor/plugins/tree/main/thermos)).
- `thermo-quality`: Maintainability, code judo, and 1000-line limit rubrics (adapted from [Cursor Thermos](https://github.com/cursor/plugins/tree/main/thermos)).

All third-party materials are licensed under the MIT License and fully attributed to their original authors with companion `LICENSE` files and frontmatter source references. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for full notices and license texts.

## Pull requests

| Command | What it does |
|---|---|
| `tija pr prepare` | Drafts the title and body for the current branch against its target and saves them to `.tija/session/pr.md`. Pushes nothing. |
| `tija pr create` | Prepares (or reuses the saved draft for this commit), then pushes the branch and opens the pull request with `gh`, or updates the open one. |

Flags (both commands):

- `--agent <claude|codex|gemini|none>` picks the agent that writes the text (default: `agents.default` of the manifest). `--no-agent` is `--agent none`.
- `--no-check` never runs `tija check` to collect test results.
- `--agent-timeout 3m` and `--check-timeout 10m` bound each agent attempt and each check step.
- `prepare --print-prompt` prints the prompt that would be sent to the agent and exits `0` without running an agent or check.
- `create --draft` opens the pull request as a draft.
- `create --edit` opens the saved draft in `git var GIT_EDITOR` and revalidates it before asking for confirmation (needs a terminal and an initialized repo).

Agent Attribution Stripping:
- In `content.agentAttribution` (default enabled: `strip: true`), Tija automatically removes agent attribution signatures (e.g. `🤖 Generated with...`, `Co-Authored-By: Claude...`, `Made with Cursor`) from commit bodies entering the prompt and from generated draft bodies.

Anti-invention rule: Tija computes the branch, target, commits, diffstat, issue number and test results; the agent only writes prose. `## Pruebas` cites only results of `.tija/session/quality.json` for the current commit on a clean tree (or a `check` run made in the same execution); otherwise it says no verified results exist. `## Evidencias` is `N/A` unless you edit it.

## Quality & Session

Tija automatically resolves the effective quality tool configuration:
1. Manifest overrides in `.tija/manifest.yaml` under `quality:`.
2. Automatic stack detection (e.g., Biome, ESLint, Prettier, .NET, Vitest, Jest).
3. Fallback to `package.json` scripts (`lint`, `test`).

Supported tool adapters:
- **Biome**: Groups lint and format into single invocations (`biome ci .` in check mode, `biome check --write .` in fix mode).
- **ESLint & Prettier**: Node-based linting and formatting.
- **.NET SDK**: `dotnet build`, `dotnet format`, and `dotnet test` with TRX report parsing.
- **Vitest & Jest**: Runs tests with `--json` reporter and parses passed, failed, and skipped counts.
- **Script fallback**: Runs `run <script>` via detected package manager (`bun`, `pnpm`, `yarn`, `npm`).

Results are saved to `.tija/session/quality.json` when the repository is initialized (`.tija/manifest.yaml` present). Individual step logs are stored in `.tija/session/logs/<kind>.log`. Uninitialized repos run checks using detection without saving sessions or writing to disk.

## Base Branches

Branch classification respects:
- Built-in default base branches: `main`, `master`, `dev`, `develop`, `desarrollo`.
- Configured base branches in `.tija/manifest.yaml` under `git.baseBranches: [...]`.
- The remote repository's default branch on `origin` (`refs/remotes/origin/HEAD`).

Base branches are never classified as nonconforming, and renaming suggestions never suggest deleting published base branches.

## TUI

Run `tija` with no arguments in a terminal to open the interactive terminal UI:
- Menu options: `Pull Request`, `Clean comments` (JS/TS projects), `Check`, `Tests`, `Workflows` (when workflows exist), `Doctor`, `Status`, and `Init`.
- Live progress screens with real-time spinner, step status, and live event updates.
- Expressive pixel cat mascot: `Idle`, `Scanning`, `Thinking`, `Success`, `Warning` (!), and `Error`.
- Clean comments interactive screen: review diffs and summaries, press `w` to apply safe fixes, `u` to confirm unsafe fixes, or `q` to exit without writing.
- Workflows screen: select and trigger workflows with live step-by-step check/cross status indicators.
- Scrollable results with automatic wrapping, supporting `↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End`.

## Docs

- Design: [docs/superpowers/specs/2026-10-22-tija-phases-design.md](docs/superpowers/specs/2026-10-22-tija-phases-design.md)
- Phase 3a Design: [docs/superpowers/specs/2026-10-29-tija-phase-3a-skill-packs-design.md](docs/superpowers/specs/2026-10-29-tija-phase-3a-skill-packs-design.md)
- Phase 3a Plan: [docs/superpowers/plans/2026-10-29-tija-phase-3a-skill-packs.md](docs/superpowers/plans/2026-10-29-tija-phase-3a-skill-packs.md)
- Reference: [docs/reference/](docs/reference/)
