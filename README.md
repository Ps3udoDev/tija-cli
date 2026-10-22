# Tija

> Your repository. Your rules. Any agent.

Tija is a CLI that prepares a repository to work with AI coding agents: it detects your stack, writes a versioned `.tija/` folder (manifest, rules, templates, workflows), verifies the health of your environment and branches, and provides deterministic quality commands (`check`, `lint`, `format`, `test`). Tija never installs tools automatically and never invokes AI agents; it operates deterministically with your local repo tools or binaries on `PATH`.

*Nota: la interfaz está en inglés; el contenido generado usa `content.language` (por defecto `es`).*

## Install

```sh
go install github.com/Ps3udoDev/tija-cli/cmd/tija@latest
```

Or download a binary for Windows, Linux or macOS from the [Releases](https://github.com/Ps3udoDev/tija-cli/releases) page.

## Commands

| Command | What it does |
|---|---|
| `tija init` | Detects the stack and template, creates `.tija/` and updates `.gitignore`. Idempotent. |
| `tija doctor` | Checks git, `gh` (and its login), the agent CLIs on `PATH`, the manifest, effective quality tools and the PR template. |
| `tija status` | Classifies the current branch against naming rules, shows ahead/behind against the PR target, the working tree and the remote. |
| `tija check` | Runs all configured or detected quality checks (build, lint, format, tests). |
| `tija lint` | Runs linter checks. Pass `--fix` to run lint + format in fix mode. |
| `tija format` | Formats source code in fix mode. |
| `tija test` | Runs the test suite and parses test result reports (TRX, Jest/Vitest JSON). |

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
- `1`: Check failure (e.g. linter error, failing tests, unformatted code).
- `2`: Usage error or missing environment dependencies (e.g. unavailable tool, syntax error, not a git repository).

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
- Menu options: `Check`, `Tests`, `Doctor`, `Status`, and `Init`.
- Live progress screen (`screenQuality`) with real-time spinner (⠋⠙⠹⠸...), step status, and durations.
- Expressive pixel cat mascot: `Idle`, `Scanning`, `Success`, `Warning` (!), and `Error`.
- Scrollable results with automatic wrapping, supporting `↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End`.

## Docs

- Design: [docs/superpowers/specs/2026-10-22-tija-phases-design.md](docs/superpowers/specs/2026-10-22-tija-phases-design.md)
- Phase 2 Plan: [docs/superpowers/plans/2026-10-22-tija-phase-2-quality.md](docs/superpowers/plans/2026-10-22-tija-phase-2-quality.md)
- Reference: [docs/reference/](docs/reference/)
