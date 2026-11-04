# Task 10 — Optional architecture presets and doctor

Implemented stack-specific preset offers, explicit repeatable `init --rules-preset`, `Action.OptIn`, and doctor validation of architecture files including collisions with the actual pack rule catalogs.

Decisions:
- `--yes` skips opt-in presets; declining an offer does not leave a pending action.
- Explicit preset selection authorizes its installation and the existing manifest update needed to record the installed hash.
- Presets stay outside the unconditional defaults catalog. Previously installed, unedited presets use normal hash/version upgrades; edited preset files are never offered for replacement.
- Interactive confirmation uses the existing default-no prompt. Explicit names are validated before application, with valid names included in the error.

Validation: `go test ./internal/engine ./internal/cli -run 'ArchitectureRules|Preset|OptIn|InitProposesReview|FreshReview|CitesReview'` passes. `go test ./internal/manifest ./internal/skills ./internal/assets` passes. Tests cover stack filtering, existing rules suppressing offers, --yes behavior, repeatable explicit mechanism, invalid name, hash registration in new/existing repositories, preservation of edited presets, empty interactive response, valid/invalid/missing architecture rules. GOCACHE was redirected to a writable temporary directory because the system cache is sandbox-restricted.

Reviewer focus: init metadata is recomputed from applied actions; skipped opt-ins are neither written nor recorded. Doctor uses repository pack catalogs with embedded fallback.
