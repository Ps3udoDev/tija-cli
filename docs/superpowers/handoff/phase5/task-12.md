# Tarea 12 — CI, tamaño y documentación

Fecha: 2026-11-06. Estado: **COMPLETADA en código y comprobaciones locales**; ejecución remota de CI pendiente.

- La matriz de CI conserva Ubuntu, Windows y macOS y ejecuta la suite completa, incluidos protocolo stdio, snapshots y cancelación, con timeout de quince minutos.
- Un paso Linux ejecuta `scripts/check-binary-size.sh`, que compila Windows/amd64, Linux/amd64 y macOS/arm64 con `CGO_ENABLED=0` y `-ldflags='-s -w'`, sin comprimir. Objetivo < 15 000 000 bytes; aviso al superar el objetivo; alerta desde 20 000 000; fallo solo si se superan 25 000 000 bytes.
- `.gitattributes` fija LF para scripts `.sh`, evitando que `core.autocrlf=true` de Windows invalide el script al hacer un checkout nuevo.
- El README documenta comandos, trece herramientas, clases de efecto, permisos, raíz fija, progreso, cancelación, timeouts, formatos de salida y privacidad.
- `docs/mcp/qapture.md` contiene «Usar Tija con Qapture»: registro de ambos MCP, flujo guiado, esquema de referencias, alcance estructural de la validación, privacidad y matriz de plataformas/clientes. No incorpora proyectos de clientes ni credenciales.
- Los avisos completos del SDK ya estaban incorporados; se conservaron sin recortar la licencia.

`bash -n scripts/check-binary-size.sh` y la ejecución real con Git Bash pasaron. Tamaños: Windows 12 818 944, Linux 12 431 520 y macOS 11 708 978 bytes, todos bajo el objetivo. Log `dist/phase5-final-sizes.log`. La ejecución nativa en Linux/macOS corresponde a los jobs remotos; la compilación cruzada no la sustituye.

La búsqueda de datos privados en README, docs/mcp y notices no encontró nombres de clientes. La palabra inglesa `claims` aparece en dos cláusulas de la licencia Apache: son coincidencias legales esperadas, no datos de un cliente, y no se eliminó texto de atribución por ese falso positivo.

No se hizo push para disparar GitHub Actions. El cierre local de tests, formato, vet y vulnerabilidades se registra en task-13.md. El agente principal implementó esta tarea tras agotarse los créditos de subagentes.
