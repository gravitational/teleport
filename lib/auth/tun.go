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
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
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

// ServerOption is the functional argument passed to the server
type ServerOption func(s *AuthTunnel) error

// SetLimiter sets rate and connection limiter for auth tunnel server
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
			return
		}
		sshCh, _, err := nch.Accept()
		if err != nil {
			log.Infof("[AUTH] could not accept channel (%s)", err)
			return
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
			return
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
	key, err := s.authServer.GetCertAuthority(services.CertAuthID{DomainName: s.authServer.DomainName, Type: services.HostCA}, false)
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
func (s *AuthTunnel) handleDirectTCPIPRequest(sconn *ssh.ServerConn, sshChannel ssh.Channel, req *sshutils.DirectTCPIPReq) {
	defer sconn.Close()

	// retreive the role from thsi connection's permissions (make sure it's a valid role)
	role := teleport.Role(sconn.Permissions.Extensions[ExtRole])
	if err := role.Check(); err != nil {
		log.Errorf(err.Error())
		return
	}

	// Forward this new SSH channel to API-with-roles server. It will try to proxy this
	// connection to the API service mapped to this role:
	if err := s.apiServer.HandleNewChannel(sconn.RemoteAddr(), sshChannel, role); err != nil {
		log.Error(err)
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

	if cert.CertType == ssh.HostCert {
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
	// we are assuming that this is a user cert
	if err := s.userCertChecker.CheckCert(conn.User(), cert); err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, trace.Wrap(err)
	}
	// we are not using cert extensions for User certificates because of OpenSSH bug
	// https://bugzilla.mindrot.org/show_bug.cgi?id=2387
	perms := &ssh.Permissions{
		Extensions: map[string]string{
			ExtHost: conn.User(),
			ExtRole: string(teleport.RoleUser),
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
			log.Warningf("password auth error: %#v", err)
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
		_, err := s.authServer.ValidateToken(string(ab.Pass))
		if err != nil {
			log.Errorf("token validation error: %v", err)
			return nil, trace.Wrap(err, fmt.Sprintf("invalid token for: %v", ab.User))
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

// TunClientOption is functional option for tunnel client
type TunClientOption func(t *TunClient)

// TunClientStorage allows tun client to set local presence service
// that it will use to sync up the latest information about auth servers
func TunClientStorage(storage utils.AddrStorage) TunClientOption {
	return func(t *TunClient) {
		t.addrStorage = storage
	}
}

// TunClient is HTTP client that works over SSH tunnel
// This is done in order to authenticate various teleport roles
// using existing SSH certificate infrastructure
type TunClient struct {
	sync.Mutex
	Client
	user          string
	authServers   []utils.NetAddr
	authMethods   []ssh.AuthMethod
	refreshTicker *time.Ticker
	closeC        chan struct{}
	closeOnce     sync.Once
	tr            *http.Transport
	addrStorage   utils.AddrStorage
}

// NewTunClient returns an instance of new HTTP client to Auth server API
// exposed over SSH tunnel, so client  uses SSH credentials to dial and authenticate
func NewTunClient(authServers []utils.NetAddr, user string, authMethods []ssh.AuthMethod, opts ...TunClientOption) (*TunClient, error) {
	if user == "" {
		return nil, trace.Wrap(teleport.BadParameter("user", "SSH connection requires a valid username"))
	}
	tc := &TunClient{
		user:          user,
		authServers:   authServers,
		authMethods:   authMethods,
		refreshTicker: time.NewTicker(defaults.AuthServersRefreshPeriod),
		closeC:        make(chan struct{}),
	}
	for _, o := range opts {
		o(tc)
	}
	tr := &http.Transport{
		Dial: tc.Dial,
	}
	clt, err := NewClient(
		"http://stub:0",
		roundtrip.HTTPClient(&http.Client{
			Transport: tr,
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.Client = *clt
	tc.tr = tr

	// use local information about auth servers if it's available
	if tc.addrStorage != nil {

		authServers, err := tc.addrStorage.GetAddresses()
		if err != nil {
			if !teleport.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			log.Infof("local storage is provided, not initialized")
		} else {
			log.Infof("using auth servers from local storage: %v", authServers)
			tc.authServers = authServers
		}
	}
	go tc.syncAuthServers()
	return tc, nil
}

// Close releases all the resources allocated for this client
func (c *TunClient) Close() error {
	c.tr.CloseIdleConnections()
	c.refreshTicker.Stop()
	c.closeOnce.Do(func() {
		close(c.closeC)
	})
	return nil
}

// GetDialer returns dialer that will connect to auth server API
func (c *TunClient) GetDialer() AccessPointDialer {
	return func() (net.Conn, error) {
		return c.Dial(c.authServers[0].AddrNetwork, "accesspoint:0")
	}
}

// GetAgent returns SSH agent that uses ReqWebSessionAgent Auth server extension
func (c *TunClient) GetAgent() (AgentCloser, error) {
	client, err := c.getClient() // we need an established connection first
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ch, _, err := client.OpenChannel(ReqWebSessionAgent, nil)
	if err != nil {
		return nil, trace.Wrap(
			teleport.ConnectionProblem(
				"failed to connect to remote API", err))
	}
	agentCloser := &tunAgent{client: client}
	agentCloser.Agent = agent.NewClient(ch)
	return agentCloser, nil
}

// Dial dials to Auth server's HTTP API over SSH tunnel
func (c *TunClient) Dial(network, address string) (net.Conn, error) {
	log.Debugf("TunDialer.Dial(%v, %v)", network, address)
	client, err := c.getClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := client.Dial(network, address)
	if err != nil {
		return nil, trace.Wrap(
			teleport.ConnectionProblem("failed to connect to remote API", err))
	}
	tc := &tunConn{client: client}
	tc.Conn = conn
	return tc, nil
}

func (c *TunClient) fetchAndSync() error {
	authServers, err := c.fetchAuthServers()
	if err != nil {
		log.Infof("failed to fetch auth servers")
		return trace.Wrap(err)
	}
	if len(authServers) == 0 {
		log.Warningf("no auth servers received")
		return trace.Wrap(teleport.NotFound("no auth servers"))
	}
	// set runtime information about auth servers
	c.setAuthServers(authServers)
	// populate local storage if it is supplied
	if c.addrStorage != nil {
		if err := c.addrStorage.SetAddresses(authServers); err != nil {
			return trace.Wrap(err, "failed to set local storage addresses")
		}
	}
	return nil
}

func (c *TunClient) syncAuthServers() {
	for {
		select {
		case <-c.refreshTicker.C:
			err := c.fetchAndSync()
			if err != nil {
				log.Infof("failed to fetch and sync servers: %v", err)
				continue
			}
		case <-c.closeC:
			return
		}
	}
}

func (c *TunClient) fetchAuthServers() ([]utils.NetAddr, error) {
	servers, err := c.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authServers := make([]utils.NetAddr, 0, len(servers))
	for _, server := range servers {
		serverAddr, err := utils.ParseAddr(server.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		authServers = append(authServers, *serverAddr)
	}
	return authServers, nil
}

func (c *TunClient) getAuthServers() []utils.NetAddr {
	c.Lock()
	defer c.Unlock()

	out := make([]utils.NetAddr, len(c.authServers))
	for i := range c.authServers {
		out[i] = c.authServers[i]
	}
	return out
}

func (c *TunClient) setAuthServers(servers []utils.NetAddr) {
	c.Lock()
	defer c.Unlock()

	log.Infof("setAuthServers(%#v)", servers)
	c.authServers = servers
}

func (c *TunClient) getClient() (*ssh.Client, error) {
	var client *ssh.Client
	var err error
	for _, authServer := range c.getAuthServers() {
		client, err = c.dialAuthServer(authServer)
		if err == nil {
			return client, nil
		}
	}
	return nil, trace.Wrap(err)
}

func (c *TunClient) dialAuthServer(authServer utils.NetAddr) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: c.user,
		Auth: c.authMethods,
	}
	client, err := ssh.Dial(authServer.AddrNetwork, authServer.Addr, config)
	log.Debugf("TunDialer.getClient(%v)", authServer.String())
	if err != nil {
		log.Infof("TunDialer could not ssh.Dial: %v", err)
		if utils.IsHandshakeFailedError(err) {
			return nil, teleport.AccessDenied(
				fmt.Sprintf("access denied to '%v': bad username or credentials", c.user))
		}
		return nil, trace.Wrap(teleport.ConvertSystemError(err))
	}
	return client, nil
}

type AgentCloser interface {
	io.Closer
	agent.Agent
}

type tunAgent struct {
	agent.Agent
	client *ssh.Client
}

func (ta *tunAgent) Close() error {
	log.Infof("tunAgent.Close")
	return ta.client.Close()
}

const (
	// DialerRetryAttempts is the amount of attempts for dialer to try and
	// connect to the remote destination
	DialerRetryAttempts = 3
	// DialerPeriodBetweenAttempts is the period between retry attempts
	DialerPeriodBetweenAttempts = time.Second
)

type tunConn struct {
	net.Conn
	client *ssh.Client
}

func (c *tunConn) Close() error {
	err := c.Conn.Close()
	err = c.client.Close()
	return trace.Wrap(err)
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
