# Guion: presentación de Tija al equipo

Duración total estimada: **20-25 minutos** más preguntas.
Archivo: `docs/presentacion/index.html` (ábrelo en Chrome o Edge).

**Controles:** `→` / `←` o clic para avanzar o retroceder · `Espacio` reproduce el video en la diapositiva 1 y avanza en las demás · `F` pantalla completa · `Inicio` / `Fin` · `index.html#8` abre directo en la diapositiva 8.
Los diagramas de las diapositivas 4 y 7 son interactivos (zoom, foco, ruta). También se abren aparte con el enlace "abrir diagrama ↗".

---

## Diapositiva 1: Video de presentación (45 s)

**Objetivo:** enganchar al equipo y mostrar el producto completo antes de explicarlo.
**Archivo:** `tija-intro.mp4` (fuente editable en `video/`).

El video sólo tiene efectos de sonido. Lo narras tú en vivo, o lo dejas correr en silencio y comentas al final. Guía por segundo:

| Tiempo | Qué se ve | Qué decir (opcional) |
|---|---|---|
| 0-6 s | "Abrir un PR, hoy": los tres pasos manuales y `pr-governance.yml` con ✗ | "Así abrimos un PR hoy." |
| 6-9 s | "intento 3"; todo se borra y queda el `>_` | "Y lo repetimos hasta que la Action lo acepta." |
| 9-13 s | Cae la mascota, se escribe TIJA | "Esto es Tija." |
| 13-15 s | Menú real de la TUI, se elige Init | |
| 15-18 s | `tija init` en ClaimsAPI (gato *scanning*) | "Detecta el stack y prepara `.tija/`." |
| 18-24 s | `tija check`: build ✓ 7.8s, format ✗ 29.1s, tests ✓ 39/39 (gato *warning*) | "Corre las herramientas del repo y guarda los resultados reales." |
| 24-31 s | `tija pr create`: contexto, gato *thinking*, preview validado, `[y/N] y` | "Arma el PR con el contexto real y lo valida antes de publicarlo." |
| 31-38 s | Fondo naranja: PR #17, Merged into dev, 53 archivos, 39/39 | "Este es un PR real de ClaimsAPI." |
| 38-45 s | "Tu repo. Tus reglas. Cualquier agente." y luego el icono y el comando de instalación | |

---

## Diapositiva 2: El problema

**Objetivo:** que todos reconozcan el dolor.

- Hoy, para abrir un PR: escribimos un resumen de memoria, lo pegamos en ChatGPT con la guía y copiamos la respuesta a GitHub.
- El modelo **sólo ve lo que pegamos**. No ve el diff real, ni los commits, ni si las pruebas pasaron.
- Después `pr-governance.yml` lo rechaza por cosas mecánicas: título sin scope, secciones faltantes, menos de 10 caracteres, valores como `N/A` o `pendiente`.
- Pregunta al equipo: *"¿Cuántas veces les ha fallado la Action por el formato?"*

**Transición:** "¿Y si la herramienta viviera dentro del repo y ya tuviera todo ese contexto?"

---

## Diapositiva 3: Qué es Tija

**Objetivo:** el concepto en una frase.

- Lee la frase grande: *los workflows, las reglas y el contexto pertenecen al repositorio; el agente es sólo un ejecutor intercambiable.*
- Es una CLI local en Go, con TUI, que vive en cada repo.
- Los cuatro principios:
  1. **Deterministic first:** todo lo que se puede calcular (rama, diff, commits, stack, pruebas) lo calcula Tija. La IA sólo redacta.
  2. **Repository native:** las reglas y templates se versionan en `.tija/` y viajan con el código.
  3. **Agent agnostic:** usa los CLIs que ya tenemos (Claude Code, Codex, Gemini), sin API keys nuevas.
  4. **No pisa nada:** cualquier cambio fuera de `.tija/` se muestra como diff y pide confirmación.

---

## Diapositiva 4: Arquitectura (diagrama Archify)

**Objetivo:** explicar "un motor, tres interfaces".

- Recorre el diagrama de izquierda a derecha. Puedes hacer clic en los nodos o usar **RUTA** para resaltar caminos.
- **Tres entradas:** el desarrollador usa la TUI; los scripts y CI usan la CLI con `--json --yes`; los agentes usarán MCP (Fase 5, línea punteada = planificado).
- **Un solo Engine:** init, doctor, status, quality y pr. Ningún workflow se implementa dos veces. Por eso la TUI, la CLI y el MCP se comportan igual.
- **A la derecha, lo que Tija orquesta** pero no reemplaza: git + gh, las herramientas del repo (dotnet, biome, vitest) y los CLIs de agentes, siempre como subproceso.
- **Abajo, `.tija/`:** manifest, reglas, templates y la sesión (`quality.json`, `pr.md`).

---

## Diapositiva 5: La TUI

**Objetivo:** mostrar la experiencia y la identidad de marca.

- Al escribir `tija` sin argumentos en una terminal se abre la startup screen: logo pixel TIJA, el gato y el checklist de detección.
- El menú **sólo muestra lo que existe**: Init, Pull Request, Check, Tests, Doctor y Status.
- La mascota tiene **seis estados que reflejan lo que pasa de verdad**: idle, scanning (init/doctor), thinking (el agente redacta), success, warning (needs your review) y error.
- Fuera de una terminal (pipes, CI) no hay TUI ni colores: texto plano o `--json`. La TUI nunca estorba la automatización.
- Paleta: naranja `#F97316`, canvas `#081823` y sand `#FED7AA`. "Small cat. Big workflows."

---

## Diapositiva 6: Lo que ya está listo

**Objetivo:** dejar claro que no es un prototipo; hay tres fases en `main`.

- **Fase 1 (PR #1):** `init`, `doctor` y `status`. Detecta .NET y React/Vite, registra el template de PR y sugiere nombres de rama conformes sin bloquear (`101-fixclaims-…` → `fix/101-claims-integration-issues`).
- **Fase 2 (PR #3):** `check`, `lint`, `format` y `test` con adapters para dotnet, Biome, ESLint/Prettier y Vitest/Jest. Guarda los resultados por commit en `.tija/session/quality.json`.
- **Fase 3 (PR #4):** `pr prepare` y `pr create`. Es el primer hito de valor.
- **Marca (PR #5):** TUI renovada y estados de la mascota.
- Exit codes estables: `0` ok, `1` fallo de check o validación, `2` error de uso o de entorno.

---

## Diapositiva 7: Flujo de `tija pr create` (diagrama Archify)

**Objetivo:** explicar por qué el PR pasa las reglas y no inventa.

- **Contexto real:** Tija calcula rama, target, commits, diffstat e issue (`Closes #15` sale del nombre de la rama).
- **Pruebas:** sólo cita `quality.json` **del mismo commit y con el árbol limpio**. Si no hay resultados verificados, lo dice.
- **El agente sólo redacta** título y cuerpo (tier `fast`). Si no hay agente o falla, Tija hace un **borrador determinista**: el flujo nunca depende de la IA.
- **Validación local** con las mismas reglas de `pr-governance.yml`: Conventional Commits con scope, secciones obligatorias, longitud mínima y valores inválidos. Si el borrador es inválido, hay un reintento con la lista de problemas.
- **Confirmación `[y/N]`** con preview editable (`--edit`). Después: push, `gh pr create` y asignación automática.

---

## Diapositiva 8: Caso real en ClaimsAPI

**Objetivo:** el ejemplo práctico. "Se corrió en Claims y se generó este PR."

- Repo: **GMX-ClaimsAPI** (.NET, `ClaimsService.sln`), rama `refactor/15-automapper-to-mapperly` → `dev`.
- `tija init`: detectó `dotnet-api`, la solución y el template de PR, y creó el manifest con `target: dev` y contenido en español.
- `tija check` en el commit `8a77911`:
  - build ✓ 7.8 s
  - format ✗ 29.1 s (diferencias de formato que **ya existían** en servicios no tocados)
  - tests ✓ **39 passed, 0 failed** en 7.0 s
- `tija pr create` armó el PR #17: 53 archivos, +1776 −1292, 3 commits.
- **Punto clave:** format falló y el PR lo cuenta tal cual. Ese es el anti-objetivo de Tija: **nunca inventar pruebas** ni esconder fallos.

> Nota honesta para el presentador: en esta corrida el borrador se generó en modo determinista (`.tija/session/pr.json` → `"source": "deterministic"`) y el autor enriqueció el texto antes de publicarlo. Los datos (rama, commits, issue, pruebas) los calculó Tija. Si alguien pregunta, dilo así.

---

## Diapositiva 9: El PR resultante

**Objetivo:** que vean un PR que cumple la plantilla.

- Título `refactor(mappings): replace AutoMapper with Mapperly`: Conventional Commits con scope.
- Recorre las secciones: Resumen, Objetivo, Cambios realizados y Pruebas (todas con contenido útil).
- **Pruebas** cita números reales (39/39) y reconoce lo de `dotnet format`.
- **Referencias:** `Closes #15`, calculado desde la rama.
- **Evidencias:** `N/A`. Tija nunca la rellena por su cuenta.
- PR mergeado en `dev` el 27/10/2026.

---

## Diapositiva 10: Roadmap

**Objetivo:** lo que vamos a sumar después.

- ✓ **Fases 1-3:** Foundation, Quality y PR (lo que acabamos de ver).
- **Fase 4, Review:** `tija review` → diff + análisis estático + tests + reglas de arquitectura de `.tija/rules/` + agente (tier `balanced`). Salida: findings con archivo, línea, severidad (Critical/High/Medium/Low), regla y posible solución.
- **Fase 5, MCP:** la siguiente diapositiva.
- **Fase 6, Test intelligence:** `tija test affected`, `tija test suggest` y E2E con Playwright (traces y screenshots).
- **Fases 7-10:** handoffs entre repos (backend → frontend), workspace multi-repo, búsqueda semántica con Qdrant y extensiones (Renovate, Ruff, Scoop/Homebrew/winget).
- Regla de producto: el menú nunca muestra algo que todavía no existe.

---

## Diapositiva 11: MCP (Fase 5)

**Objetivo:** explicar por qué el MCP es la pieza central.

- `tija mcp serve` (stdio) expone **el mismo engine** como tools: `repo.status`, `repo.rules`, `repo.context`, `quality.check`, `quality.fix`, `tests.run`, `pr.prepare`, `pr.create` y `review.run`.
- Claude Code o Codex dejan de "adivinar" el repo: piden el contexto y las reglas a Tija.
- No hay un segundo camino: un PR preparado vía MCP pasa **exactamente** las mismas validaciones que uno hecho con la CLI.
- Criterio de salida de la fase: Claude Code y Codex preparan un PR válido a través de MCP.

---

## Diapositiva 12: Cómo empezar y preguntas

**Objetivo:** llamada a la acción.

```sh
go install github.com/Ps3udoDev/tija-cli/cmd/tija@latest
tija init        # una vez por repo
tija check       # pruebas reales del commit
tija pr create   # push + PR con gh
```

- También hay binarios para Windows, Linux y macOS en GitHub Releases.
- Requisitos: git y, para PRs, `gh` autenticado (`gh auth login`). `tija doctor` dice qué falta.
- Propuesta: que cada quien lo pruebe en su próximo PR y comparta feedback.
- Abre la ronda de preguntas.

---

## Mantenimiento de los materiales

| Pieza | Cómo regenerarla |
|---|---|
| Video | `cd docs/presentacion/video` → `npx hyperframes preview` para editar, `npx hyperframes render -o ../tija-intro.mp4 --quality delivery` para renderizar. Brief y spec en `video/BRIEF.md` y `video/SPEC.md`. |
| Música | Sin cuenta HeyGen no hay catálogo. Para agregar una pista: copia el archivo a `video/assets/music.mp3`, añade `<audio id="bgm" class="clip" src="assets/music.mp3" data-start="0" data-duration="45" data-track-index="2" data-volume="0.25" data-fade-out="1.5"></audio>` en `video/index.html` y vuelve a renderizar. |
| Diagramas | Las fuentes JSON están en `.archify/*/candidate.json`. Se regeneran con `node <archify>/bin/archify.mjs finalize <architecture\|workflow> <candidate.json> diagramas/<nombre>.html --quality showcase`. |
| Mascota | `assets/cat-*.svg` salen de `internal/tui/mascot.go`. Si cambian los sprites, regenéralos. |
