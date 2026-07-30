// Copyright 2025 Gravitational, Inc
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

package types

import (
	"cmp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/wrappers"
)

func TestOIDCValidate(t *testing.T) {
	tests := []struct {
		name          string
		connectorName string
		entra         *EntraIDGroupsProvider
		errAssertion  require.ErrorAssertionFunc
	}{
		{
			name: "invalid group type",
			entra: &EntraIDGroupsProvider{
				GroupType: "random",
			},
			errAssertion: require.Error,
		},
		{
			name: "invalid endpoint",
			entra: &EntraIDGroupsProvider{
				GroupType:     "all-groups",
				GraphEndpoint: "https://example.com",
			},
			errAssertion: require.Error,
		},
		{
			name:         "empty entra_id_groups_provider",
			entra:        nil,
			errAssertion: require.NoError,
		},
		{
			name:          "name too long",
			connectorName: strings.Repeat("abc", 300),
			errAssertion:  require.Error,
		},
		{
			name: "disabled state should not skip invalid configuration",
			entra: &EntraIDGroupsProvider{
				Disabled:      true,
				GroupType:     "all-groups",
				GraphEndpoint: "https://example.com",
			},
			errAssertion: require.Error,
		},
		{
			name: "valid",
			entra: &EntraIDGroupsProvider{
				Disabled:      false,
				GroupType:     "all-groups",
				GraphEndpoint: "https://graph.microsoft.com",
			},
			errAssertion: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connector, err := NewOIDCConnector(
				cmp.Or(test.connectorName, "test-connector"),
				OIDCConnectorSpecV3{
					ClientID:     "testid",
					ClientSecret: "secret",
					ClaimsToRoles: []ClaimMapping{
						{
							Claim: "groups",
							Value: "*",
							Roles: []string{"requester"},
						},
					},
					RedirectURLs: wrappers.Strings{
						"https://example.com/proxy/oidc/callback",
					},
					EntraIdGroupsProvider: test.entra,
				},
			)
			require.NoError(t, err)

			test.errAssertion(t, connector.Validate())
		})
	}
}

func TestNewOIDCConnectorClaimsToRoles(t *testing.T) {
	tests := []struct {
		name          string
		claimsToRoles []ClaimMapping
		assertErr     require.ErrorAssertionFunc
	}{
		{
			name:          "empty",
			claimsToRoles: []ClaimMapping{},
			assertErr:     require.NoError,
		},
		{
			name:          "invalid",
			claimsToRoles: []ClaimMapping{{Claim: "claim", Value: "value", Roles: []string{}}},
			assertErr:     require.Error,
		},
		{
			name:          "valid",
			claimsToRoles: []ClaimMapping{{Claim: "claim", Value: "value", Roles: []string{"access"}}},
			assertErr:     require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run("role-mapping-"+tt.name, func(t *testing.T) {
			spec := OIDCConnectorSpecV3{
				ClientID:      "client-id",
				ClientSecret:  "client-secret",
				RedirectURLs:  []string{"https://example.com"},
				ClaimsToRoles: tt.claimsToRoles,
			}
			_, err := NewOIDCConnector(tt.name, spec)
			tt.assertErr(t, err)
		})
	}
}
