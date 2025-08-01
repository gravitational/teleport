// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package repl

import (
	"cmp"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
)

// lexer scans REPL input lines distinguishing between comments, strings,
// whitespace, delimiter, and normal query text.
type lexer struct {
	delim         string
	inComment     bool
	inStringToken token
	queryBuf      strings.Builder

	lineReader strings.Reader
	line       string
}

// clientDefaultStatementDelimiter is the default client-side delimiter between
// statements. This is the only delimiter that is recognized server-side, but it
// is sometimes necessary for the client to switch delimiters while typing at a
// REPL.
const clientDefaultStatementDelimiter = ";"

// delimiter returns the current client-side delimiter.
func (l *lexer) delimiter() string {
	return cmp.Or(l.delim, clientDefaultStatementDelimiter)
}

// validDelimiterRegex only allows somewhat sane choices for delimiters.
var validDelimiterRegex = regexp.MustCompile(`^(;|[/]{2}|[$]{2})$`)

// setDelimiter sets the client-side statement delimiter.
func (l *lexer) setDelimiter(delimiter string) error {
	delimiter = strings.TrimSpace(delimiter)
	if delimiter == "" {
		l.delim = ""
		return nil
	}
	if validDelimiterRegex.MatchString(delimiter) {
		l.delim = delimiter
		return nil
	}
	return trace.BadParameter("DELIMITER %q does not match regex used for validation %q",
		delimiter, validDelimiterRegex.String(),
	)
}

// writeString writes a string to the query buffer.
func (l *lexer) writeString(s string) {
	l.queryBuf.WriteString(s)
}

// writeRune writes a single rune to the query buffer.
func (l *lexer) writeRune(r rune) {
	l.queryBuf.WriteRune(r)
}

// getQuery builds the buffered query string and empties the query buffer.
func (l *lexer) getQuery() string {
	out := l.queryBuf.String()
	l.queryBuf.Reset()
	return out
}

// isInQuery returns true if the lexer is reading inside of a MySQL query.
func (l *lexer) isInQuery() bool {
	return l.queryBuf.Len() > 0
}

// isInComment returns true if the lexer is reading inside of a MySQL comment.
func (l *lexer) isInComment() bool {
	return l.inComment
}

// isInString returns true if the lexer is reading inside of a MySQL string.
func (l *lexer) isInString() bool {
	return len(l.inStringToken.text) > 0
}

// inStringKind returns the kind of MySQL string the lexer is currently reading
// inside of, i.e., the opening/closing rune: "'`.
func (l *lexer) inStringKind() string {
	return l.inStringToken.text
}

// isMultilineQuery returns true if the lexer is reading a multline query.
func (l *lexer) isMultilineQuery() bool {
	return l.isInQuery() && strings.Contains(l.queryBuf.String(), lineBreak)
}

// setCloseComment marks the start of a comment.
func (l *lexer) setOpenComment() {
	l.inComment = true
}

// setCloseComment marks the end of a comment.
func (l *lexer) setCloseComment() {
	l.inComment = false
}

// setCloseString marks the start of a MySQL string.
func (l *lexer) setOpenString(tok token) {
	l.inStringToken = tok
}

// setCloseString closes the currently open string state.
func (l *lexer) setCloseString() {
	l.inStringToken = token{}
}

// setLine sets the input line to read from.
func (l *lexer) setLine(line string) {
	l.lineReader.Reset(line)
	l.line = line
}

// isEmpty returns whether the line has been completely read.
func (l *lexer) isEmpty() bool {
	return l.lineReader.Len() == 0
}

// readRune reads a single rune and returns true if there was no error.
func (l *lexer) readRune() (rune, bool) {
	r, _, err := l.lineReader.ReadRune()
	ok := err == nil
	return r, ok
}

// unreadRune unreads the last rune that was read.
func (l *lexer) unreadRune() {
	l.lineReader.UnreadRune()
}

// peekString returns the remainder of the line without advancing the reader.
func (l *lexer) peekString() string {
	offset := len(l.line) - l.lineReader.Len()
	return l.line[offset:]
}

// advanceByRune advances the line reader by the given rune if and only if it
// matches.
func (l *lexer) advanceByRune(want rune) bool {
	if r, ok := l.readRune(); ok {
		if r == want {
			return true
		}
		l.unreadRune()
	}
	return false
}

// advanceByString advances the line reader by the given string if and only if
// it is a prefix of the remaining string.
func (l *lexer) advanceByDelimiter() bool {
	return l.advanceByString(l.delimiter())
}

// advanceByString advances the line reader by the given string if and only if
// it is a prefix of the remaining string.
func (l *lexer) advanceByString(s string) bool {
	if strings.HasPrefix(l.peekString(), s) {
		l.lineReader.Seek(int64(len(s)), io.SeekCurrent)
		return true
	}
	return false
}

// scan scans a token from the line reader.
func (l *lexer) scan() token {
	r, ok := l.readRune()
	if !ok {
		return token{kind: tokenEOF}
	}

	if unicode.IsSpace(r) {
		l.unreadRune()
		return token{kind: tokenSpace, text: l.readMatching(unicode.IsSpace)}
	}

	switch r {
	case '\\':
		return token{kind: tokenBackslash, text: `\`}
	case '\'':
		return token{kind: tokenSingleQuote, text: "'"}
	case '"':
		return token{kind: tokenDoubleQuote, text: `"`}
	case '`':
		return token{kind: tokenBacktick, text: "`"}
	case '-':
		if l.advanceByRune('-') {
			return token{kind: tokenSingleComment, text: "--"}
		}
	case '#':
		return token{kind: tokenSingleComment, text: "#"}
	case '/':
		if l.advanceByRune('*') {
			return token{kind: tokenOpenComment, text: "/*"}
		}
	case '*':
		if l.advanceByRune('/') {
			return token{kind: tokenCloseComment, text: "*/"}
		}
	}
	return token{kind: tokenText, text: string(r)}
}

// readMatching reads runes matching matchFn into the query buffer.
func (l *lexer) readMatching(matchFn func(rune) bool) string {
	var runes []rune
	for {
		r, ok := l.readRune()
		if !ok {
			break
		}
		if !matchFn(r) {
			l.unreadRune()
			break
		}
		runes = append(runes, r)
	}
	return string(runes)
}

// acceptEscapedRune reads the next character following an escape backslash and
// saves it to the query buffer.
func (l *lexer) acceptEscapedRune() {
	if r, ok := l.readRune(); ok {
		l.writeRune(r)
	}
}

// acceptString reads a MySQL string bounded by single-quotes, double-quotes, or
// backticks and saves it to the query buffer.
func (l *lexer) acceptString() {
	for !l.isEmpty() {
		tok := l.scan()
		l.writeString(tok.text)
		switch tok.kind {
		case tokenBackslash:
			l.acceptEscapedRune()
		case l.inStringToken.kind:
			// this must be a closing quote
			l.setCloseString()
			return
		}
	}
}

// discardRemaining discards the remainder of the line.
func (l *lexer) discardRemaining() {
	l.lineReader.Reset("")
}

// discardMatching discards runes matching matchFn.
func (l *lexer) discardMatching(matchFn func(rune) bool) {
	for {
		r, ok := l.readRune()
		if !ok {
			break
		}
		if !matchFn(r) {
			l.unreadRune()
			break
		}
	}
}

// discardWhitespace discards leading whitespace.
func (l *lexer) discardWhitespace() {
	l.discardMatching(unicode.IsSpace)
}

// discardSingleLineComment discards the rest of the line following a comment.
func (l *lexer) discardSingleLineComment() {
	l.discardRemaining()
}

// discardMultiLineComment discards everything inside of a multiline comment.
func (l *lexer) discardMultiLineComment() {
	for !l.isEmpty() {
		l.discardMatching(func(r rune) bool {
			// shortcut: scan until we get a potential end of comment token
			// note that escapes don't work in comments, so we don't check for them
			return r != '*'
		})
		if tok := l.scan(); tok.kind == tokenCloseComment {
			l.setCloseComment()
			return
		}
	}
}
