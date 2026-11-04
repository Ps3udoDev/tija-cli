---
name: react-features
description: Optional starting point for React features with hooks and services.
appliesTo: ["src/**/*.{ts,tsx,js,jsx}"]
checks:
  - id: fe-no-fetch-in-components
    severity: High
    files: ["src/**/components/**/*.{ts,tsx,js,jsx}", "src/**/*.tsx", "src/**/*.jsx"]
    forbid: '\b(fetch\s*\(|axios\s*[.(])'
    message: El acceso HTTP debe vivir en hooks o servicios, fuera del componente.
  - id: fe-no-cross-feature-imports
    severity: High
    files: "src/features/**/*.{ts,tsx,js,jsx}"
    forbid: '^\s*(import|export)\b.*(from\s*)?["\x27](@/features/|\.\./(\.\./)*[^./][^/]*["\x27/])'
    message: Evita dependencias directas entre features; mueve contratos compartidos a una capa común.
---
## fe-feature-structure: Estructura de una feature
Organiza cada feature con components, hooks, services y types según sus responsabilidades. Reporta acoplamientos concretos en el diff.

## fe-public-types-no-any: Tipos públicos sin any
Los contratos públicos deben expresar tipos útiles sin any que oculte errores comprobables.

## fe-server-state: Estado de servidor en hooks y services
Mantén llamadas y estado de servidor en hooks/services; maneja carga, errores y cancelación cuando corresponda.
