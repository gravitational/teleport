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
	goast "go/ast"
	goparse "go/parser"
	gotok "go/token"
	"strconv"

	"github.com/gravitational/trace"
)

func parsePredicate(predicate string) (astNode, error) {
	ast, err := goparse.ParseExpr(predicate)
	if err != nil {
		return nil, err
	}

	return lower(ast)
}

func lower(node goast.Node) (astNode, error) {
	switch node := node.(type) {
	case *goast.ParenExpr:
		return lower(node.X)
	case *goast.UnaryExpr:
		return lowerUnary(node)
	case *goast.BinaryExpr:
		return lowerBinary(node)
	case *goast.BasicLit:
		return lowerBasicLit(node)
	case *goast.IndexExpr:
		return lowerIndex(node)
	case *goast.SelectorExpr:
		return lowerSelector(node)
	case *goast.Ident:
		return lowerIdent(node)
	case *goast.CallExpr:
		return lowerCall(node)
	default:
		return nil, trace.BadParameter("unsupported node type %T", node)
	}
}

func lowerUnary(node *goast.UnaryExpr) (astNode, error) {
	inner, err := lower(node.X)
	if err != nil {
		return nil, err
	}

	switch node.Op {
	case gotok.NOT:
		return &eqNot{inner}, nil
	default:
		return nil, trace.BadParameter("unsupported unary operation %T", node.Op)
	}
}

func lowerBinary(node *goast.BinaryExpr) (astNode, error) {
	left, err := lower(node.X)
	if err != nil {
		return nil, err
	}

	right, err := lower(node.Y)
	if err != nil {
		return nil, err
	}

	switch node.Op {
	case gotok.EQL:
		return &eqEq{left, right}, nil
	case gotok.OR:
		return &eqOr{left, right}, nil
	case gotok.AND:
		return &eqAnd{left, right}, nil
	case gotok.XOR:
		return &eqXor{left, right}, nil
	case gotok.LSS:
		return &eqLt{left, right}, nil
	case gotok.GTR:
		return &eqLt{right, left}, nil
	case gotok.LEQ:
		return &eqLeq{left, right}, nil
	case gotok.GEQ:
		return &eqLeq{right, left}, nil
	default:
		return nil, trace.BadParameter("unsupported binary operation %T", node.Op)
	}
}

func lowerBasicLit(node *goast.BasicLit) (astNode, error) {
	switch node.Kind {
	case gotok.INT:
		value, err := strconv.Atoi(node.Value)
		if err != nil {
			return nil, err
		}

		return &eqInt{value}, nil
	case gotok.STRING:
		return &eqString{node.Value}, nil
	default:
		return nil, trace.BadParameter("unsupported literal type %T", node.Kind)
	}
}

func lowerIndex(node *goast.IndexExpr) (astNode, error) {
	inner, err := lower(node.X)
	if err != nil {
		return nil, err
	}

	index, err := lower(node.Index)
	if err != nil {
		return nil, err
	}

	return &eqIndex{inner, index}, nil
}

func lowerSelector(node *goast.SelectorExpr) (astNode, error) {
	inner, err := lower(node.X)
	if err != nil {
		return nil, err
	}

	return &eqSelector{inner, node.Sel.Name}, nil
}

func lowerIdent(node *goast.Ident) (astNode, error) {
	switch node.Name {
	case "true":
		return &eqBool{true}, nil
	case "false":
		return &eqBool{false}, nil
	default:
		return &eqIdent{name: node.Name}, nil
	}
}

func lowerCall(node *goast.CallExpr) (astNode, error) {
	fn := ""
	switch target := node.Fun.(type) {
	case *goast.Ident:
		fn = target.Name
	default:
		return nil, trace.BadParameter("unsupported call target %T", node.Fun)
	}

	args := make([]astNode, len(node.Args))
	for i, arg := range node.Args {
		lowered, err := lower(arg)
		if err != nil {
			return nil, err
		}

		args[i] = lowered
	}

	has_args := func(n int) error {
		if len(args) != n {
			return trace.BadParameter("expected %d arguments, got %d", n, len(node.Args))
		}

		return nil
	}

	switch fn {
	case "concat":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqConcat{args[0], args[1]}, nil
	case "split":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqSplit{args[0], args[1]}, nil
	case "array":
		return &eqArray{args}, nil
	case "upper":
		if err := has_args(1); err != nil {
			return nil, err
		}

		return &eqUpper{args[0]}, nil
	case "lower":
		if err := has_args(1); err != nil {
			return nil, err
		}

		return &eqLower{args[0]}, nil
	case "append":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqAppend{args[0], args[1]}, nil
	case "contains":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqContains{args[0], args[1]}, nil
	case "replace":
		if err := has_args(3); err != nil {
			return nil, err
		}

		return &eqReplace{args[0], args[1], args[2]}, nil
	case "matches":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqMatches{args[0], args[1]}, nil
	case "matches_any":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqMatchesAny{args[0], args[1]}, nil
	case "len":
		if err := has_args(1); err != nil {
			return nil, err
		}

		return &eqLen{args[0]}, nil
	case "getOrEmpty":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqGetOrEmpty{args[0], args[1]}, nil
	case "mapInsert":
		if err := has_args(3); err != nil {
			return nil, err
		}

		return &eqMapInsert{args[0], args[1], args[2]}, nil
	case "mapRemove":
		if err := has_args(2); err != nil {
			return nil, err
		}

		return &eqMapRemove{args[0], args[1]}, nil
	default:
		return nil, trace.NotImplemented("unsupported function %q", fn)
	}
}
