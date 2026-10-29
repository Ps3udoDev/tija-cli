GOAL:    el equipo termina pensando "quiero correr `tija pr create` en mi rama".
OUTPUT:  HyperFrames, 1920 x 1080, 30 fps
TIME:    45 s x 30 = 1350 frames. Sin música en el render base (hueco para assets/music.mp3).
LOOK:    tres mundos con la paleta de marca:
         A "el modo de hoy": suelo #F1F5F9, tinta #111827, rojo de fallo #EF4444 sólo en ✗.
         B "terminal": suelo #081823 (canvas real de la TUI), selección #1E3A50,
           naranja #F97316, blanco #FFFFFF, secundario #A8B8C5, separador #354957, sand #FED7AA, verde #22C55E.
         C "resultado": suelo naranja #F97316, tinta #111827.
         Display: Inter 900, 96-150 px, tracking -3%. Terminal: JetBrains Mono 400/700, 26-34 px.
         El medio es la TUI real, reconstruida a ~1.7x de su tamaño en terminal; mascota pixel nítida.
OBJECTS: mascota pixel (6 estados reales), menú real de la TUI, filas de check con duraciones reales,
         preview del PR #17 con su título real, icono oficial.
MOTION:  llegadas expo.out 0.5-0.7 s; cámara/flood power2.inOut 0.4-0.6 s; salidas power3.in, 40% más cortas.
         Palabras 0.07 s de stagger, filas 0.06-0.12 s. Resortes de la mascota back.out(1.6).
SHOTS:   0-9    A: tres pasos del modo de hoy + panel pr-governance con ✗ → "Otra vez." → `>_` crece a terminal (zoom through)
         9-13   B: la mascota cae, TIJA se escribe pixel a pixel, tagline por palabras
         13-31  B: TUI a pantalla completa: menú → tija init (scanning) → tija check (warning) → tija pr create (thinking → success → confirmación y)
         31-38  C: flood naranja desde la "y": PR #17 Merged, +1776 −1292, 53 archivos, 39/39 tests
         38-45  C→B: "Tu repo. Tus reglas. Cualquier agente." → icono + wordmark + comando de instalación
BANNED:  vacío oscuro con cosas pequeñas flotando; glow/partículas; tres tarjetas en fila; logo en esquina;
         títulos que se mueven tras aterrizar; afirmar que la gobernanza pasó; métricas inventadas.
QA:      2.5 (pasos), 4.5 (✗), 8.8 (zoom), 11.5 (reveal), 15 (init), 21 (check), 28 (preview), 34 (PR #17), 40 (claim), 44 (end card).
