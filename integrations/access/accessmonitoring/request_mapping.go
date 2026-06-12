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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// accessRequestExpressionEnv holds user details that can be mapped in an
// access request condition assertion.
type accessRequestExpressionEnv struct {
	Roles              []string
	SuggestedReviewers []string
	Annotations        map[string][]string
	User               string
	RequestReason      string
	CreationTime       time.Time
	Expiry             time.Time
}

type accessRequestExpression typical.Expression[accessRequestExpressionEnv, any]

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

func newRequestConditionParser() (*typical.Parser[accessRequestExpressionEnv, any], error) {
	typicalEnvVar := map[string]typical.Variable{
		"true":  true,
		"false": false,
		"access_request.spec.roles": typical.DynamicVariable[accessRequestExpressionEnv](func(env accessRequestExpressionEnv) (expression.Set, error) {
			return expression.NewSet(env.Roles...), nil
		}),
		"access_request.spec.suggested_reviewers": typical.DynamicVariable[accessRequestExpressionEnv](func(env accessRequestExpressionEnv) (expression.Set, error) {
			return expression.NewSet(env.SuggestedReviewers...), nil
		}),
		"access_request.spec.system_annotations": typical.DynamicMap[accessRequestExpressionEnv, expression.Set](func(env accessRequestExpressionEnv) (expression.Dict, error) {
			return expression.DictFromStringSliceMap(env.Annotations), nil
		}),
		"access_request.spec.user": typical.DynamicVariable[accessRequestExpressionEnv](func(env accessRequestExpressionEnv) (string, error) {
			return env.User, nil
		}),
		"access_request.spec.request_reason": typical.DynamicVariable[accessRequestExpressionEnv](func(env accessRequestExpressionEnv) (string, error) {
			return env.RequestReason, nil
		}),
		"access_request.spec.creation_time": typical.DynamicVariable[accessRequestExpressionEnv](func(env accessRequestExpressionEnv) (time.Time, error) {
			return env.CreationTime, nil
		}),
		"access_request.spec.expiry": typical.DynamicVariable[accessRequestExpressionEnv](func(env accessRequestExpressionEnv) (time.Time, error) {
			return env.Expiry, nil
		}),
	}
	defParserSpec := expression.DefaultParserSpec[accessRequestExpressionEnv]()
	defParserSpec.Variables = typicalEnvVar

	requestConditionParser, err := typical.NewParser[accessRequestExpressionEnv, any](defParserSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return requestConditionParser, nil
}

func MatchAccessRequest(expr string, req types.AccessRequest) (bool, error) {
	parsedExpr, err := parseAccessRequestExpression(expr)
	if err != nil {
		return false, trace.Wrap(err)
	}

	match, err := parsedExpr.Evaluate(accessRequestExpressionEnv{
		Roles:              req.GetRoles(),
		SuggestedReviewers: req.GetSuggestedReviewers(),
		Annotations:        req.GetSystemAnnotations(),
		User:               req.GetUser(),
		RequestReason:      req.GetRequestReason(),
		CreationTime:       req.GetCreationTime(),
		Expiry:             req.Expiry(),
	})
	if err != nil {
		return false, trace.Wrap(err, "evaluating access monitoring rule condition expression %q", expr)
	}
	if matched, ok := match.(bool); ok && matched {
		return true, nil
	}
	return false, nil
}
