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
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

func TestDelimiter(t *testing.T) {
	t.Parallel()
	var lex lexer
	require.Equal(t, clientDefaultStatementDelimiter, lex.delimiter())

	tests := []struct {
		desc            string
		args            string
		wantDelimiter   string
		wantErrContains string
	}{
		{
			desc:          "passing no arg is ok",
			wantDelimiter: ";",
		},
		{
			desc:          "passing the default delimiter is ok",
			args:          ";",
			wantDelimiter: ";",
		},
		{
			desc:          "passing two dollars is ok",
			args:          "$$",
			wantDelimiter: "$$",
		},
		{
			desc:          "passing two slashes is ok",
			args:          "//",
			wantDelimiter: "//",
		},
		{
			desc:            "passing too many delimiter runes is invalid",
			args:            ";;;;;;",
			wantErrContains: `DELIMITER ";;;;;;" does not match regex used for validation "^(;|[/]{2}|[$]{2})$"`,
		},
		{
			desc:            "passing rejected runes is invalid",
			args:            "x",
			wantErrContains: `DELIMITER "x" does not match regex used for validation "^(;|[/]{2}|[$]{2})$"`,
		},
		{
			desc:            "passing rejected runes is invalid",
			args:            "\\",
			wantErrContains: `DELIMITER "\\" does not match regex used for validation "^(;|[/]{2}|[$]{2})$"`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var lex lexer
			err := lex.setDelimiter(test.args)
			if test.wantErrContains != "" {
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantDelimiter, lex.delimiter())
		})
	}
}

func TestLexer_setLine(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("SELECT * FROM t;")
	require.Equal(t, "SELECT * FROM t;", l.peekString())
	require.False(t, l.isEmpty())
}

func TestLexer_advanceByRune(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("abc")
	require.True(t, l.advanceByRune('a'), "should advance on matching rune")
	require.Equal(t, "bc", l.peekString())
	require.False(t, l.advanceByRune('z'), "should not advance on non-matching rune")
	require.Equal(t, "bc", l.peekString())
}

func TestLexer_advanceByString(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("foobar")
	require.True(t, l.advanceByString("foo"), "should advance by matching prefix")
	require.Equal(t, "bar", l.peekString())
	require.False(t, l.advanceByString("baz"), "should not advance if prefix doesn't match")
	require.Equal(t, "bar", l.peekString())
}

func TestLexer_advanceByDelimiter(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setDelimiter(";")
	l.setLine("; SELECT")
	require.True(t, l.advanceByDelimiter())
	require.Equal(t, " SELECT", l.peekString())
	require.False(t, l.advanceByDelimiter())
	require.Equal(t, " SELECT", l.peekString())
	l.setLine("//")
	l.setDelimiter("//")
	require.True(t, l.advanceByDelimiter())
	require.Empty(t, l.peekString())
	require.False(t, l.advanceByDelimiter())
}

func TestLexer_getQuery(t *testing.T) {
	t.Parallel()
	var l lexer
	l.writeString("hello")
	l.writeRune(' ')
	l.writeString("world!")
	require.Equal(t, "hello world!", l.getQuery())
	require.Empty(t, l.getQuery(), "getQuery should reset buffer")
}

func TestLexer_unread(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("xy")
	r, ok := l.readRune()
	require.True(t, ok)
	require.Equal(t, 'x', r)
	l.unreadRune()

	r, ok = l.readRune()
	require.True(t, ok)
	require.Equal(t, 'x', r)

	r, ok = l.readRune()
	require.True(t, ok)
	require.Equal(t, 'y', r)

	_, ok = l.readRune()
	require.False(t, ok, "should be false at end of input")
	l.unreadRune()
	_, ok = l.readRune()
	require.False(t, ok, "cant unread eof")
}

func TestLexer_readMatching(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("123abcXYZ")
	require.Equal(t, "123", l.readMatching(unicode.IsDigit))
	require.Equal(t, "abc", l.readMatching(unicode.IsLower))
	require.Equal(t, "XYZ", l.readMatching(unicode.IsUpper))
	require.Empty(t, l.readMatching(unicode.IsDigit))
}

func TestLexer_acceptEscapedRune(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine(`\xyz`)
	tok := l.scan()
	require.Equal(t, tokenBackslash, tok.kind)
	l.writeString(tok.text)
	l.acceptEscapedRune()
	require.Equal(t, "yz", l.peekString())
	require.Equal(t, `\x`, l.getQuery())
}

func TestLexer_acceptString(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine(`"inside string "; SELECT`)
	tok := l.scan()
	require.Equal(t, tokenDoubleQuote, tok.kind)
	l.writeString(tok.text)
	l.setOpenString(tok)
	l.acceptString()
	require.Equal(t, "; SELECT", l.peekString())
	require.Equal(t, `"inside string "`, l.getQuery())
}

func TestLexer_discardRemaining(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("DROP TABLE test;")
	l.discardRemaining()
	require.True(t, l.isEmpty())
	require.Empty(t, l.peekString())
}

func TestLexer_discardMatching(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		matchFn  func(rune) bool
		expected string
	}{
		{
			name:     "discard digits",
			input:    "123abc",
			matchFn:  unicode.IsDigit,
			expected: "abc",
		},
		{
			name:     "discard lowercase",
			input:    "abcDEF",
			matchFn:  unicode.IsLower,
			expected: "DEF",
		},
		{
			name:     "nothing to discard",
			input:    "ABC123",
			matchFn:  unicode.IsLower,
			expected: "ABC123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var l lexer
			l.setLine(tc.input)
			l.discardMatching(tc.matchFn)
			require.Equal(t, tc.expected, l.peekString())
		})
	}
}

func TestLexer_discardWhitespace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "leading whitespace only",
			input:    "   SELECT",
			expected: "SELECT",
		},
		{
			name:     "no whitespace",
			input:    "SELECT",
			expected: "SELECT",
		},
		{
			name:     "whitespace and tabs",
			input:    "\t \n  abc",
			expected: "abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var l lexer
			l.setLine(tc.input)
			l.discardWhitespace()
			require.Equal(t, tc.expected, l.peekString())
		})
	}
}

func TestLexer_discardSingleLineComment(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("-- This is a comment")
	l.discardSingleLineComment()
	require.True(t, l.isEmpty())
	require.Empty(t, l.peekString())
}

func TestLexer_discardMultiLineComment(t *testing.T) {
	t.Parallel()
	var l lexer
	l.setLine("inside comment */ SELECT 1;")
	l.setOpenComment()
	l.discardMultiLineComment()
	require.False(t, l.isInComment(), "should have exited comment mode")
	remaining := l.peekString()
	require.Equal(t, " SELECT 1;", remaining)
}
