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

// Package testenv provides functions for creating test servers for testing.
package testenv

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
	"github.com/gravitational/teleport/tool/teleport/common"
)

const (
	Loopback = "127.0.0.1"
	Host     = "localhost"
)

// StaticToken is used to easily join test services
const StaticToken = "test-static-token"

func init() {
	// If the test is re-executing itself, execute the command that comes over
	// the pipe. Used to test tsh ssh and tsh scp commands.
	if srv.IsReexec() {
		common.Run(common.Options{Args: os.Args[1:]})
		return
	}

	modules.SetModules(&cliModules{})
}

// WithInsecureDevMode is a test helper that sets insecure dev mode and resets
// it in test cleanup.
// It is NOT SAFE to use in parallel tests, because it modifies a global.
// To run insecure dev mode tests in parallel, group them together under a
// parent test and then run them as parallel subtests.
// and call WithInsecureDevMode before running all the tests in parallel.
func WithInsecureDevMode(t *testing.T, mode bool) {
	originalValue := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(mode)
	// To detect tests that run in parallel incorrectly, call t.Setenv with a
	// dummy env var - that function detects tests with parallel ancestors
	// and panics, preventing improper use of this helper.
	t.Setenv("WithInsecureDevMode", "1")
	t.Cleanup(func() {
		lib.SetInsecureDevMode(originalValue)
	})
}

// WithResyncInterval is a test helper that sets the tunnel resync interval and
// resets it in test cleanup.
// Useful to substantially speedup test cluster setup - passing 0 for the
// interval selects a reasonably fast default of 100ms.
// It is NOT SAFE to use in parallel tests, because it modifies a global.
func WithResyncInterval(t *testing.T, interval time.Duration) {
	if interval == 0 {
		interval = time.Millisecond * 100
	}
	oldResyncInterval := defaults.ResyncInterval
	defaults.ResyncInterval = interval
	// To detect tests that run in parallel incorrectly, call t.Setenv with a
	// dummy env var - that function detects tests with parallel ancestors
	// and panics, preventing improper use of this helper.
	t.Setenv("WithResyncInterval", "1")
	t.Cleanup(func() {
		defaults.ResyncInterval = oldResyncInterval
	})
}

// MakeTestServer creates a Teleport Server for testing.
func MakeTestServer(t *testing.T, opts ...TestServerOptFunc) (process *service.TeleportProcess) {
	t.Helper()

	var options TestServersOpts
	for _, opt := range opts {
		opt(&options)
	}

	// Set up a test auth server with default config.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.CachePolicy.Enabled = false
	// Disables cloud auto-imported labels when running tests in cloud envs
	// such as Github Actions.
	//
	// This is required otherwise Teleport will import cloud instance
	// labels, and use them for example as labels in Kubernetes Service and
	// cause some tests to fail because the output includes unexpected
	// labels.
	//
	// It is also found that Azure metadata client can throw "Too many
	// requests" during CI which fails services.NewTeleport.
	cfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()

	cfg.Hostname = "server01"
	cfg.DataDir = t.TempDir()
	cfg.Logger = utils.NewSlogLoggerForTests()
	authAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: NewTCPListener(t, service.ListenerAuth, &cfg.FileDescriptors)}
	cfg.SetToken(StaticToken)
	cfg.SetAuthServerAddress(authAddr)

	cfg.Auth.ListenAddr = authAddr
	cfg.Auth.BootstrapResources = options.Bootstrap
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	staticToken, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   StaticToken,
		}},
	})
	require.NoError(t, err)
	cfg.Auth.StaticTokens = staticToken
	cfg.Auth.Preference.SetSignatureAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1)

	// Disable session recording to prevent writing to disk after the test concludes.
	cfg.Auth.SessionRecordingConfig.SetMode(types.RecordOff)
	// Speeds up tests considerably.
	cfg.Auth.StorageConfig.Params["poll_stream_period"] = 50 * time.Millisecond

	cfg.Proxy.WebAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: NewTCPListener(t, service.ListenerProxyWeb, &cfg.FileDescriptors)}
	cfg.Proxy.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: NewTCPListener(t, service.ListenerProxySSH, &cfg.FileDescriptors)}
	cfg.Proxy.ReverseTunnelListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: NewTCPListener(t, service.ListenerProxyTunnel, &cfg.FileDescriptors)}
	cfg.Proxy.DisableWebInterface = true

	cfg.SSH.Addr = utils.NetAddr{AddrNetwork: "tcp", Addr: NewTCPListener(t, service.ListenerNodeSSH, &cfg.FileDescriptors)}
	cfg.SSH.DisableCreateHostUser = true

	// Disabling debug service for tests so that it doesn't break if the data
	// directory path is too long.
	cfg.DebugService.Enabled = false

	// Apply options
	for _, fn := range options.ConfigFuncs {
		fn(cfg)
	}

	process, err = service.NewTeleport(cfg)
	require.NoError(t, err, trace.DebugReport(err))
	require.NoError(t, process.Start())
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	waitForServices(t, process, cfg)

	return process
}

// NewTCPListener creates a new TCP listener on 127.0.0.1:0, adds it to the
// FileDescriptor slice (with the specified type) and returns its actual local
// address as a string (for use in configuration). Takes a pointer to the slice
// so that it's convenient to call in the middle of a FileConfig or Config
// struct literal.
func NewTCPListener(t *testing.T, lt service.ListenerType, fds *[]*servicecfg.FileDescriptor) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	addr := l.Addr().String()

	// File() returns a dup of the listener's file descriptor as an *os.File, so
	// the original net.Listener still needs to be closed.
	lf, err := l.(*net.TCPListener).File()
	require.NoError(t, err)
	fd := &servicecfg.FileDescriptor{
		Type:    string(lt),
		Address: addr,
		File:    lf,
	}
	// If the file descriptor slice ends up being passed to a TeleportProcess
	// that successfully starts, listeners will either get "imported" and used
	// or discarded and closed, this is just an extra safety measure that closes
	// the listener at the end of the test anyway (the finalizer would do that
	// anyway, in principle).
	t.Cleanup(func() { require.NoError(t, fd.Close()) })

	*fds = append(*fds, fd)
	return addr
}

func waitForServices(t *testing.T, auth *service.TeleportProcess, cfg *servicecfg.Config) {
	var serviceReadyEvents []string
	if cfg.Proxy.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.ProxyWebServerReady)
	}
	if cfg.SSH.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.NodeSSHReady)
	}
	if cfg.Databases.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.DatabasesReady)
	}
	if cfg.Apps.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.AppsReady)
	}
	if cfg.Auth.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.AuthTLSReady)
	}
	waitForEvents(t, auth, serviceReadyEvents...)

	if cfg.Auth.Enabled && cfg.Databases.Enabled {
		waitForDatabases(t, auth, cfg.Databases.Databases)
	}

	if cfg.Auth.Enabled && cfg.Apps.Enabled {
		waitForApps(t, auth, cfg.Apps.Apps)
	}
}

func waitForEvents(t *testing.T, svc service.Supervisor, events ...string) {
	for _, event := range events {
		_, err := svc.WaitForEventTimeout(10*time.Second, event)
		require.NoError(t, err, "service server didn't receive %v event after 10s", event)
	}
}

func waitForDatabases(t *testing.T, auth *service.TeleportProcess, dbs []servicecfg.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)

			// Count how many input "dbs" are registered.
			var registered int
			for _, db := range dbs {
				for _, a := range all {
					if a.GetName() == db.Name {
						registered++
						break
					}
				}
			}

			if registered == len(dbs) {
				return
			}
		case <-ctx.Done():
			t.Fatal("Databases not registered after 10s")
		}
	}
}

func waitForApps(t *testing.T, auth *service.TeleportProcess, apps []servicecfg.App) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetApplicationServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)

			var registered int
			for _, app := range apps {
				for _, a := range all {
					if a.GetName() == app.Name {
						registered++
						break
					}
				}
			}

			if registered == len(apps) {
				return
			}
		case <-ctx.Done():
			t.Fatal("Apps not registered after 10s")
		}
	}
}

type TestServersOpts struct {
	Bootstrap   []types.Resource
	ConfigFuncs []func(cfg *servicecfg.Config)
}

type TestServerOptFunc func(o *TestServersOpts)

func WithBootstrap(bootstrap ...types.Resource) TestServerOptFunc {
	return func(o *TestServersOpts) {
		o.Bootstrap = append(o.Bootstrap, bootstrap...)
	}
}

func WithConfig(fn func(cfg *servicecfg.Config)) TestServerOptFunc {
	return func(o *TestServersOpts) {
		o.ConfigFuncs = append(o.ConfigFuncs, fn)
	}
}

func WithAuthConfig(fn func(*servicecfg.AuthConfig)) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		fn(&cfg.Auth)
	})
}

func WithClusterName(t *testing.T, n string) TestServerOptFunc {
	return WithAuthConfig(func(cfg *servicecfg.AuthConfig) {
		clusterName, err := services.NewClusterNameWithRandomID(
			types.ClusterNameSpecV2{
				ClusterName: n,
			})
		require.NoError(t, err)
		cfg.ClusterName = clusterName
	})
}

func WithHostname(hostname string) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Hostname = hostname
	})
}

func WithSSHPublicAddrs(addrs ...string) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.SSH.PublicAddrs = utils.MustParseAddrList(addrs...)
	})
}

func WithSSHLabel(key, value string) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		if cfg.SSH.Labels == nil {
			cfg.SSH.Labels = make(map[string]string)
		}
		cfg.SSH.Labels[key] = value
	})
}

func WithAuthPreference(authPref types.AuthPreference) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Auth.Preference = authPref
	})
}

func WithLogger(log *slog.Logger) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Logger = log
	})
}

// WithProxyKube enables the Proxy Kube listener with a random address.
func WithProxyKube(t *testing.T) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Proxy.Kube.Enabled = true
		cfg.Proxy.Kube.ListenAddr = utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        NewTCPListener(t, service.ListenerProxyKube, &cfg.FileDescriptors),
		}
	})
}

// WithDebugApp enables the app service and the debug app.
func WithDebugApp() TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Apps.Enabled = true
		cfg.Apps.DebugApp = true
	})
}

// WithTestApp enables the app service and adds a test app server
// with the given name.
func WithTestApp(t *testing.T, name string) TestServerOptFunc {
	appUrl := startDummyHTTPServer(t, name)
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Apps.Enabled = true
		cfg.Apps.Apps = append(cfg.Apps.Apps,
			servicecfg.App{
				Name: name,
				URI:  appUrl,
				StaticLabels: map[string]string{
					"name": name,
				},
			})
	})
}

func startDummyHTTPServer(t *testing.T, name string) string {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", name)
		_, _ = w.Write([]byte("hello"))
	}))

	srv.Start()

	t.Cleanup(func() {
		srv.Close()
	})

	return srv.URL
}

func SetupTrustedCluster(ctx context.Context, t *testing.T, rootServer, leafServer *service.TeleportProcess, additionalRoleMappings ...types.RoleMapping) {
	// Use insecure mode so that the trusted cluster can establish trust over reverse tunnel.
	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(isInsecure) })

	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)
	rootProxyTunnelAddr, err := rootServer.ProxyTunnelAddr()
	require.NoError(t, err)

	tc, err := types.NewTrustedCluster(rootServer.Config.Auth.ClusterName.GetClusterName(), types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                StaticToken,
		ProxyAddress:         rootProxyAddr.String(),
		ReverseTunnelAddress: rootProxyTunnelAddr.String(),
		RoleMap: append(additionalRoleMappings,
			types.RoleMapping{
				Remote: "access",
				Local:  []string{"access"},
			},
		),
	})
	require.NoError(t, err)

	_, err = leafServer.GetAuthServer().UpsertTrustedClusterV2(ctx, tc)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rt, err := rootServer.GetAuthServer().GetTunnelConnections(leafServer.Config.Auth.ClusterName.GetClusterName())
		assert.NoError(t, err)
		assert.Len(t, rt, 1)
	}, time.Second*10, time.Second)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rts, err := rootServer.GetAuthServer().GetRemoteClusters(ctx)
		assert.NoError(t, err)
		assert.Len(t, rts, 1)
	}, time.Second*10, time.Second)
}

type cliModules struct{}

func (p *cliModules) GenerateAccessRequestPromotions(_ context.Context, _ modules.AccessResourcesGetter, _ types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return &types.AccessRequestAllowedPromotions{}, nil
}

func (p *cliModules) GetSuggestedAccessLists(ctx context.Context, _ *tlsca.Identity, _ modules.AccessListSuggestionClient, _ modules.AccessListAndMembersGetter, _ string) ([]*accesslist.AccessList, error) {
	return []*accesslist.AccessList{}, nil
}

// BuildType returns build type.
func (p *cliModules) BuildType() string {
	return "CLI"
}

// LicenseExpiry returns the expiry date of the enterprise license, if applicable.
func (p *cliModules) LicenseExpiry() time.Time {
	return time.Time{}
}

// IsEnterpriseBuild returns false for [cliModules].
func (p *cliModules) IsEnterpriseBuild() bool {
	return false
}

// IsOSSBuild returns false for [cliModules].
func (p *cliModules) IsOSSBuild() bool {
	return false
}

// PrintVersion prints the Teleport version.
func (p *cliModules) PrintVersion() {
	fmt.Println("Teleport CLI")
}

// Features returns supported features
func (p *cliModules) Features() modules.Features {
	return modules.Features{
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.K8s: {Enabled: true},
			entitlements.DB:  {Enabled: true},
			entitlements.App: {Enabled: true},
		},
		AdvancedAccessWorkflows: true,
		AccessControls:          true,
	}
}

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
func (p *cliModules) IsBoringBinary() bool {
	return false
}

// AttestHardwareKey attests a hardware key.
func (p *cliModules) AttestHardwareKey(_ context.Context, _ any, _ *hardwarekey.AttestationStatement, _ crypto.PublicKey, _ time.Duration) (*keys.AttestationData, error) {
	return nil, trace.NotFound("no attestation data for the given key")
}

func (p *cliModules) EnableRecoveryCodes() {
}

func (p *cliModules) EnablePlugins() {
}

func (p *cliModules) EnableAccessGraph() {}

func (p *cliModules) EnableAccessMonitoring() {}

func (p *cliModules) SetFeatures(f modules.Features) {
}

func CreateAgentlessNode(t *testing.T, authServer *auth.Server, clusterName, nodeHostname string) *types.ServerV2 {
	t.Helper()

	ctx := context.Background()
	openSSHCA, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OpenSSHCA,
		DomainName: clusterName,
	}, false)
	require.NoError(t, err)

	caCheckers, err := sshutils.GetCheckers(openSSHCA)
	require.NoError(t, err)

	key, err := cryptosuites.GenerateKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(authServer), cryptosuites.HostSSH)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(key.Public())
	require.NoError(t, err)

	nodeUUID := uuid.New().String()
	hostCertBytes, err := authServer.GenerateHostCert(
		ctx,
		ssh.MarshalAuthorizedKey(sshPub),
		"",
		"",
		[]string{nodeUUID, nodeHostname, Loopback},
		clusterName,
		types.RoleNode,
		0,
	)
	require.NoError(t, err)

	hostCert, err := apisshutils.ParseCertificate(hostCertBytes)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromSigner(key)
	require.NoError(t, err)
	hostKeySigner, err := ssh.NewCertSigner(hostCert, signer)
	require.NoError(t, err)

	// start SSH server
	sshAddr := startSSHServer(t, caCheckers, hostKeySigner)

	// create node resource
	node := &types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: nodeUUID,
		},
		Spec: types.ServerSpecV2{
			Addr:     sshAddr,
			Hostname: nodeHostname,
		},
	}
	_, err = authServer.UpsertNode(ctx, node)
	require.NoError(t, err)

	// wait for node resource to be written to the backend
	timedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	t.Cleanup(cancel)
	w, err := authServer.NewWatcher(timedCtx, types.Watch{
		Name: "node-create watcher",
		Kinds: []types.WatchKind{
			{
				Kind: types.KindNode,
			},
		},
	})
	require.NoError(t, err)

	for nodeCreated := false; !nodeCreated; {
		select {
		case e := <-w.Events():
			if e.Type == types.OpPut {
				nodeCreated = true
			}
		case <-w.Done():
			t.Fatal("Did not receive node create event")
		}
	}
	require.NoError(t, w.Close())

	return node
}

// startSSHServer starts a SSH server that roughly mimics an unregistered
// OpenSSH (agentless) server. The SSH server started only handles a small
// subset of SSH requests necessary for testing.
func startSSHServer(t *testing.T, caPubKeys []ssh.PublicKey, hostKey ssh.Signer) string {
	t.Helper()

	sshCfg := ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			cert, ok := key.(*ssh.Certificate)
			if !ok {
				return nil, fmt.Errorf("expected *ssh.Certificate, got %T", key)
			}

			// Sanity check incoming cert from proxy has Ed25519 key.
			if cert.Key.Type() != ssh.KeyAlgoED25519 {
				return nil, trace.BadParameter("expected Ed25519 key, got %v", cert.Key.Type())
			}

			for _, pubKey := range caPubKeys {
				if bytes.Equal(cert.SignatureKey.Marshal(), pubKey.Marshal()) {
					return &ssh.Permissions{}, nil
				}
			}

			return nil, fmt.Errorf("signature key %v does not match OpenSSH CA", cert.SignatureKey)
		},
	}
	sshCfg.AddHostKey(hostKey)

	lis, err := net.Listen("tcp", Loopback+":")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, lis.Close())
	})

	go func() {
		nConn, err := lis.Accept()
		if utils.IsOKNetworkError(err) {
			return
		}
		assert.NoError(t, err)
		t.Cleanup(func() {
			if nConn != nil {
				// the error is ignored here to avoid failing on net.ErrClosed
				_ = nConn.Close()
			}
		})

		conn, channels, reqs, err := ssh.NewServerConn(nConn, &sshCfg)
		assert.NoError(t, err)
		t.Cleanup(func() {
			if conn != nil {
				// the error is ignored here to avoid failing on net.ErrClosed
				_ = conn.Close()
			}
		})
		go ssh.DiscardRequests(reqs)

		var agentForwarded bool
		var shellRequested bool
		var execRequested bool
		var sftpRequested bool
		for {
			var channelReq ssh.NewChannel
			select {
			case channelReq = <-channels:
				if channelReq == nil { // server is closed
					return
				}
			case <-t.Context().Done():
				return
			}
			if !assert.Equal(t, "session", channelReq.ChannelType()) {
				assert.NoError(t, channelReq.Reject(ssh.Prohibited, "only session channels expected"))
				continue
			}
			channel, reqs, err := channelReq.Accept()
			assert.NoError(t, err)
			t.Cleanup(func() {
				// the error is ignored here to avoid failing on net.ErrClosed
				_ = channel.Close()
			})

			go func() {
			outer:
				for {
					var req *ssh.Request
					select {
					case req = <-reqs:
						if req == nil { // channel is closed
							return
						}
					case <-t.Context().Done():
						break outer
					}
					if req.WantReply {
						assert.NoError(t, req.Reply(true, nil))
					}
					switch req.Type {
					case sshutils.AgentForwardRequest:
						agentForwarded = true
					case sshutils.ShellRequest:
						assert.NoError(t, channel.Close())
						shellRequested = true
						break outer
					case sshutils.ExecRequest:
						_, err := channel.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: 0}))
						assert.NoError(t, err)
						assert.NoError(t, channel.Close())
						execRequested = true
						break outer
					case sshutils.SubsystemRequest:
						var r sshutils.SubsystemReq
						err := ssh.Unmarshal(req.Payload, &r)
						assert.NoError(t, err)
						assert.Equal(t, "sftp", r.Name)
						sftpRequested = true

						sftpServer, err := sftp.NewServer(channel)
						assert.NoError(t, err)
						go sftpServer.Serve()
						t.Cleanup(func() {
							err := sftpServer.Close()
							if err != nil {
								assert.ErrorIs(t, err, io.EOF)
							}
						})
						break outer
					}
				}
				assert.True(t, (agentForwarded && shellRequested) || execRequested || sftpRequested)
			}()
		}
	}()

	return lis.Addr().String()
}

// MakeDefaultAuthClient reimplements the bare minimum needed to create a
// default root-level auth client for a Teleport server started by
// MakeTestServer.
func MakeDefaultAuthClient(t *testing.T, process *service.TeleportProcess) *authclient.Client {
	t.Helper()

	cfg := process.Config
	hostUUID, err := hostid.ReadFile(process.Config.DataDir)
	require.NoError(t, err)

	identity, err := storage.ReadLocalIdentity(
		filepath.Join(cfg.DataDir, teleport.ComponentProcess),
		state.IdentityID{Role: types.RoleAdmin, HostUUID: hostUUID},
	)
	require.NoError(t, err)

	authConfig := new(authclient.Config)
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	require.NoError(t, err)

	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Logger

	client, err := authclient.Connect(context.Background(), authConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}
