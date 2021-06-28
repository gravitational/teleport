/*
Copyright 2021 Gravitational, Inc.

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

package auth

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

// TestSSOUserCanReissueCert makes sure that SSO user can reissue certificate
// for themselves.
func TestSSOUserCanReissueCert(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test SSO user.
	user, _, err := CreateUserAndRole(srv.Auth(), "sso-user", []string{"role"})
	require.NoError(t, err)
	user.SetCreatedBy(types.CreatedBy{
		Connector: &types.ConnectorRef{Type: "oidc", ID: "google"},
	})
	err = srv.Auth().UpdateUser(ctx, user)
	require.NoError(t, err)

	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	_, pub, err := srv.Auth().GenerateKeyPair("")
	require.NoError(t, err)

	_, err = client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user.GetName(),
		Expires:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
}

// TestGenerateDatabaseCert makes sure users and services with appropriate
// permissions can generate certificates for self-hosted databases.
func TestGenerateDatabaseCert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// This user can't impersonate anyone and can't generate database certs.
	userWithoutAccess, _, err := CreateUserAndRole(srv.Auth(), "user", []string{"role1"})
	require.NoError(t, err)

	// This user can impersonate system role Db.
	userImpersonateDb, roleDb, err := CreateUserAndRole(srv.Auth(), "user-impersonate-db", []string{"role2"})
	require.NoError(t, err)
	roleDb.SetImpersonateConditions(types.Allow, types.ImpersonateConditions{
		Users: []string{string(types.RoleDatabase)},
		Roles: []string{string(types.RoleDatabase)},
	})
	require.NoError(t, srv.Auth().UpsertRole(ctx, roleDb))

	tests := []struct {
		desc     string
		identity TestIdentity
		err      string
	}{
		{
			desc:     "user can't sign database certs",
			identity: TestUser(userWithoutAccess.GetName()),
			err:      "access denied",
		},
		{
			desc:     "user can impersonate Db and sign database certs",
			identity: TestUser(userImpersonateDb.GetName()),
		},
		{
			desc:     "built-in admin can sign database certs",
			identity: TestAdmin(),
		},
		{
			desc:     "database service can sign database certs",
			identity: TestBuiltin(types.RoleDatabase),
		},
	}

	// Generate CSR once for speed sake.
	priv, _, err := srv.Auth().GenerateKeyPair("")
	require.NoError(t, err)
	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{CommonName: "test"}, priv)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			client, err := srv.NewClient(test.identity)
			require.NoError(t, err)

			_, err = client.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{CSR: csr})
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSetAuthPreference tests the dynamic configuration rules described
// in rfd/0016-dynamic-configuration.md § Implementation.
func TestSetAuthPreference(t *testing.T) {
	ctx := context.Background()
	testAuth, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	// Initialize with the default auth preference.
	err = testAuth.AuthServer.SetAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	storedAuthPref, err := testAuth.AuthServer.GetAuthPreference(ctx)
	require.NoError(t, err)
	require.Empty(t, resourceDiff(storedAuthPref, types.DefaultAuthPreference()))

	// Grant VerbRead and VerbUpdate privileges for cluster_auth_preference.
	allowRules := []types.Rule{
		{
			Resources: []string{"cluster_auth_preference"},
			Verbs:     []string{types.VerbRead, types.VerbUpdate},
		},
	}
	server := withAllowRules(t, testAuth, allowRules)

	dynamicAuthPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		SecondFactor: constants.SecondFactorOff,
	})
	require.NoError(t, err)
	t.Run("from default to dynamic", func(t *testing.T) {
		err = server.SetAuthPreference(ctx, dynamicAuthPref)
		require.NoError(t, err)
		storedAuthPref, err = server.GetAuthPreference(ctx)
		require.NoError(t, err)
		require.Empty(t, resourceDiff(storedAuthPref, dynamicAuthPref))
	})

	newDynamicAuthPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	t.Run("from dynamic to another dynamic", func(t *testing.T) {
		err = server.SetAuthPreference(ctx, newDynamicAuthPref)
		require.NoError(t, err)
		storedAuthPref, err = server.GetAuthPreference(ctx)
		require.NoError(t, err)
		require.Empty(t, resourceDiff(storedAuthPref, newDynamicAuthPref))
	})

	staticAuthPref := newU2FAuthPreferenceFromConfigFile(t)
	t.Run("from dynamic to static", func(t *testing.T) {
		err = server.SetAuthPreference(ctx, staticAuthPref)
		require.NoError(t, err)
		storedAuthPref, err = server.GetAuthPreference(ctx)
		require.NoError(t, err)
		require.Empty(t, resourceDiff(storedAuthPref, staticAuthPref))
	})

	newAuthPref, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	replaceStatic := func(success bool) func(t *testing.T) {
		return func(t *testing.T) {
			err = server.SetAuthPreference(ctx, newAuthPref)
			checkSetResult := require.Error
			if success {
				checkSetResult = require.NoError
			}
			checkSetResult(t, err)

			storedAuthPref, err = server.GetAuthPreference(ctx)
			require.NoError(t, err)
			expectedStored := staticAuthPref
			if success {
				expectedStored = newAuthPref
			}
			require.Empty(t, resourceDiff(storedAuthPref, expectedStored))
		}
	}

	t.Run("replacing static fails without VerbCreate privilege", replaceStatic(false))

	// Grant VerbCreate privilege for cluster_auth_preference.
	allowRules[0].Verbs = append(allowRules[0].Verbs, types.VerbCreate)
	server = withAllowRules(t, testAuth, allowRules)

	t.Run("replacing static success with VerbCreate privilege", replaceStatic(true))
}

func withAllowRules(t *testing.T, srv *TestAuthServer, allowRules []types.Rule) *ServerWithRoles {
	username := "some-user"
	_, role, err := CreateUserAndRoleWithoutRoles(srv.AuthServer, username, nil)
	require.NoError(t, err)
	role.SetRules(types.Allow, allowRules)
	err = srv.AuthServer.UpsertRole(context.TODO(), role)
	require.NoError(t, err)

	localUser := LocalUser{Username: username, Identity: tlsca.Identity{Username: username}}
	authContext, err := contextForLocalUser(localUser, srv.AuthServer.Identity, srv.AuthServer.Access)
	require.NoError(t, err)

	return &ServerWithRoles{
		authServer: srv.AuthServer,
		sessions:   srv.SessionServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}
}

func resourceDiff(res1, res2 types.Resource) string {
	return cmp.Diff(res1, res2,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
		cmpopts.EquateEmpty())
}

// TestListNodes users can retrieve nodes with the appropriate permissions.
func TestListNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test nodes.
	var err error
	testNodes := make([]types.Server, 10)
	for i := 0; i < 10; i++ {
		testNodes[i], err = types.NewServerWithLabels(
			"node"+fmt.Sprint(i),
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{fmt.Sprint(i): fmt.Sprint(i)},
		)
		require.NoError(t, err)
		testNodes[i].SetResourceID(int64(i))
		_, err = srv.Auth().UpsertNode(ctx, testNodes[i])
		require.NoError(t, err)
	}

	// create user and role
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// user should be able to list all nodes permitted.
	role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))

	nodes, _, err := clt.ListNodes(ctx, apidefaults.Namespace, 0, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(testNodes, nodes, cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// unpermitted nodes should be replaced with the next permitted node.
	role.SetNodeLabels(types.Deny, types.Labels{"3": {"3"}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))

	nodes, nextKey, err := clt.ListNodes(ctx, apidefaults.Namespace, 5, "")
	require.NoError(t, err)
	expectedNodes := append(testNodes[:3], testNodes[4:6]...)
	require.Empty(t, cmp.Diff(expectedNodes, nodes, cmpopts.IgnoreFields(types.Metadata{}, "ID")))
	require.EqualValues(t, "/nodes/default/node6", nextKey)
}
