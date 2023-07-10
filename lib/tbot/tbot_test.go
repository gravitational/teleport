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
		databaseServiceName = "test-database-service"
		databaseUsername    = "test-database-username"
		databaseName        = "test-database"
		kubeClusterName     = "test-kube-cluster"
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
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: databaseServiceName,
	}, types.DatabaseSpecV3{
		Protocol: "mysql",
		URI:      "example.com:1234",
	})
	require.NoError(t, err)
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
	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: kubeClusterName,
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
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
	require.NoError(t, rootClient.UpsertRole(ctx, role))
	// Create a blank secondary role that we can use to check that the default
	// behavior of impersonating all roles available works
	role, err = types.NewRole(secondaryRole, types.RoleSpecV6{})
	require.NoError(t, err)
	require.NoError(t, rootClient.UpsertRole(ctx, role))

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
	kubeOutput := &config.KubernetesOutput{
		// DestinationDirectory required or output will fail.
		Destination: &config.DestinationDirectory{
			Path: t.TempDir(),
		},
		KubernetesCluster: kubeClusterName,
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
			sshHostOutput,
			kubeOutput,
		},
	)
	b := New(botConfig, log)
	require.NoError(t, b.Run(ctx))

	t.Run("bot identity", func(t *testing.T) {
		// Some rough checks to ensure the bot identity used follows our
		// expected rules for bot identities.
		botIdent := b.ident()
		tlsIdent, err := tlsca.FromSubject(botIdent.X509Cert.Subject, botIdent.X509Cert.NotAfter)
		require.NoError(t, err)
		require.True(t, tlsIdent.Renewable)
		require.False(t, tlsIdent.DisallowReissue)
		require.Equal(t, uint64(1), tlsIdent.Generation)
		require.ElementsMatch(t, []string{botParams.RoleName}, tlsIdent.Groups)
	})

	t.Run("output: identity", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(t, identityOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
	})

	t.Run("output: identity with role specified", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(t, identityOutputWithRoles.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, []string{mainRole}, botParams.UserName)
	})

	t.Run("output: kubernetes", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(t, kubeOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		require.Equal(t, kubeClusterName, tlsIdent.KubernetesCluster)
		require.Equal(t, kubeGroups, tlsIdent.KubernetesGroups)
		require.Equal(t, kubeUsers, tlsIdent.KubernetesUsers)
	})

	t.Run("output: application", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(t, appOutput.GetDestination())
		requireValidOutputTLSIdent(t, tlsIdent, defaultRoles, botParams.UserName)
		route := tlsIdent.RouteToApp
		require.Equal(t, appName, route.Name)
		require.Equal(t, "test-app.example.com", route.PublicAddr)
		require.NotEmpty(t, route.SessionID)
	})

	t.Run("output: database", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(t, dbOutput.GetDestination())
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
		hostKeyBytes, err := dest.Read("ssh_host")
		require.NoError(t, err)
		hostKey, err := ssh.ParsePrivateKey(hostKeyBytes)
		require.NoError(t, err)
		testData := []byte("test-data")
		signedTestData, err := hostKey.Sign(rand.Reader, testData)
		require.NoError(t, err)

		// Validate ssh_host-cert.pub
		hostCertBytes, err := dest.Read("ssh_host-cert.pub")
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
		userCABytes, err := dest.Read("ssh_host-user-ca.pub")
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

func tlsIdentFromDest(t *testing.T, dest bot.Destination) *tlsca.Identity {
	t.Helper()
	keyBytes, err := dest.Read(identity.PrivateKeyKey)
	require.NoError(t, err)
	certBytes, err := dest.Read(identity.TLSCertKey)
	require.NoError(t, err)
	hostCABytes, err := dest.Read(config.HostCAPath)
	require.NoError(t, err)
	ident := &identity.Identity{}
	err = identity.ReadTLSIdentityFromKeyPair(ident, keyBytes, certBytes, [][]byte{hostCABytes})
	require.NoError(t, err)

	tlsIdent, err := tlsca.FromSubject(
		ident.X509Cert.Subject, ident.X509Cert.NotAfter,
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

	botConfig := testhelpers.DefaultBotConfig(t, fc, botParams, []config.Output{})
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
