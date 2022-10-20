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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/aws"
)

func TestDatabaseGetIAMPolicy(t *testing.T) {
	t.Parallel()

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "my-redshift",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
		AWS: types.AWS{
			AccountID: "my-account-id",
			RDS: types.RDS{
				ResourceID: "my-resource-id",
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, database.GetIAMAction())
	require.NotEmpty(t, database.GetIAMResources())

	databaseServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "my-redshift",
	}, types.DatabaseServerSpecV3{
		HostID:   "host-id",
		Hostname: "host-name",
		Database: database,
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	_, err = env.server.Auth().UpsertDatabaseServer(context.TODO(), databaseServer)
	require.NoError(t, err)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "user", nil /* roles */)

	resp, err := pack.clt.Get(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "database", "my-redshift", "iam", "policy"), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	requireDatabaseIAMPolicyAWS(t, resp.Bytes(), database)

	// TODO add error cases
}

func requireDatabaseIAMPolicyAWS(t *testing.T, bytes []byte, database types.Database) {
	t.Helper()

	var resp databaseIAMPolicyResponse
	require.NoError(t, json.Unmarshal(bytes, &resp))
	require.Equal(t, "aws", resp.Type)

	policyDocument, err := aws.ParsePolicyDocument(resp.AWS.PolicyDocument)
	require.NoError(t, err)

	for _, iamResource := range database.GetIAMResources() {
		alreadyExist := policyDocument.Ensure(aws.EffectAllow, database.GetIAMAction(), iamResource)
		require.True(t, alreadyExist)
	}
}
