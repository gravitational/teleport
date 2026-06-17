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

// Package resourcematcher is a design sketch for the RFD 0303 path matcher. It
// is not wired into the app agent. It exists to validate that the segment-wise
// matcher tree, its declarative string sugar, and the capture-binding
// predicate integration all collapse to one internal representation and one
// evaluator.
//
// The matcher is a tree. Each node matches exactly one path segment and
// carries its continuation as zero or more child nodes. Nesting a single child
// descends to the next segment, so a chain of single children is a sequence.
// Giving one node several children branches into alternatives, so there is no
// separate sequence or alternation node. There is deliberately no
// regular-expression matching.
package resourcematcher

import (
	"strings"

	"github.com/gravitational/trace"
)

// kind enumerates the four matcher node kinds.
type kind int

const (
	// kindLiteral matches a token equal to its text.
	kindLiteral kind = iota
	// kindGlob matches exactly one non-empty token, the `*` metacharacter.
	kindGlob
	// kindCapture matches one token and binds it under capture.<name>, the
	// `{name}` metacharacter.
	kindCapture
	// kindGreedy matches zero or more trailing tokens, the `**`
	// metacharacter. It is terminal and carries no children.
	kindGreedy
)

// Node is one node in a matcher tree. It is an ordinary Go value, so a matcher
// can be constructed and asserted on in tests with no cluster.
type Node struct {
	kind kind
	// text holds the literal text for kindLiteral and the capture name for
	// kindCapture. It is empty for kindGlob and kindGreedy.
	text string
	// children are the matchers for the next segment. Several children are
	// alternatives, OR-ed together. A node with no children is terminal: the
	// subject must end at this segment for the match to succeed.
	children []*Node
}

// Literal builds a node that matches one or more fixed segments. The string is
// split on "/", so Literal("a/b/c", child) is exactly equal to
// Literal("a", Literal("b", Literal("c", child))). A path segment can never
// contain a "/" (it is the separator), so splitting here loses nothing and
// keeps a single canonical internal form. Metacharacters inside the string
// (`{}`, `*`, `**`) are treated as plain literal text, not as captures or
// globs; use Capture, Glob, and Greedy for those.
func Literal(s string, children ...*Node) *Node {
	segments := strings.Split(s, "/")
	// Build from the innermost segment outward so the supplied children hang
	// off the last segment.
	node := &Node{kind: kindLiteral, text: segments[len(segments)-1], children: children}
	for i := len(segments) - 2; i >= 0; i-- {
		node = &Node{kind: kindLiteral, text: segments[i], children: []*Node{node}}
	}
	return node
}

// Glob builds a node that matches exactly one non-empty segment, the `*`
// metacharacter.
func Glob(children ...*Node) *Node {
	return &Node{kind: kindGlob, children: children}
}

// Capture builds a node that matches one segment and binds it under
// capture.<name>, the `{name}` metacharacter.
func Capture(name string, children ...*Node) *Node {
	return &Node{kind: kindCapture, text: name, children: children}
}

// Greedy builds a terminal node that matches zero or more trailing segments,
// the `**` metacharacter. It takes no children.
func Greedy() *Node {
	return &Node{kind: kindGreedy}
}

// Eval walks tokens against the matcher root. On a match it returns true and
// the segments bound by capture nodes. On no match it returns false and a nil
// map. Evaluation never mutates shared state and never panics.
func Eval(tokens []string, root *Node) (bool, map[string]string) {
	caps := map[string]string{}
	if matchNode(root, tokens, 0, caps) {
		return true, caps
	}
	return false, nil
}

// matchNode reports whether node matches tokens starting at index i, recursing
// into children for the following segments. Captures are recorded into caps as
// the match descends. Because a non-matching branch can still have written a
// capture before failing, callers must treat caps as meaningful only when the
// top-level match returns true.
func matchNode(node *Node, tokens []string, i int, caps map[string]string) bool {
	switch node.kind {
	case kindGreedy:
		// Greedy is terminal and matches the entire remaining suffix,
		// including zero tokens. The match always succeeds from here.
		return true
	case kindLiteral:
		if i >= len(tokens) || tokens[i] != node.text {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindGlob:
		if i >= len(tokens) || tokens[i] == "" {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindCapture:
		if i >= len(tokens) || tokens[i] == "" {
			return false
		}
		// Bind before descending so a child predicate can read the capture.
		caps[node.text] = tokens[i]
		return matchChildren(node, tokens, i, caps)
	default:
		return false
	}
}

// matchChildren handles the continuation after node has consumed tokens[i]. A
// node with no children is terminal and requires the subject to end here.
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

// Compile parses a declarative path pattern such as
// "/api/v4/projects/{project}/**" into the same Node tree the constructors
// build. This is the string sugar: it desugars to the canonical internal form,
// so the declarative and predicate surfaces cannot diverge. The mapping is
// `{name}` to Capture, `*` to Glob, `**` to Greedy, and any other segment to
// Literal.
//
// A leading "/" is required and stripped. An interior or leading empty
// segment (from "//" or a bare "/") is a compile error, since the strict
// URL rules reject the request bytes that would produce one. A single
// trailing "/" is significant: it compiles to a terminal literal matching
// the trailing empty segment a request path produces, so "/foo/" matches
// the request "/foo/" but not "/foo". A `**` in any non-final position is a
// compile error.
func Compile(pattern string) (*Node, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, trace.BadParameter("path pattern %q must start with /", pattern)
	}
	segments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	for i, seg := range segments {
		if seg == "" {
			// Permit an empty segment only as the final one of a
			// multi-segment pattern, the trailing slash. Reject a leading
			// empty segment (from "//" or a bare "/") and an interior empty
			// segment (from "//"); the strict URL rules reject the request
			// bytes that would produce one anywhere but the trailing spot.
			if i == 0 || i != len(segments)-1 {
				return nil, trace.BadParameter("path pattern %q has an empty segment", pattern)
			}
			continue
		}
		if seg == "**" && i != len(segments)-1 {
			return nil, trace.BadParameter("** is only valid as the final segment in %q", pattern)
		}
	}
	return compileSegments(pattern, segments)
}

// compileSegments builds the node chain from the last segment back to the
// first, so each node holds the next as its single child.
func compileSegments(pattern string, segments []string) (*Node, error) {
	var child *Node
	for i := len(segments) - 1; i >= 0; i-- {
		node, err := compileSegment(pattern, segments[i])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if child != nil {
			if node.kind == kindGreedy {
				return nil, trace.BadParameter("** cannot have a following segment in %q", pattern)
			}
			node.children = []*Node{child}
		}
		child = node
	}
	return child, nil
}

// compileSegment maps a single pattern segment to its node kind.
func compileSegment(pattern, seg string) (*Node, error) {
	switch {
	case seg == "**":
		return Greedy(), nil
	case seg == "*":
		return Glob(), nil
	case strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}"):
		name := seg[1 : len(seg)-1]
		if !isIdentifier(name) {
			return nil, trace.BadParameter("capture name %q in %q must be a valid identifier so it reads as vars.<name>", name, pattern)
		}
		return Capture(name), nil
	default:
		// A bare segment is a single literal. A metacharacter is only valid as
		// a whole segment: "*", "**", or "{name}". Any other appearance, such
		// as "***", "a*", or a stray brace, is a malformed pattern rather than
		// a literal, so reject it instead of treating it as literal text. The
		// segment cannot contain "/", which the split already guaranteed.
		if strings.ContainsAny(seg, "*{}") {
			return nil, trace.BadParameter("segment %q in %q is not a valid literal, glob (*), greedy (**), or capture ({name})", seg, pattern)
		}
		// A literal can only match a request path, so it must itself be a legal
		// URL path segment. A byte that cannot appear in a request path, such
		// as "<" or a space, would make the pattern match nothing, so reject it
		// at load rather than compile a dead rule.
		for i := range len(seg) {
			if !isLegalPathByte(seg[i]) {
				return nil, trace.BadParameter("literal segment %q in %q contains an illegal URL byte %q", seg, pattern, string(seg[i]))
			}
		}
		return &Node{kind: kindLiteral, text: seg}, nil
	}
}

// isIdentifier reports whether s is a non-empty Go-style identifier, the form a
// capture name must take so it reads as vars.<name> in a predicate.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		b := s[i]
		isLetter := b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_'
		isDigit := b >= '0' && b <= '9'
		if !isLetter && !(i > 0 && isDigit) {
			return false
		}
	}
	return true
}
