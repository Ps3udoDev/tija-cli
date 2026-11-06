# Tarea 7 — `pr_create` (acción externa)

Fecha: 2026-11-05. Rama: `feat/phase5-mcp`.

**Estado: COMPLETADA.** Pasan los tests focalizados y la suite completa.

## Qué se hizo

**Engine (`internal/engine/pr_create.go`):**

- **Reutilización del borrador en `PlanCreatePR`.**
  - Primero normaliza las `EvidenceRefs` recibidas: si son inválidas, el error sale antes de cualquier trabajo.
  - El borrador guardado se reutiliza solo si coinciden HEAD, rama y target **y** además se cumple una de dos cosas:
    - no se recibieron referencias (`EvidenceRefs == nil`); en ese caso se usan las guardadas;
    - las recibidas, ya normalizadas, son iguales a las guardadas. Se comparan por su JSON, así que `nil`, `[]` y listas de casos vacías equivalen.
  - En cualquier otro caso se prepara un borrador nuevo solo con las referencias recibidas, o sin ninguna. Las referencias guardadas nunca pasan a otro HEAD. Una lista vacía explícita (`[]`) quita las evidencias guardadas.
- **Revalidación de lo guardado (`revalidateStoredEvidence`).**
  - Las referencias que vienen de `pr.json` se tratan como datos guardados, no como entrada nueva.
  - Si traen `exportPath` (alguien volvió a meter una ruta), o un `exportName` que no es un nombre de archivo, o ya no pasan las reglas de §8.1, el borrador se descarta y se prepara de nuevo.
  - Esto cubre el pendiente que dejó la tarea 4.
- **Evidencias del borrador reutilizado:** `## Evidencias` se vuelve a armar desde las referencias guardadas. Una edición a mano de esa sección no sobrevive, y las de las demás secciones sí. El reporte incluye `evidence` y `evidenceValidation`.
- **`CreateReport`** suma dos campos:
  - `number`: sale de `/pull/<n>` al final de la URL, o 0 si no hay número;
  - `evidence`: las referencias normalizadas que cita el cuerpo publicado.

**MCP (`internal/mcpserver/tools_external.go`):** `pr_create` es `EffectExternal` (`destructive` y `openWorld`). Entrada: `{ draft?, evidenceRefs?, write }`.

| Caso | Resultado |
|---|---|
| Sin `write` | `{ status: "needs-write", question, ...CreatePlan }` (`report`, `push`, `existing`, `existingBase`, `assign`). Solo corre `gh auth status` y `gh pr list`; no hace push |
| El borrador tiene problemas (con o sin `write`) | `isError: true`, con el texto `pull request draft has problems; nothing was published`, y `structuredContent` con `status: "draft-has-problems"`, el plan y `report.problems` |
| Con `write` y sin problemas | `ApplyCreatePR`, que devuelve `{ status: "published", url, pushed, updated, assigned, number, evidence }` |

**`tools.go`:**
- **`resultError`:** un error que el handler devuelve como resultado `isError: true` sin perder la salida estructurada. El SDK v1.8.0 conserva `structuredContent` cuando el resultado ya trae `IsError`.
- **`adjustSchema` de `prCreateResult`:** deja solo `status` como obligatorio, igual que `quality_fix`.

## Verificación

| Comando | Resultado |
|---|---|
| `go test -count=1 ./internal/engine/ -run 'PRCreate\|PRNumber\|CreatePR'` | PASS (65 s) |
| `go test -count=1 ./internal/mcpserver/` | PASS (24 s) |
| `go vet` y `go test -count=1 -timeout 15m` sobre todos los paquetes salvo `./dist` | PASS, exit 0; log en `dist/phase5-task7-tests.log` |

**Tests nuevos:**
- **Engine:**
  - `TestPRCreateDiscardsStaleEvidence`, con tres variantes: el mismo HEAD reutiliza A; el mismo HEAD con B deja B y no A; un commit nuevo sin referencias no cita A ni B;
  - `TestPRCreateExplicitEmptyEvidenceDropsStored`;
  - `TestPRCreateTamperedStoredEvidenceRegenerates`;
  - `TestPRCreateReturnsNumberAndEvidence`: número 42, `exportName` sin ruta, y el cuerpo publicado sin `C:\Users`;
  - `TestPRNumber`.
- **MCP:**
  - `TestPRCreateWithoutWriteDoesNotPublish`: sin `pr create`/`pr edit` y sin rama en el remoto;
  - `TestPRCreateRefusesDraftWithProblems`: título no convencional, `isError`, problemas en la salida estructurada y nada publicado ni pusheado;
  - `TestPRCreatePublishesWithEvidence`: push real a un remoto bare, número 42 y evidencias en el cuerpo;
  - `TestPRCreateAnnotations`.
- **Golden:** regenerado; incluye el `outputSchema` de `pr_create`.

## Decisiones y límites

- **Un borrador con problemas es `isError` también sin `write`.** El plan solo lo pedía con `write`. Se adelantó porque, si no, `status: "needs-write"` le haría creer al agente que basta con autorizar, cuando en realidad nunca se publicaría.
- **`pr_create` no tiene parámetro `agent`.** Si hay que preparar un borrador nuevo, se usa el default del manifest, como `tija pr create`. El flujo recomendado (prompt de la tarea 10) es `pr_prepare` y después `pr_create`.
- **El CLI `tija pr create --json`** ahora incluye `number` y, si hay referencias, `evidence`. Ningún test existente dependía de la forma anterior.
- **La confirmación humana** sigue a cargo del cliente, a partir de las anotaciones `destructive` y `openWorld`. Lo que muestra cada cliente se valida en la tarea 13.
- **Aviso sobre `dist/`:** `dist/phase5-task2-mcp_test.go` sigue rompiendo `go test ./...` en local (ver task-06.md).
