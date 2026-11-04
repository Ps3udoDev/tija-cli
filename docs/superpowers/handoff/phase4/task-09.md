# Task 9 — Review command, output and workflow

Implemented `tija review` with mutually exclusive quality/deep modes, threshold/agent validation, positive timeout checks, no-agent/no-check/no-PR-comments options and engine exit propagation. JSON emits only the result on stdout; progress goes to stderr for text output. Render includes severities, file/line (never :0), confirmations, disputed severities, suggestions, PR comment URLs, discarded count, partial failures and warnings. Plain mode has no ANSI.

Registered review in Cobra and the workflow allowlist. Embedded pre-pr runs clean comments --write → check → review → pr prepare. README documents modes, flags, defaults, thresholds, evidence verification/reasons, JSON fields, optional Markdown rules, RE2 limitations and preset installation. THIRD_PARTY_NOTICES credits the Thermos parallel orchestration and synthesis.

Validation: focused engine/CLI/render/assets review and pre-pr tests passed. CLI tests establish invalid flags exit 2, exact JSON object output, fail-on override and deterministic threshold exits 0/1. Text renderer assertions establish the required report format including confirmed/disputed output. Assets test checks exact workflow order and validation. A workflow integration test runs the actual embedded workflow with simulated successful quality tools, then checks that a deterministic High stops at review with exit 1 before pr prepare. Final command results are in task-13 and HANDOFF.

No new dependency was introduced. Provider network/auth availability and architecture-preset semantic limits remain documented in the final handoff.
