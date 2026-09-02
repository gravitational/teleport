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
	"go/ast"
	"go/token"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/set"
)

// validateVars validates that no vars.<name> in the given parsed
// expression is read before a capture("<name>") bound it. The following
// examples all fail validation:
//
//	vars.p == "x"
//	vars.p == "x" && path.match(capture("p", greedy()))
//	(path.match(capture("p")) || path.match(slash())) && vars.p == "x"
//
// The third example fails for the request path "/", which matches only
// the slash() branch and binds nothing.
func validateVars(parsed ast.Expr) error {
	_, err := checkVars(parsed, set.New[string]())
	return trace.Wrap(err)
}

// checkVars checks that no vars.<name> in node is read before a
// capture("<name>") bound it.
//
// The given node is the root of the parsed app_resources_expressions entry
// on the first call and one of its subexpressions on each recursive call.
// bound contains the capture names bound before node evaluates. checkVars
// returns the capture names guaranteed bound after node evaluated to true,
// which are passed as bound for the next subexpression.
func checkVars(node ast.Expr, bound set.Set[string]) (set.Set[string], error) {
	switch n := ast.Unparen(node).(type) {
	case *ast.BinaryExpr:
		boundByX, err := checkVars(n.X, bound) // left operand
		if err != nil {
			return nil, trace.Wrap(err)
		}
		switch n.Op {
		case token.LAND:
			boundByY, err := checkVars(n.Y, union(bound, boundByX)) // right operand
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return union(boundByX, boundByY), nil
		case token.LOR:
			boundByY, err := checkVars(n.Y, bound)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return intersect(boundByX, boundByY), nil
		default: // EQ and NEQ operators (==, !=) guarantee nothing
			_, err := checkVars(n.Y, bound)
			return set.New[string](), trace.Wrap(err)
		}
	case *ast.UnaryExpr:
		_, err := checkVars(n.X, bound)
		return set.New[string](), trace.Wrap(err)
	case *ast.CallExpr:
		if isPathMatch(n) {
			// A matcher tree cannot contain a vars.<name> read.
			return checkCaptureBindings(n.Args[0], set.New[string]())
		}
		// The audit wrappers are transparent. They return their third
		// argument's value and guarantee its captures.
		if isAuditCall(n) {
			for _, arg := range n.Args[:2] {
				if _, err := checkVars(arg, bound); err != nil {
					return nil, trace.Wrap(err)
				}
			}
			return checkVars(n.Args[2], bound)
		}
		// Arguments evaluate before the call binds anything.
		for _, arg := range n.Args {
			if _, err := checkVars(arg, bound); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return set.New[string](), nil
	case *ast.SelectorExpr:
		if id, ok := n.X.(*ast.Ident); ok && id.Name == "vars" {
			if !bound.Contains(n.Sel.Name) {
				return nil, trace.BadParameter("vars.%s is read before capture %q is guaranteed bound; bind it in a path.match on every branch", n.Sel.Name, n.Sel.Name)
			}
		}
		return set.New[string](), nil
	case *ast.IndexExpr:
		if _, err := checkVars(n.X, bound); err != nil {
			return nil, trace.Wrap(err)
		}
		_, err := checkVars(n.Index, bound)
		return set.New[string](), trace.Wrap(err)
	default:
		return set.New[string](), nil
	}
}

func union(sets ...set.Set[string]) set.Set[string] {
	out := set.New[string]()
	out.Union(sets...)
	return out
}

func intersect(a, b set.Set[string]) set.Set[string] {
	out := a.Clone()
	out.Intersection(b)
	return out
}

// validateNoCaptureOverwrites validates that no path.match rebinds a
// capture name an earlier path.match may have bound, to avoid subtle
// overwrite errors with && and ||. The following examples both fail
// validation:
//
//	path.match(capture("p")) && path.match(capture("p"))
//	(path.match(capture("p")) && vars.p == "x") || path.match(glob(capture("p")))
func validateNoCaptureOverwrites(parsed ast.Expr) error {
	_, err := checkNoOverwrites(parsed, set.New[string]())
	return trace.Wrap(err)
}

// bindings contains the capture names one expression may bind.
type bindings struct {
	onTrue  set.Set[string] // names possibly bound when evaluated to true
	onFalse set.Set[string] // names possibly bound when evaluated to false
}

// negated swaps onTrue and onFalse, for a negation operand.
func (m bindings) negated() bindings {
	return bindings{onTrue: m.onFalse, onFalse: m.onTrue}
}

// both returns the names bound under either outcome.
func (m bindings) both() set.Set[string] {
	return union(m.onTrue, m.onFalse)
}

// checkNoOverwrites checks that no path.match in node rebinds a capture
// name in possiblyBound and returns the names node itself may bind.
func checkNoOverwrites(node ast.Expr, possiblyBound set.Set[string]) (bindings, error) {
	none := bindings{onTrue: set.New[string](), onFalse: set.New[string]()}
	switch n := ast.Unparen(node).(type) {
	case *ast.BinaryExpr:
		x, err := checkNoOverwrites(n.X, possiblyBound) // left operand
		if err != nil {
			return none, trace.Wrap(err)
		}
		switch n.Op {
		case token.LAND:
			y, err := checkNoOverwrites(n.Y, union(possiblyBound, x.onTrue))
			if err != nil {
				return none, trace.Wrap(err)
			}
			// &&: false when x failed, or x passed and y failed
			return bindings{
				onTrue:  union(x.onTrue, y.onTrue),
				onFalse: union(x.both(), y.onFalse),
			}, nil
		case token.LOR:
			y, err := checkNoOverwrites(n.Y, union(possiblyBound, x.onFalse))
			if err != nil {
				return none, trace.Wrap(err)
			}
			// ||: true when x passed, or x failed and y passed
			return bindings{
				onTrue:  union(x.both(), y.onTrue),
				onFalse: union(x.onFalse, y.onFalse),
			}, nil
		default: // A comparison evaluates both operands whatever they bind.
			y, err := checkNoOverwrites(n.Y, union(possiblyBound, x.both()))
			if err != nil {
				return none, trace.Wrap(err)
			}
			all := union(x.both(), y.both())
			return bindings{onTrue: all, onFalse: all}, nil
		}
	case *ast.UnaryExpr:
		x, err := checkNoOverwrites(n.X, possiblyBound)
		if err != nil {
			return none, trace.Wrap(err)
		}
		if n.Op == token.NOT {
			return x.negated(), nil
		}
		all := x.both()
		return bindings{onTrue: all, onFalse: all}, nil
	case *ast.CallExpr:
		if isPathMatch(n) {
			bound := set.New[string]()
			for _, name := range allCaptureNames(n.Args[0]) {
				if possiblyBound.Contains(name) {
					return none, trace.BadParameter("path.match can evaluate with capture %q already bound by an earlier path.match and would replace its value; use a distinct capture name", name)
				}
				bound.Add(name)
			}
			return bindings{onTrue: bound, onFalse: set.New[string]()}, nil
		}
		// The audit wrappers are transparent. They return their third
		// argument's value and bind what it binds.
		if isAuditCall(n) {
			return checkNoOverwrites(n.Args[2], possiblyBound)
		}
		// Arguments evaluate left to right whatever they bind.
		boundByArgs := set.New[string]()
		for _, arg := range n.Args {
			a, err := checkNoOverwrites(arg, possiblyBound)
			if err != nil {
				return none, trace.Wrap(err)
			}
			possiblyBound = union(possiblyBound, a.both())
			boundByArgs = union(boundByArgs, a.both())
		}
		return bindings{onTrue: boundByArgs, onFalse: boundByArgs}, nil
	case *ast.IndexExpr:
		x, err := checkNoOverwrites(n.X, possiblyBound)
		if err != nil {
			return none, trace.Wrap(err)
		}
		index, err := checkNoOverwrites(n.Index, union(possiblyBound, x.both()))
		if err != nil {
			return none, trace.Wrap(err)
		}
		all := union(x.both(), index.both())
		return bindings{onTrue: all, onFalse: all}, nil
	default:
		return none, nil
	}
}

// checkCaptureBindings checks that no capture name in node repeats on a
// root-to-leaf path. The names in bound count as already on the branch.
// It returns the names guaranteed bound however the subtree matches.
func checkCaptureBindings(node ast.Expr, bound set.Set[string]) (set.Set[string], error) {
	guaranteed := set.New[string]()
	call, ok := ast.Unparen(node).(*ast.CallExpr)
	if !ok {
		return guaranteed, nil
	}
	if name, ok := captureName(call); ok {
		if bound.Contains(name) {
			return guaranteed, trace.BadParameter("capture %q appears more than once on one root-to-leaf path; use a distinct name", name)
		}
		bound.Add(name)
		defer bound.Remove(name)
		guaranteed.Add(name)
	}
	children := childArgs(call)
	if len(children) == 0 {
		return guaranteed, nil
	}
	var fromChildren set.Set[string]
	for _, child := range children {
		childGuaranteed, err := checkCaptureBindings(child, bound)
		if err != nil {
			return guaranteed, trace.Wrap(err)
		}
		if fromChildren == nil {
			fromChildren = childGuaranteed
		} else {
			fromChildren.Intersection(childGuaranteed)
		}
	}
	if !isIdentCall(call, "optional") {
		guaranteed.Union(fromChildren)
	}
	return guaranteed, nil
}

// allCaptureNames returns all capture names in one matcher subtree, sorted.
func allCaptureNames(node ast.Expr) []string {
	seen := set.New[string]()
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if name, ok := captureName(call); ok {
				seen.Add(name)
			}
		}
		return true
	})
	names := seen.Elements()
	slices.Sort(names)
	return names
}

// childArgs returns the continuation children of a matcher constructor
// call. It returns nil for the terminals greedy and slash.
func childArgs(call *ast.CallExpr) []ast.Expr {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}
	switch id.Name {
	case "literal", "capture":
		if len(call.Args) > 1 {
			return call.Args[1:]
		}
	case "glob", "root", "optional":
		return call.Args
	}
	return nil
}

// validateMatcherConstructors validates the arguments and placement of the
// matcher node constructors inside path.match. The checks are tighter
// than the default typical parser for string values, for example
// literal("foo") is valid and literal(vars.x) is not.
func validateMatcherConstructors(parsed ast.Expr) error {
	topRoots := set.New[*ast.CallExpr]()
	ast.Inspect(parsed, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isPathMatch(call) {
			if top, ok := ast.Unparen(call.Args[0]).(*ast.CallExpr); ok && isIdentCall(top, "root") {
				topRoots.Add(top)
			}
		}
		return true
	})
	var err error
	ast.Inspect(parsed, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case isIdentCall(call, "literal") && len(call.Args) > 0:
			if s, ok := stringLiteral(call.Args[0]); ok {
				_, err = checkLiteral(s)
			} else {
				err = trace.BadParameter("the value argument of literal must be a string literal")
			}
		case isIdentCall(call, "capture") && len(call.Args) > 0:
			if s, ok := stringLiteral(call.Args[0]); !ok {
				err = trace.BadParameter("the name argument of capture must be a string literal")
			} else if !captureNameRE.MatchString(s) {
				err = trace.BadParameter("capture name %q must be a letter or underscore followed by letters, digits, or underscores", elide(s))
			}
		case isIdentCall(call, "optional") && len(call.Args) == 0:
			err = trace.BadParameter("optional requires at least one child subtree")
		case isIdentCall(call, "root"):
			if !topRoots.Contains(call) {
				err = trace.BadParameter("root must be the top node of a path.match matcher tree")
			} else if len(call.Args) == 0 {
				err = trace.BadParameter("root requires at least one alternative")
			}
		}
		return err == nil
	})
	return trace.Wrap(err)
}

// captureName returns the string-literal name "foo" of a valid
// capture("foo", ...) call, false otherwise.
func captureName(call *ast.CallExpr) (string, bool) {
	if !isIdentCall(call, "capture") || len(call.Args) == 0 {
		return "", false
	}
	return stringLiteral(call.Args[0])
}
