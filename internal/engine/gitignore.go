package engine

import (
	"bytes"
	"strings"
)

// gitignoreEntries are the local-only .tija/ paths that must not be committed.
var gitignoreEntries = []string{".tija/cache/", ".tija/index/", ".tija/session/", ".tija/tmp/"}

const gitignoreHeader = "# tija (local)"

// mergeGitignore appends the entries missing from existing under a
// "# tija (local)" header, keeping the file's line ending (CRLF or LF).
// An entry counts as present with or without a leading "/".
func mergeGitignore(existing []byte, entries []string) (out []byte, added []string) {
	nl := "\n"
	if bytes.Contains(existing, []byte("\r\n")) {
		nl = "\r\n"
	}
	present := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		present[strings.TrimPrefix(strings.TrimSpace(line), "/")] = true
	}
	for _, e := range entries {
		if !present[e] {
			added = append(added, e)
		}
	}
	if len(added) == 0 {
		return existing, nil
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		b.WriteString(nl)
	}
	if !present[gitignoreHeader] {
		b.WriteString(gitignoreHeader + nl)
	}
	for _, e := range added {
		b.WriteString(e + nl)
	}
	return []byte(b.String()), added
}
