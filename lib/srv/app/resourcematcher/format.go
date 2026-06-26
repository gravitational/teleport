/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resourcematcher

import "strings"

// DesugarResources lowers a role's sugared app_resources rules to their bare
// predicate strings, each pretty-printed with FormatPredicate. It is the
// lowering the engine's desugar() performs, surfaced so the playground and the
// golden tests can show what a declarative rule becomes as a predicate, and so
// they can compile and evaluate the lowered form alongside the sugared one to
// prove the two cannot diverge. The expression rules are already bare
// predicates, so they are not lowered here. The allow code rides in the
// predicate as an allow_code wrapper and the deny code as a deny_hint wrapper,
// so both audit fields round-trip through the desugared form.
func DesugarResources(role Role) ([]string, error) {
	out := make([]string, len(role.Resources))
	for j, rule := range role.Resources {
		lowered, err := rule.desugar()
		if err != nil {
			return nil, err
		}
		out[j] = FormatPredicate(lowered)
	}
	return out, nil
}

// FormatPredicate reformats a predicate so the matcher tree reads as a path. A
// constructor keeps its scalar arguments, a literal's text or a capture's name,
// on its own line, and breaks onto a new indented line only before an argument
// that is itself a call, the node's child. Sibling arguments that are calls
// share one level, and a child is indented one level further. So
// path.match(literal("files", capture("x", glob()))) renders as a single
// descending path.
//
// The result stays parseable: a line only ever ends in "(", ",", or an
// operator, never a ")" mid-expression, since the engine parses Go expression
// syntax where a line ending in ")" inside an argument list would take an
// inserted semicolon and fail. An empty "()" is kept on one line.
func FormatPredicate(s string) string {
	s = compactWhitespace(s)
	s = contractLiterals(s)
	s = stripRedundantParens(s)
	var b strings.Builder
	// base is the indent level at which the current call's child-call arguments
	// break. The stack restores it as each call closes.
	base := 0
	var stack []int
	inString := false
	newline := func(d int) {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat("  ", d))
	}
	// isCallStart reports whether the argument beginning at j, after any spaces
	// and a leading "!", is a call: an identifier, possibly dotted, immediately
	// followed by "(". Only a call argument breaks onto its own line; a scalar
	// stays inline on the constructor's line.
	isCallStart := func(j int) bool {
		for j < len(s) && s[j] == ' ' {
			j++
		}
		if j < len(s) && s[j] == '!' {
			j++
		}
		k := j
		for k < len(s) && (isIdentByte(s[k]) || s[k] == '.') {
			k++
		}
		return k > j && k < len(s) && s[k] == '('
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			b.WriteByte(c)
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			b.WriteByte(c)
		case '(':
			if i+1 < len(s) && s[i+1] == ')' {
				b.WriteString("()")
				i++
				continue
			}
			b.WriteByte('(')
			stack = append(stack, base)
			// A call paren follows an identifier and opens an argument list, so
			// it indents its children. A grouping paren follows an operator or
			// nothing and only wraps an expression for precedence, so it keeps
			// its content at the same level rather than adding a layer.
			p := i - 1
			for p >= 0 && s[p] == ' ' {
				p--
			}
			if p >= 0 && isIdentByte(s[p]) {
				base++
				// Break before the first argument only when it is a nested
				// call; a scalar first argument stays on the opening line.
				if isCallStart(i + 1) {
					newline(base)
				}
			}
		case ')':
			b.WriteByte(')')
			if len(stack) > 0 {
				base = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case ',':
			b.WriteByte(',')
			j := i + 1
			if j < len(s) && s[j] == ' ' {
				j++
			}
			if isCallStart(j) {
				newline(base)
			} else {
				b.WriteByte(' ')
			}
			i = j - 1
		case '&':
			if i+1 < len(s) && s[i+1] == '&' {
				b.WriteString("&&")
				i++
				newline(base)
				if i+1 < len(s) && s[i+1] == ' ' {
					i++
				}
			} else {
				b.WriteByte(c)
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// compactWhitespace collapses runs of whitespace outside string literals into a
// single space and drops spaces around "(", ")", and ",", so the formatter
// starts from a canonical single-line form regardless of how the input was
// spaced.
func compactWhitespace(s string) string {
	var b strings.Builder
	inString := false
	pendingSpace := false
	var last byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			b.WriteByte(c)
			last = c
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			pendingSpace = true
			continue
		}
		if pendingSpace && b.Len() > 0 && last != '(' && c != ')' && c != ',' {
			b.WriteByte(' ')
		}
		pendingSpace = false
		b.WriteByte(c)
		last = c
		if c == '"' {
			inString = true
		}
	}
	return b.String()
}

// isIdentByte reports whether b can appear in a predicate identifier.
func isIdentByte(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' ||
		b >= '0' && b <= '9' || b == '_'
}

// matchParen returns the index of the ")" that closes the "(" at open. A path
// segment string never contains a quote or backslash, so a plain quote toggle
// is enough to skip string contents. It returns -1 if the parenthesis is
// unbalanced.
func matchParen(s string, open int) int {
	depth, inString := 0, false
	for i := open; i < len(s); i++ {
		switch c := s[i]; {
		case inString:
			if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// readQuoted reads the "..."-quoted text starting at the opening quote i and
// returns the inner text and the index just past the closing quote.
func readQuoted(s string, i int) (string, int) {
	j := i + 1
	for j < len(s) && s[j] != '"' {
		j++
	}
	return s[i+1 : j], j + 1
}

// hasTopLevelBinaryOp reports whether s contains a "&&" or "||" outside any
// nested parenthesis and outside string literals. Parentheses around an
// expression with a top-level binary operator carry precedence and must not be
// stripped: an "&&" or "||" group inside an allow_code or deny_hint argument
// loses its meaning if the surrounding parentheses are removed.
func hasTopLevelBinaryOp(s string) bool {
	depth, inString := 0, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inString:
			if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && i+1 < len(s) && (c == '&' && s[i+1] == '&' || c == '|' && s[i+1] == '|'):
			return true
		}
	}
	return false
}

// contractLiterals rewrites literal("a", literal("b", rest)) to
// literal("a/b", rest) wherever a literal's only argument after its text is
// another literal, collapsing a hand-written segment chain into the single
// slash-joined form the path surface already uses. A literal with more than one
// child is an alternation, not a chain, so it is left alone. It is a
// display-only rewrite: literal("a/b") and literal("a", literal("b")) compile
// to the same node.
func contractLiterals(s string) string {
	const lit = "literal("
	for i := 0; i+len(lit) <= len(s); i++ {
		if s[i:i+len(lit)] != lit || (i > 0 && isIdentByte(s[i-1])) {
			continue
		}
		open := i + len(lit) - 1
		if open+1 >= len(s) || s[open+1] != '"' {
			continue
		}
		text, afterText := readQuoted(s, open+1)
		m := afterText
		if m >= len(s) || s[m] != ',' {
			continue
		}
		m++
		if m < len(s) && s[m] == ' ' {
			m++
		}
		if m+len(lit) > len(s) || s[m:m+len(lit)] != lit {
			continue
		}
		innerOpen := m + len(lit) - 1
		innerClose := matchParen(s, innerOpen)
		outerClose := matchParen(s, open)
		// The inner literal must be the sole child: the outer literal closes
		// immediately after it. Otherwise the children are alternation siblings,
		// which a slash join would silently merge.
		if innerClose < 0 || innerClose+1 != outerClose {
			continue
		}
		if innerOpen+1 >= len(s) || s[innerOpen+1] != '"' {
			continue
		}
		innerText, afterInnerText := readQuoted(s, innerOpen+1)
		rest := s[afterInnerText:innerClose]
		merged := lit + `"` + text + "/" + innerText + `"` + rest + ")"
		// Re-run from the start so a longer chain collapses fully.
		return contractLiterals(s[:i] + merged + s[outerClose+1:])
	}
	return s
}

// stripRedundantParens removes a grouping parenthesis whose content carries no
// top-level "&&" or "||", since such a wrapper only adds noise. A group that
// carries a top-level operator is kept so its precedence holds. A parenthesis
// that follows an identifier opens a call argument list and is never touched.
func stripRedundantParens(s string) string {
	inString := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inString:
			if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
		case c == '(':
			p := i - 1
			for p >= 0 && s[p] == ' ' {
				p--
			}
			if p >= 0 && isIdentByte(s[p]) {
				continue // call paren, not a grouping paren
			}
			closeIdx := matchParen(s, i)
			if closeIdx < 0 {
				continue
			}
			inner := s[i+1 : closeIdx]
			if strings.TrimSpace(inner) == "" || hasTopLevelBinaryOp(inner) {
				continue
			}
			return stripRedundantParens(s[:i] + inner + s[closeIdx+1:])
		}
	}
	return s
}
