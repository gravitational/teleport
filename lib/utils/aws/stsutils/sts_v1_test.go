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

package stsutils_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
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
		envVarValue string // value for the _DISABLE_STS_FIPS environment variable
		want        endpoints.FIPSEndpointState
	}{
		{
			name: "env not set",
			want: endpoints.FIPSEndpointStateEnabled,
		},
		{
			name:        "invalid does not change FIPS",
			envVarValue: "llama",
			want:        endpoints.FIPSEndpointStateEnabled,
		},
		{
			name:        "false does not change FIPS",
			envVarValue: "0",
			want:        endpoints.FIPSEndpointStateEnabled,
		},
		{
			name:        `"yes" disables FIPS`,
			envVarValue: "yes",
			want:        endpoints.FIPSEndpointStateDisabled,
		},
		{
			name:        "1 disables FIPS",
			envVarValue: "1",
			want:        endpoints.FIPSEndpointStateDisabled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_DISABLE_STS_FIPS", test.envVarValue)

			stsClient := stsutils.NewV1(configProvider)
			require.NotNil(t, stsClient, "*sts.Client")

			got := stsClient.Config.UseFIPSEndpoint
			assert.Equal(t, test.want, got, "opts.EndpointOptions.UseFIPSEndpoint mismatch")
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
