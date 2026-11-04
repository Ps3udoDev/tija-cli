# Task 8 — Review engine orchestration

Implemented `RunReview` with target/merge-base resolution, the committed and working-tree diff plus non-ignored untracked files, default exclusions, quality reuse/execution, architecture checks, the deterministic 1000-line ceiling, agent chunk/pass orchestration, evidence verification, deep synthesis, PR comment attachment, sorting, thresholds and persisted review sessions.

Agent calls run in the repository root with ReadOnly enabled. Each call owns a system temporary directory, so concurrent Codex last-message output cannot collide and review does not write repository temporary files. Calls use the configured review tier, deep mode forces deep, timeouts follow tier defaults, and errgroup limits total concurrency to three. Invalid output or call failures retry once; failed chunks produce exit 2 while preserving findings already obtained. Missing/disabled agents retain deterministic behavior.

Quality runs use ephemeral mode: quality sessions are not rewritten, temporary diagnostics are outside the repository, and failed-step logs remain addressable from quality findings. This is required by the review-only-session write constraint. A fresh existing quality session is reused only for a matching commit with a clean working tree; NoCheck skips quality entirely.

PR comments are read only after findings are independently verified and merged. API pagination supports page arrays with `--slurp`, line falls back to original_line, and absence/auth failures become warnings without changing the finding threshold. NoPRComments avoids the API.

Independent audit examined `review.go`, `review_context.go`, `review_agent.go`, CLI and renderer against the spec. Added `review_integration_test.go` to establish real engine deep synthesis (a shared architecture rule from both passes merges to one confirmed disputed High with both severities) and partial agent failure (two invalid responses produce exit 2 while a deterministic High remains in output and saved session). Both tests passed with temporary writable GOCACHE.

Other engine tests cover no changes, working tree/untracked/exclusions, valid and discarded evidence, retry, max-three parallel calls and deep tier, no agent, deterministic checks and ceiling, invalid rules, quality reuse/run/skip, PR comments, session-only persistence and absent merge-base. Final all-package validation and live-agent limitations are recorded in the phase handoff; simulated tests do not establish live provider enforcement.
