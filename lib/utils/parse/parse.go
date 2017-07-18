/*
Copyright 2017 Gravitational, Inc.

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

package parse

import (
	"go/ast"
	"go/parser"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// IsRoleVariable checks if the passed in string matches the variable pattern
// {{external.foo}} or {{internal.bar}}. If it does, it returns the variable
// prefix and the variable name. In the previous example this would be
// "external" or "internal" for the variable prefix and "foo" or "bar" for the
// variable name. If no variable pattern is found, trace.NotFound is returned.
func IsRoleVariable(variable string) (string, string, error) {
	// time whitespace around string if it exists
	variable = strings.TrimSpace(variable)

	// strip {{ and }} from the start and end of the variable
	if !strings.HasPrefix(variable, "{{") || !strings.HasSuffix(variable, "}}") {
		return "", "", trace.NotFound("no variable found: %v", variable)
	}
	variable = variable[2 : len(variable)-2]

	// parse and get the ast of the expression
	expr, err := parser.ParseExpr(variable)
	if err != nil {
		return "", "", trace.NotFound("no variable found: %v", variable)
	}

	// walk the ast tree and gather the variable parts
	variableParts, err := walk(expr)
	if err != nil {
		return "", "", trace.NotFound("no variable found: %v", variable)
	}

	// the variable must have two parts the prefix and the variable name itself
	if len(variableParts) != 2 {
		return "", "", trace.NotFound("no variable found: %v", variable)
	}

	return variableParts[0], variableParts[1], nil
}

// walk will walk the ast tree and gather all the variable parts into a slice and return it.
func walk(node ast.Node) ([]string, error) {
	var l []string

	switch n := node.(type) {
	case *ast.IndexExpr:
		ret, err := walk(n.X)
		if err != nil {
			return nil, err
		}
		l = append(l, ret...)

		ret, err = walk(n.Index)
		if err != nil {
			return nil, err
		}
		l = append(l, ret...)
	case *ast.SelectorExpr:
		ret, err := walk(n.X)
		if err != nil {
			return nil, err
		}
		l = append(l, ret...)

		ret, err = walk(n.Sel)
		if err != nil {
			return nil, err
		}
		l = append(l, ret...)
	case *ast.Ident:
		return []string{n.Name}, nil
	case *ast.BasicLit:
		value, err := strconv.Unquote(n.Value)
		if err != nil {
			return nil, err
		}
		return []string{value}, nil
	default:
		return nil, trace.BadParameter("unknown node type: %T", n)
	}

	return l, nil
}
