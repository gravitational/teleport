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

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strconv"

	"github.com/gravitational/trace"
)

// validateCaptures is the load-time capture check. It rejects a rule whose
// predicate reads a vars.<name> that the matcher does not bind on every path
// the rule can match. The intersection, not the union, is the safe set: a
// vars.<name> read is only sound when every matching path binds it, so the
// value is present no matter which alternative matched.
//
// This catches the cross-path mistake at load. A rule with
//
//	paths: ["/project/{project}", "/user/{user}"]
//	where: vars.project == ... || vars.user == ...
//
// binds project on one path and user on the other, so neither is bound on
// every path. The fix is the structure the RFD prescribes: split it into one
// rule per path, each carrying its own condition. The additive union across
// rules then grants each path under its own capture.
//
// The check restores at load the type-safety that vars.<name> defers to
// evaluation. Without it a typo, or a capture from the wrong path, would be
// silently unbound at request time. The runtime guard (evalState.unboundRead)
// still backstops the case this static check cannot see, such as a read placed
// before its own path.match by author error.
//
// The predicate is the same Go expression syntax the engine parses, so this
// reuses go/parser to walk the AST. An expression that does not parse is left
// to the engine, which reports the parse error with full type information.
func validateCaptures(expr string) error {
	parsed, err := goparser.ParseExpr(expr)
	if err != nil {
		return nil
	}

	guaranteed := guaranteedCaptures(parsed)
	for _, name := range referencedVars(parsed) {
		if !guaranteed[name] {
			return trace.BadParameter(
				"predicate reads vars.%s but not every path in the rule binds %q; "+
					"reference a capture only where every matching path defines it, "+
					"or split the rule so each path carries its own condition",
				name, name)
		}
	}
	return nil
}

// guaranteedCaptures returns the capture names bound no matter how the
// predicate evaluates to true. It walks the boolean structure of the
// expression: an && of two matches binds the captures of both, so its set is
// the union; an || binds only the captures common to every branch, so its set
// is the intersection; and a negated match must be false to pass, so it binds
// nothing. A path.match call binds the captures in its matcher root.
//
// This models the cross-path || the declarative multi-path form lowers to: two
// paths joined by || guarantee only the captures both define, so a
// vars.<name> read is sound only where every matching path binds it. The
// runtime guard (evalState.unboundRead) still backstops any case this static
// walk cannot see.
func guaranteedCaptures(node ast.Node) map[string]bool {
	switch n := node.(type) {
	case *ast.ParenExpr:
		return guaranteedCaptures(n.X)
	case *ast.BinaryExpr:
		switch n.Op {
		case token.LAND:
			return unionSets(guaranteedCaptures(n.X), guaranteedCaptures(n.Y))
		case token.LOR:
			return intersectSets(guaranteedCaptures(n.X), guaranteedCaptures(n.Y))
		default:
			return map[string]bool{}
		}
	case *ast.UnaryExpr:
		// A negated match must be false to pass, so it binds nothing.
		return map[string]bool{}
	case *ast.CallExpr:
		if isPathMatch(n) {
			if len(n.Args) == 0 {
				return map[string]bool{}
			}
			// Pass the matcher tree, not the whole path.match call, so a
			// top-level root() is recognised and its alternatives intersected.
			return capturesIn(n.Args[0])
		}
		return map[string]bool{}
	default:
		return map[string]bool{}
	}
}

// unionSets returns the names present in either set.
func unionSets(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for name := range a {
		out[name] = true
	}
	for name := range b {
		out[name] = true
	}
	return out
}

// intersectSets returns the names present in both sets.
func intersectSets(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for name := range a {
		if b[name] {
			out[name] = true
		}
	}
	return out
}

// isPathMatch reports whether call is a path.match(...) call.
func isPathMatch(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "match" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "path"
}

// capturesIn returns the capture names bound no matter how one matcher subtree
// matches. A node binds its own capture, if it is one, plus the captures bound
// on every one of its children. Several children are alternatives, since the
// subject takes exactly one of them, so only a capture common to all of them is
// guaranteed; the children are intersected, and the node's own capture is added
// on top. A single child is the degenerate case of that intersection, which is
// why a plain chain accumulates every capture along it.
//
// This models both the prefix-merged tree, where sibling literals branch the
// continuation, and the root() of paths that share no first segment: root()
// binds no capture itself and intersects its alternatives, falling straight out
// of the general rule. A greedy_except exclusion binds nothing and is a deny
// test, not a continuation, so childArgs never returns it and a capture written
// there never counts.
func capturesIn(node ast.Node) map[string]bool {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return map[string]bool{}
	}
	out := map[string]bool{}
	if name, ok := captureName(call); ok {
		out[name] = true
	}
	children := childArgs(call)
	if len(children) == 0 {
		return out
	}
	inter := capturesIn(children[0])
	for _, child := range children[1:] {
		inter = intersectSets(inter, capturesIn(child))
	}
	return unionSets(out, inter)
}

// childArgs returns the continuation children of a matcher constructor call,
// the arguments that are themselves matcher subtrees. Several children are
// alternatives. It returns nil for a terminal matcher (greedy, slash) and for
// the exclusion arguments of greedy_except, which are deny tests rather than
// continuations, so a capture inside one never counts as bound.
func childArgs(call *ast.CallExpr) []ast.Expr {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}
	switch id.Name {
	case "literal", "capture", "glob_encoded", "glob_without":
		if len(call.Args) > 1 {
			return call.Args[1:]
		}
	case "capture_encoded", "encoded_literal":
		if len(call.Args) > 2 {
			return call.Args[2:]
		}
	case "glob", "root":
		return call.Args
	}
	return nil
}

// isGreedyExcept reports whether call is a greedy_except(...) call.
func isGreedyExcept(call *ast.CallExpr) bool {
	return isIdentCall(call, "greedy_except")
}

// isIdentCall reports whether call is a bare name(...) call, such as root(...)
// or greedy_except(...). It does not match a selector call like path.match(...).
func isIdentCall(call *ast.CallExpr, name string) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}

// validateLiterals rejects an empty or illegally-encoded value in a
// literal("...") or encoded_literal("...", ...) call whose value is a string
// constant. It is the load-time half of the literal check, so a typo such as
// literal("") or a stray byte fails when the rule compiles rather than per
// request. A dynamic value cannot be checked until evaluation, where the
// constructor still rejects it. literal and encoded_literal both take the value
// as the first argument, but hold it to different rules: a literal value must
// be a legal segment after splitting on "/", an encoded_literal value the
// decoded form with no "%".
func validateLiterals(expr string) error {
	parsed, err := goparser.ParseExpr(expr)
	if err != nil {
		return nil
	}
	var bad error
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		check := literalValueCheck(call)
		if check == nil {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if err := check(s); err != nil {
			bad = err
		}
		return true
	})
	return trace.Wrap(bad)
}

// literalValueCheck returns the value validator for a literal or encoded_literal
// call, or nil for any other call, so validateLiterals holds each to its own
// rules.
func literalValueCheck(call *ast.CallExpr) func(string) error {
	switch {
	case isIdentCall(call, "literal"):
		return validateLiteral
	case isIdentCall(call, "encoded_literal"):
		return validateEncodedLiteralValue
	default:
		return nil
	}
}

// validateRoot rejects a root() call anywhere but as the matcher argument of a
// path.match. root() is non-consuming and only sound at the top of a tree,
// where it OR-s several first segments; nested it would silently behave as a
// mid-tree alternation, which the existing sibling children already express.
// Rejecting it at load keeps root() to its one legal spot.
func validateRoot(expr string) error {
	parsed, err := goparser.ParseExpr(expr)
	if err != nil {
		return nil
	}
	// First pass: every root() that sits as path.match's first argument is the
	// one legal placement, recorded by AST node identity.
	allowed := map[ast.Node]bool{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isPathMatch(call) || len(call.Args) == 0 {
			return true
		}
		if root, ok := call.Args[0].(*ast.CallExpr); ok && isIdentCall(root, "root") {
			allowed[root] = true
		}
		return true
	})
	// Second pass: any other root() call is illegal.
	var bad bool
	ast.Inspect(parsed, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isIdentCall(call, "root") && !allowed[call] {
			bad = true
		}
		return true
	})
	if bad {
		return trace.BadParameter("root() is only valid as the matcher argument of path.match")
	}
	return nil
}

// validateEncodedSets rejects an encoded-char set literal that names any char
// other than the separator "/". The encoded-char matchers glob_encoded and
// capture_encoded, and the allow_encoded option, each take a set(...) of
// the chars they admit, and only "/" is supported today. Checking at load turns
// a per-request evaluation error into a clear compile failure. A set built from
// a non-literal cannot be checked here and is backstopped by the constructor at
// evaluation.
func validateEncodedSets(expr string) error {
	parsed, err := goparser.ParseExpr(expr)
	if err != nil {
		return nil
	}
	var bad error
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var setArg ast.Expr
		switch {
		case isIdentCall(call, "glob_encoded") && len(call.Args) >= 1:
			setArg = call.Args[0]
		case isIdentCall(call, "allow_encoded") && len(call.Args) >= 1:
			setArg = call.Args[0]
		case isIdentCall(call, "capture_encoded") && len(call.Args) >= 2:
			setArg = call.Args[1]
		case isIdentCall(call, "encoded_literal") && len(call.Args) >= 2:
			setArg = call.Args[1]
		default:
			return true
		}
		chars, ok := setLiterals(setArg)
		if !ok {
			return true
		}
		if err := validateEncodedChars(chars); err != nil {
			bad = err
		}
		return true
	})
	return trace.Wrap(bad)
}

// setLiterals returns the string-literal members of a set(...) call. It reports
// false when the node is not a set call or any member is not a string literal,
// since a dynamic set cannot be checked at load.
func setLiterals(node ast.Expr) ([]string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || !isIdentCall(call, "set") {
		return nil, false
	}
	out := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return nil, false
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// validateExclusions rejects a capture inside a greedy_except matcher. An
// exclusion is a deny test that binds nothing, so a capture there can never be
// read through vars.<name>; rejecting it at load turns a silent no-op into a
// clear error. The constructor enforces the same rule as a backstop, but the
// constructor runs only at evaluation, so the load-time check is what surfaces
// the mistake when the rule is compiled.
func validateExclusions(expr string) error {
	parsed, err := goparser.ParseExpr(expr)
	if err != nil {
		return nil
	}
	var bad string
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isGreedyExcept(call) {
			return true
		}
		for _, arg := range call.Args {
			ast.Inspect(arg, func(m ast.Node) bool {
				if inner, ok := m.(*ast.CallExpr); ok {
					if name, ok := captureName(inner); ok {
						bad = name
					}
				}
				return true
			})
		}
		return true
	})
	if bad != "" {
		return trace.BadParameter(
			"a greedy_except matcher cannot bind capture %q: an exclusion is a deny test and binds nothing",
			bad)
	}
	return nil
}

// referencedVars returns the names read through the vars.<name> namespace.
func referencedVars(root ast.Node) []string {
	var out []string
	ast.Inspect(root, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "vars" {
				out = append(out, sel.Sel.Name)
			}
		}
		return true
	})
	return out
}

// captureName returns the bound name of a capture("name", ...) or
// capture_encoded("name", set(...), ...) call. It reports false for any other
// call, and for a capture call whose first argument is not a string literal,
// since a dynamic name cannot be checked at load.
func captureName(call *ast.CallExpr) (string, bool) {
	id, ok := call.Fun.(*ast.Ident)
	if !ok || (id.Name != "capture" && id.Name != "capture_encoded") || len(call.Args) == 0 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return name, true
}
