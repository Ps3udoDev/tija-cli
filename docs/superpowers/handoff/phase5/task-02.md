# Tarea 2 — SDK, arranque MCP y control de tamaño

**Estado: COMPLETADA; pruebas focalizadas y suite integrada de las tareas 1–5 aprobadas.**

Fecha: 2026-11-05. Rama: `feat/phase5-mcp`.

## Resultado del control

El control inicial detuvo el trabajo porque el SDK oficial `github.com/modelcontextprotocol/go-sdk v1.8.0` superó el límite original de 10 000 000 bytes. El usuario autorizó después estos criterios: objetivo **menor de 15 000 000 bytes**, alerta técnica a partir de **20 000 000 bytes** y máximo permitido **25 000 000 bytes** (bloqueo si se supera). Se reanudó con el SDK oficial; no se activó el protocolo alternativo. Las mediciones actualizadas cumplen el objetivo.

| Destino | Archivo | Bytes | Control |
| --- | --- | ---: | --- |
| Windows / amd64 | `tija-windows-amd64.exe` | 12 487 168 | Cumple objetivo |
| Linux / amd64 | `tija-linux-amd64` | 12 099 744 | Cumple objetivo |
| macOS / arm64 | `tija-darwin-arm64` | 11 404 866 | Cumple objetivo |

Compilador: `go1.27.3 windows/amd64`. `CGO_ENABLED=0`. Binarios sin comprimir, compilados con `go build -ldflags='-s -w'`. Los tres builds terminaron correctamente. Se encuentran en `dist/phase5-gate/`, ignorado por Git. Linux y macOS se compilaron de forma cruzada: esto no verifica su ejecución en esos sistemas.

La medición actualizada incluye las herramientas de lectura que el subagente de la tarea 3 preparó durante la reanudación. Antes de registrarlas, las medidas eran Windows 12 321 792, Linux 11 935 904 y macOS 11 268 658 bytes.

## Cambios realizados

- Se agregó el SDK v1.8.0 y sus dependencias mediante `go get` y `go mod tidy`.
- `internal/mcpserver/server.go` y `root.go`: servidor stdio, instrucciones en español, diagnósticos a un writer y resolución de la raíz Git antes de aceptar solicitudes. Se anuncian herramientas, prompts y logging, sin recursos.
- `internal/cli/mcp.go` y registro en `internal/cli/root.go`: `tija mcp serve`, `--dir` y `--log-level` (por defecto `warn`). Los errores del arranque se emiten en stderr aun si se hereda `--json`.
- Se añadió el texto completo del `LICENSE` upstream v1.8.0 a `THIRD_PARTY_NOTICES.md`: transición MIT / Apache-2.0 y nota de CC-BY-4.0 para documentación. La transición no se presenta como elección de licencia para todo el SDK.
- Se agregaron tests de resolución de raíz desde un subdirectorio o cwd, error fuera de un repositorio, inicialización en memoria, flags y protocolo sobre el binario real.
- Se aplicó `gofmt` y se eliminó la comparación innecesaria del método `root.Find` con nil, encontrada por `go vet` durante la pausa anterior.

No se añadió `Deps.RunStep`: el tipo `workflows.Executor` pertenece a la tarea 8 y todavía no existe. El servidor deja preparado un canal de exclusión para que la tarea 3 implemente la espera cancelable; esta tarea no incorpora herramientas de escritura.

## Verificaciones ejecutadas

```powershell
go get github.com/modelcontextprotocol/go-sdk@v1.8.0
go mod tidy

$env:CGO_ENABLED='0'
$env:GOOS='windows'; $env:GOARCH='amd64'
go build -ldflags='-s -w' -o dist/phase5-gate/tija-windows-amd64.exe ./cmd/tija
$env:GOOS='linux'; $env:GOARCH='amd64'
go build -ldflags='-s -w' -o dist/phase5-gate/tija-linux-amd64 ./cmd/tija
$env:GOOS='darwin'; $env:GOARCH='arm64'
go build -ldflags='-s -w' -o dist/phase5-gate/tija-darwin-arm64 ./cmd/tija
Remove-Item Env:GOOS
Remove-Item Env:GOARCH

go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
go test ./internal/mcpserver/ ./internal/cli/ -run 'ResolveRoot|Initialize|ServeOutside|StdoutIsJSONRPC|ServeInvalidLog' -count=1
go test ./...
go vet ./...
```

- `govulncheck` terminó con código 0: `No vulnerabilities found.` Corresponde al árbol con el SDK incorporado, antes del registro de herramientas de lectura de la tarea 3.
- Los tests focalizados pasan en Windows, también después de registrar las herramientas de lectura.
- La integración compila y lanza el binario real desde un directorio ajeno al repositorio, envía `initialize`, `notifications/initialized` y `tools/list`, comprueba cada línea de stdout como JSON-RPC 2.0, confirma la negociación del protocolo `2025-06-18` y verifica la salida limpia al cerrar stdin.
- El cierre integrado posterior ejecutó `go test -count=1 ./... -timeout 15m`: PASS, exit 0. Salida en `dist/phase5-tests.log`; también pasaron `go vet ./...`, formato y `govulncheck`. Las mediciones finales con las ocho herramientas figuran en [HANDOFF.md](HANDOFF.md).
- `go vet ./...` terminó con código 0 después de corregir `root.Find`.

## Límites y continuación

- La tarea 5 amplió el test de stdout con un proceso hijo que imprime durante `quality_check`; pasó en el cierre integrado.
- La ejecución nativa en Linux y macOS y las pruebas con clientes reales quedan para las tareas 12–13.
- No se hizo commit de esta tarea durante este trabajo; el agente principal coordina los commits para no mezclar cambios concurrentes.
- El bloqueo por tamaño queda resuelto por el nuevo criterio autorizado. La parada antes de la tarea 6 permanece vigente.
