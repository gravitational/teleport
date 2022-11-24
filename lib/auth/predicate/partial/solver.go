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

package predicate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/aclements/go-z3/z3"
	"github.com/gravitational/trace"
)

type Resolver func(string) any

func PartialSolve(predicate string, resolveIdentifier Resolver, querying string) (z3.Value, error) {
	ast, err := parser.ParseExpr(predicate)
	if err != nil {
		return nil, err
	}

	config := z3.NewContextConfig()
	ctx := &ctx{idents: make(map[string]z3.Value)}
	ctx.def = z3.NewContext(config)
	ctx.solver = z3.NewSolver(ctx.def)

	_, err = lower(ctx, ast)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

type ctx struct {
	def    *z3.Context
	solver *z3.Solver
	idents map[string]z3.Value
}

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
		xi := x.(z3.Int)
		yi := y.(z3.Int)
		return xi.Eq(yi), nil
	case token.LSS:
		xi := x.(z3.Int)
		yi := y.(z3.Int)
		return xi.LT(yi), nil
	case token.LEQ:
		xi := x.(z3.Int)
		yi := y.(z3.Int)
		return xi.LE(yi), nil
	case token.GTR:
		xi := x.(z3.Int)
		yi := y.(z3.Int)
		return xi.GT(yi), nil
	case token.GEQ:
		xi := x.(z3.Int)
		yi := y.(z3.Int)
		return xi.GE(yi), nil
	case token.LAND:
		xb := x.(z3.Bool)
		yb := y.(z3.Bool)
		return xb.And(yb), nil
	case token.LOR:
		xb := x.(z3.Bool)
		yb := y.(z3.Bool)
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

		return x.(z3.Bool).Not(), nil
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
	default:
		return nil, trace.NotImplemented("basic lit kind %v unsupported", node.Kind)
	}
}

func lowerIndexExpr(ctx *ctx, node *ast.IndexExpr) (z3.Value, error) {
	return nil, nil
}

func lowerSelectorExpr(ctx *ctx, node *ast.SelectorExpr) (z3.Value, error) {
	return nil, nil
}

func lowerIdent(ctx *ctx, node *ast.Ident) (z3.Value, error) {
	switch node.Name {
	case "true":
		return ctx.def.FromBool(true), nil
	case "false":
		return ctx.def.FromBool(true), nil
	default:
		if ident, ok := ctx.idents[node.Name]; ok {
			return ident, nil
		}

		// todo: query resolver
		// todo: figure out type here
		ident := ctx.def.IntConst(node.Name)
		ctx.idents[node.Name] = ident
		return ident, nil
	}
}

func lowerCallExpr(ctx *ctx, node *ast.CallExpr) (z3.Value, error) {
	return nil, nil
}
