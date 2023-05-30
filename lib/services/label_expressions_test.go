// Copyright 2023 Gravitational, Inc
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

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type labelExpressionTestResource struct {
	labels map[string]string
}

func (t labelExpressionTestResource) GetLabel(key string) (string, bool) {
	label, ok := t.labels[key]
	return label, ok
}

func TestLabelExpressions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc                string
		expr                string
		resourceLabels      map[string]string
		userTraits          map[string][]string
		expectParseError    []string
		expectEvaluateError []string
		expectMatch         bool
	}{
		{
			desc: "label equality",
			expr: `labels["env"] == "staging"`,
			resourceLabels: map[string]string{
				"env": "staging",
			},
			expectMatch: true,
		},
		{
			desc: "label inequality",
			expr: `labels["env"] != "production"`,
			resourceLabels: map[string]string{
				"env": "production",
			},
			expectMatch: false,
		},
		{
			desc: "wrong type",
			expr: `user.spec.traits["allow-env"] == "staging"`,
			expectParseError: []string{
				"parsing lhs of (==) operator",
				"expected type string, got expression returning type ([]string)",
			},
		},
		{
			desc: "contains match",
			expr: `contains(user.spec.traits["allow-env"], labels["env"])`,
			userTraits: map[string][]string{
				"allow-env": {"dev", "staging"},
			},
			resourceLabels: map[string]string{
				"env": "staging",
			},
			expectMatch: true,
		},
		{
			desc: "contains not match",
			expr: `contains(user.spec.traits["allow-env"], labels["env"])`,
			userTraits: map[string][]string{
				"allow-env": {"dev", "staging"},
			},
			resourceLabels: map[string]string{
				"env": "production",
			},
			expectMatch: false,
		},
		{
			desc: "contains wrong type",
			expr: `contains(labels, "env")`,
			resourceLabels: map[string]string{
				"env": "staging",
			},
			expectParseError: []string{
				"parsing first argument to (contains)",
				"expected type []string",
			},
		},
		{
			desc: "boolean logic",
			expr: `contains(user.spec.traits["allow-env"], labels["env"]) &&
				   contains(user.spec.traits["groups"], labels["group"]) ||
				   contains(user.spec.traits["groups"], "super-admin")`,
			userTraits: map[string][]string{
				"allow-env": {"dev", "staging", "production"},
				"groups":    {"devs", "security"},
			},
			resourceLabels: map[string]string{
				"env":   "staging",
				"group": "devs",
			},
			expectMatch: true,
		},
		{
			desc: "regexp.match",
			expr: `regexp.match(labels["owner"], "dev-*") &&
                  !regexp.match(labels["env"], "^prod.*$")`,
			resourceLabels: map[string]string{
				"owner": "dev-123",
				"env":   "staging",
			},
			expectMatch: true,
		},
		{
			desc: "regexp.replace",
			expr: `
			contains(
			    // call with list
				regexp.replace(user.spec.traits["groups"], "^team-(.*)$", "$1"),
				labels["team"]) &&
			contains(
			    // call with single string
				regexp.replace(labels["env"], "^(.*)$", "env-$1"),
				"env-staging")`,
			resourceLabels: map[string]string{
				"team": "security",
				"env":  "staging",
			},
			userTraits: map[string][]string{
				"groups": {"team-security"},
			},
			expectMatch: true,
		},
		{
			desc: "string helpers",
			expr: `
			contains(strings.lower(user.spec.traits["logins"]), "name") &&
			contains(strings.upper(labels["env"]), "STAGING")`,
			resourceLabels: map[string]string{
				"env": "staging",
			},
			userTraits: map[string][]string{
				"logins": {"test", "Name"},
			},
			expectMatch: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			parsedExpr, err := parseLabelExpression(tc.expr)
			for _, msg := range tc.expectParseError {
				assert.ErrorContains(t, err, msg, "parse error doesn't include expected message")
			}
			if len(tc.expectParseError) > 0 {
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			env := labelExpressionEnv{
				resourceLabelGetter: labelExpressionTestResource{
					labels: tc.resourceLabels,
				},
				userTraits: tc.userTraits,
			}

			match, err := parsedExpr.Evaluate(env)
			for _, msg := range tc.expectEvaluateError {
				assert.ErrorContains(t, err, msg, "evaluate error doesn't include expected message")
			}
			if len(tc.expectEvaluateError) > 0 {
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			require.Equal(t, tc.expectMatch, match)
		})
	}
}
