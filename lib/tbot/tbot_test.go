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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/gravitational/teleport/api/client"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	apisshutils "github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	cryptosuites.PrecomputeRSATestKeys(m)
	os.Exit(m.Run())
}

type defaultBotConfigOpts struct {
	// Makes the bot connect via the Auth Server instead of the Proxy server.
	useAuthServer bool
	// Makes the bot accept an insecure auth or proxy server
	insecure bool
}

func defaultTestServerOpts(t *testing.T, log *slog.Logger) testenv.TestServerOptFunc {
	return func(o *testenv.TestServersOpts) {
		testenv.WithClusterName(t, "root")(o)
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Logger = log
			cfg.Proxy.PublicAddrs = []utils.NetAddr{
				{AddrNetwork: "tcp", Addr: net.JoinHostPort("localhost", strconv.Itoa(cfg.Proxy.WebAddr.Port(0)))},
			}
			cfg.Proxy.TunnelPublicAddrs = []utils.NetAddr{
				cfg.Proxy.ReverseTunnelListenAddr,
			}
		})(o)
	}
}

// makeBot creates a server-side bot and returns joining parameters.
func makeBot(t *testing.T, client *authclient.Client, name string, roles ...string) (*config.OnboardingConfig, *machineidv1pb.Bot) {
	ctx := context.TODO()
	t.Helper()

	b, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: roles,
			},
		},
	})
	require.NoError(t, err)

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	require.NoError(t, err)
	tok, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(10*time.Minute),
		types.ProvisionTokenSpecV2{
			Roles:   []types.SystemRole{types.RoleBot},
			BotName: b.Metadata.Name,
		})
	require.NoError(t, err)
	err = client.CreateToken(ctx, tok)
	require.NoError(t, err)

	return &config.OnboardingConfig{
		TokenValue: tok.GetName(),
		JoinMethod: types.JoinMethodToken,
	}, b
}

// defaultBotConfig creates a usable bot config from joining parameters.
// By default it:
// - Has the outputs provided to it via the parameter `outputs`
// - Runs in oneshot mode
// - Uses a memory storage destination
// - Does not verify Proxy WebAPI certificates
func defaultBotConfig(
	t *testing.T,
	process *service.TeleportProcess,
	onboarding *config.OnboardingConfig,
	serviceConfigs config.ServiceConfigs,
	opts defaultBotConfigOpts,
) *config.BotConfig {
	t.Helper()

	var authServer = process.Config.Proxy.WebAddr.String()
	if opts.useAuthServer {
		authServer = process.Config.AuthServerAddresses()[0].String()
	}

	cfg := &config.BotConfig{
		AuthServer:            authServer,
		AuthServerAddressMode: config.WarnIfAuthServerIsProxy,
		Onboarding:            *onboarding,
		Storage: &config.StorageConfig{
			Destination: &config.DestinationMemory{},
		},
		Oneshot: true,
		// Set insecure so the bot will trust the Proxy's webapi default signed
		// certs.
		Insecure: opts.insecure,
		Services: serviceConfigs,
	}

	require.NoError(t, cfg.CheckAndSetDefaults())
	return cfg
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
	log := utils.NewSlogLoggerForTests()

	// Make a new auth server.
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

	process := testenv.MakeTestServer(
		t,
		defaultTestServerOpts(t, log),
		testenv.WithProxyKube(t),
	)
	rootClient := testenv.MakeDefaultAuthClient(t, process)
	clusterName := process.Config.Auth.ClusterName.GetClusterName()

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
	botParams, botResource := makeBot(
		t, rootClient, "test", defaultRoles...,
	)

	identityOutput := &config.IdentityOutput{
		Destination: &config.DestinationMemory{},
	}
	identityOutputWithRoles := &config.IdentityOutput{
		Destination: &config.DestinationMemory{},
		Roles:       []string{mainRole},
	}
	identityOutputWithReissue := &config.IdentityOutput{
		Destination:  &config.DestinationMemory{},
		AllowReissue: true,
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
	botConfig := defaultBotConfig(
		t, process, botParams, config.ServiceConfigs{
			identityOutput,
			identityOutputWithRoles,
			appOutput,
			dbOutput,
			dbDiscoveredNameOutput,
			sshHostOutput,
			kubeOutput,
			kubeDiscoveredNameOutput,
			identityOutputWithReissue,
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
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
		require.ElementsMatch(t, []string{botResource.Status.RoleName}, tlsIdent.Groups)
		// testenv cluster uses balanced-v1 suite, sanity check we generated an
		// ECDSA key.
		require.IsType(t, &ecdsa.PublicKey{}, botIdent.PrivateKey.Public())
	})

	t.Run("output: identity", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, identityOutput.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, false,
		)
	})

	t.Run("output: identity with role specified", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, identityOutputWithRoles.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, []string{mainRole}, botResource.Status.UserName, false,
		)
	})

	t.Run("output: identity with allow reissue", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, identityOutputWithReissue.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, true,
		)
	})

	t.Run("output: kubernetes", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, kubeOutput.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, false,
		)
		require.Equal(t, kubeClusterName, tlsIdent.KubernetesCluster)
		require.Equal(t, kubeGroups, tlsIdent.KubernetesGroups)
		require.Equal(t, kubeUsers, tlsIdent.KubernetesUsers)
	})

	t.Run("output: kubernetes discovered name", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, kubeDiscoveredNameOutput.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, false,
		)
		require.Equal(t, kubeClusterName, tlsIdent.KubernetesCluster)
		require.Equal(t, kubeGroups, tlsIdent.KubernetesGroups)
		require.Equal(t, kubeUsers, tlsIdent.KubernetesUsers)
	})

	t.Run("output: application", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, appOutput.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, false,
		)
		route := tlsIdent.RouteToApp
		require.Equal(t, appName, route.Name)
		require.Equal(t, "test-app.example.com", route.PublicAddr)
		require.NotEmpty(t, route.SessionID)
	})

	t.Run("output: database", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, dbOutput.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, false,
		)
		route := tlsIdent.RouteToDatabase
		require.Equal(t, databaseServiceName, route.ServiceName)
		require.Equal(t, databaseName, route.Database)
		require.Equal(t, databaseUsername, route.Username)
		require.Equal(t, "mysql", route.Protocol)
		// Sanity check we generated an RSA key.
		keyBytes, err := dbOutput.GetDestination().Read(ctx, identity.PrivateKeyKey)
		require.NoError(t, err)
		key, err := keys.ParsePrivateKey(keyBytes)
		require.NoError(t, err)
		require.IsType(t, &rsa.PublicKey{}, key.Public())
	})

	t.Run("output: database discovered name", func(t *testing.T) {
		tlsIdent := tlsIdentFromDest(ctx, t, dbDiscoveredNameOutput.GetDestination())
		requireValidOutputTLSIdent(
			t, tlsIdent, defaultRoles, botResource.Status.UserName, false,
		)
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
		// testenv cluster uses balanced-v1 suite, sanity check we generated an
		// Ed25519 key.
		require.Equal(t, ssh.KeyAlgoED25519, hostKey.PublicKey().Type())
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
func requireValidOutputTLSIdent(t *testing.T, ident *tlsca.Identity, wantRoles []string, botUsername string, allowReissue bool) {
	require.Equal(t, !allowReissue, ident.DisallowReissue)
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
	_, tlsIdent, _, _, _, err := identity.ParseTLSIdentity(keyBytes, certBytes, [][]byte{hostCABytes})
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
	log := utils.NewSlogLoggerForTests()

	// Make a new auth server.
	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create bot user and join token
	botParams, _ := makeBot(t, rootClient, "test", "access")

	botConfig := defaultBotConfig(t, process, botParams, config.ServiceConfigs{},
		defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
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

func TestBot_IdentityRenewalFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	// This test asserts that we can continue running (and recover) even when
	// identity renewal fails on-startup.
	//
	// How it works:
	//
	// We run a TCP proxy in front of the real Teleport proxy that can be set to
	// immediately close connections.
	//
	// We publish that address in TunnelPublicAddrs but put the *real* address
	// in tbot's config so that we allow requests to the `/webapi/find` endpoint
	// through as-is, without needing to sniff the ALPN protocols or something.
	//
	// This is necessary because tbot's resolver caches errors, so if we blocked
	// the connection to that HTTP endpoint, we'd need to wait for the cache to
	// expire.
	proxy := newFailureProxy(t)

	// Make a new auth server.
	process := testenv.MakeTestServer(t,
		func(o *testenv.TestServersOpts) {
			defaultTestServerOpts(t, log)(o)

			testenv.WithConfig(func(cfg *servicecfg.Config) {
				cfg.Proxy.TunnelPublicAddrs = []utils.NetAddr{*utils.MustParseAddr(proxy.addr())}
			})(o)
		},
	)
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create bot user and join token
	botParams, _ := makeBot(t, rootClient, "test", "access")
	botConfig := defaultBotConfig(t,
		process,
		botParams,
		config.ServiceConfigs{},
		defaultBotConfigOpts{insecure: true},
	)

	dest := &config.DestinationDirectory{
		Path:     t.TempDir(),
		Symlinks: botfs.SymlinksInsecure,
		ACLs:     botfs.ACLOff,
	}
	botConfig.Storage.Destination = dest

	// Configure our failure proxy to send traffic to the real proxy
	tunnelAddr, err := process.ProxyTunnelAddr()
	require.NoError(t, err)
	proxy.dst = tunnelAddr.String()
	go proxy.run(t)

	botConfig.ProxyServer = tunnelAddr.String()
	botConfig.AuthServer = ""

	// Run the bot a first time
	firstBot := New(botConfig, log)
	require.NoError(t, firstBot.Run(ctx))

	// Block connections. Running the bot should now fail.
	proxy.setFailing(true)
	secondBot := New(botConfig, log)
	require.ErrorContains(t, secondBot.Run(ctx), "Error while dialing")

	// Drain the notification channel so we can listen for new connections.
	select {
	case <-proxy.notif:
	default:
	}

	// Run it again in long-running mode, and it should eventually succeed once
	// the network partition has healed.
	botConfig.Oneshot = false
	outputDest := newWriteNotifier(&config.DestinationMemory{})
	require.NoError(t, outputDest.CheckAndSetDefaults())
	botConfig.Services = append(botConfig.Services, &config.IdentityOutput{
		Destination: outputDest,
	})
	thirdBot := New(botConfig, log)

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go thirdBot.Run(ctx)

	// Wait for at least one failed connection, then heal the network partition.
	select {
	case <-proxy.notif:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for connection")
	}

	// Wait for the client to be available.
	var client *client.Client
	require.Eventually(t, func() bool {
		client = thirdBot.Client()
		return client != nil
	}, 5*time.Second, 100*time.Millisecond, "timeout waiting for client to become available")

	t.Log("Healing network partition")
	proxy.setFailing(false)

	// We must reset gRPC's connection backoff, otherwise it'll keep returning
	// the cached error.
	client.GetConnection().ResetConnectBackoff()

	// Wait for the destination to be written to.
	select {
	case <-outputDest.ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for output to be written")
	}
}

func newWriteNotifier(dst bot.Destination) *writeNotifier {
	return &writeNotifier{
		Destination: dst,
		ch:          make(chan struct{}, 1),
	}
}

type writeNotifier struct {
	bot.Destination

	ch chan struct{}
}

func (w writeNotifier) Write(ctx context.Context, name string, data []byte) error {
	if name != identity.WriteTestKey {
		defer func() {
			select {
			case w.ch <- struct{}{}:
			default:
			}
		}()
	}
	return w.Destination.Write(ctx, name, data)
}

type failureProxy struct {
	dst     string
	lis     net.Listener
	failing atomic.Bool
	notif   chan struct{}
}

func newFailureProxy(t *testing.T) *failureProxy {
	t.Helper()

	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Logf("failed to close listener: %v", err)
		}
	})

	return &failureProxy{
		lis:   lis,
		notif: make(chan struct{}, 1),
	}
}

func (f *failureProxy) run(t *testing.T) {
	t.Helper()

	for {
		conn, err := f.lis.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			t.Logf("accept failed: %v", err)
			continue
		}

		select {
		case f.notif <- struct{}{}:
		default:
		}

		go f.handleConn(t, conn)
	}
}

func (f *failureProxy) handleConn(t *testing.T, conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}()

	// If we're failing, just close the connection immediately.
	if f.failing.Load() {
		return
	}

	upstream, err := net.Dial("tcp", f.dst)
	if err != nil {
		t.Logf("failed to dial upstream: %v", err)
		return
	}

	done := make(chan struct{}, 1)
	pipe := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}

	go pipe(upstream, conn)
	go pipe(conn, upstream)

	<-done
}

func (f *failureProxy) addr() string {
	return f.lis.Addr().String()
}

func (f *failureProxy) setFailing(failing bool) {
	f.failing.Store(failing)
}

func TestBot_InsecureViaProxy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	// Make a new auth server.
	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create bot user and join token
	botParams, _ := makeBot(t, rootClient, "test", "access")

	botConfig := defaultBotConfig(t, process, botParams, config.ServiceConfigs{},
		defaultBotConfigOpts{
			useAuthServer: false,
			insecure:      true,
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

func TestBotDatabaseTunnel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	// Make a new auth server.
	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Make fake postgres server and add a database access instance to expose
	// it.
	pts, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: rootClient,
		Users:      []string{"llama"},
	})
	require.NoError(t, err)
	go func() {
		t.Logf("Postgres Fake server running at %s port", pts.Port())
		require.NoError(t, pts.Serve())
	}()
	t.Cleanup(func() {
		pts.Close()
	})
	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)
	helpers.MakeTestDatabaseServer(t, *proxyAddr, testenv.StaticToken, nil, servicecfg.Database{
		Name:     "test-database",
		URI:      net.JoinHostPort("localhost", pts.Port()),
		Protocol: "postgres",
	})

	// Create role that allows the bot to access the database.
	role, err := types.NewRole("database-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			DatabaseNames: []string{"mydb"},
			DatabaseUsers: []string{"llama"},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	botListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		botListener.Close()
	})

	// Prepare the bot config
	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())
	botConfig := defaultBotConfig(
		t, process, onboarding, config.ServiceConfigs{
			&config.DatabaseTunnelService{
				Listener: botListener,
				Service:  "test-database",
				Database: "mydb",
				Username: "llama",
			},
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			// insecure required as the db tunnel will connect to proxies
			// self-signed.
			insecure: true,
		},
	)
	botConfig.Oneshot = false
	b := New(botConfig, log)

	// Spin up goroutine for bot to run in
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(ctx)
		assert.NoError(t, err, "bot should not exit with error")
		cancel()
	}()

	// We can't predict exactly when the tunnel will be ready so we use
	// EventuallyWithT to retry.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		conn, err := pgconn.Connect(ctx, fmt.Sprintf("postgres://%s/mydb?user=llama", botListener.Addr().String()))
		if !assert.NoError(t, err) {
			return
		}
		defer func() {
			conn.Close(ctx)
		}()
		_, err = conn.Exec(ctx, "SELECT 1;").ReadAll()
		assert.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	// Shut down bot and make sure it exits.
	cancel()
	wg.Wait()
}

func TestBotSSHMultiplexer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	currentUser, err := user.Current()
	require.NoError(t, err)

	// 104 length limit on UDS on macOS forces us to use a custom tmpdir.
	tmpDir := filepath.Join(os.TempDir(), t.Name())
	require.NoError(t, os.RemoveAll(tmpDir))
	require.NoError(t, os.Mkdir(tmpDir, 0777))
	t.Cleanup(func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	})

	// Make a new auth server with SSH agent
	process := testenv.MakeTestServer(
		t,
		defaultTestServerOpts(t, log),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SSH.Enabled = true
			cfg.SSH.Addr = utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        testenv.NewTCPListener(t, service.ListenerNodeSSH, &cfg.FileDescriptors),
			}
		}),
	)
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create role that allows the bot to access the database.
	role, err := types.NewRole("ssh-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			Logins: []string{currentUser.Username},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	// Prepare the bot config
	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())
	botConfig := defaultBotConfig(
		t, process, onboarding, config.ServiceConfigs{
			&config.SSHMultiplexerService{
				Destination: &config.DestinationDirectory{
					Path: tmpDir,
				},
			},
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
		},
	)
	botConfig.Oneshot = false
	b := New(botConfig, log)

	// Spin up goroutine for bot to run in
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(ctx)
		assert.NoError(t, err, "bot should not exit with error")
		cancel()
	}()
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancel()
		wg.Wait()
	})

	// Wait for files to be output
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for _, fileName := range []string{
			"known_hosts",
			"ssh_config",
		} {
			_, err := os.Stat(filepath.Join(tmpDir, fileName))
			assert.NoError(t, err)
		}
	}, 10*time.Second, 100*time.Millisecond)

	targets := []string{
		"server01.root:0\x00",      // Old style target without cluster
		"server01.root:0|root\x00", // New style target with cluster
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			agentConn, err := net.Dial("unix", filepath.Join(tmpDir, "agent.sock"))
			require.NoError(t, err)
			t.Cleanup(func() {
				agentConn.Close()
			})
			agentClient := agent.NewClient(agentConn)
			callback, err := knownhosts.New(filepath.Join(tmpDir, "known_hosts"))
			require.NoError(t, err)
			sshConfig := &ssh.ClientConfig{
				Auth: []ssh.AuthMethod{
					ssh.PublicKeysCallback(agentClient.Signers),
				},
				User:            currentUser.Username,
				HostKeyCallback: callback,
			}
			conn, err := net.Dial("unix", filepath.Join(tmpDir, "v1.sock"))
			require.NoError(t, err)
			t.Cleanup(func() {
				conn.Close()
			})
			_, err = fmt.Fprint(conn, target)
			require.NoError(t, err)
			sshConn, sshChan, sshReq, err := ssh.NewClientConn(conn, "server01.root:22", sshConfig)
			require.NoError(t, err)
			sshClient := ssh.NewClient(sshConn, sshChan, sshReq)
			t.Cleanup(func() {
				sshClient.Close()
			})
			sshSess, err := sshClient.NewSession()
			require.NoError(t, err)
			t.Cleanup(func() {
				sshSess.Close()
			})
			out, err := sshSess.CombinedOutput("echo hello")
			require.NoError(t, err)
			require.Equal(t, "hello\n", string(out))

			// Check that the agent presents a key with cert and a bare key
			// for compat with Paramiko and older versions of OpenSSH.
			keys, err := agentClient.List()
			require.NoError(t, err)
			require.Len(t, keys, 2)
		})
	}
}
