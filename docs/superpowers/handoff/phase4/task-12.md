# Task 12 — Interactive Review screen

Added Review immediately after Pull Request, including repositories without a manifest. The screen offers Review / Quality / Deep, invokes the same RunReview engine, receives progress through a channel, and shows pass/chunk and quality progress. Thinking mascot remains active during work; result exit 0/1/2 selects success/warning/error. A runner option permits meaningful model tests without a terminal or external agent.

Results are sorted by severity. Arrow keys choose findings and keep the selected row visible; Enter opens explanation, suggestion and evidence; arrows/page/home/end scroll detail. Esc closes detail, and Esc/q from results returns to the menu. The standard renderer supplies summary, warnings and uncovered PR comments.

Updated existing menu expectations. Integration with Task10 required skipping optional preset offers in the existing init consent test and adding explicit default-no behavior: Enter declines an OptIn preset; its prompt explains that default, and the test verifies no preset was installed.

Validation: `go test ./internal/tui/` passed; after the final assertion, `go test ./internal/tui/ -run 'Review|InitFlowConsent'` passed. Tests exercise deep request forwarding through the actual command batch, event updates, all result mascot cases, sorted findings, details, scrolling, return-to-menu and optional preset refusal. Used writable temporary GOCACHE.

No real-terminal visual session or live external agent was run as part of this task; the final phase verification report owns those checks and any environment limitations.
