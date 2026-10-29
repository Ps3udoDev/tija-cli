# Tija — Diseño por fases

> Fecha: 2026-10-22
> Actualizado: 2026-10-29: se agrega la Fase 3a (skill packs, `clean comments`, workflows compuestos; spec en `2026-10-29-tija-phase-3a-skill-packs-design.md`) y la Fase 4 se apoya en las rúbricas de Thermos (Cursor, MIT)
> Estado: borrador para aprobación
> Fuentes (en `docs/reference/`): `product/technical-product-draft.md`, `brand/brand-exploration-terminal.png`, `pull-requests/PULL_REQUEST_TEMPLATE_GUIDE.md`, `pull-requests/PULL_REQUEST_TEMPLATE.md`, `pull-requests/pr-governance.yml`
> Repo: https://github.com/Ps3udoDev/tija-cli

---

## 1. Intención y porqué

Tija es una herramienta local, CLI-first con TUI, que vive dentro de cada repositorio y estandariza los workflows de ingeniería para humanos y agentes de IA.

**Principio:** *los workflows, las reglas y el contexto pertenecen al repositorio; el agente es solo un ejecutor intercambiable.*

**El dolor concreto que origina el producto:** hoy, para abrir un PR, el autor copia un resumen manual de sus cambios, lo pega en ChatGPT junto con la estructura de `PULL_REQUEST_TEMPLATE_GUIDE.md` y le pide que arme el título y la descripción. Luego lo pega en GitHub, donde `pr-governance.yml` valida:

- título en Conventional Commits con `scope` obligatorio (`amannn/action-semantic-pull-request`, `requireScope: true`);
- secciones obligatorias `## Resumen`, `## Objetivo`, `## Cambios realizados`, `## Pruebas`, cada una con al menos 10 caracteres útiles (sin comentarios HTML ni viñetas) y sin valores vacíos como `N/A`, `na`, `no aplica`, `pendiente`, `todo`, `por definir`, `...`, `-`;
- al menos un assignee (la Action autoasigna al autor).

Ese ida y vuelta es lento y además el modelo solo ve lo poco que se le pega, no el diff real ni los commits. Tija debe tener el contexto completo del repo, aplicar el template y las reglas versionadas, y producir un PR que **pase la Action a la primera**. Esa es la métrica de éxito del producto en su primer hito de valor (Fase 3).

El camino se recorre en fases, de la más simple a la más laboriosa. La Fase 1 ya es un piloto usable.

## 2. Objetivos, no-objetivos y anti-objetivos

**Objetivos**
- Una sola herramienta (`tija`) que funcione igual en repos .NET y React/Vite, en Windows, Linux y macOS.
- Un solo motor compartido por CLI, TUI y (más adelante) MCP.
- Que las reglas y templates del repo viajen con el código y sean la fuente de verdad para humanos y agentes.
- Que el uso de IA no requiera API keys ni créditos adicionales: Tija invoca los CLIs de agentes que el usuario ya tiene instalados.

**No-objetivos (por ahora)**
- Reemplazar Biome, Ruff, dotnet, Playwright, Renovate, GitHub Actions o los agentes. Tija los orquesta.
- Dashboard web, almacenamiento remoto, arquitectura distribuida, editor gráfico.
- Generación completa de tests antes de tener test intelligence.

**Anti-objetivos (sería un fracaso aunque funcione)**
- Que Tija **invente** contenido en el PR: pruebas que no se ejecutaron, endpoints o cambios que no están en el diff.
- Que Tija pise archivos existentes del repo (template de PR, workflows, `.gitignore`) sin preguntar.
- Que la TUI estorbe la automatización: cualquier comando debe poder ejecutarse sin interacción y con salida legible por máquinas.
- Que el menú o los comandos muestren funcionalidades que todavía no existen.
- Que el producto quede acoplado a un proveedor de IA concreto.
- Que Tija presente como propio material de terceros (rúbricas, skills, prompts). Todo lo que venga de otro proyecto lleva autor, repo, commit, licencia y nota de adaptación.

## 3. Decisiones tomadas

### Decididas por el usuario
| Tema | Decisión |
|---|---|
| Orden de fases | Respetar el roadmap del draft: Foundation → Quality → PR → Review → MCP → … |
| Fase 1 | `tija init`, `tija doctor`, `tija status`, y que sea un piloto usable |
| Invocación de IA | Llamar a los CLIs de agentes ya instalados (Claude Code, Codex CLI, Gemini CLI) como subproceso. **Sin API keys propias.** |
| TUI | Entra desde la Fase 1 y se refina en cada fase |
| Idioma | Interfaz de Tija (menús, mensajes, `doctor`) en **inglés**. Contenido generado (PRs, reviews, handoffs) en **español** |
| Stack (del draft) | Go + Cobra (CLI) + Bubble Tea (TUI). MCP como pieza central más adelante. Qdrant solo en fase tardía |
| Identidad visual | Nombre **Tija**, paleta C (`#F97316` Orange, `#111827` Ink, `#FFFFFF` White, `#FED7AA` Sand), logo vectorial + mascota pixel en la TUI, startup screen según el concepto 06 de la imagen de marca |
| Estándar de PR | El de `PULL_REQUEST_TEMPLATE_GUIDE.md` + `PULL_REQUEST_TEMPLATE.md`, validado por `pr-governance.yml` |
| Material de terceros (2026-10-29) | Se toma solo lo necesario de proyectos como anti-slop y Thermos, **siempre atribuido a su autor** y adaptado a Tija. Ver la sección 4.1 del spec de la Fase 3a |
| Edición de código (2026-10-29) | Tija puede editar código fuente con la semántica de Biome: sin flag reporta, `--write` aplica lo seguro, `--write --unsafe` aplica también lo inseguro |
| Fase 3a (2026-10-29) | Se inserta entre la Fase 3 y la Fase 4 |

### Decididas por Claude (revisables)
| Tema | Decisión | Motivo |
|---|---|---|
| Carpeta en el repo | `.tija/` en lugar del `.forge/` del draft | Coherencia con el nombre del producto; `.forge` era provisional |
| Workspace file | `.tija-workspace.yaml` | Misma razón |
| TUI en Fase 1 | "Esqueleto" de TUI: startup screen + menú solo con comandos existentes + 4 estados estáticos de la mascota | Valor visual desde el día uno sin bloquear la fase en animaciones |
| Fallback sin agente | Si no hay ningún CLI de agente, Tija genera un borrador determinista o imprime el prompt completo para copiar | El flujo nunca debe depender de la IA para funcionar |
| Test intelligence + E2E | Fase propia (6), después de MCP | Playwright requiere levantar el frontend y gestionar traces; es más laborioso que lint/test |
| Distribución | Binario único vía GoReleaser (GitHub Releases) + `go install` | Simple y multiplataforma; Scoop/Homebrew/winget quedan para después |
| Reglas de PR vs. repo | `init` escribe las reglas por defecto (las de `pr-governance.yml`). **No** parsea el JS de los workflows. Si detecta un workflow de gobernanza, `doctor` lo menciona como `!` informativo ("verify manifest rules match <workflow>"); el usuario ajusta el manifest a mano si difiere | Parsear github-script es frágil; los repos del autor usan esta misma Action |
| Varios templates de PR | Si existe `.github/PULL_REQUEST_TEMPLATE/` con varios archivos, el manifest apunta al primero en orden alfabético y `init` lo informa | Caso raro; se ajusta a mano |
| Herramientas detectadas | `init` llena `quality.*` con lo detectado (p. ej. `lint: { provider: biome }`); `doctor` verifica esas entradas | Así `doctor` tiene algo concreto que comprobar desde la Fase 1 |
| Exit code de `status` | `status` es un reporte: exit 0 aunque la rama no cumpla la convención. `tija status --strict` devuelve 1 si la rama es inválida | Útil para scripts y CI sin romper el uso diario |
| Ramas especiales | `main`, `master`, `dev`, `develop` y la rama `target` se muestran como neutrales ("base branch"), no inválidas. HEAD detached se muestra como `!` | No son ramas de trabajo |
| Nombres de rama no conformes | Nunca bloquean (decisión del usuario: "no debería bloquearle sino recomendarle un nombre"). Se sugiere un nombre conforme que conserva el número de issue | Otros devs crean ramas desde issues de GitHub |
| Re-ejecutar `init` | Nunca reescribe el manifest existente. Solo propone, como diff confirmable, campos nuevos o detectados que falten. Las reglas y templates instalados solo se reemplazan si no fueron editados: el manifest guarda `tija.defaultsVersion` y un hash por archivo instalado | Distingue ediciones del usuario de los defaults viejos |
| `init` no interactivo sin `--yes` | Crea lo que es nuevo dentro de `.tija/`. **No** modifica archivos existentes (`.gitignore`) y lo reporta como pendiente, con exit 0 | Seguro para agentes y MCP |

## 4. Restricciones transversales (aplican a todas las fases)

1. **Un motor, tres interfaces.** La lógica vive en paquetes de engine. Cobra, Bubble Tea y MCP son capas finas que llaman al mismo engine. Ningún workflow se implementa dos veces.
2. **Deterministic first.** Lo que se puede calcular (rama, status, commits, diff, detección de stack, ejecución de herramientas, validación de formato) nunca se delega al LLM. El agente solo redacta, resume e interpreta.
3. **Modo no interactivo siempre disponible.** Todo comando acepta `--json` y `--yes` (o `--no-input`) cuando aplica. Si stdout no es una TTY, no se dibuja la mascota ni colores y la salida es texto plano estable. Exit codes: `0` ok, `1` fallo de validación/check, `2` error de uso/entorno.
4. **No pisar nada.** Cualquier archivo existente que Tija quiera modificar por iniciativa propia (configuración e infraestructura: `.gitignore`, templates, workflows) se muestra como diff y requiere confirmación (o `--yes`). `init` es idempotente. Las ediciones de código fuente que el usuario pide con un flag explícito de escritura (`tija lint --fix`, `tija clean comments --write`) quedan cubiertas por ese flag, como en Biome.
5. **Repository native.** Versionado: `.tija/manifest.yaml`, `.tija/rules/`, `.tija/templates/`, `.tija/workflows/`, `.tija/handoffs/`. Ignorado (vía `.gitignore`): `.tija/cache/`, `.tija/index/`, `.tija/session/`, `.tija/tmp/`.
6. **Agent agnostic.** Los workflows piden un *tier* (`fast` / `balanced` / `deep`), nunca un modelo concreto. Cada provider traduce el tier al flag de modelo de su CLI cuando ese CLI lo soporta; si no, ignora el tier.
7. **Idioma.** Strings de UI en inglés. El idioma del contenido generado se define en el manifest (`content.language: es` por defecto en este usuario) y se pasa al agente en el prompt.
8. **Multiplataforma real.** Windows es plataforma de primera clase (el autor desarrolla en Windows 11). Rutas, subprocesos y detección de binarios deben probarse en Windows.
9. **Branding consistente.** Toda salida coloreada usa solo la paleta C. Símbolos: `✓` éxito, `✗` error, `!` warning, `>` selección.
10. **Escritura en tres niveles.** Todo comando que pueda editar código sigue el contrato de Biome: sin flag solo reporta (exit 1 si hay cambios pendientes), `--write` aplica los cambios seguros y `--write --unsafe` también los que pueden cambiar el significado. Cuando la propuesta viene de un agente, Tija aplica los cambios y los verifica de forma determinista; el agente nunca escribe archivos.
11. **Atribución de terceros.** Todo pack o fragmento de otro proyecto lleva frontmatter de origen (autor, repo, ruta, commit, licencia), una nota visible de adaptación y su `LICENSE` original al lado. `THIRD_PARTY_NOTICES.md` viaja en cada release. Tija nunca descarga packs en tiempo de ejecución.

## 5. Las fases

Orden de menor a mayor esfuerzo. Cada fase termina con un binario usable y un criterio de salida verificable. La Fase 1 se describe con detalle porque es la próxima a planificar; las demás quedan con alcance y criterio de salida, y se detallarán en su propio spec cuando toque.

---

### Fase 1 — Foundation (piloto usable)

**Propósito:** que `tija` se pueda instalar, entienda el repo en el que está, lo prepare con reglas y templates versionados y le diga al usuario si su entorno y su rama están en orden. Esto deja todo listo para las fases de Quality y PR.

**Repos de referencia (validados el 2026-10-22)**

Son repos independientes, cada uno con un solo stack en la raíz. Por eso el manifest v1 tiene un único `project`; no hace falta soporte multi-proyecto.

| | GMX-frontend | GMX-ClaimsAPI |
|---|---|---|
| Stack | React 19 + Vite 7, JavaScript | .NET 8, una solución `ClaimsService.sln` en la raíz, 4 proyectos en `src/` |
| Package manager | `packageManager: bun@1.3.5`, pero conviven `bun.lock` y `package-lock.json` | dotnet |
| Calidad | Biome (`biome ci .` / `biome check --write .`), husky + lint-staged + commitlint | — |
| Tests | No hay | No hay proyectos de test en la `.sln` |
| PR | `.github/PULL_REQUEST_TEMPLATE.md`, `pr-governance.yml`, `pr-checks.yml` | Sin template ni gobernanza; la guía está en `templates/PULL_REQUEST_TEMPLATE_GUIDE.md` |
| Ramas base | `main` | `main`, `dev` |
| Extras | `.csproj` sueltos fuera de la solución en `scratch/` | |

Reglas de detección que salen de estos repos:
- **.NET:** la fuente de verdad es la `.sln` de la raíz y los proyectos que referencia. Los `.csproj` que no están en la solución (como `scratch/`) se ignoran. Si en la raíz hay más de una `.sln`, `init` pregunta cuál usar.
- **Package manager JS:** el campo `packageManager` de `package.json` tiene prioridad sobre los lockfiles. Si conviven varios lockfiles, `doctor` lo reporta como `!`.
- **"No hay tests"** es un estado válido: se muestra `! no tests detected`, no un error.
- Si existe una guía de PR fuera de `.tija/` (por ejemplo `templates/PULL_REQUEST_TEMPLATE_GUIDE.md`), `init` la registra como `pullRequest.rules` en vez de copiar la default.
- En repos sin template de PR (ClaimsAPI), `init` instala el default en `.tija/templates/pull-request.md` y **ofrece** copiarlo también a `.github/PULL_REQUEST_TEMPLATE.md`.
- Se detecta commitlint, y su config, si existe, se registra como regla de commits.

**Comandos**

#### `tija init`
1. Verificar que se está dentro de un repositorio Git (si no, error claro, exit 2).
2. Detectar:
   - lenguaje y framework: .NET (`*.sln`, `*.csproj`, `TargetFramework`), React/Vite (`package.json` con `react` y `vite`). Otros stacks se reportan como `unknown` sin fallar;
   - package manager (npm / pnpm / yarn / bun por lockfile; dotnet);
   - herramientas existentes: Biome, ESLint, Prettier, `dotnet format`, Playwright, xUnit/NUnit/MSTest, Vitest/Jest;
   - tests (proyectos o archivos de test y conteo aproximado);
   - CI: `.github/workflows/*`;
   - host remoto: GitHub (por `git remote`);
   - **artefactos de PR existentes:** `.github/PULL_REQUEST_TEMPLATE.md` (y variantes de ubicación que GitHub acepta) y workflows de gobernanza de PR (p. ej. que usen `action-semantic-pull-request`).
3. Crear `.tija/` con `manifest.yaml`, `rules/`, `templates/`, `workflows/` (vacío por ahora).
4. Instalar reglas y templates por defecto, empaquetados en el binario:
   - `rules/pull-request.md` ← contenido de `PULL_REQUEST_TEMPLATE_GUIDE.md`;
   - `templates/pull-request.md` ← contenido de `PULL_REQUEST_TEMPLATE.md`.
   - Si el repo **ya tiene** un template de PR en `.github/`, no se copia el default: el manifest apunta al existente.
5. Actualizar `.gitignore` con las rutas locales de `.tija/` (mostrando el diff y pidiendo confirmación).
6. Mostrar el resumen "Detected project / Created / Updated" (formato de la sección 22 del draft).
7. Re-ejecutar `init` en un repo ya inicializado no duplica nada. Ofrece actualizar lo detectado y nunca sobrescribe reglas editadas por el usuario sin confirmación.

#### `tija doctor`
Verificaciones, cada una con `✓` / `✗` / `!` y una pista de cómo resolverla:
- Git instalado; repositorio detectado;
- `.tija/manifest.yaml` existe y es válido contra su schema;
- herramientas declaradas en el manifest disponibles en PATH (dotnet, node/npm, biome, …);
- GitHub CLI (`gh`) instalado y autenticado (`gh auth status`);
- CLIs de agentes disponibles: `claude`, `codex` y `gemini` (este último vía Antigravity, binario `agy`, o `gemini`). Se reporta cuáles hay y cuál es el default; que no haya ninguno es `!` (warning), no error;
- template de PR resoluble.

Exit `0` si no hay `✗`, `1` si hay alguno.

#### `tija status`
- Rama actual clasificada en **tres estados. Tija nunca bloquea por el nombre de la rama:**
  - `✓` **conforme:** `<prefijo>/<descripcion-corta>` en minúsculas con `-`, o `release/<version>`. Prefijos aceptados: los de la guía (`feat|fix|refactor|chore|docs|ci|perf|test|build|style|revert`) más los que admite el hook `pre-push` del frontend (`feature|bugfix|hotfix`). Se permite un número de issue al inicio de la descripción: `fix/133-password-leak`.
  - `!` **no conforme, aceptada con recomendación:** cualquier otra cosa, incluidas las ramas creadas desde un issue de GitHub (`101-fixclaims-fix-integration-issues…`) y los nombres genéricos (`changes`, `updates`, `test`, `fix`). Tija propone un nombre conforme que conserva el número de issue (p. ej. `fix/101-claims-integration-issues`) e imprime el comando para renombrarla (`git branch -m <actual> <sugerida>`). Si detecta un hook de husky que valida ramas, avisa: "this repo's pre-push hook will reject this branch".
  - neutral **base branch:** `main`, `master`, `dev`, `develop` y la rama `target`.
- El número de issue, venga del nombre de la rama o del prefijo, se guarda como contexto para que la Fase 3 lo ponga en `## Referencias` (`Closes #101`).
- Los prefijos aceptados son configurables en el manifest (`git.branchPrefixes`). Si `init` detecta un hook `pre-push` con regex de ramas, toma de ahí los prefijos;
- rama destino configurada (`pullRequest.target`) y commits por delante y por detrás de ella;
- estado del árbol de trabajo (staged / unstaged / untracked);
- si hay remoto y si la rama está publicada.

**TUI (esqueleto)**
- `tija` sin argumentos y en TTY abre la startup screen del concepto 06: gato ASCII, logotipo "TIJA", tagline *Repository engineering for humans & agents*, checklist de detección (✓ proyecto, ✓ framework, ✓ GitHub, ✓ N tests), versión en la barra superior y el menú **What do you want to do?**
- El menú de la Fase 1 contiene **solo** `Init`, `Doctor` y `Status`, cada uno con su descripción a la derecha. Si el repo no está inicializado, `Init` aparece primero y resaltado.
- Mascota pixel renderizada con bloques de medio carácter, en 4 estados **estáticos**: `idle`, `scanning` (durante init/doctor), `success`, `error`.
- Navegación con flechas y `enter`, `q` para salir. Al seleccionar una opción se ejecuta el mismo engine que el comando CLI y se muestra su resultado dentro de la TUI.

**Manifest v1 (mínimo para Fase 1)**
```yaml
version: 1
project:
  name: ClaimsAPI
  type: dotnet-api          # dotnet-api | react-vite | unknown
content:
  language: es
git:
  host: github
  branchConvention: conventional   # valida feat/…, fix/…, release/<version>
pullRequest:
  target: dev
  template: .github/PULL_REQUEST_TEMPLATE.md   # o .tija/templates/pull-request.md
  rules: .tija/rules/pull-request.md
  title:
    convention: conventional-commits
    requireScope: true
    maxLength: 100           # de commitlint header-max-length; se toma del commitlint.config si existe
  requiredSections: [Resumen, Objetivo, Cambios realizados, Pruebas]
  minSectionLength: 10
  invalidValues: ["n/a", "na", "no aplica", "pendiente", "todo", "por definir", "...", "-"]
  autoAssign: true
quality: {}                 # se llena en Fase 2; init deja lo detectado como sugerencia
agents:
  default: claude           # el primero detectado; null si no hay ninguno
  tasks:
    pullRequest: { tier: fast }
    review:      { tier: balanced }
    architecture: { tier: deep }
```
Los valores por defecto de `pullRequest.*` replican exactamente lo que valida `pr-governance.yml`, de modo que la validación local de la Fase 3 coincida con la remota. `pullRequest.target` toma por defecto la rama por defecto del remoto; el usuario lo ajusta (p. ej. `dev`).

**Criterios de salida de la Fase 1**
- En un repo .NET real y en un repo React/Vite real (los del autor), `tija init` detecta correctamente el stack y el template de PR existente, crea `.tija/` y es idempotente.
- `tija doctor` reporta correctamente la presencia o ausencia de `gh` y de cada CLI de agente en Windows.
- `tija status` marca `feat/shared-imputation-form` y `bugfix/133-password-leak` como `✓`, y `changes` y `101-fixclaims-fix-integration-issues` como `!`, con un nombre sugerido. Exit 0 en todos los casos, y exit 1 con `--strict` para los `!`.
- `tija`, `tija doctor --json` y `tija doctor | cat` funcionan: la TUI aparece solo en TTY.
- Binario publicado para Windows, Linux y macOS desde un tag.
- Tests automatizados del engine contra repos de ejemplo (fixtures .NET, React/Vite, vacío, sin git).

---

### Fase 2 — Quality
**Comandos:** `tija check` (lint + format-verify + test), `tija lint`, `tija lint --fix` / `tija format`, `tija test`.
**Adapters:** `dotnet` (build, `dotnet format --verify-no-changes`, `dotnet test`), Biome, scripts de `package.json` como fallback (`npm run lint`, `npm run test`, `npm run build`). Interfaz `QualityTool { Detect, Check, Fix }`.
**Clave para la Fase 3:** cada ejecución guarda un resultado estructurado en `.tija/session/` (herramienta, comando, exit code, duración, tests pasados/fallidos, commit al que corresponde) para que `pr prepare` cite **pruebas reales** en `## Pruebas`.
**TUI:** opciones `Check` y `Tests` en el menú, progreso en vivo por herramienta y estado `warning` de la mascota.
**Salida:** `tija check` corre lo correcto sin configuración manual en ambos repos reales y su `--json` es estable.

### Fase 3 — Pull Requests (primer hito de valor)
**Comandos:** `tija pr prepare`, `tija pr create`.
**Flujo:** status → validar rama → commits y diff contra `target` → cargar reglas y template → usar resultados de Quality (o correr `check` si no hay resultados del commit actual) → pedir al agente (tier `fast`) título y descripción en `content.language` → **validar localmente** con las reglas del manifest (Conventional Commits con scope, secciones obligatorias, longitud mínima, valores inválidos) → si falla, reintentar o reportar → preview editable → aprobación → push → `gh pr create` → autoasignación.
**AgentProvider:** adaptadores por CLI (`claude -p`, `codex exec`, `agy --dangerously-skip-permissions` para Gemini vía Antigravity, con `gemini` como alternativa), invocados como subproceso con el prompt por stdin, timeout y captura de salida. Fallback sin agente: borrador determinista (tipo y scope inferidos de la rama, cambios desde los commits agrupados, pruebas desde la sesión) o `--print-prompt`.
**Reglas de contenido (de la guía):** no inventar pruebas, endpoints ni funcionalidades; mantener los nombres técnicos tal como están en el código; resumir commits por funcionalidad en lugar de copiarlos; `Evidencias: N/A` si no hay.
**TUI:** opción `Pull Request`, estado `thinking` de la mascota y preview con el título y el cuerpo.
**Salida:** en 5 PRs reales consecutivos, `pr-governance.yml` pasa a la primera y el autor no necesitó ChatGPT.

### Fase 3a — Skill packs, limpieza de comentarios y workflows compuestos
**Spec propio:** `2026-10-29-tija-phase-3a-skill-packs-design.md`.
**Comandos:** `tija skills list|show`, `tija clean comments [rutas] [--write] [--unsafe] [--diff] [--all] [--no-agent]`, `tija run <workflow>` / `tija run --list`.
**Skill packs:** rúbricas en `.tija/skills/<pack>/SKILL.md`, embebidas e instaladas por `init` con el mecanismo de `defaultsVersion` + hash. Los de terceros llevan origen, licencia y nota de adaptación (restricción 11). En esta fase se adaptan `antislop-code` (de anti-slop, Miqdad Badjuber, MIT) y `thermo-review` / `thermo-quality` (de Thermos, Cursor, MIT); los dos últimos se usan en la Fase 4. También `antislop-copywriting` (extracto) para el prompt de `pr prepare`.
**Copy de los PR:** Tija quita de forma determinista las atribuciones de agentes (`🤖 Generated with …`, `Co-Authored-By: Claude …`) de los commits que entran al prompt y del cuerpo que devuelve el agente, y `prrules` las reporta si sobreviven.
**Verificación de ediciones:** flujo de tokens sin comentarios (Go, siempre) + herramienta oficial del stack: Biome en JS/TS, `dotnet format` en .NET cuando se agregue C#. esbuild descartado por peso (+6.7 MB).
**`clean comments`:** Tija extrae los comentarios con un lexer determinista, protege las directivas (`biome-ignore`, `@ts-*`, licencias, JSDoc de tipos), aplica reglas deterministas, pide al agente (tier `fast`) solo una clasificación JSON por comentario, planifica las ediciones y verifica que el flujo de tokens sin comentarios no cambió antes de escribir. Flags con la semántica de Biome (restricción 10).
**Workflows compuestos:** `.tija/workflows/*.yaml` con pasos que solo pueden ser comandos de Tija (nunca shell). Por defecto, `pre-pr` = `clean comments --write` → `check` → `pr prepare`.
**TUI:** opciones `Clean comments` y `Workflows`.
**Salida:** en `src/pages/(features)/claims` del frontend de Team4soft, `--write` elimina los separadores y los banners, deja intactos los `biome-ignore` y los comentarios de reglas de negocio, y `tija check` y el build siguen pasando.

### Fase 4 — Review
**Comando:** `tija review` → diff → análisis estático (Quality) → tests → reglas de arquitectura de `.tija/rules/` → agente con un skill pack de Thermos.
**Modos:**
- `tija review`: pack `thermo-review` (bugs, cambios que rompen funcionalidad, seguridad, cambios que afectan al flujo local de los devs, fugas de feature flags), tier `balanced`.
- `tija review --quality`: pack `thermo-quality` (mantenibilidad: simplificaciones, regla de las 1000 líneas, condicionales sueltos, límites de tipos, capa correcta), tier `balanced`.
- `tija review --deep`: los dos en paralelo (goroutines, dos llamadas al provider) y una síntesis que deduplica por archivo, línea y regla, pondera más los hallazgos en los que coinciden ambos y resuelve los desacuerdos. Tier `deep`. La orquestación sigue la idea del skill `thermos` de Cursor, reimplementada en Go y atribuida.
- Todos limitados al diff contra `pullRequest.target`. Si hay un PR abierto, los comentarios del PR se leen con `gh` **después** de la auditoría propia (regla de Thermos).
**Salida:** findings con archivo, línea, severidad (Critical/High/Medium/Low), regla, explicación y posible solución; resumen por severidad; `--json`. Se conserva la regla de Thermos de no inflar severidades. `review` **no escribe** archivos en esta fase. TUI con estado `warning / needs review`. El workflow `pre-pr` agrega `review` antes de `pr prepare`.

### Fase 5 — MCP
**Comando:** `tija mcp serve` (stdio). Expone el mismo engine como tools: `repo.status`, `repo.rules`, `repo.context`, `quality.check`, `quality.fix`, `tests.run`, `comments.clean`, `skills.list`, `workflow.run`, `pr.prepare`, `pr.create`, `review.run`. Las tools que escriben reciben `write` y `unsafe` como parámetros explícitos, con la misma semántica que el CLI.
**Salida:** Claude Code y Codex pueden preparar un PR válido a través de MCP con exactamente las mismas reglas que el CLI.

### Fase 6 — Test intelligence y E2E
`tija test affected` (tests relacionados con los archivos del diff), `tija test suggest` (escenarios sugeridos por el agente) y `tija test e2e` con el adapter Playwright (detectar config, levantar el frontend si hace falta, ejecutar, recoger traces y screenshots, generar reporte).

### Fase 7 — Handoffs entre repositorios
`tija handoff create --target <repo>` genera `.tija/handoffs/<id>.yaml` (endpoints, contratos, sugerencias para frontend). `tija handoff consume <ruta>` lo usa como contexto en el repo destino.

### Fase 8 — Workspace multi-repo
`.tija-workspace.yaml`, `tija workspace status`, `tija workspace context`: detección de repos, lectura de sus manifests y vista agregada.

### Fase 9 — Contexto semántico (Qdrant)
`tija workspace index`, `tija context search "<pregunta>"`: chunks, embeddings e índice en Qdrant para consultas cross-repo.

### Fase 10 — Extensiones
`tija test generate`, `tija setup dependencies` (genera la config de Renovate), adapter Ruff para Python, animaciones completas de la mascota (blink, typing, walking), distribución vía Scoop/Homebrew/winget.

## 6. Arquitectura de referencia

```text
cmd/tija            → main, wiring de Cobra
internal/engine     → workflows (init, doctor, status, …): entrada/salida tipadas
internal/detect     → detección de stack, herramientas, CI, artefactos de PR
internal/manifest   → schema, carga, validación, escritura
internal/gitx       → operaciones git (vía binario git)
internal/agents     → AgentProvider + adaptadores CLI (Fase 3)
internal/quality    → QualityTool + adapters (Fase 2)
internal/skills     → skill packs: carga, origen y licencia (Fase 3a)
internal/comments   → lexer JS/TS/JSX, reglas deterministas, verificador (Fase 3a)
internal/workflows  → workflows compuestos de .tija/workflows/ (Fase 3a)
internal/tui        → Bubble Tea: startup screen, menú, mascota, vistas
internal/render     → salida CLI (texto con paleta, plano, JSON)
assets/             → reglas, templates, skill packs y workflows por defecto embebidos (go:embed), sprites
THIRD_PARTY_NOTICES.md → atribución y licencias del material de terceros (Fase 3a)
```
Esta estructura es orientativa: la organización interna exacta queda a criterio del implementador mientras se respete la regla de un único motor.

## 7. Lo que queda a criterio del implementador
- Librerías concretas de apoyo (Lip Gloss para estilos, librería YAML, librería de schema/validación, SDK de MCP en Go en su fase).
- Usar el binario `git` o una librería Go para operaciones git. Se recomienda el binario por fidelidad con el comportamiento del usuario.
- Diseño pixel exacto de los sprites, siempre fiel a la imagen de marca y legible en terminales de 80 columnas.
- Formato exacto del JSON de salida, siempre que sea estable y esté documentado.
- Estrategia de tests (fixtures en `testdata/`, golden files para la salida).
