package web

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os/user"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/trait"
	apiutils "github.com/gravitational/teleport/api/utils"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/web"
)

// webSuite is a suite of components for testing enterprise web API endpoints. It has been
// copied from teleport/lib/web/apiserver_test.go and stripped down to just what
// is needed for the test cases in this package.
type webSuite struct {
	ctx                       context.Context
	cancel                    context.CancelFunc
	user                      string
	webServer                 *httptest.Server
	webServerURL              *url.URL
	testAuthServer            *authtest.Server
	webPlugin                 *Plugin
	authPlugin                *eauth.Plugin
	proxyClient               *authclient.Client
	clock                     clockwork.Clock
	accessGraphGrpcFile       *atomic.Int32
	accessGraphGrpcQuery      *atomic.Int32
	accessGraphHTTPFile       *atomic.Int32
	accessGraphHTTPQuery      *atomic.Int32
	identitycenterService     identitycenterService
	accessGraphHTTPValidation func(*testing.T, *http.Request)
}

type identitycenterService struct {
	identityCenter    services.IdentityCenter
	provisioningState services.ProvisioningStates
}

type stubProxySettings struct{}

func (*stubProxySettings) GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error) {
	return &webclient.ProxySettings{}, nil
}

type mockCluster struct {
	name string
	reversetunnelclient.Cluster
}

func (m mockCluster) GetName() string {
	return m.name
}

// stubTunnel stubs out the reversetunnelclient.Server for the web.Handler.
// tests here require the reversetunnelclient server so we use a stubbed implementation.
type stubTunnel struct {
	cluster reversetunnelclient.Cluster
}

func (s *stubTunnel) Clusters(context.Context) ([]reversetunnelclient.Cluster, error) {
	return []reversetunnelclient.Cluster{s.cluster}, nil
}

func (s *stubTunnel) Cluster(context.Context, string) (reversetunnelclient.Cluster, error) {
	return s.cluster, nil
}

type webSuiteOption func(*webSuiteOptions)

func withPlugin(p plugin.Plugin) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.customPlugin = p
	}
}

type webSuiteOptions struct {
	customPlugin                plugin.Plugin
	accessGraphFeatures         string
	runWhileLockedRetryInterval time.Duration
	clock                       clockwork.Clock
	roundTripper                http.RoundTripper
	accessGraphHTTPValidation   func(*testing.T, *http.Request)
	uploadHandler               events.MultipartHandler
	modules                     *modulestest.Modules
	enableAuthCache             bool
}

func withAccessGraphFeatures(features string) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.accessGraphFeatures = features
	}
}

func withRunWhileLockedRetryInterval(interval time.Duration) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.runWhileLockedRetryInterval = interval
	}
}

func withClock(clock clockwork.Clock) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.clock = clock
	}
}

func withRoundTripper(tr http.RoundTripper) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.roundTripper = tr
	}
}

func withAccessGraphValidation(f func(*testing.T, *http.Request)) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.accessGraphHTTPValidation = f
	}
}

func withUploadHandler(h events.MultipartHandler) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.uploadHandler = h
	}
}

func withModules(m *modulestest.Modules) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.modules = m
	}
}

func withWebPackAuthCacheEnabled(enable bool) webSuiteOption {
	return func(o *webSuiteOptions) {
		o.enableAuthCache = enable
	}
}

func newWebSuite(t *testing.T, opts ...webSuiteOption) *webSuite {
	var options webSuiteOptions
	for _, v := range opts {
		v(&options)
	}
	if options.clock == nil {
		// Old versions of clockwork used this as the initial time, setting it
		// explicitly as a quick fix.
		options.clock = clockwork.NewFakeClockAt(time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC))
	}

	if options.modules == nil {
		options.modules = modulestest.OSSModules()
	}

	u, err := user.Current()
	require.NoError(t, err)

	log := logtest.NewLogger()

	ctx, cancel := context.WithCancel(context.Background())
	s := &webSuite{
		clock:                     options.clock,
		user:                      u.Username,
		ctx:                       ctx,
		cancel:                    cancel,
		accessGraphHTTPValidation: options.accessGraphHTTPValidation,
	}

	pluginRegistry := plugin.NewRegistry()
	accessGraphServer := accessGraphFakeHTTPServer(t, s)
	accessGraphGRPCServerAddr := accessGraphGRPCServer(t, s, options)
	webPlugin, err := NewPlugin(Config{
		AccessGraph: &AccessGraphConfig{
			Addr:     accessGraphServer.Listener.Addr().String(),
			Insecure: true,
		},
		Clock:   s.clock,
		Logger:  log,
		Modules: options.modules,
	})
	s.webPlugin = webPlugin
	require.NoError(t, err)
	err = pluginRegistry.Add(webPlugin)
	require.NoError(t, err)
	authPlugin, err := eauth.NewPlugin(eauth.Config{
		Logger:  log,
		License: eauth.ValidLicense{},
		AccessGraph: servicecfg.AccessGraphConfig{
			Enabled:  true,
			Addr:     accessGraphGRPCServerAddr.String(),
			Insecure: true,
		},
		HostedPlugins: servicecfg.HostedPluginsConfig{
			Enabled: true,
			OAuthProviders: servicecfg.PluginOAuthProviders{
				SlackCredentials: &servicecfg.OAuthClientCredentials{
					ClientID:     "test",
					ClientSecret: "test",
				},
			},
		},
		HTTPTransport: options.roundTripper,
		Modules:       options.modules,
	})
	require.NoError(t, err)

	if options.customPlugin == nil ||
		(options.customPlugin != nil && options.customPlugin.GetName() != authPlugin.GetName()) {
		require.NoError(t, pluginRegistry.Add(authPlugin))
		s.authPlugin = authPlugin
	}

	if options.customPlugin != nil {
		err = pluginRegistry.Add(options.customPlugin)
		require.NoError(t, err)
	}

	s.testAuthServer, err = authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:   t.TempDir(),
			Clock: s.clock,
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactor: "otp",
			},
			RunWhileLockedRetryInterval: options.runWhileLockedRetryInterval,
			UploadHandler:               options.uploadHandler,
			Modules:                     options.modules,
			CacheEnabled:                options.enableAuthCache,
		},
		TLS: &authtest.TLSServerConfig{
			APIConfig: &auth.APIConfig{PluginRegistry: pluginRegistry},
		},
	})
	require.NoError(t, err)

	err = s.testAuthServer.Auth().UpsertAuthServer(ctx, &types.ServerV2{
		Kind:    types.KindAuthServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "auth",
		},
		Spec: types.ServerSpecV2{
			Addr:     s.testAuthServer.TLS.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	require.NoError(t, err)

	s.proxyClient, err = s.testAuthServer.NewClient(authtest.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleProxy,
			Username: "proxy",
		},
	})
	require.NoError(t, err)

	// Expired sessions are purged immediately
	var sessionLingeringThreshold time.Duration

	// Create web server with nil handler so we can get its listen address for the handler config
	s.webServer = httptest.NewUnstartedServer(nil)

	handler, err := web.NewHandler(web.Config{
		Proxy: &stubTunnel{
			cluster: &mockCluster{name: "localhost"},
		},
		ProxyWebAddr:                    *utils.MustParseAddr(s.webServer.Listener.Addr().String()),
		AuthServers:                     utils.FromAddr(s.testAuthServer.TLS.Addr()),
		ProxySSHAddr:                    *utils.MustParseAddr("127.0.0.1:3023"), // unused
		ProxyClient:                     s.proxyClient,
		CipherSuites:                    utils.DefaultCipherSuites(),
		AccessPoint:                     s.proxyClient,
		Context:                         s.ctx,
		Emitter:                         s.proxyClient,
		CachedSessionLingeringThreshold: &sessionLingeringThreshold,
		ProxySettings:                   &stubProxySettings{},
		PluginRegistry:                  pluginRegistry,
		GetProxyClientCertificate: func() (*tls.Certificate, error) {
			return nil, nil
		},
		Modules: options.modules,
		ClusterFeatures: proto.Features{
			// Turn on the enterprise features which impact the endpoint registration.
			Cloud:         true,
			RecoveryCodes: true,
			Entitlements: map[string]*proto.EntitlementInfo{
				string(entitlements.Policy):      {Enabled: true},
				string(entitlements.AccessGraph): {Enabled: true},
			},
		},
		IntegrationAppHandler: &mockIntegrationAppHandler{},
	}, web.SetClock(s.clock))
	require.NoError(t, err)

	s.webServer.Config.Handler = handler

	pool := x509.NewCertPool()

	cAs, err := s.testAuthServer.AuthServer.AuthServer.GetCertAuthorities(ctx, types.UserCA, false)
	require.NoError(t, err)
	for _, cA := range cAs {
		for _, cert := range cA.GetTrustedTLSKeyPairs() {
			pool.AppendCertsFromPEM(cert.Cert)
		}
	}

	s.webServer.TLS = &tls.Config{
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  pool,
	}

	s.webServer.StartTLS()

	serverURL, err := url.Parse("https://" + s.webServer.Listener.Addr().String())
	require.NoError(t, err)
	s.webServerURL = serverURL

	samlIdP, err := saml.New(s.ctx, saml.Config{
		Logger:      logtest.NewLogger(),
		Clock:       s.webPlugin.Clock,
		Client:      s.webPlugin.GetProxyClient(),
		AccessPoint: s.webPlugin.GetAccessPoint(),
		Authorizer:  s.testAuthServer.AuthServer.Authorizer,
		BaseURL:     s.webServerURL.String(),
		Emitter:     s.webPlugin.GetProxyClient(),
		HighLimiter: s.webPlugin.GetHighLimiter(),
	})
	require.NoError(t, err)
	s.webPlugin.RegisterSAMLIdP(samlIdP)

	icService, err := local.NewIdentityCenterService(local.IdentityCenterServiceConfig{
		Backend: s.testAuthServer.AuthServer.Backend,
	})
	require.NoError(t, err)
	prService, err := local.NewProvisioningStateService(s.testAuthServer.AuthServer.Backend)
	require.NoError(t, err)
	s.identitycenterService = identitycenterService{
		identityCenter:    icService,
		provisioningState: prService,
	}

	t.Cleanup(func() {
		s.cancel()
		s.webServer.Close()
		err := s.testAuthServer.Shutdown(context.Background())
		require.NoError(t, err)
	})

	return s
}

type mockIntegrationAppHandler struct{}

func (m *mockIntegrationAppHandler) HandleConnection(_ net.Conn) {
	panic("HandleConnection not implemented")
}

func (s *webSuite) clientNoRedirects(opts ...roundtrip.ClientParam) *client.WebClient {
	hclient := client.NewInsecureWebClient()
	hclient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	opts = append(opts, roundtrip.HTTPClient(hclient))

	wc, err := client.NewWebClient(s.webServerURL.String(), opts...)
	if err != nil {
		panic(err)
	}
	return wc
}

func (s *webSuite) newAdminAuthClient(ctx context.Context, t *testing.T) authclient.ClientI {
	tlsConfig, err := s.testAuthServer.TLS.ClientTLSConfig(authtest.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleAdmin,
			Username: "authcli",
		},
	})
	require.NoError(t, err)

	sshConfig, err := s.testAuthServer.TLS.Identity.SSHClientConfig(false)
	require.NoError(t, err)

	authClientConfig := &authclient.Config{
		TLS:                  tlsConfig,
		SSH:                  sshConfig,
		AuthServers:          []utils.NetAddr{utils.FromAddr(s.testAuthServer.TLS.Addr())},
		Log:                  slog.Default(),
		CircuitBreakerConfig: breaker.Config{},
	}

	client, err := authclient.Connect(ctx, authClientConfig)
	require.NoError(t, err)

	return client
}

type authWebPack struct {
	clt *TestWebClient
}

func (s *webSuite) testPassword() string {
	return "abc123def456"
}

func (s *webSuite) testOtpSecret() string {
	rawOTPSecret := "def456"
	return base32.StdEncoding.EncodeToString([]byte(rawOTPSecret))
}

type webSuiteOpts func(*webSuiteOpt)

type webSuiteOpt struct {
	skipUserCreation bool
	extraRules       []types.Rule
}

func skipUserCreation() webSuiteOpts {
	return func(opts *webSuiteOpt) {
		opts.skipUserCreation = true
	}
}

func withExtraRules(rules ...types.Rule) webSuiteOpts {
	return func(opts *webSuiteOpt) {
		opts.extraRules = rules
	}
}

// newAuthWebPack creates new user and returns authenticated http client for that user.
func (s *webSuite) newAuthWebPack(t *testing.T, user string, options ...webSuiteOpts) *authWebPack {
	// login is the login principal for websuite (equivalent to OS user).
	login := s.user
	pass := s.testPassword()
	otpSecret := s.testOtpSecret()

	opts := &webSuiteOpt{}

	for _, opt := range options {
		opt(opts)
	}

	if !opts.skipUserCreation {
		s.createUser(t, user, login, pass, otpSecret, opts.extraRules...)
	}
	dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
	require.NoError(t, err)
	err = s.testAuthServer.Auth().UpsertMFADevice(context.Background(), user, dev)
	require.NoError(t, err)
	validToken, err := totp.GenerateCode(otpSecret, s.clock.Now())
	require.NoError(t, err)

	clt := s.client(t)

	rawSess, err := s.login(clt, web.CreateSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: validToken,
	})
	require.NoError(t, err)

	var session *web.CreateSessionResponse
	require.NoError(t, json.Unmarshal(rawSess.Bytes(), &session))

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	jar.SetCookies(s.webServerURL, rawSess.Cookies())
	clt = s.client(t, roundtrip.BearerAuth(session.Token), roundtrip.CookieJar(jar))
	return &authWebPack{clt: clt}
}

type TestWebClient struct {
	*client.WebClient
}

func (s *webSuite) createUser(t *testing.T, user string, login string, pass string, otpSecret string, extraRules ...types.Rule) {
	teleUser, err := types.NewUser(user)
	require.NoError(t, err)
	rules := []types.Rule{
		types.NewRule(types.KindUser, services.RW()),
		types.NewRule(types.KindRole, services.RW()),
		types.NewRule(types.KindOIDC, services.RW()),
		types.NewRule(types.KindSAML, services.RW()),
		types.NewRule(types.KindGithub, services.RW()),
		types.NewRule(types.KindOIDCRequest, services.RW()),
		types.NewRule(types.KindSAMLRequest, services.RW()),
		types.NewRule(types.KindGithubRequest, services.RW()),
		types.NewRule(types.KindClusterAuditConfig, services.RW()),
		types.NewRule(types.KindClusterAuthPreference, services.RW()),
		types.NewRule(types.KindAuthConnector, services.RW()),
		types.NewRule(types.KindClusterName, services.RW()),
		types.NewRule(types.KindClusterNetworkingConfig, services.RW()),
		types.NewRule(types.KindSessionRecordingConfig, services.RW()),
		types.NewRule(types.KindExternalAuditStorage, services.RW()),
		types.NewRule(types.KindUIConfig, services.RW()),
		types.NewRule(types.KindTrustedCluster, services.RW()),
		types.NewRule(types.KindRemoteCluster, services.RW()),
		types.NewRule(types.KindToken, services.RW()),
		types.NewRule(types.KindConnectionDiagnostic, services.RW()),
		types.NewRule(types.KindDatabase, services.RW()),
		types.NewRule(types.KindDatabaseCertificate, services.RW()),
		types.NewRule(types.KindInstaller, services.RW()),
		types.NewRule(types.KindDevice, append(services.RW(), types.VerbCreateEnrollToken, types.VerbEnroll)),
		types.NewRule(types.KindDatabaseService, services.RO()),
		types.NewRule(types.KindInstance, services.RO()),
		types.NewRule(types.KindLoginRule, services.RW()),
		types.NewRule(types.KindSAMLIdPServiceProvider, services.RW()),
		types.NewRule(types.KindUserGroup, services.RW()),
		types.NewRule(types.KindPlugin, services.RW()),
		types.NewRule(types.KindOktaImportRule, services.RW()),
		types.NewRule(types.KindOktaAssignment, services.RW()),
		types.NewRule(types.KindLock, services.RW()),
		types.NewRule(types.KindIntegration, append(services.RW(), types.VerbUse)),
		types.NewRule(types.KindBilling, services.RW()),
		types.NewRule(types.KindLicense, services.RO()),
		types.NewRule(types.KindClusterAlert, services.RW()),
		types.NewRule(types.KindAccessList, services.RW()),
		types.NewRule(types.KindNode, services.RW()),
		types.NewRule(types.KindDiscoveryConfig, services.RW()),
		types.NewRule(types.KindAccessMonitoringRule, services.RW()),
		types.NewRule(types.KindCrownJewel, services.RW()),
		types.NewRule(types.KindIdentityCenter, services.RW()),
		types.NewRule(types.KindInferenceModel, services.RW()),
		types.NewRule(types.KindInferenceSecret, services.RW()),
		types.NewRule(types.KindInferencePolicy, services.RW()),
		types.NewRule(types.KindBeam, services.RW()),
	}
	rules = append(rules, extraRules...)
	role, err := authtest.CreateRole(s.ctx, s.testAuthServer.Auth(), "editor", types.RoleSpecV6{
		Options: types.RoleOptions{
			CertificateFormat: constants.CertificateFormatStandard,
			MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
			PortForwarding:    types.NewBoolOption(true),
			ForwardAgent:      types.NewBool(true),
			BPF:               apidefaults.EnhancedEvents(),
			RecordSession: &types.RecordSession{
				Desktop: types.NewBoolOption(false),
			},
		},
		Allow: types.RoleConditions{
			Logins:    []string{login},
			Rules:     rules,
			AppLabels: types.Labels{types.Wildcard: apiutils.Strings{types.Wildcard}},
		},
	})
	require.NoError(t, err)

	teleUser.AddRole(role.GetName())
	teleUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})

	_, err = s.testAuthServer.Auth().CreateUser(s.ctx, teleUser)
	require.NoError(t, err)

	err = s.testAuthServer.Auth().UpsertPassword(user, []byte(pass))
	require.NoError(t, err)
}

func (s *webSuite) client(t *testing.T, opts ...roundtrip.ClientParam) *TestWebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	wc, err := client.NewWebClient(s.webServerURL.String(), opts...)
	require.NoError(t, err)

	return &TestWebClient{wc}
}

func (s *webSuite) login(clt *TestWebClient, reqData web.CreateSessionReq) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(reqData)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", clt.Endpoint("webapi", "sessions", "web"), bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		return clt.HTTPClient().Do(req)
	}))
}

const features = `{"grv_teleport_access_graph_http_enabled": true}`

func accessGraphFakeHTTPServer(t *testing.T, suite *webSuite) *httptest.Server {
	suite.accessGraphHTTPFile = new(atomic.Int32)
	suite.accessGraphHTTPQuery = new(atomic.Int32)
	mux := http.NewServeMux()
	mux.HandleFunc("/static/features.json", func(w http.ResponseWriter, r *http.Request) {
		suite.accessGraphHTTPFile.Add(1)
		fmt.Fprint(w, features)
	})
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		suite.accessGraphHTTPQuery.Add(1)
		w.WriteHeader(http.StatusOK)
		if suite.accessGraphHTTPValidation != nil {
			suite.accessGraphHTTPValidation(t, r)
		}
		w.Write([]byte("fake access graph response"))
	})
	// Create a REST API endpoint as TAG/Teleport handles /query and /static endpoint differently.
	mux.HandleFunc("/graph/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if suite.accessGraphHTTPValidation != nil {
			suite.accessGraphHTTPValidation(t, r)
		}
		w.Write([]byte("fake access graph response"))
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func accessGraphGRPCServer(t *testing.T, suite *webSuite, opts webSuiteOptions) net.Addr {
	cert, err := tls.X509KeyPair([]byte(fixtures.EncryptionCertPEM), []byte(fixtures.EncryptionKeyPEM))
	require.NoError(t, err)
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(
		&tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		},
	)))
	suite.accessGraphGrpcFile = new(atomic.Int32)
	suite.accessGraphGrpcQuery = new(atomic.Int32)
	accessgraphv1alpha.RegisterAccessGraphServiceServer(grpcServer, &fakeAccessGraphServer{
		queryCounter: suite.accessGraphGrpcQuery,
		fileCounter:  suite.accessGraphGrpcFile,
		features:     opts.accessGraphFeatures,
	})
	t.Cleanup(grpcServer.Stop)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	go grpcServer.Serve(lis)
	return lis.Addr()
}

type fakeAccessGraphServer struct {
	accessgraphv1alpha.UnimplementedAccessGraphServiceServer
	queryCounter *atomic.Int32
	fileCounter  *atomic.Int32
	features     string
}

func (f *fakeAccessGraphServer) Query(_ context.Context, req *accessgraphv1alpha.QueryRequest) (*accessgraphv1alpha.QueryResponse, error) {
	f.queryCounter.Add(1)
	return &accessgraphv1alpha.QueryResponse{}, nil
}

func (f *fakeAccessGraphServer) GetFile(_ context.Context, req *accessgraphv1alpha.GetFileRequest) (*accessgraphv1alpha.GetFileResponse, error) {
	f.fileCounter.Add(1)
	if strings.TrimLeft(req.GetFilepath(), "/") != "features.json" {
		return nil, fmt.Errorf("file not found")
	}
	return accessgraphv1alpha.GetFileResponse_builder{
		Data: []byte(f.features),
	}.Build(), nil
}

type createUserOpts struct {
	roles        []string
	traits       trait.Traits
	withPassword bool
}

type createUserOpt func(*createUserOpts)

func withRoles(roles ...string) createUserOpt {
	return func(opts *createUserOpts) {
		opts.roles = roles
	}
}

func withTraits(traits trait.Traits) createUserOpt {
	return func(opts *createUserOpts) {
		opts.traits = traits
	}
}

func withPassword() createUserOpt {
	return func(opts *createUserOpts) {
		opts.withPassword = true
	}
}

// createUserWithOpts creates a user with the specified options.
func createUserWithOpts(t *testing.T, s *webSuite, username string, opts ...createUserOpt) types.User {
	t.Helper()

	u, err := types.NewUser(username)
	require.NoError(t, err)

	userOpts := createUserOpts{}
	for _, opt := range opts {
		opt(&userOpts)
	}
	if len(userOpts.roles) > 0 {
		u.SetRoles(userOpts.roles)
	}
	if len(userOpts.traits) > 0 {
		u.SetTraits(userOpts.traits)
	}

	u, err = s.testAuthServer.Auth().Services.UpsertUser(t.Context(), u)
	require.NoError(t, err)

	if userOpts.withPassword {
		err = s.testAuthServer.Auth().UpsertPassword(username, []byte(s.testPassword()))
		require.NoError(t, err)
	}

	return u
}
