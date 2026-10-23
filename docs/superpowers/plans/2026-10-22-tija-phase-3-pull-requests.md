# Tija Fase 3 — Pull Requests: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar `tija pr prepare` y `tija pr create`. Deben producir un título y una descripción que pasen `pr-governance.yml` a la primera, usando el diff real, los commits y las pruebas reales de la sesión de Quality, y abrir el PR con `gh`. De paso, refinar la TUI y la mascota para acercarlas a la imagen de marca.

**Architecture:**
- `internal/prrules` replica localmente lo que valida la Action: título Conventional Commits con scope y secciones obligatorias. Es puro y no depende de git.
- `internal/agents` pasa de "detectar" a "invocar". Cada provider es un CLI (`claude`, `codex`, `gemini`/`agy`) que se ejecuta como subproceso con el prompt por stdin, con timeout. Un parser tolerante extrae título y cuerpo.
- `internal/engine` orquesta en dos funciones:
  - `PreparePR`: contexto git → sesión de Quality (o `RunQuality`) → prompt → agente o borrador determinista → validación → reintento → guarda el borrador.
  - `CreatePR`: borrador → push → `gh pr create` o `gh pr edit`.
- CLI y TUI son capas finas sobre esas dos funciones, igual que en las fases 1 y 2.
- La mascota se rediseña con las reglas de la sección 04 de la imagen de marca y suma el estado `thinking`. La startup screen se acerca al concepto 06.

**Tech Stack:** Go 1.27, Cobra, Bubble Tea v2 / Lip Gloss v2, `gh` CLI y los CLIs de agentes como subprocesos. No se agregan dependencias Go nuevas.

**Spec:** `docs/superpowers/specs/2026-10-22-tija-phases-design.md`: sección 5, Fase 3, más las secciones 2 a 4. Léelo junto con este plan, y también `docs/reference/pull-requests/pr-governance.yml` y `docs/reference/brand/brand-exploration-terminal.png`.

## Global Constraints

- Todo lo de las fases 1 y 2 sigue vigente:
  - módulo `github.com/Ps3udoDev/tija-cli`;
  - UI en inglés, paleta C (`#F97316`, `#111827`, `#FFFFFF`, `#FED7AA`) y símbolos `✓ ✗ ! >`;
  - exit codes `0` ok, `1` fallo de validación, `2` uso o entorno;
  - sin TTY, o con `--json`, `--no-color` o `NO_COLOR`: sin colores ni TUI;
  - Windows es plataforma de primera clase.
- **Contenido generado en `content.language` del manifest** (default `es`). Los mensajes de la UI siguen en inglés.
- **Anti-invención:**
  - `## Pruebas` solo cita resultados de `.tija/session/quality.json` cuyo `Commit == HEAD` y `Dirty == false`, o los de una corrida de `check` hecha en esta misma ejecución.
  - Si no hay ninguno, el texto lo dice. Nunca afirma pruebas.
  - `## Evidencias` es `N/A` salvo que el usuario la edite.
- **Deterministic first:**
  - Lo calcula Tija: rama, target, commits, diffstat, diff, número de issue, resultados de pruebas y validación.
  - El agente solo redacta.
  - El agente recibe todo en el prompt y no necesita herramientas.
- **Sin API keys.** Solo se usan los CLIs instalados. Ninguna invocación usa flags que salten permisos (`--dangerously-skip-permissions`, `--yolo` o similares). Tija solo necesita texto de vuelta.
- **Acciones hacia afuera** (`git push`, `gh pr create`, `gh pr edit`):
  - piden confirmación en TTY, o `--yes`;
  - sin TTY y sin `--yes`: no hacen nada, imprimen el preview y salen con exit 2 y el mensaje `confirmation required: rerun with --yes`.
- **Nunca se bloquea por el nombre de la rama.** Una rama no conforme genera `!` y se usa su `Suggested` para inferir tipo y scope. Sí son error (exit 2) una rama base, HEAD detached, que no haya target y que no haya commits por delante (este último es exit 1).
- Archivos que escribe la fase, todos bajo `.tija/session/` o `.tija/tmp/` y ya ignorados desde `init`:
  - `.tija/session/pr.md` (borrador);
  - `.tija/session/pr.json` (metadatos);
  - `.tija/tmp/pr/` (archivos temporales de agentes y `--body-file`).
  - En un repo sin `.tija/manifest.yaml` se usan los defaults en memoria, no se escribe nada en el repo (temporales en `os.TempDir`) y se avisa `! not initialized — draft is not saved (run: tija init)`.
- **Menú de la TUI:** `Pull Request`, `Check`, `Tests`, `Doctor`, `Status`, `Init` (con `Init` primero si el repo no está inicializado). No aparece nada de fases futuras (Review, Context).

## Review Focus

1. **El agente responde con otro formato:** preámbulo, cuerpo dentro de ```` ```markdown ````, título con comillas o en inglés, secciones faltantes.
   - El parser tolera fences y texto fuera de los marcadores.
   - La validación detecta lo que falte.
   - Hay un reintento con los errores. Si el segundo intento también es inválido, el borrador se guarda igual y el exit es 1.
   - Lo cubren Task 4 (`TestParseDraftTolerant`) y Task 6 (`TestPreparePRRetriesThenReports`).
2. **CLI de agente colgado, inexistente o shim `.cmd` en Windows:**
   - stdin se cierra después del prompt;
   - el timeout (default 3 min, `--agent-timeout`) mata el proceso;
   - cualquier fallo del agente cae al borrador determinista con `! agent <name> failed: <motivo> — using deterministic draft`.
   - Lo cubren Task 4 (`TestGenerateTimeout`) y Task 6 (`TestPreparePRAgentFailureFallsBack`).
3. **Diff enorme o con lockfiles y binarios:**
   - el prompt nunca supera `maxDiffBytes = 60_000` bytes de diff;
   - lockfiles, `bin/`, `obj/`, `dist/`, `*.min.*` y binarios aparecen solo en el diffstat;
   - el prompt dice explícitamente qué archivos se truncaron.
   - Lo cubre Task 5 (`TestCollectPRContextTruncatesDiff`).
4. **Sesión de Quality vieja** (otro commit, o el árbol sucio cuando se corrió):
   - `## Pruebas` no la cita;
   - sin `--no-check` se corre `check`;
   - con `--no-check` el texto dice que no hay pruebas verificadas para este commit.
   - Lo cubren Task 5 (`TestDeterministicDraftIgnoresStaleSession`) y Task 6 (`TestPreparePRRunsCheckWhenStale`).
5. **Rama ya con PR abierto, sin publicar, o target solo en `origin`:**
   - `create` hace `git push -u` si hace falta;
   - si existe un PR abierto para la rama, ofrece actualizarlo (`gh pr edit`) en vez de fallar;
   - el target se resuelve primero como `origin/<target>` y luego como local.
   - Lo cubren Task 2 (`TestResolveBasePrefersRemote`) y Task 7 (`TestCreatePRUpdatesExisting`, `TestCreatePRPushesUnpublished`).

Además: un cuerpo o template con CRLF (Windows) se normaliza a LF antes de validar y de escribir. Lo cubre Task 3 (`TestValidateBodyCRLF`).

---

## Estructura de archivos

```text
internal/detect/testdata/dotnet-api/      → nombres genéricos (Task 1)
internal/sys/env.go                       → + RunInput (stdin)
internal/gitx/pr.go                       → ResolveBase, Commits, DiffStat, DiffFile (solo lectura)
internal/prrules/prrules.go               → ValidateTitle, ValidateBody, Problem, Normalize
internal/agents/provider.go               → Provider, Request, Argv, Generate
internal/agents/parse.go                  → ParseDraft
internal/engine/pr_context.go             → PRContext, CollectPRContext
internal/engine/pr_prompt.go              → BuildPRPrompt
internal/engine/pr_draft.go               → DeterministicDraft, SaveDraft, LoadDraft
internal/engine/pr_prepare.go             → PreparePR, PRReport, PREvent
internal/engine/pr_create.go              → CreatePR, CreateReport
internal/render/pr.go                     → PRPrepare, PRCreate (plain/styled/json)
internal/cli/pr.go                        → tija pr prepare | create
internal/tui/mascot.go                    → sprites nuevos + Thinking + captions
internal/tui/startup.go, menu.go, brand.go→ startup estilo concepto 06
internal/tui/pr_view.go                   → flujo Pull Request en la TUI
README.md
```

---

### Task 1: Fixture .NET con nombres genéricos

**Por qué:** el fixture `dotnet-api` copia a propósito la forma del repo de referencia GMX-ClaimsAPI: una `.sln` en la raíz, proyectos en `src/` y `tests/`, y un `.csproj` suelto en `scratch/` fuera de la solución. Eso es lo que el detector tiene que manejar. Los nombres "Claims" no aportan nada a la prueba y hacen creer que Tija conoce ese repo.

**Files:**
- Rename: `internal/detect/testdata/dotnet-api/ClaimsService.sln` → `SampleService.sln`
- Rename: `src/Claims.API/Claims.API.csproj` → `src/Sample.Api/Sample.Api.csproj`
- Rename: `tests/Claims.Tests/{Claims.Tests.csproj,ClaimTests.cs}` → `tests/Sample.Tests/{Sample.Tests.csproj,SampleTests.cs}` (clase `SampleTests`, mismos 3 métodos y los mismos atributos)
- Rename: `scratch/inspect_roles.csproj` → `scratch/Scratch.csproj`
- Modify: `internal/detect/detect_test.go`, `internal/quality/adapters_test.go`, `internal/render/golden_test.go` (+ goldens), `internal/cli/commands_test.go` (`name: Claims` → `name: Sample`)

- [ ] **Step 1:** Hacer los renames con `git mv` y actualizar las rutas dentro de la `.sln`. Borrar las carpetas `obj/` locales del fixture (no están versionadas).
- [ ] **Step 2:** Reemplazar en los tests `ClaimsService.sln` → `SampleService.sln` y `src/Claims.API/Claims.API.csproj`, `tests/Claims.Tests/Claims.Tests.csproj` → las rutas nuevas. `TestCount` no cambia.
- [ ] **Step 3:** `go test ./internal/render -update` y revisar con `git diff internal/render/testdata` que solo cambiaron los nombres.
- [ ] **Step 4:** `go test ./...` → PASS. `grep -ri claim internal/` → sin resultados.
- [ ] **Step 5: Commit** `test(detect): use generic names in the .NET fixture`

---

### Task 2: `sys.RunInput` y contexto git para PRs

**Files:**
- Modify: `internal/sys/env.go`, `internal/sys/env_test.go`
- Create: `internal/gitx/pr.go`, `internal/gitx/pr_test.go`

**Interfaces:**
- Produces:
  - `type InputRunner func(ctx context.Context, dir, stdin, name string, args ...string) (stdout, stderr string, exitCode int, err error)`, con el mismo contrato que `Runner`. Va en `Env.RunInput` y lo llena `Default()`. stdin se escribe y se cierra; un ctx cancelado mata el proceso (`WaitDelay` de 2 s, como `execStream`).
  - `func (r Repo) ResolveBase(ctx, target string) (ref string, ok bool)`: devuelve `refs/remotes/origin/<target>` si existe; si no, `refs/heads/<target>`.
  - `type Commit struct { SHA, Subject, Body string }`
  - `func (r Repo) Commits(ctx, base string) []Commit`: `git log --reverse --format=%H%x1f%s%x1f%b%x1e <base>..HEAD`, del más viejo al más nuevo.
  - `type FileStat struct { Path string; Added, Deleted int; Binary bool }`
  - `func (r Repo) DiffStat(ctx, base string) []FileStat`: `git diff --numstat -z <base>...HEAD`. `-\t-` significa binario; en renames va la ruta destino.
  - `func (r Repo) DiffFile(ctx, base, path string) string`: `git diff --no-color --no-ext-diff <base>...HEAD -- <path>`.

- [ ] **Step 1: Tests que fallan**
  - `TestRunInputPassesStdin`: corre `git hash-object --stdin` con stdin `"hola\n"` y espera el SHA conocido `"…"` (calcularlo con `git hash-object` al escribir el test).
  - `TestRunInputCancelled`: con un ctx ya cancelado, `RunInput(ctx, dir, "x", "git", "hash-object", "--stdin")` devuelve `err != nil` o un exit distinto de 0, en menos de 5 s.
  - `TestResolveBasePrefersRemote`: con `testutil.WithRemote` y `dev` creado y pusheado, y además una `dev` local con un commit extra. Espera `refs/remotes/origin/dev`.
  - `TestResolveBaseLocalOnly`: espera `refs/heads/dev`.
  - `TestResolveBaseMissing`: espera `ok == false`.
  - `TestCommitsAndDiffStat`: en una rama con 2 commits (uno que agrega `a.txt` con 3 líneas y otro que agrega `img.png` con bytes no UTF-8). Espera los subjects en orden, `FileStat{Path:"a.txt",Added:3}` y `img.png` con `Binary:true`.
- [ ] **Step 2:** `go test ./internal/sys ./internal/gitx` → FAIL (no compila).
- [ ] **Step 3:** Implementar. Separar `Commits` por `\x1e` y `\x1f`. Normalizar `\r\n`.
- [ ] **Step 4:** `go test ./internal/sys ./internal/gitx` → PASS.
- [ ] **Step 5: Commit** `feat(gitx): read commits, diff stats and base refs for pull requests`

---

### Task 3: `internal/prrules` — la validación de `pr-governance.yml`, en local

**Files:**
- Create: `internal/prrules/prrules.go`, `internal/prrules/prrules_test.go`

**Interfaces:**
- Consumes: `manifest.PullRequest` (Fase 1).
- Produces:
  - `type Problem struct { Field string; Message string }`, con `Field` `"title"` o `"section:<Nombre>"`. `Message` va en inglés.
  - `func Normalize(s string) string`: CRLF a LF, y `TrimSpace` del total más `"\n"` final.
  - `func ValidateTitle(title string, r manifest.PullRequest) []Problem`
  - `func ValidateBody(body string, r manifest.PullRequest) []Problem`
  - `func SectionContent(body, title string) (content string, found bool)`
  - `var Types = []string{"feat","fix","docs","style","refactor","perf","test","build","ci","chore","revert"}`

**Reglas exactas:**
- **Título** (default de `amannn/action-semantic-pull-request@v6`, más la guía):
  - patrón `^(\w+)(?:\(([\w$.\-*/ ]+)\))?!?: (.+)$`;
  - el tipo está en `Types` y en minúsculas;
  - scope obligatorio si `Title.RequireScope` es nil o true;
  - longitud ≤ `Title.MaxLength` (si es > 0);
  - no termina en `.`.
- **Sección:** se porta 1:1 `sectionContent` del workflow:
  - regex `(?i)(?:^|\n)##\s+<titulo escapado>\s*\n([\s\S]*?)(?:\n##\s+|$)` sobre el cuerpo normalizado;
  - se quitan los comentarios `<!--…-->`;
  - se quitan los prefijos de checkbox `^\s*[-*]\s+\[[ xX]\]\s*` (multiline);
  - se quitan los bullets `^\s*[-*]\s*` (multiline);
  - `TrimSpace`.
  - Go RE2 no tiene lookahead: capturar hasta el siguiente `\n##` con un índice manual en vez de `(?=…)`.
- **Cuerpo:**
  - si quitando los comentarios queda vacío → problem `body`;
  - por cada `RequiredSections`: falta → `missing section ## X`;
  - si `utf8.RuneCountInString(content) < MinSectionLength` o el contenido entero coincide (case-insensitive) con un valor de `InvalidValues` → `section ## X needs useful content`.

- [ ] **Step 1: Tests que fallan** (tabla), con `manifest.FromDetection` como reglas:
  - `TestValidateTitle`:
    - válidos: `feat(claims): add catastrophe workflow`, `fix(ui-kit): corregir modal`;
    - inválidos: `Feat(x): a`, `feat: sin scope`, `feature(x): a`, `fix(x): termina con punto.`, un título de 101 caracteres, `fix(x):sin espacio`.
  - `TestValidateBodyTemplateFails`: el template default sin llenar (`internal/assets/defaults/templates/pull-request.md`) da exactamente 4 problems, uno por sección requerida.
  - `TestValidateBodyInvalidValues`: `## Pruebas\n\n- N/A` → problem; `## Pruebas\n\n- pendiente` → problem; `## Pruebas\n\n- [x] Se ejecutó dotnet test` → OK.
  - `TestValidateBodyCRLF`: un cuerpo válido con `\r\n` → sin problems.
  - `TestValidateBodyCaseInsensitiveHeading`: `## resumen` cuenta como `Resumen`.
  - `TestValidateBodyIgnoresEvidencias`: `## Evidencias\n\nN/A` no genera problem.
- [ ] **Step 2:** `go test ./internal/prrules` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `go test ./internal/prrules` → PASS.
- [ ] **Step 5: Commit** `feat(prrules): validate pull request titles and bodies like pr-governance`

---

### Task 4: `internal/agents` — invocar CLIs de agentes y parsear su salida

**Files:**
- Create: `internal/agents/provider.go`, `internal/agents/parse.go`, `internal/agents/provider_test.go`, `internal/agents/parse_test.go`
- Modify: `internal/agents/detect.go` (comentario del paquete: ya no "never invokes")

**Interfaces:**
- Consumes: `sys.Env.RunInput`, `agents.Binary` (Fase 1).
- Produces:
  - `type Request struct { Prompt, Tier, Dir, TmpDir string; Timeout time.Duration }`
  - `func Argv(provider, bin, tier, tmpDir string) []string`: los argumentos sin el binario.
  - `func Generate(ctx context.Context, env sys.Env, provider string, req Request) (string, error)`:
    - error `ErrNotInstalled` si `Binary` devuelve `""`;
    - `ErrTimeout` si vence el timeout;
    - `fmt.Errorf("%s exited %d: <últimas 5 líneas de stderr>")` si el exit es distinto de 0;
    - error si la salida está vacía.
  - `type Draft struct { Title, Body string }`
  - `func ParseDraft(out string) (Draft, error)`

**Invocaciones** (el prompt siempre va por stdin):

| provider | argv | tier |
|---|---|---|
| `claude` | `-p --output-format text --model <m>` | `fast→haiku`, `balanced→sonnet`, `deep→opus` |
| `codex` | `exec --sandbox read-only --skip-git-repo-check --color never -o <tmpDir>/codex-last.txt -` | se ignora. La salida es el contenido de `codex-last.txt`; si no existe, stdout |
| `gemini` vía `gemini` | `-p "Follow the instructions given on standard input."` | se ignora |
| `gemini` vía `agy` | igual que `gemini` | se ignora |

Antes de cerrar la tarea, el implementador corre `claude --help`, `codex exec --help`, `gemini --help` y `agy --help` en su máquina. Si algún flag de la tabla no existe en la versión instalada, ajusta `Argv` y esta tabla en el mismo commit y lo anota en el mensaje.

**Formato de salida que se le pide al agente y que acepta `ParseDraft`:**
```text
<<<TITLE
feat(scope): descripción
TITLE>>>
<<<BODY
## Resumen
...
BODY>>>
```
El parser:
- quita secuencias ANSI y fences ```` ``` ```` alrededor de cada bloque;
- ignora el texto fuera de los marcadores;
- quita comillas o backticks que envuelvan el título;
- si no hay marcadores pero hay una línea que cumple el patrón de título seguida de `## `, usa esa línea como título y el resto como cuerpo;
- si no encuentra nada → `ErrUnparseable`.

- [ ] **Step 1: Tests que fallan**
  - `TestArgv`: la tabla de arriba, incluido `claude` con tier `""` → sin `--model`.
  - `TestGenerateUsesStdin`: un `RunInput` falso registra stdin, name y args, y devuelve `"<<<TITLE…"`. Espera que stdin sea igual al prompt y el binario resuelto por un `LookPath` falso.
  - `TestGenerateNotInstalled` → `errors.Is(err, ErrNotInstalled)`.
  - `TestGenerateTimeout`: un `RunInput` falso bloquea hasta `ctx.Done()`. Espera `ErrTimeout` en < 1 s con `Timeout: 50ms`.
  - `TestGenerateCodexReadsLastMessage`: el `RunInput` falso escribe `codex-last.txt` en `TmpDir`. Espera ese contenido.
  - `TestParseDraftTolerant`, casos: los marcadores exactos; un preámbulo `"Claro, aquí está:"`; el bloque BODY dentro de ```` ```markdown ````; un título entre comillas; ANSI `\x1b[1m`; sin marcadores pero con título + `## Resumen`. Más uno basura → `ErrUnparseable`.
- [ ] **Step 2:** `go test ./internal/agents` → FAIL.
- [ ] **Step 3:** Implementar.
  - El timeout se aplica con `context.WithTimeout`.
  - `ErrTimeout` se detecta por `ctx.Err() == context.DeadlineExceeded`.
- [ ] **Step 4:** `go test ./internal/agents` → PASS. Verificar la tabla con `--help` (ver arriba).
- [ ] **Step 5: Commit** `feat(agents): invoke agent CLIs with stdin prompts and parse drafts`

---

### Task 5: Contexto del PR, prompt y borrador determinista

**Files:**
- Create: `internal/engine/pr_context.go`, `internal/engine/pr_prompt.go`, `internal/engine/pr_draft.go`, `internal/engine/pr_test.go`

**Interfaces:**
- Consumes: `gitx` (Task 2), `quality.LoadSession` y `quality.StepResult` (Fase 2), `branch.Verdict`, `manifest.Manifest`, `assets.Files()`.
- Produces:
  ```go
  type PRContext struct {
      Root, Branch, Target, BaseRef, Head string
      Verdict      branch.Verdict
      IssueNumber  int
      Commits      []gitx.Commit
      Files        []gitx.FileStat
      Diff         string   // ya truncado
      Truncated    []string // rutas omitidas o recortadas
      Tests        []quality.StepResult // solo frescos: Commit==Head && !Dirty
      Rules, Template string
      Manifest     manifest.Manifest // el del repo o FromDetection en memoria
      Initialized  bool
      Dirty        bool
  }
  func CollectPRContext(ctx context.Context, root string, env sys.Env) (PRContext, error)
  func FreshTests(s quality.Session, head string) []quality.StepResult
  func BuildPRPrompt(c PRContext, problems []prrules.Problem) string
  func DeterministicDraft(c PRContext) agents.Draft
  type DraftMeta struct { Head, Branch, Target, Source string; Problems []prrules.Problem }
  func SaveDraft(root string, d agents.Draft, meta DraftMeta) error   // .tija/session/pr.md + pr.json
  func LoadDraft(root string) (agents.Draft, DraftMeta, error)        // fs.ErrNotExist si no hay
  ```
- Errores de `CollectPRContext` (sentinels en `engine`):
  - `ErrBaseBranch`: la rama es base o es el target;
  - `ErrDetached`;
  - `ErrTargetNotFound`;
  - `ErrNoCommits`: 0 commits en `base..HEAD`.
  - Los tres primeros son exit 2. `ErrNoCommits` es exit 1.

**Decisiones:**
- **Diff:**
  - se recorre `Files` en orden;
  - se excluyen del diff (y se listan en `Truncated`): los lockfiles (`package-lock.json`, `bun.lock`, `bun.lockb`, `yarn.lock`, `pnpm-lock.yaml`, `packages.lock.json`), las rutas con segmento `bin/`, `obj/`, `dist/`, `node_modules/` o `.tija/`, los `*.min.*` y los `Binary`;
  - se agrega `DiffFile` por archivo mientras el total sea ≤ `maxDiffBytes = 60_000`. Un archivo que no entra se lista en `Truncated`.
- **Rules/Template:** se lee la ruta del manifest. Si el archivo no existe, se usa el embebido de `assets.Files()`.
- **Prompt** (en inglés para el agente, con la instrucción de escribir el contenido en `content.language`). Secciones en este orden:
  1. el rol;
  2. las reglas del repo (contenido de `Rules`);
  3. el template;
  4. la convención del título (tipos permitidos, scope obligatorio, `maxLength`, sin punto final);
  5. los datos de la rama (nombre, target, issue → `Closes #N` en `## Referencias`);
  6. los commits (subject + body);
  7. el diffstat;
  8. el diff y la lista `Truncated`;
  9. los **resultados verificados** de pruebas (comando, estado, conteos). Si no hay: "No automated checks were verified for this commit. Do not claim any test was run.";
  10. las prohibiciones de la guía: no inventar pruebas, endpoints ni funcionalidades; nombres técnicos tal cual; resumir por funcionalidad; `Evidencias: N/A`;
  11. el formato de salida con marcadores;
  12. si `problems` no está vacío: "Your previous answer failed these checks:" y la lista.
- **Borrador determinista** (siempre en español; no hay otro idioma determinista):
  - **tipo:** el prefijo de la rama (o de `Verdict.Suggested` si no es conforme), mapeado `feature→feat`, `bugfix|hotfix→fix`, `release→chore`;
  - **scope:** el primer segmento de la descripción de la rama sin el número de issue (`fix/133-password-leak` → `password`). Si no hay, el directorio de primer nivel más tocado en `Files`, en minúsculas y con `.` y `_` cambiados por `-`;
  - **descripción:** la descripción de la rama con `-` cambiado por espacios;
  - **cuerpo:** el template con cada sección requerida llenada así:
    - Resumen: `Cambios de la rama \`<branch>\` hacia \`<target>\`: <n> commit(s) y <m> archivo(s) modificados.`
    - Objetivo: `<descripción con mayúscula inicial>.` y, si hay issue, ` Relacionado con el issue #N.`
    - Cambios realizados: un bullet por commit, agrupados por tipo de Conventional Commit (sin el prefijo `type(scope): `).
    - Pruebas: un bullet por resultado fresco, por ejemplo `- \`dotnet test SampleService.sln\`: 74 superadas, 0 fallidas.` o `- \`biome ci .\`: sin errores.`. Si no hay ninguno: `- No se ejecutaron verificaciones automáticas para este commit.`
    - Evidencias: `N/A`.
    - Referencias: `Closes #N` o `Rama \`<branch>\`.`
  - Se quitan los comentarios HTML del template.

- [ ] **Step 1: Tests que fallan** (repos reales con `testutil`):
  - `TestCollectPRContext`: fixture `react-vite`, `WithRemote`, rama `fix/133-password-leak` con 2 commits. Espera `IssueNumber==133`, `BaseRef=="refs/remotes/origin/main"`, 2 commits, `Files` no vacío y `Rules` igual al default embebido si el repo no está inicializado.
  - `TestCollectPRContextErrors`: estar en `main` → `ErrBaseBranch`; detached → `ErrDetached`; `pullRequest.target: nope` → `ErrTargetNotFound`; una rama sin commits nuevos → `ErrNoCommits`.
  - `TestCollectPRContextTruncatesDiff`: una rama que agrega `package-lock.json` (100 KB) y `big.txt` (70 KB) más `small.go`. Espera que `Diff` contenga `small.go`, que no contenga `package-lock.json` y que `len(Diff) <= 60000`, con `Truncated` que contenga ambas rutas grandes.
  - `TestFreshTests`: resultados con `Commit==head && !Dirty`, `Commit!=head` y `Dirty==true`. Solo el primero sobrevive.
  - `TestBuildPRPromptContainsFacts`: contiene `"Closes #133"`, el subject de cada commit, `"Do not claim any test was run"` cuando `Tests` está vacío, y la lista de problems cuando se pasan.
  - `TestDeterministicDraftValid`: para las ramas `fix/133-password-leak`, `feature/shared-imputation-form` y `101-fixclaims-fix-integration-issues`, el resultado pasa `prrules.ValidateTitle` y `ValidateBody` sin problems y el título empieza con `fix(password):`, `feat(shared):` y `fix(` respectivamente.
  - `TestDeterministicDraftIgnoresStaleSession`: con una sesión de otro commit, `## Pruebas` contiene `No se ejecutaron verificaciones automáticas`.
  - `TestSaveLoadDraftRoundTrip`, con un cuerpo que tiene `\r\n`: se guarda normalizado.
- [ ] **Step 2:** `go test ./internal/engine -run 'PR|Fresh|Deterministic|Draft'` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** Mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(engine): collect pull request context, build prompts and deterministic drafts`

---

### Task 6: `engine.PreparePR`

**Files:**
- Create: `internal/engine/pr_prepare.go`, `internal/engine/pr_prepare_test.go`

**Interfaces:**
- Consumes: Tasks 3-5, `engine.RunQuality` (Fase 2), `agents.Generate`.
- Produces:
  ```go
  type PRRequest struct {
      Agent        string        // "" = manifest agents.default; "none" = determinista
      NoCheck      bool
      CheckTimeout time.Duration // default 10m
      AgentTimeout time.Duration // default 3m
  }
  type PRStage string // "context", "check", "agent", "validate", "done"
  type PREvent struct { Stage PRStage; Message string; Quality *QualityEvent }
  type PRReport struct {
      Root, Branch, Target string
      Verdict   branch.Verdict
      Title     string
      Body      string
      Source    string            // "claude" | "codex" | "gemini" | "deterministic"
      Attempts  int
      Problems  []prrules.Problem
      Commits   int
      Files     int
      Tests     []quality.StepResult
      Warnings  []string
      DraftPath string            // "" si no se guardó
  }
  func (r PRReport) ExitCode() int // 1 si len(Problems) > 0, si no 0
  func PreparePR(ctx context.Context, dir string, env sys.Env, req PRRequest, onEvent func(PREvent)) (PRReport, error)
  func PRPrompt(ctx context.Context, dir string, env sys.Env) (string, error) // para --print-prompt; nunca corre check ni agente
  ```

**Algoritmo:**
1. `CollectPRContext`. Si el `Verdict` es `Nonconforming`, agrega el warning `branch <name> does not follow the convention (suggested: <s>)`. Si `Dirty`, agrega `uncommitted changes are not part of the pull request`.
2. Si `Tests` está vacío y no hay `NoCheck`: `RunQuality` con `CheckMode` y todas las kinds, reenviando los eventos como `PREvent{Stage:"check"}`. Después se toman los resultados frescos del reporte, solo si el árbol no estaba sucio. Un check que falla no detiene el flujo: va a `Pruebas` tal cual, por ejemplo `- \`biome ci .\`: falló (exit 1).`.
3. Se elige el agente: `req.Agent`, o si no `Manifest.Agents.Default`. Si queda `""`, `"none"` o el binario no está instalado → determinista, con el warning `no agent available — using deterministic draft` (salvo que haya sido `"none"` explícito).
4. Con agente, hasta 2 intentos: `Generate` (tier `Manifest.Agents.Tasks["pullRequest"].Tier`) → `ParseDraft` → `Normalize` → validar. Si es válido, se termina. Si no, se reintenta con `problems`. Si `Generate` o `ParseDraft` fallan, se cae al determinista con el warning `agent <name> failed: <err> — using deterministic draft`.
5. El resultado final (válido o no) se guarda con `SaveDraft` si el repo está inicializado.

- [ ] **Step 1: Tests que fallan** (con `env.RunInput` y `LookPath` falsos para el agente, y git real con `testutil`):
  - `TestPreparePRWithAgent`: el agente devuelve un borrador válido. Espera `Source=="claude"`, `Attempts==1`, sin problems, y `.tija/session/pr.md` escrito.
  - `TestPreparePRRetriesThenReports`: el agente devuelve dos veces un borrador sin `## Pruebas`. Espera `Attempts==2`, que el prompt del segundo intento contenga `missing section ## Pruebas`, `ExitCode()==1` y el borrador guardado igual.
  - `TestPreparePRRetrySucceeds`: inválido y luego válido → `Attempts==2`, exit 0.
  - `TestPreparePRAgentFailureFallsBack`: `RunInput` devuelve exit 1 → `Source=="deterministic"`, un warning con `agent claude failed`, exit 0.
  - `TestPreparePRNoAgent`: `Agent:"none"` → determinista y nunca se llama a `RunInput`.
  - `TestPreparePRRunsCheckWhenStale`: repo con manifest `quality: {tests: {provider: script, script: test}}` y un `package.json` cuyo script `test` es `node -e ""` (saltar si no hay `node`). Espera un evento `check` y `Tests` con un resultado `passed` de HEAD.
  - `TestPreparePRNoCheckSkips`: con `NoCheck` no hay eventos `check` y `Pruebas` dice que no se ejecutaron verificaciones.
  - `TestPreparePRUninitialized`: no crea `.tija/`, `DraftPath==""` y aparece el warning `not initialized — draft is not saved (run: tija init)`.
- [ ] **Step 2:** `go test ./internal/engine -run PreparePR` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `go test ./internal/engine` → PASS.
- [ ] **Step 5: Commit** `feat(engine): prepare pull request drafts with agent retries and deterministic fallback`

---

### Task 7: `engine.CreatePR`

**Files:**
- Create: `internal/engine/pr_create.go`, `internal/engine/pr_create_test.go`

**Interfaces:**
- Consumes: `LoadDraft`, `PreparePR`, `gitx`, `env.Run`.
- Produces:
  ```go
  type CreateRequest struct {
      Prepare PRRequest
      Draft   bool // gh --draft
  }
  type CreatePlan struct {           // lo que se va a hacer, para pedir consentimiento
      Report   PRReport
      Push     bool   // la rama no está publicada o tiene commits sin pushear
      Existing string // URL de un PR abierto para la rama, "" si no hay
      Assign   bool
  }
  type CreateReport struct { URL string; Pushed, Updated, Assigned bool }
  func PlanCreatePR(ctx context.Context, dir string, env sys.Env, req CreateRequest, onEvent func(PREvent)) (CreatePlan, error)
  func ApplyCreatePR(ctx context.Context, dir string, env sys.Env, p CreatePlan, req CreateRequest) (CreateReport, error)
  ```

**Decisiones:**
- **Reusar el borrador:** `PlanCreatePR` usa el que dejó `prepare` si `meta.Head == HEAD`, `meta.Branch == rama actual` y `meta.Target == target` (así el usuario puede editar `pr.md` a mano entre `prepare` y `create`). Lo revalida con `prrules`. Si no se cumple, llama a `PreparePR`.
- **Problems:** si el borrador final tiene problems, `PlanCreatePR` devuelve el plan y el CLI sale con 1 **sin** push ni creación.
- **Requisitos de `gh`:** `gh` en PATH y `gh auth status` con exit 0. Si no → error `ErrGHUnavailable` (exit 2) con el hint `install GitHub CLI and run: gh auth login`.
- **PR existente:** `gh pr list --head <branch> --state open --json url --limit 1`.
- **Push:** `git push -u origin <branch>` si `!IsPublished` o si `rev-list origin/<branch>..HEAD` es > 0. Nunca `--force`.
- **Crear:** se escribe el cuerpo en `.tija/tmp/pr/body.md` (o en `os.TempDir` si no está inicializado) y se corre `gh pr create --base <target> --head <branch> --title <t> --body-file <f>`, más `--assignee @me` si `autoAssign` y `--draft` si se pidió. La URL es la última línea de stdout que empieza con `https://`.
- **Actualizar:** `gh pr edit <url> --title <t> --body-file <f>`, más `--add-assignee @me` si `autoAssign`.
- **Errores:** cualquier fallo de `git push` o `gh` devuelve un error con su stderr (exit 2).

- [ ] **Step 1: Tests que fallan** (`env.Run` envuelto: los comandos `gh` los responde un fake que registra los argv; `git` va al runner real):
  - `TestCreatePRPushesUnpublished`: `WithRemote`, una rama nueva con un commit, el fake de `gh` sin PR existente y `create` que imprime `https://github.com/o/r/pull/7`. Espera `Pushed`, `URL==…/7`, la rama publicada en el bare y argv con `--base main --head <rama> --assignee @me`.
  - `TestCreatePRUpdatesExisting`: `gh pr list` devuelve `[{"url":"https://github.com/o/r/pull/3"}]` → `Existing` lleno. Después de aplicar: `Updated`, se llamó `gh pr edit https://…/3` y nunca `gh pr create`.
  - `TestCreatePRReusesFreshDraft`: un `pr.md` editado a mano con el mismo HEAD se usa tal cual (no se llama al agente).
  - `TestCreatePRStaleDraftRegenerates`: un `pr.json` con otro HEAD → se llama a `PreparePR`.
  - `TestCreatePRGhMissing` → `errors.Is(err, ErrGHUnavailable)`.
  - `TestCreatePRInvalidDraftDoesNotPush`: un borrador con problems → `ApplyCreatePR` no se llama desde el flujo del CLI. Aquí se prueba que `PlanCreatePR` devuelve `Report.ExitCode()==1`.
- [ ] **Step 2:** `go test ./internal/engine -run CreatePR` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `go test ./internal/engine` → PASS.
- [ ] **Step 5: Commit** `feat(engine): push branches and create or update pull requests with gh`

---

### Task 8: Render y CLI `tija pr prepare | create`

**Files:**
- Create: `internal/render/pr.go`, `internal/cli/pr.go`
- Modify: `internal/render/golden_test.go` (+ goldens), `internal/cli/root.go`, `internal/cli/commands_test.go`, `README.md`

**Interfaces:**
- Consumes: Tasks 6-7, `render.Printer`.
- Produces:
  - `func render.PRPrepare(p Printer, r engine.PRReport)`
  - `func render.PRCreate(p Printer, plan engine.CreatePlan, r engine.CreateReport)`
  - `func newPRCmd(deps Deps, in io.Reader) *cobra.Command`, con los subcomandos `prepare` y `create`.

**Salida plain de `PRPrepare`** (golden):
```text
Pull request  fix/133-password-leak → main
✓ 2 commits, 5 files
✓ tests: dotnet test SampleService.sln — 74 passed, 0 failed
! branch changes does not follow the convention (suggested: feat/short-description)
✓ drafted by claude (1 attempt)

fix(password): evitar fuga de contraseña en logs

## Resumen
...

✓ title and sections pass pr-governance rules
Draft: .tija/session/pr.md
```
Con problems, la penúltima línea es un `✗ <message>` por cada problem. `--json` emite `PRReport` (y en `create`, `{"plan":…, "result":…}`).

**Flags:**
- `prepare`: `--agent <claude|codex|gemini|none>`, `--no-agent` (= `none`), `--no-check`, `--print-prompt`, `--agent-timeout 3m`, `--check-timeout 10m`.
- `create`: los mismos, más `--draft` y `--edit`:
  - `--edit` abre el borrador en el editor de `git var GIT_EDITOR` antes de confirmar y luego lo revalida. Necesita TTY y repo inicializado; si no, exit 2.
  - Confirmación en TTY: `Push <branch> and open a pull request to <target>? [y/N]`, o `Update pull request <url>? [y/N]`. Se reutiliza el prompt y/n de `internal/cli/prompt.go`.

**Mapeo de errores:** `ErrBaseBranch`, `ErrDetached`, `ErrTargetNotFound`, `ErrGHUnavailable`, `ErrNotARepo` → exit 2. `ErrNoCommits` y los problems → exit 1.

- [ ] **Step 1: Tests que fallan**
  - Goldens: `TestRenderPRPreparePlain`, `TestRenderPRPrepareProblemsPlain`, `TestRenderPRCreatePlain` (PR creado y PR actualizado).
  - CLI:
    - `TestPRPrepareNoAgentJSON`: `tija pr prepare --no-agent --no-check --json` en una rama con commits → exit 0 y JSON con `"source":"deterministic"`.
    - `TestPRPreparePrintPrompt`: imprime un texto que contiene `<<<TITLE`, no corre agente ni check y sale con 0.
    - `TestPRPrepareOnBaseBranch` → exit 2.
    - `TestPRCreateNeedsYesWithoutTTY`: sin TTY y sin `--yes` → exit 2, stdout con el preview, y el fake de `gh` sin `create`.
    - `TestPRCreateYes`: con el fake de `gh` → exit 0 y la URL impresa.
- [ ] **Step 2:** `go test ./internal/render ./internal/cli` → FAIL.
- [ ] **Step 3:** Implementar y registrar `newPRCmd` en `root.AddCommand`. README: sección "Pull requests" con los dos comandos, los flags, la tabla de agentes y la regla de anti-invención.
- [ ] **Step 4:** `go test ./internal/render -update`, revisar el diff de los goldens y `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(cli): add pr prepare and pr create commands`

---

### Task 9: Mascota refinada según la imagen de marca

**Diagnóstico (captura actual frente a la marca):**
- **Cabeza y cuerpo:** la cabeza es un rectángulo naranja sin hocico. La marca tiene **hocico y mejillas blancas** en el tercio inferior de la cara, nariz ink y orejas con interior sand de 2 px.
- **Pecho:** es un bloque blanco cuadrado que parece un delantal. En la marca el gato está **sentado de tres cuartos**, con pecho blanco en forma de babero y patas delanteras blancas separadas.
- **Cola:** sale en ángulo recto desde la base. En la marca sube curvada por el lado derecho.
- **Ojos:** son columnas ink de 2 px muy separadas que dan cara de "sorpresa". En la marca son de 1×2 y están más cerca del centro.
- **Accesorios:** los de los estados (lupa, chispas, `!`, `×`) se dibujan pegados a la cabeza y se confunden con ella. En la marca están **separados del gato** y en otro color.
- **Contexto de estado:** no existe. La marca acompaña cada estado con una **leyenda** ("Tija is ready.", "Scanning repository...", "Thinking...", "All tasks completed!", "Needs your review.", "Check failed.").

**Files:**
- Modify: `internal/tui/mascot.go`, `internal/tui/tui_test.go`, `internal/tui/model.go` (`panel`)

**Interfaces:**
- Produces:
  - `const Thinking MascotState = "thinking"`
  - `func Caption(s MascotState) string`
  - `Sprite(s)` mantiene su firma.

**Reglas del sprite nuevo:**
- **Grilla:** 20 columnas × 18 filas de píxeles (20×9 celdas). Columnas 0-13: el gato. Columnas 15-19: solo el accesorio. La columna 14 siempre queda vacía.
- **Gato base, compartido por todos los estados** (los estados solo cambian ojos, boca y orejas):
  - orejas triangulares de 3 px con 1 px `S` adentro;
  - cabeza de 11 px de ancho;
  - fila de ojos `K` de 1×2;
  - hocico `W` de 5-7 px con nariz `K` de 1 px;
  - cuerpo sentado con babero `W` y dos patas `W` en la última fila;
  - cola `O` que sube en curva por las columnas 11-13.
- **Estados** (cara y accesorio):
  - `Idle`: ojos abiertos; accesorio `…` en `S`, tres píxeles en la fila de la boca.
  - `Scanning`: ojos mirando a la derecha; lupa `S` con mango.
  - `Thinking`: ojos cerrados (`K` horizontal de 2×1); tres burbujas `S` en diagonal hacia arriba.
  - `Success`: ojos `^` (arco de 3 px); dos `+` en `S`.
  - `Warning`: ojos abiertos y orejas alertas; triángulo `O` con `!` `K`.
  - `Error`: ojos `x` de 3×3 y orejas caídas; círculo con `×` en `O`.
- **Leyendas:**
  - `Idle` → `Tija is ready.`
  - `Scanning` → `Scanning repository...`
  - `Thinking` → `Thinking...`
  - `Success` → `All tasks completed!`
  - `Warning` → `Needs your review.`
  - `Error` → `Check failed.`
- `panel` dibuja la leyenda en `mutedStyle` bajo el sprite. En `screenResult` de Doctor y Status, `Error` usa `Something failed.` (se pasa como override).

- [ ] **Step 1: Tests que fallan**
  - Actualizar `TestSpritesDistinctAndBounded`:
    - 6 estados;
    - cada grilla de 18 filas × 20 columnas, solo con `O K W S .`;
    - la columna 14 vacía en todas;
    - en `Idle`, las columnas 0-13 iguales en filas 10-17 a las de todos los demás estados (cuerpo compartido);
    - cada estado distinto de los demás;
    - `lipgloss.Width(Sprite(s)) == 20` y `lipgloss.Height(Sprite(s)) == 9`.
  - `TestCaptions`: los 6 textos exactos.
  - `TestPanelShowsCaption`: `screenRunning` muestra `Scanning repository...`.
- [ ] **Step 2:** `go test ./internal/tui` → FAIL.
- [ ] **Step 3:** Dibujar las grillas y agregar `Caption` y la leyenda en `panel`. Revisar a ojo con `go run ./cmd/tija` en Windows Terminal y en la terminal de VS Code, comparando con la sección 04 de la imagen de marca.
- [ ] **Step 4:** `go test ./internal/tui` → PASS. Hacer capturas antes y después y guardarlas como `docs/reference/brand/tui-before.png` y `tui-after.png` (reemplazan la `image.png` sin versionar).
- [ ] **Step 5: Commit** `feat(tui): redraw the pixel mascot after the brand sheet and add the thinking state`

---

### Task 10: Startup screen estilo concepto 06

**Diagnóstico:**
- **Gato y logo:** la startup usa el gato ASCII mínimo del draft (`/\_/\ ( o.o ) > ^ <`) y un "TIJA" de 4 letras en negrita. El concepto 06 tiene un **gato ASCII grande** con el prompt `>_` en la boca, al lado de un **logotipo en bloques naranjas**.
- **Menú:** el ítem seleccionado solo cambia de color. En la marca hay una **barra de selección** de ancho completo.
- **Mascota pixel:** compite con el menú en la columna derecha. En el concepto 06 la startup no la muestra; la mascota pixel es para los estados (sección 03).
- **Pie:** no tiene el remate de marca `Small cat. Big workflows.`

**Files:**
- Create: `internal/tui/brand.go`
- Modify: `internal/tui/startup.go`, `internal/tui/menu.go`, `internal/tui/model.go` (footer), `internal/tui/tui_test.go`

**Interfaces:**
- Produces:
  - `const asciiCat`, que reemplaza al actual;
  - `const logoBlock`;
  - `func brandHeader(width int) string`: el gato y el logo lado a lado si hay espacio; si no, solo el logo en texto.

**Arte exacto** (copiar tal cual; el gato en `White`, el `>_` en `Orange`, el logo en `Orange` bold):
```text
   /\_____/\
  /  0   0  \
 (    >_     )
  \  '---'  /
 .-'       '-.
(__/ |   | \__)
```
```text
████████ ██      ██  █████
   ██    ██      ██ ██   ██
   ██    ██      ██ ███████
   ██    ██ ██   ██ ██   ██
   ██    ██  █████  ██   ██
```

**Layout:**
- **Columna izquierda:**
  - `brandHeader` (el gato, dos espacios y el logo; 6 filas, el logo alineado abajo);
  - una línea en blanco y el tagline;
  - una línea en blanco y el checklist.
- **Columna derecha:** `What do you want to do?` y el menú, **sin** la mascota pixel. Mientras se detecta, a la derecha va `Sprite(Scanning)` con su leyenda (Task 9).
- **Selección:** la fila seleccionada se pinta con fondo `Ink`, `>` en `Orange` y el título en `White` bold. El relleno con espacios llega hasta el ancho del título más el de la descripción, para que se vea como una barra. El resto de las filas: el título en `White` y la descripción en `mutedStyle`.
- **Footer:** el help a la izquierda y `Small cat. Big workflows.` (en `mutedStyle`) alineado a la derecha, si caben con al menos 2 espacios. Si no, solo el help.
- **Menos de 80 columnas:** se mantiene la lógica actual de layouts, con `brandHeader` degradado a `TIJA` en `logoStyle` y el gato pequeño anterior. Nada supera el ancho en 60 columnas.

- [ ] **Step 1: Tests que fallan**
  - `TestStartupViewContents`, actualizado: contiene `>_`, la primera fila de `logoBlock`, `What do you want to do?` y `Small cat. Big workflows.`. No contiene caracteres de media celda de la mascota (`▀`/`▄`) cuando `screen == screenMenu`.
  - `TestStartupViewFitsWidth`: para anchos 60, 80, 100 y 140, todas las líneas de `ansi.Strip(view)` miden ≤ ancho.
  - `TestMenuSelectionBar`: la línea seleccionada de `renderMenu` contiene la secuencia de fondo de `Ink` (comparar con `lipgloss.NewStyle().Background(lipgloss.Color(render.Ink)).Render(" ")` sin el reset) y las demás no.
- [ ] **Step 2:** `go test ./internal/tui` → FAIL.
- [ ] **Step 3:** Implementar. Verificar a ojo en 80 y en 120 columnas contra el concepto 06.
- [ ] **Step 4:** `go test ./internal/tui` → PASS.
- [ ] **Step 5: Commit** `feat(tui): restyle the startup screen after the brand concept`

---

### Task 11: Flujo Pull Request en la TUI

**Files:**
- Create: `internal/tui/pr_view.go`
- Modify: `internal/tui/menu.go`, `internal/tui/model.go`, `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `engine.PlanCreatePR`, `engine.ApplyCreatePR`, `engine.PREvent`, `Thinking`/`Caption` (Task 9).
- Produces:
  - `itemPR = MenuItem{Key: "pr", Title: "Pull Request", Description: "Draft and open a pull request"}`;
  - `MenuItems(true)` → `[PR, Check, Tests, Doctor, Status, Init]`;
  - `MenuItems(false)` → `[Init, PR, Check, Tests, Doctor, Status]`;
  - pantallas nuevas `screenPRWorking` y `screenPRPreview`.

**Comportamiento:**
- **`screenPRWorking`:** el panel con la mascota según la etapa del `PREvent`:
  - `context` → `Scanning`;
  - `check` → `Scanning`, con la vista de pasos de Quality reutilizada;
  - `agent` → `Thinking`, con el texto `Drafting with <agent>...`.
- **`screenPRPreview`:**
  - mascota `Success` si no hay problems y `Warning` si los hay;
  - cuerpo scrolleable: `title`, una línea en blanco, el cuerpo, y la lista ✓/✗ de validación;
  - teclas:
    - `e`: abre el editor con `tea.ExecProcess` sobre `.tija/session/pr.md` y, al volver, recarga y revalida (solo si el repo está inicializado);
    - `r`: regenera;
    - `enter`: si no hay problems, pasa a consentimiento (`screenConsent` reutilizado, con el texto del plan);
    - `esc`: vuelve al menú.
- **Consentimiento:**
  - `y` → `ApplyCreatePR` → `screenResult`, con `Success` y la URL;
  - `n` → vuelve al preview.
- **Footer de preview:** `↑/↓ scroll · e edit · r regenerate · enter create · esc back`.

- [ ] **Step 1: Tests que fallan**
  - `TestMenuItemsPhase3`: los dos órdenes exactos.
  - `TestPRWorkingShowsThinking`: un `PREvent{Stage:"agent"}` lleva a una vista con `Thinking...`.
  - `TestPRPreviewKeys`: con un `PRReport` válido inyectado, `enter` → `screenConsent`; `n` → preview; `esc` → detección o menú.
  - `TestPRPreviewBlocksCreateWithProblems`: con problems, `enter` no cambia de pantalla y la vista contiene `✗`.
- [ ] **Step 2:** `go test ./internal/tui` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `go test ./...` y `go vet ./...` → PASS.
- [ ] **Step 5: Commit** `feat(tui): add the pull request flow with thinking mascot and preview`

---

### Task 12: Verificación de salida de la fase

No produce código salvo arreglos. Es el criterio de salida del spec.

- [ ] **Step 1:** `gofmt -l .` sin salida, `go vet ./...` y `go test -count=1 ./...` → PASS.
- [ ] **Step 2:** En este repo (Tija), en una rama de prueba con commits: `go run ./cmd/tija pr prepare --no-check` con cada agente instalado (`--agent claude`, `--agent codex`, `--agent gemini`) y con `--no-agent`. Anotar en el PR cuál produjo un borrador válido al primer intento.
- [ ] **Step 3:** En GMX-frontend y en GMX-ClaimsAPI: `tija pr create` real hacia la rama target. `pr-governance.yml` debe pasar a la primera. Este es el primer PR de los 5 consecutivos del criterio de salida. Registrar los 5 en la descripción del PR de la fase a medida que se completen.
- [ ] **Step 4:** Abrir la TUI (`go run ./cmd/tija`) en 80 y 120 columnas y comparar con la imagen de marca. Capturas en el PR.
- [ ] **Step 5:** El PR de la fase se abre con `tija pr create`, el propio comando.
