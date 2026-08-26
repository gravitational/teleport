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
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"golang.org/x/text/unicode/norm"
)

// Node is one node in a matcher tree. It is a recursive structure of a node
// implementation with its own data and its Node children. The children of a
// node are alternative continuations of paths that share the prefix up to
// this node, so the two patterns /api/v1/** and /api/health become
//
//	Literal("api", Literal("v1", Greedy()), Literal("health"))
//
// which is the tree
//
//	api
//	├── v1
//	│   └── **
//	└── health
//
// A Node is created with the node constructors in this package, for example
// [Literal] and [Glob]. [Compile] verifies a finished tree.
type Node interface {
	// String returns the node as its constructor call in the predicate language
	// spelling, such as literal("api") or glob().
	String() string
	children() []Node
}

// literalNode is the node [Literal] creates.
type literalNode struct {
	text       string
	childNodes []Node
}

func (n *literalNode) children() []Node { return n.childNodes }
func (n *literalNode) String() string   { return fmt.Sprintf("literal(%q)", n.text) }

// Literal creates a node that matches one or more fixed tokens. The string is
// split on "/", so Literal("foo/bar", child) matches the token sequence ["foo",
// "bar"] and is equal to Literal("foo", Literal("bar", child)).
//
// The given text must be the decoded content of tokens Tokenize accepts, for
// example "my x" rather than "my%20x", and must not start or end with "/" or
// contain "//". A token containing the encoded slash (%2F) does not match.
//
// For invalid given text, Literal returns an error node that Compile rejects.
func Literal(text string, children ...Node) Node {
	segments := strings.Split(text, "/")
	for _, seg := range segments {
		if err := validateSegment(seg); err != nil {
			return &errNode{failedCall: fmt.Sprintf("literal(%q)", text), err: err}
		}
	}
	root := &literalNode{text: segments[0]}
	node := root
	for _, seg := range segments[1:] {
		child := &literalNode{text: seg}
		node.childNodes = []Node{child}
		node = child
	}
	node.childNodes = slices.Clone(children)
	return root
}

// globNode is the node [Glob] creates.
type globNode struct {
	childNodes []Node
}

func (n *globNode) children() []Node { return n.childNodes }
func (n *globNode) String() string   { return "glob()" }

// Glob creates a node that matches exactly one non-empty token of any content.
// It is the desugared `*` metacharacter. Literal("api", Glob()) matches
// "/api/v1", but not "/api", "/api/", or "/api/v1/schema". A token containing
// the encoded slash (%2F) does not match.
func Glob(children ...Node) Node {
	return &globNode{childNodes: slices.Clone(children)}
}

// captureNode is the node [Capture] creates.
type captureNode struct {
	name       string
	childNodes []Node
}

func (n *captureNode) children() []Node { return n.childNodes }
func (n *captureNode) String() string   { return fmt.Sprintf("capture(%q)", n.name) }

// captureNameRE matches ASCII identifiers.
var captureNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Capture creates a node that matches exactly one non-empty token and binds its
// decoded content to a named variable. It is the desugared `{name}`
// placeholder. A token containing the encoded slash (%2F) does not match.
//
// The name must be an ASCII identifier ([A-Za-z_][A-Za-z0-9_]*). For an invalid
// name, Capture returns an error node that Compile rejects.
func Capture(name string, children ...Node) Node {
	if !captureNameRE.MatchString(name) {
		err := trace.BadParameter("capture name %q must be a letter or underscore followed by letters, digits, or underscores", elide(name))
		return &errNode{failedCall: fmt.Sprintf("capture(%q)", name), err: err}
	}
	return &captureNode{name: name, childNodes: slices.Clone(children)}
}

// greedyNode is the node [Greedy] creates.
type greedyNode struct{}

func (*greedyNode) children() []Node { return nil }
func (*greedyNode) String() string   { return "greedy()" }

// Greedy creates a greedy glob, a terminal node that matches zero or more
// remaining tokens. It is the desugared `**` metacharacter. Literal("api",
// Greedy()) matches "/api", "/api/", "/api/v1", "/api/v1/schema" etc. Any token
// containing the encoded slash (%2F) fails the match.
func Greedy() Node {
	return &greedyNode{}
}

// slashNode is the node [Slash] creates.
type slashNode struct{}

func (*slashNode) children() []Node { return nil }
func (*slashNode) String() string   { return "slash()" }

// Slash creates a terminal node that represents a trailing slash. To be
// precise, it matches the trailing empty token so that Literal("files",
// Slash()) matches "/files/" but not "/files", and Slash() alone matches the
// bare root "/".
func Slash() Node {
	return &slashNode{}
}

// optionalNode is the node [Optional] creates.
type optionalNode struct {
	childNodes []Node
}

func (n *optionalNode) children() []Node { return n.childNodes }
func (n *optionalNode) String() string   { return "optional()" }

// Optional creates a node that makes its subtree optional. The path may end at
// this node, or one of the children matches the remainder. So Literal("files",
// Optional(Slash())) matches both "/files" and "/files/", and Literal("files",
// Optional(Literal("reports"))) matches "/files" and "/files/reports" from one
// tree.
//
// If no children are given, Optional returns an error node that Compile
// rejects.
func Optional(children ...Node) Node {
	if len(children) == 0 {
		return &errNode{failedCall: "optional()", err: trace.BadParameter("optional requires at least one child subtree")}
	}
	return &optionalNode{childNodes: slices.Clone(children)}
}

// rootNode is the node [Root] creates.
type rootNode struct {
	childNodes []Node
}

func (n *rootNode) children() []Node { return n.childNodes }
func (n *rootNode) String() string   { return "root()" }

// Root creates the synthetic top node whose children are alternative first
// tokens, such as Root(Literal("api"), Literal("health")). It consumes no token
// of its own.
//
// If no children are given, Root returns an error node that Compile rejects.
func Root(children ...Node) Node {
	if len(children) == 0 {
		return &errNode{failedCall: "root()", err: trace.BadParameter("root requires at least one alternative")}
	}
	return &rootNode{childNodes: slices.Clone(children)}
}

// errNode replaces the node that could not be created by a constructor, e.g.
// [Literal]. Returning an error node instead of an error value keeps the
// constructors nestable. Compile reports the constructor's error, prefixed
// with the failed call at the node's position in the tree.
type errNode struct {
	failedCall string
	err        error
}

func (*errNode) children() []Node { return nil }
func (n *errNode) String() string { return n.failedCall }

// validateSegment rejects invalid literal text segments. The text must be
// non-empty, hold no "%", and pass the same checks Tokenize runs on a token's
// decoded form. For example, "my%20x" and ".." are rejected.
func validateSegment(seg string) error {
	if seg == "" {
		return trace.BadParameter("a literal segment cannot be empty; use Slash to match a trailing slash")
	}
	if strings.ContainsRune(seg, '%') {
		return trace.BadParameter("literal segment %q contains %%; write the decoded content instead", elide(seg))
	}
	if !utf8.ValidString(seg) || !norm.NFKC.IsNormalString(seg) {
		return trace.BadParameter("literal segment %q is not NFKC-normalized UTF-8", elide(seg))
	}
	for _, r := range seg {
		if r < utf8.RuneSelf && !isLegalPathByte(byte(r)) && r != ' ' {
			return trace.BadParameter("literal segment %q contains an illegal URL byte %q", elide(seg), string(r))
		}
		if !isGraphicRune(r) {
			return trace.BadParameter("literal segment %q contains the disallowed character %q", elide(seg), string(r))
		}
	}
	if err := rejectAmbiguousSegment(seg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
