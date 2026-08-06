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

package genericoidc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

const exampleGCPClaimsString = `{
  "aud": "example.teleport.sh",
  "azp": "1234567890",
  "email": "123456789012-compute@developer.gserviceaccount.com",
  "email_verified": true,
  "exp": 1781580107,
  "google": {
    "compute_engine": {
      "instance_creation_timestamp": 1666452409,
      "instance_id": "12345678901234567",
      "instance_name": "hello-world",
      "project_id": "example-123456",
      "project_number": 123456123456,
      "zone": "us-central1-a"
    }
  },
  "custom": {
	"float": 123.456,
	"list": ["a", "b", "c"],
	"largeInt": 9007199254740992
  },
  "iat": 1781576507,
  "iss": "https://accounts.google.com",
  "sub": "1234567890"
}`

// claims is a helper that parses a JSON document inline as IDTokenClaims
func idTokenClaims(t *testing.T, doc string) *IDTokenClaims {
	t.Helper()
	var parsed IDTokenClaims
	require.NoError(t, json.Unmarshal([]byte(doc), &parsed))

	// hacky, but we expect zitadel to set this
	parsed.Claims = claims(t, doc)

	return &parsed
}

func eqCondition(attribute, value string) *types.ProvisionTokenSpecV2GenericOIDC_Condition {
	return &types.ProvisionTokenSpecV2GenericOIDC_Condition{
		Attribute: attribute,
		Eq: &types.ProvisionTokenSpecV2GenericOIDC_ConditionEq{
			Value: value,
		},
	}
}

func notEqCondition(attribute, value string) *types.ProvisionTokenSpecV2GenericOIDC_Condition {
	return &types.ProvisionTokenSpecV2GenericOIDC_Condition{
		Attribute: attribute,
		NotEq: &types.ProvisionTokenSpecV2GenericOIDC_ConditionNotEq{
			Value: value,
		},
	}
}

func inCondition(attribute string, values ...string) *types.ProvisionTokenSpecV2GenericOIDC_Condition {
	return &types.ProvisionTokenSpecV2GenericOIDC_Condition{
		Attribute: attribute,
		In: &types.ProvisionTokenSpecV2GenericOIDC_ConditionIn{
			Values: values,
		},
	}
}

func notInCondition(attribute string, values ...string) *types.ProvisionTokenSpecV2GenericOIDC_Condition {
	return &types.ProvisionTokenSpecV2GenericOIDC_Condition{
		Attribute: attribute,
		NotIn: &types.ProvisionTokenSpecV2GenericOIDC_ConditionNotIn{
			Values: values,
		},
	}
}

func conditions(conditions ...*types.ProvisionTokenSpecV2GenericOIDC_Condition) []*types.ProvisionTokenSpecV2GenericOIDC_Condition {
	// silly wrapper to save characters
	return conditions
}

func TestEvaluateAllowConditions(t *testing.T) {
	t.Parallel()

	parsedClaims := idTokenClaims(t, exampleGCPClaimsString)

	tests := []struct {
		name        string
		conditions  []*types.ProvisionTokenSpecV2GenericOIDC_Condition
		expectError require.ErrorAssertionFunc
	}{
		{
			name: "all types success",
			conditions: conditions(
				eqCondition("google.compute_engine.instance_name", "hello-world"),
				eqCondition("google.compute_engine.project_number", "123456123456"), // string coercion test
				notEqCondition("email_verified", "false"),                           // contrived, but a realistic boolean test
				inCondition("azp", "1234567800", "1234567000", "1234567890"),
				notInCondition("google.compute_engine.zone", "us-central1-b", "us-central1-c", "us-central1-d"), // ban some AZs
			),
			expectError: require.NoError,
		},
		{
			name: "invalid field path, top level",
			conditions: conditions(
				eqCondition("foo", "12345"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `required claim attribute "foo" not found`)
			},
		},
		{
			name: "invalid field path, nested",
			conditions: conditions(
				eqCondition("google.foo.bar", "12345"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `required claim attribute "google.foo.bar" not found`)
			},
		},
		{
			name: "invalid field path, nested",
			conditions: conditions(
				eqCondition("google.foo.bar", "12345"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `required claim attribute "google.foo.bar" not found`)
			},
		},
		{
			name: "successful bool coercion",
			conditions: conditions(
				eqCondition("email_verified", "true"),
				notEqCondition("email_verified", "false"),
				inCondition("email_verified", "true"),
				notEqCondition("email_verified", "false"),
			),
			expectError: require.NoError,
		},
		{
			name: "successful number coercion",
			conditions: conditions(
				eqCondition("custom.float", "123.456"),
				notEqCondition("custom.float", "123.45"),
				notEqCondition("custom.float", "123.4567"),
				inCondition("custom.float", "123.0", "123", "123.456"),
				notInCondition("custom.float", "123", "123.4", "123.45", "123.4567"),
			),
			expectError: require.NoError,
		},
		{
			name: "rejects integer comparisons with large claims",
			conditions: conditions(
				eqCondition("custom.largeInt", "123"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "claim contains an integer value too large for safe comparison")
			},
		},
		{
			name: "rejects integer comparisons with large spec rule",
			conditions: conditions(
				eqCondition("custom.float", "9007199254740993"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "integers of this size cannot be safely compared")
			},
		},
		{
			name: "rejects integer comparisons with large spec rule via not_eq",
			conditions: conditions(
				notEqCondition("custom.float", "9007199254740993"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "integers of this size cannot be safely compared")
			},
		},
		{
			name: "rejects integer comparisons with large spec rule via not_in",
			conditions: conditions(
				notInCondition("custom.float", "9007199254740993"),
			),
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "integers of this size cannot be safely compared")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evaluateAllowAnyConditions(tt.conditions, parsedClaims)
			tt.expectError(t, err)
		})
	}
}
