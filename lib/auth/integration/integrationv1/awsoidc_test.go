/*
Copyright 2023 Gravitational, Inc.

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

package integrationv1

import (
	"testing"

	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGenerateAWSOIDCToken(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"

	publicURL := "https://example.com"

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, types.KindIntegration, ca, clusterName)

	ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{
			{Resources: []string{types.KindIntegration}, Verbs: []string{types.VerbUse}},
		}},
	}, localClient)

	resp, err := resourceSvc.GenerateAWSOIDCToken(ctx, &integrationv1.GenerateAWSOIDCTokenRequest{
		Issuer: publicURL,
	})
	require.NoError(t, err)

	// Get Public Key
	require.NotEmpty(t, ca.GetActiveKeys().JWT)
	jwtPubKey := ca.GetActiveKeys().JWT[0].PublicKey

	publicKey, err := utils.ParsePublicKey(jwtPubKey)
	require.NoError(t, err)

	// Validate JWT against public key
	key, err := jwt.New(&jwt.Config{
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: clusterName,
		Clock:       resourceSvc.clock,
		PublicKey:   publicKey,
	})
	require.NoError(t, err)

	_, err = key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
		RawToken: resp.GetToken(),
		Issuer:   publicURL,
	})
	require.NoError(t, err)

	// Fails if the issuer is different
	_, err = key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
		RawToken: resp.GetToken(),
		Issuer:   publicURL + "3",
	})
	require.Error(t, err)
}
