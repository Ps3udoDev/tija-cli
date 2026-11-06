# Tarea 10 — Prompt embebido `documentar_pr`

Fecha: 2026-11-06. Rama: `feat/phase5-mcp`.

**Estado: COMPLETADA; pruebas focalizadas PASS. El agente principal registra el cierre integrado.**

## Qué se hizo

- Se añadió `prompts/documentar_pr.md`, embebido en el binario mediante `go:embed`, y su registro MCP en `prompts.go`. No se instala en `.tija/` ni cambia `DefaultsVersion`.
- `prompts/list` anuncia `documentar_pr` con dos argumentos opcionales: `qaptureProject` y `mode`. `prompts/get` devuelve un mensaje de rol `user` con el flujo en español.
- El modo predeterminado es `review`; se admiten `review`, `quality` y `deep`, y un modo desconocido devuelve un error en inglés. Si falta el proyecto, el texto usa «el que indique el tester». El proyecto se cita como dato en una sola línea, y la sustitución no vuelve a interpretar marcadores incluidos en sus valores.
- El escenario A guía `repo_status` y `doctor`, revisión/calidad, diagnóstico de Qapture y confirmación del proyecto, búsqueda o creación de casos, adjuntos, exportación solo si el tester la pide, `pr_prepare` con referencias reales y revisión del borrador. `pr_create` con `write: true` exige una solicitud explícita del usuario; invocar el prompt no autoriza publicar.
- El prompt explica la privacidad de capturas y títulos de casos, respeta los permisos de Qapture y distingue la validación estructural de Tija de la existencia externa (`not_checked`). El escenario B convierte hallazgos High/Critical en propuestas de casos «Verificar: …» sin dar por probado su resultado funcional.

## Contrato y verificación

El snapshot de `prompts/list` se guarda en `testdata/prompts.golden.json`, separado de `tools.golden.json`: son métodos MCP distintos y así se conserva la forma existente del contrato de herramientas. Se regenera con `go test ./internal/mcpserver -run DocumentarPR -update`.

| Comando | Resultado |
| --- | --- |
| `go test ./internal/mcpserver -run DocumentarPR -update -count=1` | PASS; generó el snapshot de metadatos |
| `go test ./internal/mcpserver -run DocumentarPR -count=1` | PASS, 0,430 s |
| `go vet ./internal/mcpserver` | PASS, exit 0 |
| `git diff --check` sobre los archivos de esta tarea | PASS |

Las pruebas utilizan una sesión MCP real en memoria para `prompts/list` y `prompts/get`; comprueban argumentos opcionales, descripción y snapshot, rol `user`, contenido y nombres reales de las herramientas de Tija mediante `tools/list`, defaults, modos admitidos y rechazados, sustitución única y ausencia de archivos instalados al obtener el prompt.

No se ejecutaron herramientas de Qapture, agentes externos ni publicación de PR desde esta tarea. Las pruebas del flujo en clientes reales pertenecen a la tarea 13. No se lanzó otra suite global ni se modificó el índice de Git; el agente principal integra los commits.
