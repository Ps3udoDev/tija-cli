# Tarea 4 — Referencias estructuradas de evidencia

Fecha: 2026-11-05. Rama: `feat/phase5-mcp`.

**Estado: COMPLETADA.** Implementación y verificaciones focalizadas terminadas; el agente principal realiza la verificación integrada y el commit.

## Qué se hizo

- Se añadieron `EvidenceRef`, `EvidenceProject`, `EvidenceCase`, `EvidenceValidation` y `NormalizeEvidence` al engine.
- Se valida fuente `qapture`, proyecto obligatorio, contenido mínimo, identidad de casos, textos de hasta 200 caracteres sin saltos de línea, hasta 50 casos y 10 referencias. Los errores identifican el campo y comienzan con `invalid evidenceRefs:`.
- `exportPath` se reduce al nombre de archivo reconociendo `/` y `\` en cualquier sistema. La ruta completa se descarta antes de preparar resultados o persistir el borrador. Nunca se abre el archivo referido. `exportName` se calcula desde la ruta; no se acepta un nombre de salida como sustituto libre de una ruta de entrada.
- `PreparePR` valida antes de recopilar contexto o invocar al agente; referencias inválidas no guardan archivos ni llaman al agente.
- Los borradores del agente y los deterministas reciben una sección reproducible de evidencias. Se conserva el primer encabezado, se reemplaza su contenido y se eliminan secciones duplicadas de `Evidencias`, incluso con diferencias de mayúsculas, sin eliminar las demás secciones.
- `PRReport` devuelve las referencias saneadas y `evidenceValidation: {structural: "passed", externalExistence: "not_checked"}` cuando hay referencias. `DraftMeta` persiste esas referencias junto con los metadatos existentes. Sin referencias se conserva el comportamiento anterior.
- Se añadió `--evidence <archivo.json>` a `pr prepare` y `pr create`, con soporte para BOM UTF-8 y código 2 si falta el archivo, su JSON es inválido o las referencias no cumplen las reglas. `create` recibe las referencias mediante su `Prepare` existente.

## Verificación

| Comando | Resultado |
| --- | --- |
| `go test ./internal/engine ./internal/cli -run '^TestPRPrepareNoAgentJSON$' -count=1` | PASS |
| `go test ./internal/engine ./internal/cli -run 'Evidence\|EvidenciasSection\|InvalidEvidence\|FullPath' -count=1 -timeout 3m` | PASS; engine 11.370 s, CLI 6.464 s |
| `go vet ./internal/engine ./internal/cli` | PASS |
| `git diff --check` sobre los archivos de esta tarea | PASS |

Los tests cubren límites válidos e inválidos, índices en los errores, copia sin modificar la entrada, distinción `nil`/array vacío, rutas Windows y mixtas, formato exacto, reemplazo con y sin agente, secciones duplicadas y CRLF, ausencia de escritura/agente al rechazar referencias y ausencia de la ruta completa tanto en archivos como en la respuesta JSON. El CLI se compara con el cuerpo producido por el engine y se verifica que `create` prepare evidencias antes de pedir confirmación, sin publicar.

La prueba de rutas mixtas se ejecutó en Windows con una función independiente del sistema operativo. Su ejecución nativa en Linux y macOS queda para CI.

## Límites para las tareas siguientes

- No se cambió `pr_create.go`, no se implementó reutilización segura de borradores ni se publicaron PR. La tarea 7 debe comprobar HEAD, rama, target y referencias antes de reutilizar, y descartar evidencias viejas conforme al plan.
- `NormalizeEvidence` valida **entrada nueva**: sus referencias normalizadas tienen `ExportName` y ya no tienen `ExportPath`. La tarea 7 debe revalidar los metadatos guardados por un camino explícito para referencias persistidas, sin aceptar `exportName` libre como entrada del cliente ni volver a pasar referencias normalizadas como si fueran entrada nueva.
- Se conserva `nil` frente a `[]` en `NormalizeEvidence`, para que la política posterior pueda distinguir ausencia de referencias de una lista explícitamente vacía. Los campos de salida usan `omitempty` para conservar la forma previa del JSON cuando no hay referencias.
- La existencia externa de proyectos, casos o evidencias **no se comprueba**. No se consulta Qapture.
- Las tareas 6–8 no se iniciaron.
