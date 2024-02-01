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

package tbot

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/native"
	apisshutils "github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	native.PrecomputeTestKeys(m)
	os.Exit(m.Run())
}

// TestBot is a one-shot run of the bot that communicates with a stood up
// in memory auth server.
//
// This test suite should ensure that outputs result in credentials with the
// expected attributes. The exact format of rendered templates is a concern
// that should be tested at a lower level. Generally assume that the auth server
// has good behavior (e.g is enforcing rbac correctly) and avoid testing cases
// such as the bot not having a role granting access to a resource.
func TestBot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewLoggerForTests()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	const (
		fakeHostname        = "test-host"
		fakeHostID          = "uuid"
		appName             = "test-app"
		databaseServiceName = "test-database-service-rds-us-west-1-123456789012"
		databaseUsername    = "test-database-username"
		databaseName        = "test-database"
		kubeClusterName     = "test-kube-cluster-eks-us-west-1-12345679012"
		// fake some "auto-discovered" and renamed resources.
		databaseServiceDiscoveredName = "test-database-service"
		kubeClusterDiscoveredName     = "test-kube-cluster"
	)

	clusterName := string(fc.Auth.ClusterName)
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Register an application server so the bot can request certs for it.
	app, err := types.NewAppV3(types.Metadata{
		Name: appName,
	}, types.AppSpecV3{
		PublicAddr: "test-app.example.com",
		URI:        "http://test-app.example.com:1234",
	})
	require.NoError(t, err)
	appServer, err := types.NewAppServerV3FromApp(app, fakeHostname, fakeHostID)
	require.NoError(t, err)
	_, err = rootClient.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)
	// Register a database server so the bot can request certs for it.
	db := newMockDiscoveredDB(t, databaseServiceName, databaseServiceDiscoveredName)
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: databaseServiceName,
	}, types.DatabaseServerSpecV3{
		HostID:   fakeHostID,
		Hostname: fakeHostname,
		Database: db,
	})
	require.NoError(t, err)
	_, err = rootClient.UpsertDatabaseServer(ctx, dbServer)
	require.NoError(t, err)
	// Register a kubernetes server so the bot can request certs for it.
	kubeCluster := newMockDiscoveredKubeCluster(t, kubeClusterName, kubeClusterDiscoveredName)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, fakeHostname, fakeHostID)
	require.NoError(t, err)
	_, err = rootClient.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	// Fetch CAs from auth server to compare to artifacts later
	hostCA, err := rootClient.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	require.NoError(t, err)
	userCA, err := rootClient.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: clusterName,
	}, false)
	require.NoError(t, err)

	var (
		mainRole      = "main-role"
		secondaryRole = "secondary-role"
		defaultRoles  = []string{mainRole, secondaryRole}
		hostPrincipal = "node.example.com"
		kubeGroups    = []string{"system:masters"}
		kubeUsers     = []string{"kubernetes-user"}
	)
	hostCertRule := types.NewRule("host_cert", []string{"create"})
	hostCertRule.Where = fmt.Sprintf("is_subset(host_cert.principals, \"%s\")", hostPrincipal)
	role, err := types.NewRole(mainRole, types.RoleSpecV6{
		Allow: types.RoleConditions{
			// Grant access to all apps
			AppLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},

			// Grant access to all kubernetes clusters
			KubernetesLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			KubeGroups: kubeGroups,
			KubeUsers:  kubeUsers,

			// Grant access to database
			// Note: we don't actually need a role granting us database access to
			// request it. Actual access is validated via RBAC at connection time.
			// We do need an actual database and permission to list them, however.
			DatabaseLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			DatabaseNames: []string{databaseName},
			DatabaseUsers: []string{databaseUsername},
			Rules: []types.Rule{
				types.NewRule("db_server", []string{"read", "list"}),
				// Grant ability to generate a host cert
				hostCertRule,
			},
		},
	})
	require.NoError(t, err)
	_, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)
	// Create a blank secondary role that we can use to check that the default
	// behavior of impersonating all roles available works
	role, err = types.NewRole(secondaryRole, types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, rootClient, "test", defaultRoles...)

	identityOutput := &config.IdentityOutput{
		Destination: &config.DestinationMemory{},
	}
	identityOutputWithRoles := &config.IdentityOutput{
		Destination: &config.DestinationMemory{},
		Roles:       []string{mainRole},
	}
	appOutput := &config.ApplicationOutput{
		Destination: &config.DestinationMemory{},
		AppName:     appName,
	}
	dbOutput := &config.DatabaseOutput{
		Destination: &config.DestinationMemory{},
		Service:     databaseServiceName,
		Database:    databaseName,
		Username:    databaseUsername,
	}
	dbDiscoveredNameOutput := &config.DatabaseOutput{
		Destination: &config.DestinationMemory{},
		Service:     databaseServiceDiscoveredName,
		Database:    databaseName,
		Username:    databaseUsername,
	}
	kubeOutput := &config.KubernetesOutput{
		// DestinationDirectory required or output will fail.
		Destination: &config.DestinationDirectory{
			Path: t.TempDir(),
		},
		KubernetesCluster: kubeClusterName,
	}
	kubeDiscoveredNameOutput := &config.KubernetesOutput{
		// DestinationDirectory required or output will fail.
		Destination: &config.DestinationDirectory{
			Path: t.TempDir(),
		},
		KubernetesCluster: kubeClusterDiscoveredName,
	}
	sshHostOutput := &config.SSHHostOutput{
		Destination: &config.DestinationMemory{},
		Principals:  []string{hostPrincipal},
	}
	botConfig := testhelpers.DefaultBotConfig(
		t, fc, botParams, []config.Output{
			identityOutput,
			identityOutputWithRoles,
			appOutput,
			dbOutput,
			dbDiscoveredNameOutput,
			sshHostOutput,
			kubeOutput,
			kubeDiscoveredNameOutput,
		},
		testhelpers.DefaultBotConfigOpts{
			UseAuthServer: true,
			Insecure:      true,
		},
	)
	b := New(botConfig, log)
	require.NoError(t, b.Run(ctx))

	t.Run("bot identity", func(t *testing.T) {
		// Some rough checks to ensure the bot identity used follows our
		// expected rules for bot identities.
		botIdent := b.BotIdentity()
		tlsIdent, err := tlsca.FromSubject(
			botIdent.X509Cert.Subject, botIdent.X509Cert.NotAfter,
		)
		require.NoError(t, err)
		require.True(t, tlsIdent.Renewable)
		require.False(t, tlsIdent.DisallowReissue)
		require.Equal(t, uint64(1), tlsIdent.Generation)
		require.ElementsMatch(t, []string{botParams.RoleName}, tlsIdent.Groups)
	})

	t.Run("output: identity", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, identityOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
	})

	t.Run("output: identity with role specified", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, identityOutputWithRoles.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, []string{mainRole}, botParams.UserName)
	})

	t.Run("output: kubernetes", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, kubeOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		require.Equal(t, kubeClusterName, tlsIdent.KubernetesCluster)
		require.Equal(t, kubeGroups, tlsIdent.KubernetesGroups)
		require.Equal(t, kubeUsers, tlsIdent.KubernetesUsers)
	})

	t.Run("output: kubernetes discovered name", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, kubeDiscoveredNameOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		require.Equal(t, kubeClusterName, tlsIdent.KubernetesCluster)
		require.Equal(t, kubeGroups, tlsIdent.KubernetesGroups)
		require.Equal(t, kubeUsers, tlsIdent.KubernetesUsers)
	})

	t.Run("output: application", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, appOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		route := tlsIdent.RouteToApp
		require.Equal(t, appName, route.Name)
		require.Equal(t, "test-app.example.com", route.PublicAddr)
		require.NotEmpty(t, route.SessionID)
	})

	t.Run("output: database", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, dbOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		route := tlsIdent.RouteToDatabase
		require.Equal(t, databaseServiceName, route.ServiceName)
		require.Equal(t, databaseName, route.Database)
		require.Equal(t, databaseUsername, route.Username)
		require.Equal(t, "mysql", route.Protocol)
	})

	t.Run("output: database discovered name", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, dbDiscoveredNameOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		route := tlsIdent.RouteToDatabase
		require.Equal(t, databaseServiceName, route.ServiceName)
		require.Equal(t, databaseName, route.Database)
		require.Equal(t, databaseUsername, route.Username)
		require.Equal(t, "mysql", route.Protocol)
	})

	t.Run("output: ssh_host", func(t *testing.T) {
		dest := sshHostOutput.GetDestination()

		// Validate ssh_host
		hostKeyBytes, err := dest.Read(ctx, "ssh_host")
		require.NoError(t, err)
		hostKey, err := ssh.ParsePrivateKey(hostKeyBytes)
		require.NoError(t, err)
		testData := []byte("test-data")
		signedTestData, err := hostKey.Sign(rand.Reader, testData)
		require.NoError(t, err)

		// Validate ssh_host-cert.pub
		hostCertBytes, err := dest.Read(ctx, "ssh_host-cert.pub")
		require.NoError(t, err)
		hostCert, err := sshutils.ParseCertificate(hostCertBytes)
		require.NoError(t, err)

		// Check cert is signed by host CA, and that the host key can sign things
		// which can be verified with the host cert.
		publicKeys, err := apisshutils.GetCheckers(hostCA)
		require.NoError(t, err)
		hostCertChecker := ssh.CertChecker{
			IsHostAuthority: func(v ssh.PublicKey, _ string) bool {
				for _, pk := range publicKeys {
					return bytes.Equal(v.Marshal(), pk.Marshal())
				}
				return false
			},
		}
		require.NoError(t, hostCertChecker.CheckCert(hostPrincipal, hostCert), "host cert does not pass verification")
		require.NoError(t, hostCert.Key.Verify(testData, signedTestData), "signature by host key does not verify with public key in host certificate")

		// Validate ssh_host-user-ca.pub
		userCABytes, err := dest.Read(ctx, "ssh_host-user-ca.pub")
		require.NoError(t, err)
		userCAKey, _, _, _, err := ssh.ParseAuthorizedKey(userCABytes)
		require.NoError(t, err)
		matchesUserCA := false
		for _, trustedKeyPair := range userCA.GetTrustedSSHKeyPairs() {
			wantUserCAKey, _, _, _, err := ssh.ParseAuthorizedKey(trustedKeyPair.PublicKey)
			require.NoError(t, err)
			if bytes.Equal(userCAKey.Marshal(), wantUserCAKey.Marshal()) {
				matchesUserCA = true
				break
			}
		}
		require.True(t, matchesUserCA)
	})
}

// requireValidOutputTLSIdent runs general validation against the TLS identity
// created by a normal output. This ensures several key parts of the identity
// have sane values.
func requireValidOutputTLSIdent(t *testing.T, ident *tlsca.Identity, wantRoles []string, botUsername string) {
	require.True(t, ident.DisallowReissue)
	require.False(t, ident.Renewable)
	require.Equal(t, botUsername, ident.Impersonator)
	require.Equal(t, botUsername, ident.Username)
	require.Equal(t, wantRoles, ident.Groups)
}

func tlsIdentFromDest(ctx context.Context, t *testing.T, dest bot.Destination) *tlsca.Identity {
	t.Helper()
	keyBytes, err := dest.Read(ctx, identity.PrivateKeyKey)
	require.NoError(t, err)
	certBytes, err := dest.Read(ctx, identity.TLSCertKey)
	require.NoError(t, err)
	hostCABytes, err := dest.Read(ctx, config.HostCAPath)
	require.NoError(t, err)
	_, x509Cert, _, _, err := identity.ParseTLSIdentity(keyBytes, certBytes, [][]byte{hostCABytes})
	require.NoError(t, err)
	tlsIdent, err := tlsca.FromSubject(
		x509Cert.Subject, x509Cert.NotAfter,
	)
	require.NoError(t, err)
	return tlsIdent
}

// TestBot_ResumeFromStorage ensures that after the bot stops, another instance
// of the bot can be started from the state persisted to the storage
// destination. This ensures that the renewable token join method will function
// correctly.
func TestBot_ResumeFromStorage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewLoggerForTests()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Create bot user and join token
	botParams := testhelpers.MakeBot(t, rootClient, "test", "access")

	botConfig := testhelpers.DefaultBotConfig(t, fc, botParams, []config.Output{},
		testhelpers.DefaultBotConfigOpts{
			UseAuthServer: true,
			Insecure:      true,
		},
	)

	// Use a destination directory to ensure locking behaves correctly and
	// the bot isn't left in a locked state.
	directoryDest := &config.DestinationDirectory{
		Path:     t.TempDir(),
		Symlinks: botfs.SymlinksInsecure,
		ACLs:     botfs.ACLOff,
	}
	botConfig.Storage.Destination = directoryDest

	// Run the bot a first time
	firstBot := New(botConfig, log)
	require.NoError(t, firstBot.Run(ctx))

	// Run the bot a second time, with the exact same config
	secondBot := New(botConfig, log)
	require.NoError(t, secondBot.Run(ctx))

	// Simulate user removing token from config, and run the bot a third time.
	// It should see it already has a valid identity and use that - ignoring
	// the fact that the token has been cleared from config.
	botConfig.Onboarding.TokenValue = ""
	thirdBot := New(botConfig, log)
	require.NoError(t, thirdBot.Run(ctx))
}

func TestBot_InsecureViaProxy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewLoggerForTests()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Create bot user and join token
	botParams := testhelpers.MakeBot(t, rootClient, "test", "access")

	botConfig := testhelpers.DefaultBotConfig(t, fc, botParams, []config.Output{},
		testhelpers.DefaultBotConfigOpts{
			UseAuthServer: false,
			Insecure:      true,
		},
	)
	// Use a destination directory to ensure locking behaves correctly and
	// the bot isn't left in a locked state.
	directoryDest := &config.DestinationDirectory{
		Path:     t.TempDir(),
		Symlinks: botfs.SymlinksInsecure,
		ACLs:     botfs.ACLOff,
	}
	botConfig.Storage.Destination = directoryDest

	// Run the bot a first time
	firstBot := New(botConfig, log)
	require.NoError(t, firstBot.Run(ctx))
}

func TestChooseOneResource(t *testing.T) {
	t.Parallel()
	t.Run("database", testChooseOneDatabase)
	t.Run("kube cluster", testChooseOneKubeCluster)
}

func testChooseOneDatabase(t *testing.T) {
	t.Parallel()
	fooDB1 := newMockDiscoveredDB(t, "foo-rds-us-west-1-123456789012", "foo")
	fooDB2 := newMockDiscoveredDB(t, "foo-rds-us-west-2-123456789012", "foo")
	barDB := newMockDiscoveredDB(t, "bar-rds-us-west-1-123456789012", "bar")
	tests := []struct {
		desc      string
		databases []types.Database
		dbSvc     string
		wantDB    types.Database
		wantErr   string
	}{
		{
			desc:      "by exact name match",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "bar-rds-us-west-1-123456789012",
			wantDB:    barDB,
		},
		{
			desc:      "by unambiguous discovered name match",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "bar",
			wantDB:    barDB,
		},
		{
			desc:      "ambiguous discovered name matches is an error",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "foo",
			wantErr:   `"foo" matches multiple auto-discovered databases`,
		},
		{
			desc:      "no match is an error",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "xxx",
			wantErr:   `database "xxx" not found`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gotDB, err := chooseOneDatabase(test.databases, test.dbSvc)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantDB, gotDB)
		})
	}
}

func testChooseOneKubeCluster(t *testing.T) {
	fooKube1 := newMockDiscoveredKubeCluster(t, "foo-eks-us-west-1-123456789012", "foo")
	fooKube2 := newMockDiscoveredKubeCluster(t, "foo-eks-us-west-2-123456789012", "foo")
	barKube := newMockDiscoveredKubeCluster(t, "bar-eks-us-west-1-123456789012", "bar")
	tests := []struct {
		desc            string
		clusters        []types.KubeCluster
		kubeSvc         string
		wantKubeCluster types.KubeCluster
		wantErr         string
	}{
		{
			desc:            "by exact name match",
			clusters:        []types.KubeCluster{fooKube1, fooKube2, barKube},
			kubeSvc:         "bar-eks-us-west-1-123456789012",
			wantKubeCluster: barKube,
		},
		{
			desc:            "by unambiguous discovered name match",
			clusters:        []types.KubeCluster{fooKube1, fooKube2, barKube},
			kubeSvc:         "bar",
			wantKubeCluster: barKube,
		},
		{
			desc:     "ambiguous discovered name matches is an error",
			clusters: []types.KubeCluster{fooKube1, fooKube2, barKube},
			kubeSvc:  "foo",
			wantErr:  `"foo" matches multiple auto-discovered kubernetes clusters`,
		},
		{
			desc:     "no match is an error",
			clusters: []types.KubeCluster{fooKube1, fooKube2, barKube},
			kubeSvc:  "xxx",
			wantErr:  `kubernetes cluster "xxx" not found`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gotKube, err := chooseOneKubeCluster(test.clusters, test.kubeSvc)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantKubeCluster, gotKube)
		})
	}
}

func newMockDiscoveredDB(t *testing.T, name, discoveredName string) *types.DatabaseV3 {
	t.Helper()
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
		Labels: map[string]string{
			types.OriginLabel:         types.OriginCloud,
			types.DiscoveredNameLabel: discoveredName,
		},
	}, types.DatabaseSpecV3{
		Protocol: "mysql",
		URI:      "example.com:1234",
	})
	require.NoError(t, err)
	return db
}

func newMockDiscoveredKubeCluster(t *testing.T, name, discoveredName string) *types.KubernetesClusterV3 {
	t.Helper()
	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel:         types.OriginCloud,
				types.DiscoveredNameLabel: discoveredName,
			},
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	return kubeCluster
}
