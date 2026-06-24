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
	"reflect"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// kind enumerates the matcher node kinds.
type kind int

const (
	// kindLiteral matches a token equal to its text.
	kindLiteral kind = iota
	// kindGlob matches exactly one non-empty token that carries no percent
	// encoding, the `*` metacharacter. It is safe-only: an encoded slash is
	// never admitted, so a `*` cannot silently span one.
	kindGlob
	// kindCapture matches one token and binds it under capture.<name>, the
	// `{name}` metacharacter. Like glob, it is safe-only and rejects a token
	// that carries percent encoding.
	kindCapture
	// kindGreedy matches zero or more trailing tokens, the `**`
	// metacharacter. It is terminal and carries no children. It is repeated
	// safe glob: it spans only tokens with no percent encoding, so a broad `**`
	// never silently absorbs an encoded slash.
	kindGreedy
	// kindGlobEncoded matches exactly one non-empty token that is plain or
	// carries only an encoded char the node admits, kept raw. The admitted set
	// is held in allowedEncoded; today only the encoded separator (%2F/%2f, any
	// count) is supported. It is the explicit per-position opt-in that admits an
	// encoded char where a real API requires it.
	kindGlobEncoded
	// kindCaptureEncoded matches one such token and binds it raw under
	// capture.<name>, forwarded byte-faithfully with no decode and no re-encode.
	kindCaptureEncoded
	// kindEncodedLiteral matches one token whose decoded value equals its text,
	// where the admitted encoded chars are decoded for the comparison. text holds
	// the decoded value, so encoded_literal("a/b/c") matches the token
	// "a%2Fb%2Fc". The "/" in text is content, not a separator: the node is one
	// segment by definition, so unlike kindLiteral it never splits. The match is
	// hex case-insensitive (%2F and %2f both decode to "/"), and the raw token is
	// forwarded byte-faithfully. It is the only node that decodes for matching;
	// every other decode is a throwaway validation view.
	kindEncodedLiteral
	// kindRoot is the synthetic top node. It consumes no token and matches
	// each child against the same segment, so its children are alternative
	// roots OR-ed together. It is the one place an alternation can sit with no
	// consuming parent above it, so it is valid only as the matcher argument of
	// path.match and never nested.
	kindRoot
	// kindSlash is a terminal node that matches the trailing empty segment a
	// request path produces after a final "/". It is the named replacement for
	// the empty literal that once stood in for a trailing slash, so a literal
	// node never carries empty text.
	kindSlash
	// kindOptional is a terminal node that matches whether or not its subtree is
	// present: the path may end here, or one of the node's children matches the
	// remainder. It makes a trailing subtree optional, so optional(slash())
	// accepts "/foo" and "/foo/" alike, and optional(literal("reports")) accepts
	// "/files" and "/files/reports" from one tree, with no duplicated prefix.
	// Several children are alternatives. The skip branch binds nothing, so a
	// capture inside an optional is never guaranteed.
	kindOptional
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
	// allowedEncoded holds the encoded chars a kindGlobEncoded or
	// kindCaptureEncoded node admits in its segment. It is set by GlobEncoded
	// and CaptureEncoded. Today only the separator "/" is supported, so the
	// node admits a token that is plain or carries only the encoded separator.
	allowedEncoded []string
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
	// Every segment must be a non-empty, legal URL path segment. A caller that
	// passes an empty or illegally-encoded segment has a bug, since the
	// authoring surfaces (Compile and the predicate literal()) validate and
	// return an error before reaching here; panic rather than build a node that
	// can never match a real request path.
	for _, seg := range segments {
		if err := validateSegment(seg); err != nil {
			panic("resourcematcher: " + err.Error())
		}
	}
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

// Slash builds a terminal node that matches the trailing empty segment a
// request path produces after a final "/", so Literal("files", Slash()) matches
// "/files/" but not "/files", and Slash() alone matches the bare root "/". It
// is the named replacement for the empty literal, so an empty literal is never
// built.
func Slash() *Node {
	return &Node{kind: kindSlash}
}

// Optional builds a terminal node that makes its subtree optional: the path may
// end at this node, or one of the children matches the remainder. So
// Optional(Slash()) matches both "/files" and "/files/", and
// Optional(Literal("reports")) matches "/files" and "/files/reports" from one
// tree with no duplicated prefix. Several children are alternatives. It is a
// tail construct, the alternative to a greedy tail rather than a modifier on
// one, so it carries no string-pattern sugar and the declarative form lists the
// alternatives instead. An empty Optional is a load error.
func Optional(children ...*Node) (*Node, error) {
	if len(children) == 0 {
		return nil, trace.BadParameter("optional() requires at least one child subtree")
	}
	return &Node{kind: kindOptional, children: children}, nil
}

// validateSegment rejects an empty or illegally-encoded literal segment. A
// literal can only ever match a real request path segment, so it must be a
// non-empty, legal URL path segment. An empty segment is the trailing-slash
// pun that Slash now owns, and an illegal byte would only ever compile a dead
// rule. Both authoring surfaces, Compile and the predicate literal(), validate
// through here, so neither can build a segment the other would reject.
func validateSegment(seg string) error {
	if seg == "" {
		return trace.BadParameter("a literal segment cannot be empty; use slash() to match a trailing slash")
	}
	for i := range len(seg) {
		if !isLegalPathByte(seg[i]) {
			return trace.BadParameter("literal segment %q contains an illegal URL byte %q", seg, string(seg[i]))
		}
	}
	return nil
}

// validateLiteral validates every segment a literal string splits into on "/".
// It is the shared check the predicate literal() builder calls so a
// hand-written literal is held to the same rules as a compiled one.
func validateLiteral(s string) error {
	for _, seg := range strings.Split(s, "/") {
		if err := validateSegment(seg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Capture builds a node that matches one segment and binds it under
// capture.<name>, the `{name}` metacharacter.
func Capture(name string, children ...*Node) *Node {
	return &Node{kind: kindCapture, text: name, children: children}
}

// GlobEncoded builds a node that matches exactly one non-empty segment that is
// plain or carries only an admitted encoded char, kept raw. The allowed set
// names which encoded chars the segment may carry; today only the separator
// "/" is supported, so the node admits the encoded separator (%2F/%2f, any
// count). It is the explicit, per-position opt-in for an encoded char: a plain
// glob rejects any percent byte, so a rule names this matcher exactly where a
// real API requires an encoded char, such as a GitLab project id. It pairs with
// the allow_encoded option on path.match, which gates the whole match.
func GlobEncoded(allowed []string, children ...*Node) (*Node, error) {
	if err := validateEncodedChars(allowed); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Node{kind: kindGlobEncoded, allowedEncoded: allowed, children: children}, nil
}

// CaptureEncoded builds a node that matches one such segment and binds it under
// capture.<name>, the capture form of GlobEncoded. The bound value is the raw
// bytes, forwarded byte-faithfully with no decode and no re-encode, so the
// agent's view and the upstream's stay identical. The matcher sees one opaque
// blob and cannot reach inside it; an inner constraint belongs in a where:
// check on the captured raw string.
func CaptureEncoded(name string, allowed []string, children ...*Node) (*Node, error) {
	if err := validateEncodedChars(allowed); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Node{kind: kindCaptureEncoded, text: name, allowedEncoded: allowed, children: children}, nil
}

// EncodedLiteral builds a node that matches one segment whose decoded value
// equals value, with the admitted encoded chars decoded for the comparison.
// Today only the separator "/" is supported, so EncodedLiteral("a/b/c",
// set("/")) matches the token "a%2Fb%2Fc" or "a%2fb%2fc": the match is hex
// case-insensitive, the win over a plain literal, which pins the exact bytes.
// The "/" in value is content, the thing that was encoded, not a separator, so
// the node is one segment and never splits the way Literal does. The raw token
// is forwarded byte-faithfully. It pairs with the allow_encoded option on
// path.match, which gates the whole match.
func EncodedLiteral(value string, allowed []string, children ...*Node) (*Node, error) {
	if err := validateEncodedChars(allowed); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateEncodedLiteralValue(value); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Node{kind: kindEncodedLiteral, text: value, allowedEncoded: allowed, children: children}, nil
}

// validateEncodedLiteralValue rejects an encoded_literal value that is not a
// clean decoded path fragment. The value is what a request token decodes to, so
// each "/"-separated part must be a legal, non-empty URL path segment, must not
// be a relative "." or "..", and must carry no "%": the author writes the
// decoded form, and the node re-derives the encoded form to match.
func validateEncodedLiteralValue(value string) error {
	for _, part := range strings.Split(value, "/") {
		if err := validateSegment(part); err != nil {
			return trace.Wrap(err)
		}
		if part == "." || part == ".." {
			return trace.BadParameter("encoded_literal value %q has a relative segment %q", value, part)
		}
		if strings.ContainsRune(part, '%') {
			return trace.BadParameter(
				"encoded_literal value %q must be the decoded form and carry no %%-escape; write the plain value", value)
		}
	}
	return nil
}

// validateEncodedChars rejects an empty allowed set or any entry other than the
// separator "/". The model admits an encoded char only when it is byte-faithful
// to forward and structurally meaningful, and the separator is the only such
// char today, so the set is restricted to "/" until another is designed.
func validateEncodedChars(allowed []string) error {
	if len(allowed) == 0 {
		return trace.BadParameter("an encoded-char matcher must allow at least one char")
	}
	for _, c := range allowed {
		if c != "/" {
			return trace.BadParameter(
				"encoded char %q is not supported; only the separator %q is admitted for now", c, "/")
		}
	}
	return nil
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
		if containsOptional(e) {
			return nil, trace.BadParameter(
				"a greedy_except matcher cannot contain optional: its empty-match branch makes the exclusion match the zero-length tail and silently forbids the bare prefix")
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
	if n.kind == kindCapture || n.kind == kindCaptureEncoded {
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

// containsOptional reports whether the node tree contains an optional node,
// walking both the children and the greedy exclusion subtrees. An optional
// inside a greedy_except exclusion is rejected: its empty-match branch makes the
// exclusion match the zero-length tail, which refuses greedy's match-zero and
// silently forbids the bare prefix.
func containsOptional(n *Node) bool {
	if n == nil {
		return false
	}
	if n.kind == kindOptional {
		return true
	}
	for _, c := range n.children {
		if containsOptional(c) {
			return true
		}
	}
	for _, e := range n.greedyExcept {
		if containsOptional(e) {
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
		// including zero tokens. It is repeated safe glob, so the suffix must
		// carry no percent encoding: a single encoded segment anywhere in the
		// tail stops the greedy, which is why a broad `**` never silently
		// absorbs an encoded slash.
		for _, tok := range tokens[i:] {
			if strings.ContainsRune(tok, '%') {
				return false
			}
		}
		// When the node carries exclusions, the suffix must match none of them:
		// each exclusion is a negative test, walked against a throwaway capture
		// map so a binding inside an exclusion never leaks into the real match.
		for _, excl := range node.greedyExcept {
			if matchNode(excl, tokens, i, map[string]string{}) {
				return false
			}
		}
		return true
	case kindSlash:
		// A trailing slash is the empty segment that ends the token list. It
		// is terminal: the empty token must exist and be the last one.
		return i < len(tokens) && tokens[i] == "" && i+1 == len(tokens)
	case kindOptional:
		// The subtree is optional: match either the end of the path, where it is
		// absent, or one of the children against the remainder. The end branch
		// binds nothing, which is why a capture inside an optional is never
		// guaranteed.
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
		if i >= len(tokens) || tokens[i] != node.text {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindGlob:
		// Safe-only: a glob matches one segment that carries no percent
		// encoding, so it never spans an encoded slash.
		if i >= len(tokens) || tokens[i] == "" || strings.ContainsRune(tokens[i], '%') {
			return false
		}
		if slices.Contains(node.globExclude, tokens[i]) {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindCapture:
		// Safe-only, like glob: a capture binds one segment with no percent
		// encoding. Use capture_encoded to bind an encoded segment.
		if i >= len(tokens) || tokens[i] == "" || strings.ContainsRune(tokens[i], '%') {
			return false
		}
		// Bind before descending so a child predicate can read the capture.
		caps[node.text] = tokens[i]
		return matchChildren(node, tokens, i, caps)
	case kindGlobEncoded:
		// Admit one segment that is plain or carries only the encoded separator.
		if i >= len(tokens) || tokens[i] == "" || !onlyEncodedSlash(tokens[i]) {
			return false
		}
		return matchChildren(node, tokens, i, caps)
	case kindCaptureEncoded:
		if i >= len(tokens) || tokens[i] == "" || !onlyEncodedSlash(tokens[i]) {
			return false
		}
		// Bind the raw bytes so the captured value forwards byte-faithfully.
		caps[node.text] = tokens[i]
		return matchChildren(node, tokens, i, caps)
	case kindEncodedLiteral:
		// Decode the admitted encoded chars and compare to the literal value, so
		// the token's hex case does not matter. The raw token is still what gets
		// forwarded; this decode is only the comparison.
		if i >= len(tokens) || tokens[i] == "" || !onlyEncodedSlash(tokens[i]) {
			return false
		}
		if decodeSeparators(tokens[i]) != node.text {
			return false
		}
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

// onlyEncodedSlash reports whether token carries no percent-escape other than
// the encoded separator %2F/%2f. The encoded-slash nodes admit a token only
// when this holds, so a plain id and an encoded id both match but a token with
// any other escape does not. Tokenize already guarantees this for every real
// request token, so the check is a self-contained guard that keeps the node
// correct even when a token reaches it directly, such as in a unit test.
func onlyEncodedSlash(token string) bool {
	for i := 0; i < len(token); i++ {
		if token[i] != '%' {
			continue
		}
		if i+2 >= len(token) || token[i+1] != '2' || (token[i+2] != 'F' && token[i+2] != 'f') {
			return false
		}
	}
	return true
}

// mergeAlternatives folds a list of alternative matcher trees into the minimal
// set that shares common prefixes. Two alternatives whose head node matches the
// same segment the same way collapse into one node whose children are the
// merged continuations, so paths that share a prefix branch only where they
// diverge: "/api/v4/health" and "/api/v4/status" become one "api/v4" chain with
// "health" and "status" as sibling children, never a root() of two full chains
// that duplicate the prefix. A terminal alternative never merges into a
// non-terminal one, since a node cannot both end a match and require a
// continuation, so "/api" and "/api/v4" stay distinct alternatives. The input
// trees are freshly compiled per path and unshared, so merging mutates them in
// place.
func mergeAlternatives(nodes []*Node) []*Node {
	var out []*Node
	for _, n := range nodes {
		merged := false
		for _, e := range out {
			if !mergeableHead(e, n) {
				continue
			}
			// Both non-terminal: merge their continuations. Both terminal: they
			// are identical alternatives, so dropping n dedupes to the one in out.
			if len(n.children) > 0 {
				e.children = mergeAlternatives(append(e.children, n.children...))
			}
			merged = true
			break
		}
		if !merged {
			out = append(out, n)
		}
	}
	return out
}

// mergeableHead reports whether two alternatives can collapse into one node:
// they must match the same segment the same way, and both end at this node or
// both continue past it, since a node cannot be terminal and non-terminal at
// once.
func mergeableHead(a, b *Node) bool {
	return sameHead(a, b) && (len(a.children) == 0) == (len(b.children) == 0)
}

// sameHead reports whether two nodes match a segment identically, ignoring
// their children. It compares every field but children, so two literals with
// the same text, or two globs with the same exclusions, share a head.
func sameHead(a, b *Node) bool {
	ah, bh := *a, *b
	ah.children, bh.children = nil, nil
	return reflect.DeepEqual(ah, bh)
}

// Compile parses a declarative path pattern such as
// "/api/v4/projects/{project}/**" into the same Node tree the constructors
// build. This is the string sugar: it desugars to the canonical internal form,
// so the declarative and predicate surfaces cannot diverge. The mapping is
// `{name}` to Capture, `*` to Glob, `**` to Greedy, and any other segment to
// Literal.
//
// Three further forms admit the encoded separator and carve-outs without
// dropping to the expression surface. `{name:/}` maps to CaptureEncoded, the
// capture that binds a segment carrying an encoded slash, with the char after
// the colon naming the admitted encoded set. `{:/}`, with an empty name, maps
// to GlobEncoded, its anonymous form. A segment that starts with "!", such as
// `!secret`, maps to GlobWithout over the rest of the segment, so it matches
// any one segment except that value and continues to the children. A segment
// that genuinely needs a leading "!" falls back to the expression surface.
//
// A leading "/" is required and stripped. An interior or leading empty
// segment (from "//") is a compile error, since the strict URL rules reject
// the request bytes that would produce one. A trailing "/" is significant:
// it compiles to a slash() node matching the trailing empty segment a request
// path produces, so "/foo/" matches the request "/foo/" but not "/foo", and
// the bare root "/" matches only the request "/". A `**` in any non-final
// position is a compile error.
func Compile(pattern string) (*Node, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, trace.BadParameter("path pattern %q must start with /", pattern)
	}
	segments := splitPattern(strings.TrimPrefix(pattern, "/"))
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

// splitPattern splits a path pattern into segments on "/", but only at brace
// depth 0, so the "/" inside a `{name:/}` or `{:/}` encoded form stays part of
// the one segment rather than splitting it. Outside braces it is exactly
// strings.Split on "/", so it preserves the empty-segment semantics Compile
// relies on: "files/" splits to ["files", ""], "" to [""], and "//" to
// ["", ""]. An unbalanced "}" is left to compileSegment, which rejects the
// malformed segment.
func splitPattern(s string) []string {
	var segs []string
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '{':
			depth++
			b.WriteRune(r)
		case '}':
			if depth > 0 {
				depth--
			}
			b.WriteRune(r)
		case '/':
			if depth == 0 {
				segs = append(segs, b.String())
				b.Reset()
				continue
			}
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return append(segs, b.String())
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
	case seg == "":
		// An empty segment. Compile already guaranteed it can only be the final
		// one, the trailing slash, so it maps to the trailing-slash node rather
		// than an empty literal.
		return Slash(), nil
	case strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}"):
		return compileBraceSegment(pattern, seg[1:len(seg)-1])
	case strings.HasPrefix(seg, "!"):
		// A leading "!" carves one value out of an otherwise open glob, so
		// "!secret" matches any one segment except "secret" and continues to the
		// children. The excluded value is the rest of the segment; a bare "!" has
		// nothing to exclude and is a malformed pattern.
		excluded := seg[1:]
		if excluded == "" {
			return nil, trace.BadParameter("carve-out %q in %q needs a value after the !, such as !secret", seg, pattern)
		}
		node, err := GlobWithout([]string{excluded})
		if err != nil {
			return nil, trace.Wrap(err, "in path pattern %q", pattern)
		}
		return node, nil
	default:
		// A bare segment is a single literal. A metacharacter is only valid as
		// a whole segment: "*", "**", or "{name}". Any other appearance, such
		// as "***", "a*", or a stray brace, is a malformed pattern rather than
		// a literal, so reject it instead of treating it as literal text. The
		// segment cannot contain "/", which the split already guaranteed.
		if strings.ContainsAny(seg, "*{}") {
			return nil, trace.BadParameter("segment %q in %q is not a valid literal, glob (*), greedy (**), or capture ({name})", seg, pattern)
		}
		// A literal can only match a request path, so it must itself be a legal,
		// non-empty URL path segment. Validate through the shared check so the
		// string and predicate surfaces hold a literal to identical rules.
		if err := validateSegment(seg); err != nil {
			return nil, trace.Wrap(err, "in path pattern %q", pattern)
		}
		return &Node{kind: kindLiteral, text: seg}, nil
	}
}

// compileBraceSegment maps the interior of a `{...}` segment to its node kind.
// A plain interior, "{name}", is a Capture. A ":"-suffixed interior names the
// admitted encoded set after the colon and the char before it: "{name:/}" is a
// CaptureEncoded that binds a segment carrying the encoded separator, and
// "{:/}", with an empty name, is its anonymous GlobEncoded form. The char(s)
// after the colon spell the encoded set literally, so "/" lowers to set("/"),
// mirroring the constructor; today only "/" is admitted, which the constructor
// validates. An empty set, "{name:}", is a malformed pattern.
func compileBraceSegment(pattern, interior string) (*Node, error) {
	name, enc, hasEncoded := strings.Cut(interior, ":")
	if !hasEncoded {
		if !isIdentifier(name) {
			return nil, trace.BadParameter("capture name %q in %q must be a valid identifier so it reads as vars.<name>", name, pattern)
		}
		return Capture(name), nil
	}
	if enc == "" {
		return nil, trace.BadParameter(
			"encoded form %q in %q needs a char after the colon naming the admitted encoded set, such as {%s:/}", "{"+interior+"}", pattern, name)
	}
	allowed := splitEncoded(enc)
	if name == "" {
		node, err := GlobEncoded(allowed)
		return node, trace.Wrap(err, "in path pattern %q", pattern)
	}
	if !isIdentifier(name) {
		return nil, trace.BadParameter("capture name %q in %q must be a valid identifier so it reads as vars.<name>", name, pattern)
	}
	node, err := CaptureEncoded(name, allowed)
	return node, trace.Wrap(err, "in path pattern %q", pattern)
}

// splitEncoded turns the encoded-set spelling after a colon into one entry per
// rune, so "/" becomes []string{"/"}, the form the encoded constructors take.
// Forward-compatible if another encoded char is ever admitted: "/x" becomes
// []string{"/", "x"}.
func splitEncoded(s string) []string {
	out := make([]string, 0, len(s))
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
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
