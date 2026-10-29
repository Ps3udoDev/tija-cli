# Tija — Fase 3a: Skill packs, limpieza de comentarios y workflows compuestos

> Fecha: 2026-10-29
> Estado: borrador para aprobación
> Spec padre: `2026-10-22-tija-phases-design.md` (sección 5, Fase 3a)
> Material de terceros evaluado:
> - **anti-slop**, de Miqdad Badjuber — https://github.com/miqdadbadjuber/anti-slop — MIT — commit `3005235d030b4793146ad9afe8d01045efd90e1d` (2026-10-29), versión `3.2.24`
> - **Thermos**, de Cursor — https://github.com/cursor/plugins/tree/main/thermos — MIT — commit `1ba91fc25c0977741dd0e267849e6234b037ed0a` (2026-10-28)

---

## 1. Intención y porqué

Las Fases 1 a 3 dejaron a Tija leyendo el repo, corriendo la calidad y armando PRs. La Fase 4 (Review) necesita **criterio**: rúbricas que le digan al agente qué buscar. Ese criterio ya existe, bien escrito y con licencia MIT, en dos proyectos públicos: anti-slop (comentarios y textos con cara de IA) y Thermos (revisión profunda de correctitud y mantenibilidad).

La Fase 3a construye la infraestructura para **traer ese material al repo de forma versionada, atribuida y adaptada**, y la estrena con un caso real y acotado: limpiar los comentarios "slop" de un frontend.

**El dolor concreto:** en el frontend de Team4soft, `src/pages/(features)/claims` tiene 136 archivos JS/JSX (unas 31 000 líneas) y 88 de ellos tienen comentarios: unas 1230 líneas `//` y 190 bloques `/* */`. Entre ellos hay:

- **Ruido:** 54 separadores `// ----------…`, banners JSX como `{/* ---------- Fila 1 ---------- */}`, etiquetas vacías como `{/* fila 5 */}`.
- **Documentación de negocio que debe quedarse:** *"El limite se evalua acumulado a nivel siniestro y por tipo de estimacion"*, *"El jefe autoriza a nivel siniestro: se asigna a todos los…"*.
- **Directivas que no son comentarios:** tres `// biome-ignore lint/correctness/useExhaustiveDependencies: …`. Si se borran, Biome empieza a fallar.

Borrar "todos los comentarios" no necesita IA (es un regex) y sería dañino. Lo que hace falta es distinguir unos de otros, y eso sí es trabajo de criterio. Esa es la regla de anti-slop: *"a filter, not a ban"*.

**Por qué va antes de la Fase 4:** los skill packs, el modo de escritura seguro y los workflows compuestos son infraestructura que `tija review` va a reutilizar. Construirla con un caso pequeño y verificable (comentarios) es más barato que estrenarla con la revisión completa.

## 2. Objetivos, no-objetivos y anti-objetivos

**Objetivos**
- **Skill packs:** rúbricas en Markdown, versionadas en `.tija/skills/`, embebidas en el binario e instaladas por `init`. Cada pack declara de dónde viene, con qué licencia y qué se cambió.
- **`tija clean comments`:** detecta y, si se pide, elimina o reescribe comentarios slop en JS/TS/JSX. Tiene la misma semántica de flags que Biome: sin flag solo reporta, `--write` aplica lo seguro y `--write --unsafe` aplica también lo dudoso.
- **Workflows compuestos:** secuencias de comandos de Tija declaradas en `.tija/workflows/*.yaml` y ejecutadas con `tija run <nombre>`.
- Dejar en el repo los packs de Thermos, adaptados y atribuidos, listos para que la Fase 4 los use.
- **Copy de los PR sin cara de IA:** el pack `antislop-copywriting` (extracto adaptado) entra al prompt de `pr prepare`, y Tija elimina de forma determinista las líneas de atribución de agentes (`🤖 Generated with Claude Code`, `Generated with Codex`, `Co-Authored-By: Claude …`, etc.) de lo que recibe el agente y de lo que devuelve.

**No-objetivos (en esta fase)**
- Ejecutar `tija review` (es la Fase 4; aquí solo se preparan sus packs).
- Limpiar comentarios en C#, Python u otros lenguajes. Solo JS, JSX, TS y TSX.
- Eliminar código comentado de forma automática sin `--unsafe`.
- Un "borrar todos los comentarios". Ver la sección 6.4.
- Que el agente edite archivos. El agente sigue en modo solo lectura.

**Anti-objetivos (sería un fracaso aunque funcione)**
- **Presentar material de terceros como propio.** Todo pack o fragmento que venga de otro proyecto lleva su origen, su licencia, el commit de donde se tomó y una nota de qué se adaptó. Esto se verifica en tests.
- Que `clean comments` cambie **cualquier cosa que no sea un comentario**: código, identificadores, imports, strings, indentación del código.
- Que se borre una directiva funcional (`biome-ignore`, `eslint-disable`, `@ts-expect-error`, etc.) o una anotación de tipos JSDoc en un archivo `.js`.
- Que se pierda documentación de negocio por defecto.
- Que un workflow compuesto pueda ejecutar comandos arbitrarios de shell.

## 3. Decisiones tomadas

### Decididas por el usuario
| Tema | Decisión |
|---|---|
| Material de terceros | Se toma solo lo que necesitamos, **siempre atribuido a su autor**, nunca presentado como propio, y **adaptado** a Tija (sin mecánicas de otra herramienta) |
| Origen del material | `antislop-code` (anti-slop) para comentarios; `thermo-nuclear-review`, `thermo-nuclear-code-quality-review` y `thermos` (Cursor) para la Fase 4 |
| Edición | Tija puede editar archivos de código, igual que Biome con `--write`: el flag es el consentimiento |
| Orden | Fase 3a antes de la Fase 4 |
| `--remove-all` (2026-10-29) | Fuera de alcance, para no romper la lógica existente (directivas, tipos JSDoc, documentación de negocio) |
| Segunda verificación (2026-10-29) | esbuild descartado por peso (medido: +6.7 MB sobre un binario de 6.8 MB). Se usan las herramientas oficiales de cada stack: **Biome** en proyectos JS/TS/React y **`dotnet format`** en .NET |
| `antislop-copywriting` (2026-10-29) | Entra en la fase, con foco en que los PR no lleven atribuciones de agentes (`🤖 Generated with …`) ni vocabulario de IA |

### Decididas por Claude (revisables)
| Tema | Decisión | Motivo |
|---|---|---|
| Quién edita | Tija aplica los cambios; el agente solo **clasifica** comentarios ya extraídos | La garantía de "solo comentarios" queda en el código de Tija, no en la obediencia del modelo. El provider sigue en solo lectura (`internal/agents/provider.go`) |
| Extracción | Lexer propio en Go para JS/TS/JSX, sin cgo | tree-sitter requiere cgo y complica el cross-compile de GoReleaser |
| Verificación | Dos redes: (1) comparar el flujo de tokens sin comentarios antes y después, en Go y siempre; (2) la herramienta oficial del stack (Biome en JS/TS) sobre los archivos tocados, comparando diagnósticos antes y después | La primera garantiza "solo comentarios"; la segunda detecta lo que el lexer no ve, como una directiva `biome-ignore` perdida |
| Formato de Biome | Se usa `--reporter=github` (una línea por diagnóstico) | Biome 2.4.4 emite rutas de Windows con `\` sin escapar en `--reporter=json`, lo que da JSON inválido |
| Alcance por defecto | Sin rutas: archivos cambiados contra `pullRequest.target`. Con rutas: esas rutas. `--all`: todo el repo según `comments.include` | Mismo criterio que la Fase 4 ("solo lo del diff") y costo de tokens acotado |
| Tier del agente | `fast` para clasificar comentarios | Es clasificación con contexto local, no razonamiento profundo |
| Ubicación de packs | `.tija/skills/<pack>/SKILL.md` + `LICENSE` del autor original al lado | Formato reconocible y la licencia viaja con el material |
| Nombre del comando | `tija clean comments` | Deja `clean` como namespace para otras limpiezas futuras |
| Packs de Thermos | Se adaptan e instalan en 3a; se **usan** en la Fase 4 | Así la Fase 4 solo implementa el flujo de review |
| `antislop-copywriting` | Se toma un extracto: las reglas que aplican a un PR en español | La skill está escrita para copy de marketing en inglés; solo una parte es aplicable |

## 4. Material de terceros: qué se toma, de quién y cómo se adapta

### 4.1 Reglas de atribución (aplican a todo pack, hoy y en el futuro)

1. **Frontmatter de origen obligatorio** en cada `SKILL.md` que venga de terceros:
   ```yaml
   ---
   name: antislop-code
   description: Classify code comments as slop or valuable. Never touches code.
   source:
     project: anti-slop
     author: Miqdad Badjuber
     repo: https://github.com/miqdadbadjuber/anti-slop
     path: skills/antislop-code/SKILL.md
     commit: 3005235d030b4793146ad9afe8d01045efd90e1d
     version: 3.2.24
     license: MIT
     copyright: "Copyright (c) 2026 Miqdad Badjuber (antislop)"
   adapted:
     by: Tija
     date: 2026-10-29
     changes:
       - Removed references to the core antislop.md rule numbers (R-XX); the comment principle is inlined.
       - Added Tija's output contract (JSON classification per comment id).
       - Added the protected-directives list and the safe/unsafe classes.
   ---
   ```
2. **Nota visible al inicio del cuerpo:** *"Adaptado de `<proyecto>` de `<autor>` (`<repo>`, licencia MIT). Los cambios respecto del original están listados en el frontmatter."*
3. **La licencia original se copia** como `.tija/skills/<pack>/LICENSE`, sin modificar.
4. **`THIRD_PARTY_NOTICES.md`** en la raíz del repo de Tija, con cada proyecto, autor, URL, licencia y texto completo de la licencia. GoReleaser lo agrega a `archives.files` para que viaje en cada release.
5. **`tija skills list`** muestra cada pack con su origen y licencia. `tija doctor` reporta `!` si un pack de terceros perdió su frontmatter de origen o su `LICENSE`.
6. **Tests:** un test recorre los packs embebidos y falla si alguno de terceros no tiene `source.repo`, `source.commit`, `source.license` y su `LICENSE`.
7. **Actualizar desde upstream** es manual y explícito: se cambia `source.commit`, se revisa el diff del original y se re-adapta. Tija nunca descarga packs de internet en tiempo de ejecución.

### 4.2 anti-slop → pack `antislop-code` (se usa en esta fase)

**Se toma:** `skills/antislop-code/SKILL.md` (128 líneas): los patrones *Decorative Separators*, *Restating the Obvious*, *Workflow Narration*, *Empty Labels*, *Vague Placeholders*, *Signature Echo*, *Decorative Emoji*, *End Markers*, *The Over-Explained Comment*, *Line-by-Line Narration*, *Stiff or Loud Wording*, la lista *Not a Ban (preserve these)* y el *Code Comment Checklist*.

**No se toma:** el core `antislop.md` (745 líneas, centrado en diseño de UI) ni las skills `antislop-ui`, `antislop-human` y `antislop-layoutmobile`. El skill de comentarios cita R-31 del core; R-31 es una regla de diseño ("every decision must have a reason"). En el pack adaptado se reemplaza la cita por el principio aplicado a comentarios: *un comentario se queda solo si explica algo que el código no muestra*.

**Se adapta:**
- Se quita `allowed-tools: Read Write Edit Glob Grep`: en Tija el agente no edita.
- Se agrega el contrato de salida JSON de la sección 6.3.
- Se agrega la clasificación segura/insegura de la sección 6.2 y la lista de directivas protegidas.
- Se agrega una sección sobre comentarios en español: el criterio no cambia, pero los ejemplos del original están en inglés.

### 4.3 anti-slop → pack `antislop-copywriting` (se usa en `pr prepare`)

**Se toma**, de `skills/antislop-copywriting/SKILL.md`, solo lo que aplica a la descripción de un PR técnico en español:
- *Never invent facts*: ya es una regla de la Fase 3; se refuerza con la redacción del original.
- *Empty AI Vocabulary*: la idea, con equivalentes en español (*potenciar, robusto, sin fisuras, de última generación, llevar al siguiente nivel, transformar, crucial*).
- Los patrones de bullets con emoji, el hype y las MAYÚSCULAS para enfatizar.
- *Do not over-sterilize*: mantener la voz técnica del equipo.

**No se toma:** headlines, CTAs, propuestas de valor ni los patrones de landing page (es copy de marketing).

**Se agrega (propio de Tija, marcado como tal en el pack):** *no agregar atribuciones de herramientas ni firmas de agentes*. No van líneas como `🤖 Generated with [Claude Code](…)`, `Generated with Codex`, `Co-Authored-By: Claude …`, `Made with Cursor`, ni menciones a que el texto lo escribió una IA.

**Además, de forma determinista** (no depende de que el modelo obedezca):
1. Antes de armar el prompt, `pr prepare` quita de los cuerpos de los commits las líneas de atribución de agentes. Hoy entran tal cual (`internal/engine/pr_prompt.go`), y los trailers `Co-Authored-By: Claude …` que dejan los propios agentes son el origen más probable de que el PR los repita.
2. Después de recibir la respuesta, se eliminan del cuerpo las líneas que coincidan con los patrones de atribución.
3. `internal/prrules` suma un chequeo `agent-attribution` que reporta si alguna sobrevive (por ejemplo, porque el usuario la agregó editando el borrador). En ese caso Tija avisa con `!`, pero no la borra: lo que el usuario escribe a mano es suyo.

Los co-autores humanos (`Co-Authored-By: Ana <ana@empresa.com>`) no se tocan en los commits. Los patrones de atribución son configurables en `content.agentAttribution.patterns` y desactivables con `content.agentAttribution.strip: false`.

### 4.4 Thermos → packs para la Fase 4 (se adaptan e instalan en esta fase)

**Se toma:**
- `skills/thermo-nuclear-review/SKILL.md` → pack `thermo-review`: auditoría de bugs, cambios que rompen funcionalidad, seguridad, cambios que afectan al flujo local de los devs y fugas de feature flags, limitada al diff.
- `skills/thermo-nuclear-code-quality-review/SKILL.md` → pack `thermo-quality`: revisión estricta de mantenibilidad (simplificaciones "code judo", regla de las 1000 líneas, condicionales sueltos, límites de tipos, capa correcta).
- `skills/thermos/SKILL.md` → no se copia como pack. Es pura orquestación de Cursor (subagentes en paralelo y síntesis). La idea se reimplementa en Go en la Fase 4 y se atribuye en `THIRD_PARTY_NOTICES.md` y en el spec de esa fase.

**No se toma:** `agents/*.md` (envoltorios de subagentes de Cursor), `.cursor-plugin/plugin.json` ni el logo.

**Se adapta:**
- Se quitan `disable-model-invocation`, `subagent_type` y `run_in_background`.
- El paso "revisar comentarios de BugBot con gh/glab" se generaliza a "si hay un PR abierto, leer sus comentarios con `gh` **después** de la auditoría propia", conservando la regla original de hacerlo al final.
- La salida se mapea al esquema de findings de la Fase 4: archivo, línea, severidad Critical/High/Medium/Low, regla y solución.
- Se mantiene la regla de no inflar severidades (*Over-reporting Guidelines*).

## 5. Skill packs (infraestructura)

### 5.1 Estructura
```text
internal/assets/defaults/skills/
  antislop-code/SKILL.md
  antislop-code/LICENSE
  antislop-copywriting/SKILL.md
  antislop-copywriting/LICENSE
  thermo-review/SKILL.md
  thermo-review/LICENSE
  thermo-quality/SKILL.md
  thermo-quality/LICENSE
```
`init` los copia a `.tija/skills/` usando el mecanismo que ya existe: `tija.defaultsVersion` + hash sha256 por archivo en `tija.installed`. Un pack editado por el usuario nunca se pisa; un pack sin editar se actualiza cuando sube `defaultsVersion`.

Un repo puede agregar sus propios packs en `.tija/skills/<nombre>/SKILL.md`. Los packs propios no necesitan `source`.

### 5.2 Manifest
```yaml
skills:
  antislop-code:        { path: .tija/skills/antislop-code/SKILL.md }
  antislop-copywriting: { path: .tija/skills/antislop-copywriting/SKILL.md }
  thermo-review:        { path: .tija/skills/thermo-review/SKILL.md }
  thermo-quality:       { path: .tija/skills/thermo-quality/SKILL.md }
content:
  language: es
  agentAttribution:
    strip: true          # quita atribuciones de agentes del prompt y del cuerpo del PR
    patterns: []         # regex extra, además de los built-in
comments:
  include: ["src/**/*.{js,jsx,ts,tsx}"]
  exclude: ["**/*.min.js", "dist/**", "build/**", "**/*.d.ts"]
  protect: []          # patrones extra (regex) que nunca se tocan, además de los built-in
agents:
  tasks:
    comments: { tier: fast }
```
Re-ejecutar `init` en un repo de Fase 3 propone agregar estas claves como diff confirmable, igual que hoy con los campos nuevos.

### 5.3 Comandos
- `tija skills list` (`--json`): nombre, ruta, origen (`builtin` / `third-party` / `repo`), autor, licencia, commit y si fue editado localmente.
- `tija skills show <nombre>`: imprime el pack con su encabezado de origen.

## 6. `tija clean comments`

### 6.1 Modos y flags: igual que Biome

Biome separa tres niveles: `biome check` solo reporta; `biome check --write` aplica los fixes **seguros**; `biome check --write --unsafe` aplica también los **inseguros**, que pueden cambiar el significado. Tija ya usa esa semántica en `tija lint --fix` (`internal/quality/adapters.go` invoca `biome check --write .`). `clean comments` sigue el mismo contrato para que no haya nada nuevo que aprender:

| Invocación | Qué hace | Escribe | Exit |
|---|---|---|---|
| `tija clean comments [rutas]` | Reporta cada comentario slop con archivo, línea, clase y acción propuesta | No | `1` si hay cambios seguros pendientes, `0` si no |
| `… --diff` | Igual, más el diff unificado de lo que haría `--write` | No | igual |
| `… --write` | Aplica solo los cambios **seguros** | Sí | `0`; `1` si algún archivo falló la verificación |
| `… --write --unsafe` | Aplica también los cambios **inseguros** | Sí | igual |
| `… --unsafe` sin `--write` | El reporte y el `--diff` incluyen también los inseguros | No | `1` si hay cualquier cambio pendiente |

Flags comunes:
- `--all`: todo el repo según `comments.include`/`exclude`. Sin rutas ni `--all`, el alcance son los archivos cambiados contra `pullRequest.target`.
- `--no-agent`: solo reglas deterministas (sección 6.2). Es también el comportamiento automático si no hay ningún CLI de agente; el reporte dice cuántos comentarios quedaron sin clasificar.
- `--json`: salida estable (sección 6.6).
- `--concurrency N`: lotes en paralelo hacia el agente (default 2).

**`--write` es el consentimiento**, como en Biome y en `tija lint --fix`: no hay prompt adicional, así que funciona igual en CI, en agentes y por MCP. La restricción transversal 4 del spec padre ("no pisar nada sin confirmación") se precisa así: aplica a los archivos de configuración e infraestructura que Tija crea o modifica por iniciativa propia (`.gitignore`, templates, workflows de `.github/`). Las ediciones de código fuente que el usuario pide con un flag explícito de escritura quedan cubiertas por ese flag. Como red de seguridad, si un archivo a escribir no está trackeado por git (no hay forma de deshacer), Tija lo reporta con `!` y lo salta salvo `--force`.

### 6.2 Clasificación: protegido, seguro, inseguro

Cada comentario extraído termina en una de cuatro categorías:

**Protegido (nunca se toca, ni con `--unsafe`).** Lo decide Tija de forma determinista, antes de llamar al agente, y esos comentarios ni siquiera se envían:
- Directivas de herramientas: `biome-ignore`, `eslint-disable*`, `eslint-enable`, `prettier-ignore`, `@ts-ignore`, `@ts-expect-error`, `@ts-nocheck`, `@ts-check`, `istanbul ignore`, `c8 ignore`, `v8 ignore`, `@vite-ignore`, `webpackChunkName` y otros comentarios mágicos de webpack, `#__PURE__`, `@__PURE__`, `@jsx`, `@jsxImportSource`, `@flow`.
- Licencias: `/*! … */`, `@license`, `@preserve`, `@copyright`.
- JSDoc con tipos en archivos `.js`/`.jsx` (`@type`, `@typedef`, `@param {…}`, `@returns {…}`, `@template`, `@satisfies`): TypeScript con `checkJs` y el editor los usan como tipos.
- Triple-slash `/// <reference …>`.
- Shebang `#!` en la primera línea.
- Todo lo que coincida con `comments.protect` del manifest.

**Seguro (se aplica con `--write`).**
- *Determinista, sin agente:* separadores puros (solo caracteres repetidos `-=*#_~/`, sin letras); separadores JSX vacíos `{/* ------ */}`; marcadores de cierre (`// end if`, `// end of function`, `// End processOrder`) en la línea de una llave de cierre; líneas `//` vacías que no están dentro de un bloque de comentarios.
- *Del agente, con `confidence: high`:* `remove` de las clases `restates-obvious`, `workflow-narration`, `empty-label`, `labeled-separator` (por ejemplo `{/* ---------- Fila 1 ---------- */}`), `decorative-emoji`, `end-marker`, `line-by-line`.

**Inseguro (solo con `--write --unsafe`).**
- Cualquier `rewrite` (cambiar el texto pierde matices aunque el código no cambie).
- Cualquier `remove` con `confidence: low`.
- `vague-todo`: borrar un TODO pierde una intención, aunque sea vaga.
- `signature-echo`: JSDoc que solo repite la firma (en `.ts`/`.tsx`; en `.js` las etiquetas de tipo son protegidas).
- `commented-out-code`: código comentado. No está en anti-slop; es una clase propia de Tija y se marca como tal en el pack.
- `over-explained`: recortar un comentario largo a una línea es siempre un `rewrite`.

**Se queda (`keep`).** Todo lo que cae en la lista *Not a Ban* de anti-slop: lógica de negocio e intención, decisiones de arquitectura, seguridad, trade-offs de rendimiento, concurrencia, protocolos, contratos de API, workarounds, casos borde y supuestos.

### 6.3 Flujo

1. **Resolver alcance:** rutas → archivos JS/TS/JSX/TSX filtrados por `comments.include`/`exclude` y `.gitignore`.
2. **Extraer (determinista):** el lexer de Go recorre cada archivo y produce una lista de comentarios con `id` estable (`<ruta>#<offset>`), tipo (`line`, `block`, `jsx`), línea, columna, texto, si ocupa la línea completa o va al final de código, y ±3 líneas de contexto.
   - El lexer maneja strings, template literals con `${}` anidados, la ambigüedad regex/división, texto JSX (donde `//` no es comentario, como en `<a>http://…</a>`) y comentarios JSX `{/* */}`.
   - Si el lexer no puede tokenizar un archivo con certeza, ese archivo se **salta** y se reporta como `! skipped: could not tokenize`. Nunca se adivina.
3. **Proteger (determinista):** se separan los comentarios protegidos (sección 6.2).
4. **Reglas deterministas:** se marcan los seguros deterministas.
5. **Clasificar con el agente:** el resto se envía en lotes (por archivo; archivos chicos se agrupan hasta ~150 comentarios por lote). El prompt incluye el pack `antislop-code`, `content.language` y la lista de comentarios con su contexto. El agente responde **solo** JSON:
   ```json
   [
     {"id": "src/pages/(features)/claims/Form.jsx#4812",
      "action": "remove", "class": "labeled-separator",
      "confidence": "high", "reason": "Banner label that names a layout row"},
     {"id": "src/pages/(features)/claims/useLimits.js#930",
      "action": "keep", "class": "business-rule",
      "confidence": "high", "reason": "Explains cumulative limit per claim"},
     {"id": "…#2210", "action": "rewrite", "class": "over-explained",
      "confidence": "high", "text": "Default estable para no crear un arreglo en cada render."}
   ]
   ```
   Se valida con el parser de `internal/agents/parse.go`: los ids desconocidos se descartan, a un id sin respuesta se le aplica `keep`, un `rewrite` de comentario `//` debe ser de una línea y un `rewrite` de bloque no puede contener `*/`. Si el JSON es inválido se reintenta una vez; si vuelve a fallar, ese lote queda como `keep` y se reporta.
6. **Planificar ediciones:** borrar un comentario de línea completa borra la línea; si quedan dos líneas vacías seguidas, se colapsan en una. Borrar un comentario al final de una línea de código borra el comentario y el espacio previo. Borrar `{/* … */}` en su propia línea borra la línea; si va en línea con otros elementos, se borra la expresión completa con sus llaves. Un `rewrite` conserva la indentación y el tipo de comentario.
7. **Verificar (determinista, obligatorio):** por cada archivo, se tokeniza el antes y el después **descartando comentarios y espacios**, y los dos flujos de tokens deben ser idénticos. Además, si hay un `rewrite`, se verifica que el resultado siga siendo un comentario válido. Si un archivo no pasa, **no se escribe** y se reporta `✗ verification failed`.
   - **Segunda red: la herramienta oficial del stack.** Si el manifest tiene `quality.lint` o `quality.format` con provider `biome`, Tija corre `biome check --reporter=github <archivos tocados>` antes de escribir (línea base) y después, y compara los diagnósticos por archivo y categoría (no por línea, porque las líneas se corren al borrar). Si un archivo tiene una categoría nueva o más diagnósticos de una categoría que antes (por ejemplo, un `useExhaustiveDependencies` que antes estaba silenciado por un `biome-ignore`, o un error de parseo), se restaura el original y se reporta `✗ <archivo>: biome reports <categoría> after the edit`. Si el proyecto no usa Biome, esta red se omite y el reporte lo dice con `!`.
   - Para .NET, la segunda red es `dotnet format --verify-no-changes --include <archivos>` más `dotnet build`. Entra cuando se agregue el soporte de C# (fuera de esta fase); la interfaz `PostCheck` se diseña pensando en ambos.
   - **esbuild queda descartado:** agrega unos 6.7 MB al binario (medido el 2026-10-29 con esbuild v0.28.2: un "hello world" pasa de 1.66 MB a 8.34 MB, y Tija hoy pesa 6.8 MB).
8. **Escribir** (solo con `--write`): escritura atómica por archivo (archivo temporal + rename), conservando finales de línea (LF/CRLF) y BOM.
9. **Sesión:** el resultado se guarda en `.tija/session/` (archivos, conteos por clase, commit), para que `pr prepare` pueda mencionarlo en `## Cambios realizados` sin inventar.

### 6.4 Por qué no hay "borrar todos los comentarios"

**Decidido (2026-10-29): fuera de alcance.** Un `--remove-all` borraría directivas funcionales (rompe Biome), anotaciones de tipos JSDoc y documentación de negocio, y podría cambiar el comportamiento del código existente. Lo más agresivo que ofrece la Fase 3a es `--write --unsafe`, que sigue respetando los protegidos y lo que el agente marca como `keep`. Si un repo necesita un criterio más estricto, el camino es su propio pack en `.tija/skills/`.

### 6.5 TUI
- Opción de menú `Clean comments` (solo si el repo tiene archivos JS/TS).
- Mascota en `scanning` durante la extracción y en `thinking` mientras clasifica el agente.
- Vista de resultados: conteo por clase (seguros / inseguros / protegidos / se quedan) y lista navegable con el diff de cada archivo. Acciones: `w` aplica los seguros, `u` aplica también los inseguros (pide confirmación en pantalla, porque la TUI es interactiva), `q` sale sin escribir.

### 6.6 Salida JSON
```json
{
  "scope": {"mode": "paths", "paths": ["src/pages/(features)/claims"], "files": 136},
  "agent": {"provider": "claude", "tier": "fast", "batches": 12},
  "summary": {"comments": 1420, "protected": 9, "keep": 610,
              "safe": 640, "unsafe": 161, "skippedFiles": 0, "failedFiles": 0},
  "files": [
    {"path": "…/Form.jsx", "status": "written",
     "changes": [{"line": 120, "action": "remove", "class": "separator",
                  "safety": "safe", "source": "rule"}]}
  ],
  "written": true
}
```
Los números del ejemplo son ilustrativos. El formato se documenta y se cubre con golden tests.

## 7. Workflows compuestos

### 7.1 Formato
```yaml
# .tija/workflows/pre-pr.yaml
name: pre-pr
description: Clean comment slop, run quality checks and prepare the PR
steps:
  - run: clean comments --write
  - run: check
  - run: pr prepare
```
- Cada `run` es un **comando de Tija** con sus flags, nunca un comando de shell. Se valida contra el registro de comandos de Cobra al cargar el workflow; uno desconocido es un error de validación (exit 2) y `doctor` lo reporta.
- Cada paso llama directamente al engine, sin subprocesos, respetando el "un motor, tres interfaces".
- Por defecto se detiene en el primer paso que devuelve exit 1 o 2. `continueOnError: true` a nivel de paso permite seguir.
- `--json` y `--yes` del comando `run` se propagan a todos los pasos.

### 7.2 Comandos
- `tija run <nombre>`: ejecuta el workflow. Exit `0` si todos los pasos pasan, `1` si alguno falló, `2` si el workflow es inválido.
- `tija run --list` (`--json`): workflows disponibles con su descripción.
- TUI: opción `Workflows` que lista los workflows y muestra el progreso paso a paso.

### 7.3 Workflows por defecto
`init` instala `pre-pr.yaml` (el del ejemplo) con el mismo mecanismo de hash que los packs. La Fase 4 lo actualizará para agregar `review` antes de `pr prepare`; hasta entonces no se menciona `review`, porque un workflow no debe ofrecer comandos que todavía no existen.

## 8. Arquitectura

```text
internal/skills     → carga, validación de origen/licencia, listado de packs
internal/comments   → lexer JS/TS/JSX, extracción, reglas deterministas,
                      planificador de ediciones, verificador de tokens
internal/quality    → + PostCheck: diagnósticos de Biome por archivo (--reporter=github)
internal/prrules    → + attribution.go: patrones de atribución de agentes (strip y chequeo)
internal/workflows  → parser de .tija/workflows/*.yaml, validación, ejecutor
internal/engine     → clean_comments.go, run.go (orquestan lo anterior)
internal/assets/defaults/skills/, .../workflows/  → material embebido
THIRD_PARTY_NOTICES.md                            → atribución y licencias
```
El prompt de clasificación se arma en `internal/engine` reutilizando `AgentProvider` (Fase 3) sin cambios en sus flags de solo lectura.

## 9. Riesgos

| Riesgo | Mitigación |
|---|---|
| El lexer se equivoca con un caso de JSX o regex y toma código por comentario | Verificación de tokens obligatoria; archivo dudoso = archivo saltado; batería de fixtures con casos difíciles |
| El agente borra documentación de negocio | Solo los `remove` de clases de ruido con `confidence: high` son seguros; todo lo demás requiere `--unsafe`; `--diff` permite revisar antes |
| Costo de tokens en carpetas grandes | Alcance por defecto = diff; lotes; tier `fast`; las reglas deterministas resuelven sin agente lo más repetitivo |
| Los packs de terceros cambian upstream | Se fija el commit; la actualización es manual y revisada |
| El patrón de atribución borra una línea legítima del PR | Patrones anclados a línea completa y a nombres de herramientas concretas; tests con casos negativos ("Generamos el reporte con Codex de impuestos" no coincide) |
| Cambia el formato de salida de Biome | El parser de `--reporter=github` es tolerante; si no reconoce la salida, la segunda red se marca como `!` omitida, nunca como aprobada |
| Licencias | Ambos proyectos son MIT: permiten copiar y modificar si se conserva el aviso de copyright y la licencia. Se conserva en cada pack, en `THIRD_PARTY_NOTICES.md` y en los releases |

## 10. Criterios de salida de la Fase 3a

Validación sobre el frontend de Team4soft, en una rama propia (no en `dev`), con `src/pages/(features)/claims`:

- `tija clean comments "src/pages/(features)/claims"` reporta sin escribir y sale con exit 1.
- `--write` elimina los 54 separadores `// -----…` y los banners JSX `{/* ---------- Fila N ---------- */}`.
- Después de `--write`, y también después de `--write --unsafe`:
  - los tres `// biome-ignore …` siguen en su lugar;
  - los comentarios de reglas de negocio citados en la sección 1 siguen en su lugar;
  - todos los archivos pasan la verificación de tokens;
  - `tija check` (Biome) y el build del frontend pasan sin cambios respecto de antes.
- `--no-agent` aplica solo las reglas deterministas y funciona sin ningún CLI de agente instalado.
- Un `biome-ignore` borrado a propósito en un fixture hace que la segunda red restaure el archivo.
- `tija pr prepare` en una rama cuyos commits traen `Co-Authored-By: Claude …` produce un cuerpo sin atribuciones de agentes, sin `🤖` y sin vocabulario de la lista de *Empty AI Vocabulary*.
- `tija skills list` muestra `antislop-code`, `antislop-copywriting`, `thermo-review` y `thermo-quality` con autor, repo, commit y licencia. `THIRD_PARTY_NOTICES.md` existe y entra en el archivo del release.
- `tija run pre-pr` ejecuta los tres pasos, se detiene en el primero que falla y su `--json` es estable.
- Todo lo anterior probado en Windows.
- Tests automatizados:
  - lexer con fixtures: regex vs división, template literals anidados, `//` dentro de strings y de texto JSX, URLs, comentarios JSX, CRLF y BOM;
  - verificador: un cambio de código inyectado a propósito debe ser rechazado;
  - clasificación con un agente falso (fake provider) y golden files de la salida;
  - test de atribución de packs (sección 4.1, punto 6).
