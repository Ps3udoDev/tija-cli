---
name: antislop-code
description: Classify code comments as slop or valuable. Never touches code.
source:
  project: anti-slop
  author: Miqdad Badjuber
  repo: https://github.com/miqdadbadjuber/anti-slop
  path: skills/antislop-code/SKILL.md
  commit: 3005235d030b4793146ad9afe8d01045efd90e1d
  version: 3.2.24
  license: MIT
  copyright: "Copyright (c) 2026 Miqdad Badjuber (antislop)"
adapted:
  by: Tija
  date: 2026-10-29
  changes:
    - Removed references to the core antislop.md rule numbers (R-XX); the comment principle is inlined.
    - Removed allowed-tools (the agent does not edit code).
    - Added Tija's output contract (JSON classification per comment id).
    - Added the protected-directives list and the safe/unsafe classes.
    - Added examples and guidance for comments in Spanish.
---

> Adaptado de anti-slop de Miqdad Badjuber (https://github.com/miqdadbadjuber/anti-slop, licencia MIT). Los cambios respecto del original están listados en el frontmatter.

# antislop-code

Hygiene and classification rules for code comments. This skill identifies generic AI-generated slop comments (decorative, obvious, workflow narration, stiff phrasing) while preserving comments that carry real engineering value.

**Guiding principle:** Un comentario se queda solo si explica algo que el código no muestra (la razón, la restricción, el caso borde o el comportamiento no obvio).

**Scope guardrail:** This skill only classifies comments. Never modify or suggest modifying executable code, identifiers, imports, formatting, indentation, control flow, or logic.

---

## Comment Classes and Safety in Tija

Every classified comment belongs to one of the following exact class names:

- `separator`: Pure decorative dividers composed only of repeated characters (`// ------------`, `/* ======== */`).
- `labeled-separator`: Banner labels or dividers with section text (`// ---- Step 1 ----`, `{/* ---------- Row 1 ---------- */}`).
- `end-marker`: End of block markers on closing braces (`} // end if`, `// end of function`).
- `empty-line`: Blank comment lines (`//`) without surrounding block context.
- `restates-obvious`: Comments repeating what the line or identifier already shows (`let count = 0; // Initialize count`).
- `workflow-narration`: Step-by-step narration of normal control flow (`// Step 1: Validate input`).
- `empty-label`: Generic category labels without concrete information (`// Main logic`, `// Helper function`).
- `decorative-emoji`: Decorative emoji in comments (`// 🚀 Performance`, `// ✅ Done`).
- `line-by-line`: Narrating every trivial line or statement.
- `vague-todo`: Vague TODO placeholders with no actionable task (`// TODO: improve this`).
- `signature-echo`: Documentation blocks simply echoing function/parameter names without added meaning.
- `commented-out-code`: Disabled or abandoned code blocks inside comments.
- `over-explained`: Multi-line commentary padding a single simple constraint into a paragraph.
- `business-rule`: Domain constraints, accounting/legal logic, pricing rules, or policy requirements.
- `keep-other`: Architecture decisions, security considerations, performance trade-offs, concurrency invariants, protocol contracts, workarounds, edge cases, and assumptions.

### Protection and Safety Levels

1. **Protected (never touched):** Directives (`biome-ignore`, `eslint-disable*`, `eslint-enable`, `prettier-ignore`, `@ts-*`, `@jsx*`, `@flow`, webpack magic comments), legal/copyright notices, and JSDoc type definitions in JS/JSX (`@type`, `@typedef`, `@param {...}`, `@returns {...}`).
2. **Safe (`--write`):** High-confidence removals of `restates-obvious`, `workflow-narration`, `empty-label`, `labeled-separator`, `decorative-emoji`, `end-marker`, `line-by-line`, and pure `separator`.
3. **Unsafe (`--write --unsafe`):** Any `rewrite`, any removal with low confidence, `vague-todo`, `signature-echo`, `commented-out-code`, and `over-explained`.
4. **Keep (`keep`):** Comments explaining business rules, intent, architecture, security, or non-obvious constraints (`business-rule`, `keep-other`).

---

## Patterns and Diagnostic Criteria

### Decorative Separators
- **Tell:** Banner comments built from repeated characters (`// =======================`, `/* -------- */`).
- **Why:** The decoration is visual noise signaling AI generation without adding engineering value.
- **Fix:** Remove.

### Restating the Obvious
- **Tell:** Comments repeating what the next statement or signature already expresses (`// User class` above `class User`, `// Initialize count` above `let count = 0`).
- **Why:** Doubles the reading cognitive load without giving any new insight.
- **Fix:** Remove.

### Workflow Narration
- **Tell:** Step-by-step checklist narration (`// Step 1: parse`, `// Step 2: send`, `// Finally return`).
- **Why:** Control flow is visible in code structure.
- **Fix:** Remove.

### Empty Labels
- **Tell:** Labels naming a category rather than a fact (`// Business logic`, `// Entry point`, `// Note: important`).
- **Why:** Carries zero concrete facts.
- **Fix:** Remove unless it carries a specific constraint ("Note: retries happen only on 5xx").

### Vague Placeholders
- **Tell:** Vague TODOs (`// TODO: optimize`, `// Future improvements`).
- **Why:** Names a vague sentiment rather than an actionable engineering task.
- **Fix:** Unsafe remove (`vague-todo`).

### Signature Echo
- **Tell:** JSDoc echoing names (`@param price The price`, `@returns Total price`).
- **Why:** Adds length rather than comprehension.
- **Fix:** Unsafe simplify or remove. Never strip real type information or business rules.

### Decorative Emoji
- **Tell:** Emoji used as decoration (`// ✅ Validation`, `// 🚀 Cache`).
- **Why:** Visual noise.
- **Fix:** Remove or rewrite without emoji.

### End Markers
- **Tell:** Comments marking block ends (`} // end if`, `// End processOrder`).
- **Why:** Closing braces already define block bounds.
- **Fix:** Remove.

### The Over-Explained Comment
- **Tell:** Paragraphs recounting issue history, release numbers, or reasoning chains when a single constraint suffices.
- **Why:** Buries the essential constraint under narrative fluff.
- **Fix:** Rewrite to a concise 1-line constraint (`over-explained`).

### Line-by-Line Narration
- **Tell:** Comment on every single trivial line in a routine.
- **Why:** Creates noise that obscures important comments.
- **Fix:** Remove trivial ones; keep at most one concise comment per logical block.

---

## Comentarios en Español (Ejemplos de Referencia)

El criterio se aplica idéntico en español:

### Comentarios que se eliminan (slop)
- `// Inicializar el arreglo de reclamos` encima de `const claims = [];` (`restates-obvious`)
- `// Paso 1: Validar cédula` encima de `validarCedula(cedula);` (`workflow-narration`)
- `// Lógica principal` (`empty-label`)
- `// ================= RECLAMOS =================` (`separator` / `labeled-separator`)
- `// ✅ Función para calcular total` (`decorative-emoji` / `restates-obvious`)
- `} // fin del foreach` (`end-marker`)

### Comentarios que se conservan (valiosos)
- `// La Superintendencia exige retener el log de auditoría por 7 años antes de depurar.` (`business-rule`)
- `// Workaround para Safari iOS <= 15: preventDefault cancela el evento touchstart.` (`keep-other`)
- `// El cálculo de copago aplica únicamente cuando el deducible anual fue cubierto en su totalidad.` (`business-rule`)

---

## Contrato de Salida (JSON)

El agente clasificador responde **únicamente** con un arreglo JSON válido:

```json
[
  {
    "id": "src/pages/claims/Form.jsx#4812",
    "action": "remove",
    "class": "labeled-separator",
    "confidence": "high",
    "reason": "Banner label naming a layout row"
  },
  {
    "id": "src/pages/claims/useLimits.js#930",
    "action": "keep",
    "class": "business-rule",
    "confidence": "high",
    "reason": "Explains cumulative limit per claim"
  },
  {
    "id": "src/utils/calc.ts#2210",
    "action": "rewrite",
    "class": "over-explained",
    "confidence": "high",
    "text": "Default estable para no crear un arreglo en cada render."
  }
]
```

Acciones permitidas:
- `remove`: Borrar el comentario.
- `keep`: Conservar el comentario sin cambios.
- `rewrite`: Proponer un texto alternativo conciso (especificado en `text`).
