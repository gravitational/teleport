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

import "github.com/gravitational/trace"

// reading material: https://www.cs.upc.edu/~oliveras/dpllt.pdf

const (
	trueValue   = "true"
	falseValue  = "false"
	intValue    = "int"
	stringValue = "string"
)

type constraint interface {
}

type constraintSet struct {
	key        string
	ty         string
	contraints []constraint
}

type solver struct {
	constrainted []constraintSet
}

func (s *solver) evalNode(node astNode) (any, error) {
	switch node := node.(type) {
	case *eqEq:
		return s.evalEq(node)
	case *eqNot:
		return s.evalNot(node)
	case *eqOr:
		return s.evalOr(node)
	case *eqAnd:
		return s.evalAnd(node)
	case *eqXor:
		return s.evalXor(node)
	case *eqLt:
		return s.evalLt(node)
	case *eqLeq:
		return s.evalLeq(node)
	case *eqIndex:
		return s.evalIndex(node)
	case *eqSelector:
		return s.evalSelector(node)
	case *eqIdent:
		return s.evalIdent(node)
	case *eqConcat:
		return s.evalConcat(node)
	case *eqSplit:
		return s.evalSplit(node)
	case *eqArray:
		return s.evalArray(node)
	case *eqUpper:
		return s.evalUpper(node)
	case *eqLower:
		return s.evalLower(node)
	case *eqAppend:
		return s.evalAppend(node)
	case *eqContains:
		return s.evalContains(node)
	case *eqReplace:
		return s.evalReplace(node)
	case *eqMatches:
		return s.evalMatches(node)
	case *eqMatchesAny:
		return s.evalMatchesAny(node)
	default:
		return nil, trace.BadParameter("unknown node type %T", node)
	}
}
