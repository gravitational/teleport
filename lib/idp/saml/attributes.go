// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package saml

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// MappableAttribtues defines user details that can be mapped in
// SAML assertion.
type MappableAttributes struct {
	Username       string
	Roles          []string
	Traits         map[string][]string
	AccessRequests []string
}

// EvaluationEnv is the predicate expression evaluation context defined for MappableAttributes.
type EvaluationEnv struct {
	UserTraits     expression.Dict
	UserRoles      expression.Set
	Username       expression.Set
	AccessRequests expression.Set
}

// AttributeMappingParser is an initialized predicate parser.
var AttributeMappingParser = newAttributeMappingParser()

func newAttributeMappingParser() *typical.Parser[EvaluationEnv, any] {
	spec := expression.DefaultParserSpec[EvaluationEnv]()

	spec.Variables = map[string]typical.Variable{
		UID: typical.DynamicVariable[EvaluationEnv](func(env EvaluationEnv) (expression.Set, error) {
			return env.Username, nil
		}),
		Username: typical.DynamicVariable[EvaluationEnv](func(env EvaluationEnv) (expression.Set, error) {
			return env.Username, nil
		}),
		EduPersonAffiliation: typical.DynamicVariable[EvaluationEnv](func(env EvaluationEnv) (expression.Set, error) {
			return env.UserRoles, nil
		}),
		UserRoles: typical.DynamicVariable[EvaluationEnv](func(env EvaluationEnv) (expression.Set, error) {
			return env.UserRoles, nil
		}),
		UserTraits: typical.DynamicMap[EvaluationEnv, expression.Set](func(env EvaluationEnv) (expression.Dict, error) {
			return env.UserTraits, nil
		}),
		SessionAccessRequests: typical.DynamicVariable[EvaluationEnv](func(env EvaluationEnv) (expression.Set, error) {
			return env.AccessRequests, nil
		}),
	}

	attributeParser, err := typical.NewParser[EvaluationEnv, any](spec)
	if err != nil {
		panic(trace.Wrap(err, "creating attribute mapping parser (this is a bug)"))
	}
	return attributeParser
}
