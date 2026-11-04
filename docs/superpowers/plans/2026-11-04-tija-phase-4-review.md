# Tija Fase 4 — Review: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar `tija review` (modos `review`, `--quality`, `--deep`): revisión del diff por el agente con los packs de Thermos y las reglas de arquitectura del repo, con cada hallazgo verificado por Tija antes de mostrarse, exit codes por umbral e integración con `pre-pr`, `pr prepare`, `doctor`, `init` y la TUI.

**Architecture:**
- `internal/review` es puro (sin git, sin subprocesos): parseo del diff y partes, severidades, parseo y verificación de la respuesta del agente, síntesis de `--deep`, regla de las 1000 líneas, cruce con comentarios del PR, resultado, exit code, prompt y sesión.
- `internal/rules` carga y valida `.tija/rules/architecture/*.md` y ejecuta los checks deterministas. No importa `internal/review`; el engine traduce entre ambos.
- `internal/gitx` suma merge-base, diff del árbol de trabajo, archivos sin seguimiento y conteo de líneas en un commit.
- `internal/agents` suma el modo solo lectura.
- `engine.RunReview` orquesta. CLI, render, TUI, `doctor`, `init` y `pr prepare` son capas finas o integraciones.

**Tech Stack:** Go 1.27, Cobra, Bubble Tea v2 / Lip Gloss v2, yaml.v3, `golang.org/x/sync/errgroup`. **Sin dependencias nuevas.** Globs con `comments.Match` (ya existe en `internal/comments/glob.go`).

**Spec:**
- `docs/superpowers/specs/2026-11-04-tija-phase-4-review-design.md` (completo; las secciones se citan como §N).
- `docs/superpowers/specs/2026-10-22-tija-phases-design.md`: secciones 2 a 4 (restricciones 1 a 13).

## Global Constraints

- Todo lo de las fases 1 a 3b sigue vigente: módulo `github.com/Ps3udoDev/tija-cli`; UI en inglés y contenido generado en `content.language` (default `es`); paleta C, símbolos `✓ ✗ ! >`; sin TTY o con `--json`, `--no-color` o `NO_COLOR`: salida plana; Windows es plataforma de primera clase (rutas con `/` en todo dato persistido o comparado, CRLF tolerado).
- **`review` no escribe archivos del repo.** Solo `.tija/session/review.json` (ya ignorado por `init`). En un repo sin manifest no escribe nada.
- **El agente corre en solo lectura** en la raíz del repo (§3, Task 7).
- **Exit codes (§11):** `0` ningún hallazgo con severidad ≥ umbral; `1` al menos uno; `2` uso o entorno (no es repo, sin merge-base, reglas inválidas, una parte del agente que falla tras el reintento). Sin agente instalado o con `--no-agent` no es error.
- **Umbral:** `review.failOn` (default `high`), reemplazado por `--fail-on critical|high|medium|low|none`.
- **Tiers y timeouts:** `review` y `--quality` usan `agents.tasks.review.tier` (default `balanced`); `--deep` siempre `deep`. Timeout por llamada: `fast` 3 min, `balanced` 6 min, `deep` 12 min; `--agent-timeout` los reemplaza.
- **Partes:** `review.maxLines` (default `3000`) líneas cambiadas (agregadas + borradas) por parte; como máximo 3 llamadas al agente a la vez en total (`errgroup.SetLimit(3)`).
- **Defaults del manifest (§12):** `failOn: high`, `rules: [".tija/rules/architecture/*.md"]`, `maxLines: 3000`, `exclude: ["**/*.lock", "**/package-lock.json", "**/pnpm-lock.yaml", "**/*.min.js", "dist/**", "build/**", "**/bin/**", "**/obj/**", "**/*.g.cs", "**/*.generated.*"]`.
- **`DefaultsVersion` pasa a `"5"`.**
- **Binario:** `go build -ldflags="-s -w"` ≤ 10 MB (hoy ~7,36 MB).
- **Atribución (restricción 11):** la orquestación de `--deep` se atribuye al skill `thermos` de Cursor en `THIRD_PARTY_NOTICES.md` y en el comentario de paquete de `internal/review/synth.go`. Los presets de arquitectura son propios y no llevan datos de empresas.
- **Menú de la TUI:** `Review` aparece siempre que haya repo git, justo después de `Pull Request`.

## Review Focus

1. **Archivos con CRLF (Windows):** la evidencia y los números de línea deben coincidir igual que con LF. Test en Task 4 (`TestVerifyCRLF`).
2. **Rutas con espacios, renombradas o entre comillas en el diff** (`diff --git "a/x y.ts" "b/x y.ts"`, `rename from/to`): el archivo debe quedar con su ruta nueva sin comillas. Test en Task 2 (`TestParseDiffQuotedAndRenamed`).
3. **Respuesta del agente con texto o fences alrededor del JSON:** se extrae el objeto y no se gasta el reintento. Test en Task 4 (`TestParseResponseFencedAndProse`).
4. **Archivo borrado en el diff:** no se manda como archivo revisable y un hallazgo sobre él se descarta con `line-outside-diff`. Test en Task 4 (`TestVerifyDeletedFile`).
5. **Globs de reglas contra rutas con `\`:** `files`/`appliesTo` deben coincidir con rutas normalizadas. Test en Task 3 (`TestRunChecksWindowsPaths`).

---

### Task 1: Manifest `review`, catálogo de reglas en los packs y `DefaultsVersion` 5

**Files:**
- Modify: `internal/manifest/manifest.go` (struct `Review`, campo en `Manifest`, `FromDetection`, `Validate`)
- Modify: `internal/manifest/patch.go` y `internal/engine/init.go` (proponer la sección `review` en repos existentes, igual que se hizo con `comments` en la 3a)
- Modify: `internal/skills/skills.go` (`Rules` en frontmatter y `Pack`)
- Modify: `internal/assets/defaults/skills/thermo-review/SKILL.md`, `internal/assets/defaults/skills/thermo-quality/SKILL.md`
- Modify: `internal/assets/assets.go` (`DefaultsVersion = "5"`)
- Test: `internal/manifest/manifest_test.go`, `internal/skills/skills_test.go`, `internal/assets/assets_test.go`, `internal/engine/init_test.go`

**Interfaces:**
- Produces:
  - `type manifest.Review struct { FailOn string \`yaml:"failOn,omitempty"\`; Rules []string \`yaml:"rules,omitempty"\`; Exclude []string \`yaml:"exclude,omitempty"\`; MaxLines int \`yaml:"maxLines,omitempty"\` }` y `Manifest.Review Review \`yaml:"review,omitempty"\``.
  - `func manifest.DefaultReview() Review` (valores de Global Constraints) y `func (r Review) WithDefaults() Review` (rellena campos vacíos con los defaults).
  - `var manifest.FailOnValues = []string{"critical", "high", "medium", "low", "none"}`.
  - `skills.Pack.Rules []string` (json `rules,omitempty`), leído de `rules:` en el frontmatter.

- [ ] **Step 1: Tests que fallan**
  - `TestReviewDefaults`: `DefaultReview().FailOn == "high"`, `MaxLines == 3000`, `Rules == [".tija/rules/architecture/*.md"]`, `Exclude` igual a la lista de Global Constraints. `Review{FailOn: "low"}.WithDefaults()` conserva `"low"` y completa el resto.
  - `TestValidateReviewFailOn`: `failOn: urgent` es un problema de validación que nombra `review.failOn`; `failOn: none` es válido; `maxLines: -1` es inválido.
  - `TestFromDetectionHasReview`: `FromDetection(...).Review == DefaultReview()`.
  - `TestParseSkillRules`: un `SKILL.md` con `rules: [a, b]` produce `Pack.Rules == ["a","b"]`.
  - `TestThermoPacksDeclareRules` (assets): `thermo-review` declara exactamente `breaking-functionality, devex-regression, feature-leak, security-vulnerability`; `thermo-quality` declara `code-judo, 1000-lines-ceiling, spaghetti-condition, clean-design, type-boundary, canonical-layer, clean-orchestration`; ambos tienen en `adapted.changes` una línea que menciona `rules`.
  - `TestInitProposesReviewSection` (engine): un manifest existente sin `review` produce una acción de update del manifest cuyo diff agrega `review:`; uno que ya la tiene no cambia.
- [ ] **Step 2:** `go test ./internal/manifest/ ./internal/skills/ ./internal/assets/ ./internal/engine/ -run 'Review|SkillRules|ThermoPacks|InitProposesReview'` → FAIL.
- [ ] **Step 3: Implementar.** Agregar `rules:` al frontmatter de ambos packs y la línea de cambio `"Declared the rule ids in frontmatter (rules) so Tija can verify findings."`. Subir `DefaultsVersion` a `"5"`.
- [ ] **Step 4:** el mismo comando → PASS; luego `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(manifest): add review settings and declare thermos rule ids`.

---

### Task 2: `internal/review` — diff del lado nuevo y partes

**Files:**
- Create: `internal/review/diff.go`, `internal/review/chunk.go`
- Test: `internal/review/diff_test.go`, `internal/review/chunk_test.go`

**Interfaces:**
- Produces:
  - `type Hunk struct { NewStart, NewLines int; Added []int }` — `Added` son números de línea (lado nuevo) de líneas `+`.
  - `type FileDiff struct { Path string; Deleted, Binary bool; Hunks []Hunk; Text string; Removed int }` — `Path` con `/`, sin prefijo `b/` ni comillas; `Text` es el diff del archivo tal cual (para el prompt); `Removed` = líneas `-`.
  - `func ParseDiff(text string) []FileDiff`
  - `func SyntheticAdded(path, content string) FileDiff` — archivo sin seguimiento: un hunk con todas las líneas agregadas y `Text` en formato unified (`--- /dev/null`, `+++ b/<path>`).
  - `func (f FileDiff) Touches(line, endLine int) bool` — `[line,endLine]` intersecta algún `[NewStart, NewStart+NewLines-1]`.
  - `func (f FileDiff) AddedLines() []int` y `func (f FileDiff) ChangedLines() int` (agregadas + `Removed`).
  - `func Chunks(files []FileDiff, maxLines int) [][]FileDiff`

- [ ] **Step 1: Tests que fallan**
  - `TestParseDiffBasic`: un diff con dos archivos y dos hunks: `Path`, `NewStart/NewLines` de cada hunk y `Added` exactos; `ChangedLines` correcto.
  - `TestParseDiffQuotedAndRenamed` (Review Focus 2): `diff --git "a/src/x y.ts" "b/src/x y.ts"` → `Path == "src/x y.ts"`; un rename con `rename to src/new.ts` y un hunk → `Path == "src/new.ts"`.
  - `TestParseDiffDeletedAndBinary`: `+++ /dev/null` → `Deleted == true`, sin hunks revisables; `Binary files … differ` → `Binary == true`.
  - `TestParseDiffCRLF`: el mismo diff con `\r\n` da el mismo resultado que con `\n`.
  - `TestTouches`: hunk `NewStart 10, NewLines 5` → `Touches(14,14)` true, `Touches(15,20)` false, `Touches(1,10)` true.
  - `TestSyntheticAdded`: contenido de 3 líneas → `Added == [1,2,3]`, `Text` empieza con `--- /dev/null`.
  - `TestChunksByDirectory`: archivos en `a/` (2000 líneas cambiadas en total) y `b/` (1500) con `maxLines 3000` → 2 partes, una por carpeta; con `maxLines 10000` → 1 parte; un archivo de 5000 líneas → una parte solo para él. El orden de archivos dentro y entre partes es estable (ordenado por ruta).
- [ ] **Step 2:** `go test ./internal/review/ -run 'ParseDiff|Touches|Synthetic|Chunks'` → FAIL (paquete no existe).
- [ ] **Step 3: Implementar.** `Chunks`: ordenar por ruta, agrupar por `path.Dir`, llenar partes con grupos completos en orden; un grupo que no entra en la parte actual abre una nueva; un grupo mayor que `maxLines` se reparte archivo por archivo; un archivo mayor que `maxLines` va solo. `Deleted` y `Binary` no entran en las partes.
- [ ] **Step 4:** el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(review): parse the new side of a diff and split it into chunks`.

---

### Task 3: `internal/rules` — reglas de arquitectura, checks y presets

**Files:**
- Create: `internal/rules/rules.go`, `internal/rules/checks.go`
- Create: `internal/assets/presets/architecture/dotnet-clean.md`, `internal/assets/presets/architecture/react-features.md`
- Modify: `internal/assets/assets.go` (embed separado de `defaults/`, para que `Files()` **no** los instale)
- Test: `internal/rules/rules_test.go`, `internal/rules/checks_test.go`, `internal/assets/assets_test.go`

**Interfaces:**
- Produces:
  - `type Check struct { ID, Severity, Message string; Files []string; Forbid *regexp.Regexp; Source string }` — `Files` acepta string o lista en YAML; `Source` es la ruta del archivo de reglas.
  - `type AgentRule struct { ID, Title, Text, Source string; AppliesTo []string }`
  - `type File struct { Name, Description, Path string; AppliesTo []string; Checks []Check; Rules []AgentRule }`
  - `type Set struct { Files []File }` con `func (s Set) Checks() []Check`, `func (s Set) AgentRules(paths []string) []AgentRule` (solo las de archivos cuyo `appliesTo` coincide con alguna ruta; sin `appliesTo` aplica siempre).
  - `func Load(root string, globs []string, reserved []string) (Set, []error)` — `reserved` son ids que no se pueden usar (los de los packs).
  - `type Changed struct { Path string; Added map[int]string }`
  - `type Violation struct { CheckID, Severity, Message, File, Text string; Line int }`
  - `func (s Set) Run(files []Changed) []Violation` — un check se evalúa en un archivo si coincide con `Files` del check y con `AppliesTo` de su archivo de reglas (vacío = todo).
  - `func assets.Presets() []string` (nombres ordenados) y `func assets.Preset(name string) ([]byte, bool)`; `const assets.ArchitectureDir = ".tija/rules/architecture"`.

- [ ] **Step 1: Tests que fallan**
  - `TestLoadRuleFile`: el ejemplo de backend de §6.1 produce 1 archivo, 1 check (`be-domain-no-infra`, `High`) y 1 regla para el agente (`be-thin-controllers`, título `Controladores delgados`, texto sin el encabezado).
  - `TestLoadNoFiles`: sin la carpeta → `Set{}` y sin errores.
  - `TestLoadErrors`: cada caso produce un error que nombra archivo e id: regex inválido; `severity: Urgent`; id duplicado entre dos archivos; id igual a uno de `reserved`; frontmatter sin `name`; YAML inválido.
  - `TestRunChecksAddedLinesOnly`: un `forbid` que coincide con una línea agregada y con una no agregada del mismo archivo → 1 violación con línea y texto de la agregada.
  - `TestRunChecksWindowsPaths` (Review Focus 5): `Changed.Path` `src\Api.Domain\Order.cs` coincide con `files: "src/*.Domain/**/*.cs"`.
  - `TestAgentRulesAppliesTo`: una regla con `appliesTo: ["src/**/*.cs"]` sale para `["src/a.cs"]` y no para `["web/a.tsx"]`.
  - `TestPresetsEmbedded` (assets): `Presets() == ["dotnet-clean","react-features"]`; cada uno carga con `rules.Load` sin errores, tiene al menos 2 checks y 3 reglas; `Files()` no contiene ninguna ruta bajo `.tija/rules/architecture/`.
- [ ] **Step 2:** `go test ./internal/rules/ ./internal/assets/ -run 'Load|RunChecks|AgentRules|Presets'` → FAIL.
- [ ] **Step 3: Implementar.** Frontmatter con el delimitador `---` igual que `skills.Parse`. Reglas para el agente: cada línea `## <id>: <título>` abre una regla hasta el siguiente `## `. Severidades válidas `Critical|High|Medium|Low` (sin distinguir mayúsculas al leer, normalizadas al escribir). Presets con el contenido de §6.2 (los ejemplos de §6.1 del spec y de la conversación de diseño son la base).
- [ ] **Step 4:** el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(rules): load optional architecture rules and run their checks`.

---

### Task 4: `internal/review` — hallazgos, parseo de la respuesta y verificación

**Files:**
- Create: `internal/review/finding.go`, `internal/review/verify.go`
- Test: `internal/review/finding_test.go`, `internal/review/verify_test.go`

**Interfaces:**
- Consumes: `FileDiff`, `Touches` (Task 2).
- Produces:
  - `type Severity string` con `Critical, High, Medium, Low`; `func ParseSeverity(s string) (Severity, bool)` (insensible a mayúsculas); `func (s Severity) Rank() int` (Critical 4 … Low 1); `func AtLeast(s Severity, failOn string) bool` (`none` → siempre false).
  - `type Finding struct` con los campos de `findings[]` de §13: `ID, Source string` (`agent|check|quality`), `Passes []string`, `File string`, `Line, EndLine int`, `Severity Severity`, `Rule, Title, Explanation, Suggestion, Evidence string`, `ConfirmedBy []string`, `Disputed bool`, `Severities []Severity`, `PRComments []PRComment` — tags json en camelCase como en §13.
  - `type Discarded struct { Pass, File string; Line int; Reason string }` (json `pass,file,line,reason`).
  - Constantes de motivo: `ReasonFileNotInDiff = "file-not-in-diff"`, `ReasonLineOutsideDiff = "line-outside-diff"`, `ReasonEvidenceNotFound = "evidence-not-found"`, `ReasonInvalidSeverity = "invalid-severity"`, `ReasonUnknownRule = "unknown-rule"`.
  - `type AgentFinding struct` con los campos JSON de §7 (`file,line,endLine,severity,rule,title,explanation,suggestion,evidence`).
  - `func ParseResponse(out string) ([]AgentFinding, error)`
  - `type VerifyInput struct { Pass string; Files map[string]FileDiff; ReadLines func(path string) ([]string, error); Rules map[string]bool }`
  - `func Verify(in VerifyInput, raw []AgentFinding) ([]Finding, []Discarded)`

- [ ] **Step 1: Tests que fallan**
  - `TestAtLeast`: `AtLeast(High,"high")` true, `AtLeast(Medium,"high")` false, `AtLeast(Low,"low")` true, `AtLeast(Critical,"none")` false.
  - `TestParseResponseFencedAndProse` (Review Focus 3): `"Aquí va:\n```json\n{\"findings\":[…]}\n```"` y el JSON sin fences dan los mismos hallazgos; `{"findings": []}` → slice vacío sin error.
  - `TestParseResponseInvalid`: texto sin JSON, JSON sin `findings`, `findings` que no es lista → error con un mensaje apto para el reintento.
  - `TestVerifyReasons`: una tabla con un caso por motivo (los cinco de §8) más un caso válido; cada inválido da exactamente un `Discarded` con su motivo y el `Pass`; el válido da un `Finding` con `Source "agent"`, `Passes [pass]`, severidad normalizada y `EndLine = Line` si vino vacío.
  - `TestVerifyEvidenceWindow`: la evidencia está 3 líneas por encima de `line` → válido; 4 líneas por encima → `evidence-not-found`; evidencia con espacios/tabs distintos → válido; evidencia de 6 líneas → `evidence-not-found`; vacía → `evidence-not-found`.
  - `TestVerifyCRLF` (Review Focus 1): `ReadLines` devuelve líneas con `\r` final → mismo resultado que sin `\r`.
  - `TestVerifyDeletedFile` (Review Focus 4): `Files` contiene un `FileDiff{Deleted: true}` → `line-outside-diff`.
  - `TestVerifyOrderOfChecks`: un hallazgo con archivo fuera del diff **y** severidad inválida reporta `file-not-in-diff`.
- [ ] **Step 2:** `go test ./internal/review/ -run 'AtLeast|ParseResponse|Verify'` → FAIL.
- [ ] **Step 3: Implementar.** `ParseResponse`: quitar fences (misma idea que `agents.stripFences`), tomar desde el primer `{` hasta el último `}` y decodificar. Normalización de evidencia: cada línea con `strings.Fields` unidas por un espacio; se busca la secuencia de líneas de la evidencia como bloque contiguo dentro de `[line-3, endLine+3]` (acotado a los límites del archivo).
- [ ] **Step 4:** el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(review): parse agent findings and verify them against the diff`.

---

### Task 5: `internal/review` — síntesis, 1000 líneas, comentarios del PR, resultado y sesión

**Files:**
- Create: `internal/review/synth.go`, `internal/review/ceiling.go`, `internal/review/prcomments.go`, `internal/review/result.go`, `internal/review/session.go`
- Test: `internal/review/synth_test.go`, `internal/review/ceiling_test.go`, `internal/review/prcomments_test.go`, `internal/review/result_test.go`, `internal/review/session_test.go`

**Interfaces:**
- Consumes: `Finding`, `Severity`, `AtLeast` (Task 4).
- Produces:
  - `func Merge(findings []Finding) []Finding` — §9; no une hallazgos con `Source != "agent"`.
  - `func Sort(findings []Finding)` — severidad desc, luego `len(ConfirmedBy)` desc, luego archivo y línea; después asigna `ID` `F1…Fn`.
  - `type LineCount struct { Path string; Before, After int }` y `func Ceiling(counts []LineCount) []Finding` — `Before <= 1000 && After > 1000` → hallazgo `Source "check"`, `Rule "1000-lines-ceiling"`, `Severity Medium`, `Line 1001`, `File Path`.
  - `type PRComment struct { Path string; Line int; Author, Body, URL string }` y `func AttachPRComments(findings []Finding, comments []PRComment) ([]Finding, []PRComment)` — adjunta si `Path == File` y `Line ∈ [Line-3, EndLine+3]`; devuelve los no cubiertos.
  - `type Result struct` con la forma de `--json` de §13: `Mode, Base, Commit string; Dirty bool; FailOn string; Exit int; Quality QualitySummary; Findings []Finding; Discarded []Discarded; PRComments PRCommentsSummary; Chunks ChunkSummary; Summary map[Severity]int; Warnings []string; SavedAt string` (json `savedAt,omitempty`, RFC 3339; solo lo llena `SaveSession`).
  - `type QualitySummary struct { Reused bool; Status string }`, `type PRCommentsSummary struct { Read bool; Uncovered []PRComment }`, `type ChunkSummary struct { Total, Failed int }`.
  - `func (r *Result) Finalize()` — llena `Summary` y `Exit` (`2` si `Chunks.Failed > 0`, si no `1` si algún hallazgo `AtLeast(failOn)`, si no `0`).
  - `const SessionPath = ".tija/session/review.json"`, `func SaveSession(root string, r Result) error`, `func LoadSession(root string) (Result, bool, error)` (archivo ausente → `false, nil`).

- [ ] **Step 1: Tests que fallan**
  - `TestMergeConfirmed`: `review` y `quality` reportan la misma regla en `a.ts` líneas 10–12 y 11 → un hallazgo con `ConfirmedBy ["quality","review"]` (ordenado) y `Passes` ambos.
  - `TestMergeDisputed`: lo mismo con `High` y `Medium` → queda `High`, `Disputed true`, `Severities [High, Medium]`.
  - `TestMergeDifferentRulesKept`: misma ubicación, reglas distintas → dos hallazgos.
  - `TestMergeSkipsChecks`: un hallazgo `check` y uno `agent` en la misma línea y regla → siguen siendo dos.
  - `TestSortAndIDs`: orden por severidad, confirmados primero a igual severidad; IDs `F1..`.
  - `TestCeiling`: `{990→1005}` → 1 hallazgo Medium en línea 1001; `{1005→1100}` y `{900→1000}` → ninguno.
  - `TestAttachPRComments`: un comentario a 2 líneas de un hallazgo se adjunta; uno en otro archivo queda en no cubiertos.
  - `TestFinalizeExit`: con un `High` y `failOn high` → `Exit 1`; con `failOn critical` → `0`; con `none` → `0`; con `Chunks.Failed 1` → `2`; `Summary` cuenta por severidad.
  - `TestSessionRoundTrip`: `SaveSession` + `LoadSession` devuelven el mismo `Result`; `LoadSession` sin archivo → `false, nil`.
- [ ] **Step 2:** `go test ./internal/review/ -run 'Merge|Sort|Ceiling|Attach|Finalize|Session'` → FAIL.
- [ ] **Step 3: Implementar.** Comentario de paquete en `synth.go` con la atribución: *"The parallel review and synthesis follow the idea of the `thermos` skill by Cursor (https://github.com/cursor/plugins, MIT), reimplemented in Go."*
- [ ] **Step 4:** el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(review): merge deep findings, add the line ceiling and the session`.

---

### Task 6: `internal/review` — prompt

**Files:**
- Create: `internal/review/prompt.go`
- Test: `internal/review/prompt_test.go`

**Interfaces:**
- Consumes: `FileDiff` (Task 2).
- Produces:
  - `type PromptRule struct { ID, Title, Text string }`
  - `type PromptInput struct { PackBody string; Rules []PromptRule; Quality []string; Files []FileDiff; Handled []string; Language string; Retry string }` — `Quality` son líneas ya formateadas (el engine usa la misma forma que `describeResult`); `Handled` son los ids que calcula Tija; `Retry` es el error de la respuesta anterior o `""`.
  - `func BuildPrompt(in PromptInput) string`

- [ ] **Step 1: Test que falla** `TestBuildPrompt`: el prompt contiene, en este orden, el cuerpo del pack, las reglas como `## <id>: <título>`, el resumen de calidad, la lista de rutas, cada `FileDiff.Text` en un fence que no se rompe si el diff contiene ```` ``` ````, la frase que nombra cada id de `Handled` como "do not report", el nombre del idioma (`Spanish` para `es`, vía `manifest.LanguageName`), el esquema JSON de §7 y la instrucción de responder solo JSON y no modificar archivos. Con `Retry != ""` incluye el error y pide corregir el formato. Sin reglas, no aparece la sección de reglas.
- [ ] **Step 2:** `go test ./internal/review/ -run BuildPrompt` → FAIL.
- [ ] **Step 3: Implementar.** Instrucciones del prompt en inglés (como los demás prompts de Tija); el contenido pedido en `Language`. Mover la lógica de `engine.diffFence` a `func review.Fence(s string) string` y hacer que `engine` la use (sin cambiar el comportamiento de `pr prepare`).
- [ ] **Step 4:** el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(review): build the review prompt`.

---

### Task 7: Agente en solo lectura y operaciones git del review

**Files:**
- Modify: `internal/agents/provider.go` (`Request.ReadOnly`, `Argv`)
- Modify: `internal/gitx/pr.go` (o nuevo `internal/gitx/review.go`)
- Test: `internal/agents/provider_test.go`, `internal/gitx/gitx_test.go` (o `review_test.go`)

**Interfaces:**
- Produces:
  - `agents.Request.ReadOnly bool`; `func Argv(provider, bin, tier, tmpDir string, readOnly bool) []string` (actualizar los llamadores existentes con `false`).
  - `func (r Repo) MergeBase(ctx context.Context, base string) (string, bool)` — `git merge-base <base> HEAD`.
  - `func (r Repo) WorkingDiff(ctx context.Context, rev string) string` — `git diff --no-color --no-ext-diff -U3 -M <rev> --` (árbol de trabajo contra `rev`), CRLF normalizado.
  - `func (r Repo) Untracked(ctx context.Context) []string` — `git ls-files --others --exclude-standard -z`, rutas con `/`.
  - `func (r Repo) LineCountAt(ctx context.Context, rev, path string) (int, bool)` — líneas de `git show <rev>:<path>`; `false` si no existe en `rev`.

- [ ] **Step 1: Tests que fallan**
  - `TestArgvReadOnly`: claude con `readOnly` incluye `--allowedTools Read,Grep,Glob` y `--disallowedTools Bash,Edit,Write,NotebookEdit,WebFetch,WebSearch`; sin `readOnly` los args son los de hoy. codex sigue con `--sandbox read-only` en ambos casos; gemini no cambia.
  - `TestMergeBaseAndWorkingDiff` (repo temporal con `testutil`): rama con un commit y un cambio sin commitear → `WorkingDiff(mergeBase)` contiene ambos cambios.
  - `TestUntracked`: un archivo nuevo aparece; uno en `.gitignore` no; un nombre con espacio llega intacto.
  - `TestLineCountAt`: archivo de 3 líneas en el merge-base → `3, true`; archivo nuevo → `0, false`.
- [ ] **Step 2:** `go test ./internal/agents/ ./internal/gitx/ -run 'ReadOnly|MergeBase|Untracked|LineCountAt'` → FAIL.
- [ ] **Step 3: Implementar.** `Generate` pasa `req.ReadOnly` a `Argv`.
- [ ] **Step 4:** el mismo comando → PASS; `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(agents): run agents read-only and add git helpers for review`.

---

### Task 8: `engine.RunReview`

**Files:**
- Create: `internal/engine/review.go` (orquestación), `internal/engine/review_context.go` (contexto, diff, calidad, checks), `internal/engine/review_agent.go` (llamadas en paralelo, reintento)
- Test: `internal/engine/review_test.go`

**Interfaces:**
- Consumes: todo lo anterior; `RunQuality`, `FreshTests`, `loadValidManifest`, `branchRules`, `openPRFor`, `loadSkillPackBody`, `skills.Load`.
- Produces:
  - `var ErrInvalidRules = errors.New("invalid architecture rules")` y `var ErrNoMergeBase = errors.New("no merge base with the target branch")` (ambos exit 2).
  - `type ReviewRequest struct { Mode string; FailOn string; Agent string; NoAgent bool; AgentTimeout, CheckTimeout time.Duration; NoCheck, NoPRComments bool }` — `Mode` ∈ `review|quality|deep` (default `review`).
  - `type ReviewStage string` con `StageContext, StageQuality, StageChecks, StageAgent, StageVerify, StagePRComments, StageDone`.
  - `type ReviewEvent struct { Stage ReviewStage; Message, Pass string; Chunk, Chunks int; Quality *QualityEvent }`
  - `type ReviewReport struct { review.Result; Root string \`json:"-"\`; Initialized bool \`json:"-"\` }` con `func (r ReviewReport) ExitCode() int { return r.Exit }`.
  - `func RunReview(ctx context.Context, dir string, env sys.Env, req ReviewRequest, onEvent func(ReviewEvent)) (ReviewReport, error)`

- [ ] **Step 1: Tests que fallan** (repos temporales; agente simulado con `sys.Env.RunInput` falso que devuelve JSON según el pack que aparezca en el prompt)
  - `TestReviewNothingToReview`: sin cambios contra el target → exit 0, `Findings` vacío, mensaje "nothing to review".
  - `TestReviewUsesWorkingTreeAndUntracked`: un cambio commiteado, uno sin commitear y un archivo nuevo → los tres archivos están en el prompt.
  - `TestReviewExcludes`: un `package-lock.json` cambiado no aparece en el prompt.
  - `TestReviewVerifiedAndDiscarded`: el agente devuelve un hallazgo válido y uno con evidencia falsa → 1 en `Findings`, 1 en `Discarded` con `evidence-not-found`; exit 1 si el válido es `High`.
  - `TestReviewRetryOnce`: primera respuesta sin JSON, segunda válida → éxito, el segundo prompt contiene el error; dos respuestas inválidas → `Chunks.Failed 1`, exit 2, y los hallazgos de checks igual presentes.
  - `TestReviewDeepTwoPassesParallel`: `Mode deep` → dos llamadas por parte con tier `deep`, nunca más de 3 simultáneas (contador en el falso), y la síntesis aplicada.
  - `TestReviewNoAgent`: sin CLI de agente → warning `"! no agent CLI found — only deterministic checks ran"`, checks y calidad corren, exit según ellos (0 sin hallazgos).
  - `TestReviewChecksAndCeiling`: un archivo de reglas con `forbid` que coincide → hallazgo `check`; `Mode quality` con un archivo que cruza 1000 líneas → `1000-lines-ceiling`; `Mode review` → no.
  - `TestReviewInvalidRules`: regex inválido en una regla → `ErrInvalidRules`.
  - `TestReviewQualityReuseOrRun`: sesión fresca para HEAD y árbol limpio → `Quality.Reused true` y no corre `check`; árbol sucio → corre `check`; `NoCheck` → no corre; un paso fallido → hallazgo `quality` `quality-check-failed` `High` con `File` = log del paso.
  - `TestReviewChunks`: `review.maxLines: 10` y dos carpetas → `Chunks.Total 2`.
  - `TestReviewSessionOnlyWhenInitialized`: con manifest se escribe `.tija/session/review.json`; sin manifest no.
  - `TestReviewPRComments`: `gh` falso con PR abierto y un comentario en la línea del hallazgo → adjunto; `NoPRComments` → `PRComments.Read false` y no se llama a `gh api`; sin `gh` o con `gh auth status` fallando → `Read false`, una nota en `Warnings` y el exit no cambia.
- [ ] **Step 2:** `go test ./internal/engine/ -run Review` → FAIL.
- [ ] **Step 3: Implementar** con este orden (§5):
  1. Raíz, manifest válido o `FromDetection`, `target` con `branchRules`, `ResolveBase`, `MergeBase` (falla → `ErrNoMergeBase`).
  2. `ParseDiff(WorkingDiff(mergeBase))` + `SyntheticAdded` por cada `Untracked` (saltar binarios: un byte `0` en los primeros 8 KB) − `review.exclude` (vía `comments.Match`).
  3. Calidad: `FreshTests(session, head)` si el árbol está limpio; si no, `RunQuality(CheckMode)` salvo `NoCheck`.
  4. `rules.Load(root, review.rules, reserved)` con `reserved` = ids de los dos packs + `1000-lines-ceiling` + `quality-check-failed`; `Set.Run` sobre las líneas agregadas leídas del disco; cada `rules.Violation` pasa a `review.Finding{Source: "check", Rule: CheckID, Severity, File, Line, EndLine: Line, Title: Message, Explanation: Message, Evidence: Text}`; `Ceiling` en `quality` y `deep` con `LineCountAt` y el conteo actual.
  5. Sin agente o `NoAgent`: saltar al paso 8 con warning.
  6. Por cada parte × pasada, `agents.Generate` con `ReadOnly true`, `Dir: root`, tier y timeout de Global Constraints, `errgroup` con `SetLimit(3)`; respuesta inválida → un reintento con `PromptInput.Retry`; segundo fallo → parte fallida y warning con el error en una línea.
  7. `Verify` con `Rules` = `Pack.Rules` de la pasada (sin `1000-lines-ceiling`) ∪ ids de `AgentRules`; en `deep`, `Merge`.
  8. Unir hallazgos de agente, checks y calidad; `AttachPRComments` (rama con PR abierto vía `openPRFor`; comentarios con `gh api repos/{owner}/{repo}/pulls/<n>/comments --paginate`, usando `line` o, si es null, `original_line`); `Sort`; `Finalize`; `SaveSession` si hay manifest.
- [ ] **Step 4:** el mismo comando → PASS; `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(engine): run the review with verified agent findings`.

---

### Task 9: CLI `tija review`, render, workflow `pre-pr`, README y notices

**Files:**
- Create: `internal/cli/review.go`, `internal/render/review.go`
- Modify: `internal/cli/root.go` (registrar el comando), `internal/workflows/workflows.go` (`Allowed` suma `"review"`), `internal/assets/defaults/workflows/pre-pr.yaml`, `README.md`, `THIRD_PARTY_NOTICES.md`
- Test: `internal/cli/review_test.go`, `internal/render/review_test.go`, `internal/workflows/workflows_test.go`, `internal/assets/assets_test.go`

**Interfaces:**
- Consumes: `engine.RunReview`, `ReviewReport`, `ReviewEvent`.
- Produces: `func render.Review(p Printer, r engine.ReviewReport)`, `func render.ReviewProgress(p Printer, ev engine.ReviewEvent)`.

- [ ] **Step 1: Tests que fallan**
  - `TestReviewFlags`: `--quality` y `--deep` juntos → exit 2; `--fail-on urgent` → exit 2; `--fail-on` válido pisa `review.failOn`; `--agent-timeout 0s` → exit 2.
  - `TestReviewExitCodes`: el exit del proceso es `ReviewReport.ExitCode()`; `ErrNotARepo`, `ErrNoMergeBase`, `ErrInvalidRules` → 2.
  - `TestReviewJSON`: `--json` imprime el objeto de §13 (claves `mode, base, commit, dirty, failOn, exit, quality, findings, discarded, prComments, chunks, summary`) y nada más en stdout.
  - `TestRenderReview` (golden, modo plano): un `High` confirmado y disputado, un `Medium` de check y uno de calidad con línea 0 → formato de §13 (`✗` para Critical/High, `!` para Medium/Low; `file:line` omite `:0`; `[review, quality]` si está confirmado; `(disputed: High/Medium)`), la línea `Summary:` con conteos, descartes, estado de calidad y modo, y los warnings.
  - `TestPrePRHasReview` (assets): pasos `clean comments --write`, `check`, `review`, `pr prepare` en ese orden; `workflows.Validate` lo acepta.
- [ ] **Step 2:** `go test ./internal/cli/ ./internal/render/ ./internal/workflows/ ./internal/assets/ -run 'Review|PrePR'` → FAIL.
- [ ] **Step 3: Implementar.** Flags de §4. README: sección `## Review` (modos, flags, exit codes, umbral, verificación y motivos de descarte, `--json` documentado campo por campo) y `### Architecture rules` (formato de §6.1, RE2 sin lookbehind, presets y `--rules-preset`). `THIRD_PARTY_NOTICES.md`: en la entrada de Thermos, agregar que la orquestación paralela y la síntesis de `tija review --deep` siguen la idea del skill `thermos` (`internal/review/synth.go`).
- [ ] **Step 4:** el mismo comando → PASS; `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(cli): add tija review and run it in pre-pr`.

---

### Task 10: `doctor` valida las reglas e `init` ofrece los presets

**Files:**
- Modify: `internal/engine/doctor.go`, `internal/engine/init.go`, `internal/cli/init.go`, `internal/cli/prompt.go` (si hace falta mostrar la oferta)
- Test: `internal/engine/doctor_test.go`, `internal/engine/init_test.go`, `internal/cli/commands_test.go`

**Interfaces:**
- Produces:
  - `engine.Action.OptIn bool` (json `optIn,omitempty`): una acción que solo se aplica con aprobación explícita; `--yes` no la aprueba y, si no se aprueba, **no** aparece en `InitResult.Pending`.
  - Acción por preset: ID `create:.tija/rules/architecture/<name>.md`, `Reason "offer: architecture rules preset <name>"`, `NeedsConsent true`, `OptIn true`, registrada en `tija.installed` al aplicarse.
  - Flag `tija init --rules-preset <name>` (repetible).

- [ ] **Step 1: Tests que fallan**
  - `TestDoctorArchitectureRules`: sin reglas → check `ok` "no architecture rules (optional)"; con una regla válida → `ok` con el número de archivos; con un regex inválido → `fail` con archivo e id (exit 1 de `doctor`).
  - `TestInitOffersPresetForStack`: stack `react-vite` y carpeta de reglas vacía → `Actions` contiene la oferta de `react-features` (y no la de `dotnet-clean`); `dotnet-api` → `dotnet-clean`; stack `unknown` o carpeta con algún `.md` → sin ofertas.
  - `TestInitYesSkipsOptIn`: `--yes` crea todo lo demás y no crea el preset; la salida no lo lista como pendiente.
  - `TestInitRulesPresetFlag`: `--rules-preset dotnet-clean` en un repo `react-vite` crea `.tija/rules/architecture/dotnet-clean.md` y registra su hash; `--rules-preset nope` → exit 2 con los nombres válidos.
  - `TestInitInteractiveDefaultNo`: en modo interactivo, respuesta vacía → no se crea.
- [ ] **Step 2:** `go test ./internal/engine/ ./internal/cli/ -run 'ArchitectureRules|Preset|OptIn'` → FAIL.
- [ ] **Step 3: Implementar.** `PlanInit` ofrece solo el preset del stack detectado; `--rules-preset` agrega al plan la acción de cualquier preset embebido (aunque no sea del stack) y la aprueba.
- [ ] **Step 4:** el mismo comando → PASS; `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(init): offer architecture rule presets and validate rules in doctor`.

---

### Task 11: `pr prepare` cita la revisión

**Files:**
- Modify: `internal/engine/pr_context.go` (`PRContext.Review`), `internal/engine/pr_prompt.go`, `internal/engine/pr_draft.go` (borrador determinista)
- Test: `internal/engine/pr_prepare_test.go`

**Interfaces:**
- Consumes: `review.LoadSession`, `review.Result`.
- Produces: `PRContext.Review *review.Result` — no nil solo si la sesión existe, `Commit == Head` y `Dirty == false`.

- [ ] **Step 1: Tests que fallan**
  - `TestPRContextFreshReview`: sesión del commit actual y limpia → `Review` presente; de otro commit o `dirty` → nil.
  - `TestPRPromptCitesReview`: con `Review` (modo `deep`, 0 Critical, 0 High, 2 Medium), el prompt incluye una línea como fuente real de pruebas: `` `tija review --deep`: 0 critical, 0 high, 2 medium, 0 low `` y la instrucción de no copiar hallazgos; sin `Review`, nada.
  - `TestDeterministicDraftCitesReview`: el borrador sin agente incluye esa misma línea en `## Pruebas`.
- [ ] **Step 2:** `go test ./internal/engine/ -run 'FreshReview|CitesReview'` → FAIL.
- [ ] **Step 3: Implementar.** El comando mostrado es `tija review` + `--quality` o `--deep` según `Mode`.
- [ ] **Step 4:** el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat(pr): cite a fresh review in the pull request tests`.

---

### Task 12: TUI — `Review`

**Files:**
- Create: `internal/tui/review_view.go`
- Modify: `internal/tui/menu.go`, `internal/tui/model.go`
- Test: `internal/tui/tui_test.go` (o `review_view_test.go`)

**Interfaces:**
- Consumes: `engine.RunReview`, `ReviewEvent`, `ReviewReport`; patrón de `report_view.go` y `comments_view.go` (eventos por canal, `scroll.go`).

- [ ] **Step 1: Tests que fallan** (modelo, sin terminal real)
  - `TestMenuHasReview`: en un repo git, `Review` está justo después de `Pull Request`.
  - `TestReviewModeSelection`: la vista ofrece `Review`, `Quality`, `Deep`; `enter` en `Deep` inicia `RunReview` con `Mode "deep"`.
  - `TestReviewProgress`: eventos `StageAgent` con `Chunk/Chunks` y `Pass` se muestran; mascota `thinking` mientras corre.
  - `TestReviewResults`: lista agrupada por severidad, `enter` abre el detalle (explicación, sugerencia, evidencia), desplazamiento con flechas; mascota `success` con exit 0, `warning` con 1, `error` con 2; `q`/`esc` vuelve al menú.
- [ ] **Step 2:** `go test ./internal/tui/ -run Review` → FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4:** el mismo comando → PASS; `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(tui): add the review screen`.

---

### Task 13: Verificación de salida de la fase (§17)

- [ ] **Step 1: Estático y tests.** `gofmt -l .` (vacío), `go vet ./...` (0 issues), `go test ./...` (PASS).
- [ ] **Step 2: Binario.** `go build -ldflags="-s -w" -o tija.exe ./cmd/tija` → tamaño ≤ 10 MB; anotar el tamaño para el PR.
- [ ] **Step 3: Frontend real (Team4soft).** En un worktree aparte (nunca en el árbol de trabajo del usuario), en una rama real con cambios: `tija init` (aceptar la sección `review`; ofrecer `react-features`), `tija review`, `tija review --quality`, `tija review --deep`, cada uno con `--json` guardado. Revisar a mano **cada** hallazgo mostrado: archivo, línea y evidencia correctos (criterio 1). Anotar los descartes por motivo.
- [ ] **Step 4: Backend .NET real.** Lo mismo con `dotnet-clean`.
- [ ] **Step 5: Rama sembrada.** En cada worktree, una rama con una violación de cada check del preset y un bug (condición invertida en una función con test). Checks: 100 % de las violaciones (criterio 2). Agente: el bug aparece en `review` o `--deep`.
- [ ] **Step 6: Sin agente.** Con el `PATH` sin CLIs de agente: aviso, solo deterministas, exit según hallazgos (criterio 4).
- [ ] **Step 7: `pre-pr`.** En la rama sembrada, `tija run pre-pr` se detiene en `review` con exit 1 (criterio 5).
- [ ] **Step 8: `pr prepare`.** En una rama limpia con `review` sin hallazgos graves, `tija pr prepare` cita la revisión en `## Pruebas`.
- [ ] **Step 9: Handoff.** Actualizar `docs/superpowers/handoff/HANDOFF.md` con una sección de la Fase 4: commits, decisiones, verificación (tamaño, tests, resultados reales con conteos de hallazgos y descartes) y pendientes. Commit `docs(handoff): record phase 4 review verification`.
