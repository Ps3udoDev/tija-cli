# Formatos de la skill

## `manifest.json` (entrada de `collect_activity.py`)

```json
{
  "periodo": { "desde": "2026-10-01", "hasta": "2026-10-31" },
  "autores": ["usuario-github", "Nombre Apellido", "correo@dominio.com"],
  "grupos": [
    { "nombre": "ACME", "repos": [
      { "nombre": "Claims API", "path": "C:\\ruta\\ClaimsAPI" },
      { "nombre": "Tija", "path": "C:\\ruta\\Tija", "hasta": "2026-11-01" }
    ] }
  ]
}
```

- `autores`: nombres y correos de git y login de GitHub; se comparan exactos, sin distinguir mayúsculas.
  Si se omite, se usan `git config` del repo actual y la cuenta de `gh`. Revisa `git shortlog -sne` si la
  persona tiene varias identidades.
- `desde`/`hasta` por repo sustituyen el periodo global para ese repo.

## `reporte.json` (entrada de `build_activity_report.py`)

```json
{
  "meta": {
    "title": "Reporte de actividades",
    "periodo": "1 al 31 de octubre de 2026",
    "responsable": "Nombre Apellido",
    "cliente": "ACME",
    "nombre_corto": "ReporteActividades",
    "fecha": "2026-11-01",
    "version": "1.0"
  },
  "resumen": "Texto corto para la caja inicial.",
  "grupos": [
    {
      "nombre": "ACME",
      "periodo": "1-31 oct.",
      "tope_horas": 58.5,
      "proyectos": [
        {
          "nombre": "Claims API",
          "repo": "acme/claims-api",
          "destacado": "Opcional: caja resaltada bajo el nombre del proyecto.",
          "actividades": [
            {
              "desde": "2026-10-27",
              "hasta": "2026-10-29",
              "actividad": "Nombre corto",
              "descripcion": "Qué se trabajó, en una o dos frases.",
              "horas": 1.5,
              "fuente": "estimado",
              "estado": "Completado"
            }
          ]
        }
      ]
    }
  ],
  "notas": ["Texto pequeño al final del reporte."]
}
```

| Campo | Valores |
|---|---|
| `hasta` | Opcional. Sin él, la actividad es de un solo día. |
| `horas` | Número o `null`. `null` muestra `—` y no suma al total. |
| `fuente` | `estimado` (commits, se muestra `~`), `reportado` (lo dijo el usuario, sin `~`), `archivos` (fechas de archivos sin commitear, se muestra `~`). Obligatoria si hay horas. |
| `estado` | `Completado`, `En curso`, `En revisión`, `Propuesta`, `Pausado`. Se editan en `activity-template.json`. |
| `tope_horas` | Opcional. `tope_horas` del grupo en `actividad.json`; el motor avisa si se supera. No cuenta `fuente: archivos`. |

Las actividades se ordenan por fecha al generar el PDF.
Texto inline igual que la skill base: `**negrita**`, `*cursiva*`, `` `código` ``.
Sin `meta.output`, el archivo se llama `<cliente>_<nombre_corto>_<YYYYMMDD>_v<version>.pdf`.
