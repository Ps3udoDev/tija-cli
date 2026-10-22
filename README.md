# Tija

> Your repository. Your rules. Any agent.

Tija is a CLI that prepares a repository to work with AI coding agents: it detects your stack, writes a versioned `.tija/` folder (manifest, rules, templates, workflows) and tells you whether your environment and branches are healthy. Phase 1 never invokes an AI agent; it only detects which agent CLIs are on your `PATH` (`claude`, `codex`, and `gemini` via Antigravity's `agy` binary, falling back to `gemini`).

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
| `tija doctor` | Checks git, `gh` (and its login), the agent CLIs on `PATH`, the manifest, the quality tools and the PR template. |
| `tija status` | Classifies the current branch against the naming rules, shows ahead/behind against the PR target, the working tree and the remote. |

Flags:

- `--json` machine-readable output (no colors, no TUI).
- `--yes` approves every action that needs consent (Tija never edits existing files outside `.tija/` without it).
- `--no-color` plain text (also honored via `NO_COLOR`, and automatically when stdout is not a TTY).
- `status --strict` exits `1` when the branch is nonconforming or detached.

Exit codes: `0` ok, `1` validation or check failure, `2` usage or environment error. `doctor` reports environment problems (missing git, `gh` or tools, not a repository) as failed checks, so it exits `1` rather than `2`.

## TUI

Run `tija` with no arguments in a terminal to open the interactive menu (Init, Doctor, Status). Outside a TTY, or with `--json`/`--no-color`, only stable plain text or JSON is printed.

## Docs

- Design: [docs/superpowers/specs/2026-10-22-tija-phases-design.md](docs/superpowers/specs/2026-10-22-tija-phases-design.md)
- Reference: [docs/reference/](docs/reference/)
