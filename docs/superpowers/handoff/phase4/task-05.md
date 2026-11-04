# Task 5 — synthesis, results and persistence

Implemented deterministic deep synthesis, severity ordering and F1…Fn ids, the 1000-line crossing check, informational PR-comment attachment, the stable JSON result model, exit-code finalization and timestamped session persistence.

Decisions: synthesis sorts the agent results before merging, merges transitive overlaps of the same file/rule, unions ranges and pass ids, and retains the highest-severity finding's content. Confirmation requires distinct passes. Disputed severities are unique and sorted by severity. Checks and quality results never merge with agent results. PR comments without a positive current/original line stay uncovered. Finalization initializes all specified empty arrays and all four severity summary keys.

Session writes use a temporary sibling file and the existing quality-session Windows replacement strategy; only `.tija/session/review.json` is written. A missing session is distinguished from malformed or unreadable JSON. `SaveSession` stamps the saved copy with an RFC3339 UTC date and leaves the caller unchanged.

Validation: focused tests failed for absent interfaces before implementation; `go test ./internal/review/ -run 'Merge|Sort|Ceiling|Attach|Finalize|Session'` passes. Includes input-order independence, transitive overlap, different rules, checks exclusion, disputed severities, threshold/error precedence, empty JSON arrays, missing session, replacement roundtrip and corrupt JSON.

Reviewer: the Thermos attribution is present in `synth.go`; the parent integration must add its notice entry. The ceiling rule's generated prose is English at this pure layer; an engine can translate it to `content.language` when constructing the report.
