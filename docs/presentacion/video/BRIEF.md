---
workflow: general-video
flow: automation
storyboard: no
message: "Tija convierte tu repo en un PR válido, con datos reales y sin copiar y pegar"
destination: presentation
aspect: 1920x1080
language: es
audience: equipo de desarrollo interno
length: 45s
angle: launch
---

## Intent

Video de presentación de producto de ~45 s que abre la diapositiva 1 de la
presentación interna de Tija. Muestra el dolor de hoy (resumen manual → ChatGPT →
GitHub → la gobernanza falla), revela Tija y su TUI con la mascota, y lo prueba
con el caso real de ClaimsAPI (PR #17).

## Assets

- assets/img/tija-icon.png — icono oficial de la app (end card).
- assets/img/cat-*.svg — sprites de la mascota generados desde internal/tui/mascot.go.
- assets/img/tija-wordmark.svg — wordmark pixel "TIJA" como en la startup screen.

## Customizations

- Música de fondo (elegida por el usuario). Sin cuenta HeyGen el catálogo no está
  disponible: se entrega con efectos de sonido y un hueco listo para `assets/music.mp3`.

## Notes

- Datos reales de `.tija/session/quality.json` y `pr.json` de ClaimsAPI (commit 8a77911).
- No afirmar que la Action de gobernanza pasó: en PR #17 no hay checks reportados.
- El presentador narra en vivo encima; no hay voz en off.
