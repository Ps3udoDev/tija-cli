# Pull Request Instructions and Template

Este documento define las reglas que debe seguir cualquier agente o desarrollador al preparar y abrir un Pull Request en el proyecto.

El objetivo es mantener una estructura consistente, clara y fácil de revisar en todos los PRs.

---

## 1. Cuándo utilizar este template

Usar estas instrucciones siempre que se solicite alguna acción como:

- "Sube los cambios."
- "Crea el Pull Request."
- "Abre un PR."
- "Prepara el PR."
- "Genera el title y description."
- "Publica los cambios."
- "Haz push y abre el PR."

Antes de abrir el PR, el agente debe revisar los cambios realizados, los commits de la rama y el objetivo funcional del trabajo.

---

## 2. Convención para nombres de ramas

Las ramas deben utilizar una convención compatible con Conventional Commits.

Formatos recomendados:

```text
feat/<descripcion-corta>
fix/<descripcion-corta>
refactor/<descripcion-corta>
chore/<descripcion-corta>
docs/<descripcion-corta>
ci/<descripcion-corta>
release/<version>
```

Ejemplos:

```text
feat/shared-imputation-form
fix/providers-bank-accounts-create-mode
refactor/claims-api-cleanup
feat/worker-progress-enrichment
fix/security-user-modals
```

Reglas:

- Usar minúsculas.
- Separar palabras con `-`.
- Evitar nombres genéricos como `changes`, `updates`, `test` o `fix`.
- El nombre debe describir claramente el alcance principal.
- No incluir nombres personales.
- Si el cambio corresponde a una migración, puede utilizarse una estructura como:

```text
feat/migrate-claims-catalogs-and-close-endpoints
```

---

## 3. Convención para el título del Pull Request

El título del PR debe seguir Conventional Commits.

Formato:

```text
type(scope): description
```

Tipos permitidos normalmente:

```text
feat
fix
docs
style
refactor
perf
test
build
ci
chore
revert
```

Ejemplos:

```text
feat(claims): add catastrophe management workflow
fix(providers): reset bank accounts modal and improve layout
refactor(claims): standardize API endpoints
feat(shared-forms): add reinsurance selection workflow
fix(security): correct user modal UI and catalog loading
```

Reglas:

- El tipo debe estar en minúsculas.
- Siempre incluir `scope` cuando aplique.
- Mantener el título corto y descriptivo.
- No terminar el título con punto.
- Evitar títulos como:
  - `Feature/application module`
  - `Fix bug`
  - `Changes`
  - `Updates`
- El título debe describir el cambio principal, no cada detalle del PR.

---

## 4. Cómo construir la descripción del PR

La descripción debe basarse en:

1. Los cambios reales de la rama.
2. Los commits realizados.
3. La funcionalidad implementada o corregida.
4. Las pruebas ejecutadas.
5. El contexto funcional del módulo.

No inventar pruebas que no hayan sido realizadas.

Si no existen evidencias, usar:

```text
N/A
```

Si existen capturas, videos o enlaces, agregarlos en la sección `Evidencias`.

---

## 5. Estructura obligatoria del Pull Request

Todos los PR deben utilizar esta estructura:

```md
## Resumen

<!-- Describe brevemente qué cambia en este PR. -->

## Objetivo

<!-- Explica qué problema resuelve o qué funcionalidad agrega. -->

## Cambios realizados

<!-- Lista los cambios principales implementados. -->

## Pruebas

<!-- Indica cómo se validaron los cambios. -->

## Evidencias

<!-- Adjunta captura, video o escribe N/A si no aplica. -->

## Referencias

<!-- Issues, tickets, módulos, formularios legacy o contexto relacionado. -->
```

---

## 6. Reglas para cada sección

### Resumen

Debe explicar en uno o dos párrafos qué contiene el PR.

Ejemplo:

```md
## Resumen

Se implementa el flujo de selección de reaseguros dentro de `CommonAPI`, incluyendo DTOs, repositorio, servicio, endpoints y registro de dependencias.

También se deja preparada la integración con el formulario compartido correspondiente en frontend.
```

---

### Objetivo

Debe explicar el propósito funcional o técnico del cambio.

Ejemplo:

```md
## Objetivo

Migrar y exponer la funcionalidad requerida para soportar el formulario `FrmReasSeleccion` dentro de la arquitectura de `shared forms`.

El objetivo principal es permitir que el frontend consuma los endpoints necesarios manteniendo una separación clara entre aplicación, dominio, infraestructura y API.
```

---

### Cambios realizados

Usar bullets claros y concretos.

Ejemplo:

```md
## Cambios realizados

- Se agregaron los DTOs requeridos por el flujo.
- Se creó el repositorio correspondiente.
- Se implementó la lógica de negocio en el servicio.
- Se agregaron nuevos endpoints REST.
- Se registraron las dependencias necesarias.
- Se actualizaron configuraciones relacionadas.
```

No agregar cambios que no existan realmente.

---

### Pruebas

Describir únicamente pruebas ejecutadas o validaciones confirmadas.

Ejemplo:

```md
## Pruebas

- Se validó la compilación del proyecto.
- Se verificó el registro correcto de dependencias.
- Se probaron los endpoints agregados.
- Se realizaron pruebas manuales sobre el flujo actualizado.
```

Si existen resultados específicos, incluirlos:

```md
- 74 pruebas superadas.
- 0 pruebas fallidas.
- `dotnet format --verify-no-changes` sin diferencias pendientes.
```

---

### Evidencias

Si existen imágenes:

```md
## Evidencias

<img width="1580" height="850" alt="image" src="<URL_DE_LA_IMAGEN>" />
```

Si no existen:

```md
## Evidencias

N/A
```

---

### Referencias

Incluir contexto útil como:

- Issue relacionado.
- Nombre del formulario legacy.
- Nombre del módulo.
- Endpoint principal.
- Ticket.
- Rama de origen.
- Flujo funcional relacionado.

Ejemplo:

```md
## Referencias

Relacionado con la migración backend de `frmImpReas` y la implementación del módulo compartido de imputaciones REAS dentro de `CommonAPI`.
```

---

## 7. Template final para copiar

```md
## Resumen

[Describe brevemente qué cambia en este PR.]

## Objetivo

[Explica qué problema resuelve, qué funcionalidad agrega o qué flujo mejora.]

## Cambios realizados

- [Cambio principal 1]
- [Cambio principal 2]
- [Cambio principal 3]
- [Cambio principal 4]

## Pruebas

- [Prueba o validación 1]
- [Prueba o validación 2]
- [Prueba o validación 3]

## Evidencias

N/A

## Referencias

[Issue, ticket, formulario, módulo o contexto relacionado.]
```

---

## 8. Flujo recomendado para un agente

Cuando el usuario solicite "sube los cambios" o equivalente, seguir este orden:

1. Revisar `git status`.
2. Revisar los cambios pendientes.
3. Revisar los commits de la rama.
4. Confirmar el nombre de la rama.
5. Validar que el nombre cumpla la convención.
6. Ejecutar las pruebas o validaciones correspondientes al proyecto.
7. Crear los commits necesarios siguiendo Conventional Commits.
8. Hacer push de la rama.
9. Preparar el título del PR usando:
   `type(scope): description`
10. Generar la descripción usando este template.
11. Abrir el Pull Request hacia la rama destino correcta.
12. Verificar que las GitHub Actions del PR hayan sido disparadas.

---

## 9. Criterios para elegir el tipo del PR

### `feat`

Usar cuando se agrega una nueva funcionalidad.

Ejemplos:

```text
feat(claims): add catastrophe management workflow
feat(shared-forms): add reinsurance query modal
```

### `fix`

Usar cuando se corrige un error existente.

Ejemplos:

```text
fix(providers): reset bank accounts modal state
fix(security): correct user catalog loading
```

### `refactor`

Usar cuando se reorganiza o simplifica código sin agregar una funcionalidad principal nueva.

Ejemplos:

```text
refactor(claims): standardize API endpoints
refactor(monthly-closures): update worker integration
```

### `chore`

Usar para configuración, mantenimiento o tareas que no afectan directamente la funcionalidad.

Ejemplo:

```text
chore(common-api): add gitkeep files for shared form directories
```

### `ci`

Usar para cambios en GitHub Actions o pipelines.

Ejemplo:

```text
ci(actions): execute validations on all pull requests
```

---

## 10. Reglas adicionales para agentes

- No inventar pruebas.
- No inventar endpoints.
- No asumir funcionalidades que no estén en los cambios.
- No usar títulos genéricos.
- No abrir un PR con descripción vacía.
- No omitir las secciones obligatorias.
- Mantener los nombres técnicos (`DTO`, endpoints, clases, formularios legacy, hooks, services) tal como existen en el código.
- Si el PR contiene varios commits, resumirlos por funcionalidad y no copiar los mensajes de commit literalmente.
- Si existe un cambio transversal, mencionarlo en el resumen, pero mantener el título centrado en el cambio principal.
- Si el PR mezcla frontend y backend en repositorios distintos, crear un PR independiente para cada repositorio.
