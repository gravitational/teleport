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

package dynamodb

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiaws "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func TestDynamoDBEndpointConstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc            string
		target          string // from X-Amz-Target in requests
		region          string
		wantService     string
		wantSigningName string
		wantURI         string
		wantErrMsg      string
	}{
		{
			desc:            "dynamodb target in us west",
			target:          "DynamoDB_20120810.Scan",
			region:          "us-west-1",
			wantService:     "DynamoDB",
			wantSigningName: "dynamodb",
			wantURI:         "dynamodb.us-west-1.amazonaws.com:443",
		},
		{
			desc:            "dynamodb target in china",
			target:          "DynamoDB_20120810.Scan",
			region:          "cn-north-1",
			wantService:     "DynamoDB",
			wantSigningName: "dynamodb",
			wantURI:         "dynamodb.cn-north-1.amazonaws.com.cn:443",
		},
		{
			desc:            "dynamodb streams target in us west",
			target:          "DynamoDBStreams_20120810.ListStreams",
			region:          "us-west-1",
			wantService:     "DynamoDB Streams",
			wantSigningName: "dynamodb",
			wantURI:         "streams.dynamodb.us-west-1.amazonaws.com:443",
		},
		{
			desc:            "dynamodb streams target in china",
			target:          "DynamoDBStreams_20120810.ListStreams",
			region:          "cn-north-1",
			wantService:     "DynamoDB Streams",
			wantSigningName: "dynamodb",
			wantURI:         "streams.dynamodb.cn-north-1.amazonaws.com.cn:443",
		},
		{
			desc:            "dax target in us west",
			target:          "AmazonDAXV3.ListTags",
			region:          "us-west-1",
			wantService:     "DAX",
			wantSigningName: "dax",
			wantURI:         "dax.us-west-1.amazonaws.com:443",
		},
		{
			desc:            "dax target in china",
			target:          "AmazonDAXV3.ListTags",
			region:          "cn-north-1",
			wantService:     "DAX",
			wantSigningName: "dax",
			wantURI:         "dax.cn-north-1.amazonaws.com.cn:443",
		},
		{
			desc:       "unrecognizable target",
			target:     "DDB.Scan",
			wantErrMsg: "is not recognized",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			// first part of this test validates that each helper function is correct.
			service, err := parseDynamoDBServiceFromTarget(tt.target)
			if tt.wantErrMsg != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}
			require.Equal(t, tt.wantService, service)
			signingName, err := serviceToSigningName(service)
			require.NoError(t, err)
			require.Equal(t, tt.wantSigningName, signingName)
			prefix, err := endpointPrefixForService(service)
			require.NoError(t, err)
			suffix := apiaws.DynamoDBEndpointSuffixForRegion(tt.region)
			targetURI := prefix + suffix
			require.Equal(t, tt.wantURI, targetURI)

			// second part of this test validates that the engine gets the correct target URI too.
			engine := &Engine{
				EngineConfig: common.EngineConfig{
					Log: logrus.StandardLogger(),
				},
				sessionCtx: &common.Session{
					Database: &types.DatabaseV3{
						Spec: types.DatabaseSpecV3{
							URI: suffix,
						},
					},
				},
			}
			targetURI, err = engine.getTargetURI(service)
			require.NoError(t, err)
			require.Equal(t, tt.wantURI, targetURI)
		})
	}
}
