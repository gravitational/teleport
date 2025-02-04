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

	Plugin PluginExpressionEnv
}

// PluginExpressionEnv holds plugin specific condition variables.
type PluginExpressionEnv struct {
	// Name specifies the plugin name.
	Name string
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

		// Plugin provided condition variables.
		"plugin.spec.name": typical.DynamicVariable(func(env AccessRequestExpressionEnv) (string, error) {
			return env.Plugin.Name, nil
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

// IsConditionMatched returns the evaluated condition expression value.
// A true value indicates that the condition is a match for the access request env.
func IsConditionMatched(condition string, env AccessRequestExpressionEnv) (bool, error) {
	parsedExpr, err := parseAccessRequestExpression(condition)
	if err != nil {
		return false, trace.Wrap(err)
	}
	match, err := parsedExpr.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err, "evaluating access monitoring rule condition expression %q", condition)
	}
	matched, ok := match.(bool)
	return ok && matched, nil
}
