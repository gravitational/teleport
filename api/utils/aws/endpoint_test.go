/*
Copyright 2022 Gravitational, Inc.

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

package aws

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestParseRDSEndpoint(t *testing.T) {
	tests := []struct {
		name                string
		endpoint            string
		expectIsRDSEndpoint bool
		expectInstanceID    string
		expectRegion        string
		expectParseErrorIs  func(error) bool
	}{
		{
			name:                "standard",
			endpoint:            "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectInstanceID:    "aurora-instance-1",
			expectRegion:        "us-west-1",
		},
		{
			name:                "cn-north-1",
			endpoint:            "aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn",
			expectIsRDSEndpoint: true,
			expectInstanceID:    "aurora-instance-2",
			expectRegion:        "cn-north-1",
		},
		{
			name:                "localhost:5432",
			endpoint:            "localhost",
			expectIsRDSEndpoint: false,
			expectParseErrorIs:  trace.IsBadParameter,
		},
		{
			name:                "Redshift endpoint fails",
			endpoint:            "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com",
			expectIsRDSEndpoint: false,
			expectParseErrorIs:  trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRDSEndpoint, IsRDSEndpoint(test.endpoint))

			instanceID, region, err := ParseRDSEndpoint(test.endpoint)
			if test.expectParseErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectParseErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectInstanceID, instanceID)
				require.Equal(t, test.expectRegion, region)
			}
		})
	}
}

func TestParseRedshiftEndpoint(t *testing.T) {
	tests := []struct {
		name                     string
		endpoint                 string
		expectIsRedshiftEndpoint bool
		expectClusterID          string
		expectRegion             string
		expectParseErrorIs       func(error) bool
	}{
		{
			name:                     "standard",
			endpoint:                 "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5432",
			expectClusterID:          "redshift-cluster-1",
			expectRegion:             "us-east-1",
			expectIsRedshiftEndpoint: true,
		},
		{
			name:                     "cn-north-1",
			endpoint:                 "redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-1.amazonaws.com.cn",
			expectClusterID:          "redshift-cluster-2",
			expectRegion:             "cn-north-1",
			expectIsRedshiftEndpoint: true,
		},
		{
			name:                     "localhost:5432",
			endpoint:                 "localhost",
			expectIsRedshiftEndpoint: false,
			expectParseErrorIs:       trace.IsBadParameter,
		},
		{
			name:                     "RDS endpoint fails",
			endpoint:                 "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com",
			expectIsRedshiftEndpoint: false,
			expectParseErrorIs:       trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRedshiftEndpoint, IsRedshiftEndpoint(test.endpoint))

			clusterID, region, err := ParseRedshiftEndpoint(test.endpoint)
			if test.expectParseErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectParseErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectClusterID, clusterID)
				require.Equal(t, test.expectRegion, region)
			}
		})
	}
}
