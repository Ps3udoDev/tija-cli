# Task 1 — Manifest and pack rule catalogs

Implemented `manifest.Review`, defaults, threshold/maxLines validation, detection defaults, rule IDs parsed from skill frontmatter, and defaults version 5. Existing init missing-field reconciliation automatically proposes the review section without replacing configured values.

Decision: nil rule/exclusion lists inherit defaults; explicit empty YAML lists remain empty, allowing a repository to disable those lists.

Validation: targeted manifest, skills, and assets tests pass using a writable temporary GOCACHE. Engine validation is pending completion of concurrent task 8 files (currently unresolved review symbols); the added `TestInitProposesReviewSection` verifies missing-section proposal and idempotence. Full-suite verification will be recorded in final handoff.

Reviewer focus: defaults remain fresh slices, user values survive WithDefaults, pack rule catalogs include the exact documented IDs and adaptation entry.
