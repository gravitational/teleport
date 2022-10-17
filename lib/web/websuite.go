package web

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os/user"
	"testing"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/plugin"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const hostID = "00000000-0000-0000-0000-000000000000"

// TestWebSuite is a suite of components for testing the web package. It exists
// as an exported struct not in a _test.go file so that it can be used from
// outside this package by external packages that extend the web API (such as
// SAML and OIDC auth connectors in the enterprise edition).
type TestWebSuite struct {
	Ctx    context.Context
	Cancel context.CancelFunc

	Node        *regular.Server
	Proxy       *regular.Server
	ProxyTunnel reversetunnel.Server
	SrvID       string

	User      string
	WebServer *httptest.Server

	MockU2F     *mocku2f.Key
	Server      *auth.TestServer
	ProxyClient *auth.Client
	Clock       clockwork.FakeClock
}

type TestWebSuiteConfig struct {
	AssetDir       string
	PluginRegistry plugin.Registry
}

type TestWebSuiteOption func(cfg *TestWebSuiteConfig)

// WithWebSuiteAssetDir configures a TestWebSuite with an asset directory
// other than the default of ../../webassets/teleport, as this path only
// works from one level of the directory hierarchy.
func WithWebSuiteAssetDir(dir string) TestWebSuiteOption {
	return func(cfg *TestWebSuiteConfig) {
		cfg.AssetDir = dir
	}
}

// WithWebSuitePluginRegistry configures a TestWebSuite with a plugin
// registry for the web.Handler created for the test suite, allowing external
// plugins to configure a web suite for testing
func WithWebSuitePluginRegistry(reg plugin.Registry) TestWebSuiteOption {
	return func(cfg *TestWebSuiteConfig) {
		cfg.PluginRegistry = reg
	}
}

func NewTestWebSuite(t *testing.T, opts ...TestWebSuiteOption) *TestWebSuite {
	cfg := &TestWebSuiteConfig{
		AssetDir: "../../websuite/teleport",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	mockU2F, err := mocku2f.Create()
	require.NoError(t, err)
	require.NotNil(t, mockU2F)

	u, err := user.Current()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	s := &TestWebSuite{
		MockU2F: mockU2F,
		Clock:   clockwork.NewFakeClock(),
		User:    u.Username,
		Ctx:     ctx,
		Cancel:  cancel,
	}

	networkingConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		KeepAliveInterval: types.Duration(10 * time.Second),
	})
	require.NoError(t, err)

	s.Server, err = auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName:             "localhost",
			Dir:                     t.TempDir(),
			Clock:                   s.Clock,
			ClusterNetworkingConfig: networkingConfig,
		},
	})
	require.NoError(t, err)

	// Register the auth server, since test auth server doesn't start its own
	// heartbeat.
	err = s.Server.Auth().UpsertAuthServer(&types.ServerV2{
		Kind:    types.KindAuthServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: apidefaults.Namespace,
			Name:      "auth",
		},
		Spec: types.ServerSpecV2{
			Addr:     s.Server.TLS.Listener.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	require.NoError(t, err)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	// start node
	certs, err := s.Server.Auth().GenerateHostCerts(s.Ctx,
		&authproto.HostCertsRequest{
			HostID:       hostID,
			NodeName:     s.Server.ClusterName(),
			Role:         types.RoleNode,
			PublicSSHKey: pub,
			PublicTLSKey: tlsPub,
		})
	require.NoError(t, err)

	signer, err := sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)

	nodeID := "node"
	nodeClient, err := s.Server.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)

	nodeLockWatcher, err := services.NewLockWatcher(s.Ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Client:    nodeClient,
		},
	})
	require.NoError(t, err)

	// create SSH service:
	nodeDataDir := t.TempDir()
	node, err := regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.Server.ClusterName(),
		[]ssh.Signer{signer},
		nodeClient,
		nodeDataDir,
		"",
		utils.NetAddr{},
		nodeClient,
		regular.SetUUID(nodeID),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetShell("/bin/sh"),
		regular.SetEmitter(nodeClient),
		regular.SetPAMConfig(&pam.Config{Enabled: false}),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetRestrictedSessionManager(&restricted.NOP{}),
		regular.SetClock(s.Clock),
		regular.SetLockWatcher(nodeLockWatcher),
	)
	require.NoError(t, err)
	s.Node = node
	s.SrvID = node.ID()
	require.NoError(t, s.Node.Start())
	require.NoError(t, auth.CreateUploaderDir(nodeDataDir))

	// create reverse tunnel service:
	proxyID := "proxy"
	s.ProxyClient, err = s.Server.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     types.RoleProxy,
			Username: proxyID,
		},
	})
	require.NoError(t, err)

	revTunListener, err := net.Listen("tcp", fmt.Sprintf("%v:0", s.Server.ClusterName()))
	require.NoError(t, err)

	proxyLockWatcher, err := services.NewLockWatcher(s.Ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.ProxyClient,
		},
	})
	require.NoError(t, err)

	proxyNodeWatcher, err := services.NewNodeWatcher(s.Ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.ProxyClient,
		},
	})
	require.NoError(t, err)

	caWatcher, err := services.NewCertAuthorityWatcher(s.Ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.ProxyClient,
		},
		Types: []types.CertAuthType{types.HostCA, types.UserCA},
	})
	require.NoError(t, err)
	defer caWatcher.Close()

	revTunServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:                    node.ID(),
		Listener:              revTunListener,
		ClientTLS:             s.ProxyClient.TLSConfig(),
		ClusterName:           s.Server.ClusterName(),
		HostSigners:           []ssh.Signer{signer},
		LocalAuthClient:       s.ProxyClient,
		LocalAccessPoint:      s.ProxyClient,
		Emitter:               s.ProxyClient,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		LockWatcher:           proxyLockWatcher,
		NodeWatcher:           proxyNodeWatcher,
		CertAuthorityWatcher:  caWatcher,
		CircuitBreakerConfig:  breaker.NoopBreakerConfig(),
		LocalAuthAddresses:    []string{s.Server.TLS.Listener.Addr().String()},
	})
	require.NoError(t, err)
	s.ProxyTunnel = revTunServer

	// proxy server:
	s.Proxy, err = regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.Server.ClusterName(),
		[]ssh.Signer{signer},
		s.ProxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		s.ProxyClient,
		regular.SetUUID(proxyID),
		regular.SetProxyMode("", revTunServer, s.ProxyClient),
		regular.SetEmitter(s.ProxyClient),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetRestrictedSessionManager(&restricted.NOP{}),
		regular.SetClock(s.Clock),
		regular.SetLockWatcher(proxyLockWatcher),
		regular.SetNodeWatcher(proxyNodeWatcher),
	)
	require.NoError(t, err)

	// Expired sessions are purged immediately
	var sessionLingeringThreshold time.Duration
	fs, err := NewDebugFileSystem(cfg.AssetDir)
	require.NoError(t, err)
	handler, err := NewHandler(Config{
		Proxy:                           revTunServer,
		AuthServers:                     utils.FromAddr(s.Server.TLS.Addr()),
		DomainName:                      s.Server.ClusterName(),
		ProxyClient:                     s.ProxyClient,
		CipherSuites:                    utils.DefaultCipherSuites(),
		AccessPoint:                     s.ProxyClient,
		Context:                         s.Ctx,
		HostUUID:                        proxyID,
		Emitter:                         s.ProxyClient,
		StaticFS:                        fs,
		CachedSessionLingeringThreshold: &sessionLingeringThreshold,
		ProxySettings:                   &mockProxySettings{},
		PluginRegistry:                  cfg.PluginRegistry,
	}, SetSessionStreamPollPeriod(200*time.Millisecond), SetClock(s.Clock))
	require.NoError(t, err)

	s.WebServer = httptest.NewUnstartedServer(handler)
	s.WebServer.StartTLS()
	err = s.Proxy.Start()
	require.NoError(t, err)

	// Wait for proxy to fully register before starting the test.
	for start := time.Now(); ; {
		proxies, err := s.ProxyClient.GetProxies()
		require.NoError(t, err)
		if len(proxies) != 0 {
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("proxy didn't register within 5s after startup")
		}
	}

	proxyAddr := utils.MustParseAddr(s.Proxy.Addr())

	addr := utils.MustParseAddr(s.WebServer.Listener.Addr().String())
	handler.handler.cfg.ProxyWebAddr = *addr
	handler.handler.cfg.ProxySSHAddr = *proxyAddr
	_, sshPort, err := net.SplitHostPort(proxyAddr.String())
	require.NoError(t, err)
	handler.handler.sshPort = sshPort

	t.Cleanup(func() {
		// In particular close the lock watchers by canceling the context.
		s.Cancel()

		s.WebServer.Close()

		var errors []error
		if err := s.ProxyTunnel.Close(); err != nil {
			errors = append(errors, err)
		}
		if err := s.Node.Close(); err != nil {
			errors = append(errors, err)
		}
		s.WebServer.Close()
		if err := s.Proxy.Close(); err != nil {
			errors = append(errors, err)
		}
		if err := s.Server.Shutdown(context.Background()); err != nil {
			errors = append(errors, err)
		}
		require.Empty(t, errors)
	})

	return s
}

func noCache(clt auth.ClientI, cacheName []string) (auth.RemoteProxyAccessPoint, error) {
	return clt, nil
}

func (r *authPack) renewSession(ctx context.Context, t *testing.T) *roundtrip.Response {
	resp, err := r.clt.PostJSON(ctx, r.clt.Endpoint("webapi", "sessions", "renew"), nil)
	require.NoError(t, err)
	return resp
}

func (r *authPack) validateAPI(ctx context.Context, t *testing.T) {
	_, err := r.clt.Get(ctx, r.clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)
}

type authPack struct {
	otpSecret string
	user      string
	login     string
	password  string
	session   *CreateSessionResponse
	clt       *client.WebClient
	cookies   []*http.Cookie
}

// authPack returns new authenticated package consisting of created valid
// user, otp token, created web session and authenticated client.
func (s *TestWebSuite) authPack(t *testing.T, user string) *authPack {
	login := s.User
	pass := "abc123"
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	err = s.Server.Auth().SetAuthPreference(s.Ctx, ap)
	require.NoError(t, err)

	s.createUser(t, user, login, pass, otpSecret)

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, s.Clock.Now())
	require.NoError(t, err)

	clt := s.client()
	req := CreateSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: validToken,
	}

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	re, err := s.login(clt, csrfToken, csrfToken, req)
	require.NoError(t, err)

	var rawSess *CreateSessionResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &rawSess))

	sess, err := rawSess.response()
	require.NoError(t, err)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt = s.client(roundtrip.BearerAuth(sess.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), re.Cookies())

	return &authPack{
		otpSecret: otpSecret,
		user:      user,
		login:     login,
		session:   sess,
		clt:       clt,
		cookies:   re.Cookies(),
	}
}

func (s *TestWebSuite) createUser(t *testing.T, user string, login string, pass string, otpSecret string) {
	teleUser, err := types.NewUser(user)
	require.NoError(t, err)
	role := services.RoleForUser(teleUser)
	role.SetLogins(types.Allow, []string{login})
	options := role.GetOptions()
	options.ForwardAgent = types.NewBool(true)
	role.SetOptions(options)
	err = s.Server.Auth().UpsertRole(s.Ctx, role)
	require.NoError(t, err)
	teleUser.AddRole(role.GetName())

	teleUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})
	err = s.Server.Auth().CreateUser(s.Ctx, teleUser)
	require.NoError(t, err)

	err = s.Server.Auth().UpsertPassword(user, []byte(pass))
	require.NoError(t, err)

	if otpSecret != "" {
		dev, err := services.NewTOTPDevice("otp", otpSecret, s.Clock.Now())
		require.NoError(t, err)
		err = s.Server.Auth().UpsertMFADevice(context.Background(), user, dev)
		require.NoError(t, err)
	}
}

func (s *TestWebSuite) makeTerminal(t *testing.T, pack *authPack, opts ...terminalOpt) (*websocket.Conn, error) {
	req := TerminalRequest{
		Server: s.SrvID,
		Login:  pack.login,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
		SessionID: session.NewID(),
	}
	for _, opt := range opts {
		opt(&req)
	}

	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect", currentSiteShortcut),
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AccessTokenQueryParam, pack.session.Token)
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	for _, cookie := range pack.cookies {
		header.Add("Cookie", cookie.String())
	}

	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	require.NoError(t, resp.Body.Close())
	return ws, nil
}

func (s *TestWebSuite) waitForRawEvent(ws *websocket.Conn, timeout time.Duration) error {
	timeoutContext, timeoutCancel := context.WithTimeout(s.Ctx, timeout)
	defer timeoutCancel()

	done := make(chan error, 1)

	go func() {
		for {
			ty, raw, err := ws.ReadMessage()
			if err != nil {
				done <- trace.Wrap(err)
				return
			}

			if ty != websocket.BinaryMessage {
				done <- trace.BadParameter("expected binary message, got %v", ty)
				return
			}

			var envelope Envelope
			err = proto.Unmarshal(raw, &envelope)
			if err != nil {
				done <- trace.Wrap(err)
				return
			}

			if envelope.GetType() == defaults.WebsocketRaw {
				done <- nil
				return
			}
		}
	}()

	for {
		select {
		case <-timeoutContext.Done():
			return trace.BadParameter("timeout waiting for raw event")
		case err := <-done:
			return trace.Wrap(err)
		}
	}
}

func (s *TestWebSuite) waitForResizeEvent(ws *websocket.Conn, timeout time.Duration) error {
	timeoutContext, timeoutCancel := context.WithTimeout(s.Ctx, timeout)
	defer timeoutCancel()

	done := make(chan error, 1)

	go func() {
		for {
			ty, raw, err := ws.ReadMessage()
			if err != nil {
				done <- trace.Wrap(err)
				return
			}

			if ty != websocket.BinaryMessage {
				done <- trace.BadParameter("expected binary message, got %v", ty)
				return
			}

			var envelope Envelope
			err = proto.Unmarshal(raw, &envelope)
			if err != nil {
				done <- trace.Wrap(err)
				return
			}

			if envelope.GetType() != defaults.WebsocketAudit {
				continue
			}

			var e events.EventFields
			err = json.Unmarshal([]byte(envelope.GetPayload()), &e)
			if err != nil {
				done <- trace.Wrap(err)
				return
			}

			if e.GetType() == events.ResizeEvent {
				done <- nil
				return
			}
		}
	}()

	for {
		select {
		case <-timeoutContext.Done():
			return trace.BadParameter("timeout waiting for resize event")
		case err := <-done:
			return trace.Wrap(err)
		}
	}
}

func (s *TestWebSuite) listenForResizeEvent(ws *websocket.Conn) chan struct{} {
	ch := make(chan struct{})

	go func() {
		for {
			ty, raw, err := ws.ReadMessage()
			if err != nil {
				close(ch)
				return
			}

			if ty != websocket.BinaryMessage {
				close(ch)
				return
			}

			var envelope Envelope
			err = proto.Unmarshal(raw, &envelope)
			if err != nil {
				close(ch)
				return
			}

			if envelope.GetType() != defaults.WebsocketAudit {
				continue
			}

			var e events.EventFields
			err = json.Unmarshal([]byte(envelope.GetPayload()), &e)
			if err != nil {
				close(ch)
				return
			}

			if e.GetType() == events.ResizeEvent {
				ch <- struct{}{}
				return
			}
		}
	}()

	return ch
}

func (s *TestWebSuite) ClientNoRedirects(opts ...roundtrip.ClientParam) *client.WebClient {
	hclient := client.NewInsecureWebClient()
	hclient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	opts = append(opts, roundtrip.HTTPClient(hclient))
	wc, err := client.NewWebClient(s.url().String(), opts...)
	if err != nil {
		panic(err)
	}
	return wc
}

func (s *TestWebSuite) client(opts ...roundtrip.ClientParam) *client.WebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	wc, err := client.NewWebClient(s.url().String(), opts...)
	if err != nil {
		panic(err)
	}
	return wc
}

func (s *TestWebSuite) login(clt *client.WebClient, cookieToken string, reqToken string, reqData interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(reqData)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest("POST", clt.Endpoint("webapi", "sessions"), bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		AddCSRFCookieToReq(req, cookieToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrf.HeaderName, reqToken)
		return clt.HTTPClient().Do(req)
	}))
}

func (s *TestWebSuite) url() *url.URL {
	u, err := url.Parse("https://" + s.WebServer.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return u
}

func (r CreateSessionResponse) response() (*CreateSessionResponse, error) {
	return &CreateSessionResponse{TokenType: r.TokenType, Token: r.Token, TokenExpiresIn: r.TokenExpiresIn, SessionInactiveTimeoutMS: r.SessionInactiveTimeoutMS}, nil
}

type mockProxySettings struct{}

func (mock *mockProxySettings) GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error) {
	return &webclient.ProxySettings{}, nil
}

type terminalOpt func(t *TerminalRequest)

func withSessionID(sid session.ID) terminalOpt {
	return func(t *TerminalRequest) { t.SessionID = sid }
}

func withKeepaliveInterval(d time.Duration) terminalOpt {
	return func(t *TerminalRequest) { t.KeepAliveInterval = d }
}

func AddCSRFCookieToReq(req *http.Request, token string) {
	cookie := &http.Cookie{
		Name:  csrf.CookieName,
		Value: token,
	}

	req.AddCookie(cookie)
}
