package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
)

const (
	staticToken = "test-static-token"
)

func makeTestServers(t *testing.T, opts ...testServerOptFunc) (auth *service.TeleportProcess, proxy *service.TeleportProcess) {
	t.Helper()

	var options testServersOpts
	for _, opt := range opts {
		opt(&options)
	}

	authAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	var err error
	// Set up a test auth server.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()
	cfg.SetAuthServerAddress(authAddr)
	cfg.Auth.BootstrapResources = options.bootstrap
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   staticToken,
		}},
	})
	require.NoError(t, err)
	cfg.SetToken(staticToken)
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = true
	cfg.Auth.ListenAddr = authAddr
	cfg.Proxy.Enabled = true
	cfg.Proxy.WebAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.ReverseTunnelListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.DisableWebInterface = true
	cfg.Log = utils.NewLoggerForTests()

	for _, fn := range options.configFuncs {
		fn(cfg)
	}

	auth = runTeleport(t, cfg)

	// Wait for auth to become ready.
	_, err = auth.WaitForEventTimeout(30*time.Second, service.AuthTLSReady)
	// in reality, the auth server should start *much* sooner than this.  we use a very large
	// timeout here because this isn't the kind of problem that this test is meant to catch.
	require.NoError(t, err, "auth server didn't start after 30s")

	return auth, auth
}

func makeTestSSHNode(t *testing.T, authAddr *utils.NetAddr, opts ...testServerOptFunc) *service.TeleportProcess {
	var options testServersOpts
	for _, opt := range opts {
		opt(&options)
	}

	// Set up a test ssh service.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Hostname = "node"
	cfg.DataDir = t.TempDir()

	cfg.SetAuthServerAddress(*authAddr)
	cfg.SetToken(staticToken)
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *utils.MustParseAddr("127.0.0.1:0")
	cfg.SSH.PublicAddrs = []utils.NetAddr{cfg.SSH.Addr}
	cfg.SSH.DisableCreateHostUser = true
	cfg.Log = utils.NewLoggerForTests()

	for _, fn := range options.configFuncs {
		fn(cfg)
	}

	return runTeleport(t, cfg)
}

func runTeleport(t *testing.T, cfg *servicecfg.Config) *service.TeleportProcess {
	if cfg.InstanceMetadataClient == nil {
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
		cfg.InstanceMetadataClient = cloud.NewDisabledIMDSClient()
	}
	process, err := service.NewTeleport(cfg)
	require.NoError(t, err, trace.DebugReport(err))
	require.NoError(t, process.Start())
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

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
	waitForEvents(t, process, serviceReadyEvents...)

	if cfg.Auth.Enabled && cfg.Databases.Enabled {
		waitForDatabases(t, process, cfg.Databases.Databases)
	}
	return process
}

func waitForEvents(t *testing.T, svc service.Supervisor, events ...string) {
	for _, event := range events {
		_, err := svc.WaitForEventTimeout(30*time.Second, event)
		require.NoError(t, err, "service server didn't receive %v event after 30s", event)
	}
}

func localListenerAddr() string {
	return fmt.Sprintf("localhost:%d", ports.PopInt())
}

type testServersOpts struct {
	bootstrap   []types.Resource
	configFuncs []func(cfg *servicecfg.Config)
}

type testServerOptFunc func(o *testServersOpts)

func withBootstrap(bootstrap ...types.Resource) testServerOptFunc {
	return func(o *testServersOpts) {
		o.bootstrap = bootstrap
	}
}

func withConfig(fn func(cfg *servicecfg.Config)) testServerOptFunc {
	return func(o *testServersOpts) {
		o.configFuncs = append(o.configFuncs, fn)
	}
}

func withAuthConfig(fn func(*servicecfg.AuthConfig)) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		fn(&cfg.Auth)
	})
}

func withClusterName(t *testing.T, n string) testServerOptFunc {
	return withAuthConfig(func(cfg *servicecfg.AuthConfig) {
		clusterName, err := services.NewClusterNameWithRandomID(
			types.ClusterNameSpecV2{
				ClusterName: n,
			})
		require.NoError(t, err)
		cfg.ClusterName = clusterName
	})
}

func withMOTD(t *testing.T, motd string) testServerOptFunc {
	oldStdin := prompt.Stdin()
	t.Cleanup(func() {
		prompt.SetStdin(oldStdin)
	})
	prompt.SetStdin(prompt.NewFakeReader().
		AddString(""). // 3x to allow multiple logins
		AddString("").
		AddString(""))
	return withAuthConfig(func(cfg *servicecfg.AuthConfig) {
		fmt.Printf("\n\n Setting MOTD: '%s' \n\n", motd)
		cfg.Preference.SetMessageOfTheDay(motd)
	})
}

func withHostname(hostname string) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		cfg.Hostname = hostname
	})
}

func withSSHAddr(addr string) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		cfg.SSH.Addr = *utils.MustParseAddr(addr)
	})
}

func withSSHPublicAddrs(addrs ...string) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		cfg.SSH.PublicAddrs = utils.MustParseAddrList(addrs...)
	})
}

func withSSHLabel(key, value string) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		if cfg.SSH.Labels == nil {
			cfg.SSH.Labels = make(map[string]string)
		}
		cfg.SSH.Labels[key] = value
	})
}

func mustLogin(t *testing.T, proxyAddr string, opts ...cliOption) {
	err := Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr,
	}, opts...)
	require.NoError(t, err)
}

func mustLoginHome(t *testing.T, authServer *auth.Server, proxyAddr, user, connectorName string, opts ...cliOption) (tshHome string, kubeConfig string, loginOpt cliOption) {
	tshHome = t.TempDir()
	kubeConfig = filepath.Join(t.TempDir(), teleport.KubeConfigFile)

	mustLogin(t, proxyAddr, append(opts, setHomePath(tshHome), setKubeConfigPath(kubeConfig), setMockSSOLogin(t, authServer, user, connectorName))...)

	return tshHome, kubeConfig, func(cf *CLIConf) error {
		cf.HomePath = tshHome
		cf.kubeConfigPath = kubeConfig
		return nil
	}
}

func mustLoginIdentity(t *testing.T, authServer *auth.Server, proxyAddr, user, connectorName string, opts ...cliOption) (identityFilePath string, loginOpt cliOption) {
	identityFilePath = path.Join(t.TempDir(), "identity.pem")

	mustLogin(t, proxyAddr, append(opts, setMockSSOLogin(t, authServer, user, connectorName))...)

	err := Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr,
		"--out", identityFilePath,
	}, append(opts, setMockSSOLogin(t, authServer, user, connectorName))...)
	require.NoError(t, err)

	return identityFilePath, setIdentity(identityFilePath, proxyAddr)
}

func mockConnector(t *testing.T) types.OIDCConnector {
	// Connector need not be functional since we are going to mock the actual
	// login operation.
	connector, err := types.NewOIDCConnector("auth.example.com", types.OIDCConnectorSpecV3{
		IssuerURL:    "https://auth.example.com",
		RedirectURLs: []string{"https://cluster.example.com"},
		ClientID:     "fake-client",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "dummy",
				Roles: []string{"dummy"},
			},
		},
	})
	require.NoError(t, err)
	return connector
}

func mockSSOLogin(t *testing.T, authServer *auth.Server, user string) client.SSOLoginFunc {
	return func(ctx context.Context, _ string, priv *keys.PrivateKey, protocol string) (*auth.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			Key:            priv.MarshalSSHPublicKey(),
			Username:       user,
			TTL:            time.Hour,
			Compatibility:  constants.CertificateFormatStandard,
			RouteToCluster: clusterName.GetClusterName(),
		})
		require.NoError(t, err)

		// load CA cert
		authority, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		require.NoError(t, err)

		// build login response
		return &auth.SSHLoginResponse{
			Username:    user,
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: auth.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func mockHeadlessLogin(t *testing.T, authServer *auth.Server, user string) client.SSHLoginFunc {
	return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			Key:            priv.MarshalSSHPublicKey(),
			Username:       user,
			TTL:            time.Hour,
			Compatibility:  constants.CertificateFormatStandard,
			RouteToCluster: clusterName.GetClusterName(),
			MFAVerified:    "mfa-verified",
		})
		require.NoError(t, err)

		// load CA cert
		authority, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		require.NoError(t, err)

		// build login response
		return &auth.SSHLoginResponse{
			Username:    user,
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: auth.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func setupWebAuthnChallengeSolver(t *testing.T, device *mocku2f.Key, origin string, success bool) {
	oldWebauthn := *client.PromptWebauthn
	t.Cleanup(func() {
		*client.PromptWebauthn = oldWebauthn
	})

	*client.PromptWebauthn = func(ctx context.Context, realOrigin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		car, err := device.SignAssertion(origin, assertion) // use the fake origin to prevent a mismatch
		if err != nil {
			return nil, "", err
		}

		carProto := wanlib.CredentialAssertionResponseToProto(car)
		if !success {
			carProto.Type = "NOT A VALID TYPE" // set to an invalid type so the ceremony fails
		}

		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: carProto,
			},
		}, "", nil
	}
}

func setOverrideStdout(stdout io.Writer) cliOption {
	return func(cf *CLIConf) error {
		cf.overrideStdout = stdout
		return nil
	}
}

func setOverrideStderr(stderr io.Writer) cliOption {
	return func(cf *CLIConf) error {
		cf.overrideStderr = stderr
		return nil
	}
}

func setHomePath(path string) cliOption {
	return func(cf *CLIConf) error {
		cf.HomePath = path
		return nil
	}
}

func setKubeConfigPath(path string) cliOption {
	return func(cf *CLIConf) error {
		cf.kubeConfigPath = path
		return nil
	}
}

func setIdentity(path, proxy string) cliOption {
	return func(cf *CLIConf) error {
		cf.IdentityFileIn = path
		cf.Proxy = proxy
		return nil
	}
}

func setIdentityOut(path string) cliOption {
	return func(cf *CLIConf) error {
		cf.IdentityFileOut = path
		return nil
	}
}

func setClusterName(clusterName string) cliOption {
	return func(cf *CLIConf) error {
		cf.SiteName = clusterName
		return nil
	}
}

func setCmdRunner(cmdRunner func(*exec.Cmd) error) cliOption {
	return func(cf *CLIConf) error {
		cf.cmdRunner = cmdRunner
		return nil
	}
}

func setMockSSOLogin(t *testing.T, authServer *auth.Server, user, connectorName string) cliOption {
	return func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, authServer, user)
		cf.AuthConnector = connectorName
		return nil
	}
}

func setMockHeadlessLogin(t *testing.T, authServer *auth.Server, user, proxy string) cliOption {
	return func(cf *CLIConf) error {
		cf.mockHeadlessLogin = mockHeadlessLogin(t, authServer, user)
		cf.Headless = true
		cf.Username = user
		cf.Proxy = proxy
		cf.ExplicitUsername = true
		return nil
	}
}
