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
	"reflect"

	"github.com/gravitational/trace"
)

// output/result: generate a boolean output
func (s *solver) evalEq(node *eqEq) (any, error) {
	lValue, err := s.evalNode(node.left)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rValue, err := s.evalNode(node.right)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return reflect.DeepEqual(lValue, rValue), nil
}

func (s *solver) evalNot(node *eqNot) (any, error) {
	value, err := s.evalNode(node.inner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch value := value.(type) {
	case *bool:
		return !*value, nil
	default:
		return nil, trace.BadParameter("cannot apply NOT to %T", value)
	}
}

func (s *solver) evalOr(node *eqOr) (any, error) {
	return nil, nil
}

func (s *solver) evalAnd(node *eqAnd) (any, error) {
	return nil, nil
}

func (s *solver) evalXor(node *eqXor) (any, error) {
	return nil, nil
}

func (s *solver) evalLt(node *eqLt) (any, error) {
	return nil, nil
}

func (s *solver) evalLeq(node *eqLeq) (any, error) {
	return nil, nil
}

func (s *solver) evalIndex(node *eqIndex) (any, error) {
	return nil, nil
}

func (s *solver) evalSelector(node *eqSelector) (any, error) {
	return nil, nil
}

func (s *solver) evalIdent(node *eqIdent) (any, error) {
	return nil, nil
}

func (s *solver) evalConcat(node *eqConcat) (any, error) {
	return nil, nil
}

func (s *solver) evalSplit(node *eqSplit) (any, error) {
	return nil, nil
}

func (s *solver) evalArray(node *eqArray) (any, error) {
	return nil, nil
}

func (s *solver) evalUpper(node *eqUpper) (any, error) {
	return nil, nil
}

func (s *solver) evalLower(node *eqLower) (any, error) {
	return nil, nil
}

func (s *solver) evalAppend(node *eqAppend) (any, error) {
	return nil, nil
}

func (s *solver) evalContains(node *eqContains) (any, error) {
	return nil, nil
}

func (s *solver) evalReplace(node *eqReplace) (any, error) {
	return nil, nil
}

func (s *solver) evalMatches(node *eqMatches) (any, error) {
	return nil, nil
}

func (s *solver) evalMatchesAny(node *eqMatchesAny) (any, error) {
	return nil, nil
}

func (s *solver) evalLen(node *eqLen) (any, error) {
	return nil, nil
}

func (s *solver) evalGetOrEmpty(node *eqGetOrEmpty) (any, error) {
	return nil, nil
}

func (s *solver) evalMapInsert(node *eqMapInsert) (any, error) {
	return nil, nil
}

func (s *solver) evalMapRemove(node *eqMapRemove) (any, error) {
	return nil, nil
}
