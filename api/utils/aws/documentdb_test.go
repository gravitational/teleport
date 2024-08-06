/*
Copyright 2024 Gravitational, Inc.

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

func TestParseDocumentDBEndpoint(t *testing.T) {
	tests := []struct {
		name                       string
		endpoint                   string
		expectIsDocumentDBEndpoint bool
		expectDetails              *DocumentDBEndpointDetails
		expectParseErrorIs         func(error) bool
	}{
		{
			name:                       "DocuementDB cluster",
			endpoint:                   "my-documentdb-cluster-id.cluster-abcdefghijklmnop.us-east-1.docdb.amazonaws.com:27017",
			expectIsDocumentDBEndpoint: true,
			expectDetails: &DocumentDBEndpointDetails{
				ClusterID:    "my-documentdb-cluster-id",
				Region:       "us-east-1",
				EndpointType: "cluster",
			},
		},
		{
			name:                       "DocuementDB cluster in mongo URL format",
			endpoint:                   "mongodb://my-documentdb-cluster-id.cluster-abcdefghijklmnop.us-east-1.docdb.amazonaws.com:27017/?retryWrites=false",
			expectIsDocumentDBEndpoint: true,
			expectDetails: &DocumentDBEndpointDetails{
				ClusterID:    "my-documentdb-cluster-id",
				Region:       "us-east-1",
				EndpointType: "cluster",
			},
		},
		{
			name:                       "DocuementDB cluster reader",
			endpoint:                   "my-documentdb-cluster-id.cluster-ro-abcdefghijklmnop.us-east-1.docdb.amazonaws.com",
			expectIsDocumentDBEndpoint: true,
			expectDetails: &DocumentDBEndpointDetails{
				ClusterID:    "my-documentdb-cluster-id",
				Region:       "us-east-1",
				EndpointType: "reader",
			},
		},
		{
			name:                       "DocuementDB instance",
			endpoint:                   "my-instance-id.abcdefghijklmnop.us-east-1.docdb.amazonaws.com",
			expectIsDocumentDBEndpoint: true,
			expectDetails: &DocumentDBEndpointDetails{
				InstanceID:   "my-instance-id",
				Region:       "us-east-1",
				EndpointType: "instance",
			},
		},
		{
			name:                       "DocuementDB CN",
			endpoint:                   "my-documentdb-cluster-id.cluster-abcdefghijklmnop.docdb.cn-north-1.amazonaws.com.cn",
			expectIsDocumentDBEndpoint: true,
			expectDetails: &DocumentDBEndpointDetails{
				ClusterID:    "my-documentdb-cluster-id",
				Region:       "cn-north-1",
				EndpointType: "cluster",
			},
		},
		{
			name:                       "RDS instance fails",
			endpoint:                   "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
			expectIsDocumentDBEndpoint: false,
			expectParseErrorIs:         trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectIsDocumentDBEndpoint, IsDocumentDBEndpoint(test.endpoint))

			actualDetails, err := ParseDocumentDBEndpoint(test.endpoint)
			if test.expectParseErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectParseErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectDetails, actualDetails)
			}
		})
	}
}
