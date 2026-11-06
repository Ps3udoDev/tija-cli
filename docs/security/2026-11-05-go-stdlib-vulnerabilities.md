# Actualización de seguridad de la biblioteca estándar de Go

Fecha: 2026-11-05. Estado: corrección implementada con Go 1.27.3.

## Hallazgo y alcance

Se reprodujo el análisis de `govulncheck@v1.9.0 -show verbose ./...` con `GOTOOLCHAIN=go1.27.1` sobre el código de `50d9bb1`, antes de incorporar MCP. El resultado fue: **0 vulnerabilidades alcanzables**, una vulnerabilidad en paquetes importados y 13 en módulos requeridos. El comando terminó con código 0; los hallazgos de paquete/módulo no equivalen a llamadas vulnerables del programa.

Los identificadores observados fueron:

| Alcance | Identificadores | Corrección en la rama 1.27 |
| --- | --- | --- |
| Paquete importado, Windows | GO-2026-6604 | 1.27.2 |
| Módulo stdlib | GO-2026-6809 | 1.27.3 |
| Módulo stdlib | GO-2026-6617, GO-2026-6613, GO-2026-6612, GO-2026-6611, GO-2026-6610, GO-2026-6609, GO-2026-6608, GO-2026-6607, GO-2026-6605, GO-2026-6603, GO-2026-6600, GO-2026-6599 | 1.27.2 |

[GO-2026-6809 / CVE-2026-111943](https://pkg.go.dev/vuln/GO-2026-6809) afecta la verificación de certificados en `crypto/x509`. La actualización incorpora el rechazo de certificados intermedios que dejaron de ser confiables.

[GO-2026-6604 / CVE-2026-56857](https://pkg.go.dev/vuln/GO-2026-6604) afecta operaciones de `os.Root` con junctions en Windows. El análisis identificó el paquete importado `internal/syscall/windows`, sin llamadas alcanzables a los símbolos vulnerables.

## Acción y comprobación

- `go.mod` requiere Go 1.27.3; CI y release toman la versión desde ese archivo.
- CI ejecuta `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` en Linux; release lo ejecuta antes de compilar/publicar.
- El README documenta Go 1.27.3 como requisito mínimo para compilar.
- El análisis con Go 1.27.3 sobre la misma base de código terminó con **`No vulnerabilities found.`**, código 0.

La repetición posterior a la interrupción se ejecutó en una copia temporal de `50d9bb1`, modificando únicamente la directiva de Go, para aislar esta corrección del SDK MCP que se incorporaba simultáneamente. La comprobación del SDK y su tamaño pertenece a la tarea 2; estos resultados no la reemplazan. La ejecución remota de GitHub Actions queda pendiente del siguiente push autorizado.
