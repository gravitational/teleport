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

	"github.com/gravitational/teleport/api/types"
)

func specStruct(t *testing.T, doc string) *types.Struct {
	t.Helper()

	// we could just use jsonpb to parse it directly, but depguard and forbidigo
	// complain, so instead we have this dubious roundabout implementation
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &m))

	s, err := types.NewStructFromGoValues(m)
	require.NoError(t, err)

	return s
}

func TestValidateFieldRulesContainsAnyRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        *types.Struct
		expect      bool
		expectError require.ErrorAssertionFunc
	}{
		{
			name: "simple",
			spec: specStruct(t, `{
				"email": "123456789012-compute@developer.gserviceaccount.com"
			}`),
			expect:      true,
			expectError: require.NoError,
		},
		{
			name: "only fields",
			spec: specStruct(t, `{
				"google": {
					"compute_engine": {}
				},
				"custom": {}
			}`),
			expect:      false,
			expectError: require.NoError,
		},
		{
			name: "one nested check",
			spec: specStruct(t, `{
				"google": {
					"compute_engine": {
						"instance_name": "hello-world"
					}
				},
				"custom": {}
			}`),
			// empty checks that only assert a field's existence are allowed,
			// just not sufficient on their own. at least one value needs to be
			// compared in some rule.
			expect:      true,
			expectError: require.NoError,
		},
		{
			name: "contains a list rule",
			spec: specStruct(t, `{
				"google": {
					"compute_engine": {
						"instance_name": ["hello-world"]
					}
				}
			}`),
			expect: false,
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "list fields cannot be used")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := validateFieldRulesContainsAnyRule(tt.spec)
			tt.expectError(t, err)
			require.Equal(t, tt.expect, ret)
		})
	}
}

func TestEvaluateFieldRules(t *testing.T) {
	t.Parallel()

	parsedClaims := claims(t, exampleGCPClaimsString)

	tests := []struct {
		name        string
		spec        *types.Struct
		expectError require.ErrorAssertionFunc
	}{
		{
			name: "success",
			spec: specStruct(t, `{
				"email_verified": true,
				"google": {
					"compute_engine": {
						"instance_name": "hello-world",
						"project_number": 123456123456,
						"zone": "us-central1-a"
					}
				},
				"custom": {
					"float": 123.456
				},
				"azp": "1234567890"
			}`),
			expectError: require.NoError,
		},
		{
			name: "string failure",
			spec: specStruct(t, `{
				"email_verified": true,
				"google": {
					"compute_engine": {
						"instance_name": "foo",
						"project_number": 123456123456,
						"zone": "us-central1-a"
					}
				},
				"custom": {
					"float": 123.456
				},
				"azp": "1234567890"
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "incorrect value in claim: google.compute_engine.instance_name")
			},
		},
		{
			name: "bool failure",
			spec: specStruct(t, `{
				"email_verified": false,
				"google": {
					"compute_engine": {
						"instance_name": "hello-world",
						"project_number": 123456123456,
						"zone": "us-central1-a"
					}
				},
				"custom": {
					"float": 123.456
				},
				"azp": "1234567890"
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "incorrect value in claim: email_verified must be false")
			},
		},
		{
			name: "int failure",
			spec: specStruct(t, `{
				"email_verified": true,
				"google": {
					"compute_engine": {
						"instance_name": "hello-world",
						"project_number": 1234561234567,
						"zone": "us-central1-a"
					}
				},
				"custom": {
					"float": 123.456
				},
				"azp": "1234567890"
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				// Renders as engineering notation but is collision-safe below
				// 2^53, which GCP project numbers are with some margin.
				require.ErrorContains(t, err, "incorrect value in claim: google.compute_engine.project_number must be")
			},
		},
		{
			name: "float failure",
			spec: specStruct(t, `{
				"custom": {
					"float": 123.457
				}
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "incorrect value in claim: custom.float must be 123.457")
			},
		},
		{
			name: "can assert struct existence, successful pass",
			spec: specStruct(t, `{
				"google": {}
			}`),
			expectError: require.NoError,
		},
		{
			name: "can assert struct existence, successful fail",
			spec: specStruct(t, `{
				"foo": {}
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "claims missing expected key: foo")
			},
		},
		{
			name: "can assert non-existence, successful pass",
			spec: specStruct(t, `{
				"foo": null
			}`),
			expectError: require.NoError,
		},
		{
			name: "can assert non-existence, successful fail",
			spec: specStruct(t, `{
				"google": {
					"compute_engine": null
				}
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "google.compute_engine must be null or unset")
			},
		},
		{
			name: "cannot assert lists",
			spec: specStruct(t, `{
				"custom": {
					"list": ["a", "b", "c"]
				}
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "list comparison in custom.list is not supported")
			},
		},
		{
			name: "rejects large integers in claim",
			spec: specStruct(t, `{
				"custom": {
					"largeInt": 123
				}
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "claim contains an integer too large to be safely compared")
			},
		},
		{
			name: "rejects large integers in spec",
			spec: specStruct(t, `{
				"custom": {
					"float": 9007199254740993
				}
			}`),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "field rule cannot safely compare integers of this size: custom.float")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evaluateFieldRules(tt.spec, parsedClaims)
			tt.expectError(t, err)
		})
	}
}
