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
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
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

type testDynamicallyConfigurableRBACParams struct {
	kind                          string
	storeDefault, storeConfigFile func(*Server)
	get, set, reset               func(*ServerWithRoles) error
	alwaysReadable                bool
}

// TestDynamicConfigurationRBACVerbs tests the dynamic configuration RBAC verbs described
// in rfd/0016-dynamic-configuration.md § Implementation.
func testDynamicallyConfigurableRBAC(t *testing.T, p testDynamicallyConfigurableRBACParams) {
	testAuth, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	testOperation := func(op func(*ServerWithRoles) error, allowRules []types.Rule, expectErr, withConfigFile bool) func(*testing.T) {
		return func(t *testing.T) {
			if withConfigFile {
				p.storeConfigFile(testAuth.AuthServer)
			} else {
				p.storeDefault(testAuth.AuthServer)
			}
			server := serverWithAllowRules(t, testAuth, allowRules)
			opErr := op(server)
			if expectErr {
				require.Error(t, opErr)
			} else {
				require.NoError(t, opErr)
			}
		}
	}

	// runTestCases generates all non-empty RBAC verb combinations and checks the expected
	// error for each operation.
	runTestCases := func(withConfigFile bool) {
		for _, canCreate := range []bool{false, true} {
			for _, canUpdate := range []bool{false, true} {
				for _, canRead := range []bool{false, true} {
					if !canRead && !canUpdate && !canCreate {
						continue
					}
					verbs := []string{}
					expectGetErr, expectSetErr, expectResetErr := true, true, true
					if canRead || p.alwaysReadable {
						verbs = append(verbs, types.VerbRead)
						expectGetErr = false
					}
					if canUpdate {
						verbs = append(verbs, types.VerbUpdate)
						if !withConfigFile {
							expectSetErr, expectResetErr = false, false
						}
					}
					if canCreate {
						verbs = append(verbs, types.VerbCreate)
						if canUpdate {
							expectSetErr = false
						}
					}
					allowRules := []types.Rule{
						{
							Resources: []string{p.kind},
							Verbs:     verbs,
						},
					}
					t.Run(fmt.Sprintf("get %v %v", verbs, withConfigFile), testOperation(p.get, allowRules, expectGetErr, withConfigFile))
					t.Run(fmt.Sprintf("set %v %v", verbs, withConfigFile), testOperation(p.set, allowRules, expectSetErr, withConfigFile))
					t.Run(fmt.Sprintf("reset %v %v", verbs, withConfigFile), testOperation(p.reset, allowRules, expectResetErr, withConfigFile))
				}
			}
		}
	}

	runTestCases(false)
	runTestCases(true)
}

func TestAuthPreferenceRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testDynamicallyConfigurableRBAC(t, testDynamicallyConfigurableRBACParams{
		kind: types.KindClusterAuthPreference,
		storeDefault: func(s *Server) {
			s.SetAuthPreference(ctx, types.DefaultAuthPreference())
		},
		storeConfigFile: func(s *Server) {
			authPref := types.DefaultAuthPreference()
			authPref.SetOrigin(types.OriginConfigFile)
			s.SetAuthPreference(ctx, authPref)
		},
		get: func(s *ServerWithRoles) error {
			_, err := s.GetAuthPreference(ctx)
			return err
		},
		set: func(s *ServerWithRoles) error {
			return s.SetAuthPreference(ctx, types.DefaultAuthPreference())
		},
		reset: func(s *ServerWithRoles) error {
			return s.ResetAuthPreference(ctx)
		},
		alwaysReadable: true,
	})
}

func TestClusterNetworkingConfigRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testDynamicallyConfigurableRBAC(t, testDynamicallyConfigurableRBACParams{
		kind: types.KindClusterNetworkingConfig,
		storeDefault: func(s *Server) {
			s.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
		},
		storeConfigFile: func(s *Server) {
			netConfig := types.DefaultClusterNetworkingConfig()
			netConfig.SetOrigin(types.OriginConfigFile)
			s.SetClusterNetworkingConfig(ctx, netConfig)
		},
		get: func(s *ServerWithRoles) error {
			_, err := s.GetClusterNetworkingConfig(ctx)
			return err
		},
		set: func(s *ServerWithRoles) error {
			return s.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
		},
		reset: func(s *ServerWithRoles) error {
			return s.ResetClusterNetworkingConfig(ctx)
		},
	})
}

func TestSessionRecordingConfigRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testDynamicallyConfigurableRBAC(t, testDynamicallyConfigurableRBACParams{
		kind: types.KindSessionRecordingConfig,
		storeDefault: func(s *Server) {
			s.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
		},
		storeConfigFile: func(s *Server) {
			recConfig := types.DefaultSessionRecordingConfig()
			recConfig.SetOrigin(types.OriginConfigFile)
			s.SetSessionRecordingConfig(ctx, recConfig)
		},
		get: func(s *ServerWithRoles) error {
			_, err := s.GetSessionRecordingConfig(ctx)
			return err
		},
		set: func(s *ServerWithRoles) error {
			return s.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
		},
		reset: func(s *ServerWithRoles) error {
			return s.ResetSessionRecordingConfig(ctx)
		},
	})
}

// TestListNodes users can retrieve nodes with the appropriate permissions.
func TestListNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test nodes.
	for i := 0; i < 10; i++ {
		name := uuid.New()
		node, err := types.NewServerWithLabels(
			name,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": name},
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertNode(ctx, node)
		require.NoError(t, err)
	}

	testNodes, err := srv.Auth().GetNodes(ctx, defaults.Namespace)
	require.NoError(t, err)

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to list all nodes
	role.SetNodeLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))

	// listing nodes 0-4 should list first 5 nodes
	nodes, _, err := clt.ListNodes(ctx, defaults.Namespace, 5, "")
	require.NoError(t, err)
	require.EqualValues(t, 5, len(nodes))
	expectedNodes := testNodes[:5]
	require.Empty(t, cmp.Diff(expectedNodes, nodes))

	// remove permission for third node
	role.SetNodeLabels(types.Deny, types.Labels{"name": {testNodes[3].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))

	// listing nodes 0-4 should skip the third node and add the fifth to the end.
	nodes, _, err = clt.ListNodes(ctx, defaults.Namespace, 5, "")
	require.NoError(t, err)
	require.EqualValues(t, 5, len(nodes))
	expectedNodes = append(testNodes[:3], testNodes[4:6]...)
	require.Empty(t, cmp.Diff(expectedNodes, nodes))
}

// TestAPILockedOut tests Auth API when there are locks involved.
func TestAPILockedOut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create user, role and client.
	user, role, err := CreateUserAndRole(srv.Auth(), "test-user", nil)
	require.NoError(t, err)
	clt, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Prepare an operation requiring authorization.
	testOp := func() error {
		_, err := clt.GetUser(user.GetName(), false)
		return err
	}

	// With no locks, the operation should pass with no error.
	require.NoError(t, testOp())

	// With a lock targeting the user, the operation should be denied.
	lock, err := types.NewLock("user-lock", types.LockSpecV2{
		Target: types.LockTarget{User: user.GetName()},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().UpsertLock(ctx, lock))
	err = testOp()
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Delete the lock.
	require.NoError(t, srv.Auth().DeleteLock(ctx, lock.GetName()))
	require.NoError(t, testOp())

	// Create a new lock targeting the user's role.
	roleLock, err := types.NewLock("role-lock", types.LockSpecV2{
		Target: types.LockTarget{Role: role.GetName()},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().UpsertLock(ctx, roleLock))
	err = testOp()
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func serverWithAllowRules(t *testing.T, srv *TestAuthServer, allowRules []types.Rule) *ServerWithRoles {
	username := "test-user"
	_, role, err := CreateUserAndRoleWithoutRoles(srv.AuthServer, username, nil)
	require.NoError(t, err)
	role.SetRules(types.Allow, allowRules)
	err = srv.AuthServer.UpsertRole(context.TODO(), role)
	require.NoError(t, err)

	localUser := LocalUser{Username: username, Identity: tlsca.Identity{Username: username}}
	authContext, err := contextForLocalUser(localUser, srv.AuthServer)
	require.NoError(t, err)

	return &ServerWithRoles{
		authServer: srv.AuthServer,
		sessions:   srv.SessionServer,
		alog:       srv.AuditLog,
		context:    *authContext,
	}
}

// TestDatabasesCRUDRBAC verifies RBAC is applied to database CRUD methods.
func TestDatabasesCRUDRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Setup a couple of users:
	// - "dev" only has access to databases with labels env=dev
	// - "admin" has access to all databases
	dev, devRole, err := CreateUserAndRole(srv.Auth(), "dev", nil)
	require.NoError(t, err)
	devRole.SetDatabaseLabels(types.Allow, types.Labels{"env": {"dev"}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, devRole))
	devClt, err := srv.NewClient(TestUser(dev.GetName()))
	require.NoError(t, err)

	admin, adminRole, err := CreateUserAndRole(srv.Auth(), "admin", nil)
	require.NoError(t, err)
	adminRole.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, adminRole))
	adminClt, err := srv.NewClient(TestUser(admin.GetName()))
	require.NoError(t, err)

	// Prepare a couple of database resources.
	devDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:   "dev",
		Labels: map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: libdefaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	adminDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:   "admin",
		Labels: map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: libdefaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// Dev shouldn't be able to create prod database...
	err = devClt.CreateDatabase(ctx, adminDatabase)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can create dev database.
	err = devClt.CreateDatabase(ctx, devDatabase)
	require.NoError(t, err)

	// Admin can create prod database.
	err = adminClt.CreateDatabase(ctx, adminDatabase)
	require.NoError(t, err)

	// Dev shouldn't be able to update prod database...
	err = devClt.UpdateDatabase(ctx, adminDatabase)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can update dev database.
	err = devClt.UpdateDatabase(ctx, devDatabase)
	require.NoError(t, err)

	// Dev shouldn't be able to update labels on the prod database.
	adminDatabase.SetStaticLabels(map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic})
	err = devClt.UpdateDatabase(ctx, adminDatabase)
	require.True(t, trace.IsAccessDenied(err))
	adminDatabase.SetStaticLabels(map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic}) // Reset.

	// Dev shouldn't be able to get prod database...
	_, err = devClt.GetDatabase(ctx, adminDatabase.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can get dev database.
	db, err := devClt.GetDatabase(ctx, devDatabase.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devDatabase, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Admin can get both databases.
	db, err = adminClt.GetDatabase(ctx, adminDatabase.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(adminDatabase, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))
	db, err = adminClt.GetDatabase(ctx, devDatabase.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devDatabase, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// When listing databases, dev should only see one.
	dbs, err := devClt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{devDatabase}, dbs,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Admin should see both.
	dbs, err = adminClt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{adminDatabase, devDatabase}, dbs,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Dev shouldn't be able to delete dev database...
	err = devClt.DeleteDatabase(ctx, adminDatabase.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can delete dev database.
	err = devClt.DeleteDatabase(ctx, devDatabase.GetName())
	require.NoError(t, err)

	// Admin should be able to delete admin database.
	err = adminClt.DeleteDatabase(ctx, adminDatabase.GetName())
	require.NoError(t, err)

	// Create both databases again to test "delete all" functionality.
	require.NoError(t, devClt.CreateDatabase(ctx, devDatabase))
	require.NoError(t, adminClt.CreateDatabase(ctx, adminDatabase))

	// Dev should only be able to delete dev database.
	err = devClt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	dbs, err = adminClt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{adminDatabase}, dbs,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Admin should be able to delete all.
	err = adminClt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	dbs, err = adminClt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Len(t, dbs, 0)
}

func TestGetDatabaseServers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test databases.
	for i := 0; i < 5; i++ {
		name := uuid.New()
		db, err := types.NewDatabaseServerV3(types.Metadata{
			Name:   name,
			Labels: map[string]string{"name": name},
		}, types.DatabaseServerSpecV3{
			Protocol: "postgres",
			URI:      "example.com",
			Hostname: "host",
			HostID:   "hostid",
		})
		require.NoError(t, err)

		_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
		require.NoError(t, err)
	}

	testServers, err := srv.Auth().GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to get the first database
	role.SetDatabaseLabels(types.Allow, types.Labels{"name": {testServers[0].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err := clt.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, 1, len(servers))
	require.Empty(t, cmp.Diff(testServers[0:1], servers))

	// permit user to get all databases
	role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	require.Empty(t, cmp.Diff(testServers, servers))

	// deny user to get the first database
	role.SetDatabaseLabels(types.Deny, types.Labels{"name": {testServers[0].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers[1:]), len(servers))
	require.Empty(t, cmp.Diff(testServers[1:], servers))

	// deny user to get all databases
	role.SetDatabaseLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, 0, len(servers))
	require.Empty(t, cmp.Diff([]types.DatabaseServer{}, servers))
}

// TestGetApplicationServers verifies RBAC is applied when fetching app servers.
func TestGetApplicationServers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test app servers.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("app-%v", i)
		app, err := types.NewAppV3(types.Metadata{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
			types.AppSpecV3{URI: "localhost"})
		require.NoError(t, err)
		server, err := types.NewAppServerV3FromApp(app, "host", "hostid")
		require.NoError(t, err)

		_, err = srv.Auth().UpsertApplicationServer(ctx, server)
		require.NoError(t, err)
	}

	testServers, err := srv.Auth().GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to get the first app
	role.SetAppLabels(types.Allow, types.Labels{"name": {testServers[0].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err := clt.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, 1, len(servers))
	require.Empty(t, cmp.Diff(testServers[0:1], servers))

	// permit user to get all apps
	role.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	require.Empty(t, cmp.Diff(testServers, servers))

	// deny user to get the first app
	role.SetAppLabels(types.Deny, types.Labels{"name": {testServers[0].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers[1:]), len(servers))
	require.Empty(t, cmp.Diff(testServers[1:], servers))

	// deny user to get all apps
	role.SetAppLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, 0, len(servers))
}

func TestGetAppServers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create test apps.
	for i := 0; i < 5; i++ {
		name := uuid.New()
		app, err := types.NewServerWithLabels(
			name,
			types.KindAppServer,
			types.ServerSpecV2{
				Apps: []*types.App{{
					Name:         name,
					StaticLabels: map[string]string{"name": name},
					URI:          "localhost",
				}},
			},
			nil,
		)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertAppServer(ctx, app)
		require.NoError(t, err)
	}

	testServers, err := srv.Auth().GetAppServers(ctx, defaults.Namespace)
	require.NoError(t, err)

	// create user, role, and client
	username := "user"
	user, role, err := CreateUserAndRole(srv.Auth(), username, nil)
	require.NoError(t, err)
	identity := TestUser(user.GetName())
	clt, err := srv.NewClient(identity)
	require.NoError(t, err)

	// permit user to get the first app
	role.SetAppLabels(types.Allow, types.Labels{"name": {testServers[0].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err := clt.GetAppServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	for i := 1; i < len(servers); i++ {
		// servers other than the first should have no apps
		require.Empty(t, servers[i].GetApps())
		// set apps to be equal to compare other fields
		servers[i].SetApps(testServers[i].GetApps())
	}
	require.Empty(t, cmp.Diff(testServers, servers))

	// permit user to get all apps
	role.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetAppServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	require.Empty(t, cmp.Diff(testServers, servers))

	// deny user to get the first app
	role.SetAppLabels(types.Deny, types.Labels{"name": {testServers[0].GetName()}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetAppServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	// first server should have no apps
	require.Empty(t, servers[0].GetApps())
	// set apps to be equal to compare other fields
	servers[0].SetApps(testServers[0].GetApps())
	require.Empty(t, cmp.Diff(testServers, servers))

	// deny user to get all apps
	role.SetAppLabels(types.Deny, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, role))
	servers, err = clt.GetAppServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, len(testServers), len(servers))
	for i := 0; i < len(servers); i++ {
		// servers other than the first should have no apps
		require.Empty(t, servers[i].GetApps())
		// set apps to be equal to compare other fields
		servers[i].SetApps(testServers[i].GetApps())
	}
	require.Empty(t, cmp.Diff(testServers, servers))
}

// TestApps verifies RBAC is applied to app resources.
func TestApps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Setup a couple of users:
	// - "dev" only has access to apps with labels env=dev
	// - "admin" has access to all apps
	dev, devRole, err := CreateUserAndRole(srv.Auth(), "dev", nil)
	require.NoError(t, err)
	devRole.SetAppLabels(types.Allow, types.Labels{"env": {"dev"}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, devRole))
	devClt, err := srv.NewClient(TestUser(dev.GetName()))
	require.NoError(t, err)

	admin, adminRole, err := CreateUserAndRole(srv.Auth(), "admin", nil)
	require.NoError(t, err)
	adminRole.SetAppLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	require.NoError(t, srv.Auth().UpsertRole(ctx, adminRole))
	adminClt, err := srv.NewClient(TestUser(admin.GetName()))
	require.NoError(t, err)

	// Prepare a couple of app resources.
	devApp, err := types.NewAppV3(types.Metadata{
		Name:   "dev",
		Labels: map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)
	adminApp, err := types.NewAppV3(types.Metadata{
		Name:   "admin",
		Labels: map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)

	// Dev shouldn't be able to create prod app...
	err = devClt.CreateApp(ctx, adminApp)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can create dev app.
	err = devClt.CreateApp(ctx, devApp)
	require.NoError(t, err)

	// Admin can create prod app.
	err = adminClt.CreateApp(ctx, adminApp)
	require.NoError(t, err)

	// Dev shouldn't be able to update prod app...
	err = devClt.UpdateApp(ctx, adminApp)
	require.True(t, trace.IsAccessDenied(err))

	// ... but can update dev app.
	err = devClt.UpdateApp(ctx, devApp)
	require.NoError(t, err)

	// Dev shouldn't be able to update labels on the prod app.
	adminApp.SetStaticLabels(map[string]string{"env": "dev", types.OriginLabel: types.OriginDynamic})
	err = devClt.UpdateApp(ctx, adminApp)
	require.True(t, trace.IsAccessDenied(err))
	adminApp.SetStaticLabels(map[string]string{"env": "prod", types.OriginLabel: types.OriginDynamic}) // Reset.

	// Dev shouldn't be able to get prod app...
	_, err = devClt.GetApp(ctx, adminApp.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can get dev app.
	app, err := devClt.GetApp(ctx, devApp.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devApp, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Admin can get both apps.
	app, err = adminClt.GetApp(ctx, adminApp.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(adminApp, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))
	app, err = adminClt.GetApp(ctx, devApp.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(devApp, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// When listing apps, dev should only see one.
	apps, err := devClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{devApp}, apps,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Admin should see both.
	apps, err = adminClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{adminApp, devApp}, apps,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Dev shouldn't be able to delete dev app...
	err = devClt.DeleteApp(ctx, adminApp.GetName())
	require.True(t, trace.IsAccessDenied(err))

	// ... but can delete dev app.
	err = devClt.DeleteApp(ctx, devApp.GetName())
	require.NoError(t, err)

	// Admin should be able to delete admin app.
	err = adminClt.DeleteApp(ctx, adminApp.GetName())
	require.NoError(t, err)

	// Create both apps again to test "delete all" functionality.
	require.NoError(t, devClt.CreateApp(ctx, devApp))
	require.NoError(t, adminClt.CreateApp(ctx, adminApp))

	// Dev should only be able to delete dev app.
	err = devClt.DeleteAllApps(ctx)
	require.NoError(t, err)
	apps, err = adminClt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{adminApp}, apps,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Admin should be able to delete all.
	err = adminClt.DeleteAllApps(ctx)
	require.NoError(t, err)
	apps, err = adminClt.GetApps(ctx)
	require.NoError(t, err)
	require.Len(t, apps, 0)
}

// TestReplaceRemoteLocksRBAC verifies that only a remote proxy may replace the
// remote locks associated with its cluster.
func TestReplaceRemoteLocksRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(srv.AuthServer, "test-user", []string{})
	require.NoError(t, err)

	targetCluster := "cluster"
	tests := []struct {
		desc     string
		identity TestIdentity
		checkErr func(error) bool
	}{
		{
			desc:     "users may not replace remote locks",
			identity: TestUser(user.GetName()),
			checkErr: trace.IsAccessDenied,
		},
		{
			desc:     "local proxy may not replace remote locks",
			identity: TestBuiltin(types.RoleProxy),
			checkErr: trace.IsAccessDenied,
		},
		{
			desc:     "remote proxy of a non-target cluster may not replace the target's remote locks",
			identity: TestRemoteBuiltin(types.RoleProxy, "non-"+targetCluster),
			checkErr: trace.IsAccessDenied,
		},
		{
			desc:     "remote proxy of the target cluster may replace its remote locks",
			identity: TestRemoteBuiltin(types.RoleProxy, targetCluster),
			checkErr: func(err error) bool { return err == nil },
		},
	}

	lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: types.LockTarget{User: "test-user"}})
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			authContext, err := srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, test.identity.I))
			require.NoError(t, err)

			s := &ServerWithRoles{
				authServer: srv.AuthServer,
				sessions:   srv.SessionServer,
				alog:       srv.AuditLog,
				context:    *authContext,
			}

			err = s.ReplaceRemoteLocks(ctx, targetCluster, []types.Lock{lock})
			require.True(t, test.checkErr(err), trace.DebugReport(err))
		})
	}
}

// TestKindClusterConfig verifies that types.KindClusterConfig can be used
// as an alternative privilege to provide access to cluster configuration
// resources.
func TestKindClusterConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv, err := NewTestAuthServer(TestAuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)

	getClusterConfigResources := func(ctx context.Context, user types.User) []error {
		authContext, err := srv.Authorizer.Authorize(context.WithValue(ctx, ContextUser, TestUser(user.GetName()).I))
		require.NoError(t, err, trace.DebugReport(err))
		s := &ServerWithRoles{
			authServer: srv.AuthServer,
			sessions:   srv.SessionServer,
			alog:       srv.AuditLog,
			context:    *authContext,
		}
		_, err1 := s.GetClusterAuditConfig(ctx)
		_, err2 := s.GetClusterNetworkingConfig(ctx)
		_, err3 := s.GetSessionRecordingConfig(ctx)
		return []error{err1, err2, err3}
	}

	t.Run("without KindClusterConfig privilege", func(t *testing.T) {
		user, err := CreateUser(srv.AuthServer, "test-user")
		require.NoError(t, err)
		for _, err := range getClusterConfigResources(ctx, user) {
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		}
	})

	t.Run("with KindClusterConfig privilege", func(t *testing.T) {
		role, err := types.NewRole("test-role", types.RoleSpecV4{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindClusterConfig, []string{types.VerbRead}),
				},
			},
		})
		require.NoError(t, err)
		user, err := CreateUser(srv.AuthServer, "test-user", role)
		require.NoError(t, err)
		for _, err := range getClusterConfigResources(ctx, user) {
			require.NoError(t, err)
		}
	})
}
