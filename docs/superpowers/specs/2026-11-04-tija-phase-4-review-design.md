# Tija — Fase 4: Review

> Fecha: 2026-11-04
> Estado: borrador para aprobación
> Spec padre: `2026-10-22-tija-phases-design.md` (sección 5, Fase 4)
> Base existente: packs `thermo-review` y `thermo-quality` (Fase 3a, adaptados de Thermos de Cursor, MIT, commit `1ba91fc25c0977741dd0e267849e6234b037ed0a`), `internal/agents`, sesión de calidad (`.tija/session/quality.json`), workflows compuestos.

---

## 1. Intención y porqué

Hoy Tija limpia comentarios, corre la calidad y arma el PR, pero nadie revisa **lo que el diff hace**. Esa revisión la hace el autor a ojo o pegando el diff en un chat, sin reglas del repo y sin forma de saber si el modelo se inventó un problema.

La Fase 4 agrega `tija review`: una revisión del diff de la rama hecha por el agente con las rúbricas de Thermos y con las reglas de arquitectura del repo, donde **Tija verifica cada hallazgo antes de mostrarlo**. Un hallazgo que no apunta a código real del diff no llega al usuario.

**Por qué ahora:** las piezas ya existen (packs instalados, runner de agentes, sesión de calidad, workflows). Lo que falta es el motor de revisión y la verificación. Con `review` dentro de `pre-pr`, el flujo completo antes de un PR queda en un solo comando.

## 2. Objetivos, no-objetivos y anti-objetivos

**Objetivos**
- `tija review` en tres modos (`review`, `--quality`, `--deep`) limitados al diff contra `pullRequest.target`.
- Reglas de arquitectura **opcionales** y escritas por cada repo, con una parte que Tija verifica sola y otra que juzga el agente.
- Exit codes útiles como puerta: `pre-pr` se detiene si hay hallazgos graves.
- La misma salida para humanos, `--json`, TUI y (en la Fase 5) MCP.

**No-objetivos**
- Corregir código. `review` **no escribe** archivos del repo en esta fase (solo su sesión en `.tija/session/`).
- Publicar comentarios en GitHub.
- Revisar código que no está en el diff.
- Un lenguaje de reglas completo (AST, grafos de dependencias). Las verificaciones deterministas son patrones sobre líneas agregadas.

**Anti-objetivos (fracaso aunque funcione)**
- Que se muestre un hallazgo inventado: archivo que no está en el diff, línea fuera de lo cambiado o código citado que no existe.
- Que las severidades se inflen y todo salga `High` (regla de Thermos).
- Que las reglas de arquitectura sean obligatorias o que Tija imponga una arquitectura.
- Que `review` falle o se bloquee porque no hay agente instalado.
- Que el agente pueda modificar archivos durante la revisión.

## 3. Decisiones tomadas

### Decididas por el usuario (2026-11-04)
| Tema | Decisión |
|---|---|
| Hallazgos inventados | El agente no debe inventar hallazgos. Tija verifica archivo, línea y código citado; lo que no pasa se descarta y se reporta como descartado |
| Reglas de arquitectura | Markdown con frontmatter en `.tija/rules/architecture/*.md`. Checks deterministas opcionales en el frontmatter, reglas para el agente en el cuerpo. Nada obligatorio. Presets `dotnet-clean` y `react-features` incluidos |
| Síntesis de `--deep` | Determinista, sin tercera llamada al agente. Misma ubicación y regla → un hallazgo con `confirmedBy`. Severidades distintas → queda la mayor marcada `disputed` |
| Exit codes | `0` sin hallazgos en el umbral, `1` con hallazgos en el umbral (por defecto `high`: Critical o High), `2` error de uso o entorno, incluido un agente que falla |
| Sin agente instalado | Corren las verificaciones deterministas y la calidad, se avisa, y el exit code sale de esos hallazgos |
| Diffs grandes | Exclusiones por defecto; hasta ~3000 líneas cambiadas, una llamada; por encima, partes por carpeta en paralelo (máximo 3) |
| Timeouts | `fast` 3 min, `balanced` 6 min, `deep` 12 min; `--agent-timeout` los reemplaza |
| Calidad y tests | Reutiliza la sesión si es del commit actual y sin cambios; si no, corre `check`. `--no-check` lo omite. Un check fallido es un hallazgo High |
| Conexión con `pr prepare` | `.tija/session/review.json`; `pr prepare` lo cita en `## Pruebas` solo si es del commit actual. Nunca copia hallazgos al PR |
| `pre-pr` | Agrega `review` antes de `pr prepare`; un exit 1 detiene el workflow |
| Binario | Límite de 10 MB (restricción 12 del spec padre). No se esperan dependencias nuevas |

### Decididas por Claude (revisables)
| Tema | Decisión | Motivo |
|---|---|---|
| Alcance del diff | Árbol de trabajo contra el merge-base con `target` (commits de la rama + cambios sin commitear) + archivos sin seguimiento no ignorados | `review` no escribe, así que revisar lo que todavía no se commiteó es útil y sin riesgo. Además la verificación de la evidencia lee el archivo en disco: el diff tiene que corresponder a ese archivo |
| Regla de las 1000 líneas | La calcula Tija (`1000-lines-ceiling`, severidad Medium): un archivo que pasa de ≤ 1000 a > 1000 líneas en el diff. Se quita de las reglas que el agente puede reportar | Es aritmética: restricción 2 (deterministic first) |
| Catálogo de reglas de cada pack | Los packs `thermo-*` declaran sus ids en el frontmatter (`rules: [...]`). La verificación usa esa lista, así que un pack editado por el usuario puede agregar reglas. Se anota como cambio de adaptación en el frontmatter | Parsear la sección "Finding Schema" del Markdown sería frágil |
| Reglas en cada modo | Los checks deterministas corren en todos los modos; las reglas para el agente van en el prompt de cada pasada | Simple; la síntesis ya junta los duplicados |
| Comentarios del PR | Se leen con `gh` después de terminar la revisión y se cruzan por archivo y línea sin agente. No cambian el exit code | Regla de Thermos (auditoría independiente primero) y restricción 2 |
| Presets en `init` | Se ofrecen en modo interactivo (respuesta por defecto: no). `--yes` **no** los instala; `--rules-preset <nombre>` sí | Las reglas de arquitectura son una decisión del equipo, no un default |
| Validación de reglas | `tija doctor` valida los archivos de reglas (YAML, regex, ids duplicados). Sin comando `rules` nuevo | Evita superficie nueva; `doctor` ya es el lugar de la validación |
| Agente en solo lectura | Corre en la raíz del repo. Claude con herramientas de solo lectura; Codex con `--sandbox read-only` (ya existe); Gemini sin flags extra | El agente necesita leer el repo para no especular (regla de Thermos) y nunca debe escribir |
| Repo sin `tija init` | `review` funciona con detección (target = rama por defecto del remoto), sin reglas propias y sin guardar sesión | Igual que `check` en repos sin inicializar |

## 4. Comando

```text
tija review [--quality | --deep] [--fail-on critical|high|medium|low|none]
            [--agent <nombre>] [--no-agent] [--agent-timeout <duración>]
            [--no-check] [--no-pr-comments] [--json]
```

| Modo | Pack(s) | Tier | Llamadas |
|---|---|---|---|
| `tija review` | `thermo-review` | `balanced` | 1 por parte del diff |
| `tija review --quality` | `thermo-quality` | `balanced` | 1 por parte |
| `tija review --deep` | ambos en paralelo | `deep` | 2 por parte |

El tier se puede cambiar en el manifest (`agents.tasks.review`), como el resto de tareas. `--deep` siempre usa `deep`.

## 5. Flujo

1. **Contexto.** Raíz, manifest (o detección), `target`, merge-base. Sin merge-base → exit 2.
2. **Diff.** `git diff <merge-base>` del árbol de trabajo + archivos sin seguimiento como agregados. Se quitan las exclusiones (`review.exclude`). Sin cambios → exit 0 con "nothing to review".
3. **Calidad.** Sesión válida para el commit actual y árbol limpio → se reutiliza. Si no, corre `check` (salvo `--no-check`). Cada paso fallido produce un hallazgo `quality-check-failed` (High) que apunta al log.
4. **Checks deterministas.** Reglas `checks` de los archivos de arquitectura + `1000-lines-ceiling` (en `--quality` y `--deep`). Solo sobre líneas agregadas.
5. **Partes.** Si el diff supera `review.maxLines` (3000 por defecto), se agrupan los archivos por carpeta hasta llenar cada parte. Un archivo más grande que el límite va solo.
6. **Agente.** Por cada parte y pasada, un prompt (sección 7). Máximo 3 llamadas a la vez. Sin agente o con `--no-agent`, se salta con aviso.
7. **Verificación** de cada hallazgo del agente (sección 8).
8. **Síntesis** (solo `--deep`, sección 9) y unión con los hallazgos deterministas.
9. **Comentarios del PR** (sección 10).
10. **Resultado.** Orden por severidad, luego confirmados, luego archivo y línea. Se guarda `.tija/session/review.json`. Exit code según el umbral (sección 11).

## 6. Reglas de arquitectura

### 6.1 Formato

Archivos `.tija/rules/architecture/*.md` (la carpeta se configura en `review.rules`, una lista de globs). Ninguno es obligatorio.

```markdown
---
name: backend-dotnet            # obligatorio, único
description: Capas de la API     # opcional
appliesTo: ["src/**/*.cs"]       # opcional; sin él aplica a todo el diff
checks:                          # opcional: verificaciones deterministas
  - id: be-domain-no-infra       # único entre todos los archivos
    severity: High               # Critical | High | Medium | Low
    files: "src/*.Domain/**/*.cs"   # glob (o lista de globs)
    forbid: '^\s*using\s+[\w.]*\.(Infrastructure|Api)\b'   # regex RE2
    message: El dominio no puede depender de Infrastructure ni de la API.
---
## be-thin-controllers: Controladores delgados
Texto libre para el agente. Puede indicar la severidad esperada.
```

- **Checks:** `forbid` se evalúa sobre cada **línea agregada** de los archivos que cumplen `files` (y `appliesTo`). Una coincidencia es un hallazgo con la severidad del check, `message` como explicación y la línea como evidencia. La sintaxis es la de Go (RE2, sin lookbehind); el README lo documenta.
- **Reglas para el agente:** cada `## <id>: <título>` del cuerpo es una regla. El texto hasta el siguiente `##` es su descripción. Se pasan al agente solo si algún archivo del diff cumple `appliesTo`.
- **Ids:** únicos entre todos los archivos de arquitectura y distintos de los ids de los packs. Un id duplicado o un regex inválido es un error de `doctor` y hace que `review` salga con 2.

### 6.2 Presets

Embebidos en el binario y propios de Tija (sin material de terceros, sin datos de empresas):

- `dotnet-clean`: capas Domain / Application / Infrastructure / Api. Checks: dominio sin `using` de Infrastructure o Api; controladores sin `DbContext`. Reglas: controladores delgados, DTOs en el borde, async de punta a punta, configuración y secretos fuera del código.
- `react-features`: features en carpetas propias. Checks: componentes sin `fetch`/`axios` directos; sin imports entre features. Reglas: estructura de una feature (components, hooks, services, types), sin `any` en tipos públicos, estado de servidor en hooks/services.

`init` los ofrece según el stack detectado (sección 3). Se instalan con el mecanismo de `defaultsVersion` + hash, así que un preset editado por el usuario nunca se reemplaza. Las rutas de los presets son un punto de partida: cada repo las ajusta.

## 7. Prompt al agente

Por cada parte y pasada, el prompt lleva:
- El cuerpo del `SKILL.md` del pack (sin el frontmatter de origen).
- Las reglas de arquitectura para el agente que aplican a la parte.
- El resumen de calidad (pasos, estado, tests).
- La lista de archivos de la parte y su diff.
- Las reglas que ya calcula Tija (`1000-lines-ceiling` y los checks), para que el agente no las repita.
- El idioma del contenido (`content.language`) para `title`, `explanation` y `suggestion`.
- La instrucción de responder **solo** JSON con este formato:

```json
{"findings": [{
  "file": "src/a.ts", "line": 42, "endLine": 44,
  "severity": "High", "rule": "breaking-functionality",
  "title": "…", "explanation": "…", "suggestion": "…",
  "evidence": "código copiado de las líneas señaladas"
}]}
```

Sin hallazgos, `{"findings": []}`. El agente puede leer el repo para confirmar un problema, pero no escribe.

## 8. Verificación de hallazgos

Cada hallazgo del agente pasa estas comprobaciones, en orden. El primero que falla define el motivo del descarte:

| Comprobación | Motivo de descarte |
|---|---|
| `file` está en el diff de la parte (ruta normalizada con `/`) | `file-not-in-diff` |
| `[line, endLine]` toca un bloque del diff (lado nuevo, con su contexto) | `line-outside-diff` |
| `evidence` no vacía (máximo 5 líneas) y, normalizando espacios, aparece en el archivo entre `line-3` y `endLine+3` | `evidence-not-found` |
| `severity` es Critical, High, Medium o Low | `invalid-severity` |
| `rule` está en `rules` del frontmatter del pack de la pasada (sin `1000-lines-ceiling`) o en las reglas de arquitectura para el agente | `unknown-rule` |

- **Respuesta inválida** (no es JSON, o no tiene la forma de la sección 7): un reintento con el error, como en `report.go`. Si vuelve a fallar, esa parte cuenta como fallida (exit 2, sección 11).
- **Descartados:** se cuentan en el resumen y aparecen en `--json` (`discarded`, con motivo). En la salida de texto, solo el conteo y `--json` para verlos.

## 9. Síntesis de `--deep`

1. Se agrupan los hallazgos verificados de ambas pasadas por archivo y por líneas que se superponen.
2. Dentro de un grupo, los de la misma `rule` se unen: un solo hallazgo con `confirmedBy: ["review", "quality"]`. Si sus severidades difieren, queda la mayor y `disputed: true` (con ambas severidades en `--json`).
3. Hallazgos de reglas distintas en la misma ubicación se mantienen separados.
4. Los hallazgos deterministas no se unen con los del agente: ya son exactos.

La orquestación en paralelo y la síntesis siguen la idea del skill `thermos` de Cursor, reimplementada en Go. Se atribuye en `THIRD_PARTY_NOTICES.md` y en un comentario del paquete (restricción 11).

## 10. Comentarios del PR

Después del paso 8, si hay un PR abierto para la rama y `gh` está autenticado, Tija lee los comentarios de revisión (con archivo y línea) del PR:
- Un comentario en la misma ubicación que un hallazgo se adjunta a ese hallazgo (`prComments`, con su URL).
- Los demás se listan en una sección aparte, "PR comments not covered", solo informativa.
- No cambian severidades ni el exit code. Sin `gh`, sin auth o con `--no-pr-comments`, se omite con una nota.

## 11. Exit codes y umbral

- `0`: ningún hallazgo con severidad ≥ umbral.
- `1`: al menos uno.
- `2`: uso o entorno (no es un repo git, sin merge-base, regla de arquitectura inválida, agente que falla o responde mal después del reintento). Aun con 2, se muestran los hallazgos obtenidos y el resumen indica qué partes fallaron.
- Umbral: `review.failOn` en el manifest (por defecto `high`), reemplazado por `--fail-on`. `none` nunca devuelve 1.
- Sin agente instalado, o con `--no-agent`: aviso y exit según los hallazgos deterministas. No es un error.

## 12. Manifest

```yaml
review:
  failOn: high
  rules: [".tija/rules/architecture/*.md"]
  exclude: ["**/*.lock", "**/package-lock.json", "**/pnpm-lock.yaml", "**/*.min.js", "dist/**", "build/**", "**/bin/**", "**/obj/**", "**/*.g.cs", "**/*.generated.*"]
  maxLines: 3000
```

Todo opcional; si falta la sección se usan estos valores. `init` la agrega en repos nuevos y la propone como diff en repos existentes (mecanismo actual). `DefaultsVersion` sube a 5.

## 13. Salida

**Texto:**
```text
✗ High    src/claims/api.ts:42  breaking-functionality  [review, quality]
  El endpoint ahora devuelve 404 cuando …
  → Mantener … 
! Medium  src/claims/Form.tsx:118  fe-no-fetch-in-components
  …
Summary: 1 high, 1 medium · 2 discarded · quality: passed · mode: deep
```

**`--json`** (estable y documentado en el README):
```json
{
  "mode": "deep", "base": "origin/dev", "commit": "abc123", "dirty": true,
  "failOn": "high", "exit": 1,
  "quality": {"reused": true, "status": "passed"},
  "findings": [{"id": "F1", "source": "agent|check|quality",
    "passes": ["review","quality"], "file": "…", "line": 42, "endLine": 44,
    "severity": "High", "rule": "…", "title": "…", "explanation": "…",
    "suggestion": "…", "evidence": "…", "confirmedBy": [], "disputed": false,
    "severities": [], "prComments": []}],
  "discarded": [{"pass": "review", "file": "…", "line": 10, "reason": "evidence-not-found"}],
  "prComments": {"read": true, "uncovered": []},
  "chunks": {"total": 2, "failed": 0},
  "summary": {"Critical": 0, "High": 1, "Medium": 1, "Low": 0}
}
```

`.tija/session/review.json` guarda este mismo objeto con la fecha.

## 14. Integraciones

- **`pr prepare`:** si `review.json` es del commit actual y `dirty` es falso, el prompt (y el borrador sin agente) puede citar en `## Pruebas` que se ejecutó `tija review` en el modo X y su resumen por severidad. Nada más.
- **`pre-pr`:** `clean comments --write` → `check` → `review` → `pr prepare`. `review` entra en la lista de comandos permitidos de los workflows. El archivo instalado se actualiza solo si el usuario no lo editó (hash).
- **`doctor`:** valida los archivos de reglas de arquitectura.
- **TUI:** opción `Review` → elegir modo → avance por pasada y parte → lista de hallazgos por severidad con detalle (explicación, sugerencia, evidencia) y desplazamiento. Mascota: `thinking` mientras corre, `success` sin hallazgos en el umbral, `warning` con hallazgos, `error` con exit 2.
- **MCP (Fase 5):** `review.run` llama al mismo engine.

## 15. Arquitectura

```text
internal/review     → diff con hunks, partes, prompt, parseo, verificación, síntesis, umbral (puro y testeable)
internal/rules      → carga y validación de .tija/rules/architecture/*.md; checks deterministas
internal/engine/review.go → orquesta: contexto, calidad, checks, agente en paralelo, PR comments, sesión
internal/cli/review.go    → flags y render
internal/tui/review_view.go
internal/assets/defaults/rules/architecture/ → presets
```

Los nombres son orientativos (spec padre, sección 6). `internal/agents.Generate` se reutiliza; agregar las opciones de solo lectura de Claude es un cambio en `Argv`.

## 16. Riesgos

| Riesgo | Mitigación |
|---|---|
| El agente cita evidencia con pequeñas diferencias y se descartan hallazgos reales | Normalización de espacios y ventana de ±3 líneas; el conteo de descartes se revisa en la validación real y se ajusta |
| Revisiones lentas o caras en `--deep` con diffs grandes | Exclusiones, partes, concurrencia 3, timeouts por tier |
| Regex frágiles o lentos en reglas del usuario | RE2 (tiempo lineal), solo sobre líneas agregadas, errores en `doctor` |
| Ruido de los presets en repos que no siguen esa estructura | Presets opcionales, nunca instalados con `--yes` |
| Diferencias de flags entre CLIs de agentes para el modo de solo lectura | Probar los tres en Windows; si un CLI no permite restringir, documentarlo |

## 17. Criterios de salida de la Fase 4

1. En una rama real del frontend de Team4soft y otra del backend .NET, `tija review`, `--quality` y `--deep` terminan con exit 0 o 1 y, revisando a mano cada hallazgo mostrado, **ninguno** tiene archivo, línea o evidencia incorrectos.
2. En una rama de prueba con una violación sembrada de cada preset y un bug sembrado (p. ej. una condición invertida), los checks deterministas encuentran el 100 % de las violaciones y el agente reporta el bug en `review` o `--deep`.
3. Un hallazgo inventado a mano en una respuesta simulada (test) se descarta con el motivo correcto en cada uno de los cinco casos de la sección 8.
4. Sin CLI de agente: aviso, solo deterministas, exit según hallazgos.
5. `tija run pre-pr` se detiene en `review` cuando hay un High.
6. `--json` documentado y estable; `go vet` y `go test ./...` en verde; binario ≤ 10 MB.
