# Tarea 8 — Runner de workflows compartido y `workflow_run`

Fecha: 2026-11-06. Rama: `feat/phase5-mcp`.

**Estado: COMPLETADA.** Pasan los tests focalizados y la suite completa.

## Qué se hizo

**`internal/workflows/run.go` (nuevo):**

- **`Executor`, `Options` y `Run`:** el bucle de pasos se movió aquí desde `internal/cli/run.go`, sin cambiar el comportamiento:
  - agrega `--json` y `--yes` una sola vez;
  - captura la salida y la guarda como JSON o como string;
  - con `ContinueOnError` sigue tras un fallo; si no, se detiene en el primero;
  - `Exit` es el máximo de los pasos;
  - imprime `> <run>` en modo texto.
- **`Effect`** (`read`, `local`, `repo` y `external`, con `String()`) y **`Classify(step)`:**

  | Paso | Clase |
  |---|---|
  | `status`, `doctor` | `read` |
  | `review`, `check`, `test`, `pr prepare`, `lint` sin `--fix`, `clean comments` sin `--write` | `local` |
  | `format`, `lint --fix`, `clean comments --write` | `repo` (unsafe si tiene `--unsafe`) |
  | `pr create` | `external` |

  - `--flag=false` cuenta como apagado.
  - `pr create --edit` es un error («opens an editor and needs a terminal»).
  - Cualquier comando que no esté en `Allowed` es un error.
- **`Requirements(w)`** devuelve `Needs{Write, Unsafe, Publish, Steps []StepEffect}` antes de ejecutar nada. `StepEffect.Effect` es el nombre de la clase; `Class`, su valor tipado, no se serializa.

**CLI:**

- `RunWorkflow` delega en `workflows.Run` a través de `StepExecutor(deps, in)`, el executor que corre cada paso con `ExecuteWithContext`. La TUI y `tija run` lo siguen usando igual.
- **`Deps.Dir` (nuevo):** si está definido, reemplaza a `os.Getwd()` como el directorio desde el que los comandos resuelven el repositorio. Las 13 llamadas pasan por `deps.workDir()`. Sin este cambio, un paso lanzado desde `mcp serve --dir X` habría corrido sobre el directorio del proceso, no sobre X; el test de integración lo demuestra fallando cuando se quita `Dir`.
- **`mcp serve`** inyecta `RunStep: StepExecutor(Deps{Env, Dir: root, IsTTY: false}, stdin vacío)`. Los pasos nunca abren la TUI ni leen la terminal.

**MCP (`internal/mcpserver/tools_workflow.go`):** `workflow_run` es `EffectExternal` (`destructive` y `openWorld` fijos). Entrada: `{ name, write?, unsafe?, publish? }`.

1. **Carga:** `workflows.Load(root)` y `Validate`. Un nombre desconocido da `unknown workflow "<x>"`; un archivo inválido da error.
2. **Permisos:** `Requirements` los compara con los recibidos. Si falta alguno, devuelve `{ status: "needs-permission", needs: ["write"|"unsafe"|"publish"], plannedSteps: [{run, effect, unsafe?}] }` y **no ejecuta ningún paso**.
3. **Ejecución:** con todos los permisos, corre `workflows.Run` con `JSON` y `Yes`, envía progreso por paso y devuelve `{ status: "completed", name, steps, exit }`, que es el mismo `WorkflowReport` de `tija run --json`.
   - Exit 1 es un resultado normal.
   - Exit 2 devuelve `isError` y conserva el reporte, gracias al `resultError` de la tarea 7.

**`tools.go`:** el esquema de salida describe `json.RawMessage` (la salida capturada de cada paso) como cualquier valor JSON. Antes se infería como un array de bytes y el SDK rechazaba la salida. El golden solo agregó `workflow_run`; los esquemas de las demás herramientas no cambiaron.

## Verificación

| Comando | Resultado |
|---|---|
| `go test -count=1 ./internal/workflows/` | PASS |
| `go test -count=1 ./internal/mcpserver/` | PASS (26,7 s) |
| `go test -count=1 ./internal/cli/ -run 'WorkflowRunPins\|TestRun'` | PASS; los tests existentes de `tija run` pasan sin cambios |
| `go vet` y `go test -count=1 -timeout 15m` sobre todos los paquetes salvo `./dist` | PASS, exit 0; log en `dist/phase5-task8-tests.log` |
| `govulncheck@v1.9.0 ./internal/... ./cmd/...` | `No vulnerabilities found.` |
| `go build -ldflags="-s -w"` windows/amd64, `CGO_ENABLED=0` | 12 761 088 bytes (objetivo < 15 000 000) |

**Tests nuevos:**
- **`workflows`:**
  - `TestClassifySteps`: tabla con `--fix=false`, `--edit`, `rm -rf /` y `run pre-pr`;
  - `TestRequirementsPrePR`: lee el `pre-pr.yaml` embebido y da solo `write`;
  - `TestRunMatchesOldBehaviour`;
  - `TestRunPlainPrintsSteps`.
- **MCP:**
  - `TestWorkflowRunNeedsPermissionRunsNothing`: `pre-pr` sin `write` hace 0 llamadas y muestra `clean comments --write` como `repo`;
  - `TestWorkflowRunPublishRequired` y `TestWorkflowRunUnsafeRequired`;
  - `TestWorkflowRunReadOnlyNeedsNothing`;
  - `TestWorkflowRunAuthorizedRunsAll`;
  - `TestWorkflowRunRejectsEditorStep`;
  - `TestWorkflowRunStepErrorKeepsReport`;
  - `TestWorkflowRunUnknown`.
- **CLI:** `TestWorkflowRunPinsStepsToRoot`, con el executor real y el cwd fuera del repo. Se comprobó que falla sin `Dir` (mutación manual, revertida).

## Decisiones y límites

- **Se reutiliza el executor del CLI en lugar de llamar al engine paso por paso,** como dice el spec (§3.2 «Runner de workflows compartido»). Así no se traducen flags a requests por segunda vez.
- **Los permisos se comprueban todos antes de ejecutar,** aunque el plan permitía detenerse a mitad del workflow. Ningún workflow corre parcialmente por falta de un permiso.
- **`publish` se pide aparte de `write` solo en `workflow_run`.** `pr_create` como herramienta suelta sigue pidiendo solo `write`.
- **La herramienta se registra siempre.** Si el servidor se crea sin `RunStep`, que solo pasa en tests, `workflow_run` devuelve `workflow_run is not available in this server`.
- **Exclusión de escrituras:** `workflow_run` toma el turno de escritura durante todo el workflow. Sus pasos corren por el CLI, no por MCP, así que no vuelven a pedir el turno y no hay bloqueo.
- **Aviso sobre `dist/`:** `dist/phase5-task2-mcp_test.go` sigue rompiendo `go test ./...` en local (ver task-06.md).
