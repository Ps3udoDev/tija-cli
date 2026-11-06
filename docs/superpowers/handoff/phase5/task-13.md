# Tarea 13 — Verificación de cierre

Fecha: 2026-11-06. Rama: `feat/phase5-mcp`.

**Estado: PARCIAL.** Código de tareas 9–12 completo, revisión 6–8 corregida y pruebas locales aprobadas. Faltan la validación visual en Claude Desktop y Antigravity y la ejecución remota de la matriz de CI; no se declara cerrada toda la fase.

## Verificación automática

| Comprobación | Resultado |
| --- | --- |
| `go test -count=1 ./... -timeout 15m` | PASS, exit 0; engine 289,792 s, CLI 69,737 s, MCP 56,041 s |
| `go vet ./...` | PASS |
| `gofmt -l internal` | Vacío |
| `govulncheck@v1.9.0 ./...` | `No vulnerabilities found.` |
| `bash scripts/check-binary-size.sh` | PASS, tres destinos bajo el objetivo |
| `git diff --check` | PASS |

Tras endurecer el escape de caracteres de control en la configuración TOML se repitieron los tests `ClientConfig|MCPConfigCmd|DoctorMCP`: PASS (MCP 0,374 s, CLI 0,444 s, engine 1,471 s), vet y la compilación de los tres destinos. El cambio solo afecta a imprimir configuración.

Logs ignorados: `dist/phase5-close-tests.log`, `dist/phase5-close-tests.exit` (0), `dist/phase5-close-vet.log`, `dist/phase5-close-vuln.log`, `dist/phase5-final-sizes.log`, `dist/phase5-review68-tests.log` y `dist/phase5-final-mcp-tests.log`.

| Binario | Bytes | Objetivo < 15 000 000 |
| --- | ---: | --- |
| Windows / amd64 | 12 818 944 | Cumple |
| Linux / amd64 | 12 431 520 | Cumple |
| macOS / arm64 | 11 708 978 | Cumple |

Go 1.27.3, `CGO_ENABLED=0`, `-ldflags='-s -w'`, sin compresión. Artefactos en `dist/binary-size/`. SHA256 Windows: `3D5C9A2A56EFB564C9365DDBABED3BCB3EBA10198878836B3C033AE533BCC5EE`.

## Clientes reales en Windows

Las pruebas utilizan un repositorio sintético independiente dentro de `validation/client-repo`, con rama propia, commits locales, script de test Node y archivos `.tija/` inicializados. No se usó un repositorio de un cliente. La configuración se generó con `tija mcp config --dir` y se aplicó a cada invocación, sin editar los registros MCP globales del usuario.

| Cliente | Resultado observado |
| --- | --- |
| Claude Code 2.1.318 | PASS: cinco llamadas MCP reales (`repo_status`, `doctor`, `quality_check`, `review_run`, `pr_prepare`); cliente exit 0, sin denegaciones de permiso. Descubrió `mcp__tija__documentar_pr`. |
| Codex CLI 0.167.0 | PASS: las mismas cinco llamadas, todas con `_meta["tija/exitCode"] == 0`; `review_run` en modo `deep`, `agent: none`, con timeout de herramienta 900 s y arranque 20 s. |
| Claude Desktop 3.6556.2.0 | Instalación confirmada por metadatos de Windows; configuración JSON generada. Ejecución y presentación visual de herramientas/anotaciones pendientes. |
| Antigravity 2.29.0.0 | Ejecutable y procesos presentes; configuración sigue experimental. No se confirmó el flujo en su interfaz ni su soporte stdio. |
| Cursor | No validado; solo formato de configuración. |

En los dos clientes que pasaron se verificaron los eventos reales `tool_use`/`mcp_tool_call`, no solo el resumen del modelo. `quality_check` ejecutó realmente `npm run test`; review determinista terminó sin hallazgos y el PR se preparó sin problemas. `doctor` mostró un aviso no bloqueante de ausencia de framework de test reconocido en la fixture, aunque su script explícito se ejecutó correctamente. No se cambió ese comportamiento previo del detector.

Evidencia cruda, ignorada: `validation/claude-code.jsonl`, `validation/codex-client.jsonl` y sus archivos `.exit` (0). Se limitaron los clientes a las herramientas necesarias; no se habilitó publicación ni edición de código.

## Escenario A con Qapture

- Se confirmó el estado y los permisos de Qapture y se creó un proyecto exclusivo de QA con datos sintéticos, sin cambiar su código.
- Casos `EJ-001` (Claude Code) y `EJ-002` (Codex CLI): aprobados después de las cinco llamadas reales de cada cliente.
- Se adjuntó una imagen de resultados a cada caso y se comprobó mediante `get_case` que conserva id, código, estado y evidencia. Son resúmenes raster de resultados reales, **no capturas de pantalla**. No se capturaron pantallas del usuario ni se exportó una matriz sin solicitud del tester.
- Ambos clientes se ejecutaron otra vez con **Tija y Qapture registrados simultáneamente**. Cada uno llamó realmente `qapture_status`, `get_project`, `get_case` y `pr_prepare`, reutilizando su caso confirmado. Ambos terminaron con exit 0.
- Cada borrador contiene la sección `## Evidencias` con el código, título y estado devueltos por Qapture, y `evidenceValidation.structural: passed`, `externalExistence: not_checked`.
- El contraste de existencia se hizo con Qapture; Tija sigue validando solo estructura. El árbol de la fixture no muestra cambios versionados y no se publicó ningún PR.

Evidencia ignorada: `validation/claude-scenario-a.jsonl`, `validation/codex-scenario-a.jsonl`, archivos `.exit` (0), `validation/evidence-refs.json`, `validation/claude-result.png` y `validation/codex-result.png`. Los ids internos permanecen en la evidencia ignorada; aquí solo se registran los códigos sintéticos.

Para las invocaciones se reutilizó exclusivamente la configuración de transporte Qapture ya existente del usuario. Las copias privadas temporales se eliminaron al terminar; la configuración original quedó intacta. No se guardaron credenciales en los commits.

## Cancelación y permisos

`TestMCPClientCancellationKillsNestedAgent` pasó: un cliente SDK real envía cancelación mientras un proceso de agente sintético real está bloqueado. Se comprueba el error de contexto del cliente y que termina el proceso. Esto verifica el transporte y el engine, sin pagar ni depender de un modelo anidado.

Las regresiones de tareas 6–8 verifican permisos antes de todos los pasos, ausencia de publicación sin write/con problemas, evidencias de otro HEAD descartadas, cambios reportados tras cancelar y rechazo de planes que quedan obsoletos antes de aplicar. GitHub está simulado y los remotos de tests son bare locales. No se intentó crear un PR real.

Queda pendiente cancelar desde la interfaz de cada cliente y observar sus confirmaciones/anotaciones `destructive` y `openWorld`. La configuración aislada de las pruebas CLI no demuestra cómo una interfaz visual presenta esos permisos.

## Bloqueo del entorno y próximos pasos

Computer Use no pudo inicializar: `failed to write kernel assets: El sistema no puede encontrar la ruta especificada. (os error 3)`. Se intentó con Node/sky, con CUA y después de resetear el kernel; falló antes de obtener el inventario de ventanas. No se inventaron resultados de Claude Desktop ni Antigravity, ni se modificaron sus ajustes a ciegas.

Para cerrar la fase:

1. Restablecer la automatización de escritorio o hacer la comprobación manual de Claude Desktop y Antigravity sobre la misma fixture: cinco herramientas, prompt/escenario Qapture, anotaciones y cancelación.
2. Registrar el soporte stdio real de Antigravity; quitar la marca experimental solo si se confirma.
3. Ejecutar la matriz de GitHub Actions en Ubuntu, Windows y macOS. Las compilaciones cruzadas locales no sustituyen esos jobs. No se hizo push desde esta sesión.

Los tests de prompt y sus snapshots sí verifican `prompts/list` y `prompts/get` mediante el SDK; no sustituyen una interacción visual con el menú de prompts de cada cliente.
