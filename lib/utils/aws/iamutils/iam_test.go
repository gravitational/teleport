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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
)

func TestNewFromConfig(t *testing.T) {
	// Don't t.Parallel(), uses t.Setenv().

	cfg := aws.Config{}
	opts := func(opts *iam.Options) {
		opts.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
	}

	tests := []struct {
		name        string
		envVarValue string // value for the _DISABLE_FIPS environment variable
		want        aws.FIPSEndpointState
	}{
		{
			name: "fips",
			want: aws.FIPSEndpointStateEnabled,
		},
		{
			name:        "fips disabled by env",
			envVarValue: "yes",
			want:        aws.FIPSEndpointStateDisabled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_DISABLE_AWS_FIPS", test.envVarValue)

			iamClient := iamutils.NewFromConfig(cfg, opts)
			require.NotNil(t, iamClient, "*iam.Client")

			got := iamClient.Options().EndpointOptions.UseFIPSEndpoint
			assert.Equal(t, test.want, got, "opts.EndpointOptions.UseFIPSEndpoint mismatch")
		})
	}
}
