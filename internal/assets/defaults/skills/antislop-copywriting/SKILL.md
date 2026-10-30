---
name: antislop-copywriting
description: "Technical PR copy hygiene: concise, factual, no AI vocabulary, no agent attributions."
source:
  project: anti-slop
  author: Miqdad Badjuber
  repo: https://github.com/miqdadbadjuber/anti-slop
  path: skills/antislop-copywriting/SKILL.md
  commit: 3005235d030b4793146ad9afe8d01045efd90e1d
  version: 3.2.24
  license: MIT
  copyright: "Copyright (c) 2026 Miqdad Badjuber (antislop)"
adapted:
  by: Tija
  date: 2026-10-29
  changes:
    - Extracted only guidelines applicable to technical PR descriptions in Spanish.
    - Dropped marketing copy, headlines, CTAs, and landing page patterns.
    - Added Tija's strict "No agent attribution" rule (no AI tool signatures or trailers).
    - Added Spanish equivalents for AI vocabulary buzzwords.
---

> Adaptado de anti-slop de Miqdad Badjuber (https://github.com/miqdadbadjuber/anti-slop, licencia MIT). Los cambios respecto del original están listados en el frontmatter.

# antislop-copywriting

Reglas de redacción técnica y estilo para descripciones de Pull Requests y mensajes de commit. Su propósito es producir descripciones concisas, factuales y de voz profesional, eliminando la prosa inflada típica de asistentes de IA.

---

## Principios Fundamentales

1. **Nunca inventar hechos (Never invent facts).** Una redacción o resumen jamás agrega hechos, nombres de componentes, cifras, fechas o conclusiones que no provengan directamente del diff, del código fuente o de las notas provistas por el autor. Si falta detalle técnico, se reporta de forma concisa sin especular.
2. **No sobre-esterilizar (Do not over-sterilize).** Eliminar clichés de IA no implica transformar el texto en un bloque robótico o corporativo. La voz y contexto técnico del equipo deben preservarse intactos.

---

## Patrones a Evitar

### Vocabulario Vacío de IA (Empty AI Vocabulary)

Verbos y adjetivos abstractos que se usan para sonar imponentes sin aportar información técnica real:
- *Español:* potenciar, robusto, sin fisuras, de última generación, llevar al siguiente nivel, transformar, crucial, revolucionar, desbloquear el potencial, integral, holístico.
- *Inglés:* unlock, elevate, empower, delve, showcase, robust, game-changer, next-level, seamless, cutting-edge.

**En lugar de:**
> "Se implementa una solución robusta e integral que potencia el manejo de reclamos para llevar el rendimiento al siguiente nivel sin fisuras."
**Escribir:**
> "Se agrega validación de límites de póliza en la creación de reclamos y se cachean los totales calculados."

### Inflación de Importancia y Afirmaciones Sin Evidencia

Evitar frases grandilocuentes como "este cambio marca un hito crucial", "establece las bases para la nueva era de la API", o "optimiza drásticamente". Describir el cambio con precisión técnica cuantificable o delimitable.

### Listas Infladas y Emojis Decorativos

Evitar el uso de viñetas cargadas de emojis artificiales (`🚀`, `✨`, `🔥`, `✅`, `💡`) o palabras completas en MAYÚSCULAS para crear falso impacto en las notas del PR.

---

## Regla Específica de Tija: Sin Atribuciones de Agentes (No Agent Attribution)

Los Pull Requests y mensajes de commit representan el trabajo del equipo de desarrollo ante sus pares y la organización. **No deben contener firmas, marcas de agua ni atribuciones de herramientas de IA:**

- **Prohibido:** `🤖 Generated with [Claude Code](...)`, `Generated with Codex`, `Made with Cursor`, `Powered by Copilot`.
- **Prohibido:** Trailers git automáticos asignando co-autoría a inteligencias artificiales, como `Co-Authored-By: Claude <noreply@anthropic.com>` o similares.
- **Permitido:** Los co-autores humanos (`Co-Authored-By: Nombre <correo@empresa.com>`) se mantienen siempre respetados.
- **Prohibido:** Párrafos o preámbulos que aclaren que la descripción o el código fue generado por un modelo de lenguaje.
