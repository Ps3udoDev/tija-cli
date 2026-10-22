# Borrador Técnico — Tija

> Estado: borrador de producto / arquitectura
> Objetivo: definir una primera versión viable y una ruta de evolución para una herramienta local de Developer Experience orientada a repositorios, automatización de workflows y agentes de código.

## 1. Idea central

La propuesta consiste en construir una herramienta **CLI-first, TUI opcional y agnóstica al agente de IA**, capaz de instalarse o inicializarse dentro de un repositorio para estandarizar tareas de ingeniería como:

- inicialización del contexto del repositorio;
- reglas e instrucciones para agentes;
- validaciones de calidad;
- lint y format;
- ejecución de pruebas;
- preparación y creación de Pull Requests;
- revisión de cambios;
- generación de handoffs entre repositorios;
- exposición de capacidades mediante MCP;
- contexto compartido entre múltiples repositorios;
- integración con agentes como Claude, Codex, Gemini, Kimi u otros.

Principio central:

> **Los workflows, las reglas y el contexto pertenecen al repositorio; el agente es solamente un ejecutor intercambiable.**

## 2. Qué producto se está intentando construir

Definición recomendada:

> **Una plataforma local de Developer Experience, CLI-first y agent-agnostic, que estandariza workflows de ingeniería dentro de cada repositorio y permite exponerlos de forma consistente tanto a desarrolladores humanos como a agentes de código.**

Otra forma más corta:

> **Repository-native engineering orchestrator for humans and AI agents.**

El producto estaría entre varias categorías:

- repository bootstrapper;
- workflow orchestrator;
- developer cockpit;
- quality gateway;
- agent bridge;
- context engine;
- local engineering automation platform;
- engineering governance as code.

No debe plantearse como un reemplazo de Ruff, Biome, Playwright, Renovate, GitHub Actions o los agentes. Su valor es **orquestarlos bajo una interfaz y un contrato común**.

## 3. Principios de diseño

### 3.1 Agent agnostic

La herramienta no debe acoplarse a Claude, Codex, Gemini, Kimi o cualquier proveedor.

Abstracción conceptual:

```text
AgentProvider
├── ClaudeProvider
├── CodexProvider
├── GeminiProvider
├── KimiProvider
└── LocalProvider
```

### 3.2 Deterministic first

Las tareas deterministas no deberían delegarse a un LLM.

Ejemplos:

- detectar la rama actual;
- revisar `git status`;
- validar nombres de rama;
- obtener commits;
- calcular diff;
- detectar lenguaje/framework;
- ejecutar lint;
- ejecutar tests;
- validar archivos;
- detectar herramientas instaladas;
- crear estructura de carpetas.

Los agentes deberían encargarse de:

- resumir cambios;
- generar descripciones;
- revisar lógica;
- interpretar reglas de negocio;
- sugerir pruebas;
- analizar arquitectura;
- generar código cuando se solicite.

### 3.3 Repository native

Las reglas y workflows deberían viajar con el repositorio.

Estructura sugerida:

```text
.forge/
├── manifest.yaml
├── rules/
├── templates/
├── workflows/
└── agents/
```

Archivos versionados:

```text
.forge/rules/
.forge/templates/
.forge/workflows/
```

Archivos locales ignorados:

```text
.forge/cache/
.forge/index/
.forge/session/
.forge/tmp/
```

### 3.4 Orchestrate, don't reinvent

Integrar herramientas especializadas en lugar de reemplazarlas:

```text
Biome      → lint + format para JS/TS
Ruff       → lint + format para Python
Playwright → E2E
dotnet     → build + test + format
Renovate   → actualizaciones de dependencias
Git        → historial y diff
GitHub     → Pull Requests y checks
```

### 3.5 CLI primero, TUI como capa de experiencia

Todos los workflows importantes deben ser ejecutables mediante comandos no interactivos:

```bash
tija lint
tija test
tija pr create --yes
```

La TUI sería una interfaz adicional:

```bash
tija
```

Esto facilita automatización, scripting, CI, agentes y uso humano interactivo.

## 4. Stack tecnológico

### Go

Lenguaje principal por:

- binario único;
- distribución simple;
- soporte Windows/Linux/macOS;
- buen rendimiento;
- concurrencia;
- filesystem;
- subprocesses;
- HTTP;
- ecosistema sólido para herramientas CLI.

### Cobra

Capa CLI.

Ejemplo:

```text
tija
├── init
├── doctor
├── lint
├── test
├── review
├── pr
│   ├── prepare
│   └── create
├── context
│   ├── build
│   ├── export
│   └── import
├── handoff
│   ├── create
│   └── consume
└── mcp
    └── serve
```

### Bubble Tea

TUI opcional para una experiencia viva dentro de terminal.

Ejemplo:

```text
┌────────────────────────────────────────────┐
│ Project: ClaimsAPI                         │
├────────────────────────────────────────────┤
│ Repository                                 │
│ ✓ Git                                      │
│ ✓ Branch valid                             │
│ ✓ Dependencies                             │
│                                            │
│ Workflows                                  │
│ > Create Pull Request                      │
│   Review Changes                           │
│   Run Quality Checks                       │
│   Run Tests                                │
│   Export Context                           │
│   Repository Doctor                        │
└────────────────────────────────────────────┘
```

## 5. Arquitectura inicial

```text
                 ┌───────────────────┐
                 │    Bubble Tea     │
                 │       TUI         │
                 └─────────┬─────────┘
                           │
┌───────────────┐          ▼
│     Cobra     │ ───► Workflow Engine
│      CLI      │          ▲
└───────────────┘          │
                           │
                 ┌─────────┴─────────┐
                 │     MCP Server    │
                 └───────────────────┘
                           │
                           ▼
                 Repository Engine
                           │
        ┌──────────────────┼─────────────────┐
        ▼                  ▼                 ▼
     Quality            Context          Workflows
        │                  │                 │
  Biome/Ruff          Git/AST         PR / Review
  dotnet              SQLite          Test / Handoff
  Playwright          Qdrant*         Context
```

`Qdrant*` se reserva para una fase posterior.

## 6. MCP como pieza central

MCP puede convertirse en uno de los elementos diferenciales del producto.

Comando:

```bash
tija mcp serve
```

Capacidades posibles:

```text
repo.status
repo.rules
repo.context
quality.check
quality.fix
tests.run
tests.suggest
pr.prepare
pr.create
review.run
handoff.create
handoff.consume
```

Esto permite que distintos agentes utilicen exactamente los mismos workflows y reglas.

El CLI, la TUI y MCP deben compartir el mismo motor. No deben existir tres implementaciones distintas.

## 7. Abstracción de agentes

Ejemplo conceptual:

```go
type AgentProvider interface {
    Run(ctx AgentContext, task Task) (AgentResult, error)
}
```

Providers:

```text
ClaudeProvider
CodexProvider
GeminiProvider
KimiProvider
LocalProvider
```

## 8. Selección de modelos por capacidad

Evitar nombres de modelos directamente en los workflows.

Usar tiers:

```text
fast
balanced
deep
```

Ejemplo:

```yaml
agents:
  tasks:
    pullRequest:
      tier: fast

    review:
      tier: balanced

    architecture:
      tier: deep
```

Cada provider decide qué modelo concreto corresponde a cada tier.

## 9. Manifest del proyecto

Archivo:

```text
.forge/manifest.yaml
```

Ejemplo .NET:

```yaml
version: 1

project:
  name: ClaimsAPI
  type: dotnet-api

quality:
  formatter:
    provider: dotnet-format

  lint:
    provider: dotnet

  tests:
    provider: dotnet-test

pullRequest:
  template: templates/pull-request.md
  target: dev

agents:
  default: codex

  tasks:
    pullRequest:
      tier: fast

    review:
      tier: balanced

    architecture:
      tier: deep
```

Ejemplo React:

```yaml
project:
  name: GMX-frontend
  type: react-vite

quality:
  lint:
    provider: biome

  e2e:
    provider: playwright
```

## 10. Estructura recomendada en un repositorio

```text
.forge/
├── manifest.yaml
├── rules/
│   ├── general.md
│   ├── pull-request.md
│   ├── testing.md
│   └── architecture.md
├── templates/
│   ├── pull-request.md
│   ├── bug-report.md
│   └── context-handoff.md
├── workflows/
│   ├── pull-request.yaml
│   ├── review.yaml
│   ├── lint.yaml
│   └── tests.yaml
├── handoffs/
└── agents/
```

Local / ignorado:

```text
.forge/cache/
.forge/index/
.forge/session/
.forge/tmp/
```

## 11. Quality adapters

Interfaz conceptual:

```go
type QualityTool interface {
    Detect(repo Repo) bool
    Check(ctx Context) Result
    Fix(ctx Context) Result
}
```

Adapters:

```text
BiomeProvider
RuffProvider
DotNetProvider
PlaywrightProvider
```

Esto permite:

```bash
tija check
```

sin que el usuario tenga que recordar cuál herramienta usa cada proyecto.

## 12. Playwright

Playwright sí forma parte de la propuesta, como proveedor E2E y no como parte del core.

Ejemplo:

```yaml
quality:
  e2e:
    provider: playwright
```

Comando:

```bash
tija test e2e
```

Responsabilidades posibles:

1. detectar configuración Playwright;
2. iniciar frontend si hace falta;
3. ejecutar E2E;
4. recoger traces;
5. recoger screenshots;
6. generar reporte;
7. entregar resultados al agente.

La generación automática de tests se reserva para una fase posterior.

## 13. Renovate

No reimplementar Renovate.

La herramienta podría configurarlo:

```bash
tija setup dependencies
```

y generar:

```text
renovate.json
```

o aplicar presets internos.

## 14. Qdrant — fase posterior

Qdrant cobra valor cuando existen varios repositorios relacionados en un mismo workspace.

Ejemplo:

```text
workspace/
├── SecurityAPI/
├── CommonAPI/
├── ClaimsAPI/
├── GatewayAPI/
└── Frontend/
```

En una fase posterior:

```bash
tija workspace index
```

podría:

1. detectar repositorios;
2. generar chunks relevantes;
3. indexar código, documentación y contratos;
4. generar embeddings;
5. guardar el índice en Qdrant;
6. permitir consultas cross-repository.

Ejemplo:

```bash
tija context search "cómo se autentica el frontend contra SecurityAPI"
```

El contexto podría recuperar información de varios repositorios a la vez.

## 15. Workspace multi-repo

Archivo opcional:

```text
.forge-workspace.yaml
```

Ejemplo:

```yaml
version: 1

workspace:
  name: GMX

repositories:
  - ./SecurityAPI
  - ./CommonAPI
  - ./TelefonicaAPI
  - ./GatewayAPI
  - ./frontend
```

Fase inicial:

- detección;
- lectura de manifests;
- handoffs.

Fase avanzada:

- índice semántico;
- búsqueda cross-repo;
- Qdrant.

## 16. Handoff entre repositorios

Característica útil incluso antes de Qdrant.

Backend:

```bash
tija handoff create --target frontend
```

Genera:

```text
.forge/handoffs/claims-close.yaml
```

Ejemplo:

```yaml
version: 1

handoff:
  id: claims-close-flow

source:
  repository: ClaimsAPI

changes:
  endpoints:
    - method: POST
      path: /api/claims/closes

contracts:
  request:
    file: CloseClaimRequestDto.cs

  response:
    file: CloseClaimResponseDto.cs

frontend:
  suggested:
    service: claimsCloseService
    hook: useCloseClaim
    test: close-claim.spec.ts
```

En frontend:

```bash
tija handoff consume ../ClaimsAPI/.forge/handoffs/claims-close.yaml
```

o:

```bash
tija continue claims-close.yaml
```

## 17. Pull Requests como primer workflow real

Comando:

```bash
tija pr create
```

Flujo:

```text
1. git status
2. detectar rama
3. validar nombre
4. obtener commits
5. calcular diff
6. cargar reglas
7. ejecutar checks
8. seleccionar agent tier = fast
9. generar título
10. generar descripción
11. mostrar preview
12. usuario aprueba
13. push
14. abrir Pull Request
```

## 18. Review

Comando:

```bash
tija review
```

Orden recomendado:

```text
git diff
→ static analysis
→ tests
→ architecture rules
→ agent review
```

Salida:

```text
Critical      0
High          1
Medium        2
Low           4
```

Cada finding debería incluir:

- archivo;
- línea;
- severidad;
- regla;
- explicación;
- posible solución.

## 19. Testing

La primera versión no debe intentar generar automáticamente todos los tests.

Primero:

```bash
tija test
tija test affected
```

Luego:

```bash
tija test suggest
```

Ejemplo:

```text
Cambió:
CloseClaimService.cs

Tests potencialmente afectados:
CloseClaimServiceTests.cs

Escenarios sugeridos:
- usuario no titular
- reclamo cerrado
- conflicto de concurrencia
```

En una fase posterior:

```bash
tija test generate
```

Prioridad:

> test intelligence

antes que:

> test generation

## 20. Qué NO incluir en el MVP

No incluir inicialmente:

- Qdrant;
- generación completa de tests;
- editor gráfico;
- Konva;
- sistema propio de dependency updates;
- análisis semántico cross-repo avanzado;
- dashboard web;
- almacenamiento remoto;
- arquitectura distribuida.

## 21. MVP recomendado

### Versión 0.1

Comandos:

```bash
tija init
tija doctor
tija check
tija lint
tija test
tija pr prepare
tija pr create
tija review
```

Soporte inicial:

```text
Git
GitHub
.NET
React/Vite
Biome
dotnet
Playwright como runner
```

Providers iniciales:

```text
Codex
Claude
Gemini
```

## 22. Qué debería hacer `tija init`

Este debería ser uno de los primeros workflows.

Proceso:

```text
1. detectar repositorio Git
2. detectar lenguaje
3. detectar framework
4. detectar package manager
5. detectar herramientas existentes
6. detectar tests
7. detectar CI
8. crear .forge/
9. crear manifest.yaml
10. crear rules/
11. crear templates/
12. actualizar .gitignore
13. generar resumen del proyecto
```

Ejemplo:

```text
Detected project

Language:       C#
Framework:      .NET 8
Tests:          xUnit
Git:            GitHub
Formatter:      dotnet format
CI:             GitHub Actions

Created:
✓ .forge/manifest.yaml
✓ .forge/rules/
✓ .forge/templates/
✓ .forge/workflows/

Updated:
✓ .gitignore
```

## 23. `tija doctor`

Comando:

```bash
tija doctor
```

Verificaciones:

```text
✓ Git installed
✓ repository detected
✓ manifest valid
✓ required tools installed
✓ test runner available
✓ lint provider available
✓ GitHub CLI available
✓ agent provider configured
✓ MCP available
```

## 24. Hoja de ruta

### Fase 1 — Repository Foundation

```text
tija init
tija doctor
tija status
```

### Fase 2 — Quality

```text
tija check
tija lint
tija test
```

Adapters:

```text
dotnet
Biome
Playwright
```

### Fase 3 — Pull Requests

```text
tija pr prepare
tija pr create
```

### Fase 4 — Review

```text
tija review
```

### Fase 5 — MCP

```bash
tija mcp serve
```

### Fase 6 — Handoffs

```text
tija handoff create
tija handoff consume
```

### Fase 7 — Workspace

```text
.forge-workspace.yaml
tija workspace status
tija workspace context
```

### Fase 8 — Qdrant

```text
tija workspace index
tija context search
```

## 25. Posibles nombres del producto

Estas propuestas deben validarse posteriormente en GitHub, dominios y marcas.

1. **Larga** — inspirado en Largatija.
2. **Tija** — derivado de Largatija; corto y memorable.
3. **Laji** — variante inspirada en Largatija.
4. **Forge** — descriptivo, pero muy utilizado.
5. **Kora** — corto y técnico.
6. **Neko** — referencia a gato, aunque bastante usado.
7. **Moki** — corto y amigable.
8. **Kito** — breve y fácil de pronunciar.
9. **Runi** — inspirado en “run”.
10. **Lumo** — asociado a guía y claridad.

## 26. Recomendación de naming

Para una identidad realmente diferenciada, evitar nombres demasiado genéricos como:

```text
Forge
DevKit
DevFlow
AgentKit
RepoOps
```

La idea de usar a **Largatija**, la gata naranja, como inspiración aporta:

- identidad propia;
- mascota;
- historia;
- diferenciación;
- iconografía reconocible;
- personalidad visual.

Los derivados más interesantes:

```text
Larga
Tija
Laji
```

La recomendación provisional es:

```text
Tija
```

Porque funciona bien en CLI:

```bash
tija init
tija doctor
tija check
tija pr
tija review
tija mcp serve
```

## 27. Dirección visual

Concepto:

> Un pequeño gato naranja técnico que representa un coordinador de herramientas.

No debería parecer un robot genérico.

Elementos posibles:

- orejas triangulares;
- silueta compacta;
- cola formando un símbolo de terminal;
- ojos simples;
- marca sutil `>_`;
- buena lectura en 24x24 px.

## 28. Estilo de logo recomendado

### Opción principal: vector minimalista

Características:

- forma compacta;
- bordes suaves;
- máximo 2–3 formas;
- buena lectura en terminal;
- usable como favicon;
- usable como icono CLI;
- usable monocromático.

La cola o el pecho podrían formar sutilmente:

```text
>_
```

### Opción secundaria: pixel art

Mascota naranja en estilo 16/32 bits.

Ventajas:

- memorable;
- conecta con desarrolladores;
- útil en README;
- permite animaciones pequeñas en la TUI.

Estados posibles:

```text
idle
checking
thinking
success
warning
error
```

La recomendación es mantener **logo oficial vectorial** y usar **pixel art como mascota de terminal**.

## 29. Paletas propuestas

### Paleta A — Orange Terminal

```text
Burnt Orange   #F97316
Deep Navy      #0F172A
Warm Cream     #FFF7ED
Amber          #F59E0B
Slate          #475569
```

### Paleta B — Cat + Code

```text
Tangerine      #FF7A1A
Graphite       #1F2937
Terminal Green #22C55E
Cloud          #F8FAFC
Indigo         #6366F1
```

### Paleta C — Minimal Orange

```text
Orange         #F97316
Ink            #111827
White          #FFFFFF
Sand           #FED7AA
```

Recomendación inicial: **Paleta C**.

## 30. Propuesta visual final

### Nombre provisional

```text
Tija
```

### Concepto

> Small orange cat that orchestrates developer tools.

### Estilo

```text
vectorial minimalista
```

con mascota pixel-art secundaria para la TUI.

### Paleta

```text
#F97316  Orange
#111827  Ink
#FFFFFF  White
#FED7AA  Sand
```

### Símbolo

Un gato compacto cuya cola o pecho forme sutilmente:

```text
>_
```

Debe comunicar:

```text
terminal
workflow
agent
developer tools
```

sin parecer otro asistente de IA genérico.

## 31. Ejemplo de inicio

```text
        /\_/\
       ( o.o )
        > ^ <

          TIJA

Repository engineering for humans & agents

✓ ClaimsAPI detected
✓ .NET 8
✓ GitHub
✓ 74 tests

What do you want to do?

> Pull Request
  Review
  Check
  Tests
  Context
```

## 32. Taglines

Opciones:

```text
Repository workflows for humans and agents.
One workflow. Any agent.
Your repository. Your rules. Any agent.
Engineering workflows that travel with your code.
```

Recomendación:

> **Your repository. Your rules. Any agent.**

## 33. Primera definición de producto

> **Tija is a repository-native engineering orchestrator for humans and AI agents.**
>
> It detects the technology stack of a repository, installs versioned rules and templates, integrates the project's existing quality tools, exposes workflows through CLI, TUI and MCP, and allows any compatible coding agent to operate under the same engineering standards.
>
> The system starts with local workflows such as repository initialization, quality checks, tests, Pull Request preparation and code review. Later versions can add cross-repository handoffs, workspace awareness and semantic context indexing with Qdrant.

## 34. Próximo paso recomendado

Construir un spike pequeño que valide la arquitectura.

Objetivo:

```text
tija init
tija doctor
tija pr prepare
```

Debe ser capaz de:

1. detectar un repositorio real;
2. detectar .NET o React;
3. crear `.forge/`;
4. generar `manifest.yaml`;
5. instalar/copiar el template de Pull Request;
6. actualizar `.gitignore`;
7. leer branch + commits + diff;
8. generar un borrador de PR;
9. mostrarlo en terminal;
10. opcionalmente pedir a un agent provider que mejore el texto.

Si este flujo aporta valor en un repositorio real, continuar con:

```text
review
MCP
handoffs
workspace
Qdrant
```

## 35. Resumen de decisiones actuales

```text
Go             → Sí, lenguaje principal
Cobra          → Sí, CLI
Bubble Tea     → Sí, TUI
MCP            → Sí, pieza central
Biome          → Adapter
Ruff           → Adapter para Python cuando aplique
Playwright     → Adapter E2E
Renovate       → Integración/configuración, no reimplementación
Qdrant         → Fase posterior multi-repo
Konva          → Fuera de alcance
Test generation→ Fase posterior
```

Primera versión:

```text
init
doctor
check
test
pr
review
```

La arquitectura debe estar preparada desde el inicio para que:

```text
CLI
TUI
MCP
```

utilicen el mismo motor de workflows.
