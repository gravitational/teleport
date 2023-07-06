/*
Copyright 2017 Gravitational, Inc.

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

package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
)

func TestParseFromMetadata(t *testing.T) {
	t.Parallel()

	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	oc, err := UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)
	require.Equal(t, oc.GetIssuer(), "http://www.okta.com/exkafftca6RqPVgyZ0h7")
	require.Equal(t, oc.GetSSO(), "https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml")
	require.Equal(t, oc.GetAssertionConsumerService(), "https://localhost:3080/webapi/saml/acs")
	require.Equal(t, oc.GetAudience(), "https://localhost:3080/webapi/saml/acs")
	require.NotNil(t, oc.GetSigningKeyPair())
	require.Empty(t, cmp.Diff(oc.GetAttributes(), []string{"groups"}))
}

func TestCheckSAMLEntityDescriptor(t *testing.T) {
	t.Parallel()

	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	oc, err := UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)

	ed := oc.GetEntityDescriptor()
	certs, err := CheckSAMLEntityDescriptor(ed)
	require.NoError(t, err)
	require.Len(t, certs, 1)
}

func TestValidateRoles(t *testing.T) {
	t.Parallel()

	// Create a roleSet with <nil> role values as ValidateSAMLRole does
	// not care what the role value is, just that it exists and that
	// the RoleGetter does not return an error.
	var validRoles roleSet = map[string]types.Role{
		"foo":  nil,
		"bar":  nil,
		"big$": nil,
	}

	testCases := []struct {
		desc        string
		roles       []string
		expectedErr error
	}{
		{
			desc:  "valid roles",
			roles: []string{"foo", "bar"},
		},
		{
			desc:  "role templates",
			roles: []string{"foo", "$1", "$baz", "admin_${baz}"},
		},
		{
			desc:        "missing role",
			roles:       []string{"baz"},
			expectedErr: trace.BadParameter(`role "baz" specified in attributes_to_roles not found`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			connector, err := types.NewSAMLConnector("test_connector", types.SAMLConnectorSpecV2{
				AssertionConsumerService: "http://localhost:65535/acs", // not called
				Issuer:                   "test",
				SSO:                      "https://localhost:65535/sso", // not called
				AttributesToRoles: []types.AttributeMapping{
					// not used. can be any name, value but role must exist
					{Name: "groups", Value: "admin", Roles: tc.roles},
				},
			})
			require.NoError(t, err)

			err = ValidateSAMLConnector(connector, validRoles)
			require.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

// roleSet is a basic set of roles keyed by role name. It implements the
// RoleGetter interface, returning the role if it exists, or a trace.NotFound
// error if it does not exist.
type roleSet map[string]types.Role

func (rs roleSet) GetRole(ctx context.Context, name string) (types.Role, error) {
	if r, ok := rs[name]; ok {
		return r, nil
	}
	return nil, trace.NotFound("unknown role: %s", name)
}
