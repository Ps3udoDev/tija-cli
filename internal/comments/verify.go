package comments

import (
	"bytes"
	"fmt"
	"strings"
)

type tokenInfo struct {
	text string
	line int
}

func CodeTokens(src []byte, lang Lang) ([]string, error) {
	tokens, err := codeTokensWithLines(src, lang)
	if err != nil {
		return nil, err
	}
	res := make([]string, len(tokens))
	for i, t := range tokens {
		res[i] = t.text
	}
	return res, nil
}

type span struct {
	start, end int
}

func codeTokensWithLines(src []byte, lang Lang) ([]tokenInfo, error) {
	lexTokens, err := Lex(src, lang)
	if err != nil {
		return nil, err
	}

	// Identify outer spans of JSX child comments so that enclosing braces are also discarded
	var jsxCommentSpans []span
	for _, t := range lexTokens {
		if t.InJSXChild {
			jsxCommentSpans = append(jsxCommentSpans, span{start: t.OuterStart, end: t.OuterEnd})
		}
	}

	inJSXComment := func(t Token) bool {
		for _, sp := range jsxCommentSpans {
			if t.Start >= sp.start && t.End <= sp.end {
				return true
			}
		}
		return false
	}

	var res []tokenInfo
	for _, t := range lexTokens {
		if t.Kind == TokenLineComment || t.Kind == TokenBlockComment || inJSXComment(t) {
			continue
		}

		line := 1 + bytes.Count(src[:t.Start], []byte("\n"))

		if t.Kind == TokenJSXText {
			norm := normalizeJSXText(t.Text)
			if norm != "" {
				res = append(res, tokenInfo{text: norm, line: line})
			}
			continue
		}

		res = append(res, tokenInfo{text: t.Text, line: line})
	}

	return res, nil
}

func normalizeJSXText(text string) string {
	lines := strings.Split(text, "\n")
	var kept []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, " ")
}

func VerifyTokens(before, after []byte, lang Lang) error {
	bTokens, err := CodeTokens(before, lang)
	if err != nil {
		return fmt.Errorf("tokenizing before: %w", err)
	}

	aTokens, err := codeTokensWithLines(after, lang)
	if err != nil {
		return fmt.Errorf("tokenizing after: %w", err)
	}

	minLen := len(bTokens)
	if len(aTokens) < minLen {
		minLen = len(aTokens)
	}

	for i := 0; i < minLen; i++ {
		if bTokens[i] != aTokens[i].text {
			return fmt.Errorf("token mismatch at index %d (line %d): before %q, after %q",
				i, aTokens[i].line, bTokens[i], aTokens[i].text)
		}
	}

	if len(bTokens) > len(aTokens) {
		line := 1
		if len(aTokens) > 0 {
			line = aTokens[len(aTokens)-1].line
		}
		return fmt.Errorf("token count mismatch: missing token at index %d (line %d): expected %q",
			len(aTokens), line, bTokens[len(aTokens)])
	}

	if len(aTokens) > len(bTokens) {
		idx := len(bTokens)
		return fmt.Errorf("token count mismatch: unexpected extra token at index %d (line %d): got %q",
			idx, aTokens[idx].line, aTokens[idx].text)
	}

	return nil
}
