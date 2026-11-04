# Fase 4 — Handoff para revisión

Fecha: 2026-11-04. Branch: `feat/phase-4-review`.
Plan: `docs/superpowers/plans/2026-11-04-tija-phase-4-review.md`.
Spec: `docs/superpowers/specs/2026-11-04-tija-phase-4-review-design.md`.

## Estado y alcance

Tasks 1–12 implementadas y commiteadas individualmente, con reportes task-01.md a task-12.md en esta carpeta. Task 13 incluye validación local, pruebas reales y este handoff. **Actualización (revisión, 2026-11-04):** el usuario autorizó el envío del diff del backend a Claude y la revalidación con timeouts por defecto cerró la aceptación del backend (ver "Revisión y revalidación del backend" al final). La primera corrida, descrita abajo, había terminado por timeouts del sandbox.

Entregado: `tija review`, quality/deep, verificación de archivo/rango/evidencia/severidad/regla, síntesis determinista, arquitectura opcional, presets, umbrales/exit codes, chunks/concurrencia, sesión, CLI/JSON/render, `pre-pr`, doctor/init, cita en PR y TUI. No se agregó dependencia. No se publicaron comentarios, PRs ni pushes.

## Commits por tarea

| Task | Commit | Contenido |
|---|---|---|
| 1 | fe68d63 | Manifest review, defaults v5 y catálogo de reglas |
| 2 | dcf7e3b | Parseo de diff nuevo y chunks |
| 3 | 5b9ea81 | Reglas, checks y presets |
| 4 | e43013a | Parseo y verificación de hallazgos |
| 5 | 3264348 | Síntesis, ceiling, comentarios, resultado y sesión |
| 6 | 0b6d672 | Prompt y Fence |
| 7 | fda5a0b | Agentes read-only y operaciones Git |
| 8 | 8fe0bc1 | Engine integrado y pruebas |
| 9 | 4ca558c | CLI, render, pre-pr, README y notices |
| 10 | e42ab80 | Presets opt-in en init y validación doctor |
| 11 | 13e5184 | Cita de review fresco en PR, delegación de Fence |
| 12 | f1064e4 | TUI Review y pruebas del modelo |

Ajustes auditados adicionales: 0dab4ba conserva grupos completos de carpetas anidadas; e12380d estabiliza warnings como array JSON; 7643a81 limita el recorrido de reglas al prefijo literal del glob; b947003 evita dereferenciar symlinks de código. Task 13 se registra con el commit `docs(handoff): record phase 4 review verification` que contiene este documento y la evidencia.

Los commits de los subagentes se intercalaron por independencia de archivos. Todos permanecen en el branch solicitado. Los commits de hardening conservan addenda en el reporte de la tarea afectada.

## Decisiones de implementación

1. `review` persiste únicamente `.tija/session/review.json` dentro del repo inicializado. La reutilización de `RunQuality` usa `QualityRequest.Ephemeral`: sin reescribir quality.json, logs/reportes en TEMP, logs fallidos retenidos. Logs y reportes usan directorios hermanos para que la limpieza de un reporte no borre una evidencia anterior. Un repo sin manifest no guarda sesión.
2. Cada llamada de agente tiene un directorio TEMP propio; evita colisiones de `codex-last.txt` entre partes/pasadas. `errgroup.SetLimit(3)` aplica al total de llamadas de una ejecución; eventos serializados y resultados recogidos en orden estable. Los timeouts son por intento y hay máximo dos intentos.
3. `ReviewStage` es alias de `PRStage` porque `StageContext`, `StageAgent` y `StageDone` ya existen en el mismo paquete. Se preservan nombres y valores del API solicitado sin duplicar identificadores Go.
4. `Review.WithDefaults` hereda nil; listas vacías explícitas conservan la intención de desactivar reglas/exclusiones. Presets siguen siendo opcionales: --yes no los instala; --rules-preset sí. Presets editados se preservan.
5. Los presets se embeben en un FS separado y accesores en `assets/presets.go`; `assets.Files()` no los instala indiscriminadamente. Las reglas solo se buscan bajo el prefijo literal de los globs; no se recorre node_modules con el default.
6. Se excluyen archivos no regulares, incluidos symlinks, del diff revisable. Verificar evidencia vuelve a comprobar el tipo. Así no se lee accidentalmente el destino externo de un enlace como si fuera contenido Git.
7. La cita de review se incorpora también después del enforcement determinista de Pruebas, para que no desaparezca de la respuesta del agente. Solo sesión de HEAD y dirty=false; nunca copia hallazgos. `diffFence` conserva wrapper y delega a `review.Fence`.
8. El contenido de agente respeta content.language; títulos built-in de calidad/ceiling se adaptan a español por defecto. Mensajes UI siguen en inglés. Rutas persistidas usan `/`; pruebas cubren CRLF y rutas Windows.
9. `.gitattributes` fija `*.go text eol=lf`. Con core.autocrlf=true el checkout original hacía que gofmt listara archivos solo por CRLF. Se normalizó el working tree; Git confirmó que no hubo cambios de contenido Go ajeno a la implementación.
10. Las pruebas reales usan clones locales independientes con `--no-hardlinks`, en lugar de registrar worktrees en los repos del usuario: mismo aislamiento, sin modificar siquiera su metadata `.git`. Init/bootstrap se excluye solo dentro de los clones de validación; referencias, ajustes y alcance están documentados.

## Evidencia automatizada y manual

Ver task-13.md para comandos, resultado final de tests/vet, tamaño y hash del binario. Todos los tests específicos pasan, incluido el workflow embebido exacto: clean comments exit0 → check exit0 → review High exit1; pr prepare no corre. Pruebas del engine cubren modo profundo con severidad disputada, max3, reintento, resultados parciales, no agente, sesión, calidad, untracked y comentarios PR con paginación/original_line.

Se ejecutó además el binario con PATH limitado a Git y System32, sin CLIs de agente: exit0, aviso correcto, sin fallo de entorno. En ese mismo repo limpio y con commits reales, `pr prepare --no-agent --no-check --json` terminó en0 y citó review en Pruebas. No se ejecutaron tests de aplicación en esa prueba, y el PR lo declara.

> La evidencia cruda (JSON, auditorías, logs y fixtures) contiene código y datos de repositorios privados. Se conserva solo en local, en `docs/superpowers/handoff/phase4/validation/`, ignorada por git. Aquí se registran solo resultados agregados.

### Frontend real (React/Vite, repositorio privado)

Rama real con 9 archivos cambiados. Claude autenticado real, timeout 2m, no-check/no-pr-comments.

| Modo | Exit | High | Medium | Low | Descartados |
|---|---:|---:|---:|---:|---:|
| review real | 0 | 0 | 0 | 1 | 0 |
| quality real | 0 | 0 | 1 | 1 | 0 |
| deep real | 0 | 0 | 1 | 2 | 0 |
| sembrado sin agente | 1 | 2 | 0 | 0 | 0 |
| sembrado deep | 1 | 6 | 2 | 1 | 0 |

Ambos checks sembrados detectados2/2. El bug invertido produjo2 tests Node fallidos y fue hallado por ambos pases. Se auditaron los17 hallazgos mostrados entre las cinco salidas: archivo, rango y evidencia correctos.

### Backend real (.NET, repositorio privado)

Rama real con 3 archivos fuente cambiados (53+/62-).

Los tres modos reales terminaron en exit2 por timeout2m por intento (por debajo del default de 6m); no hallazgos ni descartados. Seed sin agente: exit1,3 High en3 líneas intencionalmente coincidentes, ambos checks activados. Auditoría exacta3/3. Bug de condición invertida: ejecutable .NET de prueba falla; cambiar `>` por `<=` hace pasar el control; luego se restauró el bug en el clon. Seed deep retuvo los3 checks con exit2 al agotar timeouts30s; no se afirma detección del bug por agente.

No se ejecutó build/restore global de la aplicación. El repositorio original quedó limpio.

## Riesgos y trabajo pendiente para el revisor

- **Resuelto en la revisión (ver la última sección).** Texto original: **Bloqueo real de aceptación backend:** el reintento elevado fue rechazado automáticamente por posible envío del diff privado a Claude sin autorización explícita. Se solicitó esa autorización al usuario y no se lanzó ningún intento posterior al rechazo. Se requiere respuesta afirmativa antes de nuevos envíos; después ejecutar secuencialmente review/quality/deep con timeout default y auditar cada hallazgo. Validar también detección del bug sembrado.
- La evidencia estructural no prueba el juicio semántico del agente. En frontend hubo un Low de empty options bajo breaking-functionality sin explicación y riesgos de prefijo gateway cuya configuración desplegada no se verificó. Severidades variaron entre modos. El revisor debe evaluar utilidad y clasificación, además de las citas.
- El preset cross-feature es sintáctico: puede señalar imports absolutos dentro de la misma feature. Es un punto de partida editable, no un análisis de dependencias/AST. Ajustar al repo.
- Claude se probó real en Windows. Los argumentos de Codex/Gemini se prueban unitariamente; no se probó ejecución real ni enforcement de esos proveedores. Gemini no agrega flags de restricción según el contrato aprobado; revisar esa limitación antes de usarlo con código sensible.
- Las llamadas reales usaron no-check y no-pr-comments. Calidad, reuse, GitHub comments/paginación y errores se cubren con fakes en tests; no se afirma integración real contra servicios de GitHub o builds completos de las aplicaciones reales.
- Las sesiones solo vinculan commit+dirty; no incluyen fingerprint del contenido mutable ni invalidan por cambios de herramientas posteriores. Es el contrato de fase4, no evidencia permanente de aprobación.
- Los logs fallidos en TEMP quedan retenidos para inspección; limpieza manual futura posible. No se agregaron a repos de aplicaciones.

## Ruta sugerida de revisión

1. Leer reportes task-01…13 y ejecutar comandos de task13 con GOCACHE temporal.
2. Revisar internal/review/verify.go y sus cinco motivos, CRLF, archivos borrados y parser de rutas; examinar chunk/synth y fixtures.
3. Inspeccionar engine/review*.go y QualityRequest.Ephemeral, proveedores y helpers Git; contrastar readonly/no-writes con tests.
4. Revisar reglas/presets, upgrade de defaults/hash, opt-in init, doctor y flujo TUI.
5. Reproducir workflow stop y PR citation; examinar raw JSON y auditorías reales. Mantener pendiente la aceptación backend hasta autorización y ejecución exitosa.

## Revisión y revalidación del backend (2026-11-04)

### Correcciones de la revisión

| Commit | Corrección |
|---|---|
| `8503250` (amend) | La evidencia cruda con código privado sale del repo: `docs/superpowers/handoff/**/validation/` queda ignorado y los handoffs registran solo resultados agregados. |
| `346742e` | Un `--agent` explícito que no está instalado sale con exit 2. Sin `--agent`, si `agents.default` no está instalado se usa el primer CLI disponible. Un timeout o un CLI ausente no se reintentan (solo una respuesta inválida). Los checks de los presets salvo `be-domain-no-infra` pasan a `Medium` y `fe-no-fetch-in-components` solo aplica a `components/`. |

### Revalidación autorizada

Con autorización explícita del usuario para enviar el diff del backend a Claude, se repitieron las corridas en los mismos clones aislados con el binario de `346742e`, timeouts por defecto (6 min `balanced`, 12 min `deep`), `--no-check --no-pr-comments --agent claude`. Ninguna parte falló; cada corrida tardó entre 15 y 29 s, lo que confirma que los timeouts anteriores venían del sandbox de red, no de Tija.

| Corrida | Exit | Critical | High | Medium | Low | Descartados |
|---|---:|---:|---:|---:|---:|---:|
| review real | 0 | 0 | 0 | 1 | 0 | 0 |
| quality real | 1 | 0 | 1 | 0 | 1 | 0 |
| deep real | 0 | 0 | 0 | 2 | 0 | 1 (`unknown-rule`) |
| sembrado deep | 1 | 1 | 4 | 3 | 1 | 0 |

**Auditoría manual:** los 14 hallazgos mostrados apuntan a archivos del diff, a líneas dentro de bloques del lado nuevo y citan evidencia que existe en esas líneas. El descartado de `deep` es correcto: el pase de calidad usó una regla fuera de su catálogo.

**Criterio 2 (backend):** el bug sembrado (condición invertida) lo reportan ambos pases de `deep` (`breaking-functionality` High y `clean-design` Critical); los checks deterministas encuentran 3/3 líneas sembradas. En el clon sembrado `be-controller-no-dbcontext` sigue en High porque el preset se instaló con el binario anterior a `346742e`.

**Observaciones semánticas (no afectan la verificación):**
- En `quality` real, endpoints comentados en lugar de borrados salieron como High y producen exit 1; en `review` y `deep` el mismo cambio sale Medium. La severidad del agente varía entre modos.
- En el sembrado, el pase de calidad clasificó el bug lógico como `clean-design` Critical: la regla no es la más adecuada y la severidad es alta respecto del pase de review. Como las reglas difieren, la síntesis no los une (comportamiento del spec §9).

Con esto los criterios 1 y 2 del §17 quedan cumplidos en frontend y backend. La evidencia cruda está en `validation/rerun-2026-11-04/` (local, ignorada por git).
