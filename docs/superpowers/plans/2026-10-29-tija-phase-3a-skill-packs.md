# Tija Fase 3a — Skill packs, limpieza de comentarios y workflows: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar los skill packs atribuidos (`antislop-code`, `antislop-copywriting`, `thermo-review`, `thermo-quality`), `tija clean comments` con la semántica de flags de Biome, PRs sin atribuciones de agentes y workflows compuestos con `tija run`.

**Architecture:**
- `internal/assets` embebe los packs adaptados, sus `LICENSE` originales y el workflow `pre-pr`. `init` los instala con el mecanismo que ya existe de `defaultsVersion` + hash, así que no hay que construir nada nuevo para eso.
- `internal/skills` lee los packs, su frontmatter de origen y si fueron editados.
- `internal/comments` es puro (sin git ni subprocesos):
  - lexer JS/TS/JSX;
  - extracción y protección de comentarios;
  - reglas deterministas;
  - planificador de ediciones;
  - verificador de tokens;
  - prompt y parser de la clasificación.
- `internal/quality` suma `BiomeDiagnostics`, la segunda red de verificación.
- `engine.CleanComments` orquesta todo. El agente solo clasifica y Tija escribe.
- `internal/prrules/attribution.go` limpia las atribuciones de agentes; `PreparePR` lo usa antes y después de llamar al agente.
- `internal/workflows` parsea y valida `.tija/workflows/*.yaml`. `cli/run.go` ejecuta cada paso en proceso, con un árbol de Cobra nuevo por paso.

**Tech Stack:** Go 1.27, Cobra, Bubble Tea v2 / Lip Gloss v2, yaml.v3. `golang.org/x/sync` pasa de indirecta a directa (por `errgroup`). **No se agregan dependencias nuevas**: esbuild está descartado por peso (+6.7 MB, medido) y tree-sitter por cgo.

**Spec:**
- `docs/superpowers/specs/2026-10-29-tija-phase-3a-skill-packs-design.md` (completo).
- `docs/superpowers/specs/2026-10-22-tija-phases-design.md`: secciones 2 a 4, en especial las restricciones 10 y 11.
- Material de origen (leer antes de las tasks 1 y 6): https://github.com/miqdadbadjuber/anti-slop en el commit `3005235d030b4793146ad9afe8d01045efd90e1d` y https://github.com/cursor/plugins/tree/main/thermos en el commit `1ba91fc25c0977741dd0e267849e6234b037ed0a`.

## Global Constraints

- Todo lo de las fases 1 a 3 sigue vigente:
  - módulo `github.com/Ps3udoDev/tija-cli`;
  - UI en inglés, contenido generado en `content.language` (default `es`);
  - paleta C + `Green` solo para `✓`; símbolos `✓ ✗ ! >`;
  - exit codes `0` ok, `1` validación o cambios pendientes, `2` uso o entorno;
  - sin TTY, o con `--json`, `--no-color` o `NO_COLOR`: salida plana;
  - Windows es plataforma de primera clase.
- **Atribución (restricción 11).** Todo pack de terceros tiene:
  - frontmatter `source.{project,author,repo,path,commit,version?,license,copyright}` y `adapted.{by,date,changes}`;
  - la nota visible *"Adaptado de `<proyecto>` de `<autor>` (`<repo>`, licencia MIT). Los cambios respecto del original están listados en el frontmatter."* como primera línea del cuerpo;
  - su `LICENSE` original al lado, sin cambios.

  `THIRD_PARTY_NOTICES.md` va en la raíz y en `archives.files` de `.goreleaser.yaml`.
- **El agente nunca escribe archivos.** `internal/agents/provider.go` no cambia sus flags (claude `-p`, codex `--sandbox read-only`). El agente devuelve JSON; Tija edita.
- **Escritura en tres niveles (restricción 10):**
  - sin flag: reporta, exit 1 si hay cambios seguros pendientes (con `--unsafe`, cualquier cambio pendiente);
  - `--write`: aplica los seguros;
  - `--write --unsafe`: aplica también los inseguros.

  `--write` es el consentimiento y no pregunta nada más. Los archivos no trackeados por git se saltan con `!` salvo `--force`.
- **Nunca se escribe un archivo que no pasó la verificación de tokens.** Si Biome reporta una regresión después de escribir, se restaura el contenido original.
- **No hay `--remove-all`** (decisión del usuario del 2026-10-29).
- **Extensiones que procesa `clean comments`:** `.js .jsx .mjs .cjs .ts .tsx .mts .cts`. JSX se habilita en `.js .jsx .tsx` (en los repos React del autor hay JSX dentro de `.js`) y nunca en `.ts .mts .cts`, donde `<T>` es un genérico.
- **Lotes del agente:** a lo sumo 150 comentarios por lote; `--concurrency` default 2; tier `agents.tasks.comments.tier` (default `fast`); timeout por lote 3 min, configurable con `--agent-timeout`.
- **Archivos que escribe la fase fuera del código del usuario:** `.tija/session/comments.json` y `.tija/tmp/comments/`, ambos ya ignorados por `init`. En un repo sin manifest se usan los defaults en memoria y no se escribe la sesión.
- **Menú de la TUI:**
  - `Pull Request`, `Clean comments`, `Check`, `Tests`, `Workflows`, `Doctor`, `Status`, `Init`;
  - `Clean comments` aparece solo si el proyecto es JS/TS, y `Workflows` solo si hay al menos un workflow válido;
  - `Review` no aparece.

## Review Focus

1. **Un comentario dentro de un string, de un template literal o de texto JSX** (`"http://x"`, `` `a // b ${c /* d */}` ``, `<p>// no</p>`, `/[/*]/` como regex). El lexer no debe tomarlo como comentario. Si duda, salta el archivo; nunca adivina. Lo cubre la Task 4, en `TestLexTrickyFixtures` y `TestLexUncertainSkips`.
2. **Un archivo con CRLF, con BOM o sin salto de línea final.** Después de `--write`, los finales de línea y el BOM quedan como estaban y solo desaparecen los comentarios. Task 5, `TestApplyPreservesCRLFAndBOM`.
3. **Un `{/* … */}` dentro de JSX en su propia línea.** Al borrarlo, el árbol JSX debe quedar equivalente (reglas de whitespace de JSX) y no puede quedar un `{}` vacío. Task 5, `TestApplyJSXCommentLine` y `TestVerifyJSXWhitespaceNormalized`.
4. **El agente responde con prosa alrededor del JSON, ids inventados o faltantes, o un `rewrite` de `//` con un salto de línea.** Se tolera el texto alrededor; el id desconocido se descarta; el faltante queda como `keep`; el `rewrite` inválido pasa a `keep` con un aviso. Task 7, `TestParseClassificationsTolerant`.
5. **Una línea legítima del PR que menciona a una herramienta** ("Integramos Codex de impuestos", `Co-Authored-By: Ana <ana@empresa.com>`). No se borra; solo se borran las atribuciones ancladas a línea completa. Task 9, `TestStripAttributionNegatives`.

---

## Estructura de archivos

```text
THIRD_PARTY_NOTICES.md                         (nuevo)
.goreleaser.yaml                               (+ THIRD_PARTY_NOTICES.md en archives.files)
internal/assets/
  assets.go                                    (+ packs, LICENSEs y workflow pre-pr; DefaultsVersion "2")
  attribution_test.go                          (nuevo: restricción 11)
  defaults/skills/{antislop-code,antislop-copywriting,thermo-review,thermo-quality}/{SKILL.md,LICENSE}
  defaults/workflows/pre-pr.yaml
internal/skills/      skills.go, skills_test.go
internal/manifest/    manifest.go (Skills, Comments, Content.AgentAttribution), defaults y Validate
internal/comments/
  lexer.go            tokens JS/TS/JSX
  extract.go          Comment, Extract, Protected
  rules.go            reglas deterministas
  edit.go             PlanEdit, Apply
  verify.go           CodeTokens, VerifyTokens
  classify.go         Classification, Safety, BuildPrompt, ParseClassifications, Batches
  glob.go             Match con ** y {a,b}
  testdata/           fixtures .js/.jsx/.ts/.tsx
internal/quality/     biome_diag.go (BiomeDiagnostics, Regressions)
internal/prrules/     attribution.go
internal/engine/      clean_comments.go, pr_prompt.go, pr_prepare.go, doctor.go
internal/workflows/   workflows.go
internal/cli/         clean.go, skills.go, run.go, root.go
internal/render/      comments.go, skills.go, workflow.go (+ golden en testdata/)
internal/tui/         menu.go, comments_view.go, workflow_view.go, model.go
```

---

### Task 1: Material de terceros adaptado, licencias y notices

**Files:**
- Create: `internal/assets/defaults/skills/antislop-code/SKILL.md`, `.../antislop-code/LICENSE`
- Create: `internal/assets/defaults/skills/antislop-copywriting/SKILL.md`, `.../LICENSE`
- Create: `internal/assets/defaults/skills/thermo-review/SKILL.md`, `.../LICENSE`
- Create: `internal/assets/defaults/skills/thermo-quality/SKILL.md`, `.../LICENSE`
- Create: `internal/assets/defaults/workflows/pre-pr.yaml`
- Create: `THIRD_PARTY_NOTICES.md`
- Modify: `internal/assets/assets.go`, `.goreleaser.yaml:21-25`
- Test: `internal/assets/attribution_test.go`

**Interfaces:**
- Produces:
  - `assets.DefaultsVersion = "2"`;
  - `assets.SkillsDir = ".tija/skills"`;
  - `assets.PrePRWorkflowPath = ".tija/workflows/pre-pr.yaml"`;
  - `assets.Files()` incluye ahora `.tija/skills/<pack>/SKILL.md`, `.tija/skills/<pack>/LICENSE` y `pre-pr.yaml`. Se mantiene `WorkflowsKeep`.
  - `assets.Skill(name string) ([]byte, bool)` devuelve el `SKILL.md` embebido (lo usa `PreparePR` cuando el repo no está inicializado).

- [ ] **Step 1: Escribir el test de atribución.** `TestThirdPartyPacksAreAttributed` recorre cada `defaults/skills/*/SKILL.md` y, si el frontmatter tiene `source`, exige:
  - `source.repo`, `source.commit` (40 hex), `source.license == "MIT"` y `source.copyright` no vacío;
  - `adapted.changes` con al menos 1 entrada;
  - que la primera línea no vacía del cuerpo empiece con `> Adaptado de`;
  - que exista `LICENSE` al lado, con `source.copyright` como subcadena.

  `TestNoticesListEveryPack` exige que `THIRD_PARTY_NOTICES.md` (leído vía `../../THIRD_PARTY_NOTICES.md`) contenga cada `source.repo` y el texto `MIT License`. `TestGoreleaserShipsNotices` exige que `.goreleaser.yaml` contenga `THIRD_PARTY_NOTICES.md`.
- [ ] **Step 2: Correr** `go test ./internal/assets/ -run 'ThirdParty|Notices|Goreleaser'`. Esperado: FAIL (no existen los archivos).
- [ ] **Step 3: Escribir los packs adaptados.**
  - **`antislop-code`.** Origen `skills/antislop-code/SKILL.md`, `version: 3.2.24`, copyright `Copyright (c) 2026 Miqdad Badjuber (antislop)`. Contenido según la sección 4.2 del spec:
    - se conservan los patrones con su forma Tell/Why/Fix;
    - se reemplazan las citas `R-XX` por el principio *un comentario se queda solo si explica algo que el código no muestra*;
    - se quita `allowed-tools`;
    - se agregan las clases del spec 6.2, con estos nombres exactos: `separator, labeled-separator, end-marker, empty-line, restates-obvious, workflow-narration, empty-label, decorative-emoji, line-by-line, vague-todo, signature-echo, commented-out-code, over-explained, business-rule, keep-other`;
    - se agrega una sección de ejemplos en español y el contrato JSON de la sección 6.3.
  - **`antislop-copywriting`.** Extracto según la sección 4.3 del spec, con la sección propia de Tija *"No agent attribution"* marcada en `adapted.changes`.
  - **`thermo-review` y `thermo-quality`.** Copyright `Copyright (c) 2026 Cursor`. Adaptados según la sección 4.4 del spec: sin `disable-model-invocation`, `subagent_type` ni `run_in_background`; con el paso de comentarios del PR generalizado a `gh`; y con la salida mapeada a archivo, línea, severidad Critical/High/Medium/Low, regla y solución.
  - **Cada `LICENSE`** es una copia literal del `LICENSE` del repo de origen en el commit fijado.
- [ ] **Step 4: Escribir `pre-pr.yaml`** con los tres pasos del spec 7.1 (`clean comments --write`, `check`, `pr prepare`). **Escribir `THIRD_PARTY_NOTICES.md`:** una sección por proyecto (anti-slop y Thermos) con autor, URL, commit, archivos tomados y el texto completo de la licencia, más una nota sobre el skill `thermos` (orquestación reimplementada en Go en la Fase 4, no copiada). **Agregar `THIRD_PARTY_NOTICES.md`** a `archives.files` en `.goreleaser.yaml`.
- [ ] **Step 5: Embeber** en `assets.go` con `//go:embed defaults/skills defaults/workflows` (un `embed.FS`). `Files()` recorre el FS y mapea `defaults/skills/...` → `.tija/skills/...` y `defaults/workflows/...` → `.tija/workflows/...`. Subir `DefaultsVersion` a `"2"`.
- [ ] **Step 6: Correr** `go test ./internal/assets/ ./internal/engine/ -run 'ThirdParty|Notices|Goreleaser|Init'`. Esperado: PASS. Los tests de `init` existentes que comparan la lista de archivos creados se actualizan para incluir los nuevos.
- [ ] **Step 7: Commit** `feat(assets): add attributed skill packs and third-party notices`.

### Task 2: Manifest — `skills`, `comments` y `content.agentAttribution`

**Files:**
- Modify: `internal/manifest/manifest.go`, `internal/manifest/patch.go` (si `init` necesita proponer las claves nuevas)
- Test: `internal/manifest/manifest_test.go`, `internal/engine/init_test.go`

**Interfaces:**
- Produces:
  ```go
  type SkillRef struct{ Path string `yaml:"path"` }
  type Comments struct {
      Include []string `yaml:"include"`
      Exclude []string `yaml:"exclude"`
      Protect []string `yaml:"protect"` // regex
  }
  type AgentAttribution struct {
      Strip    *bool    `yaml:"strip"`    // nil = true
      Patterns []string `yaml:"patterns"` // regex extra
  }
  // Manifest gana: Skills map[string]SkillRef `yaml:"skills"`; Comments Comments `yaml:"comments"`
  // Content gana: AgentAttribution AgentAttribution `yaml:"agentAttribution"`
  func (a AgentAttribution) StripEnabled() bool
  func DefaultComments() Comments
  ```
  - `DefaultComments()` = include `["src/**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts}"]`, exclude `["**/*.min.js","dist/**","build/**","**/*.d.ts","node_modules/**"]`.
  - `FromDetection` llena `Skills` con los 4 packs, llena `Comments` con `DefaultComments()` solo si el proyecto es `react-vite` o tiene `package.json`, y agrega `agents.tasks.comments: {tier: fast}`.

- [ ] **Step 1: Tests.**
  - `TestValidateRejectsBadRegex`: un `comments.protect: ["("]` o un `content.agentAttribution.patterns: ["["]` produce un error que nombra la clave.
  - `TestFromDetectionAddsSkillsAndComments`: con una detección `react-vite` aparecen los 4 packs, `DefaultComments()` y `tasks.comments.tier == "fast"`; con una detección `dotnet-api`, `Comments` queda vacío.
  - `TestStripEnabledDefault`: nil → true.
  - En `init_test.go`, `TestInitInstallsSkillPacks`: en un repo fresco aparecen los 8 archivos de `.tija/skills/` y `pre-pr.yaml`, y `tija.installed` tiene sus hashes.
  - `TestReinitOffersNewKeys`: con un manifest de Fase 3 (sin `skills`), re-ejecutar `init` propone agregar `skills`, `comments` y `agents.tasks.comments` como acción confirmable y no toca el resto.
  - `TestReinitKeepsEditedPack`: un `SKILL.md` editado queda igual y aparece como `differs from Tija default v2 (edited?)`.
- [ ] **Step 2: Correr** `go test ./internal/manifest/ ./internal/engine/ -run 'Regex|FromDetection|StripEnabled|SkillPacks|Reinit'`. Esperado: FAIL.
- [ ] **Step 3: Implementar** los tipos, los defaults, la validación de regex en `Validate()` y la propuesta de claves faltantes en `PlanInit`, reutilizando el camino que hoy propone campos detectados que faltan.
- [ ] **Step 4: Correr los mismos tests.** Esperado: PASS. Además `go test ./...` en verde.
- [ ] **Step 5: Commit** `feat(manifest): add skills, comments and agent attribution settings`.

### Task 3: `internal/skills`, `tija skills list|show` y chequeo en `doctor`

**Files:**
- Create: `internal/skills/skills.go`, `internal/skills/skills_test.go`, `internal/cli/skills.go`, `internal/render/skills.go`
- Modify: `internal/cli/root.go:117-127`, `internal/engine/doctor.go`
- Test: `internal/skills/skills_test.go`, `internal/render/skills_test.go` (golden), `internal/engine/doctor_test.go`

**Interfaces:**
- Produces:
  ```go
  type Source struct{ Project, Author, Repo, Path, Commit, Version, License, Copyright string }
  type Pack struct {
      Name, Path, Description string
      Origin string  // "builtin" | "third-party" | "repo"
      Source *Source // nil si no es de terceros
      Edited bool    // hash != tija.installed[Path]
      Problems []string // p. ej. "missing LICENSE", "missing source.commit"
      Body string    // cuerpo sin frontmatter
  }
  func List(root string, m manifest.Manifest) ([]Pack, error) // ordenados por nombre
  func Load(root string, m manifest.Manifest, name string) (Pack, error)
  func Parse(data []byte) (Pack, error)
  ```
  - `Origin`: `third-party` si tiene `source`; `builtin` si su ruta está en `assets.Files()` y no tiene `source`; `repo` en el resto de casos.
  - Los packs que están en `.tija/skills/*/SKILL.md` pero no en el manifest también se listan, con origen `repo`.

- [ ] **Step 1: Tests.**
  - `TestParseFrontmatter`: lee `source` y `adapted` y saca el cuerpo sin frontmatter.
  - `TestListMarksEdited`.
  - `TestListReportsMissingLicense`: un pack de terceros sin `LICENSE` tiene el problema `missing LICENSE`.
  - `TestDoctorWarnsBrokenAttribution`: doctor emite `!` con el texto `skill <name>: missing LICENSE`.
  - Golden de `render.Skills` en texto plano y JSON: columnas `NAME  ORIGIN  AUTHOR  LICENSE  COMMIT  EDITED`, commit abreviado a 7 caracteres.
- [ ] **Step 2: Correr** `go test ./internal/skills/ ./internal/render/ ./internal/engine/ -run 'Frontmatter|Edited|MissingLicense|Doctor.*Attribution|Skills'`. Esperado: FAIL.
- [ ] **Step 3: Implementar**:
  - `skills.go`: frontmatter delimitado por `---`, parseado con yaml.v3.
  - Comandos `tija skills list` y `tija skills show <name>`. `show` imprime el origen como encabezado y luego el cuerpo; un nombre desconocido es exit 2.
  - Chequeo en `doctor`.
- [ ] **Step 4: Correr los tests.** Esperado: PASS.
- [ ] **Step 5: Commit** `feat(skills): list skill packs with origin and license`.

### Task 4: Lexer JS/TS/JSX

**Files:**
- Create: `internal/comments/lexer.go`, `internal/comments/lexer_test.go`, `internal/comments/testdata/*.{js,jsx,ts,tsx}`

**Interfaces:**
- Produces:
  ```go
  type Lang struct{ TS, JSX bool }
  func LangFor(path string) (Lang, bool) // false si la extensión no se procesa
  type TokenKind int // Code, String, Template, Regex, JSXText, LineComment, BlockComment
  type Token struct {
      Kind       TokenKind
      Start, End int  // offsets en bytes
      Text       string
      InJSXChild bool // comentario de bloque que es todo el contenido de un {…} hijo de JSX
      OuterStart, OuterEnd int // span de las llaves cuando InJSXChild
  }
  var ErrUncertain = errors.New("could not tokenize with certainty")
  func Lex(src []byte, lang Lang) ([]Token, error)
  ```

**Reglas que el lexer debe implementar** (no las determinan los tests por sí solos):
- **Regex frente a división:** `/` empieza una regex si el token significativo anterior no existe, o es un punctuator distinto de `)`, `]` y `}`, o es una de las keywords `return typeof case do else in instanceof new void delete throw yield await of`. Después de `}` se trata como división; los casos raros los cubre la verificación.
- **Template literals** con `${…}` anidados mediante una pila de profundidad de llaves. Dentro de `${}` el lexer vuelve al modo código, que puede contener comentarios, strings y templates.
- **JSX** (solo con `lang.JSX`): `<` abre un elemento si en esa posición puede empezar una expresión (la misma regla que para la regex) y lo sigue un identificador, `>` o `/`. Dentro de los hijos, el texto hasta `<` o `{` es `JSXText`, donde `//` y `/*` no son comentarios. Un `{` hijo cuyo único contenido significativo es un comentario de bloque marca ese comentario con `InJSXChild` y su span exterior.
- **Strings y templates sin cerrar, JSX desbalanceado o EOF dentro de un comentario de bloque:** `ErrUncertain`.
- BOM inicial y shebang `#!` en la línea 1 son `Code`.

- [x] **Step 1: Fixtures y tests.**
  - `TestLexTrickyFixtures` (tabla). Cada fixture tiene un `.want` con los comentarios esperados (texto exacto). Casos:
    - `"http://x"`;
    - `'/* no */'`;
    - `` `a // b ${c /* d */} e` `` (solo `/* d */` es comentario);
    - `const r = /[/*]/g; // real` (solo `// real`);
    - `a = b / c; // real`;
    - `x = y / z / w`;
    - JSX `<p>// no {/* sí */}</p>` (solo `/* sí */`, marcado `InJSXChild`);
    - `.ts` con `foo<T>(x) // c` (sin JSX);
    - comentarios contiguos `//` en varias líneas;
    - CRLF.
  - `TestLexUncertainSkips`: un string sin cerrar, `<div>` sin cerrar en `.jsx` y `/*` sin cerrar devuelven `ErrUncertain`.
  - `TestLexConcatenationRoundTrip`: concatenar `src[t.Start:t.End]` de todos los tokens más el whitespace entre ellos reproduce el archivo byte a byte.
- [x] **Step 2: Correr** `go test ./internal/comments/ -run Lex`. Esperado: FAIL.
- [x] **Step 3: Implementar `Lex`** según las reglas de arriba.
- [x] **Step 4: Correr** `go test ./internal/comments/ -run Lex`. Esperado: PASS. Además, una corrida ad hoc sobre los 136 archivos de `claims` (ruta absoluta en `TIJA_LEX_CORPUS`, test con `t.Skip` si la variable no está): 0 `ErrUncertain`, o la lista de archivos para revisar a mano antes de seguir.
- [x] **Step 5: Commit** `feat(comments): add JS/TS/JSX lexer`.

### Task 5: Extracción, protección, reglas deterministas, ediciones y verificación

**Files:**
- Create: `internal/comments/extract.go`, `rules.go`, `edit.go`, `verify.go`, `glob.go` y sus `_test.go`

**Interfaces:**
- Consumes: `Lex`, `Token`, `Lang` (Task 4).
- Produces:
  ```go
  type Comment struct {
      ID         string // "<ruta slash relativa>#<offset>"
      Path       string
      Kind       string // "line" | "block" | "jsx"
      Line, Col  int
      Text       string
      FullLine   bool   // solo whitespace antes y después en su línea
      Before, After []string // ±3 líneas de contexto
      Start, End int    // span del comentario (para "jsx", el de las llaves)
  }
  func Extract(path string, src []byte, lang Lang) ([]Comment, error)
  func Protected(c Comment, ext string, extra []*regexp.Regexp) bool
  type Verdict struct{ Class string; Action string } // Action "remove"
  func Rule(c Comment, src []byte) (Verdict, bool)
  type Edit struct{ Start, End int; Replacement string }
  func PlanEdit(src []byte, c Comment, action, text string) Edit // action: "remove" | "rewrite"
  func Apply(src []byte, edits []Edit) ([]byte, error)            // error si hay solapamiento
  func CodeTokens(src []byte, lang Lang) ([]string, error)
  func VerifyTokens(before, after []byte, lang Lang) error
  func Match(pattern, path string) bool // ** y {a,b}; path con /
  ```

- [x] **Step 1: Tests.**
  - **`TestProtectedBuiltins`**, una fila por patrón del spec 6.2:
    - `biome-ignore`, `eslint-disable-next-line`, `prettier-ignore`, `@ts-expect-error`, `istanbul ignore`, `c8 ignore`, `v8 ignore`, `@vite-ignore`;
    - `webpackChunkName: "x"`, `#__PURE__`, `@jsxImportSource`, `@flow`;
    - `/*! lic */`, `@license`, `@preserve`, `@copyright`;
    - `/// <reference path="x" />`;
    - JSDoc con `@type {X}` en `.js` (protegido) frente al mismo bloque en `.ts` (no protegido);
    - shebang;
    - un patrón extra de `comments.protect`.
  - **`TestRuleDeterministic`**:
    - protegidos:
      - `// ---------------------------------------------------------------------------` → `separator`;
      - `{/* ---------- */}` → `separator`;
      - `} // end if` → `end-marker`;
      - `//` solo, con líneas vecinas sin comentarios → `empty-line`;
    - no deben coincidir:
      - `{/* ---------- Fila 1 ---------- */}` (tiene letras, va al agente);
      - `//` dentro de un bloque de comentarios contiguos;
      - `// -- ver issue 12 --`.
  - **`TestApplyRemovesFullLine`**: borra la línea completa y colapsa dos líneas vacías seguidas en una.
  - **`TestApplyTrailingComment`**: `x = 1; // hi` → `x = 1;`, sin espacios al final.
  - **`TestApplyJSXCommentLine`**: borra la línea con `{/* x */}`. Inline (`<a/>{/* x */}<b/>`) borra la expresión con sus llaves.
  - **`TestApplyRewriteKeepsIndentAndKind`**.
  - **`TestApplyPreservesCRLFAndBOM`**.
  - **`TestApplyRejectsOverlap`**.
  - **`TestVerifyDetectsCodeChange`**: un after con un `;` de menos devuelve un error que nombra la línea.
  - **`TestVerifyJSXWhitespaceNormalized`**: quitar `{/* x */}` en su propia línea entre dos hijos JSX es equivalente. `JSXText` se normaliza con la regla de React: se recortan las líneas, se descartan las vacías y se unen con un espacio.
  - **`TestMatchGlob`**: `src/**/*.{js,jsx}` contra `src/a/b/c.jsx` (sí) y `src/a.ts` (no); `dist/**` contra `dist/x/y.js` (sí).
- [x] **Step 2: Correr** `go test ./internal/comments/ -run 'Protected|Rule|Apply|Verify|Match'`. Esperado: FAIL.
- [x] **Step 3: Implementar.** `CodeTokens` usa `Lex`, descarta comentarios, devuelve el texto de cada token y normaliza `JSXText`. `VerifyTokens` compara los dos slices y reporta el primer índice distinto con su línea en `after`.
- [x] **Step 4: Correr los tests.** Esperado: PASS.
- [x] **Step 5: Commit** `feat(comments): extract, protect, plan and verify comment edits`.

### Task 6: Biome como segunda red

**Files:**
- Create: `internal/quality/biome_diag.go`, `internal/quality/biome_diag_test.go`, `internal/quality/testdata/biome-github.txt`

**Interfaces:**
- Produces:
  ```go
  type DiagCounts map[string]map[string]int // ruta slash relativa → categoría → cantidad
  func BiomeDiagnostics(ctx context.Context, env sys.Env, root string, files []string) (DiagCounts, error)
  func ParseBiomeGithub(out, root string) (DiagCounts, bool) // false si no reconoce ninguna línea y out no está vacío
  type Regression struct{ Path, Category string; Before, After int }
  func Regressions(before, after DiagCounts) []Regression
  ```
- Invocación: el binario se resuelve con `LookupBin(root, "biome", runtime.GOOS, env.LookPath)` y se corre `biome check --reporter=github --max-diagnostics=none --colors=off <files…>` con `env.Run`. El exit code de Biome se ignora (es 1 cuando hay errores); el resultado sale del stdout.

- [x] **Step 1: Verificar en el frontend real** (`node_modules/.bin/biome`, v2.4.4) que `--max-diagnostics=none` es aceptado. Si no lo es, usar `--max-diagnostics=10000` y anotarlo en un comentario de una línea junto a la invocación.
- [x] **Step 2: Tests.**
  - `TestParseBiomeGithub`: usa como fixture la salida real capturada (líneas `::warning title=lint/correctness/noUnusedVariables,file=src\pages\(features)\claims\...,line=112,...::...`). Las rutas con `\` se normalizan a slash relativa.
  - `TestParseBiomeGithubUnrecognized`: texto arbitrario devuelve `ok=false`.
  - `TestRegressions`: una categoría nueva y un aumento cuentan como regresión; una disminución no.
  - `TestBiomeDiagnosticsArgs`: con un `env.Run` falso, verifica los argumentos y que los archivos se pasan como rutas relativas.
- [x] **Step 3: Correr** `go test ./internal/quality/ -run 'Biome|Regressions'`. Esperado: FAIL. **Implementar.** **Correr de nuevo.** Esperado: PASS.
- [x] **Step 4: Commit** `feat(quality): read per-file Biome diagnostics for post-edit checks`.

### Task 7: Clasificación con el agente — prompt, parser, seguridad y lotes

**Files:**
- Create: `internal/comments/classify.go`, `internal/comments/classify_test.go`

**Interfaces:**
- Consumes: `Comment` (Task 5).
- Produces:
  ```go
  type Classification struct {
      ID, Action, Class, Confidence, Reason, Text string // Action: keep|remove|rewrite
  }
  type Safety string // "safe" | "unsafe" | "none"
  func SafetyOf(c Classification) Safety
  func BuildPrompt(pack, language string, batch []Comment) string
  func ParseClassifications(out string, batch []Comment) (map[string]Classification, []string, error) // []string = avisos
  func Batches(cs []Comment, max int) [][]Comment // agrupa por archivo y no parte un archivo salvo que supere max
  ```
- `SafetyOf`:
  - `keep` → `none`;
  - `rewrite` → `unsafe`;
  - `remove` con `Confidence != "high"` → `unsafe`;
  - `remove` con clase en {`restates-obvious`, `workflow-narration`, `empty-label`, `labeled-separator`, `decorative-emoji`, `end-marker`, `line-by-line`} → `safe`;
  - cualquier otro `remove` → `unsafe`.

- [ ] **Step 1: Tests.**
  - **`TestBuildPromptContents`**: el prompt incluye el cuerpo del pack, el idioma (`Spanish` para `es`), cada id con su texto y su contexto, y la instrucción literal `Answer with a JSON array only.`. `languageName` de `engine/pr_prompt.go` se mueve a `manifest.LanguageName(code string) string` para que lo usen `engine` y `comments` sin ciclo de imports.
  - **`TestParseClassificationsTolerant`**:
    - JSON dentro de un fence ```` ```json ```` con prosa antes y después → OK;
    - id desconocido → descartado con aviso;
    - id faltante → `keep`;
    - `rewrite` de `line` con `\n` → `keep` con aviso;
    - `rewrite` de `block` con `*/` → `keep` con aviso;
    - JSON inválido → error.
  - **`TestSafetyOf`**: tabla con todas las combinaciones.
  - **`TestBatches`**: 3 archivos de 60 comentarios con max 150 dan 2 lotes; un archivo de 400 da 3 lotes.
- [ ] **Step 2: Correr** `go test ./internal/comments/ -run 'Prompt|Classifications|Safety|Batches'`. Esperado: FAIL. **Implementar.** **Correr de nuevo.** Esperado: PASS.
- [ ] **Step 3: Commit** `feat(comments): classify comments through the agent as JSON`.

### Task 8: `engine.CleanComments`

**Files:**
- Create: `internal/engine/clean_comments.go`, `internal/engine/clean_comments_test.go`, `internal/engine/testdata/comments-repo/` (fixture JSX con separadores, `biome-ignore`, reglas de negocio y código comentado)
- Modify: `go.mod` (`golang.org/x/sync` directa)

**Interfaces:**
- Consumes: Tasks 2, 4 a 7; `agents.Generate`, `agents.Request`; resolución de target de `engine/target.go`; `gitx.Repo.DiffStat`.
- Produces:
  ```go
  type CleanRequest struct {
      Paths []string; All, Write, Unsafe, Diff, NoAgent, Force bool
      Concurrency int; AgentTimeout time.Duration
  }
  type CleanChange struct {
      Line int; Action, Class, Safety, Source string // Source: "rule" | "agent"
      Text string `json:",omitempty"`
  }
  type CleanFile struct {
      Path, Status string // pending | written | unchanged | skipped | failed | restored
      Reason string `json:",omitempty"`
      Changes []CleanChange; Diff string `json:",omitempty"`
  }
  type CleanSummary struct{ Comments, Protected, Keep, Safe, Unsafe, Unclassified, SkippedFiles, FailedFiles int }
  type CleanReport struct {
      Scope struct{ Mode string; Paths []string; Files int } // mode: diff | paths | all
      Agent struct{ Provider, Tier string; Batches int }
      Summary CleanSummary; Files []CleanFile; Written bool; Warnings []string
      PostCheck string // "biome" | "skipped: <motivo>"
  }
  func (r CleanReport) ExitCode() int
  type CleanEvent struct{ Type string; Done, Total int; Path string } // extract | classify | verify | write
  func CleanComments(ctx context.Context, dir string, env sys.Env, req CleanRequest, onEvent func(CleanEvent)) (CleanReport, error)
  ```
- **Candidatos:** `git ls-files -z --cached --others --exclude-standard -- <paths>`, filtrados por `LangFor`, `comments.include` y `comments.exclude`. El conjunto `--others` marca los archivos no trackeados.
  - Modo `diff`: sin rutas ni `--all`, se toman los archivos de `DiffStat(target)` más los no trackeados.
  - `ExitCode`:
    - `2` si no es un repo git;
    - en modo reporte, `1` si `Safe > 0` (o `Safe+Unsafe > 0` con `Unsafe`);
    - en modo escritura, `1` si `FailedFiles > 0`;
    - `0` en el resto de casos.
- **Escritura:**
  1. Se verifican los tokens.
  2. Se escribe de forma atómica (temporal en el mismo directorio + rename).
  3. Si `quality.lint` o `quality.format` es `biome`, se corre `BiomeDiagnostics` sobre los archivos tocados antes y después.
  4. Ante una regresión, se restaura el original desde memoria y el archivo queda `restored` con su razón.
  5. Por último se guarda `.tija/session/comments.json` (el `CleanReport` más `Commit` = HEAD).

- [ ] **Step 1: Tests**, con `testutil.NewRepo` sobre el fixture y el `fakeAgentEnv` de `pr_prepare_test.go` (extraído a un helper compartido del paquete):
  - `TestCleanReportOnlyDoesNotWrite`: el contenido no cambia y el exit es 1.
  - `TestCleanWriteAppliesSafeOnly`: los separadores y los `labeled-separator` de confianza alta desaparecen; `commented-out-code` y los `rewrite` siguen; `biome-ignore` y el comentario de regla de negocio siguen.
  - `TestCleanWriteUnsafeAppliesAll`: el `rewrite` se aplica y `biome-ignore` sigue.
  - `TestCleanNoAgentRulesOnly`: no hay llamadas a `RunInput` y `Unclassified > 0`.
  - `TestCleanNoAgentInstalledFallsBack`: con `LookPath` vacío, mismo resultado y un aviso `! no agent CLI found — only deterministic rules ran`.
  - `TestCleanSkipsUntrackedUnlessForce`.
  - `TestCleanVerificationFailureNotWritten`: el engine llama al planificador a través de `var planEdit = comments.PlanEdit`. El test lo reemplaza por uno que además borra un `;`, y el archivo debe quedar `failed` y sin escribir.
  - `TestCleanBiomeRegressionRestores`: con un `env.Run` falso que devuelve un `useExhaustiveDependencies` nuevo después, el archivo queda `restored` y su contenido es el original.
  - `TestCleanDefaultScopeIsDiff`: solo procesa los archivos cambiados contra el target.
  - `TestCleanSavesSession`.
- [ ] **Step 2: Correr** `go test ./internal/engine/ -run Clean`. Esperado: FAIL.
- [ ] **Step 3: Implementar.** La clasificación por lotes va en un `errgroup` con un límite de `Concurrency`. Si un lote falla (timeout, JSON inválido dos veces), sus comentarios quedan `keep` con un aviso. Los errores del agente nunca abortan el comando.
- [ ] **Step 4: Correr** `go test ./internal/engine/ -run Clean` y `go test ./...`. Esperado: PASS.
- [ ] **Step 5: Commit** `feat(engine): clean comment slop with verified edits`.

### Task 9: PRs sin atribuciones de agentes y con `antislop-copywriting`

**Files:**
- Create: `internal/prrules/attribution.go`, `internal/prrules/attribution_test.go`
- Modify: `internal/engine/pr_prompt.go`, `internal/engine/pr_prepare.go:227-300`, `internal/engine/pr_context.go` (cargar el pack)
- Test: `internal/engine/pr_prepare_test.go`

**Interfaces:**
- Produces:
  ```go
  var BuiltinAttributionPatterns []string
  func AttributionPatterns(extra []string) ([]*regexp.Regexp, error)
  func StripAttribution(text string, pats []*regexp.Regexp) (string, int) // líneas quitadas
  func CheckAttribution(body string, pats []*regexp.Regexp) []Problem    // Problem.Rule == "agent-attribution"
  ```
- Patrones built-in (case-insensitive, anclados a línea completa con `(?m)^…$`):
  ```text
  ^\s*🤖.*$
  ^\s*\**\s*generated (with|by) \[?(claude( code)?|codex|openai codex|chatgpt|gemini( cli)?|copilot|cursor|antigravity)\b.*$
  ^\s*co-authored-by:.*(claude|codex|chatgpt|gemini|copilot|cursor|noreply@anthropic\.com|noreply@openai\.com).*$
  ^\s*(made|created|written) (with|by) \[?(claude( code)?|codex|chatgpt|gemini|copilot|cursor)\b.*$
  ```
  Después de quitar líneas se colapsan las líneas vacías sobrantes al final del texto.

- [ ] **Step 1: Tests.**
  - `TestStripAttributionPositives`: `🤖 Generated with [Claude Code](https://claude.com/claude-code)`, `Generated with Codex`, `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`, `Made with Cursor`.
  - `TestStripAttributionNegatives`: `Integramos Codex de impuestos en el módulo`, `Co-Authored-By: Ana <ana@empresa.com>`, `Se generó el reporte con la librería claude-parser` (no está anclada al inicio con la forma esperada).
  - `TestCheckAttribution`: un body con una atribución da 1 `Problem`.
  - En el engine:
    - `TestPRPromptStripsCommitAttribution`: los commits del fixture traen `Co-Authored-By: Claude …` y el prompt no lo contiene.
    - `TestPRPromptIncludesCopywritingPack`: el prompt contiene la sección `# Writing style` con el cuerpo del pack.
    - `TestPreparePRStripsAgentFooter`: el agente falso devuelve un cuerpo que termina en `🤖 Generated with …` y el borrador guardado no lo tiene.
    - `TestPreparePRStripDisabled`: con `strip: false`, no se quita nada y `CheckAttribution` emite un aviso `!` sin cambiar el exit.
- [ ] **Step 2: Correr** `go test ./internal/prrules/ ./internal/engine/ -run 'Attribution|Copywriting|Footer|StripDisabled'`. Esperado: FAIL.
- [ ] **Step 3: Implementar.**
  - En `PRContext`, los cuerpos de los commits se limpian al construir el contexto, si `StripEnabled()`.
  - `BuildPRPrompt` agrega `# Writing style` con el pack: se lee de `.tija/skills/antislop-copywriting/SKILL.md` si existe y, si no, de `assets.Skill`.
  - En `draftPR`, `StripAttribution` se aplica al body antes de `Normalize`.
  - `CheckAttribution` corre sobre el borrador final y produce avisos, no errores de validación.
- [ ] **Step 4: Correr los tests.** Esperado: PASS. Además `go test ./...`.
- [ ] **Step 5: Commit** `feat(pr): keep agent attribution and AI phrasing out of pull requests`.

### Task 10: CLI y render de `tija clean comments`

**Files:**
- Create: `internal/cli/clean.go`, `internal/render/comments.go`, `internal/render/testdata/comments-*.golden`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/commands_test.go`, `internal/render/comments_test.go`

**Interfaces:**
- Consumes: `engine.CleanComments`, `CleanRequest`, `CleanReport` (Task 8).
- Produces: `newCleanCmd(deps Deps) *cobra.Command`, con el subcomando `comments [paths...]` y los flags `--write --unsafe --diff --all --no-agent --force --concurrency int(2) --agent-timeout duration(3m)`. Además `render.CleanComments(p Printer, r engine.CleanReport)` y `render.CleanProgress(p Printer, ev engine.CleanEvent)`.

- [ ] **Step 1: Tests.**
  - Golden en texto plano:
    - encabezado con alcance, archivos y agente;
    - una línea por archivo con cambios: `✓ written`, `! pending`, `✗ failed`, `! restored`;
    - debajo de cada archivo, las líneas `  L120  remove  separator  safe  rule`;
    - resumen `Comments 1420 · protected 9 · keep 610 · safe 640 · unsafe 161`;
    - pista final según el modo: `run with --write to apply safe changes`, o `--write --unsafe` si solo quedan inseguros.
  - Golden JSON con la forma del spec 6.6.
  - `TestCleanCmdFlags`: `--unsafe` sin `--write` es válido (solo amplía el reporte); un `--concurrency 0` es exit 2.
  - `TestCleanCmdExitCodes` con el engine sobre el fixture.
- [ ] **Step 2: Correr** `go test ./internal/cli/ ./internal/render/ -run 'Clean|Comments'`. Esperado: FAIL. **Implementar.** **Correr de nuevo.** Esperado: PASS.
- [ ] **Step 3: Commit** `feat(cli): add tija clean comments`.

### Task 11: Workflows compuestos — `tija run`

**Files:**
- Create: `internal/workflows/workflows.go`, `internal/workflows/workflows_test.go`, `internal/cli/run.go`, `internal/render/workflow.go`
- Modify: `internal/cli/root.go`, `internal/engine/doctor.go`, `internal/sys/editor.go` (exportar `splitCommand` como `SplitArgs`)
- Test: `internal/cli/run_test.go`, `internal/engine/doctor_test.go`

**Interfaces:**
- Produces:
  ```go
  // internal/workflows
  type Step struct{ Run string `yaml:"run"`; ContinueOnError bool `yaml:"continueOnError"` }
  type Workflow struct{ Name, Description, Path string; Steps []Step }
  var Allowed = []string{"check", "lint", "format", "test", "status", "doctor",
      "clean comments", "pr prepare", "pr create"}
  func Load(root string) ([]Workflow, []error) // todos los .yaml de .tija/workflows/
  func Validate(w Workflow) []error           // nombre vacío, sin pasos, comando fuera de Allowed, "run" anidado
  func Args(s Step) ([]string, error)         // sys.SplitArgs
  // internal/cli
  type StepResult struct{ Run string; Exit int; Output json.RawMessage `json:",omitempty"` }
  type WorkflowReport struct{ Name string; Steps []StepResult; Exit int }
  func RunWorkflow(ctx context.Context, deps Deps, in io.Reader, w workflows.Workflow, jsonOut, yes bool, out io.Writer, onStep func(i int, r StepResult)) WorkflowReport
  ```
- **Ejecución:** cada paso crea un árbol nuevo con `NewRoot(deps, in)` y le hace `SetArgs(args + ["--json"] si jsonOut + ["--yes"] si yes)`, sin subprocesos.
  - Con `--json`, el stdout del paso se captura en un buffer y va en `Output`. Sin `--json`, el stdout del paso va directo al del usuario, precedido por `> <run>`.
  - Para en el primer `Exit != 0` salvo `ContinueOnError`.
  - El exit del workflow es el mayor exit de los pasos (2 > 1 > 0).
- **Comandos:** `tija run <name>` y `tija run --list` (`--json`). Un nombre desconocido o un workflow inválido es exit 2.
- **`doctor`:** emite `✗ workflow <name>: <error>` por cada workflow inválido y `✓ workflows: N valid` si todos lo son.

- [ ] **Step 1: Tests.**
  - `TestValidateRejectsShellAndNested`: `run: npm test` → `unknown command "npm"`; `run: run pre-pr` → `nested run is not allowed`.
  - `TestAllowedResolvesInCobra` (en `cli`): cada entrada de `Allowed` resuelve con `root.Find` a un comando distinto del root. Así `Allowed` y Cobra no se desincronizan.
  - `TestRunStopsAtFirstFailure`: un workflow `status --strict` (que falla en la rama del fixture) → `check`; el segundo paso no corre y el exit es 1.
  - `TestRunContinueOnError`.
  - `TestRunJSONEmbedsStepOutput`: la salida es un único documento JSON válido con 2 pasos.
  - `TestRunList`.
  - `TestDoctorReportsInvalidWorkflow`.
- [ ] **Step 2: Correr** `go test ./internal/workflows/ ./internal/cli/ ./internal/engine/ -run 'Workflow|Run|Allowed|Doctor.*Workflow'`. Esperado: FAIL. **Implementar.** **Correr de nuevo.** Esperado: PASS.
- [ ] **Step 3: Commit** `feat(workflows): run composed Tija commands with tija run`.

### Task 12: TUI — `Clean comments` y `Workflows`

**Files:**
- Create: `internal/tui/comments_view.go`, `internal/tui/workflow_view.go`
- Modify: `internal/tui/menu.go`, `internal/tui/model.go`, `internal/cli/root.go` (inyectar el runner de workflows en la TUI)
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `engine.CleanComments` (Task 8); `tui.WorkflowRunner`, inyectado desde `cli` en las opciones de la TUI. La TUI no importa `cli`, así que los tipos espejo viven en `tui`: `type WorkflowStep struct{ Run string; Exit int }` y `type WorkflowRunner func(name string, onStep func(i int, s WorkflowStep)) (exit int)`. `cli` adapta `RunWorkflow` a esa firma.
- Produces:
  - `MenuItems(initialized, hasJS, hasWorkflows bool) []MenuItem` con `itemComments{Key:"comments", Title:"Clean comments", Description:"Remove AI-slop comments, keep the useful ones"}` e `itemWorkflows{Key:"workflows", Title:"Workflows", Description:"Run a composed sequence of commands"}`;
  - `hasJS` = `project.type == "react-vite"`, o detección con `package.json`.

- [ ] **Step 1: Tests.**
  - `TestMenuOrderWithNewItems`: el orden es el de Global Constraints y los ítems se ocultan sin JS o sin workflows.
  - `TestCommentsViewKeys`: el reporte inicial corre en modo reporte; `w` corre con `Write`; `u` muestra la confirmación `Apply unsafe changes too? (y/n)` y solo con `y` corre con `Write+Unsafe`; `q` vuelve al menú sin escribir.
  - `TestCommentsViewMascotStates`: `scanning` durante `extract` y `thinking` durante `classify`.
  - `TestWorkflowViewProgress`: muestra `✓`/`✗` por paso a medida que llegan.
- [ ] **Step 2: Correr** `go test ./internal/tui/ -run 'Menu|CommentsView|WorkflowView'`. Esperado: FAIL. **Implementar.** **Correr de nuevo.** Esperado: PASS.
- [ ] **Step 3: Commit** `feat(tui): add clean comments and workflows screens`.

### Task 13: Verificación de salida de la fase

**Files:**
- Modify: `README.md` (comandos nuevos, sección "Third-party material" que apunta a `THIRD_PARTY_NOTICES.md`), `docs/superpowers/handoff/HANDOFF.md` (sección de la Fase 3a)

- [ ] **Step 1: Suite completa.** Correr `go vet ./...`, `go test ./...` y `golangci-lint run` si está configurado. Esperado: 0 issues y PASS en todos los paquetes.
- [ ] **Step 2: Binario.** `go build -ldflags="-s -w" -o tija.exe ./cmd/tija`. Esperado: tamaño ≤ 7.5 MB (hoy 6.8 MB). Si lo supera, revisar qué dependencia entró.
- [ ] **Step 3: Validación real en el frontend de Team4soft** (Windows), en una rama nueva creada desde `dev` (`chore/clean-claims-comments`), nunca sobre `dev`:
  1. `tija init` propone las claves nuevas; se aceptan.
  2. `tija clean comments "src/pages/(features)/claims"`. Esperado: exit 1, 0 archivos `skipped` (o la lista justificada) y `Safe ≥ 54`.
  3. `--diff`: revisar a mano una muestra de 10 archivos.
  4. `--write`. Esperado:
     - 0 `failed`;
     - `grep -rc "biome-ignore"` en `claims` sigue en 3;
     - siguen los comentarios *"El limite se evalua acumulado…"* y *"El jefe autoriza a nivel siniestro…"*;
     - `grep -rE "^\s*// -{10,}"` da 0.
  5. `tija check` y el build del frontend (`bun run build` o el script del repo). Esperado: el mismo resultado que antes del cambio.
  6. `--write --unsafe` en otra copia de la rama: se repiten los checks del punto 4 y del punto 5.
  7. `tija clean comments --no-agent` con `PATH` sin CLIs de agente: solo reglas deterministas, sin error.
- [ ] **Step 4: PR sin atribuciones.** En una rama de Tija cuyos commits tengan `Co-Authored-By: Claude …`, correr `tija pr prepare`. Esperado: el borrador no contiene `🤖`, `Generated with` ni `Co-Authored-By` de agentes.
- [ ] **Step 5: Atribución.** `tija skills list` muestra los 4 packs con autor, repo, commit y licencia. `goreleaser release --snapshot --clean`: el `.tar.gz` contiene `THIRD_PARTY_NOTICES.md`.
- [ ] **Step 6: Workflow.** `tija run pre-pr` y `tija run pre-pr --json | jq .` en el frontend. Esperado: 3 pasos, JSON válido; si `check` falla, `pr prepare` no corre.
- [ ] **Step 7: Commit** `docs: document phase 3a commands and third-party material`.
