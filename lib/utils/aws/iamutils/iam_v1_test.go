// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package iamutils_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
)

func TestNewV1(t *testing.T) {
	// Don't t.Parallel(), uses t.Setenv().

	configProvider := &mockConfigProvider{
		Config: client.Config{
			Config: aws.NewConfig().WithUseFIPSEndpoint(true),
		},
	}

	tests := []struct {
		name        string
		envVarValue string // value for the _DISABLE_FIPS environment variable
		want        endpoints.FIPSEndpointState
	}{
		{
			name: "fips",
			want: endpoints.FIPSEndpointStateEnabled,
		},
		{
			name:        "fips disabled by env",
			envVarValue: "yes",
			want:        endpoints.FIPSEndpointStateDisabled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_DISABLE_AWS_FIPS", test.envVarValue)

			iamClient := iamutils.NewV1(configProvider)
			require.NotNil(t, iamClient, "*iam.IAM")

			got := iamClient.Config.UseFIPSEndpoint
			assert.Equal(t, test.want, got, "iamClient.Config.UseFIPSEndpoint mismatch")
		})
	}
}

type mockConfigProvider struct {
	Config client.Config
}

func (m *mockConfigProvider) ClientConfig(_ string, cfgs ...*aws.Config) client.Config {
	cc := m.Config
	cc.Config = cc.Config.Copy(cfgs...)
	return cc
}
