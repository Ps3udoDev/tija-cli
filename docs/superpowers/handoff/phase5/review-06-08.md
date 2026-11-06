# Revisión de las tareas 6–8

Fecha: 2026-11-06. Rama: `feat/phase5-mcp`.

Estado: **revisadas y corregidas**; verificaciones focalizadas aprobadas. El cierre global consta en task-13.md.

La revisión independiente comenzó con un subagente. Detectó dos problemas, pero la falta de créditos del espacio interrumpió su trabajo antes de corregirlos. El agente principal reprodujo los casos, completó la revisión y aplicó las correcciones con regresiones en los paquetes productivos. No se atribuye una aprobación posterior ficticia al subagente.

## Hallazgos corregidos

1. **Alto — cancelación que ocultaba cambios.** `RunQuality` comparaba solamente hashes de las rutas presentes al terminar. Un borrado no tiene hash, y restaurar un archivo previamente sucio a HEAD elimina su entrada de `git status`: ambos podían producir `ChangedFiles: []` y el mensaje falso `no files were changed`. Ahora se compara la unión de las rutas iniciales y finales, incluyendo existencia y contenido. La consulta posterior usa un contexto independiente de la cancelación y acotado a diez segundos. Regresión: `TestCancelledChangesShouldBeReported`, variantes borrado y restauración a HEAD.
2. **Medio — proveedor explícito ignorado.** `comments_clean` anunciaba `agent`, pero solo traducía `none`. Ahora transporta `CleanRequest.Agent`; el engine valida el proveedor, respeta la selección y rechaza un proveedor explícito no instalado sin sustituirlo por otro. El CLI comparte el camino mediante `clean comments --agent`. Regresiones: `TestCleanExplicitAgentSelection`, `TestCleanExplicitAgentUnavailable` y `TestCommentsCleanHonorsExplicitAgent`.
3. **Alto — plan obsoleto entre planificación y aplicación.** La reutilización de un borrador comprobaba HEAD al planificar, pero un commit o cambio de rama/target posterior podía dejar obsoleto ese plan antes del push. `CreatePlan.Head` conserva el commit internamente (sin alterar el JSON), y `ApplyCreatePR` vuelve a comprobar HEAD, rama y target antes de publicar. Un cambio exige preparar de nuevo y no hace push ni llamadas de publicación. Regresión: `TestApplyCreatePRRejectsChangedSnapshot`, con cambios de commit, rama y manifest. Esto es una comprobación previa a aplicar; no introduce un bloqueo entre procesos Git externos.

## Decisiones validadas

- `write` sigue siendo obligatorio en las herramientas que aplican código o publican; las anotaciones no sustituyen la comprobación.
- Los problemas del borrador se devuelven con `isError` también sin `write`: autorizarlos no arreglaría un borrador inválido.
- Se distinguen referencias ausentes y una lista explícitamente vacía; esta última elimina evidencias guardadas. La comparación por JSON de referencias normalizadas evita diferencias irrelevantes entre listas de casos vacías y omitidas.
- Las referencias persistidas se revalidan mediante un camino separado de la entrada nueva; no se aceptan rutas completas reintroducidas en metadatos.
- Los workflows comparten executor y runner con el CLI, fijan `Deps.Dir`, usan stdin vacío y buffers propios. Los permisos de todos los pasos se revisan antes de ejecutar cualquiera; `publish` se exige aparte de `write`.
- El canal de exclusión se toma una sola vez para todo el workflow; los pasos CLI no vuelven a adquirir el turno MCP.
- `resultError`, los esquemas ajustados para reportes opcionales y el override de `json.RawMessage` preservan resultados estructurados y la compatibilidad del SDK.

## Comprobaciones

`go test ./internal/engine ./internal/mcpserver ./internal/workflows ./internal/cli -run 'ApplyCreatePRRejectsChangedSnapshot|CommentsCleanHonorsExplicitAgent|CancelledChanges|CleanExplicitAgent|Workflow|RequireWrite|PRCreate' -count=1 -timeout 5m` pasó: engine 41,611 s, MCP 20,256 s, workflows 0,384 s, CLI 13,078 s. Log: `dist/phase5-review68-tests.log`.

Se añadió además `TestMCPClientCancellationKillsNestedAgent`: cliente SDK real en memoria, notificación de cancelación y proceso de agente sintético real, sin llamadas a modelos. Confirma que el cliente cancela y que termina el proceso anidado. Pasó junto con los tests focalizados de tareas 9–11.

Se renombró la copia temporal `dist/phase5-task2-mcp_test.go` a `.go.txt` y se trasladaron las regresiones de revisión al paquete del engine. Los artefactos ignorados ya no introducen paquetes extra que rompan `go test ./...`. Se conservó su contenido de referencia.

No hubo push, PR ni publicaciones reales durante estas correcciones. Los remotos de los tests son repositorios bare temporales y las respuestas de `gh` son simuladas.
