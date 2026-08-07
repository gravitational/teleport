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

package appresource

import (
	"strings"
)

// Eval walks tokens against the matcher root. On a match it returns true
// and the segments bound by capture nodes. On no match it returns false and
// a nil map. The tokens are the raw output of Tokenize; matching compares
// their content views, so a token sent with an encoded space or encoded
// non-ASCII text matches the literal written plain. Evaluation never
// mutates shared state and never panics.
func Eval(tokens []string, root *Node) (bool, map[string]string) {
	caps := map[string]string{}
	if matchNode(root, tokens, 0, caps) {
		return true, caps
	}
	return false, nil
}

// matchNode reports whether node matches tokens starting at index i,
// recursing into children for the following segments. Captures are recorded
// into caps as the match descends. Because a non-matching branch can still
// have written a capture before failing, callers must treat caps as
// meaningful only when the top-level match returns true.
func matchNode(node *Node, tokens []string, i int, caps map[string]string) bool {
	switch node.kind {
	case kindRoot:
		// Root consumes no token: each child is matched against the same
		// segment, so the children are alternative roots. The first that
		// matches wins.
		for _, child := range node.children {
			if matchNode(child, tokens, i, caps) {
				return true
			}
		}
		return false
	case kindGreedy:
		// Greedy is terminal and matches the entire remaining suffix,
		// including zero tokens. It is repeated glob, so the suffix must
		// carry no encoded separator: a single such segment anywhere in
		// the tail stops the greedy, which is why a broad `**` never
		// silently absorbs an encoded slash.
		for _, tok := range tokens[i:] {
			if hasEncodedSeparator(tok) {
				return false
			}
		}
		return true
	case kindSlash:
		// A trailing slash is the empty segment that ends the token list.
		// It is terminal: the empty token must exist and be the last one.
		return i < len(tokens) && tokens[i] == "" && i+1 == len(tokens)
	case kindOptional:
		// The subtree is optional: match either the end of the path, where
		// it is absent, or one of the children against the remainder. The
		// end branch binds nothing, which is why a capture inside an
		// optional is never guaranteed.
		if i == len(tokens) {
			return true
		}
		for _, child := range node.children {
			if matchNode(child, tokens, i, caps) {
				return true
			}
		}
		return false
	case kindLiteral:
		// A literal holds decoded content and never contains "%", so a
		// token carrying the encoded separator keeps its raw "%2F" in the
		// content view and fails the comparison.
		if i >= len(tokens) || contentView(tokens[i]) != node.text {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindGlob:
		// A glob matches one segment that carries no encoded separator, so
		// it never spans an encoded slash.
		if i >= len(tokens) || tokens[i] == "" || hasEncodedSeparator(tokens[i]) {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindCapture:
		// Like glob, a capture binds one segment with no encoded
		// separator. The bound value is the content view, so a where
		// condition later compares decoded text and never sees an escape.
		if i >= len(tokens) || tokens[i] == "" || hasEncodedSeparator(tokens[i]) {
			return false
		}
		// Bind before descending so a child predicate can read the capture.
		caps[node.text] = contentView(tokens[i])
		return matchChildren(node, tokens, i, caps)
	default:
		return false
	}
}

// matchChildren handles the continuation after node has consumed tokens[i].
// A node with no children is terminal and requires the subject to end here.
// Several children are alternatives: the first that matches wins.
func matchChildren(node *Node, tokens []string, i int, caps map[string]string) bool {
	if len(node.children) == 0 {
		// Terminal: the subject must end exactly at this segment.
		return i+1 == len(tokens)
	}
	for _, child := range node.children {
		if matchNode(child, tokens, i+1, caps) {
			return true
		}
	}
	return false
}

// contentView returns the token with its content escapes decoded: the
// encoded space %20 and the escapes of non-ASCII bytes become their bytes,
// while the encoded separator %2F keeps its raw escape. Matching compares
// content by value, so "My%20Project" and the literal text "My Project" are
// one segment, while the separator stays a structural question a plain node
// never answers. The raw token is what the app agent forwards; this view
// exists only for the comparison.
func contentView(token string) string {
	if !strings.ContainsRune(token, '%') {
		return token
	}
	var b strings.Builder
	b.Grow(len(token))
	for i := 0; i < len(token); i++ {
		if token[i] == '%' && i+2 < len(token) && !isEncodedSeparatorAt(token, i) {
			if v, ok := parseEscape(token, i); ok {
				b.WriteByte(v)
				i += 2
				continue
			}
		}
		b.WriteByte(token[i])
	}
	return b.String()
}

// hasEncodedSeparator reports whether the raw token carries the encoded
// separator %2F or %2f. It is the one escape a plain glob, capture, or
// greedy node refuses, because forwarding it upstream can change how many
// segments the app sees.
func hasEncodedSeparator(token string) bool {
	for i := range len(token) {
		if isEncodedSeparatorAt(token, i) {
			return true
		}
	}
	return false
}

// isEncodedSeparatorAt reports whether the escape starting at token[i] is
// the encoded separator, in either hex case.
func isEncodedSeparatorAt(token string, i int) bool {
	return i+2 < len(token) && token[i] == '%' && token[i+1] == '2' &&
		(token[i+2] == 'F' || token[i+2] == 'f')
}

// parseEscape reads the two hex digits of the escape starting at token[i]
// and reports whether they form one of the escapes Tokenize admits.
func parseEscape(token string, i int) (byte, bool) {
	hi, ok1 := hexVal(token[i+1])
	lo, ok2 := hexVal(token[i+2])
	if !ok1 || !ok2 {
		return 0, false
	}
	b := hi<<4 | lo
	if !isAllowedEscape(b) {
		return 0, false
	}
	return b, true
}

// hexVal converts one hex digit, in either case, to its value.
func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
