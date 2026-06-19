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
	"slices"
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
	// kindRoot is the synthetic top node. It consumes no token and matches
	// each child against the same segment, so its children are alternative
	// roots OR-ed together. It is the one place an alternation can sit with no
	// consuming parent above it, so it is valid only as the matcher argument of
	// path.match and never nested.
	kindRoot
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
	// globExclude holds segment values a kindGlob node must not match. It is set
	// only by GlobWithout. A token equal to any entry fails the node, so the
	// glob matches one segment that is none of the excluded values.
	globExclude []string
	// greedyExcept holds matcher subtrees a kindGreedy node must not match. It
	// is set by GreedyWithout and GreedyExcept. The greedy node matches the
	// trailing segments only when none of these subtrees match the suffix, so
	// it carves a deny out of an otherwise greedy match. The exclusion is a
	// pure negative test and binds no captures.
	greedyExcept []*Node
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

// Root builds the synthetic top node that matches each child against the same
// segment, so the children are alternative roots OR-ed together. It is the one
// way to give a tree several first segments, such as Root(Literal("api"),
// Literal("admin")), which the bare tree cannot express because a tree has one
// root node. It consumes no token, so it is valid only as the matcher argument
// of path.match and never nested; the load-time check and the parent
// constructors enforce that. An empty Root matches nothing and is a load error.
func Root(children ...*Node) (*Node, error) {
	if len(children) == 0 {
		return nil, trace.BadParameter("root() requires at least one alternative")
	}
	return &Node{kind: kindRoot, children: children}, nil
}

// GlobWithout builds a node that matches exactly one non-empty segment whose
// value is none of the excluded strings, then continues to children. It is the
// negative-set form of Glob: GlobWithout([]string{"private", "secret"}) matches
// any single segment except "private" or "secret". An empty excluded value is a
// load error, since a request path segment can never be empty under the strict
// URL rules and the entry would be dead.
func GlobWithout(excludes []string, children ...*Node) (*Node, error) {
	if err := validateExcludeValues(excludes); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Node{kind: kindGlob, globExclude: excludes, children: children}, nil
}

// GreedyWithout builds a terminal greedy node that matches the trailing
// segments unless the first of them equals an excluded string, so it excludes
// the `<excluded>/**` subtrees. GreedyWithout("private", "secret") matches any
// tail except one rooted at "private" or "secret". It is the string sugar for
// GreedyExcept(Literal(s, Greedy())) over each value, so the two collapse to
// one internal form. An empty excluded value is a load error.
func GreedyWithout(excludes ...string) (*Node, error) {
	if err := validateExcludeValues(excludes); err != nil {
		return nil, trace.Wrap(err)
	}
	except := make([]*Node, 0, len(excludes))
	for _, s := range excludes {
		except = append(except, Literal(s, Greedy()))
	}
	return &Node{kind: kindGreedy, greedyExcept: except}, nil
}

// GreedyExcept builds a terminal greedy node that matches the trailing segments
// unless they match one of the excluded matcher subtrees. The excluded
// matcher's own terminal-ness controls the scope: GreedyExcept(Literal("x"))
// excludes only the exact segment "x", while GreedyExcept(Literal("x",
// Greedy())) excludes the whole "x/**" subtree. An exclusion is a pure deny
// test that binds nothing, so a capture inside an excluded matcher is a load
// error rather than a silent no-op.
func GreedyExcept(excludes ...*Node) (*Node, error) {
	for _, e := range excludes {
		if containsCapture(e) {
			return nil, trace.BadParameter(
				"a greedy_except matcher cannot bind a capture: an exclusion is a deny test and binds nothing")
		}
	}
	return &Node{kind: kindGreedy, greedyExcept: excludes}, nil
}

// validateExcludeValues rejects an empty excluded segment value. A request path
// segment can never be empty under the strict URL rules, so an empty exclusion
// would never match anything and is a likely author mistake.
func validateExcludeValues(excludes []string) error {
	for _, s := range excludes {
		if s == "" {
			return trace.BadParameter("an excluded segment value cannot be empty")
		}
	}
	return nil
}

// containsCapture reports whether the node tree binds any capture, walking both
// the children and the greedy exclusion subtrees.
func containsCapture(n *Node) bool {
	if n == nil {
		return false
	}
	if n.kind == kindCapture {
		return true
	}
	for _, c := range n.children {
		if containsCapture(c) {
			return true
		}
	}
	for _, e := range n.greedyExcept {
		if containsCapture(e) {
			return true
		}
	}
	return false
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
		// including zero tokens. When the node carries exclusions, the suffix
		// must match none of them: each exclusion is a negative test, walked
		// against a throwaway capture map so a binding inside an exclusion never
		// leaks into the real match.
		for _, excl := range node.greedyExcept {
			if matchNode(excl, tokens, i, map[string]string{}) {
				return false
			}
		}
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
		if slices.Contains(node.globExclude, tokens[i]) {
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
// segment (from "//") is a compile error, since the strict URL rules reject
// the request bytes that would produce one. A trailing "/" is significant:
// it compiles to a terminal literal matching the trailing empty segment a
// request path produces, so "/foo/" matches the request "/foo/" but not
// "/foo", and the bare root "/" matches only the request "/". A `**` in any
// non-final position is a compile error.
func Compile(pattern string) (*Node, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, trace.BadParameter("path pattern %q must start with /", pattern)
	}
	segments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	for i, seg := range segments {
		if seg == "" {
			// Permit an empty segment only as the final one. That is the
			// trailing slash, "/foo/", or the bare root "/", both of which
			// match the trailing empty segment a request path produces.
			// Reject a leading empty segment and an interior empty segment
			// (both from "//"); the strict URL rules reject the request bytes
			// that would produce one anywhere but the trailing spot.
			if i != len(segments)-1 {
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
