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
	"context"
	"crypto"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
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

// NewTeleportProcess creates a Teleport Server for testing.
func NewTeleportProcess(dataDir string, opts ...TestServerOptFunc) (_ *service.TeleportProcess, err error) {
	var options TestServersOpts
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Set up a test auth server with default config.
	cfg := servicecfg.MakeDefaultConfig()
	defer func() {
		if err != nil {
			for _, fd := range cfg.FileDescriptors {
				fd.Close()
			}
		}
	}()

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
	cfg.DataDir = dataDir
	cfg.Logger = logtest.NewLogger()

	authAddr, err := newTCPListener(service.ListenerAuth, &cfg.FileDescriptors)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authNetAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: authAddr}
	cfg.SetToken(StaticToken)
	cfg.SetAuthServerAddress(authNetAddr)

	cfg.Auth.ListenAddr = authNetAddr
	cfg.Auth.BootstrapResources = options.Bootstrap
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	staticToken, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   StaticToken,
		}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Auth.StaticTokens = staticToken
	cfg.Auth.Preference.SetSignatureAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1)

	// Disable session recording to prevent writing to disk after the test concludes.
	cfg.Auth.SessionRecordingConfig.SetMode(types.RecordOff)
	// Speeds up tests considerably.
	cfg.Auth.StorageConfig.Params["poll_stream_period"] = 50 * time.Millisecond

	proxyWebAddr, err := newTCPListener(service.ListenerProxyWeb, &cfg.FileDescriptors)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Proxy.WebAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: proxyWebAddr}

	proxySSHAddr, err := newTCPListener(service.ListenerProxySSH, &cfg.FileDescriptors)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyTunnelAddr, err := newTCPListener(service.ListenerProxyTunnel, &cfg.FileDescriptors)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Proxy.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: proxySSHAddr}
	cfg.Proxy.ReverseTunnelListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: proxyTunnelAddr}
	cfg.Proxy.DisableWebInterface = true

	nodeSSHAddr, err := newTCPListener(service.ListenerNodeSSH, &cfg.FileDescriptors)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SSH.Addr = utils.NetAddr{AddrNetwork: "tcp", Addr: nodeSSHAddr}
	cfg.SSH.DisableCreateHostUser = true

	// Disabling debug service for tests so that it doesn't break if the data
	// directory path is too long.
	cfg.DebugService.Enabled = false

	// Apply options
	for _, fn := range options.ConfigFuncs {
		fn(cfg)
	}

	process, err := service.NewTeleport(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := process.Start(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := waitForServices(process, cfg); err != nil {
		_ = process.Close()
		_ = process.Wait()
		return nil, trace.Wrap(err)
	}

	return process, nil
}

// newTCPListener creates a new TCP listener on 127.0.0.1:0, adds it to the
// FileDescriptor slice (with the specified type) and returns its actual local
// address as a string (for use in configuration). Takes a pointer to the slice
// so that it's convenient to call in the middle of a FileConfig or Config
// struct literal.
func newTCPListener(lt service.ListenerType, fds *[]*servicecfg.FileDescriptor) (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", trace.Wrap(err)
	}

	defer l.Close()
	addr := l.Addr().String()

	// File() returns a dup of the listener's file descriptor as an *os.File, so
	// the original net.Listener still needs to be closed.
	lf, err := l.(*net.TCPListener).File()
	if err != nil {
		return "", trace.Wrap(err)
	}

	fd := &servicecfg.FileDescriptor{
		Type:    string(lt),
		Address: addr,
		File:    lf,
	}

	*fds = append(*fds, fd)
	return addr, nil
}

func waitForServices(auth *service.TeleportProcess, cfg *servicecfg.Config) error {
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
	if cfg.Kube.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.KubernetesReady)
	}
	if err := waitForEvents(auth, serviceReadyEvents...); err != nil {
		return trace.Wrap(err)
	}

	if cfg.Auth.Enabled && cfg.Databases.Enabled {
		if err := waitForDatabases(auth, cfg.Databases.Databases); err != nil {
			return trace.Wrap(err)
		}
	}

	if cfg.Auth.Enabled && cfg.Apps.Enabled {
		if err := waitForApps(auth, cfg.Apps.Apps); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func waitForEvents(svc service.Supervisor, events ...string) error {
	for _, event := range events {
		if _, err := svc.WaitForEventTimeout(10*time.Second, event); err != nil {
			return trace.Wrap(err, "service didn't receive %v event after 10s", event)
		}
	}

	return nil
}

func waitForDatabases(auth *service.TeleportProcess, dbs []servicecfg.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
			if err != nil {
				return trace.Wrap(err)
			}

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
				return nil
			}
		case <-ctx.Done():
			return trace.LimitExceeded("Databases not registered after 10s")
		}
	}
}

func waitForApps(auth *service.TeleportProcess, apps []servicecfg.App) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetApplicationServers(ctx, apidefaults.Namespace)
			if err != nil {
				return trace.Wrap(err)
			}

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
				return nil
			}
		case <-ctx.Done():
			return trace.LimitExceeded("Apps not registered after 10s")
		}
	}
}

type TestServersOpts struct {
	Bootstrap   []types.Resource
	ConfigFuncs []func(cfg *servicecfg.Config)
}

type TestServerOptFunc func(o *TestServersOpts) error

func WithBootstrap(bootstrap ...types.Resource) TestServerOptFunc {
	return func(o *TestServersOpts) error {
		o.Bootstrap = append(o.Bootstrap, bootstrap...)
		return nil
	}
}

func WithConfig(fn func(cfg *servicecfg.Config)) TestServerOptFunc {
	return func(o *TestServersOpts) error {
		o.ConfigFuncs = append(o.ConfigFuncs, fn)
		return nil
	}
}

func WithAuthConfig(fn func(*servicecfg.AuthConfig)) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		fn(&cfg.Auth)
	})
}

func WithClusterName(name string) TestServerOptFunc {
	return WithAuthConfig(func(cfg *servicecfg.AuthConfig) {
		cfg.ClusterName = &types.ClusterNameV2{
			Spec: types.ClusterNameSpecV2{
				ClusterName: name,
				ClusterID:   uuid.NewString(),
			},
		}
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
func WithProxyKube() TestServerOptFunc {
	return func(o *TestServersOpts) error {
		var fds []*servicecfg.FileDescriptor
		addr, err := newTCPListener(service.ListenerProxyKube, &fds)
		if err != nil {
			return trace.Wrap(err)
		}

		return WithConfig(func(cfg *servicecfg.Config) {
			cfg.Proxy.Kube.Enabled = true
			cfg.Proxy.Kube.ListenAddr = utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        addr,
			}
			cfg.FileDescriptors = append(cfg.FileDescriptors, fds...)
		})(o)
	}
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
func WithTestApp(name, uri string) TestServerOptFunc {
	return WithConfig(func(cfg *servicecfg.Config) {
		cfg.Apps.Enabled = true
		cfg.Apps.Apps = append(cfg.Apps.Apps,
			servicecfg.App{
				Name: name,
				URI:  uri,
				StaticLabels: map[string]string{
					"name": name,
				},
			})
	})
}

func StartDummyHTTPServer(name string) *httptest.Server {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", name)
		_, _ = w.Write([]byte("hello"))
	}))

	srv.Start()

	return srv
}

type cliModules struct{}

func (p *cliModules) GenerateLongTermResourceGrouping(_ context.Context, _ modules.AccessResourcesGetter, _ types.AccessRequest) (*types.LongTermResourceGrouping, error) {
	return &types.LongTermResourceGrouping{}, nil
}

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

// NewDefaultAuthClient reimplements the bare minimum needed to create a
// default root-level auth client for a Teleport server started by
// NewTeleportProcess.
func NewDefaultAuthClient(process *service.TeleportProcess) (*authclient.Client, error) {
	cfg := process.Config
	identity, err := storage.ReadLocalIdentityForRole(
		filepath.Join(cfg.DataDir, teleport.ComponentProcess),
		types.RoleAdmin,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authConfig := new(authclient.Config)
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Logger

	client, err := authclient.Connect(context.Background(), authConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}
