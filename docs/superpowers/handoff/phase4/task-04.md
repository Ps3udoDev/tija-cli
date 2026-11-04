# Task 4 — findings and verification

Implemented severity parsing and thresholds, stable finding/discard JSON types, prose/fence-tolerant response parsing, and verification in the specified order: file, hunk range, evidence, severity, rule.

Evidence matches a contiguous block of at most five lines within the bounded ±3-line window. Whitespace and CRLF normalize for comparison. Deleted/binary files cannot produce verified findings. Windows separators normalize before chunk membership and reads. `1000-lines-ceiling` is always rejected as an agent rule because it belongs to deterministic checks.

Decisions: a missing/null/non-array `findings` field and null/non-object entries are format errors eligible for retry. Empty arrays remain valid. Added `PRComment` here because `Finding.PRComments` requires its type; attachment logic remains Task 5. Huge end-line values cannot overflow the evidence-window upper bound.

Validation: focused tests failed for missing interfaces before implementation, then `go test ./internal/review/ -run 'AtLeast|ParseResponse|Verify'` passed. Covers all five discard reasons and their precedence, case normalization, Windows separators, empty and overlong evidence, ±3 boundary, CRLF, deleted files, prose/fences, malformed findings and threshold behavior.

Reviewer: agent output is untrusted; confirm reads are only made for paths already present in the chunk's map and rule ids come from the current pass.
