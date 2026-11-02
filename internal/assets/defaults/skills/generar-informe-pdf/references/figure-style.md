# Estilo de figuras SVG

Usar cuando el informe necesita un diagrama (flujo, secuencia, arquitectura) que el usuario no entregó.
Ver `examples/informe-ejemplo/figuras/*.svg` como modelo. Las imágenes del usuario (PNG/JPG/SVG) se usan tal cual.

## Lienzo

- `width="980"` y `viewBox="0 0 980 H"`; alto según contenido. Fondo `<rect>` blanco.
- `font-family="Arial, Helvetica, sans-serif"` en el `<svg>` raíz. Solo texto con `<text>` (sin `<foreignObject>`).
- Título: `x=20 y=28`, 17 px bold `#292B2D` + línea `x1=20 x2=960 y=38` naranja `#F97316` 2 px.
- Nada de CSS externo, `<style>` complejo, filtros ni gradientes (svglib no los soporta bien): usa atributos.

## Nodos

`<rect rx="8" stroke-width="1.4">`, típicamente 210×70, texto 15 px centrado (`text-anchor="middle"`), máximo 2 líneas.

| Significado | fill | stroke |
|---|---|---|
| Paso normal / componente | `#EEF3F9` | `#12344D` |
| Error / fallo | `#FDE2E2` | `#C62828` |
| Advertencia / reintento | `#FFF3CD` | `#D99A00` |
| Acción de recuperación / destacado | `#D1ECF1` | `#0C5460` |
| Éxito / completado | `#EAF4D3` | `#6E8F12` |
| Neutro / manual | `#F6F4F1` | `#8A8782` |

Decisiones: rombo (`<polygon>`) con colores de "paso normal".
Texto de nodo en `bold` solo para estados clave.

## Conectores

- Flechas `#292B2D` 1.4 px con `marker-end="url(#ah)"`; fallo con `#C62828`, `stroke-dasharray="5,4"` y marcador `#ahr`.
- Etiquetas de flecha 12–13 px bold del color de la flecha.
- Líneas de vida (secuencia): `#A8A49E` 1.2 px, `stroke-dasharray="4,4"`.

```xml
<defs>
  <marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#292B2D"/></marker>
  <marker id="ahr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#C62828"/></marker>
</defs>
```

## Tamaño en el PDF

`width_pct` 82 (default) ≈ 413 pt de ancho. Una figura de 980×310 ocupa ~130 pt de alto; si una figura
mide más de ~1400 px de alto, divídela o baja `width_pct`.
