/*
Copyright 2024 Gravitational, Inc.

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

package accessmonitoring

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// AccessRequestExpressionEnv holds user details that can be mapped in an
// access request condition assertion.
type AccessRequestExpressionEnv struct {
	Roles              []string
	SuggestedReviewers []string
	Annotations        map[string][]string
	User               string
	RequestReason      string
	CreationTime       time.Time
	Expiry             time.Time

	// UserTraits includes arbitrary user traits dynamically provided by the
	// access monitoring rule handler.
	UserTraits map[string][]string
}

type accessRequestExpression typical.Expression[AccessRequestExpressionEnv, any]

func parseAccessRequestExpression(expr string) (accessRequestExpression, error) {
	parser, err := newRequestConditionParser()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parsedExpr, err := parser.Parse(expr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing access monitoring rule condition expression")
	}
	return parsedExpr, nil
}

// NewAccessRequestConditionParser returns a new parser for access request
// condition expressions.
func NewAccessRequestConditionParser() (*typical.Parser[AccessRequestExpressionEnv, any], error) {
	return newRequestConditionParser()
}

func newRequestConditionParser() (*typical.Parser[AccessRequestExpressionEnv, any], error) {
	typicalEnvVar := map[string]typical.Variable{
		"true":  true,
		"false": false,
		"access_request.spec.roles": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (expression.Set, error) {
			return expression.NewSet(env.Roles...), nil
		}),
		"access_request.spec.suggested_reviewers": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (expression.Set, error) {
			return expression.NewSet(env.SuggestedReviewers...), nil
		}),
		"access_request.spec.system_annotations": typical.DynamicMap(func(env AccessRequestExpressionEnv) (expression.Dict, error) {
			return expression.DictFromStringSliceMap(env.Annotations), nil
		}),
		"access_request.spec.user": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (string, error) {
			return env.User, nil
		}),
		"access_request.spec.request_reason": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (string, error) {
			return env.RequestReason, nil
		}),
		"access_request.spec.creation_time": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (time.Time, error) {
			return env.CreationTime, nil
		}),
		"access_request.spec.expiry": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (time.Time, error) {
			return env.Expiry, nil
		}),

		"user.traits": typical.DynamicMap(func(env AccessRequestExpressionEnv) (expression.Dict, error) {
			return expression.DictFromStringSliceMap(env.UserTraits), nil
		}),
	}
	defParserSpec := expression.DefaultParserSpec[AccessRequestExpressionEnv]()
	defParserSpec.Variables = typicalEnvVar

	requestConditionParser, err := typical.NewParser[AccessRequestExpressionEnv, any](defParserSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return requestConditionParser, nil
}

// EvaluateCondition evaluates the condition expression given the provided environment.
// A true value indicates that the AMR is a match for the given access request
// environment. Returns false, if evaluated to a non-boolean value.
func EvaluateCondition(expr string, env AccessRequestExpressionEnv) (bool, error) {
	parsedExpr, err := parseAccessRequestExpression(expr)
	if err != nil {
		return false, trace.Wrap(err)
	}

	match, err := parsedExpr.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err, "evaluating access monitoring rule condition expression %q", expr)
	}
	matched, ok := match.(bool)
	return ok && matched, nil
}
