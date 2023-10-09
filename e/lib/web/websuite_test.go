package web

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os/user"
	"testing"
	"time"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/gravitational/roundtrip"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

// webSuite is a suite of components for testing enterprise web API endpoints. It has been
// copied from teleport/lib/web/apiserver_test.go and stripped down to just what
// is needed for the test cases in this package.
type webSuite struct {
	ctx            context.Context
	cancel         context.CancelFunc
	user           string
	webServer      *httptest.Server
	webServerURL   *url.URL
	testAuthServer *auth.TestServer
	webPlugin      *Plugin
	proxyClient    *auth.Client
	clock          clockwork.FakeClock
}

type stubProxySettings struct{}

func (*stubProxySettings) GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error) {
	return &webclient.ProxySettings{}, nil
}

// stubTunnel stubs out the reversetunnelclient.Server for the web.Handler. None of
// tests here require the reversetunnelclient server so we use a stubbed implementation.
type stubTunnel struct{}

func (*stubTunnel) GetSites() ([]reversetunnelclient.RemoteSite, error)      { return nil, nil }
func (*stubTunnel) GetSite(n string) (reversetunnelclient.RemoteSite, error) { return nil, nil }

func newWebSuite(t *testing.T) *webSuite {
	u, err := user.Current()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	s := &webSuite{
		// Old versions of clockwork used this as the initial time, setting it
		// explicitly as a quick fix.
		clock:  clockwork.NewFakeClockAt(time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC)),
		user:   u.Username,
		ctx:    ctx,
		cancel: cancel,
	}

	pluginRegistry := plugin.NewRegistry()
	webPlugin, err := NewPlugin(Config{})
	s.webPlugin = webPlugin
	require.NoError(t, err)
	err = pluginRegistry.Add(webPlugin)
	require.NoError(t, err)
	authPlugin, err := eauth.NewPlugin(eauth.Config{
		License: eauth.ValidLicense{},
		HostedPlugins: servicecfg.HostedPluginsConfig{
			Enabled: true,
			OAuthProviders: servicecfg.PluginOAuthProviders{
				Slack: &oauth2.ClientCredentials{
					ID:     "test",
					Secret: "test",
				},
			},
		},
	})
	require.NoError(t, err)
	err = pluginRegistry.Add(authPlugin)
	require.NoError(t, err)

	s.testAuthServer, err = auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			Dir:   t.TempDir(),
			Clock: s.clock,
		},
		TLS: &auth.TestTLSServerConfig{
			APIConfig: &auth.APIConfig{PluginRegistry: pluginRegistry},
		},
	})
	require.NoError(t, err)

	err = s.testAuthServer.Auth().UpsertAuthServer(ctx, &types.ServerV2{
		Kind:    types.KindAuthServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: apidefaults.Namespace,
			Name:      "auth",
		},
		Spec: types.ServerSpecV2{
			Addr:     s.testAuthServer.TLS.Listener.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	require.NoError(t, err)

	s.proxyClient, err = s.testAuthServer.NewClient(auth.TestIdentity{
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
		Proxy:                           &stubTunnel{},
		ProxyWebAddr:                    *utils.MustParseAddr(s.webServer.Listener.Addr().String()),
		AuthServers:                     utils.FromAddr(s.testAuthServer.TLS.Addr()),
		DomainName:                      s.testAuthServer.ClusterName(),
		ProxySSHAddr:                    *utils.MustParseAddr("127.0.0.1:3023"), // unused
		ProxyClient:                     s.proxyClient,
		CipherSuites:                    utils.DefaultCipherSuites(),
		AccessPoint:                     s.proxyClient,
		Context:                         s.ctx,
		Emitter:                         s.proxyClient,
		CachedSessionLingeringThreshold: &sessionLingeringThreshold,
		ProxySettings:                   &stubProxySettings{},
		PluginRegistry:                  pluginRegistry,
		ClusterFeatures: proto.Features{
			// Turn on the enterprise features which impact the endpoint registration.
			Cloud:         true,
			RecoveryCodes: true,
		},
	}, web.SetSessionStreamPollPeriod(200*time.Millisecond), web.SetClock(s.clock))
	require.NoError(t, err)

	s.webServer.Config.Handler = handler
	s.webServer.StartTLS()

	serverURL, err := url.Parse("https://" + s.webServer.Listener.Addr().String())
	require.NoError(t, err)
	s.webServerURL = serverURL

	t.Cleanup(func() {
		s.cancel()
		s.webServer.Close()
		err := s.testAuthServer.Shutdown(context.Background())
		require.NoError(t, err)
	})

	return s
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

func (s *webSuite) newAdminAuthClient(ctx context.Context, t *testing.T) auth.ClientI {
	tlsConfig, err := s.testAuthServer.TLS.ClientTLSConfig(auth.TestIdentity{
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
		Log:                  logrus.StandardLogger(),
		CircuitBreakerConfig: breaker.Config{},
	}

	client, err := authclient.Connect(ctx, authClientConfig)
	require.NoError(t, err)

	return client
}

type authWebPack struct {
	clt       *TestWebClient
	csrfToken string
}

func (s *webSuite) testPassword() string {
	return "abc123"
}

func (s *webSuite) testOtpSecret() string {
	rawOTPSecret := "def456"
	return base32.StdEncoding.EncodeToString([]byte(rawOTPSecret))
}

type webSuiteOpts func(*webSuiteOpt)

type webSuiteOpt struct {
	skipUserCreation bool
}

func skipUserCreation() webSuiteOpts {
	return func(opts *webSuiteOpt) {
		opts.skipUserCreation = true
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
		s.createUser(t, user, login, pass, otpSecret)
	}

	validToken, err := totp.GenerateCode(otpSecret, s.clock.Now())
	require.NoError(t, err)

	clt := s.client(t)

	const csrfToken = "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	rawSess, err := s.login(clt, csrfToken, web.CreateSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: validToken,
	})
	require.NoError(t, err)

	var session *web.CreateSessionResponse
	require.NoError(t, json.Unmarshal(rawSess.Bytes(), &session))

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	sessionCookieWithCSRF := append(rawSess.Cookies(), &http.Cookie{
		Name:  csrf.CookieName,
		Value: csrfToken,
	})

	jar.SetCookies(s.webServerURL, sessionCookieWithCSRF)

	clt = s.client(t, roundtrip.BearerAuth(session.Token), roundtrip.CookieJar(jar))

	return &authWebPack{
		clt:       clt,
		csrfToken: csrfToken,
	}
}

type TestWebClient struct {
	*client.WebClient
}

func (s *webSuite) createUser(t *testing.T, user string, login string, pass string, otpSecret string) {
	teleUser, err := types.NewUser(user)
	require.NoError(t, err)

	role := services.NewPresetEditorRole()
	role.SetLogins(types.Allow, []string{login})

	err = s.testAuthServer.Auth().UpsertRole(s.ctx, role)
	require.NoError(t, err)

	teleUser.AddRole(role.GetName())
	teleUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})

	_, err = s.testAuthServer.Auth().CreateUserWithContext(s.ctx, teleUser)
	require.NoError(t, err)

	err = s.testAuthServer.Auth().UpsertPassword(user, []byte(pass))
	require.NoError(t, err)

	if otpSecret != "" {
		dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
		require.NoError(t, err)
		err = s.testAuthServer.Auth().UpsertMFADevice(context.Background(), user, dev)
		require.NoError(t, err)
	}
}

func (s *webSuite) client(t *testing.T, opts ...roundtrip.ClientParam) *TestWebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	wc, err := client.NewWebClient(s.webServerURL.String(), opts...)
	require.NoError(t, err)

	return &TestWebClient{wc}
}

func (s *webSuite) login(clt *TestWebClient, csrfToken string, reqData web.CreateSessionReq) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(reqData)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", clt.Endpoint("webapi", "sessions", "web"), bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}

		req.AddCookie(&http.Cookie{
			Name:  csrf.CookieName,
			Value: csrfToken,
		})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrf.HeaderName, csrfToken)
		return clt.HTTPClient().Do(req)
	}))
}
