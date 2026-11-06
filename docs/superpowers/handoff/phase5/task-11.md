# Tarea 11 — Configuración MCP y diagnóstico

Fecha: 2026-11-06. Estado: **COMPLETADA**.

Se añadió `ClientConfig` y `tija mcp config <cliente> [--dir]`. Imprime la ruta de `os.Executable()` y argumentos del servidor; no modifica la configuración global. Ofrece Claude Code, Codex, Claude Desktop, Cursor y Antigravity (experimental). Un cliente desconocido termina con exit 2 y muestra la lista admitida.

Codex incluye `tool_timeout_sec = 900` y `startup_timeout_sec = 20`. TOML usa literales cuando son seguros y strings básicos JSON compatibles con TOML si la ruta contiene apóstrofos o controles. El comando de Claude Code usa comillas de PowerShell en Windows y comillas POSIX en otros sistemas, evitando interpretar caracteres especiales de las rutas. La configuración de Desktop y Cursor es JSON generado con `json.MarshalIndent`.

`doctor` añade el check informativo `mcp`, siempre `ok`, con el comando de configuración visible tanto en texto como en JSON. No cambia el código de salida de los demás checks.

Pruebas `ClientConfig`, `MCPConfigCmd` y `DoctorMCP` aprobadas; cubren roundtrip de rutas Windows con espacios y apóstrofos, opciones TOML, JSON de los clientes, directorio opcional, ruta real del ejecutable y cliente desconocido. Se ejecutaron junto con las regresiones de revisión: engine 5,231 s, CLI 0,408 s y MCP 3,119 s. Verificación global en task-13.md.

Las configuraciones generadas se utilizaron después en pruebas reales aisladas de Claude Code y Codex CLI. No se afirma compatibilidad de un cliente solo porque su formato se imprima correctamente. El subagente asignado a esta tarea no pudo iniciar por falta de créditos; la implementó y verificó el agente principal.
