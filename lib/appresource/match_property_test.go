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
	"maps"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// segmentText draws plain path segments that both Tokenize and validateSegment
// accept.
var segmentText = rapid.StringMatching(`[a-zA-Z0-9_\-]{1,8}`)

// smallAlphabet is the tiny alphabet smallText draws from, so a drawn tree
// often matches a drawn path and often binds one capture name at several
// depths.
var smallAlphabet = []string{"a", "b", "c"}

// smallText draws literal text, capture names, and tokens from smallAlphabet.
var smallText = rapid.SampledFrom(smallAlphabet)

// encodedSegmentText draws path segments holding a percent-escape, so a drawn
// path reaches the encoded-separator and decoded-content branches.
var encodedSegmentText = rapid.SampledFrom([]string{"a%2Fb", "a%20b", "caf%C3%A9"})

// The node kinds drawTree draws from.
const (
	kindLiteral = iota
	kindGlob
	kindGlobWithout
	kindCapture
	kindGreedy
	kindSlash
	kindOptional
	kindRoot
)

// treeKind draws a node kind for drawTree. Capture appears three times, so a
// drawn tree usually holds one and the bindings get exercised.
var treeKind = rapid.SampledFrom([]int{kindLiteral, kindGlob, kindGlobWithout, kindCapture, kindCapture, kindCapture, kindGreedy, kindSlash, kindOptional, kindRoot})

// drawTree draws an arbitrary tree of bounded depth and width that
// Compile accepts.
func drawTree(t *rapid.T, depth int, top bool, bound []string) Node {
	drawChildren := func(bound []string) []Node {
		if depth == 0 {
			return nil
		}
		return rapid.SliceOfN(rapid.Custom(func(t *rapid.T) Node {
			return drawTree(t, depth-1, false, bound)
		}), 0, 3).Draw(t, "children")
	}
	switch treeKind.Draw(t, "kind") {
	case kindLiteral:
		return Literal(smallText.Draw(t, "literal"), drawChildren(bound)...)
	case kindGlob:
		return Glob(drawChildren(bound)...)
	case kindGlobWithout:
		return GlobWithout([]string{smallText.Draw(t, "exclude")}, drawChildren(bound)...)
	case kindCapture:
		var free []string
		for _, name := range smallAlphabet {
			if !slices.Contains(bound, name) {
				free = append(free, name)
			}
		}
		if len(free) == 0 {
			return Glob(drawChildren(bound)...)
		}
		name := rapid.SampledFrom(free).Draw(t, "capture")
		return Capture(name, drawChildren(append(slices.Clone(bound), name))...)
	case kindGreedy:
		return Greedy()
	case kindSlash:
		return Slash()
	case kindOptional:
		children := drawChildren(bound)
		if len(children) == 0 {
			return Greedy()
		}
		return Optional(children...)
	default:
		children := drawChildren(bound)
		if len(children) == 0 {
			return Greedy()
		}
		if !top {
			return Optional(children...)
		}
		return Root(children...)
	}
}

// compilePattern compiles a drawn tree, failing the property on a tree Compile
// rejects, since drawTree only creates accepted trees.
func compilePattern(t *rapid.T, root Node) *Pattern {
	pattern, err := Compile(root)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return pattern
}

// matchRef matches like Match with cloned bindings and an index instead
// of a shared map and reslicing, for TestMatchEqualsReferenceWalk.
func matchRef(node Node, tokens []Token, i int, captures map[string]string) (bool, map[string]string) {
	switch n := node.(type) {
	case *rootNode:
		return matchRefChildren(n.childNodes, tokens, i, captures)
	case *optionalNode:
		if i == len(tokens) {
			return true, captures
		}
		return matchRefChildren(n.childNodes, tokens, i, captures)
	case *greedyNode:
		for _, tok := range tokens[i:] {
			if tok.hasEncodedSlash() {
				return false, nil
			}
		}
		return true, captures
	case *slashNode:
		if i < len(tokens) && tokens[i].Raw == "" && i+1 == len(tokens) {
			return true, captures
		}
		return false, nil
	case *literalNode:
		if i >= len(tokens) || tokens[i].Decoded != n.text {
			return false, nil
		}
	case *globNode:
		if i >= len(tokens) || tokens[i].Raw == "" || tokens[i].hasEncodedSlash() {
			return false, nil
		}
	case *globWithoutNode:
		if i >= len(tokens) || tokens[i].Raw == "" || tokens[i].hasEncodedSlash() || slices.Contains(n.excludes, tokens[i].Decoded) {
			return false, nil
		}
	case *captureNode:
		if i >= len(tokens) || tokens[i].Raw == "" || tokens[i].hasEncodedSlash() {
			return false, nil
		}
		captures = maps.Clone(captures)
		captures[n.name] = tokens[i].Decoded
	default:
		return false, nil
	}
	// The token-consuming kinds share this tail.
	childNodes := node.children()
	if len(childNodes) == 0 {
		if i+1 == len(tokens) {
			return true, captures
		}
		return false, nil
	}
	return matchRefChildren(childNodes, tokens, i+1, captures)
}

// matchRefChildren returns the bindings of the first child that matches.
func matchRefChildren(children []Node, tokens []Token, i int, captures map[string]string) (bool, map[string]string) {
	for _, child := range children {
		if matched, out := matchRef(child, tokens, i, captures); matched {
			return true, out
		}
	}
	return false, nil
}

// TestMatchOwnLiteralChain checks that a path always matches the literal chain
// created from its own segments.
func TestMatchOwnLiteralChain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		segments := rapid.SliceOfN(segmentText, 1, 5).Draw(t, "segments")
		tokens, err := Tokenize("/" + strings.Join(segments, "/"))
		if err != nil {
			t.Fatalf("Tokenize: %v", err)
		}
		pattern := compilePattern(t, Literal(strings.Join(segments, "/")))
		matched, captures := Match(pattern, tokens)
		if !matched {
			t.Fatalf("path %q does not match its own literal chain", segments)
		}
		if len(captures) != 0 {
			t.Fatalf("literal chain bound captures %v", captures)
		}
	})
}

// TestMatchArbitraryTrees checks that Match never panics for any compiled
// constructor tree and any Tokenize-accepted path, that a no-match returns a
// nil map, and that a match binds only names a capture node in the tree can
// bind.
func TestMatchArbitraryTrees(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		root := drawTree(t, 3, true, nil)
		segments := rapid.SliceOfN(rapid.OneOf(segmentText, encodedSegmentText), 0, 5).Draw(t, "segments")
		path := "/" + strings.Join(segments, "/")
		if rapid.Bool().Draw(t, "trailingSlash") && len(segments) > 0 {
			path += "/"
		}
		tokens, err := Tokenize(path)
		if err != nil {
			t.Fatalf("Tokenize(%q): %v", path, err)
		}
		matched, captures := Match(compilePattern(t, root), tokens)
		if !matched && captures != nil {
			t.Fatalf("no match for %q returned a non-nil map %v", path, captures)
		}
		names := captureNames(root)
		for name := range captures {
			if !slices.Contains(names, name) {
				t.Fatalf("match for %q bound %q, which no capture node holds", path, name)
			}
		}
	})
}

// TestMatchEqualsReferenceWalk compares Match against matchRef for arbitrary
// trees and paths: same match result, same bindings.
func TestMatchEqualsReferenceWalk(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		root := drawTree(t, 3, true, nil)
		segments := rapid.SliceOfN(rapid.OneOf(smallText, encodedSegmentText), 0, 5).Draw(t, "segments")
		path := "/" + strings.Join(segments, "/")
		if rapid.Bool().Draw(t, "trailingSlash") && len(segments) > 0 {
			path += "/"
		}
		tokens, err := Tokenize(path)
		if err != nil {
			t.Fatalf("Tokenize(%q): %v", path, err)
		}
		matched, captures := Match(compilePattern(t, root), tokens)
		wantMatched, wantCaptures := matchRef(root, tokens, 0, map[string]string{})
		if matched != wantMatched {
			t.Fatalf("Match(%q) = %v, reference walk = %v", path, matched, wantMatched)
		}
		if !matched {
			return
		}
		if !maps.Equal(captures, wantCaptures) {
			t.Fatalf("Match(%q) bound %v, reference walk bound %v", path, captures, wantCaptures)
		}
	})
}
