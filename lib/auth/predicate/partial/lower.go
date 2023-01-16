/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package partial

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/aclements/go-z3/z3"
	"github.com/gravitational/trace"
)

// Resolver resolves an identifier to a value. Returns nil if the identifier is not found and is unknown.
type Resolver func([]string) any

// Type represents a type of a value within a predicate expression.
type Type int

const (
	// TypeInt is an Go int type.
	TypeInt Type = iota

	// TypeBool is a Go bool type.
	TypeBool

	// TypeString is a Go string type.
	TypeString
)

type ctx struct {
	// def is the Z3 context instance
	def *z3.Context
	// solver is the Z3 solver instance
	solver *z3.Solver
	// idents is a map of identifiers to their Z3 values
	idents map[string]z3.Value
	// the type of the unknown identifier, needed for type checking during lowering
	unkTy z3.Sort
	// resolveIdentifier is a function which resolves an identifier to a value
	resolveIdentifier Resolver
}

// resolve an identifier to a value, if the identifier is not found, the resolver is called to resolve it
func (ctx *ctx) resolve(fields []string) (z3.Value, error) {
	full := strings.Join(fields, ".")
	if v, ok := ctx.idents[full]; ok {
		return v, nil
	}

	if val := ctx.resolveIdentifier(fields); val != nil {
		var ident z3.Value
		switch val := val.(type) {
		case bool:
			ident = ctx.def.FromBool(val)
		case int:
			ident = ctx.def.FromInt(int64(val), ctx.def.IntSort())
		case string:
			ident = ctx.def.FromString(val)
		default:
			return nil, trace.BadParameter("unsupported type %T", val)
		}

		ctx.idents[full] = ident
		return ident, nil
	}

	val := ctx.def.Const(full, ctx.unkTy)
	ctx.idents[full] = val
	return val, nil
}

// lower lowers a Go AST into a Z3 AST.
// Lowering is a term commonly used within compiler development/interpreters etc to describe
// the process of transforming a high level construct into a lower level construct, usually with more primitive operators
// and more data/constraints. In this case, lowering is the process of converting a Go AST into a set
// of Z3 variables and constraints. Variables are used to describe the input and output of each primitive
// operation such as arithmetic or comparisons and constraints are used to define a relationship between them.
// For the uninitiated, such a constraint set is the usual form that a SMT solver sees problems in.
//
// To keep it simple, lower operates by recursively traversing the Go AST depth-first
// and at each AST node, defining one or more variables and possibly one or more constraints between those variables.
// To organize this, each lowering operation returns the variable that represents the result of the relation between
// the input and output variables. Note that this function doesn't actually do any problem computation;
// it merely converts it into a simpler form that Z3 can grasp.
func lower(ctx *ctx, node ast.Expr) (z3.Value, error) {
	switch n := node.(type) {
	case *ast.ParenExpr:
		return lower(ctx, n.X)
	case *ast.BinaryExpr:
		return lowerBinary(ctx, n)
	case *ast.UnaryExpr:
		return lowerUnary(ctx, n)
	case *ast.BasicLit:
		return lowerBasicLit(ctx, n)
	case *ast.IndexExpr:
		return lowerIndexExpr(ctx, n)
	case *ast.SelectorExpr:
		return lowerSelectorExpr(ctx, n)
	case *ast.Ident:
		return lowerIdent(ctx, n)
	case *ast.CallExpr:
		return lowerCallExpr(ctx, n)
	default:
		return nil, trace.NotImplemented("node type %T unsupported", n)
	}
}

func pickType(ctx *ctx, x, y z3.Value) (z3.Kind, error) {
	xk := x.Sort().Kind()
	yk := y.Sort().Kind()
	if xk != yk {
		return 0, trace.BadParameter("type mismatch %v != %v", xk, yk)
	}

	return xk, nil
}

func lowerBinary(ctx *ctx, node *ast.BinaryExpr) (z3.Value, error) {
	x, err := lower(ctx, node.X)
	if err != nil {
		return nil, err
	}

	y, err := lower(ctx, node.Y)
	if err != nil {
		return nil, err
	}

	switch node.Op {
	case token.EQL:
		ty, err := pickType(ctx, x, y)
		if err != nil {
			return nil, err
		}

		switch ty {
		case z3.KindInt:
			return x.(z3.Int).Eq(y.(z3.Int)), nil
		case z3.KindBool:
			return x.(z3.Bool).Eq(y.(z3.Bool)), nil
		case z3.KindSequence:
			return x.(z3.String).Eq(y.(z3.String)), nil
		default:
			return nil, trace.BadParameter("type %v does not support equals", ty)
		}
	case token.LSS:
		xi, ok1 := x.(z3.Int)
		yi, ok2 := y.(z3.Int)
		if !ok1 || !ok2 {
			return nil, trace.BadParameter("type invalid, must be int %v, %v", x.Sort().Kind(), y.Sort().Kind())
		}

		return xi.LT(yi), nil
	case token.LEQ:
		xi, ok1 := x.(z3.Int)
		yi, ok2 := y.(z3.Int)
		if !ok1 || !ok2 {
			return nil, trace.BadParameter("type invalid, must be int %v, %v", x.Sort().Kind(), y.Sort().Kind())
		}

		return xi.LE(yi), nil
	case token.GTR:
		xi, ok1 := x.(z3.Int)
		yi, ok2 := y.(z3.Int)
		if !ok1 || !ok2 {
			return nil, trace.BadParameter("type invalid, must be int %v, %v", x.Sort().Kind(), y.Sort().Kind())
		}

		return xi.GT(yi), nil
	case token.GEQ:
		xi, ok1 := x.(z3.Int)
		yi, ok2 := y.(z3.Int)
		if !ok1 || !ok2 {
			return nil, trace.BadParameter("type invalid, must be int %v, %v", x.Sort().Kind(), y.Sort().Kind())
		}

		return xi.GE(yi), nil
	case token.LAND:
		xb, ok1 := x.(z3.Bool)
		yb, ok2 := y.(z3.Bool)
		if !ok1 || !ok2 {
			return nil, trace.BadParameter("type invalid, must be bool %v, %v", x.Sort().Kind(), y.Sort().Kind())
		}

		return xb.And(yb), nil
	case token.LOR:
		xb, ok1 := x.(z3.Bool)
		yb, ok2 := y.(z3.Bool)
		if !ok1 || !ok2 {
			return nil, trace.BadParameter("type invalid, must be bool %v, %v", x.Sort().Kind(), y.Sort().Kind())
		}

		return xb.Or(yb), nil
	default:
		return nil, trace.NotImplemented("unary op %v unsupported", node.Op)
	}
}

func lowerUnary(ctx *ctx, node *ast.UnaryExpr) (z3.Value, error) {
	switch node.Op {
	case token.NOT:
		x, err := lower(ctx, node.X)
		if err != nil {
			return nil, err
		}

		xb, ok := x.(z3.Bool)
		if !ok {
			return nil, trace.BadParameter("expected bool, got %T", x)
		}

		return xb.Not(), nil
	default:
		return nil, trace.NotImplemented("unary op %v unsupported", node.Op)
	}
}

func lowerBasicLit(ctx *ctx, node *ast.BasicLit) (z3.Value, error) {
	switch {
	case node.Kind == token.INT:
		value, err := strconv.Atoi(node.Value)
		if err != nil {
			return nil, err
		}

		return ctx.def.FromInt(int64(value), ctx.def.IntSort()), nil
	case node.Kind == token.STRING:
		return ctx.def.FromString(node.Value[1 : len(node.Value)-1]), nil
	default:
		return nil, trace.NotImplemented("basic lit kind %v unsupported", node.Kind)
	}
}

func lowerIndexExpr(ctx *ctx, node *ast.IndexExpr) (z3.Value, error) {
	// TODO(joel): impl maps
	return nil, trace.BadParameter("maps are not supported")
}

func lowerSelectorExpr(ctx *ctx, node *ast.SelectorExpr) (z3.Value, error) {
	fields, err := evaluateSelector(node, []string{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ctx.resolve(fields)
}

func evaluateSelector(sel *ast.SelectorExpr, fields []string) ([]string, error) {
	fields = append([]string{sel.Sel.Name}, fields...)
	switch l := sel.X.(type) {
	case *ast.SelectorExpr:
		return evaluateSelector(l, fields)
	case *ast.Ident:
		fields = append([]string{l.Name}, fields...)
		return fields, nil
	default:
		return nil, trace.BadParameter("unsupported selector type: %T", l)
	}
}

func lowerIdent(ctx *ctx, node *ast.Ident) (z3.Value, error) {
	switch node.Name {
	case "true":
		return ctx.def.FromBool(true), nil
	case "false":
		return ctx.def.FromBool(false), nil
	default:
		return ctx.resolve([]string{node.Name})
	}
}

func lowerCallExpr(ctx *ctx, node *ast.CallExpr) (z3.Value, error) {
	args := make([]z3.Value, len(node.Args))
	for i, arg := range node.Args {
		v, err := lower(ctx, arg)
		if err != nil {
			return nil, err
		}

		args[i] = v
	}

	fn, ok := node.Fun.(*ast.Ident)
	if !ok {
		return nil, trace.BadParameter("expected basic lit function name, got %T", node.Fun)
	}

	var fnGen func(*z3.Context) (z3.FuncDecl, error)
	switch fn.Name {
	case "upper":
		fnGen = builtinUpper
	case "lower":
		fnGen = builtinLower
	case "split":
		fnGen = builtinSplit
	case "contains":
		fnGen = builtinStringListContains
	case "append":
		fnGen = builtinStringListAppend
	case "array":
		fnGen = builtinStringListArray(len(args))
	case "replace":
		fnGen = builtinReplace
	case "len":
		fnGen = builtinLenString
	case "string_list_len":
		fnGen = builtinLenStringList
	case "regex":
		fnGen = builtinRegex
	case "matches":
		fnGen = builtinMatches
	case "contains_regex":
		fnGen = builtinContainsRegex
	case "first":
		fnGen = builtinFirst
	default:
		return nil, trace.BadParameter("unknown function %v", fn.Name)
	}

	decl, err := fnGen(ctx.def)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return decl.Apply(args...), nil
}
