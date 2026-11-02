---
name: generar-informe-pdf
description: Se usa cuando se pide generar un informe, propuesta técnica, diagnóstico, reporte o entregable en PDF con plantilla de marca (header con logo, ficha técnica y footer) — incluye "genera el informe en PDF", "arma la propuesta de X con estas imágenes", "exporta esto como informe". Genera el PDF sin Chromium/navegador, 100 % Python.
---

# Generar informe PDF

Convierte contenido (texto, tablas, código, imágenes SVG/PNG) en un PDF con la plantilla de marca.
El motor es `scripts/build_report.py` (ReportLab): **no usa Chromium, navegadores, LibreOffice
ni Word**. Ejemplo completo: `examples/informe-ejemplo/`.

## Estructura

```
generar-informe-pdf/
├── SKILL.md
├── template.json                 # marca y layout: colores, fuentes, márgenes, header/footer, estilos, ficha técnica
├── requirements.txt
├── assets/branding/              # header-pattern.png, logo.png
├── scripts/build_report.py       # motor JSON -> PDF
├── references/content-schema.md  # tipos de bloque del JSON de contenido
├── references/figure-style.md    # cómo dibujar figuras SVG con la paleta de la plantilla
└── examples/informe-ejemplo/  # informe.json + figuras/ (ejemplo completo)
```

## Pasos

1. **Reunir el contenido.** Del usuario: tema/propósito, secciones, imágenes provistas, cliente
   (p. ej. ACME), versión y fecha. Si faltan datos de la ficha técnica (código, aprobador) no preguntes:
   el motor pone `[por confirmar]`.

2. **Preparar dependencias** (una vez por entorno):
   ```bash
   pip install -r <skill>/requirements.txt     # reportlab + svglib
   ```

3. **Crear la carpeta de trabajo** (fuera de la skill), p. ej. `docs/informes/<nombre_corto>/`:
   - Copia ahí las imágenes del usuario (en `figuras/`).
   - Si hace falta un diagrama que el usuario no dio, dibújalo como SVG siguiendo
     `references/figure-style.md`.

4. **Escribir `informe.json`** según `references/content-schema.md`. Partir de
   `examples/informe-ejemplo/informe.json` como modelo. Reglas:
   - La sección **1. Ficha Técnica** la genera el motor a partir de `meta`; tus `h1` empiezan en 2
     (la numeración es automática, no escribas "2." en el texto).
   - Texto inline: `**negrita**`, `*cursiva*`, `` `código` `` (azul monoespaciado), `[texto](url)`.
   - Rutas de figuras relativas al JSON.

5. **Generar el PDF:**
   ```bash
   python <skill>/scripts/build_report.py docs/informes/<nombre>/informe.json --preview docs/informes/<nombre>/preview
   ```
   Sin `-o`, el nombre sale de `template.json → naming.pattern`:
   `<cliente>_<nombre_corto>_<YYYYMMDD>_v<version>.pdf` junto al JSON.

6. **Verificar** abriendo los PNG de `--preview` (requiere `pymupdf`; si no está, omite y revisa el PDF):
   títulos huérfanos al final de página, figuras demasiado grandes, tablas cortadas. Ajusta con
   `{"type":"page_break"}`, `{"type":"keep_space","height":200}` o `width_pct` de la figura y regenera.
   Borra la carpeta `preview/` al terminar si el usuario no la pidió.

## Personalizar la marca

Todo lo visual vive en `template.json` (no en el código): colores, tamaños, header (`caption`), texto del
footer, campos de la ficha, paginación (`footer.page_numbers`). Para usar tu marca,
reemplaza los PNG de `assets/branding/` y edita `template.json`, o pasa otra plantilla con
`--template ruta.json`.

## Fuentes

Busca Arial/Consolas (Windows) → Liberation/DejaVu (Linux) en las carpetas del sistema, en
`assets/fonts/` de la skill o en `$INFORME_FONTS_DIR`. Si no encuentra ninguna usa Helvetica/Courier
base (el PDF se genera igual, con aviso). En Linux sin fuentes: `apt install fonts-liberation`
o copia los `.ttf` a `assets/fonts/`.

## Errores comunes

| Síntoma | Causa / solución |
|---|---|
| `Falta ReportLab` | `pip install -r requirements.txt` |
| `Bloque #N tipo 'x' desconocido` | Usa un tipo de `content-schema.md` |
| `Fila con N celdas` | Cada fila de `table.rows` debe tener tantas celdas como `columns` |
| `Figura no encontrada` | La ruta de `src` es relativa al JSON |
| SVG sin svglib | Instala svglib, o pymupdf (lo rasteriza), o entrega un PNG con el mismo nombre |
