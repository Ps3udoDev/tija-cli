package sys

import (
	"bytes"
	"strings"
	"sync"
)

// TailWriter keeps the last Max lines written to it.
type TailWriter struct {
	Max int

	mu      sync.Mutex
	buf     []byte
	lines   []string
}

// Write appends p to the writer, splitting by newlines and keeping at most Max lines.
func (w *TailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			w.buf = append(w.buf, p...)
			break
		}

		lineBytes := append(w.buf, p[:idx]...)
		w.buf = nil
		p = p[idx+1:]

		line := strings.TrimRight(string(lineBytes), "\r")
		w.addLine(line)
	}

	return total, nil
}

func (w *TailWriter) addLine(line string) {
	if w.Max <= 0 {
		return
	}
	if len(w.lines) >= w.Max {
		copy(w.lines, w.lines[1:])
		w.lines[len(w.lines)-1] = line
	} else {
		w.lines = append(w.lines, line)
	}
}

// Lines returns the last Max lines without "\r" and without a trailing empty line.
func (w *TailWriter) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var result []string
	if len(w.lines) > 0 {
		result = make([]string, len(w.lines))
		copy(result, w.lines)
	}

	if len(w.buf) > 0 {
		pending := strings.TrimRight(string(w.buf), "\r")
		if pending != "" {
			if w.Max > 0 {
				if len(result) >= w.Max {
					result = append(result[1:], pending)
				} else {
					result = append(result, pending)
				}
			}
		}
	}

	if len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}

	return result
}
