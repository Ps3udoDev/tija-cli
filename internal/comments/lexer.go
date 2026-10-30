package comments

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Lang struct {
	TS  bool
	JSX bool
}

// LangFor returns Lang and true for supported extensions:
// .js, .jsx -> TS: false, JSX: true (author React repos have JSX in .js)
// .mjs, .cjs -> TS: false, JSX: false
// .ts, .mts, .cts -> TS: true, JSX: false (no JSX, <T> is generic)
// .tsx -> TS: true, JSX: true
// others -> false
func LangFor(path string) (Lang, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx":
		return Lang{TS: false, JSX: true}, true
	case ".mjs", ".cjs":
		return Lang{TS: false, JSX: false}, true
	case ".ts", ".mts", ".cts":
		return Lang{TS: true, JSX: false}, true
	case ".tsx":
		return Lang{TS: true, JSX: true}, true
	default:
		return Lang{}, false
	}
}

type TokenKind int

const (
	TokenCode TokenKind = iota
	TokenString
	TokenTemplate
	TokenRegex
	TokenJSXText
	TokenLineComment
	TokenBlockComment
)

type Token struct {
	Kind       TokenKind
	Start, End int    // byte offsets
	Text       string // src[Start:End]
	InJSXChild bool   // true if this block comment is the entire content of a JSX child { ... }
	OuterStart int    // offset of opening '{' when InJSXChild is true
	OuterEnd   int    // offset after closing '}' when InJSXChild is true
}

var ErrUncertain = errors.New("could not tokenize with certainty")

type modeKind int

const (
	modeCode modeKind = iota
	modeJSXTag
	modeJSXChildren
)

type modeFrame struct {
	kind       modeKind
	braceDepth int    // brace depth within interpolation or expression
	inTemplate bool   // true if code mode pushed for template interpolation ${}
	inJSXExpr  bool   // true if code mode pushed for JSX expression {}
	jsxTag     string // tag name for modeJSXTag
}

type lexer struct {
	src      []byte
	n        int
	pos      int
	lang     Lang
	tokens   []Token
	lastSig  *Token
	modes    []modeFrame
	jsxStack []string // stack of open JSX tag names
}

func Lex(src []byte, lang Lang) ([]Token, error) {
	l := &lexer{
		src:      src,
		n:        len(src),
		pos:      0,
		lang:     lang,
		tokens:   make([]Token, 0, len(src)/16),
		modes:    []modeFrame{{kind: modeCode}},
		jsxStack: make([]string, 0, 8),
	}

	return l.tokenize()
}

func (l *lexer) tokenize() ([]Token, error) {
	// Handle UTF-8 BOM
	if l.n >= 3 && l.src[0] == 0xef && l.src[1] == 0xbb && l.src[2] == 0xbf {
		l.emitToken(TokenCode, 0, 3)
		l.pos = 3
	}

	// Handle shebang on line 1
	if l.pos+1 < l.n && l.src[l.pos] == '#' && l.src[l.pos+1] == '!' {
		start := l.pos
		for l.pos < l.n && l.src[l.pos] != '\r' && l.src[l.pos] != '\n' {
			l.pos++
		}
		l.emitToken(TokenCode, start, l.pos)
	}

	for l.pos < l.n {
		curMode := l.currentMode()

		switch curMode.kind {
		case modeJSXChildren:
			if err := l.lexJSXChildren(); err != nil {
				return nil, err
			}

		case modeJSXTag:
			if err := l.lexJSXTag(); err != nil {
				return nil, err
			}

		case modeCode:
			if err := l.lexCode(); err != nil {
				return nil, err
			}
		}
	}

	// Verify all stacks are empty
	if len(l.jsxStack) > 0 {
		return nil, ErrUncertain
	}
	if len(l.modes) > 1 {
		return nil, ErrUncertain
	}

	return l.tokens, nil
}

func (l *lexer) currentMode() *modeFrame {
	return &l.modes[len(l.modes)-1]
}

func (l *lexer) pushMode(m modeFrame) {
	l.modes = append(l.modes, m)
}

func (l *lexer) popMode() modeFrame {
	top := l.modes[len(l.modes)-1]
	l.modes = l.modes[:len(l.modes)-1]
	return top
}

func (l *lexer) emitToken(kind TokenKind, start, end int) *Token {
	tok := Token{
		Kind:  kind,
		Start: start,
		End:   end,
		Text:  string(l.src[start:end]),
	}
	l.tokens = append(l.tokens, tok)
	if kind != TokenLineComment && kind != TokenBlockComment {
		l.lastSig = &l.tokens[len(l.tokens)-1]
	}
	return &l.tokens[len(l.tokens)-1]
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\v' || b == '\f'
}

func (l *lexer) skipWhitespace() {
	for l.pos < l.n && isWhitespace(l.src[l.pos]) {
		l.pos++
	}
}

func (l *lexer) lexCode() error {
	l.skipWhitespace()
	if l.pos >= l.n {
		return nil
	}

	b := l.src[l.pos]

	// Line or block comment or regex or division
	if b == '/' {
		if l.pos+1 < l.n && l.src[l.pos+1] == '/' {
			l.scanLineComment()
			return nil
		}
		if l.pos+1 < l.n && l.src[l.pos+1] == '*' {
			return l.scanBlockComment(false, 0, 0)
		}
		// Regex vs division
		if l.canStartExpression() {
			return l.scanRegex()
		}
		l.scanDivision()
		return nil
	}

	// Strings
	if b == '"' || b == '\'' {
		return l.scanString(b)
	}

	// Template literals
	if b == '`' {
		return l.scanTemplate()
	}

	// JSX opening
	if b == '<' && l.lang.JSX && l.canStartExpression() && l.isJSXTagStart() {
		return l.startJSXElement()
	}

	curMode := l.currentMode()

	// Closing brace in code mode
	if b == '}' {
		if curMode.inTemplate || curMode.inJSXExpr {
			if curMode.braceDepth == 0 {
				if curMode.inTemplate {
					// Exits interpolation back into template scanning
					l.popMode()
					return l.resumeTemplate()
				}
				// Exits JSX expression
				l.emitToken(TokenCode, l.pos, l.pos+1)
				l.pos++
				l.popMode()
				return nil
			}
			curMode.braceDepth--
		}
		l.emitToken(TokenCode, l.pos, l.pos+1)
		l.pos++
		return nil
	}

	// Opening brace in code mode
	if b == '{' {
		if curMode.inTemplate || curMode.inJSXExpr {
			curMode.braceDepth++
		}
		l.emitToken(TokenCode, l.pos, l.pos+1)
		l.pos++
		return nil
	}

	// Identifiers and keywords
	if isIdentStart(b) || (b >= 0x80 && isUnicodeIdentStart(l.src[l.pos:])) {
		l.scanIdentifier()
		return nil
	}

	// Numbers
	if b >= '0' && b <= '9' {
		l.scanNumber()
		return nil
	}
	if b == '.' && l.pos+1 < l.n && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9' {
		l.scanNumber()
		return nil
	}

	// Punctuators and operators
	l.scanOperator()
	return nil
}

func (l *lexer) scanLineComment() {
	start := l.pos
	l.pos += 2
	for l.pos < l.n && l.src[l.pos] != '\r' && l.src[l.pos] != '\n' {
		l.pos++
	}
	l.emitToken(TokenLineComment, start, l.pos)
}

func (l *lexer) scanBlockComment(inJSXChild bool, outerStart, outerEnd int) error {
	start := l.pos
	l.pos += 2
	for l.pos+1 < l.n {
		if l.src[l.pos] == '*' && l.src[l.pos+1] == '/' {
			l.pos += 2
			tok := Token{
				Kind:       TokenBlockComment,
				Start:      start,
				End:        l.pos,
				Text:       string(l.src[start:l.pos]),
				InJSXChild: inJSXChild,
				OuterStart: outerStart,
				OuterEnd:   outerEnd,
			}
			l.tokens = append(l.tokens, tok)
			return nil
		}
		l.pos++
	}
	return ErrUncertain
}

func (l *lexer) scanRegex() error {
	start := l.pos
	l.pos++ // skip leading '/'

	for l.pos < l.n {
		c := l.src[l.pos]
		if c == '\r' || c == '\n' {
			return ErrUncertain
		}
		if c == '\\' {
			l.pos++
			if l.pos >= l.n || l.src[l.pos] == '\r' || l.src[l.pos] == '\n' {
				return ErrUncertain
			}
			l.pos++
			continue
		}
		if c == '[' {
			l.pos++
			closed := false
			for l.pos < l.n {
				cc := l.src[l.pos]
				if cc == '\r' || cc == '\n' {
					return ErrUncertain
				}
				if cc == '\\' {
					l.pos++
					if l.pos >= l.n || l.src[l.pos] == '\r' || l.src[l.pos] == '\n' {
						return ErrUncertain
					}
					l.pos++
					continue
				}
				if cc == ']' {
					l.pos++
					closed = true
					break
				}
				l.pos++
			}
			if !closed {
				return ErrUncertain
			}
			continue
		}
		if c == '/' {
			l.pos++ // past closing '/'
			// scan flags
			for l.pos < l.n && isIdentPart(l.src[l.pos]) {
				l.pos++
			}
			l.emitToken(TokenRegex, start, l.pos)
			return nil
		}
		l.pos++
	}
	return ErrUncertain
}

func (l *lexer) scanDivision() {
	start := l.pos
	l.pos++
	if l.pos < l.n && l.src[l.pos] == '=' {
		l.pos++
	}
	l.emitToken(TokenCode, start, l.pos)
}

func (l *lexer) scanString(quote byte) error {
	start := l.pos
	l.pos++ // skip opening quote

	for l.pos < l.n {
		c := l.src[l.pos]
		if c == quote {
			l.pos++
			l.emitToken(TokenString, start, l.pos)
			return nil
		}
		if c == '\r' || c == '\n' {
			return ErrUncertain
		}
		if c == '\\' {
			l.pos++
			if l.pos >= l.n {
				return ErrUncertain
			}
			// line continuation \ \r\n or \ \n
			if l.src[l.pos] == '\r' && l.pos+1 < l.n && l.src[l.pos+1] == '\n' {
				l.pos += 2
				continue
			}
			l.pos++
			continue
		}
		l.pos++
	}
	return ErrUncertain
}

func (l *lexer) scanTemplate() error {
	start := l.pos
	l.pos++ // skip opening backtick
	return l.scanTemplateContent(start)
}

func (l *lexer) resumeTemplate() error {
	start := l.pos
	l.pos++ // skip opening '}' from interpolation
	return l.scanTemplateContent(start)
}

func (l *lexer) scanTemplateContent(start int) error {
	for l.pos < l.n {
		c := l.src[l.pos]
		if c == '`' {
			l.pos++
			l.emitToken(TokenTemplate, start, l.pos)
			return nil
		}
		if c == '\\' {
			l.pos++
			if l.pos >= l.n {
				return ErrUncertain
			}
			l.pos++
			continue
		}
		if c == '$' && l.pos+1 < l.n && l.src[l.pos+1] == '{' {
			l.pos += 2
			l.emitToken(TokenTemplate, start, l.pos)
			l.pushMode(modeFrame{kind: modeCode, inTemplate: true, braceDepth: 0})
			return nil
		}
		l.pos++
	}
	return ErrUncertain
}

func (l *lexer) canStartExpression() bool {
	if l.lastSig == nil {
		return true
	}
	if l.lastSig.Kind != TokenCode {
		return false
	}

	txt := l.lastSig.Text

	// Operators and punctuators after which expression cannot start: ), ], }, ++, --
	switch txt {
	case ")", "]", "}", "++", "--":
		return false
	}

	// Keywords after which expression can start
	switch txt {
	case "return", "typeof", "case", "do", "else", "in", "instanceof",
		"new", "void", "delete", "throw", "yield", "await", "of", "default":
		return true
	}

	// Punctuators after which expression can start
	switch txt {
	case "(", "[", "{", ";", ",", ":", "?", "...",
		"=", "+=", "-=", "*=", "/=", "%=", "**=", "<<=", ">>=", ">>>=",
		"&=", "|=", "^=", "&&=", "||=", "??=",
		"==", "!=", "===", "!==", "<", ">", "<=", ">=", "=>",
		"+", "-", "*", "/", "%", "**", "&", "|", "^", "~", "!", "&&", "||", "??":
		return true
	}

	return false
}

func (l *lexer) isJSXTagStart() bool {
	if l.pos+1 >= l.n {
		return false
	}
	next := l.src[l.pos+1]
	return isIdentStart(next) || next == '>' || next == '/'
}

func (l *lexer) startJSXElement() error {
	start := l.pos
	l.pos++ // past '<'

	// Closing tag `</`
	if l.pos < l.n && l.src[l.pos] == '/' {
		l.pos++
		l.emitToken(TokenCode, start, start+1)   // '<'
		l.emitToken(TokenCode, start+1, start+2) // '/'
		l.skipWhitespace()

		// Fragment closing `</>`
		if l.pos < l.n && l.src[l.pos] == '>' {
			l.emitToken(TokenCode, l.pos, l.pos+1)
			l.pos++
			if len(l.jsxStack) == 0 || l.jsxStack[len(l.jsxStack)-1] != "" {
				return ErrUncertain
			}
			l.jsxStack = l.jsxStack[:len(l.jsxStack)-1]
			if len(l.modes) > 0 && l.currentMode().kind == modeJSXChildren {
				l.popMode() // pop modeJSXChildren
			}
			return nil
		}

		// Named closing tag `</tag>`
		tagNameStart := l.pos
		for l.pos < l.n && (isIdentPart(l.src[l.pos]) || l.src[l.pos] == '.' || l.src[l.pos] == ':' || l.src[l.pos] == '-') {
			l.pos++
		}
		if l.pos == tagNameStart {
			return ErrUncertain
		}
		tagName := string(l.src[tagNameStart:l.pos])
		l.emitToken(TokenCode, tagNameStart, l.pos)

		l.skipWhitespace()
		if l.pos >= l.n || l.src[l.pos] != '>' {
			return ErrUncertain
		}
		l.emitToken(TokenCode, l.pos, l.pos+1)
		l.pos++

		if len(l.jsxStack) == 0 || l.jsxStack[len(l.jsxStack)-1] != tagName {
			return ErrUncertain
		}
		l.jsxStack = l.jsxStack[:len(l.jsxStack)-1]
		if len(l.modes) > 0 && l.currentMode().kind == modeJSXChildren {
			l.popMode() // pop modeJSXChildren
		}
		return nil
	}

	// Fragment opening `<>`
	if l.pos < l.n && l.src[l.pos] == '>' {
		l.emitToken(TokenCode, start, start+1)
		l.emitToken(TokenCode, l.pos, l.pos+1)
		l.pos++
		l.jsxStack = append(l.jsxStack, "")
		l.pushMode(modeFrame{kind: modeJSXChildren})
		return nil
	}

	// Opening tag `<tag`
	l.emitToken(TokenCode, start, start+1)
	l.skipWhitespace()

	tagNameStart := l.pos
	for l.pos < l.n && (isIdentPart(l.src[l.pos]) || l.src[l.pos] == '.' || l.src[l.pos] == ':' || l.src[l.pos] == '-') {
		l.pos++
	}
	if l.pos == tagNameStart {
		return ErrUncertain
	}
	tagName := string(l.src[tagNameStart:l.pos])
	l.emitToken(TokenCode, tagNameStart, l.pos)

	l.pushMode(modeFrame{kind: modeJSXTag, jsxTag: tagName})
	return nil
}

func (l *lexer) lexJSXTag() error {
	for {
		l.skipWhitespace()
		if l.pos >= l.n {
			return ErrUncertain
		}

		// Comments inside tag
		if l.src[l.pos] == '/' && l.pos+1 < l.n && l.src[l.pos+1] == '/' {
			l.scanLineComment()
			continue
		}
		if l.src[l.pos] == '/' && l.pos+1 < l.n && l.src[l.pos+1] == '*' {
			if err := l.scanBlockComment(false, 0, 0); err != nil {
				return err
			}
			continue
		}

		// Self-closing `/>`
		if l.src[l.pos] == '/' && l.pos+1 < l.n && l.src[l.pos+1] == '>' {
			l.emitToken(TokenCode, l.pos, l.pos+1)
			l.emitToken(TokenCode, l.pos+1, l.pos+2)
			l.pos += 2
			l.popMode() // pop modeJSXTag
			return nil
		}

		// End of opening tag `>`
		if l.src[l.pos] == '>' {
			l.emitToken(TokenCode, l.pos, l.pos+1)
			l.pos++
			tagName := l.currentMode().jsxTag
			l.popMode() // pop modeJSXTag
			l.jsxStack = append(l.jsxStack, tagName)
			l.pushMode(modeFrame{kind: modeJSXChildren})
			return nil
		}

		// Spread attribute `{...props}` or expression
		if l.src[l.pos] == '{' {
			l.emitToken(TokenCode, l.pos, l.pos+1)
			l.pos++
			l.pushMode(modeFrame{kind: modeCode, inJSXExpr: true, braceDepth: 0})
			return nil
		}

		// Attribute name
		attrStart := l.pos
		for l.pos < l.n && (isIdentPart(l.src[l.pos]) || l.src[l.pos] == ':' || l.src[l.pos] == '-') {
			l.pos++
		}
		if l.pos == attrStart {
			return ErrUncertain
		}
		l.emitToken(TokenCode, attrStart, l.pos)

		l.skipWhitespace()
		if l.pos < l.n && l.src[l.pos] == '=' {
			l.emitToken(TokenCode, l.pos, l.pos+1)
			l.pos++
			l.skipWhitespace()
			if l.pos >= l.n {
				return ErrUncertain
			}

			if l.src[l.pos] == '"' || l.src[l.pos] == '\'' {
				if err := l.scanString(l.src[l.pos]); err != nil {
					return err
				}
			} else if l.src[l.pos] == '{' {
				l.emitToken(TokenCode, l.pos, l.pos+1)
				l.pos++
				l.pushMode(modeFrame{kind: modeCode, inJSXExpr: true, braceDepth: 0})
				return nil
			} else {
				return ErrUncertain
			}
		}
	}
}

func (l *lexer) lexJSXChildren() error {
	// Text up to '<' or '{'
	textStart := l.pos
	for l.pos < l.n && l.src[l.pos] != '<' && l.src[l.pos] != '{' {
		l.pos++
	}

	if l.pos > textStart {
		textBytes := l.src[textStart:l.pos]
		if hasNonWhitespace(textBytes) {
			l.emitToken(TokenJSXText, textStart, l.pos)
		}
	}

	if l.pos >= l.n {
		return ErrUncertain // unclosed JSX element
	}

	// At '{': check if InJSXChild block comment
	if l.src[l.pos] == '{' {
		if l.tryJSXChildComment() {
			return nil
		}

		// Normal JSX child expression { ... }
		l.emitToken(TokenCode, l.pos, l.pos+1)
		l.pos++
		l.pushMode(modeFrame{kind: modeCode, inJSXExpr: true, braceDepth: 0})
		return nil
	}

	// At '<'
	if l.src[l.pos] == '<' {
		if l.isJSXTagStart() {
			return l.startJSXElement()
		}
		return ErrUncertain
	}

	return nil
}

func (l *lexer) tryJSXChildComment() bool {
	p := l.pos + 1
	for p < l.n && isWhitespace(l.src[p]) {
		p++
	}

	if p+1 >= l.n || l.src[p] != '/' || l.src[p+1] != '*' {
		return false
	}

	commentStart := p
	p += 2
	closed := false
	for p+1 < l.n {
		if l.src[p] == '*' && l.src[p+1] == '/' {
			p += 2
			closed = true
			break
		}
		p++
	}
	if !closed {
		return false
	}
	commentEnd := p

	for p < l.n && isWhitespace(l.src[p]) {
		p++
	}

	if p >= l.n || l.src[p] != '}' {
		return false
	}

	outerStart := l.pos
	outerEnd := p + 1

	l.emitToken(TokenCode, outerStart, outerStart+1)
	tok := Token{
		Kind:       TokenBlockComment,
		Start:      commentStart,
		End:        commentEnd,
		Text:       string(l.src[commentStart:commentEnd]),
		InJSXChild: true,
		OuterStart: outerStart,
		OuterEnd:   outerEnd,
	}
	l.tokens = append(l.tokens, tok)
	l.emitToken(TokenCode, p, outerEnd)
	l.pos = outerEnd
	return true
}

func hasNonWhitespace(b []byte) bool {
	for _, c := range b {
		if !isWhitespace(c) {
			return true
		}
	}
	return false
}

func isIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == '$'
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

func isUnicodeIdentStart(b []byte) bool {
	r, _ := utf8.DecodeRune(b)
	return unicode.IsLetter(r)
}

func isUnicodeIdentPart(b []byte) bool {
	r, _ := utf8.DecodeRune(b)
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (l *lexer) scanIdentifier() {
	start := l.pos
	for l.pos < l.n {
		b := l.src[l.pos]
		if isIdentPart(b) {
			l.pos++
			continue
		}
		if b >= 0x80 && isUnicodeIdentPart(l.src[l.pos:]) {
			_, size := utf8.DecodeRune(l.src[l.pos:])
			l.pos += size
			continue
		}
		break
	}
	l.emitToken(TokenCode, start, l.pos)
}

func (l *lexer) scanNumber() {
	start := l.pos

	// 0x, 0b, 0o
	if l.src[l.pos] == '0' && l.pos+1 < l.n {
		next := l.src[l.pos+1]
		if next == 'x' || next == 'X' {
			l.pos += 2
			for l.pos < l.n && (isHexDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
				l.pos++
			}
			if l.pos < l.n && l.src[l.pos] == 'n' {
				l.pos++
			}
			l.emitToken(TokenCode, start, l.pos)
			return
		}
		if next == 'b' || next == 'B' {
			l.pos += 2
			for l.pos < l.n && (l.src[l.pos] == '0' || l.src[l.pos] == '1' || l.src[l.pos] == '_') {
				l.pos++
			}
			if l.pos < l.n && l.src[l.pos] == 'n' {
				l.pos++
			}
			l.emitToken(TokenCode, start, l.pos)
			return
		}
		if next == 'o' || next == 'O' {
			l.pos += 2
			for l.pos < l.n && (l.src[l.pos] >= '0' && l.src[l.pos] <= '7' || l.src[l.pos] == '_') {
				l.pos++
			}
			if l.pos < l.n && l.src[l.pos] == 'n' {
				l.pos++
			}
			l.emitToken(TokenCode, start, l.pos)
			return
		}
	}

	// Decimal integer or float
	for l.pos < l.n && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9' || l.src[l.pos] == '_') {
		l.pos++
	}

	// Dot and fraction
	if l.pos < l.n && l.src[l.pos] == '.' {
		// Ensure it's not followed by another dot (e.g. 1..toString or 1...spread)
		if l.pos+1 < l.n && l.src[l.pos+1] == '.' {
			l.emitToken(TokenCode, start, l.pos)
			return
		}
		l.pos++
		for l.pos < l.n && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9' || l.src[l.pos] == '_') {
			l.pos++
		}
	}

	// Exponent
	if l.pos < l.n && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.pos++
		if l.pos < l.n && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
		}
		for l.pos < l.n && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9' || l.src[l.pos] == '_') {
			l.pos++
		}
	}

	// BigInt suffix
	if l.pos < l.n && l.src[l.pos] == 'n' {
		l.pos++
	}

	l.emitToken(TokenCode, start, l.pos)
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func (l *lexer) scanOperator() {
	start := l.pos
	rem := l.n - l.pos

	// 4-char
	if rem >= 4 {
		op4 := string(l.src[l.pos : l.pos+4])
		if op4 == ">>>=" {
			l.pos += 4
			l.emitToken(TokenCode, start, l.pos)
			return
		}
	}

	// 3-char
	if rem >= 3 {
		op3 := string(l.src[l.pos : l.pos+3])
		switch op3 {
		case "===", "!==", ">>>", "<<=", ">>=", "**=", "&&=", "||=", "??=", "...":
			l.pos += 3
			l.emitToken(TokenCode, start, l.pos)
			return
		}
	}

	// 2-char
	if rem >= 2 {
		op2 := string(l.src[l.pos : l.pos+2])
		switch op2 {
		case "==", "!=", "<=", ">=", "=>", "&&", "||", "??", "++", "--",
			"<<", ">>", "**", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "?.":
			l.pos += 2
			l.emitToken(TokenCode, start, l.pos)
			return
		}
	}

	// 1-char
	l.pos++
	l.emitToken(TokenCode, start, l.pos)
}
