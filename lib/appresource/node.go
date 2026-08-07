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
	"unicode/utf8"

	"github.com/gravitational/trace"
	"golang.org/x/text/unicode/norm"
)

// kind enumerates the matcher node kinds.
type kind int

const (
	// kindLiteral matches a token whose content view equals its text.
	kindLiteral kind = iota
	// kindGlob matches exactly one non-empty token that carries no encoded
	// separator, the `*` metacharacter. A `*` therefore never silently
	// spans an encoded slash.
	kindGlob
	// kindCapture matches one token like kindGlob and binds its content
	// view under the capture name, the `{name}` metacharacter.
	kindCapture
	// kindGreedy matches zero or more trailing tokens, the `**`
	// metacharacter. It is terminal and carries no children. It spans only
	// tokens with no encoded separator, so a broad `**` never silently
	// absorbs an encoded slash.
	kindGreedy
	// kindRoot is the synthetic top node. It consumes no token and matches
	// each child against the same segment, so its children are alternative
	// roots OR-ed together.
	kindRoot
	// kindSlash is a terminal node that matches the trailing empty segment a
	// request path produces after a final "/". It is the named replacement
	// for an empty literal, so a literal node never carries empty text.
	kindSlash
	// kindOptional is a terminal node that matches whether or not its
	// subtree is present: the path may end here, or one of the node's
	// children matches the remainder. Several children are alternatives.
	// The skip branch binds nothing, so a capture inside an optional is
	// never guaranteed.
	kindOptional
)

// Node is one node in a matcher tree. Each node matches exactly one path
// segment and carries its continuation as zero or more child nodes. Nesting
// a single child descends to the next segment, so a chain of single children
// is a sequence. Giving one node several children branches into
// alternatives, so there is no separate sequence or alternation node. A Node
// is an ordinary Go value, so a matcher can be constructed and asserted on
// in tests with no cluster.
type Node struct {
	kind kind
	// text holds the literal text for kindLiteral and the capture name for
	// kindCapture. It is empty for every other kind.
	text string
	// children are the matchers for the next segment. Several children are
	// alternatives, OR-ed together. A node with no children is terminal:
	// the subject must end at this segment for the match to succeed.
	children []*Node
}

// Literal builds a node that matches one or more fixed segments. The string
// is split on "/", so Literal("a/b/c", child) is exactly equal to
// Literal("a", Literal("b", Literal("c", child))). A path segment can never
// contain a "/" (it is the separator), so splitting here loses nothing and
// keeps a single canonical internal form. Metacharacters inside the string
// (`{}`, `*`, `**`) are treated as plain literal text, not as captures or
// globs; use Capture, Glob, and Greedy for those. The text is the decoded
// content: a segment that a request sends encoded, such as a space or
// non-ASCII text, is written plain. Literal panics on a segment that
// validateLiteral rejects; the predicate literal() validates and returns
// the error before building a node.
func Literal(s string, children ...*Node) *Node {
	segments := strings.Split(s, "/")
	for _, seg := range segments {
		if err := validateSegment(seg); err != nil {
			panic("appresource: " + err.Error())
		}
	}
	// Build from the innermost segment outward so the supplied children
	// hang off the last segment.
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

// Capture builds a node that matches one segment and binds its content view
// under name, the `{name}` metacharacter.
func Capture(name string, children ...*Node) *Node {
	return &Node{kind: kindCapture, text: name, children: children}
}

// Greedy builds a terminal node that matches zero or more trailing segments,
// the `**` metacharacter. It takes no children.
func Greedy() *Node {
	return &Node{kind: kindGreedy}
}

// Slash builds a terminal node that matches the trailing empty segment a
// request path produces after a final "/", so Literal("files", Slash())
// matches "/files/" but not "/files", and Slash() alone matches the bare
// root "/".
func Slash() *Node {
	return &Node{kind: kindSlash}
}

// Optional builds a terminal node that makes its subtree optional: the path
// may end at this node, or one of the children matches the remainder. So
// Optional(Slash()) matches both "/files" and "/files/", and
// Optional(Literal("reports")) matches "/files" and "/files/reports" from
// one tree with no duplicated prefix. Several children are alternatives. An
// empty Optional is a load error.
func Optional(children ...*Node) (*Node, error) {
	if len(children) == 0 {
		return nil, trace.BadParameter("optional() requires at least one child subtree")
	}
	return &Node{kind: kindOptional, children: children}, nil
}

// Root builds the synthetic top node that matches each child against the
// same segment, so the children are alternative roots OR-ed together. It is
// the way to give a tree several first segments, such as
// Root(Literal("api"), Literal("admin")), which a bare tree cannot express
// because it has one root node. It consumes no token, so a nested root is a
// grouping that behaves the same as sibling children in the parent. An
// empty Root matches nothing and is a load error.
func Root(children ...*Node) (*Node, error) {
	if len(children) == 0 {
		return nil, trace.BadParameter("root() requires at least one alternative")
	}
	return &Node{kind: kindRoot, children: children}, nil
}

// validateSegment rejects a literal segment that no request token's content
// view can equal. The text must be non-empty, hold no "%" or "/", and pass
// the same decoded-view checks Tokenize runs on a request segment, so a
// literal that validates can never be a dead rule. An empty segment is the
// trailing-slash pun that Slash owns.
func validateSegment(seg string) error {
	if seg == "" {
		return trace.BadParameter("a literal segment cannot be empty; use slash() to match a trailing slash")
	}
	// A literal pins the decoded content, so a "%" in it is a dead rule: a
	// token's content view never contains one unless the raw token carried
	// the encoded separator, which a plain literal never matches.
	if strings.ContainsRune(seg, '%') {
		return trace.BadParameter("literal segment %q contains %%; write the decoded content instead", seg)
	}
	if !utf8.ValidString(seg) || !norm.NFKC.IsNormalString(seg) {
		return trace.BadParameter("literal segment %q is not NFKC-normalized UTF-8", seg)
	}
	for _, r := range seg {
		if r < utf8.RuneSelf && !isLegalPathByte(byte(r)) && r != ' ' {
			return trace.BadParameter("literal segment %q contains an illegal URL byte %q", seg, string(r))
		}
		if !isGraphicRune(r) {
			return trace.BadParameter("literal segment %q contains the disallowed character %q", seg, string(r))
		}
	}
	if err := rejectDotSegment(seg); err != nil {
		return trace.Wrap(err)
	}
	if err := rejectLeadingMark(seg); err != nil {
		return trace.Wrap(err)
	}
	if err := rejectEdgeSpace(seg); err != nil {
		return trace.Wrap(err)
	}
	if err := rejectTrailingDot(seg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateLiteral validates every segment a literal string splits into on
// "/". It is the check the predicate literal() builder calls, so an
// expression-built literal is held to the same rules as the Literal
// constructor.
func validateLiteral(s string) error {
	for _, seg := range strings.Split(s, "/") {
		if err := validateSegment(seg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
