/*
Copyright 2015 Gravitational, Inc.

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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gokyle/hotp"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
	. "gopkg.in/check.v1"
)

func TestWeb(t *testing.T) {
	TestingT(t)
}

type WebSuite struct {
	node        *srv.Server
	srvAddress  string
	srvID       string
	srvHostPort string
	bk          backend.Backend
	roleAuth    *auth.AuthWithRoles
	dir         string
	user        string
	domainName  string
	signer      ssh.Signer
	tunServer   *auth.AuthTunnel
	webServer   *httptest.Server
	freePorts   []string

	// audit log and its dir:
	auditLog events.IAuditLog
	logDir   string
	mockU2F  *mocku2f.Key
}

var _ = Suite(&WebSuite{})

func (s *WebSuite) SetUpSuite(c *C) {
	var err error
	os.Unsetenv(teleport.DebugEnvVar)
	utils.InitLoggerForTests()

	// configure tests to use static assets from web/dist:
	debugAssetsPath = "../../web/dist"
	os.Setenv(teleport.DebugEnvVar, "true")

	sessionStreamPollPeriod = time.Millisecond
	s.logDir = c.MkDir()
	s.auditLog, err = events.NewAuditLog(s.logDir)
	c.Assert(err, IsNil)
	c.Assert(s.auditLog, NotNil)
	s.mockU2F, err = mocku2f.Create()
	c.Assert(err, IsNil)
	c.Assert(s.mockU2F, NotNil)
}

func (s *WebSuite) TearDownSuite(c *C) {
	os.RemoveAll(s.logDir)
	os.Unsetenv(teleport.DebugEnvVar)
}

func (s *WebSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username

	s.freePorts, err = utils.GetFreeTCPPorts(3)
	c.Assert(err, IsNil)

	s.bk, err = boltbk.New(backend.Params{"path": s.dir})
	c.Assert(err, IsNil)

	access := local.NewAccessService(s.bk)
	identity := local.NewIdentityService(s.bk)

	s.domainName = "localhost"
	authServer := auth.NewAuthServer(&auth.InitConfig{
		Backend:    s.bk,
		Authority:  authority.New(),
		DomainName: s.domainName,
		U2F: services.U2F{
			Enabled: true,
			AppID:   "https://" + s.domainName,
			Facets:  []string{"https://" + s.domainName},
		},
		Identity: identity,
		Access:   access,
	})

	teleUser, err := services.NewUser(s.user)
	c.Assert(err, IsNil)
	role := services.RoleForUser(teleUser)
	role.SetLogins([]string{s.user})
	role.SetResource(services.Wildcard, services.RW())
	err = authServer.UpsertRole(role)
	c.Assert(err, IsNil)

	teleUser.AddRole(role.GetName())
	err = authServer.UpsertUser(teleUser)
	c.Assert(err, IsNil)

	newChecker, err := auth.NewAccessChecker(access, identity)
	c.Assert(err, IsNil)

	c.Assert(authServer.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, s.domainName), backend.Forever), IsNil)
	c.Assert(authServer.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, s.domainName), backend.Forever), IsNil)

	sessionServer, err := sess.New(s.bk)
	c.Assert(err, IsNil)

	checker, err := newChecker(s.user)
	c.Assert(err, IsNil)

	s.roleAuth = auth.NewAuthWithRoles(authServer, checker, s.user, sessionServer, s.auditLog)

	// set up host private key and certificate
	hpriv, hpub, err := authServer.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := authServer.GenerateHostCert(
		hpub, s.domainName, s.domainName, teleport.Roles{teleport.RoleAdmin}, 0)
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	s.signer, err = sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	// start node
	nodePort := s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]

	s.srvAddress = fmt.Sprintf("127.0.0.1:%v", nodePort)
	node, err := srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srvAddress},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		srv.SetShell("/bin/sh"),
		srv.SetSessionServer(sessionServer),
		srv.SetAuditLog(s.roleAuth),
	)
	c.Assert(err, IsNil)
	s.node = node
	s.srvID = node.ID()

	c.Assert(s.node.Start(), IsNil)

	revTunServer, err := reversetunnel.NewServer(
		utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        fmt.Sprintf("%v:0", s.domainName),
		},
		[]ssh.Signer{s.signer},
		s.roleAuth,
		reversetunnel.DirectSite(s.domainName, s.roleAuth),
	)
	c.Assert(err, IsNil)

	apiPort := s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]

	tunAddr := utils.NetAddr{
		AddrNetwork: "tcp", Addr: fmt.Sprintf("127.0.0.1:%v", apiPort),
	}
	s.tunServer, err = auth.NewTunnel(
		tunAddr,
		s.signer,
		&auth.APIConfig{
			AuthServer:     authServer,
			SessionService: sessionServer,
			NewChecker:     newChecker,
			AuditLog:       s.auditLog,
		})
	c.Assert(err, IsNil)
	c.Assert(s.tunServer.Start(), IsNil)

	// start handler
	handler, err := NewHandler(Config{
		Proxy:       revTunServer,
		AuthServers: tunAddr,
		DomainName:  s.domainName,
	}, SetSessionStreamPollPeriod(200*time.Millisecond))
	c.Assert(err, IsNil)

	s.webServer = httptest.NewUnstartedServer(handler)
	s.webServer.StartTLS()
}

func (s *WebSuite) url() *url.URL {
	u, err := url.Parse("https://" + s.webServer.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return u
}

func (s *WebSuite) client(opts ...roundtrip.ClientParam) *webClient {
	opts = append(opts, roundtrip.HTTPClient(newInsecureClient()))
	clt, err := newWebClient(s.url().String(), opts...)
	if err != nil {
		panic(err)
	}
	return clt
}

func (s *WebSuite) TearDownTest(c *C) {
	c.Assert(s.node.Close(), IsNil)
	c.Assert(s.tunServer.Close(), IsNil)
	s.webServer.Close()
}

func (s *WebSuite) TestNewUser(c *C) {
	token, err := s.roleAuth.CreateSignupToken(services.UserV1{Name: "bob", AllowedLogins: []string{s.user}})
	c.Assert(err, IsNil)

	tokens, err := s.roleAuth.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 1)
	c.Assert(tokens[0].Token, Equals, token)

	clt := s.client()
	re, err := clt.Get(clt.Endpoint("webapi", "users", "invites", token), url.Values{})
	c.Assert(err, IsNil)

	var out *renderUserInviteResponse
	c.Assert(json.Unmarshal(re.Bytes(), &out), IsNil)
	c.Assert(out.User, Equals, "bob")
	c.Assert(out.InviteToken, Equals, token)

	_, _, hotpValues, err := s.roleAuth.GetSignupTokenData(token)
	c.Assert(err, IsNil)

	tempPass := "abc123"

	re, err = clt.PostJSON(clt.Endpoint("webapi", "users"), createNewUserReq{
		InviteToken:       token,
		Pass:              tempPass,
		SecondFactorToken: hotpValues[0],
	})
	c.Assert(err, IsNil)

	var rawSess *createSessionResponseRaw
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

	re, err = clt.Get(clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	var sites *getSitesResponse
	c.Assert(json.Unmarshal(re.Bytes(), &sites), IsNil)

	// in absense of session cookie or bearer auth the same request fill fail

	// no session cookie:
	clt = s.client(roundtrip.BearerAuth(rawSess.Token))
	re, err = clt.Get(clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)

	// no bearer token:
	clt = s.client(roundtrip.CookieJar(jar))
	re, err = clt.Get(clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

type authPack struct {
	user    string
	otp     *hotp.HOTP
	session *CreateSessionResponse
	clt     *webClient
	cookies []*http.Cookie
}

func (s *WebSuite) authPackFromResponse(c *C, re *roundtrip.Response) *authPack {
	var sess *createSessionResponseRaw
	c.Assert(json.Unmarshal(re.Bytes(), &sess), IsNil)

	jar, err := cookiejar.New(nil)
	c.Assert(err, IsNil)

	clt := s.client(roundtrip.BearerAuth(sess.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), re.Cookies())

	session, user, err := sess.response()
	if err != nil {
		panic(err)
	}
	if session.ExpiresIn < 0 {
		c.Errorf("expected expiry time to be in the future but got %v", session.ExpiresIn)
	}
	return &authPack{
		user:    user.GetName(),
		session: session,
		clt:     clt,
		cookies: re.Cookies(),
	}
}

// authPack returns new authenticated package consisting
// of created valid user, hotp token, created web session and
// authenticated client
func (s *WebSuite) authPack(c *C) *authPack {
	user := s.user
	pass := "abc123"

	teleUser, err := services.NewUser(user)
	c.Assert(err, IsNil)
	role := services.RoleForUser(teleUser)
	role.SetLogins([]string{s.user})
	err = s.roleAuth.UpsertRole(role)
	c.Assert(err, IsNil)
	teleUser.AddRole(role.GetName())

	err = s.roleAuth.UpsertUser(teleUser)
	c.Assert(err, IsNil)

	hotpURL, _, err := s.roleAuth.UpsertPassword(user, []byte(pass))
	c.Assert(err, IsNil)
	otp, _, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	otp.Increment()

	clt := s.client()

	re, err := clt.PostJSON(clt.Endpoint("webapi", "sessions"), createSessionReq{
		User:              user,
		Pass:              pass,
		SecondFactorToken: otp.OTP(),
	})
	c.Assert(err, IsNil)

	var rawSess *createSessionResponseRaw
	c.Assert(json.Unmarshal(re.Bytes(), &rawSess), IsNil)

	sess, _, err := rawSess.response()
	c.Assert(err, IsNil)

	jar, err := cookiejar.New(nil)
	c.Assert(err, IsNil)

	clt = s.client(roundtrip.BearerAuth(sess.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), re.Cookies())

	return &authPack{
		user:    user,
		session: sess,
		clt:     clt,
		cookies: re.Cookies(),
	}
}

func (s *WebSuite) TestWebSessionsCRUD(c *C) {
	pack := s.authPack(c)

	// make sure we can use client to make authenticated requests
	re, err := pack.clt.Get(pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	var sites *getSitesResponse
	c.Assert(json.Unmarshal(re.Bytes(), &sites), IsNil)

	// now delete session
	_, err = pack.clt.Delete(
		pack.clt.Endpoint("webapi", "sessions"))
	c.Assert(err, IsNil)

	// subsequent requests trying to use this session will fail
	re, err = pack.clt.Get(pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *WebSuite) TestWebSessionsRenew(c *C) {
	pack := s.authPack(c)

	// make sure we can use client to make authenticated requests
	// before we issue this request, we will recover session id and bearer token
	//
	prevSessionCookie := *pack.cookies[0]
	prevBearerToken := pack.session.Token
	re, err := pack.clt.PostJSON(pack.clt.Endpoint("webapi", "sessions", "renew"), nil)
	c.Assert(err, IsNil)

	newPack := s.authPackFromResponse(c, re)

	// new session is functioning
	re, err = newPack.clt.Get(pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	// old session is stil valid too (until it expires)
	jar, err := cookiejar.New(nil)
	c.Assert(err, IsNil)
	oldClt := s.client(roundtrip.BearerAuth(prevBearerToken), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), []*http.Cookie{&prevSessionCookie})
	re, err = oldClt.Get(pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	// now delete session
	_, err = newPack.clt.Delete(
		pack.clt.Endpoint("webapi", "sessions"))
	c.Assert(err, IsNil)

	// subsequent requests trying to use this session will fail
	re, err = newPack.clt.Get(pack.clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *WebSuite) TestWebSessionsBadInput(c *C) {
	user := "bob"
	pass := "abc123"

	hotpURL, _, err := s.roleAuth.UpsertPassword(user, []byte(pass))
	c.Assert(err, IsNil)
	otp, _, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	otp.Increment()

	clt := s.client()

	token := otp.OTP()

	reqs := []createSessionReq{
		// emtpy request
		{},
		// missing user
		{
			Pass:              pass,
			SecondFactorToken: token,
		},
		// missing pass
		{
			User:              user,
			SecondFactorToken: token,
		},
		// bad pass
		{
			User:              user,
			Pass:              "bla bla",
			SecondFactorToken: token,
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
		_, err = clt.PostJSON(clt.Endpoint("webapi", "sessions"), req)
		c.Assert(err, NotNil, Commentf("tc %v", i))
		c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("tc %v %T is not access denied", i, err))
	}
}

func (s *WebSuite) TestGetSiteNodes(c *C) {
	pack := s.authPack(c)

	// get site nodes
	re, err := pack.clt.Get(pack.clt.Endpoint("webapi", "sites", s.domainName, "nodes"), url.Values{})
	c.Assert(err, IsNil)

	var nodes *getSiteNodesResponse
	c.Assert(json.Unmarshal(re.Bytes(), &nodes), IsNil)
	c.Assert(len(nodes.Nodes), Equals, 1)

	// get site nodes using shortcut
	re, err = pack.clt.Get(pack.clt.Endpoint("webapi", "sites", currentSiteShortcut, "nodes"), url.Values{})
	c.Assert(err, IsNil)

	var nodes2 *getSiteNodesResponse
	c.Assert(json.Unmarshal(re.Bytes(), &nodes2), IsNil)
	c.Assert(len(nodes.Nodes), Equals, 1)

	c.Assert(nodes2, DeepEquals, nodes)
}

func (s *WebSuite) makeTerminal(c *C, pack *authPack, opts ...session.ID) *websocket.Conn {
	var sessionID session.ID
	if len(opts) == 0 {
		sessionID = session.NewID()
	} else {
		sessionID = opts[0]
	}
	u := url.URL{Host: s.url().Host, Scheme: WSS, Path: fmt.Sprintf("/v1/webapi/sites/%v/connect", currentSiteShortcut)}
	data, err := json.Marshal(terminalRequest{
		ServerID:  s.srvID,
		Login:     s.user,
		Term:      session.TerminalParams{W: 100, H: 100},
		SessionID: sessionID,
	})
	c.Assert(err, IsNil)

	q := u.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AccessTokenQueryParam, pack.session.Token)
	u.RawQuery = q.Encode()

	wscfg, err := websocket.NewConfig(u.String(), "http://localhost")
	wscfg.TlsConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	c.Assert(err, IsNil)
	for _, cookie := range pack.cookies {
		wscfg.Header.Add("Cookie", cookie.String())
	}
	clt, err := websocket.DialConfig(wscfg)
	c.Assert(err, IsNil)

	return clt
}

func (s *WebSuite) sessionStream(c *C, pack *authPack, sessionID session.ID, opts ...string) *websocket.Conn {
	u := url.URL{
		Host:   s.url().Host,
		Scheme: WSS,
		Path: fmt.Sprintf(
			"/v1/webapi/sites/%v/sessions/%v/events/stream",
			currentSiteShortcut,
			sessionID),
	}
	q := u.Query()
	q.Set(roundtrip.AccessTokenQueryParam, pack.session.Token)
	u.RawQuery = q.Encode()
	wscfg, err := websocket.NewConfig(u.String(), "http://localhost")
	wscfg.TlsConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	c.Assert(err, IsNil)
	for _, cookie := range pack.cookies {
		wscfg.Header.Add("Cookie", cookie.String())
	}
	clt, err := websocket.DialConfig(wscfg)
	c.Assert(err, IsNil)

	return clt
}

func (s *WebSuite) TestTerminal(c *C) {
	term := s.makeTerminal(c, s.authPack(c))
	defer term.Close()

	_, err := io.WriteString(term, "echo vinsong\r\n")
	c.Assert(err, IsNil)

	resultC := make(chan struct{})

	go func() {
		out := make([]byte, 100)
		for {
			n, err := term.Read(out)
			c.Assert(err, IsNil)
			c.Assert(n > 0, Equals, true)
			if strings.Contains(removeSpace(string(out)), "vinsong") {
				close(resultC)
				return
			}
		}
	}()

	select {
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for proper response")
	case <-resultC:
		// everything is as expected
	}

}

func (s *WebSuite) TestNodesWithSessions(c *C) {
	sid := session.NewID()
	pack := s.authPack(c)
	clt := s.makeTerminal(c, pack, sid)
	defer clt.Close()

	// to make sure we have a session
	_, err := io.WriteString(clt, "echo vinsong\r\n")
	c.Assert(err, IsNil)

	// make sure server has replied
	out := make([]byte, 1024)
	n := 0
	for err == nil {
		clt.SetReadDeadline(time.Now().Add(time.Millisecond * 20))
		n, err = clt.Read(out)
		if err == nil && n > 0 {
			break
		}
		ne, ok := err.(net.Error)
		if ok && ne.Timeout() {
			err = nil
			continue
		}
		c.Error(err)
	}

	var nodes *getSiteNodesResponse
	for i := 0; i < 10; i++ {
		// get site nodes and make sure the node has our active party
		re, err := pack.clt.Get(pack.clt.Endpoint("webapi", "sites", s.domainName, "nodes"), url.Values{})
		c.Assert(err, IsNil)

		c.Assert(json.Unmarshal(re.Bytes(), &nodes), IsNil)
		c.Assert(len(nodes.Nodes), Equals, 1)

		if len(nodes.Nodes[0].Sessions) == 1 {
			break
		}
		// sessions do not appear momentarily as there's async heartbeat
		// procedure
		time.Sleep(30 * time.Millisecond)
	}

	c.Assert(len(nodes.Nodes[0].Sessions), Equals, 1)
	c.Assert(nodes.Nodes[0].Sessions[0].ID, Equals, sid)

	// connect to session stream and receive events
	stream := s.sessionStream(c, pack, sid)
	defer stream.Close()
	var event *sessionStreamEvent
	c.Assert(websocket.JSON.Receive(stream, &event), IsNil)
	c.Assert(event, NotNil)
}

func (s *WebSuite) TestCloseConnectionsOnLogout(c *C) {
	sid := session.NewID()
	pack := s.authPack(c)
	clt := s.makeTerminal(c, pack, sid)
	defer clt.Close()

	// to make sure we have a session
	_, err := io.WriteString(clt, "expr 137 + 39\r\n")
	c.Assert(err, IsNil)

	// make sure server has replied
	out := make([]byte, 100)
	clt.Read(out)

	_, err = pack.clt.Delete(
		pack.clt.Endpoint("webapi", "sessions"))
	c.Assert(err, IsNil)

	// wait until we timeout or detect that connection has been closed
	after := time.After(time.Second)
	errC := make(chan error)
	go func() {
		for {
			_, err := clt.Read(out)
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
	pack := s.authPack(c)

	sess := session.Session{
		TerminalParams: session.TerminalParams{W: 300, H: 120},
		Login:          s.user,
	}

	re, err := pack.clt.PostJSON(
		pack.clt.Endpoint("webapi", "sites", s.domainName, "sessions"),
		siteSessionGenerateReq{Session: sess},
	)
	c.Assert(err, IsNil)

	var created *siteSessionGenerateResponse
	c.Assert(json.Unmarshal(re.Bytes(), &created), IsNil)
	c.Assert(created.Session.ID, Not(Equals), "")
}

func (s *WebSuite) TestResizeTerminal(c *C) {
	sid := session.NewID()
	pack := s.authPack(c)
	term := s.makeTerminal(c, pack, sid)
	defer term.Close()

	// to make sure we have a session
	_, err := io.WriteString(term, "expr 137 + 39\r\n")
	c.Assert(err, IsNil)

	// make sure server has replied
	out := make([]byte, 100)
	term.Read(out)

	params := session.TerminalParams{W: 300, H: 120}
	_, err = pack.clt.PutJSON(
		pack.clt.Endpoint("webapi", "sites", s.domainName, "sessions", string(sid)),
		siteSessionUpdateReq{TerminalParams: session.TerminalParams{W: 300, H: 120}},
	)
	c.Assert(err, IsNil)

	re, err := pack.clt.Get(
		pack.clt.Endpoint("webapi", "sites", s.domainName, "sessions", string(sid)), url.Values{})
	c.Assert(err, IsNil)

	var se *sess.Session
	c.Assert(json.Unmarshal(re.Bytes(), &se), IsNil)
	c.Assert(se.TerminalParams, DeepEquals, params)
}

func (s *WebSuite) TestPlayback(c *C) {
	pack := s.authPack(c)
	sid := session.NewID()
	term := s.makeTerminal(c, pack, sid)
	defer term.Close()
}

func removeSpace(in string) string {
	for _, c := range []string{"\n", "\r", "\t"} {
		in = strings.Replace(in, c, " ", -1)
	}
	return strings.TrimSpace(in)
}

func (s *WebSuite) TestNewU2FUser(c *C) {
	token, err := s.roleAuth.CreateSignupToken(services.UserV1{Name: "bob", AllowedLogins: []string{s.user}})
	c.Assert(err, IsNil)

	tokens, err := s.roleAuth.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 1)
	c.Assert(tokens[0].Token, Equals, token)

	clt := s.client()
	re, err := clt.Get(clt.Endpoint("webapi", "u2f", "signuptokens", token), url.Values{})
	c.Assert(err, IsNil)

	var u2fRegReq u2f.RegisterRequest
	c.Assert(json.Unmarshal(re.Bytes(), &u2fRegReq), IsNil)

	u2fRegResp, err := s.mockU2F.RegisterResponse(&u2fRegReq)
	c.Assert(err, IsNil)

	tempPass := "abc123"

	re, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "users"), createNewU2FUserReq{
		InviteToken:         token,
		Pass:                tempPass,
		U2FRegisterResponse: *u2fRegResp,
	})
	c.Assert(err, IsNil)

	var rawSess *createSessionResponseRaw
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

	re, err = clt.Get(clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, IsNil)

	var sites *getSitesResponse
	c.Assert(json.Unmarshal(re.Bytes(), &sites), IsNil)

	// in absense of session cookie or bearer auth the same request fill fail

	// no session cookie:
	clt = s.client(roundtrip.BearerAuth(rawSess.Token))
	re, err = clt.Get(clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)

	// no bearer token:
	clt = s.client(roundtrip.CookieJar(jar))
	re, err = clt.Get(clt.Endpoint("webapi", "sites"), url.Values{})
	c.Assert(err, NotNil)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *WebSuite) TestU2FLogin(c *C) {
	token, err := s.roleAuth.CreateSignupToken(services.UserV1{Name: "bob", AllowedLogins: []string{s.user}})
	c.Assert(err, IsNil)

	u2fRegReq, err := s.roleAuth.GetSignupU2FRegisterRequest(token)
	c.Assert(err, IsNil)

	u2fRegResp, err := s.mockU2F.RegisterResponse(u2fRegReq)
	c.Assert(err, IsNil)

	tempPass := "abc123"

	_, err = s.roleAuth.CreateUserWithU2FToken(token, tempPass, *u2fRegResp)
	c.Assert(err, IsNil)

	// normal login

	clt := s.client()
	re, err := clt.PostJSON(clt.Endpoint("webapi", "u2f", "signrequest"), u2fSignRequestReq{
		User: "bob",
		Pass: tempPass,
	})
	c.Assert(err, IsNil)
	var u2fSignReq u2f.SignRequest
	c.Assert(json.Unmarshal(re.Bytes(), &u2fSignReq), IsNil)

	u2fSignResp, err := s.mockU2F.SignResponse(&u2fSignReq)
	c.Assert(err, IsNil)

	_, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignResp,
	})
	c.Assert(err, IsNil)

	// bad login: corrupted sign responses, should fail

	re, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "signrequest"), u2fSignRequestReq{
		User: "bob",
		Pass: tempPass,
	})
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal(re.Bytes(), &u2fSignReq), IsNil)

	u2fSignResp, err = s.mockU2F.SignResponse(&u2fSignReq)
	c.Assert(err, IsNil)

	// corrupted KeyHandle
	u2fSignRespCopy := u2fSignResp
	u2fSignRespCopy.KeyHandle = u2fSignRespCopy.KeyHandle + u2fSignRespCopy.KeyHandle

	_, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignRespCopy,
	})
	c.Assert(err, NotNil)

	// corrupted SignatureData
	u2fSignRespCopy = u2fSignResp
	u2fSignRespCopy.SignatureData = u2fSignRespCopy.SignatureData[:10] + u2fSignRespCopy.SignatureData[20:]

	_, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignRespCopy,
	})
	c.Assert(err, NotNil)

	// corrupted ClientData
	u2fSignRespCopy = u2fSignResp
	u2fSignRespCopy.ClientData = u2fSignRespCopy.ClientData[:10] + u2fSignRespCopy.ClientData[20:]

	_, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignRespCopy,
	})
	c.Assert(err, NotNil)

	// bad login: counter not increasing, should fail

	s.mockU2F.SetCounter(0)

	re, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "signrequest"), u2fSignRequestReq{
		User: "bob",
		Pass: tempPass,
	})
	c.Assert(err, IsNil)
	c.Assert(json.Unmarshal(re.Bytes(), &u2fSignReq), IsNil)

	u2fSignResp, err = s.mockU2F.SignResponse(&u2fSignReq)
	c.Assert(err, IsNil)

	_, err = clt.PostJSON(clt.Endpoint("webapi", "u2f", "sessions"), u2fSignResponseReq{
		User:            "bob",
		U2FSignResponse: *u2fSignResp,
	})
	c.Assert(err, NotNil)
}
