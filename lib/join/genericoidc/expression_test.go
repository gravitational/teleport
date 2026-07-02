/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package genericoidc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// claims is a helper that parses a JSON document inline
func claims(t *testing.T, doc string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &m))
	return m
}

func TestEvaluateExpression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		claims      map[string]any
		expression  string
		expect      bool
		expectError require.ErrorAssertionFunc
	}{
		{
			name: "simple success",
			claims: claims(t, `{
				"organization_name": "acme-corp"
			}`),
			expression:  `claims.organization_name == "acme-corp"`,
			expect:      true,
			expectError: require.NoError,
		},
		{
			name: "nested success",
			claims: claims(t, `{
				"organization": {
					"id": "abc123",
					"http://acme-corp.com/": {
						"name": "acme-corp"
					}
				}
			}`),
			// note: predicate can't accept f["a"].foo - once you use a index
			// expression, all following terms must remain index expressions
			expression:  `claims.organization["http://acme-corp.com/"]["name"] == "acme-corp" && claims.organization.id == "abc123"`,
			expect:      true,
			expectError: require.NoError,
		},
		{
			name: "invalid variable",
			claims: claims(t, `{
				"organization_id": "acme-corp"
			}`),
			expression: `claims.organization_name == "acme-corp"`,
			expect:     false,
			expectError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "field not found: organization_name")
			},
		},
		{
			name: "invalid nested variable first level",
			claims: claims(t, `{
				"organization_id": "acme-corp"
			}`),
			expression: `claims.foo.bar == "acme-corp"`,
			expect:     false,
			expectError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "field not found: foo")
			},
		},
		{
			name: "invalid nested variable second level",
			claims: claims(t, `{
				"foo": {
					"organization_id": "acme-corp"
				}
			}`),
			expression: `claims.foo.bar == "acme-corp"`,
			expect:     false,
			expectError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "field not found: foo.bar")
			},
		},
		{
			name: "floats not supported",
			claims: claims(t, `{
				"number": 1
			}`),
			expression: `claims.number == 1.0`,
			expect:     false,
			expectError: func(t require.TestingT, err error, i ...interface{}) {
				// typical is bad and it should feel bad
				require.ErrorContains(t, err, "operator (==) not supported for type: float64")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluateExpression(tt.expression, &Environment{
				Claims: tt.claims,
			})
			tt.expectError(t, err)
			require.Equal(t, tt.expect, result)
		})
	}
}
