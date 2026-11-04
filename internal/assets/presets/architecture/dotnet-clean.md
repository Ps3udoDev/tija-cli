---
name: dotnet-clean
description: Optional starting point for Domain, Application, Infrastructure and Api layers.
appliesTo: ["src/**/*.cs"]
checks:
  - id: be-domain-no-infra
    severity: High
    files: "src/*.Domain/**/*.cs"
    forbid: '^\s*using\s+[\w.]*\.(Infrastructure|Api)\b'
    message: El dominio no puede depender de Infrastructure ni de la API.
  - id: be-controller-no-dbcontext
    severity: High
    files: "src/**/Controllers/**/*.cs"
    forbid: '\b\w*DbContext\b'
    message: Los controladores no deben acceder directamente al DbContext.
---
## be-thin-controllers: Controladores delgados
Los controladores traducen HTTP a casos de uso. Reporta lógica de negocio concreta que deba pertenecer a Application o Domain.

## be-dtos-at-boundary: DTOs en el borde
Usa DTOs para los contratos HTTP; evita exponer entidades de dominio o persistencia directamente.

## be-async-end-to-end: Async de punta a punta
Propaga async/await y CancellationToken al ejecutar I/O. Señala bloqueos síncronos demostrables.

## be-config-and-secrets: Configuración y secretos fuera del código
Carga configuración desde proveedores y secretos desde almacenamiento apropiado. No reportes valores de ejemplo como secretos reales.
