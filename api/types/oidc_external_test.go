// Copyright 2022 Gravitational, Inc
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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestOIDCClaimsRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		src  OIDCClaims
	}{
		{
			name: "empty",
			src:  OIDCClaims{},
		},
		{
			name: "full",
			src: OIDCClaims(map[string]interface{}{
				"email_verified": true,
				"groups":         []interface{}{"everyone", "idp-admin", "idp-dev"},
				"email":          "superuser@example.com",
				"sub":            "00001234abcd",
				"exp":            1652091713.0,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.src.Size())
			count, err := tt.src.MarshalTo(buf)
			require.NoError(t, err)
			require.Equal(t, tt.src.Size(), count)

			dst := &OIDCClaims{}
			err = dst.Unmarshal(buf)
			require.NoError(t, err)
			require.Equal(t, &tt.src, dst)
		})
	}
}

func TestClientSecretFileURI(t *testing.T) {
	_, err := NewOIDCConnector("test-connector", OIDCConnectorSpecV3{
		ClientID:     "some-client-id",
		ClientSecret: "file://is-not-allowed",
		ClaimsToRoles: []ClaimMapping{
			{
				Claim: "team",
				Value: "dev",
				Roles: []string{"dev-team-access"},
			},
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "file:// URLs are not supported")
}
