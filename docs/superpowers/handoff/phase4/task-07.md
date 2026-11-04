# Task 7 — Read-only agent requests and git review helpers

Added `Request.ReadOnly` and explicit `Argv(..., readOnly bool)`. Claude read-only requests allow Read/Grep/Glob and deny Bash/Edit/Write/NotebookEdit/WebFetch/WebSearch. Existing callers use false; Codex retains its existing read-only sandbox; Gemini arguments are unchanged as required by the plan. Gemini therefore relies on prompt instructions rather than an OS sandbox (live CLI validation remains pending).

Added MergeBase, WorkingDiff (working tree plus staged changes against a revision), NUL-delimited Untracked, and LineCountAt (including empty and unterminated files). Persisted paths and diff CRLF are normalized.

Validation: `go test ./internal/agents/ ./internal/gitx/` passed using temporary writable GOCACHE. Tests include real temporary repositories, committed and uncommitted changes together, ignored/untracked files with spaces, invalid refs, exact line counts, provider argument preservation, and Generate forwarding ReadOnly.
