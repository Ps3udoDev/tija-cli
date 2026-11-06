# Usar Tija con Qapture

Tija revisa el repositorio y prepara el PR. Qapture administra los casos y sus evidencias. Registra ambos servidores MCP en el mismo cliente: el agente coordina sus herramientas; Tija no se conecta a Qapture.

## Preparación

1. Inicializa un repositorio propio con `tija init --yes`. Para validar la integración, utiliza datos sintéticos.
2. Ejecuta `tija mcp config <cliente> --dir <ruta-absoluta-del-repo>` y aplica la configuración que imprime. Los clientes disponibles son `claude-code`, `codex`, `claude-desktop`, `cursor` y `antigravity`.
3. Registra también el servidor de Qapture siguiendo su configuración local. Mantén Qapture abierto y habilita Lectura y Escritura para crear casos y adjuntar evidencias.
4. Comprueba `qapture_status` y confirma el proyecto mediante `get_project` antes de escribir. No copies credenciales de Qapture en el repositorio ni en los reportes.

La raíz de Tija queda fija al arrancar. Usa `--dir` en Claude Desktop y otros clientes que puedan iniciar desde el directorio de usuario. Ninguna herramienta acepta un campo `dir`.

## PR con evidencias: escenario A

Solicita el prompt MCP `documentar_pr`, opcionalmente con `qaptureProject` y `mode` (`review`, `quality` o `deep`). Está embebido en el binario: no instala archivos en `.tija/`.

El flujo comprueba `repo_status` y `doctor`, ejecuta `review_run` y `quality_check`, confirma el proyecto de Qapture y crea o reutiliza casos mediante `list_cases` y `create_case`. Adjunta las evidencias con `add_evidence`. El tester decide las capturas; no captures pantallas con datos personales o de clientes sin avisar. `export_excel` solo se usa si el tester pide exportar la matriz.

El agente construye referencias con los identificadores y estados que Qapture devolvió y llama `pr_prepare`:

```json
{
  "agent": "none",
  "evidenceRefs": [{
    "source": "qapture",
    "project": {"id": "proyecto-sintetico", "name": "Portal de prueba"},
    "cases": [{"id": "caso-sintetico", "code": "EJ-014", "title": "Inicio con MFA", "status": "Aprobado"}],
    "exportPath": "C:\\qa\\matriz.xlsx",
    "notes": "Datos sintéticos de validación"
  }]
}
```

Tija reemplaza `## Evidencias` con un bloque reproducible. Solo conserva `matriz.xlsx` como `exportName`: descarta la ruta completa, no abre el archivo ni comprueba que exista. `exportName` es de salida; no lo envíes como sustituto libre de `exportPath`.

Tija valida fuente `qapture`, proyecto obligatorio, contenido mínimo, identidad de casos y límites de texto (200 caracteres, sin saltos de línea), 50 casos por referencia y 10 referencias. Devuelve:

```json
{"evidenceValidation":{"structural":"passed","externalExistence":"not_checked"}}
```

Eso **no confirma la existencia externa**. Contrasta los ids, códigos y estados con `get_project` y `get_case` de Qapture. Los nombres del ejemplo son ilustrativos, no registros reales.

Revisa el borrador antes de publicar. `pr_create` solo publica con `write: true` y cuando no hay problemas; el usuario debe pedirlo explícitamente. Sin `write` devuelve un plan, o un error con los problemas del borrador. Las evidencias de un HEAD, rama o target anteriores no se reutilizan para otro commit. Una lista explícita `evidenceRefs: []` elimina las guardadas.

El CLI también admite `tija pr prepare --evidence archivo.json` y `tija pr create --evidence archivo.json`; el archivo contiene `{"evidenceRefs":[...]}` y puede tener BOM UTF-8.

## Permisos, tiempos y privacidad

`quality_check`, `review_run` y `pr_prepare` guardan sesiones locales y temporales del engine; no requieren `write`. `quality_fix` y `comments_clean` requieren `write: true` para modificar código; los cambios inseguros requieren además `unsafe: true`. `workflow_run` verifica **todos** los permisos antes de ejecutar un paso: un workflow que publica requiere `write` y `publish`.

Las anotaciones ayudan al cliente a mostrar confirmaciones; el servidor comprueba los permisos aunque el cliente las ignore. El código de salida va en `_meta["tija/exitCode"]`; un exit 1 de review indica hallazgos y no un fallo de la herramienta.

Los clientes reciben progreso si envían `progressToken`. La cancelación de un formateador puede dejar cambios aplicados: revisa el árbol y la respuesta del servidor. Un cliente que cancela puede mostrar solamente su error de contexto, sin recibir el reporte final. Codex debe usar `tool_timeout_sec = 900` y `startup_timeout_sec = 20`, incluidos en la configuración generada.

El agente puede ver código y evidencias en la misma conversación. Los títulos y notas de casos se incluyen en el cuerpo del PR. Revisa datos personales, secretos y datos del cliente antes de adjuntar o publicar. Tija no captura pantallas ni instala runtimes o dependencias de Qapture.

## Plataformas y clientes

| Entorno | Alcance |
| --- | --- |
| Windows | Plataforma obligatoria para validar Tija con Qapture |
| Linux | Tija MCP soportado; colaboración con Qapture experimental |
| macOS | Tija MCP soportado; Qapture fuera de alcance |
| Claude Code, Codex, Claude Desktop | Clientes obligatorios de cierre en Windows; resultados reales en el handoff de fase 5 |
| Cursor | Formato de configuración disponible; cliente no validado |
| Antigravity | Configuración experimental hasta comprobar stdio en el cliente real |

La configuración impresa no demuestra por sí sola compatibilidad del cliente.

En el escenario B, los hallazgos High o Critical pueden convertirse en casos «Verificar: …». Mantén separado el hallazgo de código de la prueba funcional y no marques un caso como aprobado hasta ejecutarlo.
