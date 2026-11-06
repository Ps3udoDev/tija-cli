# Tarea 6 — Herramientas que escriben en el repo: `quality_fix` y `comments_clean`

Fecha: 2026-11-05. Rama: `feat/phase5-mcp`.

**Estado: COMPLETADA.** Pasan los tests focalizados y la suite completa.

## Qué se hizo

**MCP (`internal/mcpserver/tools_repo.go`):** las dos herramientas tienen efecto `EffectRepo`, así que llevan `destructiveHint: true`, `openWorldHint: false` y comparten la exclusión de escrituras.

- **`quality_fix`** — entrada `{ kinds?: ["lint"|"format"], write }`.
  - Sin `kinds` corre lint y format, igual que `tija lint --fix`. Un `kinds` vacío o con `test` es un error de uso.
  - **Sin `write`:** devuelve `{ status: "needs-write", would: [...] }` y no llama al engine.
  - **Con `write`:** corre `RunQuality` en `FixMode` y devuelve `status: "applied"` junto con todos los campos del `QualityReport`, al mismo nivel (embebido): el JSON es el del CLI más `status`.
- **`comments_clean`** — entrada `{ paths?, all?, agent?, write, unsafe? }`.
  - **Sin `write`:** pasa `Diff: true` al engine y devuelve el plan con el diff de cada archivo, sin escribir nada.
  - **`unsafe` sin `write`:** error de uso, `unsafe requires write: true`.
  - **`agent: "none"`:** equivale a `NoAgent`.
- **Cancelación:**
  - `quality_fix` devuelve `cancelled; changed files: <lista>` o `cancelled; no files were changed`;
  - `comments_clean` cancelado antes de escribir devuelve `cancelled; no files were changed`.
- **Hook `schemaAdjuster` (`tools.go`):** permite que un tipo de salida corrija el esquema inferido. El SDK v1.8.0 valida la salida contra `outputSchema` y marcaba como obligatorios los campos del reporte embebido. `qualityFixResult` deja solo `status` como obligatorio.

**Engine:**

- **`CleanComments`:**
  - **Antes del bucle de escritura:** si `ctx` está cancelado, devuelve `cancelled before writing: context canceled` (envuelve `context.Canceled`) sin escribir archivos ni la sesión `comments.json`.
  - **Durante el bucle:** no se interrumpe.
  - **Verificación posterior con Biome:** solo corre si `ctx` sigue vivo. Si se cancela antes o durante esa verificación, `PostCheck = "skipped: cancelled"` y no se restaura nada.
- **`RunQuality`:**
  - una vez cancelado `ctx`, los pasos restantes se marcan `skipped`;
  - en `FixMode`, `ChangedFiles` se calcula con `context.WithoutCancel`, así que un fix cancelado igual informa lo que cambió;
  - solo en `FixMode` la cancelación devuelve un error (`cancelled: context canceled`) junto con el reporte. `CheckMode` no cambia.

## Verificación

| Comando | Resultado |
|---|---|
| `go test -count=1 ./internal/engine/ -run 'QualityFixCancel\|CancelBeforeWrite\|CancelDuringPostCheck\|RunQuality\|TestClean'` | PASS |
| `go test -count=1 ./internal/mcpserver/` | PASS (15,3 s) |
| `go vet` y `go test -count=1 -timeout 15m` sobre todos los paquetes salvo `./dist` | PASS, exit 0; log en `dist/phase5-task6-tests.log` (ignorado por git) |
| `gofmt -l internal/` | Vacío |

**Tests nuevos:**
- **Engine:**
  - `TestRunQualityFixCancelReportsChangedFiles`: un formateador falso escribe y se bloquea; el error envuelve `Canceled`, aparece `src/a.ts` y no corren más pasos;
  - `TestCommentsCleanCancelBeforeWrite`: no cambia ningún archivo ni se guarda sesión;
  - `TestCommentsCleanCancelDuringPostCheck`: los archivos quedan `written` y `PostCheck` vale `skipped: cancelled`.
- **MCP:**
  - `TestRepoToolsRequireWrite`: sin `write` no cambian archivos versionados y hay diff en el plan;
  - `TestCommentsCleanWriteApplies`;
  - `TestUnsafeWithoutWriteIsUsageError`;
  - `TestQualityFixRejectsUnknownKind`;
  - `TestQualityFixCancelReportsChangedFiles` (interno: llama al método `qualityFix`, porque un cliente que canceló nunca ve la respuesta);
  - `TestCancelledErrorWithoutChanges`.
- **Golden:** regenerado con `quality_fix` y `comments_clean`.

## Decisiones y límites

- **`unsafe` sin `write`** es un error de uso en MCP, como pide el plan. El CLI sigue aceptando `--unsafe` sin `--write` como reporte.
- **Al cancelar `comments_clean` durante la verificación posterior,** la herramienta no devuelve error: los archivos ya se escribieron, y el resultado lo dice con `postCheck: "skipped: cancelled"`.
- **Los tests de cancelación por MCP son internos.** Con el SDK, un cliente que cancela recibe el error de su propio contexto y no la respuesta, así que el texto `cancelled; …` solo se puede comprobar llamando al handler. La cancelación real desde un cliente se valida en la tarea 13.
- **Aviso sobre `dist/`:** contiene `dist/phase5-task2-mcp_test.go`, una copia que dejó otra sesión. Como `go` trata `./dist` como un paquete, ese archivo rompe `go vet ./...` y `go test ./...` en local. Está ignorado por git, así que no afecta a CI; aquí se excluyó con `go list ./... | grep -v /dist$`. Conviene borrarlo o renombrarlo a `.txt`.
