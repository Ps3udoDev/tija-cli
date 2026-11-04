# Task 2 — diff and chunks

Implemented `internal/review` unified diff parsing, new-side hunk ranges, added and removed counts, synthetic untracked files, and stable directory-aware chunks. Deleted and binary files stay available for verification but are omitted from agent chunks.

Decisions: normalize CRLF to LF in diff text as well as parsing; decode Git quoted paths with Go's C-compatible unquoting (including octal UTF-8 bytes); preserve file boundaries for oversized files; never mutate the caller's file slice. Empty synthetic files contain zero added lines.

Validation: the requested focused command first failed because the package interfaces did not exist. `go test ./internal/review/ -run 'ParseDiff|Touches|Synthetic|Chunks'` passes after implementation. Tests cover quoted paths, renames, deleted and binary files, CRLF, range boundaries, synthetic additions, stable ordering and oversized files.

Reviewer: verify merge-base git output against `ParseDiff`, especially paths with spaces and quotes. No git or subprocess dependencies were introduced into this package.
