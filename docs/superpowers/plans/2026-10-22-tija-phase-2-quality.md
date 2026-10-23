# Tija Fase 2 — Quality: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar `tija check`, `tija lint [--fix]`, `tija format` y `tija test`. Deben correr las herramientas correctas en repos .NET y React/Vite sin configuración manual, con `--json` estable. Cada ejecución guarda un resultado estructurado en `.tija/session/` para que la Fase 3 cite pruebas reales.

**Architecture:**
- Un paquete nuevo, `internal/quality`, traduce la configuración efectiva en invocaciones concretas.
  - La configuración efectiva sale del manifest y, si falta, de la detección y de los scripts de `package.json`.
  - Cada provider tiene un adapter (`biome`, `dotnet`, `dotnet-format`, `dotnet-test`, `vitest`, `jest`, `eslint`, `prettier`, `script`).
  - El paquete también lee los reportes de tests.
- `internal/engine` ejecuta las invocaciones en orden, emite eventos de progreso, guarda logs y la sesión, y devuelve un `QualityReport`.
- CLI y TUI son capas finas sobre `engine.RunQuality`, igual que en la Fase 1.

**Tech Stack:** Go 1.27, Cobra, Bubble Tea v2 / Lip Gloss v2 (ya en `go.mod`), `encoding/xml` y `encoding/json` de la stdlib. No se agregan dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-10-22-tija-phases-design.md`: sección 5, Fase 2, más las secciones 2 a 4. Léelo junto con este plan.

## Global Constraints

- Todo lo de la Fase 1 sigue vigente:
  - módulo `github.com/Ps3udoDev/tija-cli`;
  - UI en inglés, paleta C y símbolos `✓ ✗ ! >`;
  - exit codes `0` ok, `1` fallo de check, `2` uso o entorno;
  - sin TTY, o con `--json`, `--no-color` o `NO_COLOR`: sin colores ni TUI;
  - Windows es plataforma de primera clase.
- **Deterministic first:** ningún comando de esta fase invoca a un agente de IA.
- Tija **orquesta, no reemplaza.** Usa siempre el binario local del repo (`node_modules/.bin`) o el del PATH. Nunca descarga herramientas: nada de `npx`/`bunx` sin instalar ni `dotnet tool install`.
- Orden fijo de las kinds: `build` → `lint` → `format` → `tests`. La kind `e2e` del manifest se ignora en esta fase (es de la Fase 6).
- Archivos que escribe Tija en esta fase:
  - `.tija/session/quality.json`, `.tija/session/logs/<kind>.log` y `.tija/tmp/quality/`, todos ignorados por git desde `init`.
  - **Solo si existe `.tija/manifest.yaml`.** En un repo sin inicializar, `check` corre igual y no crea nada en disco.
- `lint --fix` y `format` modifican archivos del repo. Llamarlos es la intención explícita del usuario, así que no piden consentimiento. Al terminar listan los archivos que cambiaron.
- El menú de la TUI pasa a ser: `Check`, `Tests`, `Doctor`, `Status`, `Init` (con `Init` primero si el repo no está inicializado). No aparece nada de fases futuras.
- Pendientes de la Fase 1 que entran aquí:
  - scroll y wrap del resultado en la TUI;
  - envelope JSON de errores;
  - que la TUI muestre el error real de `FindRoot`.

## Review Focus

1. **Repo sin `.tija/`:** `tija check` corre con la configuración que sale de la detección, imprime `! not initialized — results are not saved (run: tija init)` y no crea `.tija/`. Lo cubre Task 5 (`TestRunQualityUninitializedWritesNothing`).
2. **Herramienta declarada pero no instalada** (`node_modules` sin instalar): el paso queda `unavailable` con el Hint `run: bun install` (o el del package manager), sin panic. Los demás pasos corren y el exit es 2 si no hubo fallos reales. Lo cubren Task 5 (`TestRunQualityUnavailableTool`) y Task 3 (`TestLookupBinWindowsShim`).
3. **Shims `.cmd` de `node_modules/.bin` en Windows:** `biome.cmd` se resuelve y se ejecuta. Lo cubre Task 3 (`TestLookupBinWindowsShim`, que corre en todos los OS con un `goos` inyectado).
4. **Herramienta colgada o Ctrl+C:** el timeout por paso (default 10 min, `--timeout`) mata el proceso. El paso queda `failed` con `TimedOut=true` y los siguientes se marcan `skipped`. Lo cubre Task 5 (`TestRunQualityTimeout`).
5. **Salida enorme** (un `dotnet build` con miles de líneas): la salida completa va al log y en memoria solo se guardan las últimas 40 líneas. La CLI muestra esa cola y la ruta del log. Lo cubren Task 1 (`TestTailWriter`) y Task 5 (`TestRunQualityLogsAndTail`).

---

## Estructura de archivos

```text
internal/sys/env.go                 → + Stream (salida combinada a un io.Writer)
internal/sys/tail.go                → TailWriter (últimas N líneas)
internal/gitx/gitx.go               → + Head, StatusPaths
internal/detect/node.go             → + Scripts
internal/manifest/manifest.go       → QualityEntry.Script, DetectedQuality, validación de quality
internal/quality/kind.go            → Kind, Order, Invocation
internal/quality/bin.go             → LookupBin (node_modules/.bin y PATH, shims de Windows)
internal/quality/adapters.go        → Adapter y los 9 providers
internal/quality/resolve.go         → Resolve: manifest + detección + scripts → []Step
internal/quality/reports.go         → ParseTRX, ParseJestJSON (también para vitest) → TestCounts
internal/quality/session.go         → Session, LoadSession, SaveSession
internal/quality/testdata/          → reportes de ejemplo (.trx, json)
internal/engine/quality.go          → RunQuality, QualityReport, eventos
internal/engine/doctor.go           → usa quality.LookupBin y quality.Resolve
internal/render/quality.go          → Quality (plain/styled/json)
internal/render/error.go            → JSONError
internal/cli/quality.go             → check, lint, format, test
internal/cli/root.go                → envelope JSON de errores
internal/tui/{menu,model,mascot,quality_view,scroll}.go
README.md
```

---

### Task 1: `sys.Stream`, `TailWriter` y `gitx.Head/StatusPaths`

**Files:**
- Modify: `internal/sys/env.go`, `internal/gitx/gitx.go`
- Create: `internal/sys/tail.go`
- Test: `internal/sys/env_test.go`, `internal/sys/tail_test.go`, `internal/gitx/gitx_test.go`

**Interfaces:**
- Produces:

```go
// Stream runs name in dir writing stdout and stderr, interleaved, to out.
// Same exit-code contract as Runner. A cancelled ctx kills the process.
type Streamer func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (exitCode int, err error)
type Env struct { Run Runner; Stream Streamer; LookPath func(string) (string, error); Getenv func(string) string }

// TailWriter keeps the last Max lines written to it.
type TailWriter struct { Max int /* … */ }
func (w *TailWriter) Write(p []byte) (int, error)
func (w *TailWriter) Lines() []string // sin "\r", sin la línea vacía final

func (r Repo) Head(ctx context.Context) (sha string, ok bool)   // git rev-parse HEAD; ok=false sin commits
func (r Repo) StatusPaths(ctx context.Context) []string          // rutas de `git status --porcelain=v1 -z -uall`, '/' y ordenadas; en renames, la ruta destino
```

  `Default()` llena `Stream` con una implementación basada en `exec.CommandContext` que asigna el mismo writer a `Stdout` y `Stderr`. En Windows, fija `cmd.WaitDelay = 2 * time.Second` para que un hijo huérfano no bloquee la espera.

- [ ] **Step 1: Tests que fallan**

```go
func TestStreamCombinesOutputAndExitCode(t *testing.T) // `go env GOROOT` → exit 0 y salida no vacía; `go nosuchcmd` → exit ≠ 0, err nil
func TestStreamNotFound(t *testing.T)                  // binario inexistente → exitCode -1, err != nil
func TestStreamCancel(t *testing.T)                    // ctx con timeout de 200ms sobre un proceso que duerme (helper TestMain con -sleep) → retorna en < 5s
func TestTailWriter(t *testing.T)                      // Max 3; escribir "a\nb\r\nc\nd\ne" en trozos partidos a mitad de línea → Lines() == ["c","d","e"]; 100k líneas no superan Max en memoria
func TestHeadAndStatusPaths(t *testing.T)              // testutil repo: Head ok y 40 hex; sin commits → ok=false; archivo nuevo "a b/ñ.txt" y modificado → StatusPaths ordenado incluye "a b/ñ.txt"
```

- [ ] **Step 2: Ejecutar** `go test ./internal/sys/... ./internal/gitx/...`. Expected: FAIL (no compila).
- [ ] **Step 3: Implementar.** `TestStreamCancel` usa el patrón de proceso helper (`os.Args[0]` con `-test.run=TestHelperProcess`) para no depender de `sleep` del sistema.
- [ ] **Step 4: Ejecutar.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(sys): stream combined process output and add git head/status paths`

---

### Task 2: Scripts de `package.json` y quality en el manifest

**Files:**
- Modify: `internal/detect/detect.go`, `internal/detect/node.go`, `internal/manifest/manifest.go`
- Test: `internal/detect/detect_test.go`, `internal/manifest/manifest_test.go`

**Interfaces:**
- Produces:

```go
// detect.Detection
Scripts []string `json:"scripts"` // nombres de package.json "scripts", ordenados

// manifest
type QualityEntry struct {
    Provider string `yaml:"provider"`
    Script   string `yaml:"script,omitempty"` // solo con provider "script"
}
var QualityProviders = []string{"biome", "eslint", "prettier", "dotnet", "dotnet-format", "dotnet-test", "vitest", "jest", "playwright", "script", "none"}
var QualityKinds = []string{"build", "lint", "format", "tests", "e2e"}
// DetectedQuality es el mapa que hoy arma FromDetection a partir de qualityByTool, extraído para reutilizarlo.
func DetectedQuality(d detect.Detection) map[string]QualityEntry
```

  `FromDetection` usa `DetectedQuality` y su salida no cambia. `Validate` agrega estos errores, en orden de kind:
  - `quality.<kind>: unknown kind "<k>" (want one of …)`;
  - `quality.<kind>.provider: invalid value "<p>" (want one of …)`;
  - `quality.<kind>.script: required for provider "script"`.

- [ ] **Step 1: Tests que fallan**

```go
func TestDetectNodeScripts(t *testing.T)             // package.json con scripts {"lint":…, "test":…, "build":…} → Scripts == ["build","lint","test"]; sin "scripts" → nil
func TestDetectedQualityMatchesFromDetection(t *testing.T) // fixture react-vite: DetectedQuality(d) == FromDetection(d,…).Quality
func TestValidateQuality(t *testing.T)               // {"lint":{biome}} ok; {"lnt":{biome}} → error unknown kind; {"lint":{"rome"}} → invalid provider; {"tests":{"script"}} → script required; {"tests":{"none"}} ok
```

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar** `go test ./internal/detect/... ./internal/manifest/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(manifest): validate quality entries and record package.json scripts`

---

### Task 3: Paquete `quality`: kinds, `LookupBin`, adapters y `Resolve`

**Files:**
- Create: `internal/quality/{kind,bin,adapters,resolve}.go`
- Test: `internal/quality/{bin,adapters,resolve}_test.go`

**Interfaces:**
- Consumes: `manifest.QualityEntry`, `manifest.DetectedQuality`, `detect.Detection` (`Root`, `PackageManager`, `Solution`, `Scripts`).
- Produces:

```go
type Kind string
const (Build Kind = "build"; Lint Kind = "lint"; Format Kind = "format"; Tests Kind = "tests")
var Order = []Kind{Build, Lint, Format, Tests}

type Mode int
const (CheckMode Mode = iota; FixMode)

// Invocation is one process to run; it may cover several kinds (biome ci = lint + format).
type Invocation struct {
    Kinds    []Kind
    Provider string
    Bin      string   // nombre lógico ("biome", "dotnet", "bun")
    Path     string   // ejecutable resuelto; "" si no está disponible
    Args     []string
    Report   string   // ruta absoluta del reporte de tests a leer después ("" si no hay)
    Hint     string   // cómo instalarlo si Path == ""
}
func (i Invocation) Display() string // "biome ci ." — Bin + Args, con comillas en args con espacios

type Project struct { Root, PackageManager, Solution string; Scripts []string; TmpDir string; GOOS string }
type Adapter interface {
    Provider() string
    // Plan returns the invocation for kinds in mode, or ok=false when the provider cannot do it (p. ej. Fix de dotnet-test).
    Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool)
}
func AdapterFor(provider string) (Adapter, bool)

// LookupBin resolves name: root/node_modules/.bin/<name> (on Windows, <name>.cmd and <name>.exe first) for node tools, then lookPath.
func LookupBin(root, name, goos string, lookPath func(string) (string, error)) (string, bool)
var NodeTools = []string{"biome", "eslint", "prettier", "vitest", "jest", "playwright"}

type Source string // "manifest" | "detected" | "script"
type Step struct { Kind Kind; Entry manifest.QualityEntry; Source Source }
// Resolve returns the effective configuration for kinds (nil = all of Order):
// manifest entry if present (provider "none" drops the kind); otherwise DetectedQuality(d);
// otherwise, for Lint and Tests only, a package.json script named "lint" / "test".
func Resolve(m *manifest.Manifest, d detect.Detection, kinds []Kind) []Step
// Group merges consecutive steps with the same provider into one invocation when the adapter supports it.
func Plan(p Project, steps []Step, mode Mode, lookPath func(string) (string, error)) []Invocation
```

  Comandos exactos por adapter (`<sln>` = `p.Solution`, que se omite si está vacío; `<pm>` = `p.PackageManager`):

| provider | check | fix | kinds juntas |
|---|---|---|---|
| `biome` | lint+format: `biome ci .`; lint: `biome lint .`; format: `biome format .` | lint+format: `biome check --write .`; lint: `biome lint --write .`; format: `biome format --write .` | lint+format |
| `eslint` | `eslint .` | `eslint . --fix` | — |
| `prettier` | `prettier --check .` | `prettier --write .` | — |
| `dotnet` (build) | `dotnet build <sln> --nologo` | — | — |
| `dotnet-format` | `dotnet format <sln> --verify-no-changes` | `dotnet format <sln>` | — |
| `dotnet-test` | `dotnet test <sln> --nologo --logger trx --results-directory <TmpDir>/trx`, Report = `<TmpDir>/trx` | — | — |
| `vitest` | `vitest run --reporter=default --reporter=json --outputFile.json=<TmpDir>/vitest.json` | — | — |
| `jest` | `jest --json --outputFile=<TmpDir>/jest.json` | — | — |
| `script` | `<pm> run <Script>` (npm si `<pm>` está vacío) | — | — |

  Los hints son `run: <pm> install` para las node tools, `install the .NET SDK: https://dot.net` para dotnet y `install <pm>` para script.

- [ ] **Step 1: Tests que fallan**

```go
func TestLookupBinWindowsShim(t *testing.T)  // tmp root con node_modules/.bin/biome.cmd, goos "windows" → path a biome.cmd; goos "linux" con .bin/biome → ese path; sin .bin y lookPath falla → ok=false
func TestLookupBinNonNodeUsesPath(t *testing.T) // "dotnet" no se busca en node_modules
func TestBiomeGroupsLintAndFormat(t *testing.T)  // steps lint+format biome → 1 Invocation, Kinds [lint format], Args ["ci","."]; FixMode → ["check","--write","."]
func TestDotnetCommands(t *testing.T)            // Solution "ClaimsService.sln" → build ["build","ClaimsService.sln","--nologo"], format ["format","ClaimsService.sln","--verify-no-changes"]; Solution "" → sin el argumento
func TestDotnetTestReportDir(t *testing.T)       // Args contienen "--logger","trx","--results-directory",<TmpDir>/trx; Report == <TmpDir>/trx
func TestScriptAdapter(t *testing.T)             // pm "bun", script "lint" → bun ["run","lint"]; pm "" → npm
func TestFixUnsupportedDropped(t *testing.T)     // FixMode con dotnet-test → no hay Invocation para tests
func TestResolvePrecedence(t *testing.T)         // manifest lint biome + detección eslint → biome (Source manifest); manifest tests none → sin tests; sin manifest y Scripts ["lint","test"] sin tools → lint/tests "script" (Source script); format nunca usa script
func TestResolveKindsFilter(t *testing.T)        // kinds [Tests] → solo tests
func TestResolveIgnoresE2E(t *testing.T)         // manifest e2e playwright → no aparece
func TestDisplayQuotes(t *testing.T)             // Args con "a b" → "\"a b\""
```

- [ ] **Step 2: Ejecutar** `go test ./internal/quality/...`. Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(quality): add tool adapters and effective configuration resolution`

---

### Task 4: Reportes de tests y sesión

**Files:**
- Create: `internal/quality/reports.go`, `internal/quality/session.go`, `internal/quality/testdata/{trx/a.trx,trx/b.trx,vitest.json,jest-failed.json}`
- Test: `internal/quality/reports_test.go`, `internal/quality/session_test.go`

**Interfaces:**
- Produces:

```go
type TestCounts struct { Total, Passed, Failed, Skipped int } // json: total, passed, failed, skipped
// ParseTRX sums <ResultSummary><Counters total passed failed …> of every *.trx in dir (notExecuted cuenta como Skipped).
func ParseTRX(dir string) (*TestCounts, error)
// ParseJestJSON reads numTotalTests/numPassedTests/numFailedTests/numPendingTests (+numTodoTests en Skipped); sirve para jest y vitest.
func ParseJestJSON(path string) (*TestCounts, error)
func ParseReport(provider, report string) (*TestCounts, error) // dotnet-test → ParseTRX; vitest/jest → ParseJestJSON; otro → nil, nil

type StepStatus string // "passed" | "failed" | "skipped" | "unavailable"
type StepResult struct {
    Kinds      []Kind      `json:"kinds"`
    Provider   string      `json:"provider"`
    Command    string      `json:"command"`
    Status     StepStatus  `json:"status"`
    ExitCode   int         `json:"exitCode"`
    TimedOut   bool        `json:"timedOut,omitempty"`
    DurationMs int64       `json:"durationMs"`
    StartedAt  time.Time   `json:"startedAt"`
    Commit     string      `json:"commit"`
    Dirty      bool        `json:"dirty"`
    Tests      *TestCounts `json:"tests,omitempty"`
    Log        string      `json:"log,omitempty"`  // ruta relativa al repo, '/'
    Hint       string      `json:"hint,omitempty"`
    Tail       []string    `json:"-"`
}
// Session is .tija/session/quality.json: the latest result per kind.
type Session struct { Version int `json:"version"`; Results map[Kind]StepResult `json:"results"` }
const SessionPath = ".tija/session/quality.json"
func LoadSession(root string) (Session, error)          // archivo ausente → Session{Version:1, Results:{}} y nil
func SaveSession(root string, results []StepResult) error // fusiona por kind con lo existente (un `tija test` no borra el lint previo); escribe vía archivo temporal + rename
```

  Un `StepResult` que cubre varias kinds (`biome ci .` = lint + format) se guarda bajo cada una de sus kinds. Phase 3 consume `LoadSession`. Para saber si un resultado corresponde al estado actual, compara `Commit` con HEAD y `Dirty==false`.

- [ ] **Step 1: Tests que fallan**

```go
func TestParseTRXSumsFiles(t *testing.T)    // a.trx total 3 passed 3 + b.trx total 2 passed 1 failed 1 → {5,4,1,0}
func TestParseTRXEmptyDir(t *testing.T)     // dir sin .trx → nil, nil
func TestParseJestJSON(t *testing.T)        // vitest.json → {12,11,0,1}; jest-failed.json → Failed 2
func TestParseReportMalformed(t *testing.T) // json inválido → error, sin panic
func TestSessionMergeByKind(t *testing.T)   // Save [lint passed] y luego Save [tests failed] → Load tiene lint y tests; el segundo Save de lint reemplaza al primero
func TestLoadSessionMissing(t *testing.T)   // → Version 1, Results vacío, err nil
```

  Los archivos de `testdata` son reportes reales mínimos: un TRX con `ResultSummary/Counters` y un JSON de vitest con los campos `num*`.

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar** `go test ./internal/quality/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(quality): parse test reports and persist quality session`

---

### Task 5: `engine.RunQuality`

**Files:**
- Create: `internal/engine/quality.go`
- Test: `internal/engine/quality_test.go`

**Interfaces:**
- Consumes: `gitx.FindRoot/Head/StatusPaths/WorkTree`, `manifest.Load/Validate`, `detect.Detect`, `quality.Resolve/Plan/ParseReport/SaveSession`, `sys.Env.Stream`, `sys.TailWriter`.
- Produces:

```go
type QualityRequest struct {
    Kinds   []quality.Kind // nil = todas
    Mode    quality.Mode
    Timeout time.Duration  // por paso; 0 → 10 * time.Minute
    Verbose io.Writer      // si no es nil, recibe también la salida en vivo de cada proceso
}
type QualityEventType string // "start" | "done"
type QualityEvent struct { Type QualityEventType; Index, Total int; Step quality.StepResult }
type QualityReport struct {
    Root         string               `json:"root"`
    Initialized  bool                 `json:"initialized"`
    Mode         string               `json:"mode"`        // "check" | "fix"
    Commit       string               `json:"commit"`
    Dirty        bool                 `json:"dirty"`
    Steps        []quality.StepResult `json:"steps"`
    ChangedFiles []string             `json:"changedFiles"` // solo en fix; [] si ninguno
    Warnings     []string             `json:"warnings"`
}
func (r QualityReport) ExitCode() int // 1 si algún failed; si no, 2 si algún unavailable o si no hay Steps; si no, 0
func RunQuality(ctx context.Context, dir string, env sys.Env, req QualityRequest, onEvent func(QualityEvent)) (QualityReport, error)
```

  Reglas:
  - Fuera de un repo, error con exit 2, igual que `status`.
  - El manifest se usa si existe y es válido.
    - Si existe pero es inválido: `Warnings += "manifest invalid — using detected tools"` y se usa la detección.
    - Si no existe: `Warnings += "not initialized — results are not saved (run: tija init)"`.
  - `TmpDir = <root>/.tija/tmp/quality`, que se vacía antes de cada paso con reporte. Si el repo no está inicializado, se usa `os.MkdirTemp` y se borra al final.
  - Cada invocación corre con `context.WithTimeout`. La salida va a un `TailWriter{Max: 40}` y, si el repo está inicializado, también a `.tija/session/logs/<primera-kind>.log`.
  - `Path == ""` → `unavailable` con el `Hint` del adapter, sin ejecutar nada.
  - Si `build` termina `failed`, los pasos siguientes quedan `skipped` sin ejecutarse. Lo mismo ocurre después de un timeout.
  - `Commit` y `Dirty` (`WorkTree` con algún contador > 0) se calculan una vez, **antes** de correr.
  - En `FixMode`:
    - antes y después se calculan `StatusPaths` y el sha256 de cada archivo listado;
    - `ChangedFiles` son las rutas cuyo hash cambió o que aparecieron;
    - no se guarda la sesión, porque un fix no prueba nada.
  - En `CheckMode` con el repo inicializado, `quality.SaveSession(root, steps)`.
  - Se llama a `onEvent` con `start` y `done` por paso. `onEvent` puede ser nil.

- [ ] **Step 1: Tests que fallan** (con `Stream` fake que registra las llamadas y escribe salida y exit codes predefinidos, sobre repos `testutil`)

```go
func TestRunQualityFrontendBiome(t *testing.T)         // fixture react-vite inicializado, node_modules/.bin/biome(.cmd) creado → 1 invocación "biome ci .", passed, session guardada con lint y format
func TestRunQualityDotnetBuildFailSkipsRest(t *testing.T) // fixture dotnet-api con manifest build+format+tests: build exit 1 → format y tests skipped, ExitCode 1
func TestRunQualityTestCounts(t *testing.T)            // el fake escribe un .trx en el --results-directory recibido → Steps[tests].Tests == {3,3,0,0}
func TestRunQualityUnavailableTool(t *testing.T)       // sin node_modules → biome unavailable, Hint "run: bun install", ExitCode 2; Stream no fue llamado para biome
func TestRunQualityUninitializedWritesNothing(t *testing.T) // repo sin .tija → corre, Warnings contiene "not initialized", y después del run no existe <root>/.tija
func TestRunQualityTimeout(t *testing.T)               // Stream fake bloquea hasta ctx.Done → failed, TimedOut, siguiente skipped
func TestRunQualityLogsAndTail(t *testing.T)           // fake escribe 1000 líneas → Tail tiene 40, el log tiene 1000, Log == ".tija/session/logs/build.log"
func TestRunQualityFixReportsChangedFiles(t *testing.T) // FixMode; el fake modifica src/a.js → ChangedFiles == ["src/a.js"], session no escrita
func TestRunQualityNothingConfigured(t *testing.T)     // fixture unknown → Steps vacío, ExitCode 2
func TestRunQualityEvents(t *testing.T)                // 2 pasos → eventos start,done,start,done con Index 0..1, Total 2
```

- [ ] **Step 2: Ejecutar** `go test ./internal/engine/ -run RunQuality`. Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar** `go test ./internal/engine/...`. Expected: PASS, incluidos los tests de la Fase 1.
- [ ] **Step 5: Commit** `feat(engine): run quality steps with timeouts, logs and session results`

---

### Task 6: `doctor` sobre la configuración efectiva

**Files:**
- Modify: `internal/engine/doctor.go`
- Test: `internal/engine/doctor_test.go`

**Interfaces:**
- Consumes: `quality.Resolve`, `quality.Plan`, `quality.LookupBin`.
- Produces: no cambia la firma de `Doctor`. Los checks `tool.<binary>` salen de las invocaciones de `quality.Plan(…, CheckMode, …)` sobre `quality.Resolve(m, d, nil)`, deduplicadas por `Bin`. Al final se agrega el package manager, como hoy. `toolBins`, `toolFound` y `nodeTools` se eliminan del engine.
  - Si una invocación tiene `Path == ""`, el check es fail con el `Hint` del adapter.
  - `tests.none` sigue usando `HasTests()`.

- [ ] **Step 1: Tests que fallan**

```go
func TestDoctorChecksScriptFallbackTool(t *testing.T) // sin manifest.quality, Scripts ["lint"], pm bun → hay check "tool.bun"
func TestDoctorMissingBiomeHint(t *testing.T)         // manifest lint biome sin node_modules → tool.biome fail, Hint "run: bun install"
```

  Los tests existentes de doctor (`TestDoctorAllGood`, etc.) deben seguir pasando sin cambios en sus aserciones.

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar** `go test ./internal/engine/...`. Expected: PASS.
- [ ] **Step 5: Commit** `refactor(doctor): check the tools of the effective quality configuration`

---

### Task 7: Render de quality y envelope JSON de errores

**Files:**
- Create: `internal/render/quality.go`, `internal/render/error.go`, `internal/render/testdata/{quality_check_plain,quality_fix_plain}.golden`
- Test: `internal/render/golden_test.go`

**Interfaces:**
- Consumes: `engine.QualityReport`, `quality.StepResult`.
- Produces:

```go
func Quality(p Printer, r engine.QualityReport)
func QualityProgress(p Printer, ev engine.QualityEvent) // una línea por evento "done" en Plain/Styled; nada en JSON
func JSONError(w io.Writer, code int, err error)        // {"error":{"code":2,"message":"…"}}
```

  Formato plain de `Quality`:

```text
Check
✓ build          dotnet build ClaimsService.sln --nologo                 12.4s
✗ format         dotnet format ClaimsService.sln --verify-no-changes      3.1s
    <hasta 40 líneas de Tail, con sangría de 4 espacios>
    log: .tija/session/logs/format.log
- tests          skipped (build failed)
! lint           biome unavailable — run: bun install
✓ tests          vitest run …                       8.2s  12 passed, 1 skipped

! not initialized — results are not saved (run: tija init)
```

  - El título es `Check` o `Fix`.
  - Kinds juntas se muestran como `lint + format`.
  - Duración: `%.1fs` por debajo de un minuto; desde un minuto, `1m05s`.
  - `skipped` usa el símbolo `-` en `Sand`.
  - Sin `Steps` se imprime `! nothing to run — no quality tools configured or detected`.
  - En Fix se agrega `Changed files:` con la lista, o `No files changed`.
  - En JSON se emite el `QualityReport` como un único documento. `steps`, `changedFiles` y `warnings` nunca son `null`.

- [ ] **Step 1: Tests que fallan**

```go
func TestRenderQualityCheckPlain(t *testing.T) // golden con passed, failed con tail+log, skipped, unavailable y warning
func TestRenderQualityFixPlain(t *testing.T)   // golden con Changed files
func TestRenderQualityJSONStable(t *testing.T) // report vacío → "steps":[], "changedFiles":[], "warnings":[]; un solo documento JSON
func TestFormatDuration(t *testing.T)          // 1240ms → "1.2s"; 65s → "1m05s"
func TestJSONError(t *testing.T)               // code 2, "not a git repository" → {"error":{"code":2,"message":"not a git repository"}}
```

- [ ] **Step 2: Ejecutar** `go test ./internal/render/...`. Expected: FAIL.
- [ ] **Step 3: Implementar.** Para generar los goldens, `go test ./internal/render/ -update`, con el flag que ya existe.
- [ ] **Step 4: Ejecutar.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(render): render quality reports and JSON error envelopes`

---

### Task 8: Comandos CLI `check`, `lint`, `format`, `test`

**Files:**
- Create: `internal/cli/quality.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/commands_test.go`, `internal/cli/root_test.go`

**Interfaces:**
- Consumes: `engine.RunQuality`, `render.Quality`, `render.QualityProgress`, `render.JSONError`.
- Produces: estos comandos.

| comando | Kinds | Mode |
|---|---|---|
| `tija check` | todas | Check |
| `tija lint` | lint | Check |
| `tija lint --fix` | lint, format | Fix |
| `tija format` | format | Fix |
| `tija test` | tests | Check |

  - Flags locales: `--timeout duration` (default `10m`) y `--verbose`, que manda la salida en vivo al stderr. En `--json`, `--verbose` se ignora.
  - Exit code = `QualityReport.ExitCode()`.
  - Envelope JSON de errores en `ExecuteWith`: si `--json` está activo y hay un error con mensaje, se escribe `render.JSONError(out, code, err)` en stdout en lugar de `✗ msg` en stderr. Aplica a todos los comandos, también a los de la Fase 1.

- [ ] **Step 1: Tests que fallan**

```go
func TestCheckCommandJSON(t *testing.T)      // Deps con Stream fake; `check --json` → JSON válido con "mode":"check", exit = ExitCode()
func TestLintFixRunsFixMode(t *testing.T)    // `lint --fix` → el Stream fake recibe "check","--write","." para biome
func TestTestCommandOnlyTests(t *testing.T)  // `test` → solo se invoca el adapter de tests
func TestCheckTimeoutFlag(t *testing.T)      // `check --timeout x` → exit 2 (error de flag)
func TestJSONErrorEnvelope(t *testing.T)     // `status --json` fuera de un repo → stdout {"error":{"code":2,…}}, stderr vacío, exit 2
```

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar** `go test ./internal/cli/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(cli): add check, lint, format and test commands`

---

### Task 9: TUI: Check y Tests, progreso en vivo, mascota `warning` y scroll

**Files:**
- Modify: `internal/tui/menu.go`, `internal/tui/model.go`, `internal/tui/mascot.go`, `internal/tui/startup.go`
- Create: `internal/tui/quality_view.go`, `internal/tui/scroll.go`
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `engine.RunQuality` con `onEvent`, `render.Quality`.
- Produces:
  - **Menú:** `MenuItems(initialized)` → `[Check, Tests, Doctor, Status, Init]`, o `[Init, Check, Tests, Doctor, Status]` sin inicializar.
    - `Check`: "Lint, format and tests".
    - `Tests`: "Run the test suite".
  - **Mascota:** nuevo `Warning MascotState = "warning"`, con un sprite de 8 líneas en la paleta C. Es el gato con un `!` en lugar de la `✓` o la `✗`, fiel al estilo de los otros tres.
  - **Pantalla de progreso** (`screenQuality`): una línea por paso.
    - Estados: pendiente `·`, corriendo con spinner (`tea.Tick` de 120ms, frames `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`), `✓`, `✗`, `-` o `!`.
    - Cada línea lleva el comando y la duración. La mascota está en `Scanning`.
    - Los eventos llegan por un `chan engine.QualityEvent` que se lee con un `tea.Cmd` que se re-encola.
  - **Mascota en el resultado:** `Success` si el exit es 0, `Warning` si es 1 o 2 con algún paso ejecutado, `Error` si el engine devolvió error. Doctor con solo `warn` también muestra `Warning`.
  - **Scroll del resultado:**
    - el cuerpo se ajusta (wrap) al ancho disponible;
    - si no cabe en `height - 6`, se recorta con offset (`↑/↓`, `pgup/pgdn`, `home/end`);
    - el footer pasa a `↑/↓ scroll · esc back · q quit`.
  - **Error de `FindRoot`:** la startup screen muestra `! <mensaje real>` en lugar de solo `not a git repository` cuando el error no es "not a repo".

- [ ] **Step 1: Tests que fallan**

```go
func TestMenuItemsPhase2(t *testing.T)          // initialized → Check, Tests, Doctor, Status, Init; no inicializado → Init primero
func TestQualityProgressView(t *testing.T)      // modelo en screenQuality con eventos start(0), done(0 passed), start(1) → vista contiene "✓ build" y un frame del spinner
func TestQualityResultWarningMascot(t *testing.T) // resultado con ExitCode 1 → la vista usa el sprite Warning (comparar con Sprite(Warning))
func TestResultScroll(t *testing.T)             // resultado de 100 líneas, height 30 → vista sin la línea 99; tras "end" contiene la línea 99 y no la 0
func TestResultWraps(t *testing.T)              // línea de 200 chars, width 80 → ninguna línea de la vista excede 80 de ancho (lipgloss.Width)
func TestStartupShowsFindRootError(t *testing.T) // startupInfo con Err "dubious ownership…" → vista contiene ese texto
```

- [ ] **Step 2: Ejecutar** `go test ./internal/tui/...`. Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Ejecutar.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(tui): add check and tests with live progress, warning mascot and scrolling`

---

### Task 10: Ramas base configurables

**Origen:** se encontró al validar AccountingAPI (2026-10-22). Su rama de integración se llama `desarrollo`. `tija status` la marca `!` y sugiere `git branch -m desarrollo feat/desarrollo` seguido de `git push origin --delete desarrollo`, un consejo destructivo sobre una rama compartida.

**Files:**
- Modify: `internal/branch/branch.go`, `internal/manifest/manifest.go`, `internal/manifest/patch.go`, `internal/engine/status.go`, `internal/engine/init.go`
- Test: `internal/branch/branch_test.go`, `internal/manifest/manifest_test.go`, `internal/engine/status_test.go`, `internal/engine/init_test.go`

**Interfaces:**
- Produces:

```go
var DefaultBases = []string{"main", "master", "dev", "develop", "desarrollo"}
// manifest.Git
BaseBranches []string `yaml:"baseBranches"`
```

  - **Bases en `status`:**
    - con manifest válido, `m.Git.BaseBranches` (si viene vacío, `DefaultBases`);
    - siempre se suman la rama `target` y la rama por defecto del remoto (`DefaultRemoteBranch`).
  - **`FromDetection`:** escribe `BaseBranches: DefaultBases`.
  - **Re-ejecutar `init`:** si un manifest no tiene `baseBranches`, ofrece agregarlo con el mecanismo existente de campos faltantes (`MissingFields` / `FillMissing`), sin cambios de comportamiento.
  - **Ramas publicadas:** el hint de rename de una rama publicada **no** incluye `git push origin --delete`. Solo muestra `then: git push origin -u <nueva>` y la nota `delete the old remote branch only if nobody else uses it`.

- [ ] **Step 1: Tests que fallan**

```go
func TestClassifyDesarrolloIsBase(t *testing.T)         // "desarrollo" con DefaultBases → State base
func TestStatusUsesManifestBaseBranches(t *testing.T)   // manifest baseBranches ["qa"] y rama "qa" → base
func TestStatusRemoteDefaultIsBase(t *testing.T)        // origin/HEAD → "trunk" y rama "trunk" → base
func TestInitOffersBaseBranches(t *testing.T)           // manifest de la Fase 1 sin baseBranches → el plan incluye la actualización de git.baseBranches
func TestRenameHintNeverDeletesRemote(t *testing.T)     // render de status para una rama publicada no contiene "--delete"
```

- [ ] **Step 2: Ejecutar.** Expected: FAIL.
- [ ] **Step 3: Implementar** y regenerar los goldens de status (`-update`).
- [ ] **Step 4: Ejecutar** `go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `fix(status): treat configured and remote default branches as base branches`

---

### Task 11: README y validación de punta a punta

**Files:**
- Modify: `README.md`
- Test: suite completa

- [ ] **Step 1: README.** Agregar una sección **Quality** con:
  - los 5 comandos;
  - la tabla de providers → comando (la de Task 3);
  - la precedencia manifest → detección → scripts;
  - `provider: none` para desactivar una kind;
  - qué se guarda en `.tija/session/quality.json`;
  - exit codes.
- [ ] **Step 2: Suite.** `go vet ./...` y `go test -count=1 ./...`. Expected: todos los paquetes PASS.
- [ ] **Step 3: Validación en este repo.** `go run ./cmd/tija check --json` en la raíz de Tija (sin `.tija/`, stack `unknown`). Expected: JSON válido con `"steps":[]`, una advertencia de `not initialized` y exit 2.
- [ ] **Step 4: Validación en los repos reales.** La hace **el usuario**, en su rama de prueba, porque construye y escribe en `.tija/session/`. Se le entregan estos comandos:
  - **Frontend:**
    - `tija check`: debe correr `biome ci .`;
    - `tija check --json`: JSON con un paso `lint + format`;
    - `tija test`: con el repo sin tests ni script `test`, exit 2 y `! nothing to run …`.
  - **ClaimsAPI:** `tija init`, luego `tija check`.
    - Debe correr `dotnet build ClaimsService.sln --nologo` y después `dotnet format ClaimsService.sln --verify-no-changes`.
    - El check `tests` no aparece.
  - **CollectionsAPI:** igual que ClaimsAPI (`CollectionsService.sln`, sin tests).
  - **AccountingAPI:** `tija init` con target `desarrollo`, luego `tija test`.
    - Debe correr `dotnet test AccountingService.sln --nologo --logger trx …`.
    - Debe reportar los tests pasados y fallidos desde el TRX. Hay unos 313 `[Fact]`/`[Theory]` en 3 proyectos xUnit.
    - `tija status` en `desarrollo` debe mostrar `base branch`.
- [ ] **Step 5: Commit** `docs: document quality commands`

---

## Decisiones tomadas en este plan (revisables)

| Tema | Decisión | Motivo |
|---|---|---|
| `build` en JS | No se agrega solo; únicamente si el manifest lo declara (`quality.build: {provider: script, script: build}`) | El `build` del frontend hace `npm install` del website y es lento. `check` debe ser rápido por defecto |
| Fallback a scripts | Solo para `lint` y `tests` (scripts `lint` y `test`) | Son los únicos con nombre convencional fiable |
| Biome lint+format | Una sola invocación, `biome ci .` | Es lo que ya usa el script `lint` y el CI `pr-checks.yml` del frontend |
| Tests .NET | Se cuentan con el reporte TRX, no leyendo la consola | La salida de `dotnet test` cambia con el idioma de Windows |
| `lint --fix` / `format` | Sin consentimiento, con la lista de archivos cambiados al final | Correr el comando ya es la intención explícita de modificar |
| Sin `.tija/` | `check` funciona, pero no guarda nada | La regla de no crear archivos sin `init` sigue vigente |
| Exit con herramienta no instalada | 2 (entorno), salvo que también haya fallos reales (1) | Coherente con los exit codes del spec |
