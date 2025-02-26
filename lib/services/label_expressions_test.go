/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				"operator (==) not supported for type: []string",
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
			desc: "email.local",
			expr: `contains(email.local(user.spec.traits["email"]), labels["owner"]) &&
			       contains(email.local(user.spec.traits["full email"]), labels["owner"])`,
			resourceLabels: map[string]string{
				"owner": "test",
			},
			userTraits: map[string][]string{
				"email":      {"test@example.com"},
				"full email": {"Test <test@example.com>"},
			},
			expectMatch: true,
		},
		{
			desc: "email.local error",
			expr: `contains(email.local(user.spec.traits["email"]), labels["owner"])`,
			resourceLabels: map[string]string{
				"owner": "test",
			},
			userTraits: map[string][]string{
				"email": {"test"},
			},
			expectEvaluateError: []string{`failed to parse "email.local" argument`},
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
		{
			desc: "labels_matching name",
			expr: `contains(labels_matching("^project-(name|label)$"), "skunkworks")`,
			resourceLabels: map[string]string{
				"project-name":  "skunkworks",
				"project-label": "secret",
			},
			expectMatch: true,
		},
		{
			desc: "labels_matching label",
			expr: `contains(labels_matching("^project-(name|label)$"), "skunkworks")`,
			resourceLabels: map[string]string{
				"project-name":  "secret",
				"project-label": "skunkworks",
			},
			expectMatch: true,
		},
		{
			desc: "labels_matching glob",
			expr: `contains(labels_matching("project-*"), "skunkworks")`,
			resourceLabels: map[string]string{
				"project-name":  "skunkworks",
				"project-label": "secret",
			},
			expectMatch: true,
		},
		{
			desc: "labels_matching no match",
			expr: `contains(labels_matching("project-*"), "skunkworks")`,
			resourceLabels: map[string]string{
				"project": "skunkworks",
			},
			expectMatch: false,
		},
		{
			desc: "contains_any match",
			expr: `contains_any(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{
				"projects": {"parser", "skunkworks", "algorithms"},
			},
			resourceLabels: map[string]string{
				"project-name":  "skunkworks",
				"project-label": "secret",
			},
			expectMatch: true,
		},
		{
			desc: "contains_any no match",
			expr: `contains_any(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{
				"projects": {"parser", "algorithms"},
			},
			resourceLabels: map[string]string{
				"project-name":  "skunkworks",
				"project-label": "secret",
			},
			expectMatch: false,
		},
		{
			desc:       "contains_any empty first arg",
			expr:       `contains_any(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{},
			resourceLabels: map[string]string{
				"project-name":  "skunkworks",
				"project-label": "secret",
			},
			expectMatch: false,
		},
		{
			desc: "contains_any empty second arg",
			expr: `contains_any(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{
				"projects": {"parser", "algorithms"},
			},
			resourceLabels: map[string]string{
				"team": "security",
			},
			expectMatch: false,
		},
		{
			desc: "contains_all match",
			expr: `contains_all(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{
				"projects": {"parser", "skunkworks", "algorithms"},
			},
			resourceLabels: map[string]string{
				"project-primary":   "parser",
				"project-secondary": "algorithms",
			},
			expectMatch: true,
		},
		{
			desc: "contains_all no match",
			expr: `contains_all(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{
				"projects": {"parser", "skunkworks", "algorithms"},
			},
			resourceLabels: map[string]string{
				"project-primary":   "parser",
				"project-secondary": "algorithms",
				"project-label":     "secret",
			},
			expectMatch: false,
		},
		{
			desc:       "contains_all empty first arg",
			expr:       `contains_all(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{},
			resourceLabels: map[string]string{
				"project-primary":   "parser",
				"project-secondary": "algorithms",
				"project-label":     "secret",
			},
			expectMatch: false,
		},
		{
			desc: "contains_all empty second arg",
			expr: `contains_all(user.spec.traits["projects"], labels_matching("project-*"))`,
			userTraits: map[string][]string{
				"projects": {"parser", "skunkworks", "algorithms"},
			},
			// This resource seems unrelated to the contains_all expression. To
			// avoid footguns, contains_all intentionally returns false when the
			// second argument is empty.
			resourceLabels: map[string]string{
				"team": "security",
			},
			expectMatch: false,
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
				resourceLabelGetter: mapLabelGetter(tc.resourceLabels),
				userTraits:          tc.userTraits,
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
