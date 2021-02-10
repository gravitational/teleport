/*
Copyright 2015-2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/tls"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	authclient "github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	authresource "github.com/gravitational/teleport/lib/auth/resource"
	authserver "github.com/gravitational/teleport/lib/auth/server"
	test "github.com/gravitational/teleport/lib/auth/test/services"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/beevik/etree"
	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	lemma_secret "github.com/mailgun/lemma/secret"
	"github.com/pborman/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	. "gopkg.in/check.v1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const hostID = "00000000-0000-0000-0000-000000000000"

func TestWeb(t *testing.T) {
	TestingT(t)
}

type WebSuite struct {
	node        *regular.Server
	proxy       *regular.Server
	proxyTunnel reversetunnel.Server
	srvID       string

	user      string
	webServer *httptest.Server

	mockU2F     *mocku2f.Key
	server      *test.TLSServer
	proxyClient *authclient.Client
	clock       clockwork.FakeClock
}

var _ = Suite(&WebSuite{})

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if len(os.Args) == 2 &&
		(os.Args[1] == teleport.ExecSubCommand || os.Args[1] == teleport.ForwardSubCommand) {
		srv.RunAndExit(os.Args[1])
		return
	}

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

func (s *WebSuite) SetUpSuite(c *C) {
	os.Unsetenv(teleport.DebugEnvVar)

	var err error
	s.mockU2F, err = mocku2f.Create()
	c.Assert(err, IsNil)
	c.Assert(s.mockU2F, NotNil)
}

func (s *WebSuite) SetUpTest(c *C) {
	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username
	s.clock = clockwork.NewFakeClock()

	authServer, err := test.NewAuthServer(test.AuthServerConfig{
		ClusterName: "localhost",
		Dir:         c.MkDir(),
		Clock:       s.clock,
	})
	c.Assert(err, IsNil)
	s.server, err = authServer.NewTLSServer()
	c.Assert(err, IsNil)
	// Register the auth server, since test auth server doesn't start its own
	// heartbeat.
	err = authServer.AuthServer.UpsertAuthServer(&services.ServerV2{
		Kind:    services.KindAuthServer,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      "auth",
		},
		Spec: services.ServerSpecV2{
			Addr:     s.server.Listener.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	c.Assert(err, IsNil)

	// start node
	certs, err := s.server.Auth().GenerateServerKeys(authserver.GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: s.server.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleNode},
	})
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(certs.Key, certs.Cert)
	c.Assert(err, IsNil)

	nodeID := "node"
	nodeClient, err := s.server.NewClient(test.Identity{
		I: authserver.BuiltinRole{
			Role:     teleport.RoleNode,
			Username: nodeID,
		},
	})
	c.Assert(err, IsNil)

	// create SSH service:
	nodeDataDir := c.MkDir()
	node, err := regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		[]ssh.Signer{signer},
		nodeClient,
		nodeDataDir,
		"",
		utils.NetAddr{},
		regular.SetUUID(nodeID),
		regular.SetNamespace(defaults.Namespace),
		regular.SetShell("/bin/sh"),
		regular.SetSessionServer(nodeClient),
		regular.SetEmitter(nodeClient),
		regular.SetPAMConfig(&pam.Config{Enabled: false}),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	s.node = node
	s.srvID = node.ID()
	c.Assert(s.node.Start(), IsNil)

	c.Assert(test.CreateUploaderDir(nodeDataDir), IsNil)

	// create reverse tunnel service:
	proxyID := "proxy"
	s.proxyClient, err = s.server.NewClient(test.Identity{
		I: authserver.BuiltinRole{
			Role:     teleport.RoleProxy,
			Username: proxyID,
		},
	})
	c.Assert(err, IsNil)

	revTunListener, err := net.Listen("tcp", fmt.Sprintf("%v:0", s.server.ClusterName()))
	c.Assert(err, IsNil)

	revTunServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:                    node.ID(),
		Listener:              revTunListener,
		ClientTLS:             s.proxyClient.TLSConfig(),
		ClusterName:           s.server.ClusterName(),
		HostSigners:           []ssh.Signer{signer},
		LocalAuthClient:       s.proxyClient,
		LocalAccessPoint:      s.proxyClient,
		Emitter:               s.proxyClient,
		NewCachingAccessPoint: authclient.NoCache,
		DirectClusters:        []reversetunnel.DirectCluster{{Name: s.server.ClusterName(), Client: s.proxyClient}},
		DataDir:               c.MkDir(),
	})
	c.Assert(err, IsNil)
	s.proxyTunnel = revTunServer

	// proxy server:
	s.proxy, err = regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		[]ssh.Signer{signer},
		s.proxyClient,
		c.MkDir(),
		"",
		utils.NetAddr{},
		regular.SetUUID(proxyID),
		regular.SetProxyMode(revTunServer),
		regular.SetSessionServer(s.proxyClient),
		regular.SetEmitter(s.proxyClient),
		regular.SetNamespace(defaults.Namespace),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
	)
	c.Assert(err, IsNil)

	// Expired sessions are purged immediately
	var sessionLingeringThreshold time.Duration = 0
	fs, err := NewDebugFileSystem("../../webassets/teleport")
	c.Assert(err, IsNil)
	handler, err := NewHandler(Config{
		Proxy:                           revTunServer,
		AuthServers:                     utils.FromAddr(s.server.Addr()),
		DomainName:                      s.server.ClusterName(),
		ProxyClient:                     s.proxyClient,
		CipherSuites:                    utils.DefaultCipherSuites(),
		AccessPoint:                     s.proxyClient,
		Context:                         context.Background(),
		HostUUID:                        proxyID,
		Emitter:                         s.proxyClient,
		StaticFS:                        fs,
		cachedSessionLingeringThreshold: &sessionLingeringThreshold,
	}, SetSessionStreamPollPeriod(200*time.Millisecond), SetClock(s.clock))
	c.Assert(err, IsNil)

	s.webServer = httptest.NewUnstartedServer(handler)
	s.webServer.StartTLS()
	err = s.proxy.Start()
	c.Assert(err, IsNil)

	// Wait for proxy to fully register before starting the test.
	for start := time.Now(); ; {
		proxies, err := s.proxyClient.GetProxies()
		c.Assert(err, IsNil)
		if len(proxies) != 0 {
			break
		}
		if time.Since(start) > 5*time.Second {
			c.Fatal("proxy didn't register within 5s after startup")
		}
	}

	proxyAddr := utils.MustParseAddr(s.proxy.Addr())

	addr := utils.MustParseAddr(s.webServer.Listener.Addr().String())
	handler.handler.cfg.ProxyWebAddr = *addr
	handler.handler.cfg.ProxySSHAddr = *proxyAddr
	_, sshPort, err := net.SplitHostPort(proxyAddr.String())
	c.Assert(err, IsNil)
	handler.handler.sshPort = sshPort
}

func (s *WebSuite) TearDownTest(c *C) {
	c.Assert(s.node.Close(), IsNil)
	c.Assert(s.server.Close(), IsNil)
	s.webServer.Close()
	s.proxy.Close()
	s.proxyTunnel.Close()
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
	session   *CreateSessionResponse
	clt       *client.WebClient
	cookies   []*http.Cookie
}

// authPack returns new authenticated package consisting of created valid
// user, otp token, created web session and authenticated client.
func (s *WebSuite) authPack(c *C, user string) *authPack {
	login := s.user
	pass := "abc123"
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetAuthPreference(ap)
	c.Assert(err, IsNil)

	s.createUser(c, user, login, pass, otpSecret)

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, s.clock.Now())
	c.Assert(err, IsNil)

	clt := s.client()
	req := CreateSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: validToken,
	}

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	re, err := s.login(clt, csrfToken, csrfToken, req)
	c.Assert(err, IsNil)

	var rawSess *CreateSessionResponse
	c.Assert(json.Unmarshal(re.Bytes(), &rawSess), IsNil)

	sess, err := rawSess.response()
	c.Assert(err, IsNil)

	jar, err := cookiejar.New(nil)
	c.Assert(err, IsNil)

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

func (s *WebSuite) createUser(c *C, user string, login string, pass string, otpSecret string) {
	ctx := context.Background()
	teleUser, err := services.NewUser(user)
	c.Assert(err, IsNil)
	role := auth.RoleForUser(teleUser)
	role.SetLogins(services.Allow, []string{login})
	options := role.GetOptions()
	options.ForwardAgent = services.NewBool(true)
	role.SetOptions(options)
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, IsNil)
	teleUser.AddRole(role.GetName())

	teleUser.SetCreatedBy(services.CreatedBy{
		User: services.UserRef{Name: "some-auth-user"},
	})
	err = s.server.Auth().CreateUser(ctx, teleUser)
	c.Assert(err, IsNil)

	err = s.server.Auth().UpsertPassword(user, []byte(pass))
	c.Assert(err, IsNil)

	if otpSecret != "" {
		dev, err := auth.NewTOTPDevice("otp", otpSecret, s.clock.Now())
		c.Assert(err, IsNil)
		err = s.server.Auth().UpsertMFADevice(context.Background(), user, dev)
		c.Assert(err, IsNil)
	}
}

func (s *WebSuite) TestSAMLSuccess(c *C) {
	ctx := context.Background()
	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw authresource.UnknownResource
	err := decoder.Decode(&raw)
	c.Assert(err, IsNil)

	connector, err := authresource.UnmarshalSAMLConnector(raw.Raw)
	c.Assert(err, IsNil)
	err = auth.ValidateSAMLConnector(connector)
	c.Assert(err, IsNil)

	role, err := services.NewRole(connector.GetAttributesToRoles()[0].Roles[0], services.RoleSpecV3{
		Options: services.RoleOptions{
			MaxSessionTTL: services.NewDuration(defaults.MaxCertDuration),
		},
		Allow: services.RoleConditions{
			NodeLabels: services.Labels{services.Wildcard: []string{services.Wildcard}},
			Namespaces: []string{defaults.Namespace},
			Rules: []services.Rule{
				services.NewRule(services.Wildcard, auth.RW()),
			},
		},
	})
	c.Assert(err, IsNil)
	role.SetLogins(services.Allow, []string{s.user})
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, IsNil)

	err = s.server.Auth().CreateSAMLConnector(connector)
	c.Assert(err, IsNil)
	s.server.AuthServer.AuthServer.SetClock(clockwork.NewFakeClockAt(time.Date(2017, 05, 10, 18, 53, 0, 0, time.UTC)))
	clt := s.clientNoRedirects()

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"

	baseURL, err := url.Parse(clt.Endpoint("webapi", "saml", "sso") + `?redirect_url=http://localhost/after;connector_id=` + connector.GetName())
	c.Assert(err, IsNil)
	req, err := http.NewRequest("GET", baseURL.String(), nil)
	c.Assert(err, IsNil)
	addCSRFCookieToReq(req, csrfToken)
	re, err := clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})
	c.Assert(err, IsNil)

	// we got a redirect
	locationURL := re.Headers().Get("Location")
	u, err := url.Parse(locationURL)
	c.Assert(err, IsNil)
	c.Assert(u.Scheme+"://"+u.Host+u.Path, Equals, fixtures.SAMLOktaSSO)
	data, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	c.Assert(err, IsNil)
	buf, err := ioutil.ReadAll(flate.NewReader(bytes.NewReader(data)))
	c.Assert(err, IsNil)
	doc := etree.NewDocument()
	err = doc.ReadFromBytes(buf)
	c.Assert(err, IsNil)
	id := doc.Root().SelectAttr("ID")
	c.Assert(id, NotNil)

	authRequest, err := s.server.Auth().GetSAMLAuthRequest(id.Value)
	c.Assert(err, IsNil)

	// now swap the request id to the hardcoded one in fixtures
	authRequest.ID = fixtures.SAMLOktaAuthRequestID
	authRequest.CSRFToken = csrfToken
	err = s.server.Auth().Identity.CreateSAMLAuthRequest(*authRequest, backend.Forever)
	c.Assert(err, IsNil)

	// now respond with pre-recorded request to the POST url
	in := &bytes.Buffer{}
	fw, err := flate.NewWriter(in, flate.DefaultCompression)
	c.Assert(err, IsNil)

	_, err = fw.Write([]byte(fixtures.SAMLOktaAuthnResponseXML))
	c.Assert(err, IsNil)
	err = fw.Close()
	c.Assert(err, IsNil)
	encodedResponse := base64.StdEncoding.EncodeToString(in.Bytes())
	c.Assert(encodedResponse, NotNil)

	// now send the response to the server to exchange it for auth session
	form := url.Values{}
	form.Add("SAMLResponse", encodedResponse)
	req, err = http.NewRequest("POST", clt.Endpoint("webapi", "saml", "acs"), strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	addCSRFCookieToReq(req, csrfToken)
	c.Assert(err, IsNil)
	authRe, err := clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})

	c.Assert(err, IsNil)
	comment := Commentf("Response: %v", string(authRe.Bytes()))
	c.Assert(authRe.Code(), Equals, http.StatusFound, comment)
	// we have got valid session
	c.Assert(authRe.Headers().Get("Set-Cookie"), Not(Equals), "")
	// we are being redirected to orignal URL
	c.Assert(authRe.Headers().Get("Location"), Equals, "/after")
}

func (s *WebSuite) TestWebSessionsCRUD(c *C) {
	pack := s.authPack(c, "foo")

	// make sure we can use client to make authenticated requests
	re, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	var clusters []ui.Cluster
	c.Assert(json.Unmarshal(re.Bytes(), &clusters), IsNil)

	// now delete session
	_, err = pack.clt.Delete(
		context.Background(),
		pack.clt.Endpoint("webapi", "sessions"))
	c.Assert(err, IsNil)

	// subsequent requests trying to use this session will fail
	_, err = pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *WebSuite) TestNamespace(c *C) {
	pack := s.authPack(c, "foo")

	_, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "namespaces", "..%252fevents%3f", "nodes"), url.Values{})
	c.Assert(err, NotNil)

	_, err = pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "namespaces", "default", "nodes"), url.Values{})
	c.Assert(err, IsNil)
}

func (s *WebSuite) TestCSRF(c *C) {
	type input struct {
		reqToken    string
		cookieToken string
	}

	// create a valid user
	user := "csrfuser"
	pass := "abc123"
	otpSecret := base32.StdEncoding.EncodeToString([]byte("def456"))
	s.createUser(c, user, user, pass, otpSecret)

	// create a valid login form request
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)
	loginForm := CreateSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: validToken,
	}

	encodedToken1 := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	encodedToken2 := "bf355921bbf3ef3672a03e410d4194077dfa5fe863c652521763b3e7f81e7b11"
	invalid := []input{
		{reqToken: encodedToken2, cookieToken: encodedToken1},
		{reqToken: "", cookieToken: encodedToken1},
		{reqToken: "", cookieToken: ""},
		{reqToken: encodedToken1, cookieToken: ""},
	}

	clt := s.client()

	// valid
	_, err = s.login(clt, encodedToken1, encodedToken1, loginForm)
	c.Assert(err, IsNil)

	// invalid
	for i := range invalid {
		_, err := s.login(clt, invalid[i].cookieToken, invalid[i].reqToken, loginForm)
		c.Assert(err, NotNil)
		c.Assert(trace.IsAccessDenied(err), Equals, true)
	}
}

func (s *WebSuite) TestPasswordChange(c *C) {
	pack := s.authPack(c, "foo")

	// invalidate the token
	s.clock.Advance(1 * time.Minute)
	validToken, err := totp.GenerateCode(pack.otpSecret, s.clock.Now())
	c.Assert(err, IsNil)

	req := changePasswordReq{
		OldPassword:       []byte("abc123"),
		NewPassword:       []byte("abc1234"),
		SecondFactorToken: validToken,
	}

	_, err = pack.clt.PutJSON(context.Background(), pack.clt.Endpoint("webapi", "users", "password"), req)
	c.Assert(err, IsNil)
}

func (s *WebSuite) TestWebSessionsBadInput(c *C) {
	user := "bob"
	pass := "abc123"
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	err := s.server.Auth().UpsertPassword(user, []byte(pass))
	c.Assert(err, IsNil)

	dev, err := auth.NewTOTPDevice("otp", otpSecret, s.clock.Now())
	c.Assert(err, IsNil)
	err = s.server.Auth().UpsertMFADevice(context.Background(), user, dev)
	c.Assert(err, IsNil)

	// create valid token
	validToken, err := totp.GenerateCode(otpSecret, time.Now())
	c.Assert(err, IsNil)

	clt := s.client()

	reqs := []CreateSessionReq{
		// empty request
		{},
		// missing user
		{
			Pass:              pass,
			SecondFactorToken: validToken,
		},
		// missing pass
		{
			User:              user,
			SecondFactorToken: validToken,
		},
		// bad pass
		{
			User:              user,
			Pass:              "bla bla",
			SecondFactorToken: validToken,
		},
		// bad hotp token
		{
			User:              user,
			Pass:              pass,
			SecondFactorToken: "bad token",
		},
		// missing hotp token
		{
			User: user,
			Pass: pass,
		},
	}
	for i, req := range reqs {
		_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "sessions"), req)
		c.Assert(err, NotNil, Commentf("tc %v", i))
		c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("tc %v %T is not access denied", i, err))
	}
}

type getSiteNodeResponse struct {
	Items []ui.Server `json:"items"`
}

func (s *WebSuite) TestGetSiteNodes(c *C) {
	pack := s.authPack(c, "foo")

	// get site nodes
	re, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "nodes"), url.Values{})
	c.Assert(err, IsNil)

	nodes := getSiteNodeResponse{}
	c.Assert(json.Unmarshal(re.Bytes(), &nodes), IsNil)
	c.Assert(len(nodes.Items), Equals, 1)

	// get site nodes using shortcut
	re, err = pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", currentSiteShortcut, "nodes"), url.Values{})
	c.Assert(err, IsNil)

	nodes2 := getSiteNodeResponse{}
	c.Assert(json.Unmarshal(re.Bytes(), &nodes2), IsNil)
	c.Assert(len(nodes.Items), Equals, 1)
	c.Assert(nodes2, DeepEquals, nodes)
}

func (s *WebSuite) TestSiteNodeConnectInvalidSessionID(c *C) {
	_, err := s.makeTerminal(s.authPack(c, "foo"), session.ID("/../../../foo"))
	c.Assert(err, NotNil)
}

func (s *WebSuite) TestResolveServerHostPort(c *C) {
	sampleNode := services.ServerV2{}
	sampleNode.SetName("eca53e45-86a9-11e7-a893-0242ac0a0101")
	sampleNode.Spec.Hostname = "nodehostname"

	// valid cases
	validCases := []struct {
		server       string
		nodes        []services.Server
		expectedHost string
		expectedPort int
	}{
		{
			server:       "localhost",
			expectedHost: "localhost",
			expectedPort: 0,
		},
		{
			server:       "localhost:8080",
			expectedHost: "localhost",
			expectedPort: 8080,
		},
		{
			server:       "eca53e45-86a9-11e7-a893-0242ac0a0101",
			nodes:        []services.Server{&sampleNode},
			expectedHost: "nodehostname",
			expectedPort: 0,
		},
	}

	// invalid cases
	invalidCases := []struct {
		server      string
		expectedErr string
	}{
		{
			server:      ":22",
			expectedErr: "empty hostname",
		},
		{
			server:      ":",
			expectedErr: "empty hostname",
		},
		{
			server:      "",
			expectedErr: "empty server name",
		},
		{
			server:      "host:",
			expectedErr: "invalid port",
		},
		{
			server:      "host:port",
			expectedErr: "invalid port",
		},
	}

	for _, testCase := range validCases {
		host, port, err := resolveServerHostPort(testCase.server, testCase.nodes)
		c.Assert(err, IsNil, Commentf(testCase.server))
		c.Assert(host, Equals, testCase.expectedHost)
		c.Assert(port, Equals, testCase.expectedPort)
	}

	for _, testCase := range invalidCases {
		_, _, err := resolveServerHostPort(testCase.server, nil)
		c.Assert(err, NotNil, Commentf(testCase.expectedErr))
		c.Assert(err, ErrorMatches, ".*"+testCase.expectedErr+".*")
	}

}

func (s *WebSuite) TestNewTerminalHandler(c *C) {
	validNode := services.ServerV2{}
	validNode.SetName("eca53e45-86a9-11e7-a893-0242ac0a0101")
	validNode.Spec.Hostname = "nodehostname"

	validServer := "localhost"
	validLogin := "root"
	validSID := session.ID("eca53e45-86a9-11e7-a893-0242ac0a0101")
	validParams := session.TerminalParams{
		H: 1,
		W: 1,
	}

	makeProvider := func(server services.ServerV2) AuthProvider {
		return authProviderMock{
			server: server,
		}
	}

	// valid cases
	validCases := []struct {
		req          TerminalRequest
		authProvider AuthProvider
		expectedHost string
		expectedPort int
	}{
		{
			req: TerminalRequest{
				Login:     validLogin,
				Server:    validServer,
				SessionID: validSID,
				Term:      validParams,
			},
			authProvider: makeProvider(validNode),
			expectedHost: validServer,
			expectedPort: 0,
		},
		{
			req: TerminalRequest{
				Login:     validLogin,
				Server:    "eca53e45-86a9-11e7-a893-0242ac0a0101",
				SessionID: validSID,
				Term:      validParams,
			},
			authProvider: makeProvider(validNode),
			expectedHost: "nodehostname",
			expectedPort: 0,
		},
	}

	// invalid cases
	invalidCases := []struct {
		req          TerminalRequest
		authProvider AuthProvider
		expectedErr  string
	}{
		{
			expectedErr:  "invalid session",
			authProvider: makeProvider(validNode),
			req: TerminalRequest{
				SessionID: "",
				Login:     validLogin,
				Server:    validServer,
				Term:      validParams,
			},
		},
		{
			expectedErr:  "bad term dimensions",
			authProvider: makeProvider(validNode),
			req: TerminalRequest{
				SessionID: validSID,
				Login:     validLogin,
				Server:    validServer,
				Term: session.TerminalParams{
					H: -1,
					W: 0,
				},
			},
		},
		{
			expectedErr:  "invalid server name",
			authProvider: makeProvider(validNode),
			req: TerminalRequest{
				Server:    "localhost:port",
				SessionID: validSID,
				Login:     validLogin,
				Term:      validParams,
			},
		},
	}

	for _, testCase := range validCases {
		term, err := NewTerminal(testCase.req, testCase.authProvider, nil)
		c.Assert(err, IsNil)
		c.Assert(term.params, DeepEquals, testCase.req)
		c.Assert(term.hostName, Equals, testCase.expectedHost)
		c.Assert(term.hostPort, Equals, testCase.expectedPort)
	}

	for _, testCase := range invalidCases {
		_, err := NewTerminal(testCase.req, testCase.authProvider, nil)
		c.Assert(err, ErrorMatches, ".*"+testCase.expectedErr+".*")
	}
}

func (s *WebSuite) TestResizeTerminal(c *C) {
	sid := session.NewID()

	// Create a new user "foo", open a terminal to a new session, and wait for
	// it to be ready.
	pack1 := s.authPack(c, "foo")
	ws1, err := s.makeTerminal(pack1, sid)
	c.Assert(err, IsNil)
	defer ws1.Close()
	err = s.waitForRawEvent(ws1, 5*time.Second)
	c.Assert(err, IsNil)

	// Create a new user "bar", open a terminal to the session created above,
	// and wait for it to be ready.
	pack2 := s.authPack(c, "bar")
	ws2, err := s.makeTerminal(pack2, sid)
	c.Assert(err, IsNil)
	defer ws2.Close()
	err = s.waitForRawEvent(ws2, 5*time.Second)
	c.Assert(err, IsNil)

	// Look at the audit events for the first terminal. It should have two
	// resize events from the second terminal (80x25 default then 100x100). Only
	// the second terminal will get these because resize events are not sent
	// back to the originator.
	err = s.waitForResizeEvent(ws1, 5*time.Second)
	c.Assert(err, IsNil)
	err = s.waitForResizeEvent(ws1, 5*time.Second)
	c.Assert(err, IsNil)

	// Look at the stream events for the second terminal. We don't expect to see
	// any resize events yet. It will timeout.
	err = s.waitForResizeEvent(ws2, 1*time.Second)
	c.Assert(err, NotNil)

	// Resize the second terminal. This should be reflected on the first terminal
	// because resize events are not sent back to the originator.
	params, err := session.NewTerminalParamsFromInt(300, 120)
	c.Assert(err, IsNil)
	data, err := json.Marshal(events.EventFields{
		events.EventType:      events.ResizeEvent,
		events.EventNamespace: defaults.Namespace,
		events.SessionEventID: sid.String(),
		events.TerminalSize:   params.Serialize(),
	})
	c.Assert(err, IsNil)
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketResize,
		Payload: string(data),
	}
	envelopeBytes, err := proto.Marshal(envelope)
	c.Assert(err, IsNil)
	err = websocket.Message.Send(ws2, envelopeBytes)
	c.Assert(err, IsNil)

	// This time the first terminal will see the resize event.
	err = s.waitForResizeEvent(ws1, 5*time.Second)
	c.Assert(err, IsNil)

	// The second terminal will not see any resize event. It will timeout.
	err = s.waitForResizeEvent(ws2, 1*time.Second)
	c.Assert(err, NotNil)
}

func (s *WebSuite) TestTerminal(c *C) {
	ws, err := s.makeTerminal(s.authPack(c, "foo"))
	c.Assert(err, IsNil)
	defer ws.Close()

	termHandler := newTerminalHandler()
	stream := termHandler.asTerminalStream(ws)

	_, err = io.WriteString(stream, "echo vinsong\r\n")
	c.Assert(err, IsNil)

	err = waitForOutput(stream, "vinsong")
	c.Assert(err, IsNil)
}

func (s *WebSuite) TestWebsocketPingLoop(c *C) {
	// change cluster default config for keep alive interval to be ran faster
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording:    services.RecordAtNode,
		ProxyChecksHostKeys: services.HostKeyCheckYes,
		KeepAliveInterval:   services.NewDuration(250 * time.Millisecond),
		LocalAuth:           services.NewBool(true),
	})
	c.Assert(err, IsNil)

	err = s.server.Auth().SetClusterConfig(clusterConfig)
	c.Assert(err, IsNil)

	ws, err := s.makeTerminal(s.authPack(c, "foo"))
	c.Assert(err, IsNil)

	// flush out raw event (pty texts)
	err = s.waitForRawEvent(ws, 5*time.Second)
	c.Assert(err, IsNil)

	var numPings int
	start := time.Now()
	for {
		frame, err := ws.NewFrameReader()
		c.Assert(err, IsNil)
		// We should get a mix of output (binary) and ping frames. Count only
		// the ping frames.
		if int(frame.PayloadType()) == websocket.PingFrame {
			numPings++
		}
		if numPings > 1 {
			break
		}
		if time.Since(start) > 5*time.Second {
			c.Fatalf("received %d ping frames within 5s of opening a socket, expected at least 2", numPings)
		}
	}

	err = ws.Close()
	c.Assert(err, IsNil)
}

func (s *WebSuite) TestWebAgentForward(c *C) {
	ws, err := s.makeTerminal(s.authPack(c, "foo"))
	c.Assert(err, IsNil)
	defer ws.Close()

	termHandler := newTerminalHandler()
	stream := termHandler.asTerminalStream(ws)

	_, err = io.WriteString(stream, "echo $SSH_AUTH_SOCK\r\n")
	c.Assert(err, IsNil)

	err = waitForOutput(stream, "/")
	c.Assert(err, IsNil)
}

func (s *WebSuite) TestActiveSessions(c *C) {
	sid := session.NewID()
	pack := s.authPack(c, "foo")

	ws, err := s.makeTerminal(pack, sid)
	c.Assert(err, IsNil)
	defer ws.Close()

	termHandler := newTerminalHandler()
	stream := termHandler.asTerminalStream(ws)

	// To make sure we have a session.
	_, err = io.WriteString(stream, "echo vinsong\r\n")
	c.Assert(err, IsNil)

	// Make sure server has replied.
	err = waitForOutput(stream, "vinsong")
	c.Assert(err, IsNil)

	// Make sure this session appears in the list of active sessions.
	var sessResp *siteSessionsGetResponse
	for i := 0; i < 10; i++ {
		// Get site nodes and make sure the node has our active party.
		re, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "sessions"), url.Values{})
		c.Assert(err, IsNil)

		c.Assert(json.Unmarshal(re.Bytes(), &sessResp), IsNil)
		c.Assert(len(sessResp.Sessions), Equals, 1)

		// Sessions do not appear momentarily as there's async heartbeat
		// procedure.
		time.Sleep(250 * time.Millisecond)
	}

	c.Assert(len(sessResp.Sessions), Equals, 1)

	sess := sessResp.Sessions[0]
	c.Assert(sess.ID, Equals, sid)
	c.Assert(sess.Namespace, Equals, s.node.GetNamespace())
	c.Assert(sess.Parties, NotNil)
	c.Assert(sess.TerminalParams.H > 0, Equals, true)
	c.Assert(sess.TerminalParams.W > 0, Equals, true)
	c.Assert(sess.Login, Equals, pack.login)
	c.Assert(sess.Created.IsZero(), Equals, false)
	c.Assert(sess.LastActive.IsZero(), Equals, false)
	c.Assert(sess.ServerID, Equals, s.srvID)
	c.Assert(sess.ServerHostname, Equals, s.node.GetInfo().GetHostname())
	c.Assert(sess.ServerAddr, Equals, s.node.GetInfo().GetAddr())
	c.Assert(sess.ClusterName, Equals, s.server.ClusterName())
}

// DELETE IN: 5.0.0
// Tests the code snippet from apiserver.(*Handler).siteSessionGet/siteSessionsGet
// that tests empty ClusterName and ServerHostname gets set.
func (s *WebSuite) TestEmptySessionClusterHostnameIsSet(c *C) {
	nodeClient, err := s.server.NewClient(test.Builtin(teleport.RoleNode))
	c.Assert(err, IsNil)

	// Create a session with empty ClusterName.
	sess1 := session.Session{
		ClusterName:    "",
		ServerID:       string(session.NewID()),
		ID:             session.NewID(),
		Namespace:      defaults.Namespace,
		Login:          "foo",
		Created:        time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
		LastActive:     time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
		TerminalParams: session.TerminalParams{W: 100, H: 100},
	}
	err = nodeClient.CreateSession(sess1)
	c.Assert(err, IsNil)

	// Retrieve the session with the empty ClusterName.
	pack := s.authPack(c, "baz")
	res, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "namespaces", "default", "sessions", sess1.ID.String()), url.Values{})
	c.Assert(err, IsNil)

	// Test that empty ClusterName and ServerHostname got set.
	var sessionResult *session.Session
	err = json.Unmarshal(res.Bytes(), &sessionResult)
	c.Assert(err, IsNil)
	c.Assert(sessionResult.ClusterName, Equals, s.server.ClusterName())
	c.Assert(sessionResult.ServerHostname, Equals, sess1.ServerID)

	// Create another session to test sessions list.
	sess2 := sess1
	sess2.ID = session.NewID()
	sess2.ServerID = string(session.NewID())
	err = nodeClient.CreateSession(sess2)
	c.Assert(err, IsNil)

	// Retrieve sessions list.
	res, err = pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "namespaces", "default", "sessions"), url.Values{})
	c.Assert(err, IsNil)

	var sessionList *siteSessionsGetResponse
	err = json.Unmarshal(res.Bytes(), &sessionList)
	c.Assert(err, IsNil)

	s1 := sessionList.Sessions[0]
	s2 := sessionList.Sessions[1]

	c.Assert(s1.ClusterName, Equals, s.server.ClusterName())
	c.Assert(s2.ClusterName, Equals, s.server.ClusterName())
	c.Assert(s1.ServerHostname, Equals, s1.ServerID)
	c.Assert(s2.ServerHostname, Equals, s2.ServerID)
}

func (s *WebSuite) TestCloseConnectionsOnLogout(c *C) {
	sid := session.NewID()
	pack := s.authPack(c, "foo")

	ws, err := s.makeTerminal(pack, sid)
	c.Assert(err, IsNil)
	defer ws.Close()

	termHandler := newTerminalHandler()
	stream := termHandler.asTerminalStream(ws)

	// to make sure we have a session
	_, err = io.WriteString(stream, "expr 137 + 39\r\n")
	c.Assert(err, IsNil)

	// make sure server has replied
	out := make([]byte, 100)
	_, err = stream.Read(out)
	c.Assert(err, IsNil)

	_, err = pack.clt.Delete(
		context.Background(),
		pack.clt.Endpoint("webapi", "sessions"))
	c.Assert(err, IsNil)

	// wait until we timeout or detect that connection has been closed
	after := time.After(5 * time.Second)
	errC := make(chan error)
	go func() {
		for {
			_, err := stream.Read(out)
			if err != nil {
				errC <- err
			}
		}
	}()

	select {
	case <-after:
		c.Fatalf("timeout")
	case err := <-errC:
		c.Assert(err, Equals, io.EOF)
	}
}

func (s *WebSuite) TestCreateSession(c *C) {
	pack := s.authPack(c, "foo")

	// get site nodes
	re, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "nodes"), url.Values{})
	c.Assert(err, IsNil)

	nodes := getSiteNodeResponse{}
	c.Assert(json.Unmarshal(re.Bytes(), &nodes), IsNil)
	node := nodes.Items[0]

	sess := session.Session{
		TerminalParams: session.TerminalParams{W: 300, H: 120},
		Login:          s.user,
	}

	// test using node UUID
	sess.ServerID = node.Name
	re, err = pack.clt.PostJSON(
		context.Background(),
		pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "sessions"),
		siteSessionGenerateReq{Session: sess},
	)
	c.Assert(err, IsNil)

	var created *siteSessionGenerateResponse
	c.Assert(json.Unmarshal(re.Bytes(), &created), IsNil)
	c.Assert(created.Session.ID, Not(Equals), "")
	c.Assert(created.Session.ServerHostname, Equals, node.Hostname)

	// test empty serverID (older version does not supply serverID)
	sess.ServerID = ""
	_, err = pack.clt.PostJSON(
		context.Background(),
		pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "sessions"),
		siteSessionGenerateReq{Session: sess},
	)
	c.Assert(err, IsNil)
}

func (s *WebSuite) TestPlayback(c *C) {
	pack := s.authPack(c, "foo")
	sid := session.NewID()
	ws, err := s.makeTerminal(pack, sid)
	c.Assert(err, IsNil)
	defer ws.Close()
}

func (s *WebSuite) TestLogin(c *C) {
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOff,
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetAuthPreference(ap)
	c.Assert(err, IsNil)

	// create user
	s.createUser(c, "user1", "root", "password", "")

	loginReq, err := json.Marshal(CreateSessionReq{
		User: "user1",
		Pass: "password",
	})
	c.Assert(err, IsNil)

	clt := s.client()
	req, err := http.NewRequest("POST", clt.Endpoint("webapi", "sessions"), bytes.NewBuffer(loginReq))
	c.Assert(err, IsNil)

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	addCSRFCookieToReq(req, csrfToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrf.HeaderName, csrfToken)

	re, err := clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})
	c.Assert(err, IsNil)

	var rawSess *CreateSessionResponse
	c.Assert(json.Unmarshal(re.Bytes(), &rawSess), IsNil)
	cookies := re.Cookies()
	c.Assert(len(cookies), Equals, 1)

	// now make sure we are logged in by calling authenticated method
	// we need to supply both session cookie and bearer token for
	// request to succeed
	jar, err := cookiejar.New(nil)
	c.Assert(err, IsNil)

	clt = s.client(roundtrip.BearerAuth(rawSess.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), re.Cookies())

	re, err = clt.Get(context.Background(), clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	var clusters []ui.Cluster
	c.Assert(json.Unmarshal(re.Bytes(), &clusters), IsNil)

	// in absence of session cookie or bearer auth the same request fill fail

	// no session cookie:
	clt = s.client(roundtrip.BearerAuth(rawSess.Token))
	_, err = clt.Get(context.Background(), clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)

	// no bearer token:
	clt = s.client(roundtrip.CookieJar(jar))
	_, err = clt.Get(context.Background(), clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *WebSuite) TestChangePasswordWithTokenOTP(c *C) {
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetAuthPreference(ap)
	c.Assert(err, IsNil)

	// create user
	s.createUser(c, "user1", "root", "password", "")

	// create password change token
	token, err := s.server.Auth().CreateResetPasswordToken(context.TODO(), authserver.CreateResetPasswordTokenRequest{
		Name: "user1",
	})
	c.Assert(err, IsNil)

	clt := s.client()
	re, err := clt.Get(context.Background(), clt.Endpoint("webapi", "users", "password", "token", token.GetName()), url.Values{})
	c.Assert(err, IsNil)

	var uiToken *ui.ResetPasswordToken
	c.Assert(json.Unmarshal(re.Bytes(), &uiToken), IsNil)
	c.Assert(uiToken.User, Equals, token.GetUser())
	c.Assert(uiToken.TokenID, Equals, token.GetName())

	secrets, err := s.server.Auth().RotateResetPasswordTokenSecrets(context.TODO(), token.GetName())
	c.Assert(err, IsNil)

	// Advance the clock to invalidate the TOTP token
	s.clock.Advance(1 * time.Minute)
	secondFactorToken, err := totp.GenerateCode(secrets.GetOTPKey(), s.clock.Now())
	c.Assert(err, IsNil)

	data, err := json.Marshal(authserver.ChangePasswordWithTokenRequest{
		TokenID:           token.GetName(),
		Password:          []byte("abc123"),
		SecondFactorToken: secondFactorToken,
	})
	c.Assert(err, IsNil)

	req, err := http.NewRequest("PUT", clt.Endpoint("webapi", "users", "password", "token"), bytes.NewBuffer(data))
	c.Assert(err, IsNil)

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	addCSRFCookieToReq(req, csrfToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrf.HeaderName, csrfToken)

	re, err = clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})
	c.Assert(err, IsNil)

	var rawSess *CreateSessionResponse
	c.Assert(json.Unmarshal(re.Bytes(), &rawSess), IsNil)
	c.Assert(rawSess.Token != "", Equals, true)
}

func (s *WebSuite) TestChangePasswordWithTokenU2F(c *C) {
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorU2F,
		U2F: &services.U2F{
			AppID:  "https://" + s.server.ClusterName(),
			Facets: []string{"https://" + s.server.ClusterName()},
		},
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetAuthPreference(ap)
	c.Assert(err, IsNil)

	s.createUser(c, "user2", "root", "password", "")

	// create reset password token
	token, err := s.server.Auth().CreateResetPasswordToken(context.TODO(), authserver.CreateResetPasswordTokenRequest{
		Name: "user2",
	})
	c.Assert(err, IsNil)

	clt := s.client()
	re, err := clt.Get(context.Background(), clt.Endpoint("webapi", "u2f", "signuptokens", token.GetName()), url.Values{})
	c.Assert(err, IsNil)

	var u2fRegReq u2f.RegisterChallenge
	c.Assert(json.Unmarshal(re.Bytes(), &u2fRegReq), IsNil)

	u2fRegResp, err := s.mockU2F.RegisterResponse(&u2fRegReq)
	c.Assert(err, IsNil)

	data, err := json.Marshal(authserver.ChangePasswordWithTokenRequest{
		TokenID:             token.GetName(),
		Password:            []byte("qweQWE"),
		U2FRegisterResponse: u2fRegResp,
	})
	c.Assert(err, IsNil)

	req, err := http.NewRequest("PUT", clt.Endpoint("webapi", "users", "password", "token"), bytes.NewBuffer(data))
	c.Assert(err, IsNil)

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	addCSRFCookieToReq(req, csrfToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrf.HeaderName, csrfToken)

	re, err = clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})
	c.Assert(err, IsNil)

	var rawSess *CreateSessionResponse
	c.Assert(json.Unmarshal(re.Bytes(), &rawSess), IsNil)
	c.Assert(rawSess.Token != "", Equals, true)
}

func TestU2FLogin(t *testing.T) {
	for _, sf := range []constants.SecondFactorType{
		constants.SecondFactorU2F,
		constants.SecondFactorOptional,
		constants.SecondFactorOn,
		constants.SecondFactorOff,
	} {
		sf := sf
		t.Run(fmt.Sprintf("second_factor_%s", sf), func(t *testing.T) {
			t.Parallel()
			testU2FLogin(t, sf)
		})
	}
}

func testU2FLogin(t *testing.T, secondFactor constants.SecondFactorType) {
	env := newWebPack(t, 1)

	// configure cluster authentication preferences
	cap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorU2F,
		U2F: &services.U2F{
			AppID:  "https://" + env.server.ClusterName(),
			Facets: []string{"https://" + env.server.ClusterName()},
		},
	})
	require.NoError(t, err)
	err = env.server.Auth().SetAuthPreference(cap)
	require.NoError(t, err)

	// create user
	ctx := context.TODO()
	env.proxies[0].createUser(ctx, t, "bob", "root", "password", "")

	// create password change token
	token, err := env.server.Auth().CreateResetPasswordToken(context.TODO(), authserver.CreateResetPasswordTokenRequest{
		Name: "bob",
	})
	require.NoError(t, err)

	u2fRegReq, err := env.proxies[0].client.GetSignupU2FRegisterRequest(token.GetName())
	require.NoError(t, err)

	mockU2F, err := mocku2f.Create()
	require.NoError(t, err)
	u2fRegResp, err := mockU2F.RegisterResponse(u2fRegReq)
	require.NoError(t, err)

	tempPass := []byte("abc123")
	_, err = env.proxies[0].client.ChangePasswordWithToken(context.TODO(), authserver.ChangePasswordWithTokenRequest{
		TokenID:             token.GetName(),
		U2FRegisterResponse: u2fRegResp,
		Password:            tempPass,
	})
	require.NoError(t, err)

	// normal login
	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)
	re, err := clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "signrequest"), client.MFAChallengeRequest{
		User: "bob",
		Pass: string(tempPass),
	})
	require.NoError(t, err)
	var u2fSignReq u2f.AuthenticateChallenge
	require.NoError(t, json.Unmarshal(re.Bytes(), &u2fSignReq))

	u2fSignResp, err := mockU2F.SignResponse(&u2fSignReq)
	require.NoError(t, err)

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignResp,
	})
	require.NoError(t, err)

	// bad login: corrupted sign responses, should fail
	re, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "signrequest"), client.MFAChallengeRequest{
		User: "bob",
		Pass: string(tempPass),
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &u2fSignReq))

	u2fSignResp, err = mockU2F.SignResponse(&u2fSignReq)
	require.NoError(t, err)

	// corrupted KeyHandle
	u2fSignRespCopy := u2fSignResp
	u2fSignRespCopy.KeyHandle = u2fSignRespCopy.KeyHandle + u2fSignRespCopy.KeyHandle
	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignRespCopy,
	})
	require.Error(t, err)

	// corrupted SignatureData
	u2fSignRespCopy = u2fSignResp
	u2fSignRespCopy.SignatureData = u2fSignRespCopy.SignatureData[:10] + u2fSignRespCopy.SignatureData[20:]

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignRespCopy,
	})
	require.Error(t, err)

	// corrupted ClientData
	u2fSignRespCopy = u2fSignResp
	u2fSignRespCopy.ClientData = u2fSignRespCopy.ClientData[:10] + u2fSignRespCopy.ClientData[20:]

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignRespCopy,
	})
	require.Error(t, err)

	// bad login: counter not increasing, should fail
	mockU2F.SetCounter(0)
	re, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "signrequest"), client.MFAChallengeRequest{
		User: "bob",
		Pass: string(tempPass),
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &u2fSignReq))

	u2fSignResp, err = mockU2F.SignResponse(&u2fSignReq)
	require.NoError(t, err)

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignResp,
	})
	require.Error(t, err)
}

// TestPing ensures that a response is returned by /webapi/ping
// and that that response body contains authentication information.
func (s *WebSuite) TestPing(c *C) {
	wc := s.client()

	re, err := wc.Get(context.Background(), wc.Endpoint("webapi", "ping"), url.Values{})
	c.Assert(err, IsNil)

	var out *apiclient.PingResponse
	c.Assert(json.Unmarshal(re.Bytes(), &out), IsNil)

	preference, err := s.server.Auth().GetAuthPreference()
	c.Assert(err, IsNil)

	c.Assert(out.Auth.Type, Equals, preference.GetType())
	c.Assert(out.Auth.SecondFactor, Equals, preference.GetSecondFactor())
}

func (s *WebSuite) TestMultipleConnectors(c *C) {
	ctx := context.Background()
	wc := s.client()

	// create two oidc connectors, one named "foo" and another named "bar"
	oidcConnectorSpec := services.OIDCConnectorSpecV2{
		RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
		ClientID:     "000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com",
		ClientSecret: "AAAAAAAAAAAAAAAAAAAAAAAA",
		IssuerURL:    "https://oidc.example.com",
		Display:      "Login with Example",
		Scope:        []string{"group"},
		ClaimsToRoles: []services.ClaimMapping{
			{
				Claim: "group",
				Value: "admin",
				Roles: []string{"admin"},
			},
		},
	}
	err := s.server.Auth().UpsertOIDCConnector(ctx, services.NewOIDCConnector("foo", oidcConnectorSpec))
	c.Assert(err, IsNil)
	err = s.server.Auth().UpsertOIDCConnector(ctx, services.NewOIDCConnector("bar", oidcConnectorSpec))
	c.Assert(err, IsNil)

	// set the auth preferences to oidc with no connector name
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type: "oidc",
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	// hit the ping endpoint to get the auth type and connector name
	re, err := wc.Get(ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	c.Assert(err, IsNil)
	var out *apiclient.PingResponse
	c.Assert(json.Unmarshal(re.Bytes(), &out), IsNil)

	// make sure the connector name we got back was the first connector
	// in the backend, in this case it's "bar"
	oidcConnectors, err := s.server.Auth().GetOIDCConnectors(ctx, false)
	c.Assert(err, IsNil)
	c.Assert(out.Auth.OIDC.Name, Equals, oidcConnectors[0].GetName())

	// update the auth preferences and this time specify the connector name
	authPreference, err = services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:          "oidc",
		ConnectorName: "foo",
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	// hit the ping endpoing to get the auth type and connector name
	re, err = wc.Get(ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal(re.Bytes(), &out), IsNil)

	// make sure the connector we get back is "foo"
	c.Assert(out.Auth.OIDC.Name, Equals, "foo")
}

// TestConstructSSHResponse checks if the secret package uses AES-GCM to
// encrypt and decrypt data that passes through the ConstructSSHResponse
// function.
func (s *WebSuite) TestConstructSSHResponse(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)

	u, err := url.Parse("http://www.example.com/callback")
	c.Assert(err, IsNil)
	query := u.Query()
	query.Set("secret_key", key.String())
	u.RawQuery = query.Encode()

	rawresp, err := ConstructSSHResponse(AuthParams{
		Username:          "foo",
		Cert:              []byte{0x00},
		TLSCert:           []byte{0x01},
		ClientRedirectURL: u.String(),
	})
	c.Assert(err, IsNil)

	c.Assert(rawresp.Query().Get("secret"), Equals, "")
	c.Assert(rawresp.Query().Get("secret_key"), Equals, "")
	c.Assert(rawresp.Query().Get("response"), Not(Equals), "")

	plaintext, err := key.Open([]byte(rawresp.Query().Get("response")))
	c.Assert(err, IsNil)

	var resp *authserver.SSHLoginResponse
	err = json.Unmarshal(plaintext, &resp)
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, "foo")
	c.Assert(resp.Cert, DeepEquals, []byte{0x00})
	c.Assert(resp.TLSCert, DeepEquals, []byte{0x01})
}

// TestConstructSSHResponseLegacy checks if the secret package uses NaCl to
// encrypt and decrypt data that passes through the ConstructSSHResponse
// function.
func (s *WebSuite) TestConstructSSHResponseLegacy(c *C) {
	key, err := lemma_secret.NewKey()
	c.Assert(err, IsNil)

	lemma, err := lemma_secret.New(&lemma_secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)

	u, err := url.Parse("http://www.example.com/callback")
	c.Assert(err, IsNil)
	query := u.Query()
	query.Set("secret", lemma_secret.KeyToEncodedString(key))
	u.RawQuery = query.Encode()

	rawresp, err := ConstructSSHResponse(AuthParams{
		Username:          "foo",
		Cert:              []byte{0x00},
		TLSCert:           []byte{0x01},
		ClientRedirectURL: u.String(),
	})
	c.Assert(err, IsNil)

	c.Assert(rawresp.Query().Get("secret"), Equals, "")
	c.Assert(rawresp.Query().Get("secret_key"), Equals, "")
	c.Assert(rawresp.Query().Get("response"), Not(Equals), "")

	var sealedData *lemma_secret.SealedBytes
	err = json.Unmarshal([]byte(rawresp.Query().Get("response")), &sealedData)
	c.Assert(err, IsNil)

	plaintext, err := lemma.Open(sealedData)
	c.Assert(err, IsNil)

	var resp *authserver.SSHLoginResponse
	err = json.Unmarshal(plaintext, &resp)
	c.Assert(err, IsNil)
	c.Assert(resp.Username, Equals, "foo")
	c.Assert(resp.Cert, DeepEquals, []byte{0x00})
	c.Assert(resp.TLSCert, DeepEquals, []byte{0x01})
}

// TestSearchClusterEvents makes sure web API allows querying events by type.
func (s *WebSuite) TestSearchClusterEvents(c *C) {
	sessionEvents := events.GenerateTestSession(events.SessionParams{
		PrintEvents: 3,
		Clock:       s.clock,
		ServerID:    s.proxy.ID(),
	})

	for _, e := range sessionEvents {
		c.Assert(s.proxyClient.EmitAuditEvent(context.TODO(), e), IsNil)
	}

	sessionStart := sessionEvents[0]
	sessionPrint := sessionEvents[1]
	sessionEnd := sessionEvents[4]

	testCases := []struct {
		// Comment is the test case description.
		Comment string
		// Query is the search query sent to the API.
		Query url.Values
		// Result is the expected returned list of events.
		Result []events.AuditEvent
	}{
		{
			Comment: "Empty query",
			Query:   url.Values{},
			Result:  sessionEvents,
		},
		{
			Comment: "Query by session start event",
			Query:   url.Values{"include": []string{sessionStart.GetType()}},
			Result:  sessionEvents[:1],
		},
		{
			Comment: "Query session start and session end events",
			Query:   url.Values{"include": []string{sessionEnd.GetType() + ";" + sessionStart.GetType()}},
			Result:  []events.AuditEvent{sessionStart, sessionEnd},
		},
		{
			Comment: "Query events with filter by type and limit",
			Query: url.Values{
				"include": []string{sessionPrint.GetType() + ";" + sessionEnd.GetType()},
				"limit":   []string{"1"},
			},
			Result: []events.AuditEvent{sessionPrint},
		},
	}

	pack := s.authPack(c, "foo")
	for _, tc := range testCases {
		result := s.searchEvents(c, pack.clt, tc.Query, []string{sessionStart.GetType(), sessionPrint.GetType(), sessionEnd.GetType()})
		c.Assert(result, HasLen, len(tc.Result), Commentf(tc.Comment))
		for i, resultEvent := range result {
			c.Assert(resultEvent.GetType(), Equals, tc.Result[i].GetType(), Commentf(tc.Comment))
			c.Assert(resultEvent.GetID(), Equals, tc.Result[i].GetID(), Commentf(tc.Comment))
		}
	}
}

func (s *WebSuite) searchEvents(c *C, clt *client.WebClient, query url.Values, filter []string) (result []events.EventFields) {
	response, err := clt.Get(context.Background(), clt.Endpoint("webapi", "sites", s.server.ClusterName(), "events", "search"), query)
	c.Assert(err, IsNil)
	var out eventsListGetResponse
	c.Assert(json.Unmarshal(response.Bytes(), &out), IsNil)
	for _, event := range out.Events {
		if utils.SliceContainsStr(filter, event.GetType()) {
			result = append(result, event)
		}
	}
	return result
}

func (s *WebSuite) TestGetClusterDetails(c *C) {
	site, err := s.proxyTunnel.GetSite(s.server.ClusterName())
	c.Assert(err, IsNil)
	c.Assert(site, NotNil)

	cluster, err := ui.GetClusterDetails(site)
	c.Assert(err, IsNil)
	c.Assert(cluster.Name, Equals, s.server.ClusterName())
	c.Assert(cluster.ProxyVersion, Equals, teleport.Version)
	c.Assert(cluster.PublicURL, Equals, fmt.Sprintf("%v:%v", s.server.ClusterName(), defaults.HTTPListenPort))
	c.Assert(cluster.Status, Equals, teleport.RemoteClusterStatusOnline)
	c.Assert(cluster.LastConnected, NotNil)
	c.Assert(cluster.AuthVersion, Equals, teleport.Version)

	nodes, err := s.proxyClient.GetNodes(defaults.Namespace)
	c.Assert(err, IsNil)
	c.Assert(nodes, HasLen, cluster.NodeCount)
}

type testModules struct {
	modules.Modules
}

func (m *testModules) Features() modules.Features {
	return modules.Features{
		App: false, // Explicily turn off application access.
	}
}

// TestApplicationAccessDisabled makes sure application access can be disabled
// via modules.
func TestApplicationAccessDisabled(t *testing.T) {
	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testModules{})

	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo@example.com")

	// Register an application.
	server := &types.ServerV2{
		Kind:    types.KindAppServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: defaults.Namespace,
			Name:      uuid.New(),
		},
		Spec: types.ServerSpecV2{
			Version: teleport.Version,
			Apps: []*types.App{
				{
					Name:       "panel",
					PublicAddr: "panel.example.com",
					URI:        "http://127.0.0.1:8080",
				},
			},
		},
	}
	_, err := env.server.Auth().UpsertAppServer(context.Background(), server)
	require.NoError(t, err)

	endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
	_, err = pack.clt.PostJSON(context.Background(), endpoint, &CreateAppSessionRequest{
		FQDN:        "panel.example.com",
		PublicAddr:  "panel.example.com",
		ClusterName: "localhost",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster doesn't support application access")
}

// TestCreateAppSession verifies that an existing session to the Web UI can
// be exchanged for a application specific session.
func (s *WebSuite) TestCreateAppSession(c *C) {
	pack := s.authPack(c, "foo@example.com")

	// Register an application called "panel".
	server := &services.ServerV2{
		Kind:    services.KindAppServer,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      uuid.New(),
		},
		Spec: services.ServerSpecV2{
			Version: teleport.Version,
			Apps: []*services.App{
				{
					Name:       "panel",
					PublicAddr: "panel.example.com",
					URI:        "http://127.0.0.1:8080",
				},
			},
		},
	}
	_, err := s.server.Auth().UpsertAppServer(context.Background(), server)
	c.Assert(err, IsNil)

	// Extract the session ID and bearer token for the current session.
	rawCookie := *pack.cookies[0]
	cookieBytes, err := hex.DecodeString(rawCookie.Value)
	c.Assert(err, IsNil)
	var sessionCookie SessionCookie
	err = json.Unmarshal(cookieBytes, &sessionCookie)
	c.Assert(err, IsNil)

	var tests = []struct {
		inComment       CommentInterface
		inCreateRequest *CreateAppSessionRequest
		outError        bool
		outFQDN         string
		outUsername     string
	}{
		{
			inComment: Commentf("Valid request: all fields."),
			inCreateRequest: &CreateAppSessionRequest{
				FQDN:        "panel.example.com",
				PublicAddr:  "panel.example.com",
				ClusterName: "localhost",
			},
			outError:    false,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			inComment: Commentf("Valid request: without FQDN."),
			inCreateRequest: &CreateAppSessionRequest{
				PublicAddr:  "panel.example.com",
				ClusterName: "localhost",
			},
			outError:    false,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			inComment: Commentf("Valid request: only FQDN."),
			inCreateRequest: &CreateAppSessionRequest{
				FQDN: "panel.example.com",
			},
			outError:    false,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			inComment: Commentf("Invalid request: only public address."),
			inCreateRequest: &CreateAppSessionRequest{
				PublicAddr: "panel.example.com",
			},
			outError: true,
		},
		{
			inComment: Commentf("Invalid request: only cluster name."),
			inCreateRequest: &CreateAppSessionRequest{
				ClusterName: "localhost",
			},
			outError: true,
		},
		{
			inComment: Commentf("Invalid application."),
			inCreateRequest: &CreateAppSessionRequest{
				FQDN:        "panel.example.com",
				PublicAddr:  "invalid.example.com",
				ClusterName: "localhost",
			},
			outError: true,
		},
		{
			inComment: Commentf("Invalid cluster name."),
			inCreateRequest: &CreateAppSessionRequest{
				FQDN:        "panel.example.com",
				PublicAddr:  "panel.example.com",
				ClusterName: "example.com",
			},
			outError: true,
		},
		{
			inComment: Commentf("Malicious request: all fields."),
			inCreateRequest: &CreateAppSessionRequest{
				FQDN:        "panel.example.com@malicious.com",
				PublicAddr:  "panel.example.com",
				ClusterName: "localhost",
			},
			outError:    false,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			inComment: Commentf("Malicious request: only FQDN."),
			inCreateRequest: &CreateAppSessionRequest{
				FQDN: "panel.example.com@malicious.com",
			},
			outError: true,
		},
	}

	for _, tt := range tests {
		// Make a request to create an application session for "panel".
		endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
		resp, err := pack.clt.PostJSON(context.Background(), endpoint, tt.inCreateRequest)
		c.Assert(err != nil, Equals, tt.outError, tt.inComment)
		if tt.outError {
			continue
		}

		// Unmarshal the response.
		var response *CreateAppSessionResponse
		c.Assert(json.Unmarshal(resp.Bytes(), &response), IsNil, tt.inComment)
		c.Assert(response.FQDN, Equals, tt.outFQDN, tt.inComment)

		// Verify that the application session was created.
		session, err := s.server.Auth().GetAppSession(context.Background(), services.GetAppSessionRequest{
			SessionID: response.CookieValue,
		})
		c.Assert(err, IsNil)
		c.Assert(session.GetUser(), Equals, tt.outUsername, tt.inComment)
		c.Assert(session.GetName(), Equals, response.CookieValue, tt.inComment)
	}
}

// TestWebSessionsRenewDoesNotBreakExistingTerminalSession validates that the
// session renewed via one proxy does not force the terminals created by another
// proxy to disconnect
//
// See https://github.com/gravitational/teleport/issues/5265
func TestWebSessionsRenewDoesNotBreakExistingTerminalSession(t *testing.T) {
	env := newWebPack(t, 2)

	proxy1, proxy2 := env.proxies[0], env.proxies[1]
	// Connect to both proxies
	pack1 := proxy1.authPack(t, "foo")
	pack2 := proxy2.authPackFromPack(t, pack1)

	ws := proxy2.makeTerminal(t, pack2, session.NewID())

	// Advance the time before renewing the session.
	// This will allow the new session to have a more plausible
	// expiration
	const delta = 30 * time.Second
	env.clock.Advance(authserver.BearerTokenTTL - delta)

	// Renew the session using the 1st proxy
	resp := pack1.renewSession(context.TODO(), t)

	// Expire the old session and make sure it has been removed.
	// The bearer token is also removed after this point, so we have to
	// use the new session data for future connects
	env.clock.Advance(delta + 1*time.Second)
	pack2 = proxy2.authPackFromResponse(t, resp)

	// Verify that access via the 2nd proxy also works for the same session
	pack2.validateAPI(context.TODO(), t)

	// Check whether the terminal session is still active
	validateTerminalStream(t, ws)
}

// TestWebSessionsRenewAllowsOldBearerTokenToLinger validates that the
// bearer token bound to the previous session is still active after the
// session renewal, if the renewal happens with a time margin.
//
// See https://github.com/gravitational/teleport/issues/5265
func TestWebSessionsRenewAllowsOldBearerTokenToLinger(t *testing.T) {
	// Login to implicitly create a new web session
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo")

	delta := 30 * time.Second
	// Advance the time before renewing the session.
	// This will allow the new session to have a more plausible
	// expiration
	env.clock.Advance(authserver.BearerTokenTTL - delta)

	// make sure we can use client to make authenticated requests
	// before we issue this request, we will recover session id and bearer token
	//
	prevSessionCookie := *pack.cookies[0]
	prevBearerToken := pack.session.Token
	resp := pack.renewSession(context.TODO(), t)

	newPack := proxy.authPackFromResponse(t, resp)

	// new session is functioning
	newPack.validateAPI(context.TODO(), t)

	sessionCookie := *newPack.cookies[0]
	bearerToken := newPack.session.Token
	require.NotEmpty(t, bearerToken)
	require.NotEmpty(t, cmp.Diff(bearerToken, prevBearerToken))

	prevSessionID := decodeSessionCookie(t, prevSessionCookie.Value)
	activeSessionID := decodeSessionCookie(t, sessionCookie.Value)
	require.NotEmpty(t, cmp.Diff(prevSessionID, activeSessionID))

	// old session is still valid
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	oldClt := proxy.newClient(t, roundtrip.BearerAuth(prevBearerToken), roundtrip.CookieJar(jar))
	jar.SetCookies(&proxy.webURL, []*http.Cookie{&prevSessionCookie})
	_, err = oldClt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)

	// now expire the old session and make sure it has been removed
	env.clock.Advance(delta)

	_, err = proxy.client.GetWebSession(context.TODO(), types.GetWebSessionRequest{
		User:      "foo",
		SessionID: prevSessionID,
	})
	require.Regexp(t, "^key.*not found$", err.Error())

	// now delete session
	_, err = newPack.clt.Delete(
		context.Background(),
		pack.clt.Endpoint("webapi", "sessions"))
	require.NoError(t, err)

	// subsequent requests to use this session will fail
	_, err = newPack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.True(t, trace.IsAccessDenied(err))
}

type authProviderMock struct {
	server services.ServerV2
}

func (mock authProviderMock) GetNodes(n string, opts ...auth.MarshalOption) ([]services.Server, error) {
	return []services.Server{&mock.server}, nil
}

func (mock authProviderMock) GetSessionEvents(n string, s session.ID, c int, p bool) ([]events.EventFields, error) {
	return []events.EventFields{}, nil
}

func (s *WebSuite) makeTerminal(pack *authPack, opts ...session.ID) (*websocket.Conn, error) {
	var sessionID session.ID
	if len(opts) == 0 {
		sessionID = session.NewID()
	} else {
		sessionID = opts[0]
	}

	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect", currentSiteShortcut),
	}
	data, err := json.Marshal(TerminalRequest{
		Server: s.srvID,
		Login:  pack.login,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
		SessionID: sessionID,
	})
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AccessTokenQueryParam, pack.session.Token)
	u.RawQuery = q.Encode()

	wscfg, err := websocket.NewConfig(u.String(), "http://localhost")
	wscfg.TlsConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	if err != nil {
		return nil, err
	}

	for _, cookie := range pack.cookies {
		wscfg.Header.Add("Cookie", cookie.String())
	}

	ws, err := websocket.DialConfig(wscfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ws, nil
}

func waitForOutput(stream *terminalStream, substr string) error {
	timeoutCh := time.After(10 * time.Second)

	for {
		select {
		case <-timeoutCh:
			return trace.BadParameter("timeout waiting on terminal for output: %v", substr)
		default:
		}

		out := make([]byte, 100)
		_, err := stream.Read(out)
		if err != nil {
			return trace.Wrap(err)
		}
		if strings.Contains(removeSpace(string(out)), substr) {
			return nil
		}
	}
}

func (s *WebSuite) waitForRawEvent(ws *websocket.Conn, timeout time.Duration) error {
	timeoutContext, timeoutCancel := context.WithTimeout(context.Background(), timeout)
	defer timeoutCancel()

	done := make(chan error, 1)

	go func() {
		for {
			var raw []byte
			err := websocket.Message.Receive(ws, &raw)
			if err != nil {
				done <- trace.Wrap(err)
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

func (s *WebSuite) waitForResizeEvent(ws *websocket.Conn, timeout time.Duration) error {
	timeoutContext, timeoutCancel := context.WithTimeout(context.Background(), timeout)
	defer timeoutCancel()

	done := make(chan error, 1)

	go func() {
		for {
			var raw []byte
			err := websocket.Message.Receive(ws, &raw)
			if err != nil {
				done <- trace.Wrap(err)
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

func (s *WebSuite) clientNoRedirects(opts ...roundtrip.ClientParam) *client.WebClient {
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

func (s *WebSuite) client(opts ...roundtrip.ClientParam) *client.WebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	wc, err := client.NewWebClient(s.url().String(), opts...)
	if err != nil {
		panic(err)
	}
	return wc
}

func (s *WebSuite) login(clt *client.WebClient, cookieToken string, reqToken string, reqData interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(reqData)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest("POST", clt.Endpoint("webapi", "sessions"), bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		addCSRFCookieToReq(req, cookieToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrf.HeaderName, reqToken)
		return clt.HTTPClient().Do(req)
	}))
}

func (s *WebSuite) url() *url.URL {
	u, err := url.Parse("https://" + s.webServer.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return u
}

func addCSRFCookieToReq(req *http.Request, token string) {
	cookie := &http.Cookie{
		Name:  csrf.CookieName,
		Value: token,
	}

	req.AddCookie(cookie)
}

func removeSpace(in string) string {
	for _, c := range []string{"\n", "\r", "\t"} {
		in = strings.Replace(in, c, " ", -1)
	}
	return strings.TrimSpace(in)
}

func newTerminalHandler() TerminalHandler {
	return TerminalHandler{
		log:     logrus.WithFields(logrus.Fields{}),
		encoder: unicode.UTF8.NewEncoder(),
		decoder: unicode.UTF8.NewDecoder(),
	}
}

func decodeSessionCookie(t *testing.T, value string) (sessionID string) {
	sessionBytes, err := hex.DecodeString(value)
	require.NoError(t, err)
	var cookie struct {
		User      string `json:"user"`
		SessionID string `json:"sid"`
	}
	require.NoError(t, json.Unmarshal(sessionBytes, &cookie))
	return cookie.SessionID
}

func (r CreateSessionResponse) response() (*CreateSessionResponse, error) {
	return &CreateSessionResponse{Type: r.Type, Token: r.Token, ExpiresIn: r.ExpiresIn}, nil
}

func newWebPack(t *testing.T, numProxies int) *webPack {
	clock := clockwork.NewFakeClock()

	authServer, err := test.NewAuthServer(test.AuthServerConfig{
		ClusterName: "localhost",
		Dir:         t.TempDir(),
		Clock:       clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	server, err := authServer.NewTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Close()) })

	// Register the auth server, since test auth server doesn't start its own
	// heartbeat.
	err = authServer.AuthServer.UpsertAuthServer(&services.ServerV2{
		Kind:    services.KindAuthServer,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      "auth",
		},
		Spec: services.ServerSpecV2{
			Addr:     server.Listener.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	require.NoError(t, err)

	// start auth server
	certs, err := server.Auth().GenerateServerKeys(authserver.GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: server.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleNode},
	})
	require.NoError(t, err)

	signer, err := sshutils.NewSigner(certs.Key, certs.Cert)
	require.NoError(t, err)

	const nodeID = "node"
	nodeClient, err := server.NewClient(test.Identity{
		I: authserver.BuiltinRole{
			Role:     teleport.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeClient.Close()) })

	hostSigners := []ssh.Signer{signer}
	// create SSH service:
	nodeDataDir := t.TempDir()
	node, err := regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		server.ClusterName(),
		hostSigners,
		nodeClient,
		nodeDataDir,
		"",
		utils.NetAddr{},
		regular.SetUUID(nodeID),
		regular.SetNamespace(defaults.Namespace),
		regular.SetShell("/bin/sh"),
		regular.SetSessionServer(nodeClient),
		regular.SetEmitter(nodeClient),
		regular.SetPAMConfig(&pam.Config{Enabled: false}),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(clock),
	)
	require.NoError(t, err)

	require.NoError(t, node.Start())
	t.Cleanup(func() { require.NoError(t, node.Close()) })
	require.NoError(t, test.CreateUploaderDir(nodeDataDir))

	var proxies []*proxy
	for p := 0; p < numProxies; p++ {
		proxyID := fmt.Sprintf("proxy%v", p)
		proxies = append(proxies, createProxy(t, proxyID, node, server, hostSigners, clock))
	}

	// Wait for proxies to fully register before starting the test.
	for start := time.Now(); ; {
		proxies, err := proxies[0].client.GetProxies()
		require.NoError(t, err)
		if len(proxies) == numProxies {
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("Proxies didn't register within 5s after startup; registered: %d, want: %d", len(proxies), numProxies)
		}
	}

	return &webPack{
		proxies: proxies,
		server:  server,
		node:    node,
		clock:   clock,
	}
}

func createProxy(t *testing.T, proxyID string, node *regular.Server, authServer *test.TLSServer,
	hostSigners []ssh.Signer, clock clockwork.FakeClock) *proxy {

	// create reverse tunnel service:
	client, err := authServer.NewClient(test.Identity{
		I: authserver.BuiltinRole{
			Role:     teleport.RoleProxy,
			Username: proxyID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	revTunListener, err := net.Listen("tcp", fmt.Sprintf("%v:0", authServer.ClusterName()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, revTunListener.Close()) })

	revTunServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:                    node.ID(),
		Listener:              revTunListener,
		ClientTLS:             client.TLSConfig(),
		ClusterName:           authServer.ClusterName(),
		HostSigners:           hostSigners,
		LocalAuthClient:       client,
		LocalAccessPoint:      client,
		Emitter:               client,
		NewCachingAccessPoint: authclient.NoCache,
		DirectClusters:        []reversetunnel.DirectCluster{{Name: authServer.ClusterName(), Client: client}},
		DataDir:               t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, revTunServer.Close()) })

	proxyServer, err := regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		authServer.ClusterName(),
		hostSigners,
		client,
		t.TempDir(),
		"",
		utils.NetAddr{},
		regular.SetUUID(proxyID),
		regular.SetProxyMode(revTunServer),
		regular.SetSessionServer(client),
		regular.SetEmitter(client),
		regular.SetNamespace(defaults.Namespace),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(clock),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyServer.Close()) })

	fs, err := NewDebugFileSystem("../../webassets/teleport")
	require.NoError(t, err)
	handler, err := NewHandler(Config{
		Proxy:        revTunServer,
		AuthServers:  utils.FromAddr(authServer.Addr()),
		DomainName:   authServer.ClusterName(),
		ProxyClient:  client,
		CipherSuites: utils.DefaultCipherSuites(),
		AccessPoint:  client,
		Context:      context.Background(),
		HostUUID:     proxyID,
		Emitter:      client,
		StaticFS:     fs,
	}, SetSessionStreamPollPeriod(200*time.Millisecond), SetClock(clock))
	require.NoError(t, err)

	webServer := httptest.NewTLSServer(handler)
	t.Cleanup(webServer.Close)
	require.NoError(t, proxyServer.Start())

	proxyAddr := utils.MustParseAddr(proxyServer.Addr())
	addr := utils.MustParseAddr(webServer.Listener.Addr().String())
	handler.handler.cfg.ProxyWebAddr = *addr
	handler.handler.cfg.ProxySSHAddr = *proxyAddr
	_, sshPort, err := net.SplitHostPort(proxyAddr.String())
	require.NoError(t, err)
	handler.handler.sshPort = sshPort

	url, err := url.Parse("https://" + webServer.Listener.Addr().String())
	require.NoError(t, err)

	return &proxy{
		clock:   clock,
		auth:    authServer,
		client:  client,
		revTun:  revTunServer,
		node:    node,
		proxy:   proxyServer,
		web:     webServer,
		handler: handler,
		webURL:  *url,
	}
}

// webPack represents the state of a single web test.
// It replicates most of the WebSuite and serves to gradually
// transition the test suite to use the testing package
// directly.
type webPack struct {
	proxies []*proxy
	server  *test.TLSServer
	node    *regular.Server
	clock   clockwork.FakeClock
}

type proxy struct {
	clock   clockwork.FakeClock
	client  *authclient.Client
	auth    *test.TLSServer
	revTun  reversetunnel.Server
	node    *regular.Server
	proxy   *regular.Server
	handler *RewritingHandler
	web     *httptest.Server
	webURL  url.URL
}

// authPack returns new authenticated package consisting of created valid
// user, otp token, created web session and authenticated client.
func (r *proxy) authPack(t *testing.T, user string) *authPack {
	const (
		loginUser = "user"
		pass      = "abc123"
		rawSecret = "def456"
	)
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)

	err = r.auth.Auth().SetAuthPreference(ap)
	require.NoError(t, err)

	r.createUser(context.TODO(), t, user, loginUser, pass, otpSecret)

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, r.clock.Now())
	require.NoError(t, err)

	clt := r.newClient(t)
	req := CreateSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: validToken,
	}

	csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	resp := login(t, clt, csrfToken, csrfToken, req)

	var rawSession *CreateSessionResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &rawSession))

	session, err := rawSession.response()
	require.NoError(t, err)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt = r.newClient(t, roundtrip.BearerAuth(session.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&r.webURL, resp.Cookies())

	return &authPack{
		otpSecret: otpSecret,
		user:      user,
		login:     loginUser,
		session:   session,
		clt:       clt,
		cookies:   resp.Cookies(),
	}
}

func (r *proxy) authPackFromPack(t *testing.T, pack *authPack) *authPack {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt := r.newClient(t, roundtrip.BearerAuth(pack.session.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&r.webURL, pack.cookies)

	result := *pack
	result.clt = clt
	return &result
}

func (r *proxy) authPackFromResponse(t *testing.T, httpResp *roundtrip.Response) *authPack {
	var resp *CreateSessionResponse
	require.NoError(t, json.Unmarshal(httpResp.Bytes(), &resp))

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt := r.newClient(t, roundtrip.BearerAuth(resp.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&r.webURL, httpResp.Cookies())

	session, err := resp.response()
	require.NoError(t, err)
	if session.ExpiresIn < 0 {
		t.Errorf("Expected expiry time to be in the future but got %v", session.ExpiresIn)
	}
	return &authPack{
		session: session,
		clt:     clt,
		cookies: httpResp.Cookies(),
	}
}

func (r *proxy) createUser(ctx context.Context, t *testing.T, user, login, pass, otpSecret string) {
	teleUser, err := services.NewUser(user)
	require.NoError(t, err)

	role := auth.RoleForUser(teleUser)
	role.SetLogins(services.Allow, []string{login})
	options := role.GetOptions()
	options.ForwardAgent = services.NewBool(true)
	role.SetOptions(options)
	err = r.auth.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	teleUser.AddRole(role.GetName())
	teleUser.SetCreatedBy(services.CreatedBy{
		User: services.UserRef{Name: "some-auth-user"},
	})

	err = r.auth.Auth().CreateUser(ctx, teleUser)
	require.NoError(t, err)

	err = r.auth.Auth().UpsertPassword(user, []byte(pass))
	require.NoError(t, err)

	if otpSecret != "" {
		dev, err := auth.NewTOTPDevice("otp", otpSecret, r.clock.Now())
		require.NoError(t, err)
		err = r.auth.Auth().UpsertMFADevice(ctx, user, dev)
		require.NoError(t, err)
	}
}

func (r *proxy) newClient(t *testing.T, opts ...roundtrip.ClientParam) *client.WebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	clt, err := client.NewWebClient(r.webURL.String(), opts...)
	require.NoError(t, err)
	return clt
}

func (r *proxy) makeTerminal(t *testing.T, pack *authPack, sessionID session.ID) *websocket.Conn {
	u := url.URL{
		Host:   r.webURL.Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect", currentSiteShortcut),
	}
	data, err := json.Marshal(TerminalRequest{
		Server: r.node.ID(),
		Login:  pack.login,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
		SessionID: sessionID,
	})
	require.NoError(t, err)

	q := u.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AccessTokenQueryParam, pack.session.Token)
	u.RawQuery = q.Encode()

	wscfg, err := websocket.NewConfig(u.String(), "http://localhost")
	wscfg.TlsConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	require.NoError(t, err)

	for _, cookie := range pack.cookies {
		wscfg.Header.Add("Cookie", cookie.String())
	}

	ws, err := websocket.DialConfig(wscfg)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return ws
}

func login(t *testing.T, clt *client.WebClient, cookieToken, reqToken string, reqData interface{}) *roundtrip.Response {
	resp, err := httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(reqData)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest("POST", clt.Endpoint("webapi", "sessions"), bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		addCSRFCookieToReq(req, cookieToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrf.HeaderName, reqToken)
		return clt.HTTPClient().Do(req)
	}))
	require.NoError(t, err)
	return resp
}

func validateTerminalStream(t *testing.T, conn *websocket.Conn) {
	termHandler := newTerminalHandler()
	stream := termHandler.asTerminalStream(conn)
	_, err := io.WriteString(stream, "echo foo\r\n")
	require.NoError(t, err)

	err = waitForOutput(stream, "foo")
	require.NoError(t, err)
}
