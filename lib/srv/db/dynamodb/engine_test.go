/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package dynamodb

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiaws "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"
	libaws "github.com/gravitational/teleport/lib/utils/aws"
)

func TestResolveEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc            string
		target          string // from X-Amz-Target in requests
		region          string
		wantEndpointID  string
		wantSigningName string
		wantURL         string
		wantErrMsg      string
	}{
		{
			desc:            "dynamodb target in us west",
			target:          "DynamoDB_20120810.Scan",
			region:          "us-west-1",
			wantEndpointID:  "dynamodb",
			wantSigningName: "dynamodb",
			wantURL:         "https://dynamodb.us-west-1.amazonaws.com",
		},
		{
			desc:            "dynamodb target in china",
			target:          "DynamoDB_20120810.Scan",
			region:          "cn-north-1",
			wantEndpointID:  "dynamodb",
			wantSigningName: "dynamodb",
			wantURL:         "https://dynamodb.cn-north-1.amazonaws.com.cn",
		},
		{
			desc:            "dynamodb streams target in us west",
			target:          "DynamoDBStreams_20120810.ListStreams",
			region:          "us-west-1",
			wantEndpointID:  "streams.dynamodb",
			wantSigningName: "dynamodb",
			wantURL:         "https://streams.dynamodb.us-west-1.amazonaws.com",
		},
		{
			desc:            "dynamodb streams target in china",
			target:          "DynamoDBStreams_20120810.ListStreams",
			region:          "cn-north-1",
			wantEndpointID:  "streams.dynamodb",
			wantSigningName: "dynamodb",
			wantURL:         "https://streams.dynamodb.cn-north-1.amazonaws.com.cn",
		},
		{
			desc:            "dax target in us west",
			target:          "AmazonDAXV3.ListTags",
			region:          "us-west-1",
			wantEndpointID:  "dax",
			wantSigningName: "dax",
			wantURL:         "https://dax.us-west-1.amazonaws.com",
		},
		{
			desc:            "dax target in china",
			target:          "AmazonDAXV3.ListTags",
			region:          "cn-north-1",
			wantEndpointID:  "dax",
			wantSigningName: "dax",
			wantURL:         "https://dax.cn-north-1.amazonaws.com.cn",
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
			// mock a request.
			req := &http.Request{Header: make(http.Header)}
			req.Header.Set(libaws.AmzTargetHeader, tt.target)

			// check that the correct endpoint ID is extracted.
			endpointID, err := extractEndpointID(req)
			if tt.wantErrMsg != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}
			require.Equal(t, tt.wantEndpointID, endpointID)

			// check that the engine resolves the correct URL.
			db := &types.DatabaseV3{
				Spec: types.DatabaseSpecV3{
					URI: apiaws.DynamoDBURIForRegion(tt.region),
					AWS: types.AWS{
						Region:    tt.region,
						AccountID: "12345",
					},
				},
			}
			engine := &Engine{
				EngineConfig: common.EngineConfig{
					Log: slog.Default(),
				},
				sessionCtx: &common.Session{
					Database: db,
				},
			}
			re, err := engine.resolveEndpoint(req)
			require.NoError(t, err)
			require.Equal(t, tt.wantURL, re.URL)
			require.Equal(t, tt.wantSigningName, re.SigningName)

			// now use a custom URI and check that it overrides the resolved URL.
			db.Spec.URI = "foo.com"
			re, err = engine.resolveEndpoint(req)
			require.NoError(t, err)
			require.Equal(t, "https://foo.com", re.URL)
			require.Equal(t, tt.wantSigningName, re.SigningName)
		})
	}
}
