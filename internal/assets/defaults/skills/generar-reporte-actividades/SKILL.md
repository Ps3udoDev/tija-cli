---
name: generar-reporte-actividades
description: Se usa cuando se pide un reporte de actividades, de trabajo realizado o de horas de un periodo (mes, semana, rango de fechas) sobre uno o varios repositorios o proyectos, en PDF con plantilla de marca — incluye "dame el reporte de actividades de octubre", "qué se hizo este mes en ACME", "agrupa estos repos en ACME", "reporte consolidado de actividades en PDF".
---

# Generar reporte de actividades

Convierte el trabajo real de una persona en un periodo en un PDF agrupado por **grupo → proyecto → actividad**,
con fecha, descripción, tiempo y estado. Usa la marca de `generar-informe-pdf` (header, logo y footer), que debe
estar instalada junto a esta skill. No usa navegador. Sin ficha técnica ni código de documento.

## Reglas del contenido

- **Solo el trabajo del usuario.** Filtra por sus identidades (`autores`). Incluye a otras personas únicamente si
  lo pide.
- **Lo que dice el usuario manda.** Su contexto (despliegues, reuniones, horas) entra como actividad o corrige lo
  inferido, con `fuente: reportado`. Si se corrige al dictar ("dos horas, bueno, una"), vale la corrección.
- **Cada actividad tiene evidencia:** commits, PRs, archivos o palabras del usuario. Sin evidencia, no se incluye.
- **Tareas en curso que anuncie el usuario:** `estado: "En curso"`, `horas: null`, aunque no tengan commits.

## Pasos

1. **Definir el alcance.**
   - Periodo: "octubre" va del 1 al 31. Si el mes está en curso, termina hoy.
   - Grupos: los que indique el usuario ("agrupa ACME estos repos"). Un repo o proyecto puede extender su periodo con `hasta`.
2. **Recolectar.** Escribe `docs/reportes/actividades-<periodo>/manifest.json` (ver
   `references/report-schema.md`) y ejecuta:
   ```bash
   python <skill>/scripts/collect_activity.py <carpeta>/manifest.json --fetch
   ```
   En Windows, si `python` abre la Store, usa `py`. Genera `actividad.md` (para leer) y `actividad.json`.
   Revisa los avisos: un repo sin acceso a PRs o sin identidades coincidentes da un reporte incompleto.
3. **Redactar `<carpeta>/reporte.json`.**
   - **Actividad = funcionalidad o entrega,** no commit: agrupa commits y PRs relacionados, entre 3 y 10 por proyecto.
   - **Descripción:** qué se logró, en lenguaje de negocio; versión o PR si ayuda.
   - **Estado:** PR fusionado o commit integrado → `Completado`; PR abierto → `En revisión`; rama sin PR → `En curso`; documento sin aprobar → `Propuesta`.
   - **Trabajo sin commitear:** usa `sin_commitear` de `actividad.md`; incluye solo lo que puedas identificar leyendo los archivos.
4. **Asignar tiempos.**
   - Parte de `horas_por_dia` de cada repo y reparte entre las actividades de ese día.
   - Las horas `reportado` sustituyen a la parte estimada del mismo trabajo; no se suman encima.
   - Ajusta para que la suma de horas de cada grupo basadas en commits o reportadas no supere su `tope_horas`:
     trabajar en dos repos a la vez cuenta una sola vez.
   - Sin commits, estima por fechas de archivos con `fuente: archivos`.
5. **Generar y verificar.**
   ```bash
   python <skill>/scripts/build_activity_report.py <carpeta>/reporte.json --preview <carpeta>/preview
   ```
   Abre todos los PNG y comprueba: encabezados huérfanos, filas cortadas, totales y que no falte ningún proyecto.
   Borra `preview/` al terminar.
6. **Entregar:** ruta del PDF, total de horas, y qué fechas u horas son inferencia tuya para que el usuario las confirme.

## Errores comunes

| Síntoma | Causa / solución |
|---|---|
| Faltan commits del primer día | `git --since` con solo fecha usa la hora actual; el recolector ya pide un día de margen. No lo sustituyas por `git log` manual. |
| Lista de PRs vacía en un repo con PRs | Repo renombrado o movido; el recolector resuelve el nombre canónico. Revisa `aviso_prs`. |
| Faltan commits del usuario | Identidad distinta: compara con `git shortlog -sne --all` y amplía `autores`. |
| `AVISO: ... superan el tope` | Hay trabajo simultáneo contado dos veces: reduce las horas `estimado` de ese grupo. |
| `No se encontro la skill base` | `generar-informe-pdf` debe estar en la misma carpeta de skills. |
| `Falta ReportLab` | `pip install -r <generar-informe-pdf>/requirements.txt` |
