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

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
)

func TestHandleDatabasesGetIAMPolicy(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "user", nil /* roles */)

	redshift, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-redshift",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
	})
	require.NoError(t, err)

	selfHosted, err := types.NewDatabaseV3(types.Metadata{
		Name: "self-hosted",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:12345",
	})
	require.NoError(t, err)

	// Add database servers for above databases.
	for _, db := range []*types.DatabaseV3{redshift, selfHosted} {
		_, err = env.server.Auth().UpsertDatabaseServer(context.TODO(), mustCreateDatabaseServer(t, db))
		require.NoError(t, err)
	}

	tests := []struct {
		inputDatabaseName string
		verifyResponse    func(*testing.T, *roundtrip.Response, error)
	}{
		{
			inputDatabaseName: "aws-redshift",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
				requireDatabaseIAMPolicyAWS(t, resp.Bytes(), redshift)
			},
		},
		{
			inputDatabaseName: "self-hosted",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.True(t, trace.IsBadParameter(err))
				require.Equal(t, http.StatusBadRequest, resp.Code())
			},
		},
		{
			inputDatabaseName: "not-found",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.True(t, trace.IsNotFound(err))
				require.Equal(t, http.StatusNotFound, resp.Code())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.inputDatabaseName, func(t *testing.T) {
			resp, err := pack.clt.Get(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databases", test.inputDatabaseName, "iam", "policy"), nil)
			test.verifyResponse(t, resp, err)
		})
	}
}

func mustCreateDatabaseServer(t *testing.T, db *types.DatabaseV3) types.DatabaseServer {
	t.Helper()

	databaseServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: db.GetName(),
	}, types.DatabaseServerSpecV3{
		HostID:   "host-id",
		Hostname: "host-name",
		Database: db,
	})
	require.NoError(t, err)
	return databaseServer
}

func requireDatabaseIAMPolicyAWS(t *testing.T, respBody []byte, database types.Database) {
	t.Helper()

	var resp databaseIAMPolicyResponse
	require.NoError(t, json.Unmarshal(respBody, &resp))
	require.Equal(t, "aws", resp.Type)

	expectedPolicyDocument, expectedPlaceholders, err := dbiam.GetAWSPolicyDocumentMarshaled(database)
	require.NoError(t, err)
	require.Equal(t, expectedPolicyDocument, resp.AWS.PolicyDocument)
	require.Equal(t, []string(expectedPlaceholders), resp.AWS.Placeholders)
}
