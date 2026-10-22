# Tija Fase 1 — Foundation: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Entregar un binario `tija` instalable que ejecute `init`, `doctor` y `status` sobre repos .NET y React/Vite reales, con salida CLI con marca, `--json`, y una TUI esqueleto (startup screen + menú + mascota en 4 estados).

**Architecture:** Todo el comportamiento vive en `internal/engine`, que expone funciones con entradas y salidas tipadas (`PlanInit/ApplyInit`, `Doctor`, `Status`). `internal/cli` (Cobra) y `internal/tui` (Bubble Tea) son capas finas que llaman a esas funciones y delegan el formato a `internal/render`. Las operaciones externas (git, gh, PATH) pasan por un `sys.Env` inyectable para poder probarlas con fakes y con repos git temporales reales.

**Tech Stack:** Go 1.27 (instalado: go1.27.1 windows/amd64), Cobra, Bubble Tea + Lip Gloss (última versión estable mayor que resuelva `go get`), `gopkg.in/yaml.v3`, `golang.org/x/term`, GoReleaser.

**Spec:** `docs/superpowers/specs/2026-10-22-tija-phases-design.md` (sección 5, Fase 1, más las secciones 3 y 4). Léelo junto con este plan. Referencias en `docs/reference/`.

## Global Constraints

- Módulo Go: `github.com/Ps3udoDev/tija-cli`. Binario: `cmd/tija`.
- Strings de UI en **inglés**. El contenido generado usa `content.language` (default `es`).
- Paleta única: Orange `#F97316`, Ink `#111827`, White `#FFFFFF`, Sand `#FED7AA`. Símbolos: `✓` ok, `✗` fail, `!` warn, `>` selección.
- Exit codes: `0` ok, `1` fallo de validación o check, `2` error de uso o entorno.
- Si stdout no es una TTY, o con `--json`, `--no-color` o `NO_COLOR`: sin colores, sin mascota y sin TUI. Texto plano estable o JSON.
- Todo comando funciona sin interacción. `--yes` aprueba todas las acciones que requieren consentimiento.
- Tija nunca modifica un archivo existente fuera de lo nuevo en `.tija/` sin consentimiento (`--yes` o confirmación interactiva).
- Versionado en `.tija/`: `manifest.yaml`, `rules/`, `templates/`, `workflows/`. Entradas de `.gitignore`: `.tija/cache/`, `.tija/index/`, `.tija/session/`, `.tija/tmp/`.
- Windows es plataforma de primera clase. Los tests deben pasar en Windows, Linux y macOS.
- El menú de la TUI muestra **solo** `Init`, `Doctor` y `Status`.
- Ningún comando de la Fase 1 invoca a un agente de IA. Solo se detecta qué CLIs (`claude`, `codex`, `gemini`) existen en PATH.

## Review Focus

1. **Rutas con espacios y no ASCII** (el repo real vive en `.../azure 2/github/ClaimsAPI`): git, detección y escritura deben funcionar. Lo cubre Task 3 (`TestRepoInPathWithSpaces`).
2. **`.gitignore` con CRLF** (los repos del autor tienen CRLF): se agregan entradas respetando el fin de línea y sin duplicar. Lo cubre Task 7 (`TestGitignoreCRLFPreserved`).
3. **Ejecutar desde un subdirectorio** (`src/`): `init` y `status` resuelven la raíz del repo y no crean `.tija/` en el subdirectorio. Lo cubre Task 7 (`TestInitFromSubdirectory`).
4. **Repo sin remoto o sin `origin/HEAD`, o sin commits**: `status` y la elección de `target` no fallan. Lo cubre Task 9 (`TestStatusNoRemoteNoCommits`).
5. **Manifest editado a mano con YAML inválido**: `doctor` da `✗` con el error y `init` no lo sobrescribe. Lo cubren Task 8 (`TestDoctorInvalidManifest`) y Task 7 (`TestInitDoesNotOverwriteInvalidManifest`).

---

## Estructura de archivos

```text
cmd/tija/main.go                       → os.Exit(cli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
internal/sys/env.go                    → Env (Runner, LookPath), Default()
internal/testutil/repo.go              → NewRepo(t, fixture) crea un repo git temporal real
internal/gitx/gitx.go                  → consultas git vía binario
internal/detect/detect.go              → Detection, Detect()
internal/detect/dotnet.go              → .sln, proyectos, framework, tests
internal/detect/node.go                → package.json, package manager, herramientas JS
internal/detect/pr.go                  → template PR, workflow de gobernanza, guía, husky, commitlint
internal/detect/testdata/{dotnet-api,react-vite,unknown,multi-sln}/
internal/branch/branch.go              → ClassifyBranch (función pura)
internal/manifest/manifest.go          → tipos, Load, Save, Validate, FromDetection
internal/assets/assets.go + defaults/  → reglas y templates embebidos (go:embed)
internal/agents/detect.go              → Available(lookPath) []string
internal/engine/init.go                → PlanInit, ApplyInit
internal/engine/gitignore.go           → mergeGitignore
internal/engine/doctor.go              → Doctor
internal/engine/status.go              → Status
internal/render/{mode,palette,init,doctor,status}.go
internal/cli/{root,init,doctor,status,version,prompt}.go
internal/tui/{model,menu,mascot,startup}.go
.goreleaser.yaml, .github/workflows/{ci,release}.yml, README.md, .gitignore
```

---

### Task 1: Scaffold, Env y CLI raíz

**Files:**
- Create: `go.mod`, `.gitignore`, `cmd/tija/main.go`, `internal/sys/env.go`, `internal/cli/root.go`, `internal/cli/version.go`
- Test: `internal/cli/root_test.go`

**Interfaces:**
- Produces:
  - `sys.Runner func(ctx context.Context, dir, name string, args ...string) (stdout string, exitCode int, err error)`. `err` solo cuando el proceso no pudo arrancar; un exit code distinto de 0 no es `err`.
  - `sys.Env struct { Run Runner; LookPath func(string) (string, error); Getenv func(string) string }` y `sys.Default() Env` (`os/exec`, `exec.LookPath`, `os.Getenv`).
  - `cli.Version = "dev"`, variable sobrescrita por ldflags.
  - `cli.ExitError struct { Code int; Err error }`, que implementa `error`.
  - `cli.Execute(args []string, in io.Reader, out, errOut io.Writer) int`. Flags persistentes: `--json`, `--yes`, `--no-color`.
  - `cli.Deps struct { Env sys.Env; IsTTY func(any) bool; RunTUI func(dir string) error }`, inyectable vía `cli.ExecuteWith(deps, args, in, out, errOut) int`.

- [ ] **Step 1: Inicializar el módulo y las dependencias.** `go mod init github.com/Ps3udoDev/tija-cli`, `go get github.com/spf13/cobra gopkg.in/yaml.v3 golang.org/x/term`. En `.gitignore`: `dist/`, `/tija`, `/tija.exe`, `coverage.out`.
- [ ] **Step 2: Escribir los tests que fallan**

```go
func TestVersionCommand(t *testing.T) // Execute([]string{"version"}) → code 0, out contiene "tija dev"
func TestUnknownCommandExit2(t *testing.T) // Execute([]string{"nope"}) → code 2
func TestNoArgsNonTTYPrintsHelp(t *testing.T) // IsTTY=false, args vacíos → code 0, out contiene "Usage:", RunTUI no llamado
func TestNoArgsTTYRunsTUI(t *testing.T) // IsTTY=true → RunTUI llamado una vez con el cwd
```

- [ ] **Step 3: Ejecutar** `go test ./internal/cli/...`. Expected: FAIL (paquete sin símbolos).
- [ ] **Step 4: Implementar** `root.go`, `version.go`, `env.go` y `main.go`. Sin argumentos se abre la TUI solo si stdin **y** stdout son TTY y no hay `--json`. Los errores de flags o de comando desconocido terminan con exit 2. `SilenceUsage` y `SilenceErrors` en true, y los errores se imprimen en `errOut` como `✗ <mensaje>` (texto plano).
- [ ] **Step 5: Ejecutar** `go test ./... && go vet ./...`. Expected: PASS.
- [ ] **Step 6: Commit** `feat(cli): scaffold tija binary with root and version commands`

---

### Task 2: Render (modo, paleta, símbolos)

**Files:**
- Create: `internal/render/mode.go`, `internal/render/palette.go`
- Test: `internal/render/mode_test.go`

**Interfaces:**
- Produces:
  - `render.Mode int` con `Styled`, `Plain` y `JSON`.
  - `render.ModeFor(isTTY bool, jsonFlag, noColor bool, getenv func(string) string) Mode`. JSON tiene prioridad. Es Styled solo si `isTTY && !noColor && getenv("NO_COLOR")==""`.
  - `render.Printer struct { W io.Writer; Mode Mode }` con `Ok(msg)`, `Warn(msg)`, `Fail(msg)`, `Heading(msg)`, `KV(key, value string)` y `JSONOut(v any) error`.
  - Constantes `render.Orange`, `Ink`, `White` y `Sand` con los hex de Global Constraints. Los estilos Lip Gloss se construyen solo a partir de ellas.

- [ ] **Step 1: Tests que fallan**

```go
func TestModeFor(t *testing.T) // tabla: (tty,json,nocolor,env) → (true,false,false,"")=Styled; (true,true,*,*)=JSON; (false,false,false,"")=Plain; (true,false,false,"1")=Plain; (true,false,true,"")=Plain
func TestPlainHasNoANSI(t *testing.T) // Printer{Plain}.Ok("Git installed") → exactamente "✓ Git installed\n", sin "\x1b["
func TestWarnFailSymbols(t *testing.T) // Plain: Warn → "! x\n", Fail → "✗ x\n"
```

- [ ] **Step 2: Ejecutar** `go test ./internal/render/...`. Expected: FAIL.
- [ ] **Step 3: Implementar.** Agregar `go get` de Lip Gloss. En Styled, el símbolo `✓` se pinta Orange, `✗` en Orange bold, `!` en Sand, y los headings en bold.
- [ ] **Step 4: Ejecutar los tests.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(render): add output modes, palette and status symbols`

---

### Task 3: testutil + gitx

**Files:**
- Create: `internal/testutil/repo.go`, `internal/gitx/gitx.go`
- Test: `internal/gitx/gitx_test.go`

**Interfaces:**
- Consumes: `sys.Runner`.
- Produces:
  - `testutil.NewRepo(t *testing.T, fixtureDir string, opts ...RepoOpt) string` copia el fixture (si no es "") a `t.TempDir()/<"azure 2">/<repo>`, ejecuta `git init -b main`, configura usuario y hace un commit inicial. Devuelve la ruta. `RepoOpt`: `WithoutCommit()`, `WithBranch(name)`, `WithRemote(bareDir, setHead bool)`.
  - `testutil.Git(t, dir, args...) string` ejecuta git y falla el test si hay error.
  - `gitx.Repo struct { Dir string; Run sys.Runner }`
  - `gitx.FindRoot(ctx, run sys.Runner, dir string) (root string, ok bool, err error)` (`git rev-parse --show-toplevel`; `ok=false` fuera de un repo).
  - Métodos de `gitx.Repo`: `CurrentBranch(ctx) (name string, detached bool, err error)`, `HasCommits(ctx) bool`, `LocalBranches(ctx) []string`, `DefaultRemoteBranch(ctx) string` ("" si no hay `origin/HEAD`), `RemoteURL(ctx) string`, `AheadBehind(ctx, base string) (ahead, behind int, found bool)` (busca `base` local y luego `origin/base`), `WorkTree(ctx) (staged, unstaged, untracked int)` (vía `git status --porcelain=v1`), `IsPublished(ctx, branch string) bool` (existe `refs/remotes/origin/<branch>`).

- [ ] **Step 1: Tests que fallan**

```go
func TestRepoInPathWithSpaces(t *testing.T) // NewRepo en ".../azure 2/..." → FindRoot desde <root>/src devuelve root (comparar con filepath.EvalSymlinks), ok=true
func TestFindRootOutsideRepo(t *testing.T) // t.TempDir() vacío → ok=false, err=nil
func TestCurrentBranchAndDetached(t *testing.T) // WithBranch("feat/x") → "feat/x",false; tras `git checkout --detach` → detached=true
func TestNoCommits(t *testing.T) // WithoutCommit() → HasCommits=false; CurrentBranch="main"
func TestAheadBehind(t *testing.T) // main + rama con 2 commits nuevos → AheadBehind("main") = 2,0,true; AheadBehind("dev") found=false
func TestWorkTreeCounts(t *testing.T) // 1 archivo staged, 1 modificado, 2 untracked → 1,1,2
func TestDefaultRemoteBranch(t *testing.T) // WithRemote(bare,setHead=true) → "main"; sin remoto → ""
```

- [ ] **Step 2: Ejecutar** `go test ./internal/gitx/...`. Expected: FAIL.
- [ ] **Step 3: Implementar.** testutil fija `GIT_AUTHOR_NAME`/`GIT_COMMITTER_NAME`/`..._EMAIL` y `-c core.autocrlf=false` vía `t.Setenv`.
- [ ] **Step 4: Ejecutar los tests.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(gitx): add git queries and test repo helpers`

---

### Task 4: Detección de stack (.NET y React/Vite)

**Files:**
- Create: `internal/detect/detect.go`, `internal/detect/dotnet.go`, `internal/detect/node.go`
- Create fixtures:
  - `internal/detect/testdata/dotnet-api/`: `ClaimsService.sln` que referencia `src\Claims.API\Claims.API.csproj` y `tests\Claims.Tests\Claims.Tests.csproj`; esos dos csproj (`<TargetFramework>net8.0</TargetFramework>`, el de tests con `PackageReference Include="xunit"`); `tests/Claims.Tests/ClaimTests.cs` con 2 `[Fact]` y 1 `[Theory]`; `scratch/inspect_roles.csproj`, que NO está en la sln.
  - `internal/detect/testdata/react-vite/`: `package.json` con `"packageManager": "bun@1.3.5"`, deps `react ^19.2.0` y devDeps `vite ^7.3.1`, `@biomejs/biome 2.4.4`; además `bun.lock`, `package-lock.json`, `biome.json`, `vite.config.js` y `src/App.jsx`.
  - `internal/detect/testdata/multi-sln/`: `A.sln` y `B.sln` vacías.
  - `internal/detect/testdata/unknown/`: `README.md`.
- Test: `internal/detect/detect_test.go`

**Interfaces:**
- Produces:

```go
type Stack string // "dotnet-api" | "react-vite" | "unknown"
type Detection struct {
    Root, ProjectName      string   // ProjectName = filepath.Base(Root)
    Stack                  Stack
    Language, Framework    string   // "C#", ".NET 8" | "JavaScript"/"TypeScript", "React 19 + Vite 7"
    Solutions              []string // .sln de la raíz, rutas relativas, orden alfabético
    Solution               string   // la única, o "" si hay 0 o >1
    Projects               []string // csproj referenciados por Solution (con '/')
    PackageManager         string   // "bun"|"pnpm"|"yarn"|"npm"|"dotnet"|""
    Lockfiles              []string
    Tools                  []string // subconjunto ordenado de: biome eslint prettier dotnet-format playwright vitest jest xunit nunit mstest husky commitlint
    TestFramework          string
    TestCount              int
    CI                     []string // .github/workflows/*.yml|yaml relativos
    GitHost                string   // "github" si RemoteURL contiene github.com, si no ""
    // completados en Task 5:
    PRTemplate, PRGovernanceWorkflow, PRGuide string
    PRTemplateCandidates   []string
    BranchPrefixes         []string
    HuskyBranchHook        bool
    CommitlintHeaderMax    int
}
func Detect(root, remoteURL string) (Detection, error)
func (d Detection) WithSolution(sln string) (Detection, error) // recalcula Projects/Framework/Tests para la sln elegida
```

  Reglas (del spec): .NET usa solo la `.sln` de la raíz y sus proyectos, e ignora `.csproj` que no estén en ella; con varias `.sln`, `Solution=""` hasta que se llame `WithSolution`. Para JS, `packageManager` tiene prioridad sobre los lockfiles (bun.lock/bun.lockb→bun, pnpm-lock.yaml→pnpm, yarn.lock→yarn, package-lock.json→npm). Framework con la versión mayor del rango semver. TestCount: .NET cuenta `[Fact]`, `[Theory]`, `[Test]` y `[TestMethod]` en los proyectos de test de la sln; JS cuenta archivos `*.test.*` / `*.spec.*` fuera de `node_modules`. Se omiten `node_modules`, `bin`, `obj`, `dist` y `.git`.

- [ ] **Step 1: Tests que fallan**

```go
func TestDetectDotnet(t *testing.T) // Stack="dotnet-api", Language="C#", Framework=".NET 8", Solution="ClaimsService.sln", Projects=["src/Claims.API/Claims.API.csproj","tests/Claims.Tests/Claims.Tests.csproj"], PackageManager="dotnet", Tools contiene "dotnet-format","xunit" y NO hay proyecto scratch, TestFramework="xunit", TestCount=3
func TestDetectReactVite(t *testing.T) // Stack="react-vite", Framework="React 19 + Vite 7", PackageManager="bun", Lockfiles=["bun.lock","package-lock.json"], Tools contiene "biome", TestCount=0
func TestDetectMultiSln(t *testing.T) // Solutions=["A.sln","B.sln"], Solution=""; WithSolution("B.sln") → Solution="B.sln"
func TestDetectUnknown(t *testing.T) // Stack="unknown", err=nil
func TestGitHost(t *testing.T) // remoteURL "https://github.com/TEAMS4SOFT/GMX-frontend" → "github"; "" → ""
```

- [ ] **Step 2: Ejecutar** `go test ./internal/detect/...`. Expected: FAIL.
- [ ] **Step 3: Implementar** `Detect`. Las referencias de la sln se parsean con la regex `Project\("\{[^}]+\}"\)\s*=\s*"[^"]*",\s*"([^"]+\.csproj)"`, normalizando `\` a `/`.
- [ ] **Step 4: Ejecutar los tests.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(detect): detect .NET and React/Vite stacks`

---

### Task 5: Detección de artefactos de PR, husky y commitlint

**Files:**
- Create: `internal/detect/pr.go`
- Modify fixtures: en `react-vite/`, copiar `docs/reference/pull-requests/PULL_REQUEST_TEMPLATE.md` → `.github/PULL_REQUEST_TEMPLATE.md` y `pr-governance.yml` → `.github/workflows/pr-governance.yml`; `.husky/pre-push` con la línea real `VALID_BRANCH_REGEX="^(main|dev|feature|feat|bugfix|fix|hotfix|docs|refactor|chore|release)(\/[a-zA-Z0-9._-]+)?$"`; y `commitlint.config.js` con `"header-max-length": [2, "always", 100]`. En `dotnet-api/`, `templates/PULL_REQUEST_TEMPLATE_GUIDE.md`. Crear `testdata/multi-template/.github/PULL_REQUEST_TEMPLATE/{feature.md,bugfix.md}`.
- Test: `internal/detect/pr_test.go`

**Interfaces:**
- Produces: `Detect` completa los campos PR de `Detection` (ver Task 4).
  - Ubicaciones de template, en orden: `.github/pull_request_template.md` (sin distinguir mayúsculas), `PULL_REQUEST_TEMPLATE.md` en la raíz, `docs/` y luego `.github/PULL_REQUEST_TEMPLATE/*.md` (alfabético, se toma el primero y todos van a `PRTemplateCandidates`).
  - Workflow de gobernanza: el primer `.github/workflows/*` que contenga `action-semantic-pull-request`.
  - Guía: archivo que cumpla `**/PULL_REQUEST_TEMPLATE_GUIDE.md` a profundidad ≤ 2, fuera de `.tija/`.
  - Husky: si `.husky/pre-push` contiene `VALID_BRANCH_REGEX=`, se extraen las alternativas del primer grupo `^(...)`, se quitan `main|master|dev|develop` y quedan en `BranchPrefixes`; además `HuskyBranchHook=true`.
  - Commitlint: `commitlint.config.{js,cjs,mjs,ts}` o `.commitlintrc*` agregan `"commitlint"` a Tools. `header-max-length` se lee con la regex `"header-max-length"\s*:\s*\[\s*\d+\s*,\s*"always"\s*,\s*(\d+)`.

- [ ] **Step 1: Tests que fallan**

```go
func TestDetectPRArtifactsFrontend(t *testing.T) // PRTemplate=".github/PULL_REQUEST_TEMPLATE.md", PRGovernanceWorkflow=".github/workflows/pr-governance.yml", BranchPrefixes=["feature","feat","bugfix","fix","hotfix","docs","refactor","chore","release"], HuskyBranchHook=true, CommitlintHeaderMax=100, Tools contiene "husky","commitlint"
func TestDetectPRGuideBackend(t *testing.T) // PRGuide="templates/PULL_REQUEST_TEMPLATE_GUIDE.md", PRTemplate=""
func TestDetectMultipleTemplates(t *testing.T) // PRTemplate=".github/PULL_REQUEST_TEMPLATE/bugfix.md", len(PRTemplateCandidates)=2
```

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar** `pr.go`.
- [ ] **Step 4: Ejecutar** `go test ./internal/detect/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(detect): detect PR template, governance workflow, husky and commitlint`

---

### Task 6: Clasificación de ramas

**Files:**
- Create: `internal/branch/branch.go`
- Test: `internal/branch/branch_test.go`

**Interfaces:**
- Produces:

```go
var DefaultPrefixes = []string{"feat","fix","refactor","chore","docs","ci","perf","test","build","style","revert","feature","bugfix","hotfix","release"}
var DefaultBases = []string{"main","master","dev","develop"}
type State string // "conforming" | "nonconforming" | "base" | "detached"
type Rules struct { Prefixes, Bases []string; Target string }
type Verdict struct { State State; Name, Suggested, Reason string; IssueNumber int }
func Classify(name string, detached bool, r Rules) Verdict
```

  Algoritmo:
  - `detached` → State detached.
  - `name` está en `Bases` o es igual a `Target` → base.
  - `release/<v>` con `v` que cumple `^v?\d+(\.\d+)*([-.][0-9a-z]+)*$` → conforming.
  - Cumple `^(<prefixes>)/(\d+-)?[a-z0-9]+(-[a-z0-9]+)*$` → conforming. El número inicial se guarda en `IssueNumber`.
  - Cualquier otro caso → nonconforming con `Suggested`, construido así: pasar a minúsculas; separar por `[^a-z0-9]+`; un primer token numérico es el issue; el tipo es el primer token igual a un prefijo permitido, o que **empiece** por uno, eligiendo el prefijo más largo que coincida (`fixclaims` → `fix` + resto `claims`); `bug` → `fix` si está permitido; por defecto `feat`, o el primer prefijo de `Prefixes` si no existe `feat`. La descripción son los tokens restantes sin los que ya se usaron como tipo, sin stopwords (`and in the of a to for on with de la el en y con`) y sin duplicados, máximo 3. Si queda vacía, o el nombre era genérico (`changes|updates|update|test|fix|wip|temp`), `Suggested = <tipo>/<short-description>` y `Reason = "generic branch name"`. Si no, `Reason = "does not follow <prefix>/<short-description>"`. El formato final es `<tipo>/[<issue>-]<desc>`.

- [ ] **Step 1: Tests que fallan (valores del spec)**

```go
func TestClassify(t *testing.T) // tabla con Rules{Prefixes: DefaultPrefixes, Bases: DefaultBases, Target: "dev"}:
// "feat/shared-imputation-form"   → conforming
// "bugfix/133-password-leak"      → conforming, IssueNumber=133
// "release/1.2.3"                 → conforming
// "main", "dev"                   → base
// "changes"                       → nonconforming, Suggested="feat/short-description", Reason="generic branch name"
// "101-fixclaims-fix-integration-issues-and-ui-bugs-in-claims-module" → nonconforming, Suggested="fix/101-claims-integration-issues", IssueNumber=101
// "Feat/Upper-Case"               → nonconforming, Suggested="feat/upper-case"
// "accidents-refactor"            → nonconforming, Suggested="refactor/accidents"
func TestClassifyDetached(t *testing.T) // detached=true → State "detached"
func TestClassifyHuskyPrefixes(t *testing.T) // Prefixes sin "ci": "ci/pipeline" → nonconforming
```

- [ ] **Step 2: Ejecutar** `go test ./internal/branch/...`. Expected: FAIL.
- [ ] **Step 3: Implementar** `Classify`.
- [ ] **Step 4: Ejecutar.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(branch): classify branch names and suggest conforming names`

---

### Task 7: Manifest, assets embebidos e `init` del engine

**Files:**
- Create: `internal/manifest/manifest.go`, `internal/assets/assets.go`, `internal/assets/defaults/rules/pull-request.md` (copia exacta de `docs/reference/pull-requests/PULL_REQUEST_TEMPLATE_GUIDE.md`), `internal/assets/defaults/templates/pull-request.md` (copia exacta de `PULL_REQUEST_TEMPLATE.md`), `internal/agents/detect.go`, `internal/engine/init.go`, `internal/engine/gitignore.go`
- Test: `internal/manifest/manifest_test.go`, `internal/engine/init_test.go`, `internal/engine/gitignore_test.go`

**Interfaces:**
- Consumes: `detect.Detect`, `Detection.WithSolution`, `gitx.*`, `branch.DefaultPrefixes`, `sys.Env`.
- Produces:

```go
// manifest
type Manifest struct {
    Version     int         `yaml:"version"`
    Project     Project     `yaml:"project"`     // Name, Type, Solution(omitempty)
    Content     Content     `yaml:"content"`     // Language
    Git         Git         `yaml:"git"`         // Host, BranchConvention, BranchPrefixes
    PullRequest PullRequest `yaml:"pullRequest"` // Target, Template, Rules, Title{Convention,RequireScope,MaxLength}, RequiredSections, MinSectionLength, InvalidValues, AutoAssign
    Quality     map[string]QualityEntry `yaml:"quality"` // QualityEntry{Provider}
    Agents      Agents      `yaml:"agents"`      // Default, Tasks map[string]Task{Tier}
    Tija        Meta        `yaml:"tija"`        // DefaultsVersion, Installed map[string]string (ruta→sha256)
}
const Path = ".tija/manifest.yaml"
func FromDetection(d detect.Detection, agents []string, target string) Manifest
func Load(root string) (Manifest, error)   // error de parseo envuelto con la ruta
func Save(root string, m Manifest) error
func (m Manifest) Validate() []error
func MissingFields(existing, fresh Manifest) []string // claves YAML con punto ("pullRequest.title.maxLength") que están vacías en existing y con valor en fresh

// assets
const DefaultsVersion = "1"
func Files() map[string][]byte // ".tija/rules/pull-request.md", ".tija/templates/pull-request.md", ".tija/workflows/.gitkeep"

// agents
func Available(lookPath func(string) (string, error)) []string // orden: claude, codex, gemini

// engine
type ActionKind string // "create" | "update" | "replace"
type Action struct { ID string; Kind ActionKind; Path, Diff, Reason string; NeedsConsent bool }
type Choice struct { ID, Question string; Options []string; Default string } // IDs: "solution", "target"
type InitPlan struct { Root string; Detection detect.Detection; Manifest manifest.Manifest; Actions []Action; Choices []Choice; AlreadyInitialized bool }
type InitResult struct { Detection detect.Detection; Created, Updated []string; Pending []Action; Notes []string }
var ErrNotARepo = errors.New("not a git repository")
func PlanInit(ctx context.Context, dir string, env sys.Env) (InitPlan, error)
func ApplyInit(ctx context.Context, plan InitPlan, answers map[string]string, approved map[string]bool) (InitResult, error)
func mergeGitignore(existing []byte, entries []string) (out []byte, added []string)
```

  Valores de `FromDetection` (del spec, coinciden con `pr-governance.yml`): `Version 1`; `Project.Type = d.Stack`; `Content.Language "es"`; `Git.Host = d.GitHost`, `BranchConvention "conventional"`; `BranchPrefixes = d.BranchPrefixes` o, si no hay, `branch.DefaultPrefixes`; `Title{ "conventional-commits", true, d.CommitlintHeaderMax o 100 }`; `RequiredSections [Resumen, Objetivo, Cambios realizados, Pruebas]`; `MinSectionLength 10`; `InvalidValues ["n/a","na","no aplica","pendiente","todo","por definir","...","-"]`; `AutoAssign true`. `Template` es `d.PRTemplate` o `.tija/templates/pull-request.md`; `Rules` es `d.PRGuide` o `.tija/rules/pull-request.md`. `Quality` se arma desde Tools: biome→`lint` y `format`: biome; eslint→`lint`; prettier→`format`; dotnet-format→`format: dotnet-format` y `build: dotnet`; xunit/nunit/mstest→`tests: dotnet-test`; vitest/jest→`tests`; playwright→`e2e`. `Agents.Default` es el primero de `agents` o ""; las Tasks son pullRequest fast, review balanced y architecture deep.

  Default de `target`: `DefaultRemoteBranch`; si no hay, `main` si existe localmente, luego `master`, luego la rama actual. Se agrega una Choice `target` cuando existen ≥2 de {main, master, dev, develop} localmente o en origin. Se agrega una Choice `solution` cuando `len(Solutions)>1`, con default el primero alfabético.

  Reglas de acciones de `PlanInit` (del spec, sección 3 "Decididas por Claude"):
  - Fuera de un repo → `ErrNotARepo`. Siempre se trabaja sobre la raíz del repo, aunque `dir` sea un subdirectorio.
  - Los archivos nuevos dentro de `.tija/` se crean con `NeedsConsent=false`.
  - `.gitignore` → `update`, con `NeedsConsent=true` y `Diff` con las líneas `+`.
  - Repo sin template de PR → acción `create` de `.github/PULL_REQUEST_TEMPLATE.md` con `NeedsConsent=true`, `Reason "offer: copy PR template to .github"`.
  - Sin guía detectada → crear `.tija/rules/pull-request.md`. Con guía, no se crea y el manifest apunta a la guía.
  - Re-ejecución con manifest existente: nunca se reemplaza. Si `MissingFields` no está vacío → `update` del manifest con `NeedsConsent=true` (merge solo de esos campos). Si el manifest no parsea → ninguna acción sobre él, y se agrega una Note `"manifest is invalid, fix it manually: <err>"`.
  - Archivos instalados: si existen y su sha256 coincide con `Tija.Installed` pero difiere del embebido → `replace` sin consentimiento. Si el sha256 no coincide (el usuario lo editó) y difiere del embebido → `replace` con `NeedsConsent=true`. Si no existen → `create`.
  - `ApplyInit` ejecuta las acciones sin consentimiento y las aprobadas; el resto va a `Pending`. Actualiza `Tija.Installed` y `DefaultsVersion`. Resuelve las Choices con `answers` o con su default.

  `mergeGitignore`: detecta `\r\n` en `existing` y lo usa en lo agregado; agrega `# tija (local)` más las entradas que falten; no duplica; si el archivo no termina en salto de línea, agrega uno antes.

- [ ] **Step 1: Tests que fallan (manifest)**

```go
func TestFromDetectionFrontend(t *testing.T) // con Detection del fixture react-vite: Template=".github/PULL_REQUEST_TEMPLATE.md", BranchPrefixes = husky, Title.MaxLength=100, RequireScope=true, RequiredSections exactos, Quality["lint"].Provider="biome"
func TestManifestRoundTrip(t *testing.T) // Save→Load igual; Validate()==nil
func TestValidate(t *testing.T) // Version 2 → error; tier "ultra" → error; Target "" → error; Agents.Default "gpt" → error
func TestMissingFields(t *testing.T) // existing sin title.maxLength → ["pullRequest.title.maxLength"]
```

- [ ] **Step 2: Tests que fallan (gitignore)**

```go
func TestGitignoreAppends(t *testing.T)       // "node_modules\n" → agrega "# tija (local)\n.tija/cache/\n..." (4 entradas)
func TestGitignoreCRLFPreserved(t *testing.T) // "dist\r\n" → todo lo agregado termina en "\r\n"; no aparece "\n" sin "\r"
func TestGitignoreIdempotent(t *testing.T)    // segunda llamada → added vacío, out igual
func TestGitignoreNoTrailingNewline(t *testing.T) // "dist" → "dist\n# tija (local)\n..."
```

- [ ] **Step 3: Tests que fallan (engine init, con `testutil.NewRepo`)**

```go
func TestInitFrontendNonInteractive(t *testing.T) // fixture react-vite, approved=nil → Created incluye ".tija/manifest.yaml"; ".gitignore" está en Pending, no modificado; NO crea .tija/templates/pull-request.md (ya hay template)
func TestInitBackendWithYes(t *testing.T) // fixture dotnet-api, todas las acciones aprobadas → .github/PULL_REQUEST_TEMPLATE.md creado con el contenido embebido; manifest.PullRequest.Rules="templates/PULL_REQUEST_TEMPLATE_GUIDE.md"
func TestInitIdempotent(t *testing.T)       // dos ApplyInit con todo aprobado → la segunda tiene Created y Updated vacíos
func TestInitKeepsUserEditedRules(t *testing.T) // editar .tija/rules/pull-request.md y re-ejecutar sin aprobar → contenido intacto; acción replace en Pending
func TestInitFromSubdirectory(t *testing.T) // PlanInit desde <root>/src → plan.Root==root; no existe <root>/src/.tija
func TestInitNotARepo(t *testing.T)         // t.TempDir() → errors.Is(err, ErrNotARepo)
func TestInitMultiSlnChoice(t *testing.T)   // fixture multi-sln → Choice "solution" con Options ["A.sln","B.sln"]; answers{"solution":"B.sln"} → manifest.Project.Solution="B.sln"
func TestInitDoesNotOverwriteInvalidManifest(t *testing.T) // manifest con ":::" → tras ApplyInit con todo aprobado, bytes iguales; Notes contiene "manifest is invalid"
```

- [ ] **Step 4: Ejecutar** `go test ./internal/manifest/... ./internal/engine/...`. Expected: FAIL.
- [ ] **Step 5: Implementar** manifest, assets, agents y engine/init y gitignore. `Env.LookPath` en los tests es un fake que devuelve error para todo, salvo `git`.
- [ ] **Step 6: Ejecutar** `go test ./...`. Expected: PASS.
- [ ] **Step 7: Commit** `feat(engine): add manifest, embedded defaults and init plan/apply`

---

### Task 8: `doctor` del engine

**Files:**
- Create: `internal/engine/doctor.go`
- Test: `internal/engine/doctor_test.go`

**Interfaces:**
- Consumes: `gitx.FindRoot`, `manifest.Load/Validate`, `detect.Detect`, `agents.Available`, `sys.Env`.
- Produces:

```go
type CheckStatus string // "ok" | "warn" | "fail"
type Check struct { ID, Title string; Status CheckStatus; Hint string }
type DoctorReport struct { Root string; Checks []Check; Agents []string; DefaultAgent string }
func (r DoctorReport) ExitCode() int // 1 si algún fail, si no 0
func Doctor(ctx context.Context, dir string, env sys.Env) (DoctorReport, error)
```

  Checks en este orden (ID → regla):
  - `git.installed`: fail si `LookPath("git")` falla.
  - `repo.detected`: fail fuera de un repo, y en ese caso se omite el resto salvo `gh.*` y `agents`.
  - `manifest.present`: fail con Hint `"run: tija init"`.
  - `manifest.valid`: fail con el error de parseo o de Validate.
  - `tool.<provider>` por cada entrada de `Quality`: biome, eslint, prettier, vitest, jest y playwright se buscan en `node_modules/.bin/<n>` (con `.cmd`/`.exe` en Windows) o en PATH; dotnet-* en `dotnet`. Además, el binario del package manager (`bun`, `npm`…).
  - `gh.installed` (fail) y `gh.auth` (`gh auth status` con exit ≠ 0 → fail, Hint `"run: gh auth login"`).
  - `agents.available`: warn si no hay ninguno; Title con la lista y el default.
  - `pr.template`: fail si el archivo de `PullRequest.Template` no existe.
  - `pr.governance`: warn informativo si `PRGovernanceWorkflow != ""`, con Title `"verify manifest rules match <workflow>"`.
  - `lockfiles.multiple`: warn si `len(Lockfiles)>1`.
  - `tests.none`: warn `"no tests detected"` si `TestCount==0`.

  Los Titles en inglés siguen el spec ("Git installed", "repository detected", "manifest valid"…).

- [ ] **Step 1: Tests que fallan**

```go
func TestDoctorFrontendNoGhNoAgents(t *testing.T) // fixture react-vite inicializado; LookPath fake: solo git; → gh.installed fail, agents.available warn, lockfiles.multiple warn, tests.none warn, ExitCode()=1
func TestDoctorAllGood(t *testing.T) // LookPath encuentra git, gh, dotnet, claude, codex; Run fake: gh auth status exit 0 → sin fails, Agents=["claude","codex"], DefaultAgent="claude", ExitCode()=0
func TestDoctorNotInitialized(t *testing.T) // manifest.present fail con Hint "run: tija init"
func TestDoctorInvalidManifest(t *testing.T) // manifest ":::" → manifest.valid fail con mensaje no vacío; sin panic
func TestDoctorOutsideRepo(t *testing.T) // repo.detected fail; no hay checks manifest.*
```

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar** `Doctor`.
- [ ] **Step 4: Ejecutar** `go test ./internal/engine/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(engine): add doctor checks`

---

### Task 9: `status` del engine

**Files:**
- Create: `internal/engine/status.go`
- Test: `internal/engine/status_test.go`

**Interfaces:**
- Consumes: `gitx.*`, `branch.Classify`, `manifest.Load`, `detect.Detect` (para `HuskyBranchHook`), el target por defecto de Task 7 (`defaultTarget(ctx, repo gitx.Repo) string`, mismo paquete).
- Produces:

```go
type StatusReport struct {
    Root          string
    Branch        branch.Verdict
    Target        string
    TargetFound   bool
    Ahead, Behind int
    Staged, Unstaged, Untracked int
    HasRemote, Published, HasCommits bool
    HuskyWarning  bool // HuskyBranchHook && Branch.State=="nonconforming"
}
func Status(ctx context.Context, dir string, env sys.Env) (StatusReport, error)
func (r StatusReport) ExitCode(strict bool) int // 1 solo si strict y State ∈ {nonconforming, detached}
```

  Si hay manifest válido se usan sus `BranchPrefixes` y su `Target`. Si no, `branch.DefaultPrefixes` y el target por defecto de Task 7. El número de issue queda en `Branch.IssueNumber`; persistirlo para la Fase 3 queda fuera de esta fase.

- [ ] **Step 1: Tests que fallan**

```go
func TestStatusConforming(t *testing.T) // rama "feat/shared-imputation-form", 1 commit sobre main → State conforming, Ahead=1, ExitCode(true)=0
func TestStatusIssueBranch(t *testing.T) // rama "101-fixclaims-fix-integration-issues" → nonconforming, Suggested="fix/101-claims-integration-issues", ExitCode(false)=0, ExitCode(true)=1
func TestStatusHuskyWarning(t *testing.T) // fixture react-vite inicializado + rama "changes" → HuskyWarning=true
func TestStatusNoRemoteNoCommits(t *testing.T) // WithoutCommit, sin remoto → sin error, HasCommits=false, HasRemote=false, TargetFound=false
func TestStatusBaseBranch(t *testing.T) // en "main" → State base
```

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar** `Status`.
- [ ] **Step 4: Ejecutar** `go test ./internal/engine/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(engine): add status report with branch classification`

---

### Task 10: Comandos CLI `init`, `doctor` y `status` con render

**Files:**
- Create: `internal/cli/init.go`, `internal/cli/doctor.go`, `internal/cli/status.go`, `internal/cli/prompt.go`, `internal/render/init.go`, `internal/render/doctor.go`, `internal/render/status.go`
- Test: `internal/cli/commands_test.go`, `internal/render/golden_test.go` con `internal/render/testdata/*.golden`

**Interfaces:**
- Consumes: `engine.PlanInit/ApplyInit/Doctor/Status`, `render.Printer`, `render.ModeFor`.
- Produces:
  - `render.InitResult(p Printer, r engine.InitResult)`, `render.Doctor(p Printer, r engine.DoctorReport)`, `render.Status(p Printer, r engine.StatusReport)`. En modo JSON serializan el struct; agregar tags `json:"..."` en camelCase a los structs del engine.
  - `cli.prompter interface { Confirm(a engine.Action) bool; Choose(c engine.Choice) string }`: implementación stdin con `[y/N]` cuando es interactivo.
  - Flags: `init` no tiene flags propios (usa `--yes` y `--json`); `status --strict`.

  Comportamiento:
  - `init`: con TTY y sin `--yes`/`--json` → pregunta las Choices y cada acción con `NeedsConsent`, mostrando el `Diff`. Con `--yes` aprueba todo y usa las Choices por defecto. Sin TTY ni `--yes` → no aprueba nada. `ErrNotARepo` → exit 2.
  - La salida de `init` sigue el formato del spec (bloques "Detected project", "Created:", "Updated:" y además "Pending (run with --yes or interactively):").
  - `doctor` → `ExitCode()`.
  - `status` → `ExitCode(strict)`. Para ramas nonconforming imprime `! branch "<name>" — <reason>`, `  suggested: <Suggested>` y `  rename: git branch -m <name> <Suggested>`. Si `HuskyWarning`, imprime `! this repo's pre-push hook will reject this branch`.

- [ ] **Step 1: Tests golden que fallan (modo Plain)** con `-update` para regenerarlos: `TestRenderInitPlain`, `TestRenderDoctorPlain` y `TestRenderStatusIssueBranchPlain`. Los golden deben contener las líneas literales de arriba y `Language:       JavaScript`.
- [ ] **Step 2: Tests de comandos que fallan**

```go
func TestInitCommandNonTTY(t *testing.T) // repo react-vite, IsTTY=false → code 0; out contiene "Pending"; .gitignore sin cambios
func TestInitCommandYes(t *testing.T)    // --yes → .gitignore contiene ".tija/cache/"
func TestInitOutsideRepoExit2(t *testing.T)
func TestDoctorJSON(t *testing.T)        // --json → JSON válido con "checks"; sin "\x1b["
func TestStatusStrictExit1(t *testing.T) // rama "changes", --strict → code 1; sin --strict → 0
```

- [ ] **Step 3: Ejecutar** `go test ./internal/cli/... ./internal/render/...`. Expected: FAIL.
- [ ] **Step 4: Implementar.**
- [ ] **Step 5: Ejecutar** `go test ./...`. Expected: PASS.
- [ ] **Step 6: Verificación manual en los repos reales, solo lectura:** `go run ./cmd/tija doctor` y `go run ./cmd/tija status` con cwd en `C:\Users\DELL\Desktop\code\Team4soft\Front\v0\frontend` y en `C:\Users\DELL\Desktop\code\Team4soft\GMX\azure 2\github\ClaimsAPI`. **No ejecutar `init` en esos repos sin permiso del usuario.** Expected: el frontend sale como React 19 + Vite 7, bun, `! multiple lockfiles`; ClaimsAPI como .NET 8 con `ClaimsService.sln`.
- [ ] **Step 7: Commit** `feat(cli): wire init, doctor and status commands with plain, styled and JSON output`

---

### Task 11: TUI esqueleto

**Files:**
- Create: `internal/tui/model.go`, `internal/tui/menu.go`, `internal/tui/mascot.go`, `internal/tui/startup.go`
- Modify: `internal/cli/root.go` (por defecto `Deps.RunTUI = tui.Run`)
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `engine.*`, `render.*` (para el texto de resultados), `cli.Version`, que se pasa como parámetro para no crear un import cíclico.
- Produces:

```go
type MascotState string // "idle" | "scanning" | "success" | "error"
func Sprite(s MascotState) string // half-blocks ▀▄█ con la paleta; ≤16 columnas, ≤8 filas
type MenuItem struct { Key, Title, Description string }
func MenuItems(initialized bool) []MenuItem
func Run(dir, version string, env sys.Env) error
// model interno: handleKey(k string) (model, tea.Cmd), con "up","down","k","j","enter","esc","q","ctrl+c"
```

  `MenuItems(false)` = `[Init, Doctor, Status]`; `MenuItems(true)` = `[Doctor, Status, Init]`. Descripciones: Init `"Detect the stack and set up .tija/"`, Doctor `"Check tools, manifest and agents"`, Status `"Branch, commits and working tree"`.

  Startup (concepto 06 de `docs/reference/brand/brand-exploration-terminal.png`): una barra superior con `tija  <ruta>` y `v<version>`. La columna izquierda tiene el gato ASCII del draft §31, el logo `TIJA` en Orange, el tagline `Repository engineering for humans & agents` y un checklist de detección (`✓ <ProjectName> detected`, `✓ <Framework>`, `✓ GitHub` si aplica, `✓ <N> tests` o `! no tests detected`). La columna derecha tiene `What do you want to do?` y el menú con `>` en el ítem seleccionado y su descripción. Con menos de 80 columnas, las columnas se apilan. Durante la detección inicial se muestra la mascota en `scanning`; con el menú, en `idle`.

  Al elegir una opción se ejecuta el engine en un `tea.Cmd` y se muestra el resultado con `render` en modo Styled, más la mascota en `success` (ExitCode 0) o `error`. `esc` vuelve al menú. Init desde la TUI hace PlanInit; si hay acciones con `NeedsConsent`, muestra la lista con su diff y `y`/`n` por acción; las Choices se responden con las flechas.

- [ ] **Step 1: Tests que fallan**

```go
func TestMenuItemsOnlyPhase1(t *testing.T) // títulos de MenuItems(false) == ["Init","Doctor","Status"]; MenuItems(true) == ["Doctor","Status","Init"]
func TestSpritesDistinctAndBounded(t *testing.T) // 4 estados distintos entre sí; cada línea ≤16 celdas (lipgloss.Width); ≤8 líneas
func TestNavigation(t *testing.T) // model con menú, "down" → cursor 1, "up" en 0 → se queda en 0, "q" → tea.Quit
func TestStartupViewContents(t *testing.T) // View() con Detection fake contiene "TIJA", "Repository engineering for humans & agents", "What do you want to do?", "> Init"
```

- [ ] **Step 2: Ejecutar** `go test ./internal/tui/...`. Expected: FAIL.
- [ ] **Step 3: Implementar.** `go get` de Bubble Tea. Usar la versión mayor estable que resuelva; `handleKey` recibe `msg.String()` para aislar los tests del tipo de mensaje de teclado de cada versión.
- [ ] **Step 4: Ejecutar** `go test ./...`. Expected: PASS.
- [ ] **Step 5: Verificación manual:** `go run ./cmd/tija` en una terminal Windows (Windows Terminal/WezTerm) dentro de un repo de prueba. Navegar, ejecutar Doctor y volver con `esc`. `go run ./cmd/tija | cat` → muestra la ayuda sin TUI.
- [ ] **Step 6: Commit** `feat(tui): add startup screen, phase 1 menu and mascot states`

---

### Task 12: Distribución, CI y README

**Files:**
- Create: `.goreleaser.yaml`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes: `cli.Version` (ldflags `-X github.com/Ps3udoDev/tija-cli/internal/cli.Version={{.Version}}`).

- [ ] **Step 1: Escribir `ci.yml`.** Se dispara con `push` a `main` y `pull_request`, en una matriz `ubuntu-latest`, `windows-latest` y `macos-latest`, con `actions/setup-go` (go-version-file `go.mod`) y los pasos `go vet ./...` y `go test ./...`.
- [ ] **Step 2: Escribir `.goreleaser.yaml`.** Build de `./cmd/tija`, `CGO_ENABLED=0`, goos windows/linux/darwin, goarch amd64/arm64. Archives zip en Windows y tar.gz en el resto, y checksums.
- [ ] **Step 3: Escribir `release.yml`.** Se dispara con tags `v*` y usa `goreleaser/goreleaser-action` con `GITHUB_TOKEN` y `permissions: contents: write`.
- [ ] **Step 4: Validar** `goreleaser check`, si está instalado (si no, `go run github.com/goreleaser/goreleaser/v2@latest check`). Expected: `config is valid`. Luego `go build -ldflags "-X github.com/Ps3udoDev/tija-cli/internal/cli.Version=0.1.0" -o tija.exe ./cmd/tija && ./tija.exe version`. Expected: `tija 0.1.0`.
- [ ] **Step 5: README.** Tagline *Your repository. Your rules. Any agent.*, instalación (`go install github.com/Ps3udoDev/tija-cli/cmd/tija@latest` y Releases), los comandos `init`/`doctor`/`status` con flags y exit codes, y un enlace al spec.
- [ ] **Step 6: Ejecutar** `go test ./... && go vet ./...`. Expected: PASS.
- [ ] **Step 7: Commit** `ci: add CI matrix, GoReleaser config and README`

---

## Criterios de salida (del spec), cómo se verifican

| Criterio | Verificación |
|---|---|
| `init` detecta stack y template e idempotente | Tasks 4, 5 y 7 (tests). Prueba real de `init` en una copia o rama de los repos del autor, con su permiso |
| `doctor` reporta gh y agentes en Windows | Task 8 (tests) y Task 10, Step 6 (manual) |
| Clasificación de ramas y `--strict` | Tasks 6, 9 y 10 |
| TUI solo en TTY; `--json`; `doctor \| cat` | Tasks 1, 10 y 11 |
| Binarios para los 3 SO desde un tag | Task 12 |
| Tests con fixtures .NET, React/Vite, vacío y sin git | Tasks 3, 4, 7, 8 y 9 |
