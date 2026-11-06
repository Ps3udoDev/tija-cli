# Fase 5: tareas 1–12 completas y cierre parcial

Fecha: 2026-11-05 (tareas 1–5); actualizado 2026-11-06 (revisión 6–8 y continuación 9–13). Rama: `feat/phase5-mcp`.

## Estado

La medición inicial de la tarea 2 superó el antiguo máximo de 10 MB. El usuario autorizó continuar con el SDK oficial y actualizó el criterio: objetivo < 15 MB, alerta técnica desde 20 MB y máximo permitido 25 MB (bytes decimales). No se activó el fallback manual. Tras autorizar la continuación, se revisaron y corrigieron las tareas 6–8 y se completaron 9–12. **La tarea 13 sigue parcial**: CLI y escenario Qapture pasan en Claude Code y Codex; faltan Claude Desktop, interfaz de Antigravity y CI remoto. Véanse [revisión 6–8](review-06-08.md) y [cierre](task-13.md).

| Tarea | Estado | Reporte |
| --- | --- | --- |
| 1. Go 1.27.3 y govulncheck | Completada | [task-01.md](task-01.md) |
| 2. SDK y arranque MCP | Completada | [task-02.md](task-02.md) |
| 3. Núcleo y herramientas de lectura | Completada | [task-03.md](task-03.md) |
| 4. EvidenceRef y CLI | Completada | [task-04.md](task-04.md) |
| 5. Herramientas locales | Completada | [task-05.md](task-05.md) |
| 6. `quality_fix` y `comments_clean` | Completada | [task-06.md](task-06.md) |
| 7. `pr_create` | Completada | [task-07.md](task-07.md) |
| 8. Runner compartido y `workflow_run` | Completada | [task-08.md](task-08.md) |
| 9. `report_activity` | Completada | [task-09.md](task-09.md) |
| 10. Prompt `documentar_pr` | Completada | [task-10.md](task-10.md) |
| 11. `mcp config` y doctor | Completada | [task-11.md](task-11.md) |
| 12. CI, tamaño y documentación | Completada localmente; jobs remotos pendientes | [task-12.md](task-12.md) |
| 13. Verificación de cierre | Parcial; pendiente interfaz de clientes y CI remoto | [task-13.md](task-13.md) |

Se usaron subagentes por tarea, con dependencias coordinadas y revisión de integración por el agente principal. Los cambios previos del usuario en los specs, plan y propuesta se conservaron fuera de los commits de implementación; el plan y los specs también contienen las actualizaciones de tamaño y contrato realizadas en esta sesión. La interrupción del primer turno terminó los subagentes; se retomó su trabajo desde el árbol existente.

Commits locales, uno por tarea: `7fd31c6` (1), `5af46a8` (2), `9ddc3e5` (3), `57ce092` (4) y `e10041f` (5). No se hizo push ni se abrió un PR.

## Verificación histórica de las tareas 1–5

| Comprobación | Resultado |
| --- | --- |
| `go test -count=1 ./... -timeout 15m` | PASS, exit 0; engine 252,450 s, CLI 66,164 s, MCP 29,791 s |
| `go vet ./...` | PASS, exit 0 |
| `gofmt -l` sobre archivos Go modificados y `internal/mcpserver` | Sin archivos pendientes de formato |
| `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | Exit 0; `No vulnerabilities found.` |
| `git diff --check` | PASS |

Logs y artefactos locales ignorados por Git: `dist/phase5-tests.log`, `dist/phase5-tests.exit` (0), `dist/phase5-govulncheck.log`, `dist/phase5-sizes.log` y `dist/phase5-gate/`.

| Destino | Bytes finales | Objetivo < 15 MB |
| --- | ---: | --- |
| Windows / amd64 | 12 598 784 | Cumple |
| Linux / amd64 | 12 210 336 | Cumple |
| macOS / arm64 | 11 523 026 | Cumple |

Compilador: Go 1.27.3; `CGO_ENABLED=0`; `go build -ldflags="-s -w"`, sin compresión. Los tamaños corresponden a las ocho herramientas, con evidencias integradas. SHA256 del binario Windows: `FCAFA445AEC306E35A3974820DEEB08BFFB4AEEA71453D8B9829F07C8E8F565E`.

Herramientas registradas: `repo_status`, `doctor`, `repo_rules`, `repo_context`, `skills_list`, `quality_check`, `review_run` y `pr_prepare`. El snapshot conserva nombres, entradas, anotaciones y los esquemas de salida requeridos para las herramientas presentes.

## Decisiones y revisión

1. Aplicar el nuevo criterio de tamaño autorizado: objetivo < 15 000 000 bytes; alerta desde 20 000 000; máximo 25 000 000. Plan y specs actualizados; mantener el SDK.
2. La tarea 2 ya incluye pruebas de raíz, inicialización y stdout, además de los avisos de licencia. El fallo transitorio de `go vet` por `root.Find != nil` fue corregido y la comprobación posterior pasó.
3. El resultado final de una primera suite aislada de la tarea 1 se perdió al terminar la sesión del subagente. No se atribuye un PASS a esa ejecución. La suite integrada posterior, con salida guardada, sí pasó y cerró la verificación de las tareas 1–5.
4. Se conserva la paridad del JSON del CLI: código de salida en `_meta["tija/exitCode"]`, `review.exit` intacto y lista de skills envuelta en `{ "skills": [...] }`. El SDK devuelve argumentos inválidos como `isError: true` sin ejecutar el handler, mientras que una herramienta inexistente da error JSON-RPC. Los mapas nil del engine se admiten como `null` en los esquemas de salida. Adaptaciones documentadas en el reporte de la tarea 3 y en el contrato.
5. El runner compartido de workflows pertenece a la tarea 8. No anticipar ese cambio al agregar `Deps.RunStep` en la tarea 2.
6. Las tareas 6–8 se implementaron en el relevo autorizado y se revisaron en la continuación del 2026-11-06. Se corrigieron detección de cambios tras cancelar, selección explícita de agente y revalidación previa a publicar un plan. Los permisos y las regresiones están documentados en [review-06-08.md](review-06-08.md).
7. La tarea 7 revalida explícitamente las referencias persistidas mediante `revalidateStoredEvidence`, sin tratarlas como entrada nueva ni aceptar rutas completas reintroducidas. `ApplyCreatePR` también rechaza cambios de HEAD, rama o target posteriores a la planificación.

## Alcance de las verificaciones

Las mediciones son compilaciones cruzadas desde Windows, sin comprimir y con `-ldflags="-s -w"`; no equivalen a pruebas de ejecución en Linux o macOS. Las tareas 1–8 no validaron clientes reales ni Qapture; los resultados posteriores de Claude Code, Codex y Qapture constan en task-13.md.

## Tareas 6–8 (2026-11-06)

**Revisión previa de las tareas 1–5:** se leyeron sus reportes y el código del núcleo (`server.go`, `tools.go`, `progress.go`, `tools_local.go`, `evidence.go`). `go vet`, `gofmt` y los tests de `internal/mcpserver` y de las evidencias pasaron. No hizo falta corregir nada. Las adaptaciones que documentó la tarea 3 (`_meta["tija/exitCode"]`, `{skills: [...]}` y mapas anulables) se mantienen.

Commits, uno por tarea: `719b85d` (6), `20e5029` (7) y `3843d05` (8). Sin push ni PR.

| Comprobación (al cerrar la tarea 8) | Resultado |
|---|---|
| `go vet` y `go test -count=1 -timeout 15m` sobre todos los paquetes salvo `./dist` | PASS, exit 0 (`dist/phase5-task8-tests.log`) |
| `govulncheck@v1.9.0` | `No vulnerabilities found.` |
| Binario windows/amd64 con `-s -w` | 12 761 088 bytes, cumple el objetivo < 15 MB |

**Herramientas registradas (12):** `repo_status`, `doctor`, `repo_rules`, `repo_context`, `skills_list`, `quality_check`, `review_run`, `pr_prepare`, `quality_fix`, `comments_clean`, `pr_create` y `workflow_run`. Falta `report_activity` (tarea 9).

**Notas heredadas del cierre de la tarea 8:**

1. **`resultError` (tools.go):** úsalo cuando una herramienta falle pero deba devolver su salida estructurada. **`adjustSchema`:** úsalo cuando la salida embebe un reporte que no siempre está presente; si no, el SDK rechaza la salida por campos obligatorios faltantes.
2. **`json.RawMessage` en las salidas** ya se describe como cualquier valor JSON. Cualquier otro tipo con JSON propio necesita un override parecido en `outputSchema`.
3. **`cli.Deps.Dir`** fija el repositorio de los comandos. La tarea 11 (`mcp config`) no lo necesita, pero cualquier ejecución nueva de comandos del CLI desde MCP debe pasarlo.
4. **Nombres reales para el prompt `documentar_pr` (tarea 10):**
   - **Herramientas de Tija:** las de la lista de arriba.
   - **`workflow_run`:** responde `needs-permission` con la lista `needs`.
   - **`pr_create`:** sin `write` responde `needs-write` con `question`; con un borrador con problemas responde `draft-has-problems` (`isError`).
5. **Golden:** `go test ./internal/mcpserver -run ToolsGolden -update` después de agregar una herramienta.
6. **Resuelto en la continuación:** `dist/phase5-task2-mcp_test.go` se renombró a `.go.txt`. La suite final usa `go test ./...` y `go vet ./...` sin excluir paquetes.
7. **Pendientes de validar en la tarea 13 con clientes reales:**
   - cancelación desde el cliente;
   - cómo muestra cada cliente las anotaciones `destructive` y `openWorld` de `quality_fix`, `comments_clean`, `pr_create` y `workflow_run`.

## Continuación 9–13 y revisión final (2026-11-06)

Se validaron las decisiones heredadas y se corrigieron los hallazgos de [review-06-08.md](review-06-08.md). El registro actual tiene **trece herramientas**, incluida `report_activity`, y el prompt embebido `documentar_pr`. `DefaultsVersion` no cambia. Las tareas 9 y 10 se hicieron con subagentes; al agotarse sus créditos, el agente principal completó correcciones, tareas 11–12 y validación 13.

Commits locales de esta continuación: `f2337af` (correcciones 6–8), `3d506db` (9), `80cb099` (10), `104b304` (11) y `58f894a` (12). No se hizo push ni se abrió un PR.

La suite completa terminó con exit 0 (engine 289,792 s, CLI 69,737 s, MCP 56,041 s). También pasaron vet, formato, govulncheck y compilaciones. El escape TOML se reforzó después con sus regresiones específicas, vet y nueva compilación de los tres destinos. Logs y alcance exactos en task-13.md.

| Destino final | Bytes | Objetivo < 15 MB |
| --- | ---: | --- |
| Windows / amd64 | 12 818 944 | Cumple |
| Linux / amd64 | 12 431 520 | Cumple |
| macOS / arm64 | 11 708 978 | Cumple |

SHA256 Windows: `3D5C9A2A56EFB564C9365DDBABED3BCB3EBA10198878836B3C033AE533BCC5EE`. Binarios sin comprimir con `-s -w`; Go 1.27.3 y CGO desactivado.

**Clientes reales:** Claude Code 2.1.318 y Codex CLI 0.167.0 completaron cinco llamadas MCP sobre un repositorio sintético, con código 0. Ambos también consultaron Qapture real y generaron `## Evidencias` a partir de los casos sintéticos confirmados `EJ-001` y `EJ-002`, cada uno con su imagen de resultados adjunta. Son resúmenes raster, no capturas del escritorio. No se modificó código de Qapture, no se publicaron PR y no se cambió la configuración global de los clientes.

**Pendiente para cerrar:** Computer Use falla antes de inicializar con `failed to write kernel assets ... (os error 3)`, incluso tras reset. No se pudo verificar la interfaz de Claude Desktop ni Antigravity ni sus confirmaciones visuales. La cancelación se probó con un cliente SDK real y un proceso de agente sintético, pero queda observarla desde los clientes de escritorio. Los jobs remotos de GitHub Actions tampoco se ejecutaron desde esta sesión. La fase no se declara cerrada hasta completar esos puntos.
