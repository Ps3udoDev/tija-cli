# Esquema del JSON de contenido

```json
{
  "meta":   { ... },
  "blocks": [ { "type": "...", ... }, ... ]
}
```

## `meta`

| Campo | Obligatorio | Uso |
|---|---|---|
| `title` | sí | Título azul marino 18 pt + línea naranja. También "Nombre del Documento" en la ficha. |
| `subtitle` | no | Subtítulo en negrita 13 pt bajo la línea. |
| `cliente` | no | Para el nombre de archivo (`ACME`). |
| `nombre_corto` | no | Para el nombre de archivo, sin espacios (`MigracionFacturacion`). |
| `version` | no | `"1.0"`. Ficha + nombre de archivo. |
| `fecha` | no | ISO `YYYY-MM-DD`; se muestra `dd/mm/aaaa`. |
| `clasificacion` | no | Default `Confidencial`. |
| `aprobo` | no | Nombre del aprobador; si falta, `[por confirmar]`. |
| `autor` | no | Metadato del PDF (default `Tija`). |
| `output` | no | Ruta del PDF relativa al JSON (ignora `naming.pattern`). |
| `ficha` | no | Sobrescribe filas de la ficha técnica: `codigo`, `nombre`, `modulo`, `area`, `estado`, `version_fecha`, `aprobadores`. Vacíos → `[por confirmar]`. |
| `ficha_tecnica` | no | `false` para omitir la sección 1 (documentos sin control de versiones). |

## Bloques

| `type` | Campos | Render |
|---|---|---|
| `h1` | `text`, `numbered` (default true) | Sección numerada automáticamente, 13 pt negrita. |
| `h2` | `text` | Subsección gris 11.5 pt negrita, sin número. |
| `paragraph` | `text` | Párrafo 11 pt. |
| `list` | `style`: `numbered`\|`bullet`, `items` | Numerada: números azules en negrita. Viñetas: ● naranja; anidadas ○. |
| `code` | `text` (con `\n`), `lang` (informativo) | Bloque monoespaciado 8.5 pt, fondo gris, barra azul a la izquierda. Puede partirse entre páginas. |
| `table` | `columns`, `rows`, `col_widths_pct`, `first_col_bold` | Encabezado azul con texto blanco, filas alternas gris claro, rejilla gris. El encabezado se repite si cruza página. |
| `note` | `text`, `label` (default `NOTA:`), `bg`, `bar` | Caja gris con barra naranja. `bg`/`bar` aceptan nombre de color de la plantilla o `#hex`. |
| `figure` | `src` (.svg/.png/.jpg), `caption`, `width_pct` (default 82) | Imagen centrada + "Figura N. caption" (numeración automática). |
| `page_break` | — | Salto de página. |
| `keep_space` | `height` (pt) | Salta de página si quedan menos de `height` pt. |
| `spacer` | `height` (pt) | Espacio vertical. |

Alias: `heading`=`h1`, `subheading`=`h2`, `p`=`paragraph`, `image`=`figure`.

### Items de lista

Un item es un string o un objeto con sub-items:

```json
{ "type": "list", "style": "numbered", "items": [
  "**Primer punto:** texto con `codigo`.",
  { "text": "**Segundo punto:**", "items": ["sub-item A", "sub-item B"], "style": "bullet" }
] }
```

### Texto inline (todos los campos `text`, celdas, captions)

`**negrita**` · `*cursiva*` · `` `código` `` · `[texto](https://url)` · salto de línea `\n`.
No escribas HTML: `<` y `>` se muestran literalmente.

## Layout de página (de `template.json`)

```
 Carta 612×792 pt
 ┌───────────────────────────────────────────────┐
 │ header-pattern.png (0,0 → 612×72)             │
 │ logo (55,24 → 108×35)                         │
 ├─ y=92 ─── área de contenido (x 55 → 557) ─────┤
 │ Título / línea naranja / subtítulo (solo pág. 1)│
 │ 1. Ficha Técnica (solo pág. 1)               │
 │ bloques...                                    │
 ├─ y=722 ───────────────────────────────────────┤
 │ ───────── línea naranja (y=735.5) ───────────── │
 │ Documento generado con Tija                   │
 └───────────────────────────────────────────────┘
```
