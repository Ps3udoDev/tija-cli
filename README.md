# Tija

> Your repository. Your rules. Any agent.

Tija is a CLI that prepares a repository to work with AI coding agents: it detects your stack, writes a versioned `.tija/` folder (manifest, rules, templates, workflows, skill packs), verifies the health of your environment and branches, and provides deterministic quality commands (`check`, `lint`, `format`, `test`), safe comment cleaning (`clean comments`), and composed workflows (`run`). Tija never installs tools automatically; it operates deterministically with your local repo tools or binaries on `PATH`.

*Nota: la interfaz está en inglés; el contenido generado usa `content.language` (por defecto `es`).*

## Install

Building from source requires Go 1.27.3 or later.

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
| `tija report activity` | Generates the PDF activity report of a period (see Report packs). |
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

- `generar-informe-pdf`: Branded PDF reports from a JSON file (own pack; holds the branding assets).
- `generar-reporte-actividades`: PDF activity reports from commits and pull requests over a period (own pack; reuses the assets of `generar-informe-pdf`).

All third-party materials are licensed under the MIT License and fully attributed to their original authors with companion `LICENSE` files and frontmatter source references. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for full notices and license texts.

### Report packs

**Activity report from Tija.** Run `tija` and choose `Activity report`, then pick this month, last month, the last 7 days or a custom range. Or use the command:

```sh
tija report activity --since 2026-10-01 --until 2026-10-31
```

Tija collects the commits and pull requests of the period, the agent writes the content as text, and Tija saves it and generates the PDF in `docs/reportes/actividades-<since>_<until>/`. That folder also keeps `actividad.md` and `reporte.json`, and `docs/reportes/` is ignored by git because reports name clients and hours. Flags: `--agent <name>`, `--no-agent` (stops after collecting) and `--agent-timeout` (default `5m`). Without an agent CLI, Tija also stops after collecting. Hours are estimates from commit times, marked with `~` in the PDF: review them before delivering the report.

By default the report covers the current repository and your `git` and `gh` identities. To group several repositories, create `docs/reportes/manifest.json` with the groups, repositories and authors described in `references/report-schema.md`; every report uses it and only replaces the period.

**Other uses.** `generar-informe-pdf` has no command: an agent or a person follows its `SKILL.md` and runs the script. `tija skills show <name>` prints the instructions.

Requirements: Python 3 (tested with 3.14) and `pip install -r .tija/skills/generar-informe-pdf/requirements.txt`. Tija does not install them. The activity collector also needs `git`, and `gh` to read pull requests. Previews with `--preview` need `pymupdf`.

Generate a report from the bundled example:

```sh
python .tija/skills/generar-informe-pdf/scripts/build_report.py .tija/skills/generar-informe-pdf/examples/informe-ejemplo/informe.json -o informe.pdf
```

Generate an activity report by hand (`manifest.json` and `reporte.json` are described in `references/report-schema.md`):

```sh
python .tija/skills/generar-reporte-actividades/scripts/collect_activity.py docs/reportes/octubre/manifest.json --fetch
# Write docs/reportes/octubre/reporte.json from the generated actividad.md
python .tija/skills/generar-reporte-actividades/scripts/build_activity_report.py docs/reportes/octubre/reporte.json
```

To use your own brand, replace the PNG files in `.tija/skills/generar-informe-pdf/assets/branding/` and edit `template.json` next to them (colors, header and footer text). The default logo sources are in `assets/brand/`.

On Windows, use `py` if `python` opens the Microsoft Store. Keep both packs in the same folder: the activity report loads the engine, template and assets of `generar-informe-pdf`.

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
- Menu options: `Pull Request`, `Clean comments` (JS/TS projects), `Check`, `Tests`, `Workflows` (when workflows exist), `Activity report` (when the report packs are installed), `Doctor`, `Status`, and `Init`.
- Live progress screens with real-time spinner, step status, and live event updates.
- Expressive pixel cat mascot: `Idle`, `Scanning`, `Thinking`, `Success`, `Warning` (!), and `Error`.
- Clean comments interactive screen: review diffs and summaries, press `w` to apply safe fixes, `u` to confirm unsafe fixes, or `q` to exit without writing.
- Workflows screen: select and trigger workflows with live step-by-step check/cross status indicators.
- Activity report screen: pick a period and follow each stage until the PDF is ready (shown when the report packs are installed).
- Scrollable results with automatic wrapping, supporting `↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End`.

## Docs

- Design: [docs/superpowers/specs/2026-10-22-tija-phases-design.md](docs/superpowers/specs/2026-10-22-tija-phases-design.md)
- Phase 3a Design: [docs/superpowers/specs/2026-10-29-tija-phase-3a-skill-packs-design.md](docs/superpowers/specs/2026-10-29-tija-phase-3a-skill-packs-design.md)
- Phase 3a Plan: [docs/superpowers/plans/2026-10-29-tija-phase-3a-skill-packs.md](docs/superpowers/plans/2026-10-29-tija-phase-3a-skill-packs.md)
- Reference: [docs/reference/](docs/reference/)

## Review

`tija review` inspects committed changes, working tree changes and untracked text files against the target branch's merge base. It runs optional architecture checks, quality checks and a read-only agent pass. `tija review --quality` uses the quality pack; `tija review --deep` runs both packs with a deterministic synthesis. Deep mode confirms overlapping findings with the same rule and marks severity disagreements as disputed. At most three agent calls run concurrently.

```sh
tija review --no-agent --no-check
tija review --deep --fail-on medium --json
tija review --quality --agent claude --agent-timeout 8m --no-pr-comments
```

`--agent` selects claude, codex, gemini or none; `--no-agent` runs deterministic checks only. Without `--agent`, Tija uses `agents.default` or, if that CLI is missing, the first installed one; when none is installed it warns and runs deterministic checks only. An explicit `--agent` that is not installed exits 2. `--no-check` skips quality checks; otherwise fresh results for a clean HEAD are reused or check runs. `--check-timeout` defaults to 10 minutes. Agent calls default to 3/6/12 minutes for fast/balanced/deep; deep always uses the deep tier. `--agent-timeout` overrides the timeout. Invalid agent output is retried once; a timeout is not retried. `--no-pr-comments` skips reading existing PR comments; review never posts comments.

Exit codes: **0** means no finding reaches the threshold; **1** means at least one does; **2** means invalid usage/environment, invalid architecture rules or a failed agent chunk after retry. `--fail-on critical|high|medium|low|none` overrides `review.failOn` (default high). `none` disables finding failures, but environment failures still return 2. `pre-pr` runs clean comments, check, review and pr prepare, stopping on a nonzero exit.

Every agent finding is verified in order: file belongs to the chunk; the location overlaps a new-side diff hunk; evidence of at most five lines occurs within three lines of that range after whitespace normalization; severity is valid; rule belongs to the pass's pack or applicable architecture rules. Rejected findings are listed in JSON with `file-not-in-diff`, `line-outside-diff`, `evidence-not-found`, `invalid-severity` or `unknown-rule`. Deleted and binary files are not sent for review. Files are grouped by directory into chunks of up to `review.maxLines` changed lines (default 3000); oversized files run alone. `review.exclude` defaults exclude lockfiles, generated code and build output.

`review` only persists `.tija/session/review.json` inside an initialized repository. Check artifacts and agent output use system temporary directories; failed check logs are retained there. In an uninitialized repository no session is saved. A clean session for the current commit can be cited by `pr prepare` in its tests section; findings are never copied into the PR.

`--json` emits one object. Its stable fields are:

| Field | Meaning |
| --- | --- |
| `mode`, `base`, `commit`, `dirty` | Mode, resolved target, reviewed HEAD and working tree state |
| `failOn`, `exit` | Effective threshold and exit code |
| `quality` | `reused` and status: passed, failed, unavailable or skipped |
| `findings` | Verified agent, check and quality findings, ordered by severity and assigned `F1…Fn` |
| `discarded` | Rejected agent locations with pass and reason |
| `prComments` | Whether comments were read and the uncovered comments |
| `chunks` | Total chunks and chunks with a failed pass |
| `summary` | Counts under Critical, High, Medium and Low |
| `warnings` | Nonfatal limitations or failed-call diagnostics |
| `savedAt` | RFC 3339 timestamp in the saved session |

Findings contain `id`, `source`, `passes`, `file`, `line`, `endLine`, `severity`, `rule`, `title`, `explanation`, `suggestion`, `evidence`, `confirmedBy`, `disputed`, `severities` and `prComments`. Quality failures use their log path and line zero. Comments are informational and never change severity or the exit code.

### Architecture rules

Rules are optional Markdown files at `.tija/rules/architecture/*.md`, configurable with `review.rules`. `doctor` validates YAML, regexes and unique IDs, including collisions with pack rules. Checks run on added lines only; rule bodies go to the agent only for matching paths.

```markdown
---
name: backend-dotnet
appliesTo: ["src/**/*.cs"]
checks:
  - id: be-domain-no-infra
    severity: High
    files: "src/*.Domain/**/*.cs"
    forbid: '^\s*using\s+[\w.]*\.(Infrastructure|Api)\b'
    message: El dominio no puede depender de Infrastructure ni de la API.
---
## be-thin-controllers: Controladores delgados
Mantener la lógica de negocio fuera de los controladores.
```

`name` is required. `files` accepts a string or list; `appliesTo` is optional. `forbid` uses Go RE2 syntax, with no lookbehind. Severities are Critical, High, Medium and Low, case insensitive. IDs must be unique across all architecture checks/rules and distinct from pack IDs. Each `## id: title` starts an agent rule.

Presets `dotnet-clean` and `react-features` are embedded starting points. Interactive init offers the detected stack's preset with default no. `--yes` does not install presets. Explicitly select one or more with `tija init --rules-preset dotnet-clean` or repeat the flag. Edited presets are preserved. Adjust the preset paths to your project. Only `be-domain-no-infra` is `High`; the other preset checks are pattern-based and `Medium`, so they do not block `pre-pr` with the default threshold.

## MCP

Run Tija as a local stdio MCP server using the same engine as the CLI:

```sh
tija mcp serve --dir /absolute/path/to/repository
tija mcp config codex --dir /absolute/path/to/repository
```

The repository root is resolved once at startup. Starting outside a repository without a valid `--dir` exits with code 2 on stderr. Tools cannot change the root. Stdout contains only JSON-RPC; diagnostics go to stderr (`--log-level error|warn|info|debug`, default `warn`).

`mcp config` prints configuration and never edits client files. Supported formats: `claude-code`, `codex`, `claude-desktop`, `cursor`, and experimental `antigravity`. For Claude Desktop, use `--dir` because the client may start in your home directory. The Claude Code command uses PowerShell quoting on Windows and POSIX shell quoting elsewhere. Codex configuration includes `tool_timeout_sec = 900` and `startup_timeout_sec = 20`.

| Tools | Effect | Permission |
| --- | --- | --- |
| `repo_status`, `doctor`, `repo_rules`, `repo_context`, `skills_list` | Read | None |
| `quality_check`, `review_run`, `pr_prepare` | Session files and engine temporary artifacts | None |
| `report_activity` | Ignored `docs/reportes/`; reads PRs with `gh` | None |
| `quality_fix`, `comments_clean` | Repository files | `write: true`; unsafe comment changes also require `unsafe: true` |
| `pr_create` | Push and GitHub PR creation/update | `write: true` |
| `workflow_run` | Highest effect of its steps | Checks all steps first; publishing requires both `write: true` and `publish: true` |

Without `write`, `quality_fix` describes what it would run, `comments_clean` returns its plan/diff and `pr_create` returns its plan. Invalid PR drafts are rejected even when write is absent. Missing workflow permissions return `needs-permission` before any step executes. Tool annotations inform clients; server-side checks enforce permissions. Scripts configured by the repository run with the normal local process permissions.

Calls report progress when the client sends a `progressToken`. Writing operations are serialized with a cancellable gate. Cancelling an external formatter may leave changes; the server reports changed files, while a cancelling client may only display its own context error. `agent: "none"` selects deterministic processing. `comments_clean` and `tija clean comments --agent <provider>` honor explicit provider selection.

Structured results preserve the CLI JSON; `_meta["tija/exitCode"]` carries the exit code. Review also retains `exit`. Exit 1 is a normal finding result, while environment failures are `isError`. `skills_list` wraps its CLI array in `{ "skills": [...] }` because MCP structured output must be an object.

The embedded prompt `documentar_pr` guides QA and PR preparation. Its optional arguments are `qaptureProject` and `mode` (`review`, `quality`, `deep`). It is not installed into `.tija/` and does not change defaults versions. See [Usar Tija con Qapture](docs/mcp/qapture.md) for evidence references, permissions and platform limits. Case titles and notes can appear in a PR: review private data before attaching or publishing. Tija validates evidence structure, never the external existence of Qapture records, and discards full export paths.

CI tests the stdio protocol and tool snapshot on Windows, Linux and macOS. `bash scripts/check-binary-size.sh` cross-compiles stripped, uncompressed binaries for Windows/amd64, Linux/amd64 and macOS/arm64. Size policy uses decimal bytes: goal below 15 MB, technical alert from 20 MB, maximum 25 MB; exceeding the maximum fails the script.
