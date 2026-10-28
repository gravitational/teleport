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
	"slices"

	"github.com/gravitational/trace"
)

// Pattern is a valid path pattern tree verified by [Compile]. It never contains
// error nodes. A Pattern is safe for concurrent use.
type Pattern struct {
	root Node
	// encoded is the set of capture names given AllowUnsafeEncoded.
	encoded map[string]struct{}
}

// CompileOption is an option of [Compile].
type CompileOption func(*compileConfig)

// compileConfig contains the options of one Compile call.
type compileConfig struct {
	encoded []encodedCapture
}

// encodedCapture is the arguments of one AllowUnsafeEncoded option.
type encodedCapture struct {
	name  string
	chars string
}

// Compile verifies every node of a tree. If all nodes are valid, it returns the
// tree-wrapping [Pattern] type accepted by the [Match] function. If not all
// nodes are valid, Compile returns the first error derived from an error node,
// found in a depth-first traversal. Error nodes result from invalid constructor
// use. For example:
//
//	Compile(Root(Literal("health"), Optional()))
//
// results in the error
//
//	root() > optional(): optional requires at least one child subtree
//
// A nil tree, a nil child, a node with children reused under several parents,
// a capture nested under a capture of the same name, an invalid capture name,
// and a Root below the top of the tree also result in errors.
// Invalid options, such as a duplicate capture name or invalid chars in the
// [AllowUnsafeEncoded] option, also result in errors.
func Compile(root Node, options ...CompileOption) (*Pattern, error) {
	if root == nil {
		return nil, trace.BadParameter("cannot compile a nil tree")
	}
	var cfg compileConfig
	for _, option := range options {
		option(&cfg)
	}
	seen := map[Node]struct{}{}
	bound := map[string]struct{}{}
	if err := compileNode(root, seen, bound); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(cfg.encoded) == 0 {
		return &Pattern{root: root}, nil
	}
	names := map[string]struct{}{}
	for _, name := range captureNames(root) {
		names[name] = struct{}{}
	}
	encoded := make(map[string]struct{}, len(cfg.encoded))
	for _, e := range cfg.encoded {
		if err := validateCaptureName(e.name); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := validateAllowedEncoded(e.chars); err != nil {
			return nil, trace.BadParameter("encoded capture %q: %v", e.name, err)
		}
		if _, ok := names[e.name]; !ok {
			return nil, trace.BadParameter("%q is not a capture in the tree", e.name)
		}
		if _, ok := encoded[e.name]; ok {
			return nil, trace.BadParameter("AllowUnsafeEncoded(%q, ...) is given twice", e.name)
		}
		encoded[e.name] = struct{}{}
	}
	return &Pattern{root: root, encoded: encoded}, nil
}

// AllowUnsafeEncoded lets Capture name match the percent-encoded form of chars.
// The only supported chars is "/", so the capture can match a segment with an
// encoded slash (%2F). Compile returns an error for any other chars.
func AllowUnsafeEncoded(name, chars string) CompileOption {
	return func(c *compileConfig) {
		c.encoded = append(c.encoded, encodedCapture{name: name, chars: chars})
	}
}

// Match walks tokens against the compiled pattern. On a match it returns true
// and the decoded content of the tokens bound by captures on the matching
// branch. If there is no match it returns false and a nil map. A nil pattern
// and an empty tokens slice never match.
//
// The tokens must be the output of Tokenize, the segments of a valid request
// path. Matching compares their decoded content.
func Match(pattern *Pattern, tokens []Token) (bool, map[string]string) {
	if pattern == nil || len(tokens) == 0 {
		return false, nil
	}
	m := newMatcher(tokens, pattern.encoded)
	if m.matchNode(pattern.root, tokens) {
		return true, m.captures
	}
	return false, nil
}

// compileNode checks node and its subtree. In case of an error each ancestor
// prefixes its own string representation on return.
func compileNode(node Node, seen map[Node]struct{}, bound map[string]struct{}) error {
	childNodes := node.children()
	switch n := node.(type) {
	case *errNode:
		return trace.BadParameter("%s: %v", n, n.err)
	case *captureNode:
		if _, ok := bound[n.name]; ok {
			return trace.BadParameter("%s: capture %q is already bound on this branch", n, n.name)
		}
		bound[n.name] = struct{}{}
		defer delete(bound, n.name) // Unbind on return, so a sibling branch may bind the name again.
	case *greedyExceptNode:
		childNodes = n.exclusions // Compile checks the exclusions like children.
	}
	if len(childNodes) > 0 {
		// Track only nodes with children; zero-size leaves, such as every
		// Greedy, may share one address in Go.
		if _, ok := seen[node]; ok {
			return trace.BadParameter("%s appears twice in the tree; a pattern must be a tree", node)
		}
		seen[node] = struct{}{}
	}
	for _, child := range childNodes {
		if child == nil {
			return trace.BadParameter("%s has a nil child", node)
		}
		if _, ok := child.(*rootNode); ok {
			return trace.BadParameter("%s > %s must be the top node", node, child)
		}
		if err := compileNode(child, seen, bound); err != nil {
			return trace.BadParameter("%s > %v", node, err)
		}
	}
	return nil
}

// matcher is the state of one Match call.
type matcher struct {
	captures map[string]string   // bound on descent, undone on a failed branch
	encoded  map[string]struct{} // Pattern.encoded
	tokens   []Token             // the whole path
	// plainSuffixLen is the number of trailing tokens without an encoded
	// slash, or -1 before suffixHasEncodedSlash first counts them.
	plainSuffixLen int
}

// newMatcher returns a matcher for tokens.
func newMatcher(tokens []Token, encoded map[string]struct{}) matcher {
	return matcher{
		captures:       map[string]string{},
		encoded:        encoded,
		tokens:         tokens,
		plainSuffixLen: -1,
	}
}

// suffixHasEncodedSlash returns true when a token in tokens has an encoded
// slash (%2F). tokens must be a suffix of the path.
func (m *matcher) suffixHasEncodedSlash(tokens []Token) bool {
	if m.plainSuffixLen < 0 {
		// Scan once per Match, and only when a greedy node is reached, so a
		// literal that fails early does not cost a scan of the whole path.
		n := 0
		for n < len(m.tokens) && !m.tokens[len(m.tokens)-1-n].hasEncodedSlash() {
			n++
		}
		m.plainSuffixLen = n
	}
	return len(tokens) > m.plainSuffixLen
}

// matchNode reports whether the node's tree matches tokens recursively,
// collecting captures on descent.
func (m *matcher) matchNode(node Node, tokens []Token) bool {
	switch n := node.(type) {
	case *rootNode:
		return m.matchNodes(n.childNodes, tokens)
	case *greedyNode:
		return !m.suffixHasEncodedSlash(tokens)
	case *greedyExceptNode:
		return !m.suffixHasEncodedSlash(tokens) && !m.matchNodes(n.exclusions, tokens)
	case *slashNode:
		return len(tokens) == 1 && tokens[0].Raw == ""
	case *optionalNode:
		return len(tokens) == 0 || m.matchNodes(n.childNodes, tokens)
	case *globNode:
		if len(tokens) == 0 || tokens[0].Raw == "" || tokens[0].hasEncodedSlash() {
			return false
		}
		return m.matchNodes(n.childNodes, tokens[1:])
	case *globWithoutNode:
		if len(tokens) == 0 || tokens[0].Raw == "" || tokens[0].hasEncodedSlash() || slices.Contains(n.excludes, tokens[0].Decoded) {
			return false
		}
		return m.matchNodes(n.childNodes, tokens[1:])
	case *literalNode:
		// n.text never contains "/", so the equality includes
		// !hasEncodedSlash().
		if len(tokens) == 0 || tokens[0].Decoded != n.text {
			return false
		}
		return m.matchNodes(n.childNodes, tokens[1:])
	case *captureNode:
		if len(tokens) == 0 || tokens[0].Raw == "" {
			return false
		}
		if _, ok := m.encoded[n.name]; !ok && tokens[0].hasEncodedSlash() {
			return false
		}
		m.captures[n.name] = tokens[0].Decoded
		if m.matchNodes(n.childNodes, tokens[1:]) {
			return true
		}
		delete(m.captures, n.name) // Undo the binding if the subtree does not match.
		return false
	default:
		return false // Unreachable for a Compile-verified pattern.
	}
}

// matchNodes reports whether any of the nodes matches tokens.
func (m *matcher) matchNodes(nodes []Node, tokens []Token) bool {
	if len(nodes) == 0 {
		return len(tokens) == 0
	}
	match := func(node Node) bool { return m.matchNode(node, tokens) }
	return slices.ContainsFunc(nodes, match)
}
