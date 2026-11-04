# Task 6 — review prompt

Implemented `BuildPrompt`, `PromptInput`, `PromptRule` and the shared `Fence` helper. Prompt sections follow the plan order: pack body, applicable architecture rules, quality summary, chunk files and diffs, handled deterministic rule ids, language, JSON schema and read-only instructions. Invalid responses can be retried with the concrete format error.

Decisions: absent quality results are explicitly unknown; repository/diff content is described as data rather than instructions; deterministic rules are named under “do not report”; output language uses `manifest.LanguageName` (empty defaults to Spanish); diff fences exceed the longest embedded backtick run. No new dependencies.

Validation: prompt tests initially failed for missing interfaces. `go test ./internal/review/` now passes all Tasks 2, 4, 5 and 6 tests. Prompt tests cover section order, optional rule/retry sections, schema fields, language, read-only constraints and embedded triple/six-backtick runs.

Integration: the parent agent owns the existing `engine.diffFence` call-site migration to `review.Fence` in the PR prompt; this task intentionally did not edit shared engine files. Review the final engine diff to confirm this migration and the unchanged PR fence tests.
