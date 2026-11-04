# Task 13 — Verificación y entrega

Estado: implementación local verificada; aceptación externa backend pendiente por autorización. No se marca completa la aceptación de fase17 del spec.

## Validaciones locales finales

Ejecutadas en Windows, Go1.27.1, con `GOCACHE = Join-Path $env:TEMP 'tija-phase4-go-cache'` porque el cache por defecto de usuario no es escribible desde el sandbox.

| Comando | Resultado |
|---|---|
| `gofmt -l .` | Sin salida |
| `go vet ./...` | Exit0, sin issues |
| `go test -count=1 ./... -timeout 10m` | Exit0, todos los paquetes pasan, sin reutilizar resultados cacheados |
| `go build -ldflags="-s -w" -o $env:TEMP/tija-phase4-final.exe ./cmd/tija` | Exit0;7.593.472 bytes, menor a10.000.000 |
| `git diff --check` | Sin errores |

Log completo de la suite: `validation/go-test-all.txt`. El paquete engine ejecutó sus tests en232.156s; los tests de integración crean repos temporales y procesos Git reales. Una corrida diagnóstica anterior con timeout global2m se interrumpió al vencer ese límite durante un test PR que solo llevaba2s; no fue una regresión del test. La corrida final de10m pasó. Las corridas redundantes con cache prolongado se detuvieron; el log final sin cache es la evidencia autoritativa.

SHA256 del binario medido: `3F83971205EFC276E30ED65AAD2EADC3ECC479577F10A237F2C8CC724738BAE4`. Construido con código hasta b947003 y documentación en preparación; no se entrega un binario de otra versión. Go emitió un warning no fatal al intentar escribir el stat cache de la versión del módulo en el cache restringido de usuario; build y vet terminaron0. No se cambiaron go.mod ni go.sum.

Se agregó `*.go text eol=lf` en .gitattributes: el checkout core.autocrlf=true hacía que gofmt listara prácticamente todo el repo solo por CRLF. Normalizar el árbol no produjo diferencias de contenido en los Go preexistentes; `git add --renormalize -- '*.go'` refrescó el índice sin staged diff adicional.

## Integraciones verificadas

- Prueba `TestPrePRStopsAtReview`: workflow embebido exacto, herramientas de calidad simuladas exitosas, sin agentes instalados; limpieza0, check0, review1 por High determinista, no corre pr prepare. Incluido en la suite final.
- Binario real en repo temporal limpio, con PATH reducido a Git y System32: `review --no-check --no-pr-comments --json` termina0 y advierte ausencia de agente.- En el mismo repo/commit limpio, `pr prepare --no-agent --no-check --json` termina0 y agrega la cita real de review bajo Pruebas.- Pruebas de sesión vieja/dirty, cita determinista/agente, evidencia CRLF, cinco descartes, quoted/renamed paths, archivo borrado, glob Windows, concurrencia3, síntesis disputada, reintento y resultados parciales pasan.
- `TestReviewSkipsSymlinkContents` pasó en este Windows: enlaces nuevos y staged quedaron fuera del diff revisable; no se leyó el sentinel externo.

## Aplicaciones reales y semillas

Frontend real (React/Vite, privado): tres modos exit0; review1Low, quality1Medium+1Low, deep1Medium+2Low. Seed sin agente2High (2/2 checks), seed deep6High+2Medium+1Low y bug invertido detectado por ambos pases. Todos los17 hallazgos mostrados auditados: archivo/rango/evidencia correctos; descartados0. Node confirma bug con2 tests fallidos.

Backend real (.NET, privado): los tres modos exit2 por timeout de Claude2m por intento;0 hallazgos,0 descartados. Semilla determinista3High en3 líneas, ambos checks detectados3/3 y auditados. El ejecutable .NET de prueba falla con bug invertido y pasa al aplicar la corrección de control. Seed deep retiene los3 checks y termina2 por timeout30s.
Los clones locales aislados protegen tanto el working tree como metadata .git de los repos originales. Las llamadas reales usaron no-check/no-pr-comments: no se afirma build/test global de las aplicaciones ni integración GitHub real. Esas rutas sí tienen tests con fakes. Los13 JSON quedaron UTF8 sin BOM y se parsearon correctamente. La evidencia cruda se conserva solo en local (`validation/`, ignorada por git) porque contiene código privado.

## Bloqueo explícito de aceptación

El reintento backend a6m con ejecución elevada fue rechazado automáticamente antes de lanzar: posible envío del diff privado del backend a Claude sin autorización explícita del usuario. Se pidió esa autorización en la conversación; no llegó respuesta afirmativa durante esta entrega y no se hicieron llamadas adicionales posteriores al rechazo. No se eludió la decisión.

Pendientes para cerrar la aceptación: autorización explícita del envío backend a Claude; repetir secuencialmente review/quality/deep con timeout suficiente; auditar hallazgos y confirmar detección del bug sembrado. La implementación y el handoff quedan commiteados para revisión sin afirmar que ese criterio pasó.

Handoff principal: `HANDOFF.md` en esta carpeta. Se actualizó también `docs/superpowers/handoff/HANDOFF.md` con el enlace a fase4.
