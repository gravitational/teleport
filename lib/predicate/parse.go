// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package predicate

import (
	goast "go/ast"
	goparse "go/parser"
	gotok "go/token"

	"github.com/gravitational/trace"
)

func evaluatePredicate(s string) (bool, map[string]any) {
	clause, err := parse(s)
	if err != nil {
		panic(err)
	}

	state := newState(clause)
	sat := dpll(state)
	if sat {
		m := make(map[string]any)
		for _, a := range state.assignments {
			m[a.key] = a.value
		}

		return true, m
	}

	return false, nil
}

func parse(s string) (node, error) {
	astNode, err := goparse.ParseExpr(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	node, err := lower(astNode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return node, nil
}

func lower(astNode goast.Expr) (node, error) {
	switch n := astNode.(type) {
	case *goast.ParenExpr:
		return lower(n.X)
	case *goast.BinaryExpr:
		return lowerBinary(n)
	case *goast.UnaryExpr:
		return lowerUnary(n)
	case *goast.Ident:
		return lowerIdent(n)
	default:
		return nil, trace.BadParameter("unsupported expression type %T", n)
	}
}

func lowerBinary(n *goast.BinaryExpr) (node, error) {
	switch n.Op {
	case gotok.AND:
		left, err := lower(n.X)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		right, err := lower(n.Y)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &nodeAnd{left: left, right: right}, nil
	case gotok.OR:
		left, err := lower(n.X)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		right, err := lower(n.Y)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &nodeOr{left: left, right: right}, nil
	default:
		return nil, trace.BadParameter("unsupported binary operator %v", n.Op)
	}
}

func lowerUnary(n *goast.UnaryExpr) (node, error) {
	switch n.Op {
	case gotok.NOT:
		left, err := lower(n.X)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &nodeNot{left: left}, nil
	default:
		return nil, trace.BadParameter("unsupported unary operator %v", n.Op)
	}
}

func lowerIdent(n *goast.Ident) (node, error) {
	switch n.Name {
	case "true":
		return &nodeLiteral{true}, nil
	case "false":
		return &nodeLiteral{false}, nil
	default:
		return &nodeIdentifier{key: n.Name}, nil
	}
}
