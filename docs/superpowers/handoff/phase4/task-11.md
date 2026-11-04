# Task 11 — Cite verified reviews in pull requests

Added `PRContext.Review`, loaded only when the session belongs to HEAD and was recorded on a clean tree. The agent prompt and deterministic draft cite the review command/mode and severity counts in `## Pruebas`; finding details are not passed as review context. Missing, corrupt, dirty, or stale review sessions provide no citation.

Decisions:
- The citation contains only the mode and severity summary, matching the documented example; no review finding is copied into the PR.
- `enforceVerifiedTests` also receives the review, so its deterministic replacement of the Spanish tests section preserves the verified citation after an agent response.
- Existing callers remain source compatible through an optional review argument.
- Completed the Task 6 integration: `engine.diffFence` delegates to `review.Fence`, preserving existing PR diff fencing behavior.

Validation: `go test ./internal/engine ./internal/cli -run 'ArchitectureRules|Preset|OptIn|InitProposesReview|FreshReview|CitesReview'` passes. Tests cover current/stale/dirty sessions; no-session prompt; review/quality/deep draft citations; anti-invention enforcement; and full simulated-agent PreparePR citation survival. This run also completes the previously pending Task 1 init proposal/idempotence test.

Reviewer focus: session freshness depends on commit identity and the session's dirty flag; citation never implies that automated tests ran. Agent Pruebas content still passes through existing anti-invention enforcement.
