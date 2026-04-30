// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package beams_test

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	proto "github.com/gravitational/teleport/api/client/proto"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	vnetv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/services/beams"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/dns"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestVNetService(t *testing.T) {
	t.Setenv("TELEPORT_BEAMS_RUNTIME", "yes")

	// Start a fake upstream nameserver to check recursion works.
	logger := logtest.NewLogger()
	upstreamResolvedIP := net.ParseIP("1.2.3.4")
	upstreamNameserver, err := dns.NewServer(
		staticResult{A: [4]byte(upstreamResolvedIP.To4())},
		beams.StaticUpstreamNameservers{},
		logger,
	)
	require.NoError(t, err)

	nsConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	go func() {
		_ = upstreamNameserver.ListenAndServeUDP(t.Context(), nsConn)
	}()

	// Start an HTTP server that we'll access over VNet.
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello, world!"))
	}))
	t.Cleanup(httpServer.Close)

	// Spin up a Teleport process, configured to expose the HTTP server by
	// application access.
	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(logger),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name:       "intranet",
					URI:        httpServer.URL,
					PublicAddr: "intranet.dunder-mifflin.com",
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	// Configure VNet to add a custom DNS zone.
	_, err = rootClient.VnetConfigClient().
		UpsertVnetConfig(t.Context(), &vnetv1.VnetConfig{
			Kind:    types.KindVnetConfig,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: types.MetaNameVnetConfig,
			},
			Spec: &vnetv1.VnetConfigSpec{
				CustomDnsZones: []*vnetv1.CustomDNSZone{
					{Suffix: "dunder-mifflin.com"},
				},
			},
		})
	require.NoError(t, err)

	// Create a user with access to all applications,
	user, err := types.NewUser("alice")
	require.NoError(t, err)

	role, err := types.NewRole("app-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": {"*"},
			},
		},
	})
	require.NoError(t, err)
	_, err = rootClient.CreateRole(t.Context(), role)
	require.NoError(t, err)

	user.AddRole(role.GetName())
	user, err = rootClient.CreateUser(t.Context(), user)
	require.NoError(t, err)

	// Create a delegation session for the user.
	aliceClient := makeUserClient(t, process, rootClient, user.GetName())
	session, err := aliceClient.DelegationSessionServiceClient().
		CreateDelegationSession(t.Context(), &delegationv1.CreateDelegationSessionRequest{
			Spec: &delegationv1.DelegationSessionSpec{
				User: user.GetName(),
				Resources: []*delegationv1.DelegationResourceSpec{
					{Kind: types.Wildcard, Name: types.Wildcard},
				},
				AuthorizedUsers: []*delegationv1.DelegationUserSpec{
					{
						Kind:    types.KindBot,
						Matcher: &delegationv1.DelegationUserSpec_BotName{BotName: "test-bot"},
					},
				},
			},
			Ttl: durationpb.New(5 * time.Minute),
		})
	require.NoError(t, err)

	// Create a fake host network.
	hostNetwork, err := vnet.NewFakeHostNetwork()
	require.NoError(t, err)
	t.Cleanup(hostNetwork.Close)

	// Run the bot.
	b, err := bot.New(bot.Config{
		Connection: connection.Config{
			Address:     proxyAddr.Addr,
			AddressKind: connection.AddressKindProxy,
			Insecure:    true,
		},
		Onboarding: makeBot(t, rootClient, "test-bot"),
		Logger:     logger,
		Services: []bot.ServiceBuilder{
			beams.VNetServiceBuilder(
				&beams.VNetServiceConfig{
					DelegationSessionID: session.GetMetadata().GetName(),
					UpstreamNameservers: []string{nsConn.LocalAddr().String()},
				},
				beams.WithInsecure(),
				beams.WithTUNDevice(hostNetwork.TUNDevice()),
				beams.WithConfigureHost(hostNetwork.Configure),
			),
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() { errCh <- b.Run(ctx) }()

	// Wait for the host network to be configured, or the bot to fail.
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-hostNetwork.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for host network to be configured")
	}

	// Call the HTTP app over VNet.
	client := &http.Client{Transport: hostNetwork.HTTPTransport()}
	rsp, err := client.Get("http://intranet.dunder-mifflin.com")
	require.NoError(t, err)
	defer rsp.Body.Close()

	rspBody, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello, world!", string(rspBody))

	// Try to resolve a non-existent app to check it hits our upstream nameserver.
	ips, err := hostNetwork.DNSResolver().
		LookupIP(t.Context(), "ip4", "blog.dunder-mifflin.com")
	require.NoError(t, err)
	require.Len(t, ips, 1)
	require.True(t, ips[0].Equal(upstreamResolvedIP))

	cancel()
	require.NoError(t, <-errCh)
}

func defaultTestServerOpts(log *slog.Logger) testenv.TestServerOptFunc {
	return func(o *testenv.TestServersOpts) error {
		testenv.WithClusterName("root")(o)
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Logger = log
			cfg.Proxy.PublicAddrs = []utils.NetAddr{
				{AddrNetwork: "tcp", Addr: net.JoinHostPort("localhost", strconv.Itoa(cfg.Proxy.WebAddr.Port(0)))},
			}
			cfg.Proxy.TunnelPublicAddrs = []utils.NetAddr{
				cfg.Proxy.ReverseTunnelListenAddr,
			}
		})(o)

		return nil
	}
}

func makeBot(t *testing.T, client *authclient.Client, name string, roles ...string) onboarding.Config {
	t.Helper()

	b, err := client.BotServiceClient().CreateBot(t.Context(), &machineidv1pb.CreateBotRequest{
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

	err = client.CreateToken(t.Context(), tok)
	require.NoError(t, err)

	return onboarding.Config{
		TokenValue: tok.GetName(),
		JoinMethod: types.JoinMethodToken,
	}
}

func makeUserClient(t *testing.T, process *service.TeleportProcess, rootClient *authclient.Client, username string) *authclient.Client {
	t.Helper()

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	tlsPrivateKey, err := keys.MarshalPrivateKey(tlsKey)
	require.NoError(t, err)

	tlsPublicKey, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	certs, err := rootClient.GenerateUserCerts(t.Context(), proto.UserCertsRequest{
		TLSPublicKey: tlsPublicKey,
		Username:     username,
		Expires:      time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)

	tlsCert, err := keys.X509KeyPair(certs.TLS, tlsPrivateKey)
	require.NoError(t, err)

	identity, err := storage.ReadLocalIdentityForRole(
		t.Context(),
		filepath.Join(process.Config.DataDir, teleport.ComponentProcess),
		types.RoleAdmin,
	)
	require.NoError(t, err)

	tlsConfig, err := identity.TLSConfig(process.Config.CipherSuites)
	require.NoError(t, err)
	tlsConfig.Certificates = []tls.Certificate{tlsCert}

	userClient, err := authclient.Connect(t.Context(), &authclient.Config{
		TLS:         tlsConfig,
		AuthServers: process.Config.AuthServerAddresses(),
		Log:         process.Config.Logger,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, userClient.Close())
	})

	return userClient
}

type staticResult dns.Result

func (s staticResult) ResolveA(context.Context, string) (dns.Result, error) {
	return dns.Result(s), nil
}

func (s staticResult) ResolveAAAA(context.Context, string) (dns.Result, error) {
	return dns.Result(s), nil
}
