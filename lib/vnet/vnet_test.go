// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package vnet

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math/big"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/sshutils"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

const (
	appCertLifetime = time.Hour
)

func TestMain(m *testing.M) {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	os.Exit(m.Run())
}

type testPack struct {
	vnetIPv6Prefix tcpip.Address
	dnsIPv6        tcpip.Address
	ns             *networkStack

	testStack        *stack.Stack
	testLinkEndpoint *channel.Endpoint
	localAddress     tcpip.Address
}

type testPackConfig struct {
	clock         clockwork.Clock
	fakeClientApp *fakeClientApp
	homePath      string
}

func newTestPack(t *testing.T, ctx context.Context, cfg testPackConfig) *testPack {
	if cfg.homePath == "" {
		cfg.homePath = t.TempDir()
	}

	// Create two sides of an emulated TUN interface: writes to one can be read on the other, and vice versa.
	tun1, tun2 := newSplitTUN()

	// Create an isolated userspace networking stack that can be manipulated from test code. It will be
	// connected to the VNet over the TUN interface. This emulates the host networking stack.
	// This is a completely separate gvisor network stack than the one that will be created in
	// NewNetworkStack - the two will be connected over a fake TUN interface. This exists so that the
	// test can setup IP routes without affecting the host running the Test.
	testStack, testLinkEndpoint, err := createStack()
	require.NoError(t, err)

	errIsOK := func(err error) bool {
		return err == nil || errors.Is(err, context.Canceled) || utils.IsOKNetworkError(err) || errors.Is(err, errFakeTUNClosed)
	}

	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "test network stack",
		Task: func(ctx context.Context) error {
			if err := forwardBetweenTunAndNetstack(ctx, tun1, testLinkEndpoint); !errIsOK(err) {
				return trace.Wrap(err)
			}
			return nil
		},
		Terminate: func() error {
			testLinkEndpoint.Close()
			return trace.Wrap(tun1.Close())
		},
	})

	// Assign a local IP address to the test stack.
	localAddress, err := randomULAAddress()
	require.NoError(t, err)
	protocolAddr, err := protocolAddress(localAddress)
	require.NoError(t, err)
	tcpErr := testStack.AddProtocolAddress(nicID, protocolAddr, stack.AddressProperties{})
	require.Nil(t, tcpErr)

	// Route the VNet range to the TUN interface - this emulates the route that will be installed on the host.
	vnetIPv6Prefix, err := newIPv6Prefix()
	require.NoError(t, err)
	subnet, err := tcpip.NewSubnet(vnetIPv6Prefix, tcpip.MaskFromBytes(net.CIDRMask(64, 128)))
	require.NoError(t, err)
	testStack.SetRouteTable([]tcpip.Route{{
		Destination: subnet,
		NIC:         nicID,
	}})

	dnsIPv6 := ipv6WithSuffix(vnetIPv6Prefix, []byte{2})

	// In reality the VNet networking stack runs in a separate process from the
	// client application and communicates over gRPC. For the test, everything
	// runs in a single process, but we still set up the gRPC service and only
	// interface with fakeClientApp via the gRPC client.
	clt := runTestClientApplicationService(t, ctx, cfg)
	appProvider := newAppProvider(clt)
	sshProvider, err := newSSHProvider(ctx, sshProviderConfig{
		clt:                clt,
		clock:              cfg.clock,
		overrideNodeDialer: cfg.fakeClientApp.dialSSHNode,
	})
	require.NoError(t, err)
	tcpHandlerResolver := newTCPHandlerResolver(&tcpHandlerResolverConfig{
		clt:                      clt,
		appProvider:              appProvider,
		sshProvider:              sshProvider,
		clock:                    cfg.clock,
		alwaysTrustRootClusterCA: true,
	})

	// Create the VNet and connect it to the other side of the TUN.
	ns, err := newNetworkStack(&networkStackConfig{
		tunDevice:                tun2,
		ipv6Prefix:               vnetIPv6Prefix,
		dnsIPv6:                  dnsIPv6,
		tcpHandlerResolver:       tcpHandlerResolver,
		upstreamNameserverSource: noUpstreamNameservers{},
	})
	require.NoError(t, err)

	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "VNet",
		Task: func(ctx context.Context) error {
			if err := ns.run(ctx); !errIsOK(err) {
				return trace.Wrap(err)
			}
			return nil
		},
	})

	return &testPack{
		vnetIPv6Prefix:   vnetIPv6Prefix,
		dnsIPv6:          dnsIPv6,
		ns:               ns,
		testStack:        testStack,
		testLinkEndpoint: testLinkEndpoint,
		localAddress:     localAddress,
	}
}

// dialIPPort dials the VNet address [addr] from the test virtual netstack.
func (p *testPack) dialIPPort(ctx context.Context, addr tcpip.Address, port uint16) (*gonet.TCPConn, error) {
	conn, err := gonet.DialTCPWithBind(
		ctx,
		p.testStack,
		tcpip.FullAddress{
			NIC:      nicID,
			Addr:     p.localAddress,
			LinkAddr: p.testLinkEndpoint.LinkAddress(),
		},
		tcpip.FullAddress{
			NIC:      nicID,
			Addr:     addr,
			Port:     port,
			LinkAddr: p.ns.linkEndpoint.LinkAddress(),
		},
		ipv6.ProtocolNumber,
	)
	return conn, trace.Wrap(err)
}

func (p *testPack) dialUDP(ctx context.Context, addr tcpip.Address, port uint16) (net.Conn, error) {
	conn, err := gonet.DialUDP(
		p.testStack,
		&tcpip.FullAddress{
			NIC:      nicID,
			Addr:     p.localAddress,
			LinkAddr: p.testLinkEndpoint.LinkAddress(),
		},
		&tcpip.FullAddress{
			NIC:      nicID,
			Addr:     addr,
			Port:     port,
			LinkAddr: p.ns.linkEndpoint.LinkAddress(),
		},
		ipv6.ProtocolNumber,
	)
	return conn, trace.Wrap(err)
}

func (p *testPack) lookupHost(ctx context.Context, host string) ([]string, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := p.dialUDP(ctx, p.dnsIPv6, 53)
			return conn, err
		},
	}
	return resolver.LookupHost(ctx, host)
}

func (p *testPack) dialHost(ctx context.Context, host string, port int) (net.Conn, error) {
	addrs, err := p.lookupHost(ctx, host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var allErrs []error
	for _, addr := range addrs {
		netIP := net.ParseIP(addr)
		ip := tcpip.AddrFromSlice(netIP)
		conn, err := p.dialIPPort(ctx, ip, uint16(port))
		if err != nil {
			allErrs = append(allErrs, trace.Wrap(err, "dialing %s", addr))
			continue
		}
		return conn, nil
	}
	return nil, trace.Wrap(trace.NewAggregate(allErrs...), "dialing %s", host)
}

// runTestClientApplicationService runs the gRPC service that's normally used to
// expose the client application and Teleport client methods to the VNet
// admin/networking process over gRPC. It returns a client of the gRPC service.
func runTestClientApplicationService(t *testing.T, ctx context.Context, cfg testPackConfig) *clientApplicationServiceClient {
	clusterConfigCache := NewClusterConfigCache(cfg.clock)
	leafClusterCache, err := newLeafClusterCache(cfg.clock)
	require.NoError(t, err)
	fqdnResolver := newFQDNResolver(&fqdnResolverConfig{
		clientApplication:  cfg.fakeClientApp,
		clusterConfigCache: clusterConfigCache,
		leafClusterCache:   leafClusterCache,
	})
	clientApplicationService, err := newClientApplicationService(&clientApplicationServiceConfig{
		clientApplication:     cfg.fakeClientApp,
		fqdnResolver:          fqdnResolver,
		localOSConfigProvider: nil, // OS configuration is not needed in tests.
		homePath:              cfg.homePath,
		clock:                 cfg.clock,
	})
	require.NoError(t, err)

	ipcCredentials, err := newIPCCredentials()
	require.NoError(t, err)
	serverTLSConfig, err := ipcCredentials.server.serverTLSConfig()
	require.NoError(t, err)

	grpcServer := grpc.NewServer(
		grpc.Creds(grpccredentials.NewTLS(serverTLSConfig)),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)
	vnetv1.RegisterClientApplicationServiceServer(grpcServer, clientApplicationService)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "user process gRPC server",
		Task: func(ctx context.Context) error {
			return trace.Wrap(grpcServer.Serve(listener), "serving VNet user process gRPC service")
		},
		Terminate: func() error {
			grpcServer.Stop()
			return nil
		},
	})

	// Write the service credentials to and from disk as that's what really
	// happens outside of tests.
	credDir := t.TempDir()
	require.NoError(t, ipcCredentials.client.write(credDir, 0400), "writing service credentials to disk")
	clientCreds, err := readCredentials(credDir)
	require.NoError(t, err, "reading service credentials from disk")

	clt, err := newClientApplicationServiceClient(ctx, clientCreds, listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { clt.close() })
	return clt
}

type noUpstreamNameservers struct{}

func (n noUpstreamNameservers) UpstreamNameservers(ctx context.Context) ([]string, error) {
	return nil, nil
}

type appSpec struct {
	// publicAddr is used both as the name of the app and its public address in the final spec.
	publicAddr string
	tcpPorts   []*types.PortRange
}

type nodeSpec struct {
	denyAccess bool
}

type testClusterSpec struct {
	apps           []appSpec
	nodes          map[string]nodeSpec
	cidrRange      string
	customDNSZones []string
	leafClusters   map[string]testClusterSpec
}

type fakeClientApp struct {
	cfg *fakeClientAppConfig

	tlsCA    tls.Certificate
	dialOpts *vnetv1.DialOptions

	userTLSCertMu      sync.Mutex
	userTLSCert        tls.Certificate
	userTLSCertExpires time.Time

	teleportHostCA ssh.Signer
	teleportUserCA ssh.Signer

	onNewConnectionCallCount    atomic.Uint32
	onInvalidLocalPortCallCount atomic.Uint32
	// requestedRouteToApps indexed by public address.
	requestedRouteToApps   map[string][]*proto.RouteToApp
	requestedRouteToAppsMu sync.RWMutex
}

type fakeClientAppConfig struct {
	clusters                map[string]testClusterSpec
	clock                   clockwork.Clock
	signatureAlgorithmSuite types.SignatureAlgorithmSuite
}

// newFakeClientApp returns a faked implementation of [ClientApplication].
// This is what ultimately defines the environment for the test. VNet should be
// able to run with any implementation of [ClientApplication] and little to no
// other configuration.
func newFakeClientApp(ctx context.Context, t *testing.T, cfg *fakeClientAppConfig) *fakeClientApp {
	teleportHostCAKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	teleportHostCA, err := ssh.NewSignerFromSigner(teleportHostCAKey)
	require.NoError(t, err)

	teleportUserCAKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	teleportUserCA, err := ssh.NewSignerFromSigner(teleportUserCAKey)
	require.NoError(t, err)

	tlsCA := newSelfSignedCA(t)
	dialOpts := mustStartFakeWebProxy(ctx, t, fakeWebProxyConfig{
		tlsCA:  tlsCA,
		hostCA: teleportHostCA,
		userCA: teleportUserCA,
		clock:  cfg.clock,
		suite:  cfg.signatureAlgorithmSuite,
	})

	return &fakeClientApp{
		cfg:                  cfg,
		tlsCA:                tlsCA,
		dialOpts:             dialOpts,
		teleportHostCA:       teleportHostCA,
		teleportUserCA:       teleportUserCA,
		requestedRouteToApps: make(map[string][]*proto.RouteToApp),
	}
}

// ListProfiles lists the names of all profiles saved for the user.
func (p *fakeClientApp) ListProfiles() ([]string, error) {
	return slices.Collect(maps.Keys(p.cfg.clusters)), nil
}

// GetCachedClient returns a [ClusterClient] for the given profile and leaf cluster.
// [leafClusterName] may be empty when requesting a client for the root cluster.
// Returned clients are expected to be cached, as this may be called frequently.
func (p *fakeClientApp) GetCachedClient(ctx context.Context, profileName, leafClusterName string) (ClusterClient, error) {
	rootCluster, ok := p.cfg.clusters[profileName]
	if !ok {
		return nil, trace.NotFound("no cluster for %s", profileName)
	}
	if leafClusterName == "" {
		return &fakeClusterClient{
			authClient: &fakeAuthClient{
				clusterSpec:     rootCluster,
				clusterName:     profileName,
				rootClusterName: profileName,
			},
			clusterSpec:    &rootCluster,
			teleportHostCA: p.teleportHostCA,
			teleportUserCA: p.teleportUserCA,
			clock:          p.cfg.clock,
		}, nil
	}
	leafCluster, ok := rootCluster.leafClusters[leafClusterName]
	if !ok {
		return nil, trace.NotFound("no cluster for %s.%s", profileName, leafClusterName)
	}
	return &fakeClusterClient{
		authClient: &fakeAuthClient{
			clusterSpec:     leafCluster,
			clusterName:     leafClusterName,
			rootClusterName: profileName,
		},
		clusterSpec:    &leafCluster,
		teleportHostCA: p.teleportHostCA,
		teleportUserCA: p.teleportUserCA,
		clock:          p.cfg.clock,
	}, nil
}

func (p *fakeClientApp) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error) {
	p.requestedRouteToAppsMu.Lock()
	defer p.requestedRouteToAppsMu.Unlock()

	routeToApp := RouteToApp(appInfo, targetPort)
	p.requestedRouteToApps[routeToApp.PublicAddr] = append(p.requestedRouteToApps[routeToApp.PublicAddr], routeToApp)

	return newClientCert(ctx,
		p.tlsCA,
		"testclient",
		p.cfg.clock.Now().Add(appCertLifetime),
		p.cfg.signatureAlgorithmSuite,
		cryptosuites.UserTLS)
}

func (p *fakeClientApp) UserTLSCert(ctx context.Context, profileName string) (tls.Certificate, error) {
	p.userTLSCertMu.Lock()
	defer p.userTLSCertMu.Unlock()

	now := p.cfg.clock.Now()
	if now.Before(p.userTLSCertExpires) {
		return p.userTLSCert, nil
	}
	expiry := now.Add(defaults.CertDuration)
	userTLSCert, err := newClientCert(ctx,
		p.tlsCA,
		"testuser",
		expiry,
		p.cfg.signatureAlgorithmSuite,
		cryptosuites.UserTLS)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	p.userTLSCert = userTLSCert
	p.userTLSCertExpires = expiry
	return userTLSCert, nil
}

func (p *fakeClientApp) RequestedRouteToApps(publicAddr string) []*proto.RouteToApp {
	p.requestedRouteToAppsMu.RLock()
	defer p.requestedRouteToAppsMu.RUnlock()

	requestedRoutes := p.requestedRouteToApps[publicAddr]
	returnedRoutes := make([]*proto.RouteToApp, len(requestedRoutes))
	copy(returnedRoutes, requestedRoutes)

	return returnedRoutes
}

func (p *fakeClientApp) GetDialOptions(ctx context.Context, profileName string) (*vnetv1.DialOptions, error) {
	return p.dialOpts, nil
}

func (p *fakeClientApp) GetVnetConfig(ctx context.Context, profileName, leafClusterName string) (*vnet.VnetConfig, error) {
	rootCluster, ok := p.cfg.clusters[profileName]
	if !ok {
		return nil, trace.Errorf("no cluster for %s", profileName)
	}
	if leafClusterName == "" {
		if rootCluster.cidrRange == "" {
			return nil, trace.NotFound("vnet_config not found")
		}
		cfg := &vnet.VnetConfig{
			Kind:    types.KindVnetConfig,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "vnet-config",
			},
			Spec: &vnet.VnetConfigSpec{
				Ipv4CidrRange: rootCluster.cidrRange,
			},
		}
		for _, zone := range rootCluster.customDNSZones {
			cfg.Spec.CustomDnsZones = append(cfg.Spec.CustomDnsZones,
				&vnet.CustomDNSZone{Suffix: zone},
			)
		}
		return cfg, nil
	}
	leafCluster, ok := rootCluster.leafClusters[leafClusterName]
	if !ok {
		return nil, trace.Errorf("no cluster for %s.%s", profileName, leafClusterName)
	}
	if leafCluster.cidrRange == "" {
		return nil, trace.NotFound("vnet_config not found")
	}
	cfg := &vnet.VnetConfig{
		Kind:    types.KindVnetConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "vnet-config",
		},
		Spec: &vnet.VnetConfigSpec{
			Ipv4CidrRange: leafCluster.cidrRange,
		},
	}
	for _, zone := range leafCluster.customDNSZones {
		cfg.Spec.CustomDnsZones = append(cfg.Spec.CustomDnsZones,
			&vnet.CustomDNSZone{Suffix: zone},
		)
	}
	return cfg, nil
}

func (p *fakeClientApp) OnNewConnection(_ context.Context, _ *vnetv1.AppKey) error {
	p.onNewConnectionCallCount.Add(1)
	return nil
}

func (p *fakeClientApp) OnInvalidLocalPort(_ context.Context, _ *vnetv1.AppInfo, _ uint16) {
	p.onInvalidLocalPortCallCount.Add(1)
}

func (p *fakeClientApp) dialSSHNode(
	ctx context.Context,
	target dialTarget,
	tlsConfig *tls.Config,
	dialOpts *vnetv1.DialOptions,
) (net.Conn, error) {
	targetCluster, ok := p.cfg.clusters[target.profile]
	if !ok {
		return nil, trace.NotFound("no such profile")
	}
	if target.cluster != target.profile {
		targetCluster, ok = targetCluster.leafClusters[target.cluster]
		if !ok {
			return nil, trace.NotFound("no such cluster")
		}
	}
	if _, ok := targetCluster.nodes[target.hostname]; !ok {
		return nil, trace.NotFound("no such host")
	}
	tlsConfig.NextProtos = []string{string(alpncommon.ProtocolProxySSH)}
	return tls.Dial("tcp", dialOpts.GetWebProxyAddr(), tlsConfig)
}

type fakeClusterClient struct {
	authClient     *fakeAuthClient
	clusterSpec    *testClusterSpec
	teleportHostCA ssh.Signer
	teleportUserCA ssh.Signer
	clock          clockwork.Clock
}

func (c *fakeClusterClient) CurrentCluster() authclient.ClientI {
	return c.authClient
}

func (c *fakeClusterClient) ClusterName() string {
	return c.authClient.clusterName
}

func (c *fakeClusterClient) RootClusterName() string {
	return c.authClient.rootClusterName
}

func (c *fakeClusterClient) SessionSSHKeyRing(ctx context.Context, user string, target client.NodeDetails) (*client.KeyRing, bool, error) {
	targetHost, _, err := net.SplitHostPort(target.Addr)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	nodeSpec, ok := c.clusterSpec.nodes[targetHost]
	if !ok {
		return nil, false, trace.NotFound("no such node")
	}
	if nodeSpec.denyAccess {
		return nil, false, trace.AccessDenied("access denied to %s", targetHost)
	}
	userSSHKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.Ed25519)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	userSSHSigner, err := ssh.NewSignerFromSigner(userSSHKey)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	now := c.clock.Now()
	cert := &ssh.Certificate{
		Key:             userSSHSigner.PublicKey(),
		Serial:          1,
		CertType:        ssh.UserCert,
		ValidPrincipals: []string{user},
		ValidAfter:      uint64(now.Add(-1 * time.Minute).Unix()),
		ValidBefore:     uint64(now.Add(time.Minute).Unix()),
	}
	if err := cert.SignCert(rand.Reader, c.teleportUserCA); err != nil {
		return nil, false, trace.Wrap(err)
	}
	trustedCert := ssh.MarshalAuthorizedKey(c.teleportHostCA.PublicKey())
	k := &client.KeyRing{
		SSHPrivateKey: userSSHKey,
		Cert:          ssh.MarshalAuthorizedKey(cert),
		TrustedCerts: []authclient.TrustedCerts{
			{
				ClusterName:    c.ClusterName(),
				AuthorizedKeys: [][]byte{trustedCert},
			},
		},
	}
	return k, false, nil
}

// fakeAuthClient is a fake auth client that answers GetResources requests with a static list of apps and
// basic/faked predicate filtering.
type fakeAuthClient struct {
	authclient.ClientI
	clusterSpec     testClusterSpec
	clusterName     string
	rootClusterName string
}

func (c *fakeAuthClient) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	resp := &proto.ListResourcesResponse{}
	for _, app := range c.clusterSpec.apps {
		// Poor-man's predicate expression filter.
		if !strings.Contains(req.PredicateExpression, app.publicAddr) {
			continue
		}
		spec := &types.AppV3{
			Metadata: types.Metadata{
				Name: app.publicAddr,
			},
			Spec: types.AppSpecV3{
				PublicAddr: app.publicAddr,
				URI:        "tcp://" + app.publicAddr,
			},
		}

		if len(app.tcpPorts) != 0 {
			spec.SetTCPPorts(app.tcpPorts)
		}

		resp.Resources = append(resp.Resources, &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_AppServer{
				AppServer: &types.AppServerV3{
					Kind: types.KindAppServer,
					Metadata: types.Metadata{
						Name: app.publicAddr,
					},
					Spec: types.AppServerSpecV3{
						App: spec,
					},
				},
			},
		})
	}
	resp.TotalCount = int32(len(resp.Resources))
	return resp, nil
}

func (c *fakeAuthClient) ListRemoteClusters(ctx context.Context, pageSize int, pageToken string) ([]types.RemoteCluster, string, error) {
	remoteClusters := make([]types.RemoteCluster, 0, len(c.clusterSpec.leafClusters))
	for leafClusterName := range c.clusterSpec.leafClusters {
		rc, err := types.NewRemoteCluster(leafClusterName)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		remoteClusters = append(remoteClusters, rc)
	}
	return remoteClusters, "", nil
}

func (c *fakeAuthClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{
		ClusterName:     c.clusterName,
		ProxyPublicAddr: c.clusterName,
	}, nil
}

func (c *fakeAuthClient) GetVnetConfig(ctx context.Context) (*vnet.VnetConfig, error) {
	vnetConfig := &vnet.VnetConfig{
		Spec: &vnet.VnetConfigSpec{
			Ipv4CidrRange: c.clusterSpec.cidrRange,
		},
	}
	for _, zone := range c.clusterSpec.customDNSZones {
		vnetConfig.Spec.CustomDnsZones = append(vnetConfig.Spec.CustomDnsZones, &vnet.CustomDNSZone{
			Suffix: zone,
		})
	}
	return vnetConfig, nil
}

// TestDialFakeApp tests basic functionality of TCP app access via VNet.
func TestDialFakeApp(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClockAt(time.Now())

	clientApp := newFakeClientApp(ctx, t, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"root1.example.com": {
				apps: []appSpec{
					appSpec{publicAddr: "echo1.root1.example.com"},
					appSpec{publicAddr: "echo2.root1.example.com"},
					appSpec{publicAddr: "echo.myzone.example.com"},
					appSpec{publicAddr: "echo.nested.myzone.example.com"},
					appSpec{publicAddr: "not.in.a.custom.zone"},
					appSpec{
						publicAddr: "multi-port.root1.example.com",
						tcpPorts: []*types.PortRange{
							&types.PortRange{
								Port: 1337,
							},
							&types.PortRange{
								Port: 4242,
							},
						},
					},
				},
				customDNSZones: []string{
					"myzone.example.com",
				},
				cidrRange: "192.168.2.0/24",
				leafClusters: map[string]testClusterSpec{
					"leaf1.example.com": {
						apps: []appSpec{
							appSpec{publicAddr: "echo1.leaf1.example.com"},
							appSpec{
								publicAddr: "multi-port.leaf1.example.com",
								tcpPorts: []*types.PortRange{
									&types.PortRange{
										Port: 1337,
									},
									&types.PortRange{
										Port: 4242,
									},
								},
							},
						},
					},
					"leaf2.example.com": {
						apps: []appSpec{
							appSpec{publicAddr: "echo1.leaf2.example.com"},
						},
					},
				},
			},
			"root2.example.com": {
				apps: []appSpec{
					appSpec{publicAddr: "echo1.root2.example.com"},
					appSpec{publicAddr: "echo2.root2.example.com"},
				},
				leafClusters: map[string]testClusterSpec{
					"leaf3.example.com": {
						apps: []appSpec{
							appSpec{publicAddr: "echo1.leaf3.example.com"},
						},
					},
				},
			},
		},
		clock:                   clock,
		signatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
	})

	p := newTestPack(t, ctx, testPackConfig{
		fakeClientApp: clientApp,
		clock:         clock,
	})

	validTestCases := []struct {
		app              string
		port             int
		expectCIDR       string
		expectRouteToApp proto.RouteToApp
	}{
		{
			app:        "echo1.root1.example.com",
			expectCIDR: "192.168.2.0/24",
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo1.root1.example.com",
				PublicAddr:  "echo1.root1.example.com",
				ClusterName: "root1.example.com",
			},
		},
		{
			app:        "echo2.root1.example.com",
			expectCIDR: "192.168.2.0/24",
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo2.root1.example.com",
				PublicAddr:  "echo2.root1.example.com",
				ClusterName: "root1.example.com",
			},
		},
		{
			app:        "echo.myzone.example.com",
			expectCIDR: "192.168.2.0/24",
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo.myzone.example.com",
				PublicAddr:  "echo.myzone.example.com",
				ClusterName: "root1.example.com",
			},
		},
		{
			app:        "echo.nested.myzone.example.com",
			expectCIDR: "192.168.2.0/24",
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo.nested.myzone.example.com",
				PublicAddr:  "echo.nested.myzone.example.com",
				ClusterName: "root1.example.com",
			},
		},
		{
			app:        "echo1.leaf1.example.com",
			expectCIDR: typesvnet.DefaultIPv4CIDRRange,
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo1.leaf1.example.com",
				PublicAddr:  "echo1.leaf1.example.com",
				ClusterName: "leaf1.example.com",
			},
		},
		{
			app:        "echo1.leaf2.example.com",
			expectCIDR: typesvnet.DefaultIPv4CIDRRange,
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo1.leaf2.example.com",
				PublicAddr:  "echo1.leaf2.example.com",
				ClusterName: "leaf2.example.com",
			},
		},
		{
			app:        "echo1.root2.example.com",
			expectCIDR: typesvnet.DefaultIPv4CIDRRange,
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo1.root2.example.com",
				PublicAddr:  "echo1.root2.example.com",
				ClusterName: "root2.example.com",
			},
		},
		{
			app:        "echo2.root2.example.com",
			expectCIDR: typesvnet.DefaultIPv4CIDRRange,
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo2.root2.example.com",
				PublicAddr:  "echo2.root2.example.com",
				ClusterName: "root2.example.com",
			},
		},
		{
			app:        "echo1.leaf3.example.com",
			expectCIDR: typesvnet.DefaultIPv4CIDRRange,
			expectRouteToApp: proto.RouteToApp{
				Name:        "echo1.leaf3.example.com",
				PublicAddr:  "echo1.leaf3.example.com",
				ClusterName: "leaf3.example.com",
			},
		},
		{
			app:        "multi-port.root1.example.com",
			port:       1337,
			expectCIDR: "192.168.2.0/24",
			expectRouteToApp: proto.RouteToApp{
				Name:        "multi-port.root1.example.com",
				PublicAddr:  "multi-port.root1.example.com",
				ClusterName: "root1.example.com",
				TargetPort:  1337,
			},
		},
		{
			app:        "multi-port.leaf1.example.com",
			port:       1337,
			expectCIDR: typesvnet.DefaultIPv4CIDRRange,
			expectRouteToApp: proto.RouteToApp{
				Name:        "multi-port.leaf1.example.com",
				PublicAddr:  "multi-port.leaf1.example.com",
				ClusterName: "leaf1.example.com",
				TargetPort:  1337,
			},
		},
	}

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		// Connect to each app 3 times, advancing the clock past the cert lifetime between each
		// connection to trigger a cert refresh.
		//
		// It's important not to run these subtests which advance a shared clock in parallel. It's okay for
		// the inner app dial/connection tests to run in parallel because they don't advance the clock.
		for i := 0; i < 3; i++ {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				for _, tc := range validTestCases {
					if tc.expectRouteToApp.URI == "" && tc.expectRouteToApp.PublicAddr != "" {
						tc.expectRouteToApp.URI = "tcp://" + tc.expectRouteToApp.PublicAddr
					}
					t.Run(tc.app, func(t *testing.T) {
						t.Parallel()

						_, expectNet, err := net.ParseCIDR(tc.expectCIDR)
						require.NoError(t, err)

						const defaultPort = 80
						conn, err := p.dialHost(ctx, tc.app, cmp.Or(tc.port, defaultPort))
						require.NoError(t, err)
						t.Cleanup(func() { assert.NoError(t, conn.Close()) })

						remoteAddr, _, err := net.SplitHostPort(conn.RemoteAddr().String())
						require.NoError(t, err)
						remoteIP := net.ParseIP(remoteAddr)
						require.NotNil(t, remoteIP)

						// The app name may have resolved to a v4 or v6 address, either way the 4-byte suffix should be a
						// valid IPv4 address in the expected CIDR range.
						remoteIPSuffix := remoteIP[len(remoteIP)-4:]
						assert.True(t, expectNet.Contains(remoteIPSuffix), "expected CIDR range %s does not include remote IP %s", expectNet, remoteIPSuffix)

						testEchoConnection(t, conn)

						for _, requestedRouteToApp := range clientApp.RequestedRouteToApps(tc.app) {
							assert.Equal(t, &tc.expectRouteToApp, requestedRouteToApp,
								"requested cert RouteToApp did not match expected for app")
						}
					})
				}
			})
			clock.Advance(2 * appCertLifetime)
		}
	})

	t.Run("invalid FQDN", func(t *testing.T) {
		t.Parallel()
		invalidTestCases := []string{
			"not.an.app.example.com.",
			"not.in.a.custom.zone",
		}
		for _, fqdn := range invalidTestCases {
			t.Run(fqdn, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
				defer cancel()
				_, err := p.lookupHost(ctx, fqdn)
				require.Error(t, err)
			})
		}
	})

	t.Run("invalid target port", func(t *testing.T) {
		t.Parallel()
		app := "multi-port.root1.example.com"
		port := 1000

		_, err := p.dialHost(ctx, app, port)
		// VNet is expected to refuse the connection rather than let the dial go through and then
		// immediately close it.
		require.ErrorContains(t, err, "connection was refused")

		requestedRoutes := clientApp.RequestedRouteToApps(app)
		require.False(t, slices.ContainsFunc(requestedRoutes, func(route *proto.RouteToApp) bool {
			return int(route.TargetPort) == port
		}), "no certs are supposed to be requested for target port %d in app %s", port, app)
		require.Equal(t, uint32(1), clientApp.onInvalidLocalPortCallCount.Load(), "unexpected number of calls to OnInvalidLocalPort")
	})
}

func testEchoConnection(t *testing.T, conn net.Conn) {
	t.Helper()
	const testString = "1........."
	writeBuf := bytes.Repeat([]byte(testString), 200)
	readBuf := make([]byte, len(writeBuf))

	for i := 0; i < 10; i++ {
		written, err := conn.Write(writeBuf)
		for written < len(writeBuf) && err == nil {
			var n int
			n, err = conn.Write(writeBuf[written:])
			written += n
		}
		require.NoError(t, err)
		require.Equal(t, len(writeBuf), written)

		n, err := io.ReadFull(conn, readBuf)
		require.NoError(t, err)
		require.Equal(t, string(writeBuf), string(readBuf[:n]))
	}
}

// TestOnNewConnection tests that the client applications OnNewConnection method
// is called when a user connects to a valid TCP app.
func TestOnNewConnection(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClockAt(time.Now())

	clientApp := newFakeClientApp(ctx, t, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"root1.example.com": {
				apps: []appSpec{
					appSpec{publicAddr: "echo1"},
				},
				cidrRange:    "192.168.2.0/24",
				leafClusters: map[string]testClusterSpec{},
			},
		},
		clock:                   clock,
		signatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
	})

	validAppName := "echo1.root1.example.com"
	invalidAppName := "not.an.app.example.com."

	p := newTestPack(t, ctx, testPackConfig{
		clock:         clock,
		fakeClientApp: clientApp,
	})

	// Attempt to establish a connection to an invalid app and verify that OnNewConnection was not
	// called.
	lookupCtx, lookupCtxCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer lookupCtxCancel()
	_, err := p.lookupHost(lookupCtx, invalidAppName)
	require.Error(t, err, "Expected lookup of an invalid app to fail")
	require.Equal(t, uint32(0), clientApp.onNewConnectionCallCount.Load())

	// Establish a connection to a valid app and verify that OnNewConnection was called.
	conn, err := p.dialHost(ctx, validAppName, 80 /* bogus port */)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.Equal(t, uint32(1), clientApp.onNewConnectionCallCount.Load())
}

// TestWithAlgorithmSuites tests basic VNet functionality with each signature
// algorithm suite to make sure that the suite does not break anything. It
// mostly makes a difference when TLS signatures are completed over gRPC with
// the private key only accessible to the client application.
func TestWithAlgorithmSuites(t *testing.T) {
	t.Parallel()
	for _, suite := range []types.SignatureAlgorithmSuite{
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
	} {
		t.Run(suite.String(), func(t *testing.T) {
			t.Parallel()
			testWithAlgorithmSuite(t, suite)
		})
	}
}

func testWithAlgorithmSuite(t *testing.T, suite types.SignatureAlgorithmSuite) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClockAt(time.Now())

	clientApp := newFakeClientApp(ctx, t, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"root.example.com": {
				apps: []appSpec{
					appSpec{publicAddr: "echo1"},
					appSpec{publicAddr: "echo2"},
				},
				cidrRange: "192.168.2.0/24",
				leafClusters: map[string]testClusterSpec{
					"leaf.example.com": {
						apps: []appSpec{
							appSpec{publicAddr: "echo1"},
							appSpec{publicAddr: "echo2"},
						},
						cidrRange: "192.168.2.0/24",
					},
				},
			},
		},
		clock:                   clock,
		signatureAlgorithmSuite: suite,
	})

	p := newTestPack(t, ctx, testPackConfig{
		fakeClientApp: clientApp,
		clock:         clock,
	})

	// References to each app private key may be cached so try repeated
	// out-of-order connections to different apps.
	for _, app := range []string{
		"echo1.root.example.com",
		"echo2.root.example.com",
		"echo1.root.example.com",
		"echo1.leaf.example.com",
		"echo2.leaf.example.com",
		"echo1.leaf.example.com",
	} {
		conn, err := p.dialHost(ctx, app, 123)
		require.NoError(t, err)
		testEchoConnection(t, conn)
	}

	// Sanity check connecting to a non-existent app fails.
	dialCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_, err := p.dialHost(dialCtx, "badapp.root.example.com.", 123)
	require.Error(t, err)
}

// TestSSH tests basic VNet SSH functionality.
func TestSSH(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewRealClock()
	homePath := t.TempDir()

	const (
		root1CIDR = "192.168.1.0/24"
		leaf1CIDR = "192.168.2.0/24"
		root2CIDR = "192.168.3.0/24"
		leaf2CIDR = "192.168.4.0/24"
	)
	clientApp := newFakeClientApp(ctx, t, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"root1.example.com": {
				cidrRange: root1CIDR,
				nodes: map[string]nodeSpec{
					"node":     {},
					"denynode": {denyAccess: true},
				},
				leafClusters: map[string]testClusterSpec{
					"leaf1.example.com": {
						cidrRange: leaf1CIDR,
						nodes: map[string]nodeSpec{
							"node": {},
						},
					},
				},
			},
			"root2.example.com": {
				cidrRange: root2CIDR,
				nodes: map[string]nodeSpec{
					"node": {},
				},
				leafClusters: map[string]testClusterSpec{
					"leaf2.example.com": {
						cidrRange: leaf2CIDR,
						nodes: map[string]nodeSpec{
							"node": {},
						},
					},
				},
			},
		},
		clock:                   clock,
		signatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
	})

	p := newTestPack(t, ctx, testPackConfig{
		fakeClientApp: clientApp,
		clock:         clock,
		homePath:      homePath,
	})

	// Read the generated vnet_known_hosts file to get the trusted host CA key.
	knownHosts, err := os.ReadFile(keypaths.VNetKnownHostsPath(homePath))
	require.NoError(t, err)
	marker, hosts, hostCAPubKey, _, _, err := ssh.ParseKnownHosts(knownHosts)
	require.NoError(t, err)
	require.Equal(t, "cert-authority", marker)
	require.Equal(t, []string{"*"}, hosts)

	// Read the generated id_vnet file to get the user key.
	sshUserKey, err := os.ReadFile(keypaths.VNetClientSSHKeyPath(homePath))
	require.NoError(t, err)
	sshUserSigner, err := ssh.ParsePrivateKey(sshUserKey)
	require.NoError(t, err)

	// Create a fake user key to test failed authentication.
	badUserKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	badUserSigner, err := ssh.NewSignerFromSigner(badUserKey)
	require.NoError(t, err)

	for _, tc := range []struct {
		dialAddr                 string
		dialPort                 int
		expectCIDR               string
		expectLookupToFail       bool
		expectDialToFail         bool
		sshUser                  string
		sshUserSigner            ssh.Signer
		expectSSHHandshakeToFail bool
		expectBannerMessages     []string
	}{
		{
			// Connection to node in root cluster should work.
			dialAddr:      "node.root1.example.com",
			dialPort:      22,
			expectCIDR:    root1CIDR,
			sshUser:       "testuser",
			sshUserSigner: sshUserSigner,
		},
		{
			// Fully-qualified hostname should also work.
			dialAddr:      "node.root1.example.com.",
			dialPort:      22,
			expectCIDR:    root1CIDR,
			sshUser:       "testuser",
			sshUserSigner: sshUserSigner,
		},
		{
			// Dial should fail on non-standard SSH port.
			dialAddr:         "node.root1.example.com",
			dialPort:         23,
			expectCIDR:       root1CIDR,
			expectDialToFail: true,
		},
		{
			// SSH handshake should fail if using the wrong user key.
			dialAddr:                 "node.root1.example.com",
			dialPort:                 22,
			expectCIDR:               root1CIDR,
			sshUser:                  "baduser",
			sshUserSigner:            badUserSigner,
			expectSSHHandshakeToFail: true,
		},
		{
			// Access to denied node should be denied with appropriate banner
			// messages.
			dialAddr:      "denynode.root1.example.com",
			dialPort:      22,
			expectCIDR:    root1CIDR,
			sshUser:       "testuser",
			sshUserSigner: sshUserSigner,
			expectBannerMessages: []string{
				"VNet: building SSH client config\n\tcalling SessionSSHConfig rpc\n\t\tgetting KeyRing for SSH session\n\taccess denied to denynode\n",
			},
			expectSSHHandshakeToFail: true,
		},
		{
			// username "denyuser" is hardcoded to be denied.
			dialAddr:      "node.root1.example.com",
			dialPort:      22,
			expectCIDR:    root1CIDR,
			sshUser:       "denyuser",
			sshUserSigner: sshUserSigner,
			expectBannerMessages: []string{
				"VNet: access denied to denyuser connecting to node\n",
			},
			expectSSHHandshakeToFail: true,
		},
		{
			// Connection to node in leaf cluster should work.
			dialAddr:      "node.leaf1.example.com.root1.example.com",
			dialPort:      22,
			expectCIDR:    leaf1CIDR,
			sshUser:       "testuser",
			sshUserSigner: sshUserSigner,
		},
		{
			// Connection to node in root cluster in alternate profile should
			// work.
			dialAddr:      "node.root2.example.com",
			dialPort:      22,
			expectCIDR:    root2CIDR,
			sshUser:       "testuser",
			sshUserSigner: sshUserSigner,
		},
		{
			// Connection to node in leaf cluster in alternate profile should
			// work.
			dialAddr:      "node.leaf2.example.com.root2.example.com",
			dialPort:      22,
			expectCIDR:    leaf2CIDR,
			sshUser:       "testuser",
			sshUserSigner: sshUserSigner,
		},
		{
			// DNS lookup should fail if the FQDN doesn't match any cluster.
			dialAddr:           "node.bogus.example.com.",
			dialPort:           22,
			expectLookupToFail: true,
		},
		{
			// If the FQDN matches a cluster but no node, the DNS lookup should
			// succeed but the TCP dial should fail.
			dialAddr:         "bogus.root1.example.com",
			dialPort:         22,
			expectCIDR:       root1CIDR,
			expectDialToFail: true,
		},
	} {
		t.Run(fmt.Sprintf("%s@%s:%d", tc.sshUser, tc.dialAddr, tc.dialPort), func(t *testing.T) {
			t.Parallel()

			lookupCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			// The DNS lookup for *.<cluster-name> should resolve to an IP in
			// the expected CIDR range for the cluster.
			resolvedAddrs, err := p.lookupHost(lookupCtx, tc.dialAddr)
			if tc.expectLookupToFail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			_, expectNet, err := net.ParseCIDR(tc.expectCIDR)
			require.NoError(t, err)

			for _, resolvedAddr := range resolvedAddrs {
				resolvedIP := net.ParseIP(resolvedAddr)
				// The query may have resolved to a v4 or v6 address or both,
				// either way the 4-byte suffix should be a valid IPv4 address
				// in the expected CIDR range.
				resolvedIPSuffix := resolvedIP[len(resolvedIP)-4:]
				assert.True(t, expectNet.Contains(resolvedIPSuffix),
					"expected CIDR range %s does not include resolved IP %s", expectNet, resolvedIPSuffix)
			}

			// TCP dial the target address, it should fail if the node doesn't
			// exist.
			dialCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			conn, err := p.dialHost(dialCtx, tc.dialAddr, tc.dialPort)
			if tc.expectDialToFail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer conn.Close()

			// Initiate an SSH connection to the target. At this point the
			// handshake should complete successfully as long as the right keys
			// are used, but the SSH connection will be immediately closed by
			// the server.
			certChecker := ssh.CertChecker{
				IsHostAuthority: func(auth ssh.PublicKey, address string) bool {
					return sshutils.KeysEqual(auth, hostCAPubKey)
				},
				Clock: clock.Now,
			}
			var bannerMessages []string
			clientConfig := &ssh.ClientConfig{
				User:            tc.sshUser,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(tc.sshUserSigner)},
				HostKeyCallback: certChecker.CheckHostKey,
				BannerCallback: func(msg string) error {
					bannerMessages = append(bannerMessages, msg)
					return nil
				},
			}
			sshConn, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:%d", tc.dialAddr, tc.dialPort), clientConfig)
			assert.Equal(t, tc.expectBannerMessages, bannerMessages, "actual banner messages did not match the expected")
			if tc.expectSSHHandshakeToFail {
				assert.Error(t, err, "expected SSH handshake to fail")
				return
			}
			require.NoError(t, err)
			defer sshConn.Close()

			testConnectionToSshEchoServer(t, sshConn, chans, reqs)
		})
	}

	// Test that a fresh SSH host cert is used on each connection.
	t.Run("ephemeral certs", func(t *testing.T) {
		// Set up the SSH client config to capture the host certs it sees.
		var checkedHostCerts []*ssh.Certificate
		clientConfig := &ssh.ClientConfig{
			User: "testuser",
			Auth: []ssh.AuthMethod{ssh.PublicKeys(sshUserSigner)},
			HostKeyCallback: func(addr string, remote net.Addr, key ssh.PublicKey) error {
				checkedHostCerts = append(checkedHostCerts, key.(*ssh.Certificate))
				return nil
			},
		}
		const connections = 3
		for range connections {
			conn, err := p.dialHost(ctx, "node.root1.example.com", 22)
			require.NoError(t, err)
			sshConn, _, _, err := ssh.NewClientConn(conn, "node.root1.example.com:22", clientConfig)
			require.NoError(t, err)
			sshConn.Close()
		}
		require.Len(t, checkedHostCerts, connections)
		for i := range connections - 1 {
			require.NotEqual(t, checkedHostCerts[i], checkedHostCerts[i+1])
		}
	})
}

func randomULAAddress() (tcpip.Address, error) {
	var bytes [16]byte
	bytes[0] = 0xfd
	if _, err := rand.Read(bytes[1:16]); err != nil {
		return tcpip.Address{}, trace.Wrap(err)
	}
	return tcpip.AddrFrom16(bytes), nil
}

var errFakeTUNClosed = errors.New("TUN closed")

type fakeTUN struct {
	name                            string
	writePacketsTo, readPacketsFrom chan []byte
	closed                          chan struct{}
	closeOnce                       func()
}

// newSplitTUN returns two fake TUN devices that are tied together: writes to one can be read on the other,
// and vice versa.
func newSplitTUN() (*fakeTUN, *fakeTUN) {
	aClosed := make(chan struct{})
	bClosed := make(chan struct{})
	ab := make(chan []byte)
	ba := make(chan []byte)
	return &fakeTUN{
			name:            "tun1",
			writePacketsTo:  ab,
			readPacketsFrom: ba,
			closed:          aClosed,
			closeOnce:       sync.OnceFunc(func() { close(aClosed) }),
		}, &fakeTUN{
			name:            "tun2",
			writePacketsTo:  ba,
			readPacketsFrom: ab,
			closed:          bClosed,
			closeOnce:       sync.OnceFunc(func() { close(bClosed) }),
		}
}

func (f *fakeTUN) BatchSize() int {
	return 1
}

// Write one or more packets to the device (without any additional headers).
// On a successful write it returns the number of packets written. A nonzero
// offset can be used to instruct the Device on where to begin writing from
// each packet contained within the bufs slice.
func (f *fakeTUN) Write(bufs [][]byte, offset int) (int, error) {
	if len(bufs) != 1 {
		return 0, trace.BadParameter("batchsize is 1")
	}
	packet := make([]byte, len(bufs[0][offset:]))
	copy(packet, bufs[0][offset:])
	select {
	case <-f.closed:
		return 0, errFakeTUNClosed
	case f.writePacketsTo <- packet:
	}
	return 1, nil
}

// Read one or more packets from the Device (without any additional headers).
// On a successful read it returns the number of packets read, and sets
// packet lengths within the sizes slice. len(sizes) must be >= len(bufs).
// A nonzero offset can be used to instruct the Device on where to begin
// reading into each element of the bufs slice.
func (f *fakeTUN) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	if len(bufs) != 1 {
		return 0, trace.BadParameter("batchsize is 1")
	}
	var packet []byte
	select {
	case <-f.closed:
		return 0, errFakeTUNClosed
	case packet = <-f.readPacketsFrom:
	}
	sizes[0] = copy(bufs[0][offset:], packet)
	return 1, nil
}

func (f *fakeTUN) Close() error {
	f.closeOnce()
	return nil
}

func newSelfSignedCA(t *testing.T) tls.Certificate {
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, signer.Public(), signer)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  signer,
	}
}

func newServerCert(
	ctx context.Context,
	ca tls.Certificate,
	cn string,
	expires time.Time,
	suite types.SignatureAlgorithmSuite,
	purpose cryptosuites.KeyPurpose,
) (tls.Certificate, error) {
	return newLeafCert(ctx, ca, cn, expires, x509.ExtKeyUsageServerAuth, suite, purpose)
}

func newClientCert(
	ctx context.Context,
	ca tls.Certificate,
	cn string,
	expires time.Time,
	suite types.SignatureAlgorithmSuite,
	purpose cryptosuites.KeyPurpose,
) (tls.Certificate, error) {
	return newLeafCert(ctx, ca, cn, expires, x509.ExtKeyUsageClientAuth, suite, purpose)
}

func newLeafCert(
	ctx context.Context,
	ca tls.Certificate,
	cn string,
	expires time.Time,
	keyUsage x509.ExtKeyUsage,
	suite types.SignatureAlgorithmSuite,
	purpose cryptosuites.KeyPurpose,
) (tls.Certificate, error) {
	signer, err := cryptosuites.GenerateKey(ctx, cryptosuites.StaticAlgorithmSuite(suite), purpose)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	caCert, err := x509.ParseCertificate(ca.Certificate[0])
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:   time.Now(),
		NotAfter:    expires,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{keyUsage},
		DNSNames:    []string{cn},
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, signer.Public(), ca.PrivateKey)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  signer,
	}, nil
}

type fakeWebProxyConfig struct {
	tlsCA  tls.Certificate
	hostCA ssh.Signer
	userCA ssh.Signer
	clock  clockwork.Clock
	suite  types.SignatureAlgorithmSuite
}

func mustStartFakeWebProxy(
	ctx context.Context,
	t *testing.T,
	cfg fakeWebProxyConfig,
) *vnetv1.DialOptions {
	t.Helper()

	roots := x509.NewCertPool()
	caX509, err := x509.ParseCertificate(cfg.tlsCA.Certificate[0])
	require.NoError(t, err)
	roots.AddCert(caX509)

	const proxyCN = "testproxy"
	proxyCert, err := newServerCert(
		ctx,
		cfg.tlsCA,
		proxyCN,
		cfg.clock.Now().Add(365*24*time.Hour),
		cfg.suite,
		cryptosuites.HostIdentity,
	)
	require.NoError(t, err)

	proxyTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{proxyCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
		NextProtos: []string{
			string(alpncommon.ProtocolProxySSH),
			string(alpncommon.ProtocolTCP),
		},
	}

	tcpAppHandler := func(conn net.Conn) error {
		// All fake TCP apps for the tests get routed to this handler which
		// simply echos any input back on the tcp connection.
		_, err := io.Copy(conn, conn)
		return trace.Wrap(err, "io.Copy error in proxy echo server")
	}
	sshHandler := func(conn net.Conn) error {
		// All fake SSH nodes for the tests get routed to this handler which
		// terminates the incoming SSH connection with an ephemeral host cert.
		// It trusts cfg.userCA for incoming SSH connections but always denies
		// access for SSH users named "denyuser". After completing the handshake
		// it runs a test "echo" SSH server implemented in
		// runTestSSHServerInstance.
		hostCert, err := newHostCert("node", cfg.hostCA)
		if err != nil {
			return trace.Wrap(err)
		}
		certChecker := ssh.CertChecker{
			IsUserAuthority: func(auth ssh.PublicKey) bool {
				return sshutils.KeysEqual(auth, cfg.userCA.PublicKey())
			},
			Clock: cfg.clock.Now,
		}
		serverConfig := &ssh.ServerConfig{
			PublicKeyCallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
				if conn.User() == "denyuser" {
					return nil, trace.AccessDenied("access denied for denyuser")
				}
				return certChecker.Authenticate(conn, pubKey)
			},
		}
		serverConfig.AddHostKey(hostCert)
		return trace.Wrap(runTestSSHServerInstance(conn, serverConfig))
	}

	// Run a simplified TLS router for the test.
	protocolHandlers := map[alpncommon.Protocol]func(net.Conn) error{
		alpncommon.ProtocolTCP:      tcpAppHandler,
		alpncommon.ProtocolProxySSH: sshHandler,
	}

	listener, err := tls.Listen("tcp", "localhost:0", proxyTLSConfig)
	require.NoError(t, err)

	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "web proxy",
		Task: func(ctx context.Context) error {
			for {
				conn, err := listener.Accept()
				if err != nil {
					if utils.IsOKNetworkError(err) {
						return nil
					}
					return trace.Wrap(err)
				}
				go func() {
					defer conn.Close()

					// Not using require/assert here and below because this is not in the right subtest or in
					// the main test goroutine. The test will fail if the conn is not handled.
					tlsConn, ok := conn.(*tls.Conn)
					if !ok {
						t.Log("client conn is not TLS")
						return
					}
					if err := tlsConn.Handshake(); err != nil {
						t.Log("error completing tls handshake")
						return
					}
					clientCerts := tlsConn.ConnectionState().PeerCertificates
					if len(clientCerts) == 0 {
						t.Log("client has no certs")
						return
					}
					// Manually checking the cert expiry compared to the time of the fake clock, since the TLS
					// library will only compare the cert expiry to the real clock.
					// It's important that the fake clock is never far behind the real clock, and that the
					// cert NotBefore is always at/before the real current time, so the TLS library is
					// satisfied.
					if cfg.clock.Now().After(clientCerts[0].NotAfter) {
						t.Logf("client cert is expired: currentTime=%s expiry=%s", cfg.clock.Now(), clientCerts[0].NotAfter)
						return
					}

					protocol := tlsConn.ConnectionState().NegotiatedProtocol
					handler, ok := protocolHandlers[alpncommon.Protocol(protocol)]
					if !ok {
						t.Logf("unhandled proxy protocol %s", protocol)
						return
					}
					if err := handler(conn); err != nil {
						t.Logf("error in protocol handler: %v", err)
					}
				}()
			}
		},
		Terminate: func() error {
			if err := listener.Close(); !utils.IsOKNetworkError(err) {
				return trace.Wrap(err)
			}
			return nil
		},
	})

	caPEM, err := tlsca.MarshalCertificatePEM(caX509)
	require.NoError(t, err)
	dialOpts := &vnetv1.DialOptions{
		WebProxyAddr:          listener.Addr().String(),
		RootClusterCaCertPool: caPEM,
		Sni:                   proxyCN,
	}
	return dialOpts
}
