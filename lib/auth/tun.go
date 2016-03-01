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
	"os"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// AuthTunnel listens on TCP/IP socket and accepts SSH connections. It then stablishes
// an SSH tunnell which HTTP requests travel over. In other words, the Auth Service API
// runs on HTTP-via-SSH-tunnel.
type AuthTunnel struct {
	// authServer implements the "beef" of the Auth service
	authServer *AuthServer
	// apiServer maintains a map of API servers, each assigned
	// to a certain role. this allows to break Auth API into
	// a set of restricted API services with well-defined permissions
	apiServer *APIWithRoles
	// sshServer implements the nuts & bolts of serving an SSH connection
	// to create a tunnel
	sshServer       *sshutils.Server
	hostSigner      ssh.Signer
	hostCertChecker ssh.CertChecker
	userCertChecker ssh.CertChecker
	limiter         *limiter.Limiter
}

type ServerOption func(s *AuthTunnel) error

func SetLimiter(limiter *limiter.Limiter) ServerOption {
	return func(s *AuthTunnel) error {
		s.limiter = limiter
		return nil
	}
}

// NewTunnel creates a new SSH tunnel server which is not started yet
func NewTunnel(addr utils.NetAddr,
	hostSigners []ssh.Signer,
	apiServer *APIWithRoles,
	authServer *AuthServer,
	opts ...ServerOption) (tunnel *AuthTunnel, err error) {

	tunnel = &AuthTunnel{
		authServer: authServer,
		apiServer:  apiServer,
	}
	tunnel.limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// apply functional options:
	for _, o := range opts {
		if err := o(tunnel); err != nil {
			return nil, err
		}
	}
	// create an SSH server and assign the tunnel to be it's "new SSH channel handler"
	tunnel.sshServer, err = sshutils.NewServer(
		addr,
		tunnel,
		hostSigners,
		sshutils.AuthMethods{
			Password:  tunnel.passwordAuth,
			PublicKey: tunnel.keyAuth,
		},
		sshutils.SetLimiter(tunnel.limiter),
	)
	if err != nil {
		return nil, err
	}
	tunnel.userCertChecker = ssh.CertChecker{IsAuthority: tunnel.isUserAuthority}
	tunnel.hostCertChecker = ssh.CertChecker{IsAuthority: tunnel.isHostAuthority}
	return tunnel, nil
}

func (s *AuthTunnel) Addr() string {
	return s.sshServer.Addr()
}

func (s *AuthTunnel) Start() error {
	return s.sshServer.Start()
}

func (s *AuthTunnel) Close() error {
	return s.sshServer.Close()
}

// HandleNewChan implements NewChanHandler interface: it gets called every time a new SSH
// connection is established
func (s *AuthTunnel) HandleNewChan(_ net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	log.Infof("[AUTH] new channel request: %v", nch.ChannelType())
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
			log.Errorf("[AUTH] failed to parse request data: %v, err: %v",
				string(nch.ExtraData()), err)
			nch.Reject(ssh.UnknownChannelType,
				"failed to parse direct-tcpip request")
		}
		sshCh, _, err := nch.Accept()
		if err != nil {
			log.Infof("[AUTH] could not accept channel (%s)", err)
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
			log.Infof("[AUTH] could not accept channel (%s)", err)
		}
		go s.handleWebAgentRequest(sconn, ch)
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf(
			"unknown channel type: %v", cht))
	}
}

// isHostAuthority is called during checking the client key, to see if the signing
// key is the real host CA authority key.
func (s *AuthTunnel) isHostAuthority(auth ssh.PublicKey) bool {
	key, err := s.authServer.GetCertAuthority(services.CertAuthID{DomainName: s.authServer.Hostname, Type: services.HostCA}, false)
	if err != nil {
		log.Errorf("failed to retrieve user authority key, err: %v", err)
		return false
	}
	checkers, err := key.Checkers()
	if err != nil {
		log.Errorf("failed to parse CA keys: %v", err)
		return false
	}
	for _, checker := range checkers {
		if sshutils.KeysEqual(checker, auth) {
			return true
		}
	}
	return false
}

// isUserAuthority is called during checking the client key, to see if the signing
// key is the real user CA authority key.
func (s *AuthTunnel) isUserAuthority(auth ssh.PublicKey) bool {
	keys, err := s.getTrustedCAKeys(services.UserCA)
	if err != nil {
		log.Errorf("failed to retrieve trusted keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}
	return false
}

func (s *AuthTunnel) getTrustedCAKeys(CertType services.CertAuthType) ([]ssh.PublicKey, error) {
	cas, err := s.authServer.GetCertAuthorities(CertType)
	if err != nil {
		return nil, err
	}
	out := []ssh.PublicKey{}
	for _, ca := range cas {
		checkers, err := ca.Checkers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, checkers...)
	}
	return out, nil
}

func (s *AuthTunnel) haveExt(sconn *ssh.ServerConn, ext ...string) bool {
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

func (s *AuthTunnel) handleWebAgentRequest(sconn *ssh.ServerConn, ch ssh.Channel) {
	defer ch.Close()

	if sconn.Permissions.Extensions[ExtRole] != string(teleport.RoleWeb) {
		log.Errorf("role %v doesn't have permission to request agent",
			sconn.Permissions.Extensions[ExtRole])
		return
	}

	log.Infof("handleWebAgentRequest start for %v", sconn.RemoteAddr())

	ws, err := s.authServer.GetWebSession(sconn.User(), sconn.Permissions.Extensions[ExtWebSession])
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
	newKeyAgent := agent.NewKeyring()
	if err := newKeyAgent.Add(addedKey); err != nil {
		log.Errorf("failed to add: %v", err)
		return
	}
	if err := agent.ServeAgent(newKeyAgent, ch); err != nil {
		log.Errorf("Serve agent err: %v", err)
	}
}

// handleDirectTCPIPRequest accepts an incoming SSH connection via TCP/IP and forwards
// it to the local auth server which listens on local UNIX pipe
func (s *AuthTunnel) handleDirectTCPIPRequest(sconn *ssh.ServerConn, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	defer sconn.Close()

	log.Debugf("[TUNNEL] Remote address: %v", sconn.RemoteAddr())

	role := teleport.Role(sconn.Permissions.Extensions[ExtRole])
	if err := role.Check(); err != nil {
		log.Errorf(err.Error())
		return
	}
	addr := &utils.NetAddr{Addr: "localhost", AddrNetwork: "tcp://"}
	conn := utils.NewPipeNetConn(ch, ch, ch, addr, addr)
	if err := s.apiServer.HandleConn(conn, role); err != nil {
		log.Errorf(err.Error())
		return
	}
}

func (s *AuthTunnel) keyAuth(
	conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cid := fmt.Sprintf(
		"conn(%v->%v, user=%v)", conn.RemoteAddr(),
		conn.LocalAddr(), conn.User())

	log.Infof("%v auth attempt with key %v", cid, key.Type())

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return nil, trace.Errorf("ERROR: Server doesn't support provided key type")
	}

	if cert.CertType == ssh.UserCert {
		_, err := s.userCertChecker.Authenticate(conn, key)
		if err != nil {
			log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
				conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
			return nil, err
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtHost: conn.User(),
				ExtRole: string(teleport.RoleHangoutRemoteUser),
			},
		}
		return perms, nil
	}

	err := s.hostCertChecker.CheckHostKey(conn.User(), conn.RemoteAddr(), key)
	if err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: failed auth user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, err
	}

	if err := s.hostCertChecker.CheckCert(conn.User(), cert); err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, trace.Wrap(err)
	}

	perms := &ssh.Permissions{
		Extensions: map[string]string{
			ExtHost: conn.User(),
			ExtRole: cert.Permissions.Extensions[utils.CertExtensionRole],
		},
	}

	return perms, nil
}

func (s *AuthTunnel) passwordAuth(
	conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	var ab *authBucket
	if err := json.Unmarshal(password, &ab); err != nil {
		return nil, err
	}
	log.Infof("got authentication attempt for user '%v' type '%v'", conn.User(), ab.Type)
	switch ab.Type {
	case AuthWebPassword:
		if err := s.authServer.CheckPassword(conn.User(), ab.Pass, ab.HotpToken); err != nil {
			log.Errorf("Password auth error: %v", err)
			return nil, trace.Wrap(err)
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebPassword: "<password>",
				ExtRole:        string(teleport.RoleUser),
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
				ExtRole:       string(teleport.RoleWeb),
			},
		}
		if _, err := s.authServer.GetWebSession(conn.User(), string(ab.Pass)); err != nil {
			return nil, trace.Errorf("session resume error: %v", trace.Wrap(err))
		}
		log.Infof("session authenticated user: '%v'", conn.User())
		return perms, nil
	case AuthToken:
		_, err := s.authServer.ValidateToken(string(ab.Pass), ab.User)
		if err != nil {
			log.Errorf("token validation error: %v", err)
			return nil, trace.Wrap(err, fmt.Sprintf("failed to validate user: %v", ab.User))
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtToken: string(password),
				ExtRole:  string(teleport.RoleProvisionToken),
			}}
		utils.Consolef(os.Stdout, "[AUTH] Successfully accepted token %v for %v", string(password), conn.User())
		return perms, nil
	case AuthSignupToken:
		_, _, err := s.authServer.GetSignupToken(string(ab.Pass))
		if err != nil {
			return nil, trace.Errorf("token validation error: %v", trace.Wrap(err))
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtToken: string(password),
				ExtRole:  string(teleport.RoleSignup),
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

func NewClientFromSSHClient(sshClient *ssh.Client) (*Client, error) {
	tr := &http.Transport{
		Dial: sshClient.Dial,
	}
	clt, err := NewClient(
		"http://stub:0",
		roundtrip.HTTPClient(&http.Client{
			Transport: tr,
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clt, nil
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
	ExtRole        = "role@teleport"

	AuthWebPassword = "password"
	AuthWebSession  = "session"
	AuthToken       = "provision-token"
	AuthSignupToken = "signup-token"
)

// AccessPointDialer dials to auth access point  remote HTTP api
type AccessPointDialer func() (net.Conn, error)
