# Tarea 5 — Herramientas MCP de efecto local

Fecha: 2026-11-05. Rama: `feat/phase5-mcp`.

**Estado: COMPLETADA; pruebas focalizadas PASS. El cierre integrado de la rama se registra en HANDOFF.md.**

## Qué se hizo

- Se registraron `quality_check`, `review_run` y `pr_prepare` como herramientas de efecto local. Comparten la exclusión cancelable de escrituras de la tarea 3 y declaran entradas cerradas, esquemas de salida y anotaciones no destructivas.
- Los handlers llaman directamente al engine existente, conservando los defaults del CLI, y trasladan sus eventos a progreso MCP. `quality_check` utiliza `CheckMode`, `Verbose: nil` y traduce el tipo público `test` al tipo interno `tests`.
- Se validan agentes, modo, umbral, tipos de calidad y timeout positivo antes de ejecutar el engine. `agent: "none"` mantiene el camino determinista. Las referencias de evidencia las valida y normaliza el engine de la tarea 4.
- La salida estructurada mantiene el JSON del CLI, sin envoltorios adicionales ni cambios al CLI. El código de salida está en `_meta["tija/exitCode"]`, según la adaptación de contrato de la tarea 3; review conserva su campo `exit`. Los hallazgos con exit 1 son resultados normales, con `isError: false`.
- Se amplió el snapshot a ocho herramientas, incluidos los esquemas de `review_run` y `pr_prepare`. La copia de las cinco herramientas anterior a esta tarea quedó temporalmente en `dist/phase5-task3-tools.golden.json` para permitir commits independientes.

## Pruebas y resultados

```powershell
go test ./internal/mcpserver ./internal/cli -run 'Parity|ExitOne|NeverTouchTracked|EvidenceValidationBlock|LocalToolInput|LocalToolsListed|ToolsGolden|StdoutIsJSONRPC' -count=1
```

PASS: `internal/mcpserver` (13,745 s) y `internal/cli` (4,854 s).

Las pruebas comparan llamadas MCP reales en memoria con `cli.ExecuteWith` y su salida `--json` sobre el mismo repositorio temporal. Cubren:

- `quality_check` frente a `check`, conservando todos los campos salvo la normalización explícita de `startedAt` y `durationMs` entre ejecuciones.
- `review_run` en modo `deep` frente al CLI con el mismo agente falso; se comprueba que ambos pases del agente terminan correctamente.
- `pr_prepare` frente al CLI con agente falso y archivo `--evidence`, incluidos cuerpo, referencias normalizadas y ausencia de rutas privadas completas.
- Un hallazgo High determinista con exit 1 sin convertirlo en fallo MCP.
- La existencia de los artefactos de sesión y un `git status --porcelain --untracked-files=all` idéntico antes y después de las tres herramientas.
- El bloque `evidenceValidation` con `structural: passed` y `externalExistence: not_checked`.
- Entradas inválidas rechazadas sin invocar el agente ni modificar el árbol visible, además de las anotaciones y esquemas de las herramientas locales.

La prueba `TestServeStdoutIsJSONRPCOnly` lanza el binario de producción y envía `initialize`, `tools/list` y `tools/call quality_check`. Para esa llamada compila un ejecutable de calidad falso, nativo para la plataforma, que imprime marcadores en stdout y stderr. Comprueba que la ejecución pasa, que ambos marcadores quedan capturados en el log local y que cada línea del stdout del servidor sigue siendo JSON-RPC válido. No depende de una instalación de npm.

Después se reforzaron las aserciones de paridad de los agentes con `go test ./internal/mcpserver -run 'ReviewRunParity|PRPrepareParity' -count=1`: PASS (9,454 s).

## Alcance y límites

Se ejecutaron pruebas locales en Windows; la ejecución nativa en Linux y macOS corresponde al cierre y CI de tareas posteriores. Los fixtures ejercitan los adaptadores del engine y verifican que no cambian archivos versionados; no se ha añadido un sandbox para scripts configurados por el repositorio.

Los artefactos persistentes se guardan en `.tija/session/`. Los temporales de calidad y PR existentes en el engine se mantienen como permite el spec §7; las descripciones los explican. No se modificó el engine para esta tarea. No se implementaron herramientas de escritura en el repositorio, publicación ni workflows (tareas 6–8).

No se ejecutó otra suite global desde este subagente ni se creó un commit. El agente principal integra los commits y ejecuta la verificación conjunta con logs persistidos.
