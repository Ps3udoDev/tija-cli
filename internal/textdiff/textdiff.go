// Package textdiff computes minimal line diffs (longest common subsequence).
package textdiff

import "strings"

// Op is one line of an edit script: Kind is ' ' (kept), '-' (removed) or
// '+' (added).
type Op struct {
	Kind byte
	Text string
}

// Split returns the lines of b with CRLF normalized to LF and without the
// final empty line.
func Split(b []byte) []string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Ops returns a minimal edit script turning a into b.
func Ops(a, b []string) []Op {
	n, m := len(a), len(b)
	// lcs[i][j] is the LCS length of a[i:] and b[j:].
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []Op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, Op{' ', a[i]})
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, Op{'-', a[i]})
			i++
		default:
			ops = append(ops, Op{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, Op{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, Op{'+', b[j]})
	}
	return ops
}

// Lines renders only the changed lines of old→new as "-line" / "+line",
// joined with "\n". It is "" when the contents are equal (ignoring CRLF).
func Lines(old, new []byte) string {
	var out []string
	for _, op := range Ops(Split(old), Split(new)) {
		if op.Kind != ' ' {
			out = append(out, string(op.Kind)+op.Text)
		}
	}
	return strings.Join(out, "\n")
}
