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
func Compile(root Node) (*Pattern, error) {
	if root == nil {
		return nil, trace.BadParameter("cannot compile a nil tree")
	}
	seen := map[Node]struct{}{}
	bound := map[string]struct{}{}
	if err := compileNode(root, seen, bound); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Pattern{root: root}, nil
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
	captures := map[string]string{}
	if matchNode(pattern.root, tokens, captures) {
		return true, captures
	}
	return false, nil
}

// compileNode checks node and its subtree. In case of an error each ancestor
// prefixes its own string representation on return.
func compileNode(node Node, seen map[Node]struct{}, bound map[string]struct{}) error {
	switch n := node.(type) {
	case *errNode:
		return trace.BadParameter("%s: %v", n, n.err)
	case *captureNode:
		if _, ok := bound[n.name]; ok {
			return trace.BadParameter("%s: capture %q is already bound on this branch", n, n.name)
		}
		bound[n.name] = struct{}{}
		defer delete(bound, n.name) // Unbind on return, so a sibling branch may bind the name again.
	}
	childNodes := node.children()
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

// matchNode reports whether the node's tree matches tokens recursively,
// collecting captures on descent.
func matchNode(node Node, tokens []Token, captures map[string]string) bool {
	switch n := node.(type) {
	case *rootNode:
		return matchNodes(n.childNodes, tokens, captures)
	case *greedyNode:
		return !slices.ContainsFunc(tokens, Token.hasEncodedSlash)
	case *slashNode:
		return len(tokens) == 1 && tokens[0].Raw == ""
	case *optionalNode:
		return len(tokens) == 0 || matchNodes(n.childNodes, tokens, captures)
	case *globNode:
		if len(tokens) == 0 || tokens[0].Raw == "" || tokens[0].hasEncodedSlash() {
			return false
		}
		return matchNodes(n.childNodes, tokens[1:], captures)
	case *literalNode:
		// n.text never contains "/", so the equality includes
		// !hasEncodedSlash().
		if len(tokens) == 0 || tokens[0].Decoded != n.text {
			return false
		}
		return matchNodes(n.childNodes, tokens[1:], captures)
	case *captureNode:
		if len(tokens) == 0 || tokens[0].Raw == "" || tokens[0].hasEncodedSlash() {
			return false
		}
		captures[n.name] = tokens[0].Decoded
		if matchNodes(n.childNodes, tokens[1:], captures) {
			return true
		}
		delete(captures, n.name) // Undo the binding if the subtree does not match.
		return false
	default:
		return false // Unreachable for a Compile-verified pattern.
	}
}

// matchNodes reports whether any of the nodes matches tokens.
func matchNodes(nodes []Node, tokens []Token, captures map[string]string) bool {
	if len(nodes) == 0 {
		return len(tokens) == 0
	}
	match := func(node Node) bool { return matchNode(node, tokens, captures) }
	return slices.ContainsFunc(nodes, match)
}
