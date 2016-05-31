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
	"sort"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// dialRetryInterval specifies the time interval tun client waits to retry
// dialing the same auth server
const dialRetryInterval = time.Duration(time.Millisecond * 50)

// AuthTunnel listens on TCP/IP socket and accepts SSH connections. It then establishes
// an SSH tunnell which HTTP requests travel over. In other words, the Auth Service API
// runs on HTTP-via-SSH-tunnel.
//
// Use auth.TunClient to connect to AuthTunnel
type AuthTunnel struct {
	// authServer implements the "beef" of the Auth service
	authServer *AuthServer
	config     *APIConfig

	// sshServer implements the nuts & bolts of serving an SSH connection
	// to create a tunnel
	sshServer       *sshutils.Server
	hostSigner      ssh.Signer
	hostCertChecker ssh.CertChecker
	userCertChecker ssh.CertChecker
	limiter         *limiter.Limiter
}

// TunClient is HTTP client that works over SSH tunnel
// This is done in order to authenticate various teleport roles
// using existing SSH certificate infrastructure
type TunClient struct {
	sync.Mutex

	// embed auth API HTTP client
	Client

	user string

	// static auth servers are CAs set via configuration (--auth flag) and
	// they do not change
	staticAuthServers []utils.NetAddr
	// discoveredAuthServers are CAs that get discovered at runtime
	discoveredAuthServers []utils.NetAddr
	authMethods           []ssh.AuthMethod
	refreshTicker         *time.Ticker
	closeC                chan struct{}
	closeOnce             sync.Once
	addrStorage           utils.AddrStorage
	// purpose is used for more informative logging. it explains _why_ this
	// client was created
	purpose string
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

// NewTunnel creates a new SSH tunnel server which is not started yet.
// This is how "site API" (aka "auth API") is served: by creating
// an "tunnel server" which serves HTTP via SSH.
func NewTunnel(addr utils.NetAddr,
	hostSigner ssh.Signer,
	apiConf *APIConfig,
	opts ...ServerOption) (tunnel *AuthTunnel, err error) {

	tunnel = &AuthTunnel{
		authServer: apiConf.AuthServer,
		config:     apiConf,
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
		[]ssh.Signer{hostSigner},
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
	if s != nil && s.sshServer != nil {
		return s.sshServer.Close()
	}
	return nil
}

// HandleNewChan implements NewChanHandler interface: it gets called every time a new SSH
// connection is established
func (s *AuthTunnel) HandleNewChan(_ net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	log.Infof("[AUTH] new channel request: %v", nch.ChannelType())
	cht := nch.ChannelType()
	switch cht {

	// New connection to the Auth API via SSH:
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
		go s.onAPIConnection(sconn, sshCh, req)

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
	cas, err := s.authServer.GetCertAuthorities(CertType, false)
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
	if err := agent.ServeAgent(newKeyAgent, ch); err != nil && err != io.EOF {
		log.Errorf("Serve agent err: %v", err)
	}
}

// onAPIConnection accepts an incoming SSH connection via TCP/IP and forwards
// it to the local auth server which listens on local UNIX pipe
//
func (s *AuthTunnel) onAPIConnection(sconn *ssh.ServerConn, sshChan ssh.Channel, req *sshutils.DirectTCPIPReq) {
	defer sconn.Close()

	// retreive the role from thsi connection's permissions (make sure it's a valid role)
	role := teleport.Role(sconn.Permissions.Extensions[ExtRole])
	if err := role.Check(); err != nil {
		log.Errorf(err.Error())
		return
	}

	api := NewAPIServer(s.config, role)
	socket := fakeSocket{
		closed:      make(chan int),
		connections: make(chan net.Conn),
	}

	go func() {
		connection := &FakeSSHConnection{
			remoteAddr: sconn.RemoteAddr(),
			sshChan:    sshChan,
			closed:     make(chan int),
		}
		// fakesocket.Accept() will pick it up:
		socket.connections <- connection

		// wait for the connection wrapper to close, so we'll close
		// the fake socket, causing http.Serve() below to stop
		<-connection.closed
		socket.Close()
	}()

	// serve HTTP API via this SSH connection until it gets closed:
	http.Serve(&socket, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// take SSH client name and pass it to HTTP API via HTTP Auth
		r.SetBasicAuth(sconn.User(), "")
		api.ServeHTTP(w, r)
	}))
}

func (s *AuthTunnel) keyAuth(
	conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

	log.Infof("keyAuth: %v->%v, user=%v", conn.RemoteAddr(), conn.LocalAddr(), conn.User())
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

	log.Infof("auth attempt: user '%v' type '%v'", conn.User(), ab.Type)

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
	// when a new server tries to use the auth API to register in the cluster,
	// it will use the token as a passowrd (happens only once during registration):
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
		utils.Consolef(os.Stdout, "[AUTH] Successfully accepted token for %v", conn.User())
		return perms, nil
	case AuthSignupToken:
		_, err := s.authServer.GetSignupToken(string(ab.Pass))
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

// NewTunClient returns an instance of new HTTP client to Auth server API
// exposed over SSH tunnel, so client  uses SSH credentials to dial and authenticate
//  - purpose is mostly for debuggin, like "web client" or "reverse tunnel client"
//  - authServers: list of auth servers in this cluster (they are supposed to be in sync)
//  - authMethods: how to authenticate (via cert, web passwowrd, etc)
//  - opts : functional arguments for further extending
func NewTunClient(purpose string,
	authServers []utils.NetAddr,
	user string,
	authMethods []ssh.AuthMethod,
	opts ...TunClientOption) (*TunClient, error) {
	if user == "" {
		return nil, trace.BadParameter("SSH connection requires a valid username")
	}
	tc := &TunClient{
		purpose:           purpose,
		user:              user,
		staticAuthServers: authServers,
		authMethods:       authMethods,
		closeC:            make(chan struct{}),
	}
	for _, o := range opts {
		o(tc)
	}
	log.Infof("newTunClient(%s) with auth: %v", purpose, authServers)

	clt, err := NewClient("http://stub:0", tc.Dial)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.Client = *clt

	// use local information about auth servers if it's available
	if tc.addrStorage != nil {
		cachedAuthServers, err := tc.addrStorage.GetAddresses()
		if err != nil {
			log.Errorf("unable to load from auth server cache: %v", err)
		} else {
			tc.setAuthServers(cachedAuthServers)
		}
	}
	return tc, nil
}

// Close releases all the resources allocated for this client
func (c *TunClient) Close() error {
	if c != nil {
		log.Infof("TunClient[%s].Close()", c.purpose)
		c.GetTransport().CloseIdleConnections()
		c.closeOnce.Do(func() {
			close(c.closeC)
		})
	}
	return nil
}

// GetDialer returns dialer that will connect to auth server API
func (c *TunClient) GetDialer() AccessPointDialer {
	addrNetwork := c.staticAuthServers[0].AddrNetwork
	const dialRetryTimes = 5

	return func() (conn net.Conn, err error) {
		for attempt := 0; attempt < dialRetryTimes; attempt++ {
			conn, err = c.Dial(addrNetwork, "accesspoint:0")
			if err == nil {
				return conn, nil
			}
			time.Sleep(dialRetryInterval * time.Duration(attempt))
		}
		log.Error(err)
		return nil, trace.Wrap(err)
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
		return nil, trace.ConnectionProblem(err, "failed to connect to remote API")
	}
	agentCloser := &tunAgent{client: client}
	agentCloser.Agent = agent.NewClient(ch)
	return agentCloser, nil
}

// Dial dials to Auth server's HTTP API over SSH tunnel
func (c *TunClient) Dial(network, address string) (net.Conn, error) {
	log.Infof("TunClient[%s].Dial()", c.purpose)
	client, err := c.getClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := client.Dial(network, address)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "can't connect to auth API")
	}
	// dialed & authenticated? lets start synchronizing the
	// list of auth servers:
	if c.refreshTicker == nil {
		c.refreshTicker = time.NewTicker(defaults.AuthServersRefreshPeriod)
		go c.authServersSyncLoop()
	}
	return &tunConn{client: client, Conn: conn}, nil
}

func (c *TunClient) fetchAndSync() error {
	authServers, err := c.fetchAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(authServers) == 0 {
		return trace.NotFound("no auth servers with remote IPs advertised")
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

// authServersSyncLoop continuously refreshes the list of available auth servers
// for this client
func (c *TunClient) authServersSyncLoop() {
	log.Infof("TunClient[%s]: authServersSyncLoop() started", c.purpose)
	defer c.refreshTicker.Stop()

	// initial fetch for quick start-ups
	c.fetchAndSync()
	for {
		select {
		// timer-based refresh:
		case <-c.refreshTicker.C:
			c.fetchAndSync()
		// received a signal to quit?
		case <-c.closeC:
			log.Infof("TunClient[%s]: authServersSyncLoop() exited", c.purpose)
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
		if !serverAddr.IsLocal() {
			authServers = append(authServers, *serverAddr)
		}
	}
	return authServers, nil
}

// getAuthServers returns a sorted list of auth servers
func (c *TunClient) getAuthServers() (out []utils.NetAddr) {
	c.Lock()
	defer c.Unlock()

	// return static auth servers followed by discovered ones. this guarantees
	// that the client will try statically configured ones first
	out = make([]utils.NetAddr, 0, len(c.staticAuthServers)+len(c.discoveredAuthServers))
	out = append(out, c.staticAuthServers...)
	out = append(out, c.discoveredAuthServers...)
	return out
}

// byAddress allows to sort slices of addresses by implementing sort.Interface
type byAddress []utils.NetAddr

func (a byAddress) Len() int           { return len(a) }
func (a byAddress) Less(i, j int) bool { return a[i].Addr < a[j].Addr }
func (a byAddress) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// setAuthServers assigns a new list of auth servers (CAs) who all together
// control the cluster (site)
//
// it keeps the list of auth servers sorted
func (c *TunClient) setAuthServers(servers []utils.NetAddr) {
	sort.Sort(byAddress(servers))

	c.Lock()
	defer c.Unlock()

	c.discoveredAuthServers = servers
}

// getClient returns an established SSH connection to one of the auth servers (CAs)
// for the cluster.
func (c *TunClient) getClient() (client *ssh.Client, err error) {
	// see if we have any auth servers online:
	authServers := c.getAuthServers()
	if len(authServers) == 0 {
		return nil, trace.Errorf("all auth servers are offline")
	}
	log.Infof("tunClient(%s).authServers: %v", c.purpose, authServers)

	// try to connect to the 1st one who will pick up:
	for _, authServer := range authServers {
		client, err = c.dialAuthServer(authServer)
		if err == nil {
			return client, nil
		}
	}
	return nil, trace.Wrap(err)
}

func (c *TunClient) dialAuthServer(authServer utils.NetAddr) (sshClient *ssh.Client, err error) {
	config := &ssh.ClientConfig{
		User: c.user,
		Auth: c.authMethods,
	}
	const dialRetryTimes = 5
	for attempt := 0; attempt < dialRetryTimes; attempt++ {
		log.Debugf("tunClient.Dial(to=%v, attempt=%d)", authServer.Addr, attempt+1)
		sshClient, err = ssh.Dial(authServer.AddrNetwork, authServer.Addr, config)
		// success -> get out of here
		if err == nil {
			break
		}
		if utils.IsHandshakeFailedError(err) {
			return nil, trace.AccessDenied("access denied to '%v': bad username or credentials", c.user)
		}
		time.Sleep(dialRetryInterval * time.Duration(attempt))
	}
	return sshClient, trace.Wrap(err)
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
