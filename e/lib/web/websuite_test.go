package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/user"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

// webSuite is a suite of components for testing SAML authentication. It has been
// copied from teleport/lib/web/apiserver_test.go and stripped down to just what
// is needed for the test cases in this package.
type webSuite struct {
	ctx    context.Context
	cancel context.CancelFunc

	user           string
	webServer      *httptest.Server
	testAuthServer *auth.TestServer
	proxyClient    *auth.Client
	clock          clockwork.FakeClock
}

type stubProxySettings struct{}

func (*stubProxySettings) GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error) {
	return &webclient.ProxySettings{}, nil
}

// stubTunnel stubs out the reversetunnel.Server for the web.Handler. None of
// tests here require the reversetunnel server so we use a stubbed implementation.
type stubTunnel struct{}

func (*stubTunnel) GetSites() ([]reversetunnel.RemoteSite, error)      { return nil, nil }
func (*stubTunnel) GetSite(n string) (reversetunnel.RemoteSite, error) { return nil, nil }

func newWebSuite(t *testing.T) *webSuite {
	u, err := user.Current()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	s := &webSuite{
		clock:  clockwork.NewFakeClock(),
		user:   u.Username,
		ctx:    ctx,
		cancel: cancel,
	}

	s.testAuthServer, err = auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			Dir:   t.TempDir(),
			Clock: s.clock,
		},
	})
	require.NoError(t, err)

	// Plug in SAML service
	sas, err := eauth.NewSAMLAuthService(&eauth.SAMLAuthServiceConfig{
		Auth: s.testAuthServer.Auth(),
	})
	require.NoError(t, err)
	s.testAuthServer.Auth().SetSAMLService(sas)

	err = s.testAuthServer.Auth().UpsertAuthServer(&types.ServerV2{
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
		I: auth.BuiltinRole{
			Role:     types.RoleProxy,
			Username: "proxy",
		},
	})
	require.NoError(t, err)

	pluginRegistry := plugin.NewRegistry()
	webPlugin, err := NewPlugin(Config{})
	require.NoError(t, err)
	err = pluginRegistry.Add(webPlugin)
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
	}, web.SetSessionStreamPollPeriod(200*time.Millisecond), web.SetClock(s.clock))
	require.NoError(t, err)

	s.webServer.Config.Handler = handler
	s.webServer.StartTLS()

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
	u, err := url.Parse("https://" + s.webServer.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	wc, err := client.NewWebClient(u.String(), opts...)
	if err != nil {
		panic(err)
	}
	return wc
}
