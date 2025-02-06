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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

func TestNewFromConfig(t *testing.T) {
	// Don't t.Parallel(), uses t.Setenv().

	cfg := aws.Config{}
	opts := func(opts *sts.Options) {
		opts.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
	}

	tests := []struct {
		name        string
		envVarValue string // value for the _DISABLE_STS_FIPS environment variable
		want        aws.FIPSEndpointState
	}{
		{
			name: "env not set",
			want: aws.FIPSEndpointStateEnabled,
		},
		{
			name:        "invalid does not change FIPS",
			envVarValue: "llama",
			want:        aws.FIPSEndpointStateEnabled,
		},
		{
			name:        "false does not change FIPS",
			envVarValue: "0",
			want:        aws.FIPSEndpointStateEnabled,
		},
		{
			name:        `"yes" disables FIPS`,
			envVarValue: "yes",
			want:        aws.FIPSEndpointStateDisabled,
		},
		{
			name:        "1 disables FIPS",
			envVarValue: "1",
			want:        aws.FIPSEndpointStateDisabled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_DISABLE_STS_FIPS", test.envVarValue)

			stsClient := stsutils.NewFromConfig(cfg, opts)
			require.NotNil(t, stsClient, "*sts.Client")

			got := stsClient.Options().EndpointOptions.UseFIPSEndpoint
			assert.Equal(t, test.want, got, "opts.EndpointOptions.UseFIPSEndpoint mismatch")
		})
	}
}
