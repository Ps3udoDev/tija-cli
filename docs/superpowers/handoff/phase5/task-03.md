# Tarea 3 — Núcleo MCP y herramientas de lectura

Estado: **completada**. Fecha: 2026-11-05. Rama: `feat/phase5-mcp`.

## Cambios

- Registro tipado de `repo_status`, `doctor`, `repo_rules`, `repo_context` y `skills_list`, sin parámetro `dir`, con entradas cerradas y esquemas de salida.
- Cuatro clases de efecto y sus anotaciones. Las cinco herramientas de esta tarea son de lectura.
- Serialización por servidor mediante un canal de capacidad uno. La espera comprueba el contexto antes y después de adquirir el turno; una cancelación devuelve `cancelled; no files were changed` sin ejecutar el handler ni esperar a que termine otra escritura.
- Eventos de progreso enviados únicamente si existe un `progressToken`, con avance monotónico y mensajes de etapa.
- Errores de entorno convertidos por el SDK en resultados `isError`; exit 1 conserva un resultado normal.
- Snapshot de nombres, entradas y anotaciones; incluye el esquema de salida de `repo_status`. Las tareas que añadan herramientas deben regenerarlo mediante `go test ./internal/mcpserver -run ToolsGolden -update`.
- `repo_rules` reutiliza manifest, `prrules.Types`, carga de packs y `rules.Load`; devuelve los patrones compilados como texto, además de las reglas de PR y arquitectura. `repo_context` llama directamente a `engine.CollectPRContext`.

## Adaptaciones explícitas del contrato

- El JSON existente del CLI no siempre incluye `exitCode`: `repo_status` no lo incluye y review usa `exit`. Para conservar la paridad literal, el helper MCP escribe el código en `_meta["tija/exitCode"]`; no modifica los reportes del engine ni el JSON del CLI.
- `skills_list` devuelve `{ "skills": [...] }` para clientes con MCP 2025-06-18, que esperan un objeto en `structuredContent`. El array interior coincide con `tija skills list --json`.
- En SDK v1.8.0, `mcp.AddTool` rechaza argumentos incompatibles con el esquema mediante `isError: true`, no mediante error JSON-RPC. Se conserva ese comportamiento nativo. Una herramienta inexistente sí produce error JSON-RPC.
- La inferencia predeterminada del SDK declara los mapas como objetos no anulables, pero los reportes existentes pueden serializar mapas nil como `null` (por ejemplo `PRContext.Manifest.Tija.Installed`). El esquema de salida se deriva de los tipos con un override para permitir `null` en mapas. Los valores del engine permanecen intactos.

## Verificación ejecutada

Comando final de esta tarea:

```powershell
go test ./internal/mcpserver -run 'ReadTools|ToolsGolden|Parity|ToolError|WriteMutex|ProgressOnly|ToolExit|RepoRules'
```

Resultado: **PASS**, paquete `internal/mcpserver` (6.874 s de tests).

Cobertura comprobada:

- Cinco herramientas de lectura listadas, `readOnlyHint`, entradas sin `dir` y `additionalProperties: false`, esquemas de salida presentes.
- Snapshot reproducible de herramientas.
- Paridad campo a campo con el CLI real mediante `cli.ExecuteWith`: `repo_status` y array interior de `skills_list`, sobre el mismo repositorio temporal y las mismas dependencias.
- Error real de `doctor` al recibir una raíz de detección inaccesible: `isError`, texto de entorno preservado y sin error JSON-RPC.
- Cancelación de una segunda escritura mientras la primera mantiene el turno, tanto en el handler como a través de una sesión SDK en memoria con contexto de recepción cancelado. No se ejecuta la segunda escritura.
- Cero notificaciones de progreso sin token; mensajes y avance monotónico con token.
- Exit 1 normal, exit 2 como error, rechazo de argumentos ajenos al esquema y de nombres inexistentes.
- Las cinco lecturas no cambian archivos versionados ni crean archivos sin seguimiento.
- `repo_rules` devuelve reglas de arquitectura, el patrón regex como texto y rutas con `/`; rechaza una expresión regular inválida.

La suite global y la medición integrada del binario se coordinan con el agente raíz al cerrar las tareas 1–5. No se han implementado las tareas de alto riesgo 6–8.

## Archivos

- `internal/mcpserver/tools.go`
- `internal/mcpserver/tools_read.go`
- `internal/mcpserver/progress.go`
- `internal/mcpserver/tools_test.go`
- `internal/mcpserver/parity_test.go`
- `internal/mcpserver/testdata/tools.golden.json`
- Registro de herramientas en `internal/mcpserver/server.go`, coordinado con la tarea 2.

## Revisión posterior

Las pruebas de protocolo usan el SDK en memoria. El arranque del binario real por stdio pertenece a la tarea 2; su ampliación con procesos de calidad pertenece a la tarea 5. La validación manual con clientes queda para la tarea 13.
