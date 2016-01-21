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
package auth

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/log"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/session"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type TunServer struct {
	certChecker ssh.CertChecker
	a           *AuthServer
	l           net.Listener
	srv         *sshutils.Server
	hostSigner  ssh.Signer
	apiServer   *APIWithRoles
}

type ServerOption func(s *TunServer) error

// New returns an unstarted server
func NewTunServer(addr utils.NetAddr, hostSigners []ssh.Signer,
	apiServer *APIWithRoles, a *AuthServer,
	limiter *limiter.Limiter,
	opts ...ServerOption) (*TunServer, error) {

	srv := &TunServer{
		a:         a,
		apiServer: apiServer,
	}
	for _, o := range opts {
		if err := o(srv); err != nil {
			return nil, err
		}
	}

	s, err := sshutils.NewServer(
		addr,
		srv,
		hostSigners,
		sshutils.AuthMethods{
			Password:  srv.passwordAuth,
			PublicKey: srv.keyAuth,
		},
		limiter,
	)
	if err != nil {
		return nil, err
	}

	srv.certChecker = ssh.CertChecker{IsAuthority: srv.isAuthority}
	srv.srv = s
	return srv, nil
}

func (s *TunServer) Addr() string {
	return s.srv.Addr()
}

func (s *TunServer) Start() error {
	return s.srv.Start()
}

func (s *TunServer) Close() error {
	return s.srv.Close()
}

func (s *TunServer) HandleNewChan(sconn *ssh.ServerConn, nch ssh.NewChannel) {
	log.Infof("got new channel request: %v", nch.ChannelType())
	cht := nch.ChannelType()
	switch cht {
	case ReqDirectTCPIP:
		if !s.haveExt(sconn, ExtHost, ExtWebSession, ExtWebPassword) {
			nch.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("register clients can not TCPIP: %v", cht))
			return
		}
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			log.Errorf("failed to parse request data: %v, err: %v",
				string(nch.ExtraData()), err)
			nch.Reject(ssh.UnknownChannelType,
				"failed to parse direct-tcpip request")
		}
		sshCh, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleDirectTCPIPRequest(sconn, sshCh, req)
	case ReqWebSessionAgent:
		// this is a protective measure, so web requests can be only done
		// if have session ready
		if !s.haveExt(sconn, ExtWebSession) {
			nch.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("don't have web session for: %v", cht))
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleWebAgentRequest(sconn, ch)
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf(
			"unknown channel type: %v", cht))
	}
}

// isAuthority is called during checking the client key, to see if the signing
// key is the real CA authority key.
func (s *TunServer) isAuthority(auth ssh.PublicKey) bool {
	key, err := s.a.GetHostCertificateAuthority()
	if err != nil {
		log.Errorf("failed to retrieve user authority key, err: %v", err)
		return false
	}
	cert, _, _, _, err := ssh.ParseAuthorizedKey(key.PublicKey)
	if err != nil {
		log.Errorf("failed to parse CA cert '%v', err: %v", string(key.PublicKey), err)
		return false
	}

	if !sshutils.KeysEqual(cert, auth) {
		log.Warningf("authority signature check failed, signing keys mismatch")
		return false
	}
	return true
}

func (s *TunServer) haveExt(sconn *ssh.ServerConn, ext ...string) bool {
	if sconn.Permissions == nil {
		return false
	}
	for _, e := range ext {
		if sconn.Permissions.Extensions[e] != "" {
			return true
		}
	}
	return true
}

func (s *TunServer) handleWebAgentRequest(sconn *ssh.ServerConn, ch ssh.Channel) {
	defer ch.Close()

	if sconn.Permissions.Extensions["role"] != RoleWeb {
		log.Errorf("role %v doesn't have permission to request agent",
			sconn.Permissions.Extensions["role"])
		return
	}

	a := agent.NewKeyring()
	log.Infof("handleWebAgentRequest start for %v", sconn.RemoteAddr())

	sessionID := session.SecureID(sconn.Permissions.Extensions[ExtWebSession])

	ws, err := s.a.GetWebSession(sconn.User(), sessionID)
	if err != nil {
		log.Errorf("session error: %v", err)
		return
	}

	priv, err := ssh.ParseRawPrivateKey(ws.WS.Priv)
	if err != nil {
		log.Errorf("session error: %v", err)
		return
	}

	pub, _, _, _, err := ssh.ParseAuthorizedKey(ws.WS.Pub)
	if err != nil {
		log.Errorf("session error: %v", err)
		return
	}

	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		log.Errorf("session error, not a cert: %T", pub)
		return
	}
	addedKey := agent.AddedKey{
		PrivateKey:       priv,
		Certificate:      cert,
		Comment:          "web-session@teleport",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := a.Add(addedKey); err != nil {
		log.Errorf("failed to add: %v", err)
		return
	}
	if err := agent.ServeAgent(a, ch); err != nil {
		log.Errorf("Serve agent err: %v", err)
	}
}

// this direct tcp-ip request ignores port and host requested by client
// and always forwards it to the local auth server listening on local socket
func (s *TunServer) handleDirectTCPIPRequest(sconn *ssh.ServerConn, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	addr, _ := utils.ParseAddr("tcp://localhost")
	conn := utils.NewPipeNetConn(
		ch, ch, ch,
		addr, addr,
	)
	role := sconn.Permissions.Extensions["role"]
	if err := s.apiServer.HandleConn(conn, role); err != nil {
		log.Errorf(err.Error())
	}
}

func (s *TunServer) keyAuth(
	conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cid := fmt.Sprintf(
		"conn(%v->%v, user=%v)", conn.RemoteAddr(),
		conn.LocalAddr(), conn.User())

	log.Infof("%v auth attempt with key %v", cid, key.Type())

	err := s.certChecker.CheckHostKey(conn.User(), conn.RemoteAddr(), key)
	if err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: failed auth user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, err
	}

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return nil, trace.Wrap(err)
	}

	if err := s.certChecker.CheckCert(conn.User(), cert); err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, trace.Wrap(err)
	}

	perms := &ssh.Permissions{
		Extensions: map[string]string{
			ExtHost: conn.User(),
			"role":  cert.Permissions.Extensions["role"],
		},
	}

	return perms, nil
}

func (s *TunServer) passwordAuth(
	conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	var ab *authBucket
	if err := json.Unmarshal(password, &ab); err != nil {
		return nil, err
	}
	log.Infof("got authentication attempt for user '%v' type '%v'", conn.User(), ab.Type)
	switch ab.Type {
	case AuthWebPassword:
		if err := s.a.CheckPassword(conn.User(), ab.Pass, ab.HotpToken); err != nil {
			log.Errorf("Password auth error: %v", err)
			return nil, trace.Wrap(err)
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebPassword: "<password>",
				"role":         RoleUser,
			},
		}
		log.Infof("password authenticated user: '%v'", conn.User())
		return perms, nil
	case AuthWebSession:
		// we use extra permissions mechanism to keep the connection data
		// after authorization, in this case the session
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebSession: string(ab.Pass),
				"role":        RoleWeb,
			},
		}
		if _, err := s.a.GetWebSession(conn.User(), session.SecureID(ab.Pass)); err != nil {
			return nil, trace.Errorf("session resume error: %v", trace.Wrap(err))
		}
		log.Infof("session authenticated user: '%v'", conn.User())
		return perms, nil
	case AuthToken:
		_, err := s.a.ValidateToken(string(ab.Pass), ab.User)
		if err != nil {
			return nil, trace.Errorf("%v token validation error: %v", ab.User, trace.Wrap(err))
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtToken: string(password),
				"role":   RoleProvisionToken,
			}}
		log.Infof("session authenticated prov. token: '%v'", conn.User())
		return perms, nil
	case AuthSignupToken:
		_, err := s.a.GetSignupToken(string(ab.Pass))
		if err != nil {
			return nil, trace.Errorf("token validation error: %v", trace.Wrap(err))
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtToken: string(password),
				"role":   RoleSignup,
			}}
		log.Infof("session authenticated prov. token: '%v'", conn.User())
		return perms, nil
	default:
		return nil, trace.Errorf("unsupported auth method: '%v'", ab.Type)
	}
}

// authBucket uses password to transport app-specific user name and
// auth-type in addition to the password to support auth
type authBucket struct {
	User      string `json:"user"`
	Type      string `json:"type"`
	Pass      []byte `json:"pass"`
	HotpToken string `json:"hotpToken"`
}

func NewTokenAuth(domainName, token string) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthToken,
		User: domainName,
		Pass: []byte(token),
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewWebSessionAuth(user string, session []byte) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthWebSession,
		User: user,
		Pass: session,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewWebPasswordAuth(user string, password []byte, hotpToken string) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type:      AuthWebPassword,
		User:      user,
		Pass:      password,
		HotpToken: hotpToken,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewSignupTokenAuth(token string) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthSignupToken,
		Pass: []byte(token),
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewHostAuth(key, cert []byte) ([]ssh.AuthMethod, error) {
	signer, err := sshutils.NewSigner(key, cert)
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

type TunClient struct {
	Client
	dialer *TunDialer
	tr     *http.Transport
}

func NewTunClient(addr utils.NetAddr, user string, auth []ssh.AuthMethod) (*TunClient, error) {
	tc := &TunClient{
		dialer: &TunDialer{auth: auth, addr: addr, user: user},
	}
	tr := &http.Transport{
		Dial: tc.dialer.Dial,
	}
	clt, err := NewClient(
		"http://stub:0",
		roundtrip.HTTPClient(&http.Client{
			Transport: tr,
		}))
	if err != nil {
		return nil, err
	}
	tc.Client = *clt
	tc.tr = tr
	return tc, nil
}

func (c *TunClient) GetAgent() (agent.Agent, error) {
	return c.dialer.GetAgent()
}

func (c *TunClient) Close() error {
	c.tr.CloseIdleConnections()
	return c.dialer.Close()
}

func (c *TunClient) GetDialer() AccessPointDialer {
	return func() (net.Conn, error) {
		return c.dialer.Dial(c.dialer.addr.AddrNetwork, "accesspoint:0")
	}
}

type TunDialer struct {
	sync.Mutex
	auth []ssh.AuthMethod
	user string
	tun  *ssh.Client
	addr utils.NetAddr
}

func (t *TunDialer) Close() error {
	if t.tun != nil {
		return t.tun.Close()
	}
	return nil
}

func (t *TunDialer) GetAgent() (agent.Agent, error) {
	_, err := t.getClient(false) // we need an established connection first
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ch, _, err := t.tun.OpenChannel(ReqWebSessionAgent, nil)
	if err != nil {
		// reconnecting and trying again
		_, err := t.getClient(true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ch, _, err = t.tun.OpenChannel(ReqWebSessionAgent, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	log.Infof("opened agent channel")
	return agent.NewClient(ch), nil
}

func (t *TunDialer) getClient(reset bool) (*ssh.Client, error) {
	t.Lock()
	defer t.Unlock()
	if t.tun != nil {
		if !reset {
			return t.tun, nil
		} else {
			go t.tun.Close()
			t.tun = nil
		}
	}

	config := &ssh.ClientConfig{
		User: t.user,
		Auth: t.auth,
	}
	client, err := ssh.Dial(t.addr.AddrNetwork, t.addr.Addr, config)
	if err != nil {
		return nil, err
	}
	t.tun = client
	return t.tun, nil
}

func (t *TunDialer) Dial(network, address string) (net.Conn, error) {
	c, err := t.getClient(false)
	if err != nil {
		return nil, err
	}
	conn, err := c.Dial(network, address)
	if err == nil {
		return conn, err
	} else {
		// reconnecting and trying again
		c, err = t.getClient(true)
		if err != nil {
			return nil, err
		}
		return c.Dial(network, address)
	}
}

const (
	ReqWebSessionAgent = "web-session-agent@teleport"
	ReqProvision       = "provision@teleport"
	ReqDirectTCPIP     = "direct-tcpip"
	ReqNewAuth         = "new-auth@teleport"

	ExtWebSession  = "web-session@teleport"
	ExtWebPassword = "web-password@teleport"
	ExtToken       = "provision@teleport"
	ExtHost        = "host@teleport"

	AuthWebPassword = "password"
	AuthWebSession  = "session"
	AuthToken       = "provision-token"
	AuthSignupToken = "signup-token"
)

// AccessPointDialer dials to auth access point  remote HTTP api
type AccessPointDialer func() (net.Conn, error)
