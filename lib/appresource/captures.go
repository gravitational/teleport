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
	"maps"
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
// call. It returns nil for the terminals greedy, slash, and greedy_except.
func childArgs(call *ast.CallExpr) []ast.Expr {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}
	switch id.Name {
	case "literal", "capture", "glob_without":
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
			} else {
				err = validateCaptureName(s)
			}
		case isIdentCall(call, "glob_without") && len(call.Args) > 0:
			err = checkExcludes(call.Args[0])
		case isIdentCall(call, "greedy_except"):
			err = checkGreedyExcept(call.Args)
		case isIdentCall(call, "allow_unsafe_encoded") && len(call.Args) > 1:
			err = checkEncodedOption(call.Args[0], call.Args[1])
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

// checkGreedyExcept checks that greedy_except has one or more exclusions and
// that no exclusion contains a capture, an optional, or greedy_except.
func checkGreedyExcept(args []ast.Expr) error {
	if len(args) == 0 {
		return trace.BadParameter("greedy_except requires at least one exclusion")
	}
	for _, arg := range args {
		if containsCall(arg, "capture") {
			return trace.BadParameter("an exclusion cannot contain a capture")
		}
		if containsCall(arg, "optional") {
			return trace.BadParameter("an exclusion cannot contain optional")
		}
		if containsCall(arg, "greedy_except") {
			return trace.BadParameter("an exclusion cannot contain greedy_except")
		}
	}
	return nil
}

// containsCall returns true when expr contains a call of the named function.
func containsCall(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && isIdentCall(call, name) {
			found = true
		}
		return !found
	})
	return found
}

// checkExcludes checks that the excludes argument of glob_without is a
// set(...) of string literals that validateExclude accepts.
func checkExcludes(arg ast.Expr) error {
	const want = "the excludes argument of glob_without must be a set of string literals"
	setCall, ok := ast.Unparen(arg).(*ast.CallExpr)
	if !ok || !isIdentCall(setCall, "set") {
		return trace.BadParameter(want)
	}
	for _, elem := range setCall.Args {
		s, ok := stringLiteral(elem)
		if !ok {
			return trace.BadParameter(want)
		}
		if err := validateExclude(s); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// checkEncodedOption checks the name and characters arguments of an
// allow_unsafe_encoded call.
func checkEncodedOption(nameArg, charsArg ast.Expr) error {
	name, ok := stringLiteral(nameArg)
	if !ok {
		return trace.BadParameter("the name argument of allow_unsafe_encoded must be a string literal")
	}
	if err := validateCaptureName(name); err != nil {
		return trace.Wrap(err)
	}
	chars, ok := stringLiteral(charsArg)
	if !ok {
		return trace.BadParameter("the characters argument of allow_unsafe_encoded must be a string literal")
	}
	return trace.Wrap(validateAllowedEncoded(chars))
}

// validateEncodedOptions validates the allow_unsafe_encoded options of every
// path.match in parsed.
func validateEncodedOptions(parsed ast.Expr) error {
	var err error
	var first map[string]string
	ast.Inspect(parsed, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || !isPathMatch(call) {
			return true
		}
		var options map[string]string
		options, err = encodedOptions(call)
		if err != nil {
			return false
		}
		names := allCaptureNames(call.Args[0])
		for _, name := range slices.Sorted(maps.Keys(options)) {
			if !slices.Contains(names, name) {
				err = trace.BadParameter("allow_unsafe_encoded(%q, ...) has no capture(%q) in its path.match tree", name, name)
				return false
			}
		}
		if first == nil {
			first = options
			return true
		}
		if !maps.Equal(first, options) {
			err = trace.BadParameter("every path.match in one entry needs the same allow_unsafe_encoded options")
			return false
		}
		return true
	})
	return trace.Wrap(err)
}

// encodedOptions returns the allow_unsafe_encoded options of one path.match
// call as a map from capture name to allowed encoded characters.
func encodedOptions(call *ast.CallExpr) (map[string]string, error) {
	options := map[string]string{}
	for _, arg := range call.Args[1:] {
		option, ok := ast.Unparen(arg).(*ast.CallExpr)
		if !ok || !isIdentCall(option, "allow_unsafe_encoded") || len(option.Args) != 2 {
			continue
		}
		name, _ := stringLiteral(option.Args[0])
		chars, _ := stringLiteral(option.Args[1])
		if _, dup := options[name]; dup {
			return nil, trace.BadParameter("allow_unsafe_encoded(%q, ...) is given twice", name)
		}
		options[name] = chars
	}
	return options, nil
}

// captureName returns the string-literal name "foo" of a valid
// capture("foo", ...) call, false otherwise.
func captureName(call *ast.CallExpr) (string, bool) {
	if !isIdentCall(call, "capture") || len(call.Args) == 0 {
		return "", false
	}
	return stringLiteral(call.Args[0])
}
