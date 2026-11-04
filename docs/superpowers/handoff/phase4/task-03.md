# Task 3 — Architecture rules and optional presets

Implemented `internal/rules` with CRLF-tolerant frontmatter, scalar/list globs, normalized paths, global rule ID and file-name validation, reserved ID rejection, case-insensitive severities, RE2 compilation, agent rule sections, and deterministic checks restricted to added lines. Rule output ordering is stable.

Embedded `dotnet-clean` and `react-features` separately in `internal/assets/presets.go`; default installation never includes presets. This separate Go file avoids concurrent edits to defaults version and has the same package API required by the plan.

Validation: `go test ./internal/rules/ ./internal/assets/ -run 'Load|RunChecks|AgentRules|Presets|PresetSeeded'` passed. Tests cover malformed YAML, missing names, invalid regex/severity, duplicate/reserved IDs, cross-file duplicates, CRLF, Windows paths, appliesTo, presets and all four seeded checks. Used writable temporary GOCACHE because the system cache returned access denied.

Presets are adjustable pattern-based starting points. Cross-feature imports are conservatively recognized syntactically; they cannot resolve aliases, dependency graphs, or determine semantic ownership. Tests do not establish live-agent behavior.
