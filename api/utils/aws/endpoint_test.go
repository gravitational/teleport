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

func TestRDSEndpoint(t *testing.T) {
	tests := []struct {
		name                string
		uri                 string
		expectIsRDSEndpoint bool
		expectInstanceID    string
		expectRegion        string
		expectParseErrorIs  func(error) bool
	}{
		{
			name:                "standard",
			uri:                 "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRDSEndpoint: true,
			expectInstanceID:    "aurora-instance-1",
			expectRegion:        "us-west-1",
		},
		{
			name:                "cn-north-1",
			uri:                 "aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn:5432",
			expectIsRDSEndpoint: true,
			expectInstanceID:    "aurora-instance-2",
			expectRegion:        "cn-north-1",
		},
		{
			name:                "localhost",
			uri:                 "localhost:5432",
			expectIsRDSEndpoint: false,
			expectParseErrorIs:  trace.IsBadParameter,
		},
		{
			name:                "Redshift URI fails",
			uri:                 "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5432",
			expectIsRDSEndpoint: false,
			expectParseErrorIs:  trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRDSEndpoint, IsRDSEndpoint(test.uri))

			instanceID, region, err := ParseRDSEndpoint(test.uri)
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

func TestRedshiftEndpoint(t *testing.T) {
	tests := []struct {
		name                     string
		uri                      string
		expectIsRedshiftEndpoint bool
		expectClusterID          string
		expectRegion             string
		expectParseErrorIs       func(error) bool
	}{
		{
			name:                     "standard",
			uri:                      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5432",
			expectClusterID:          "redshift-cluster-1",
			expectRegion:             "us-east-1",
			expectIsRedshiftEndpoint: true,
		},
		{
			name:                     "cn-north-1",
			uri:                      "redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-1.amazonaws.com:5432",
			expectClusterID:          "redshift-cluster-2",
			expectRegion:             "cn-north-1",
			expectIsRedshiftEndpoint: true,
		},
		{
			name:                     "localhost",
			uri:                      "localhost:5432",
			expectIsRedshiftEndpoint: false,
			expectParseErrorIs:       trace.IsBadParameter,
		},
		{
			name:                     "RDS URI fails",
			uri:                      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsRedshiftEndpoint: false,
			expectParseErrorIs:       trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsRedshiftEndpoint, IsRedshiftEndpoint(test.uri))

			clusterID, region, err := ParseRedshiftEndpoint(test.uri)
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
