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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/mailgun/ttlmap"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/tstranex/u2f"
)

// dialRetryInterval specifies the time interval tun client waits to retry
// dialing the same auth server
const dialRetryInterval = 100 * time.Millisecond

// AuthTunnel listens on TCP/IP socket and accepts SSH connections. It then establishes
// an SSH tunnel which HTTP requests travel over. In other words, the Auth Service API
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
	// disableRefresh will disable the refresh ticker. Used when we only call a
	// single function with a TunClient (initial fetch of certs).
	disableRefresh bool
	closeC         chan struct{}
	closeOnce      sync.Once
	addrStorage    utils.AddrStorage
	// purpose is used for more informative logging. it explains _why_ this
	// client was created
	purpose string
	// throttler is used to throttle auth servers that we have failed to dial
	// for some period of time
	throttler *ttlmap.TtlMap
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
		teleport.ComponentAuth,
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
	log.Debugf("[AUTH] new channel request for %v from %v", nch.ChannelType(), sconn.RemoteAddr())
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

	case "session":
		nch.Reject(ssh.UnknownChannelType,
			"Cannot open new SSH session on the auth server. Are you connecting to the right port?")
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf(
			"unknown channel type: %v", cht))
	}
}

// isHostAuthority is called during checking the client key, to see if the signing
// key is the real host CA authority key.
func (s *AuthTunnel) isHostAuthority(auth ssh.PublicKey) bool {
	domainName, err := s.authServer.GetDomainName()
	if err != nil {
		return false
	}

	key, err := s.authServer.GetCertAuthority(services.CertAuthID{DomainName: domainName, Type: services.HostCA}, false)
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

// findUserAuthority finds matching user CA based on its public key
func (s *AuthTunnel) findUserAuthority(auth ssh.PublicKey) (services.CertAuthority, error) {
	cas, err := s.authServer.GetCertAuthorities(services.UserCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ca := range cas {
		checkers, err := ca.Checkers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(checker, auth) {
				return ca, nil
			}
		}
	}
	return nil, trace.NotFound("no matching certificate authority found")
}

// isUserAuthority is called during checking the client key, to see if the signing
// key is the real user CA authority key.
func (s *AuthTunnel) isUserAuthority(auth ssh.PublicKey) bool {
	_, err := s.findUserAuthority(auth)
	if err != nil {
		if !trace.IsNotFound(err) {
			// something bad happened, need to log
			log.Error(err)
		}
		return false
	}
	return true
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

	if sconn.Permissions.Extensions[ExtOrigin] != string(teleport.RoleWeb) {
		log.Errorf("role %v doesn't have permission to request agent",
			sconn.Permissions.Extensions[ExtOrigin])
		return
	}

	ws, err := s.authServer.GetWebSession(sconn.User(), sconn.Permissions.Extensions[ExtWebSession])
	if err != nil {
		log.Errorf("session error: %v", trace.DebugReport(err))
		return
	}

	priv, err := ssh.ParseRawPrivateKey(ws.GetPriv())
	if err != nil {
		log.Errorf("session error: %v", trace.DebugReport(err))
		return
	}

	pub, _, _, _, err := ssh.ParseAuthorizedKey(ws.GetPub())
	if err != nil {
		log.Errorf("session error: %v", trace.DebugReport(err))
		return
	}

	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		log.Errorf("session error, not a certificate: %T", pub)
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
		log.Errorf("failed to add: %v", trace.DebugReport(err))
		return
	}
	if err := agent.ServeAgent(newKeyAgent, ch); err != nil && err != io.EOF {
		log.Errorf("Serve agent err: %v", trace.DebugReport(err))
	}
}

// onAPIConnection accepts an incoming SSH connection via TCP/IP and forwards
// it to the local auth server which listens on local UNIX pipe
func (s *AuthTunnel) onAPIConnection(sconn *ssh.ServerConn, sshChan ssh.Channel, req *sshutils.DirectTCPIPReq) {
	defer sconn.Close()

	var user interface{} = nil
	if extRole, ok := sconn.Permissions.Extensions[ExtRole]; ok {
		// retreive the role from thsi connection's permissions (make sure it's a valid role)
		systemRole := teleport.Role(extRole)
		if err := systemRole.Check(); err != nil {
			log.Error(err.Error())
			return
		}
		user = teleport.BuiltinRole{Role: systemRole}
	} else if clusterName, ok := sconn.Permissions.Extensions[utils.CertTeleportUserCA]; ok {
		// we got user signed by remote certificate authority
		var remoteRoles []string
		var err error
		data, ok := sconn.Permissions.Extensions[teleport.CertExtensionTeleportRoles]
		if ok {
			remoteRoles, err = services.UnmarshalCertRoles(data)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}
		user = teleport.RemoteUser{
			ClusterName: clusterName,
			Username:    sconn.Permissions.Extensions[utils.CertTeleportUser],
			RemoteRoles: remoteRoles,
		}
	} else if teleportUser, ok := sconn.Permissions.Extensions[utils.CertTeleportUser]; ok {
		// we got user signed by local certificate authority
		user = teleport.LocalUser{
			Username: teleportUser,
		}
	} else {
		log.Errorf("expected %v or %v extensions for %v, found none in %v", ExtRole, utils.CertTeleportUser, sconn.User(), sconn.Permissions.Extensions)
		return
	}

	api := NewAPIServer(s.config)
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
		api.ServeHTTP(w, r.WithContext(context.WithValue(context.TODO(), teleport.ContextUser, user)))
	}))
}

func (s *AuthTunnel) keyAuth(
	conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

	log.Infof("[AUTH] keyAuth: %v->%v, user=%v", conn.RemoteAddr(), conn.LocalAddr(), conn.User())
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

	ca, err := s.findUserAuthority(cert.SignatureKey)
	if err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.authServer.GetDomainName()
	if err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, trace.Wrap(err)
	}

	// if this is local CA, we assume that local user exists
	if clusterName == ca.GetClusterName() {
		// we are not using cert extensions for User certificates because of OpenSSH bug
		// https://bugzilla.mindrot.org/show_bug.cgi?id=2387
		return &ssh.Permissions{
			Extensions: map[string]string{
				ExtHost:                conn.User(),
				utils.CertTeleportUser: cert.KeyId,
			},
		}, nil
	}
	// otherwise we return this as a remote CA
	permissions := &ssh.Permissions{
		Extensions: map[string]string{
			ExtHost:                  conn.User(),
			utils.CertTeleportUserCA: ca.GetID().DomainName,
			utils.CertTeleportUser:   cert.KeyId,
		},
	}
	extensions, ok := cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles]
	if ok {
		permissions.Extensions[teleport.CertExtensionTeleportRoles] = extensions
	}
	return permissions, nil
}

// passwordAuth is called to authenticate an incoming SSH connection
// to the auth server. Such connections are usually created using a
// TunClient object
//
func (s *AuthTunnel) passwordAuth(
	conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	var ab *authBucket
	if err := json.Unmarshal(password, &ab); err != nil {
		return nil, err
	}

	log.Infof("[AUTH] login attempt: user %q type %q", conn.User(), ab.Type)

	switch ab.Type {
	// user is trying to get in using their password+otp
	case AuthWebPassword:
		err := s.authServer.WithUserLock(conn.User(), func() error {
			return s.authServer.CheckPassword(conn.User(), ab.Pass, ab.OTPToken)
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebPassword:         "<password>",
				utils.CertTeleportUser: conn.User(),
			},
		}
		log.Infof("[AUTH] password+otp authenticated user: %q", conn.User())
		return perms, nil
	// user is trying to get in using their password only
	case AuthWebPasswordWithoutOTP:
		err := s.authServer.WithUserLock(conn.User(), func() error {
			return s.authServer.CheckPasswordWOToken(conn.User(), ab.Pass)
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebPassword:         "<password>",
				utils.CertTeleportUser: conn.User(),
			},
		}
		log.Infof("[AUTH] password authenticated user: %q", conn.User())
		return perms, nil
	// user is trying to get in using u2f (step 1)
	case AuthWebU2FSign:
		err := s.authServer.WithUserLock(conn.User(), func() error {
			return s.authServer.CheckPasswordWOToken(conn.User(), ab.Pass)
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// notice RoleNop here - it can literally call to nothing except one
		// method that everyone is authorized to do - request a sign in
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebPassword: "<password>",
				ExtRole:        string(teleport.RoleNop),
			},
		}
		log.Infof("[AUTH] u2f sign authenticated user: '%v'", conn.User())
		return perms, nil
	// user is trying to get in using u2f (step 2)
	case AuthWebU2F:
		err := s.authServer.WithUserLock(conn.User(), func() error {
			return s.authServer.CheckU2FSignResponse(conn.User(), &ab.U2FSignResponse)
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebU2F:              "<u2f-sign-response>",
				utils.CertTeleportUser: conn.User(),
			},
		}
		return perms, nil
	case AuthWebSession:
		// we use extra permissions mechanism to keep the connection data
		// after authorization, in this case the session
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebSession: string(ab.Pass),
				// Origin is used to mark this connection as
				// originated with web, as some features
				// like agent request are only available
				// for web users
				ExtOrigin:              string(teleport.RoleWeb),
				utils.CertTeleportUser: conn.User(),
			},
		}
		if _, err := s.authServer.GetWebSession(conn.User(), string(ab.Pass)); err != nil {
			return nil, trace.AccessDenied("session resume error: %v", err)
		}
		log.Infof("[AUTH] session authenticated user: '%v'", conn.User())
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
		log.Infof("[AUTH] Successfully accepted token for %v", conn.User())
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
		log.Infof("[AUTH] session authenticated prov. token: '%v'", conn.User())
		return perms, nil
	case AuthValidateTrustedCluster:
		err := s.authServer.validateTrustedClusterToken(string(ab.Pass))
		if err != nil {
			return nil, trace.AccessDenied("trusted cluster token validation error: %v", err)
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtToken: string(password),
				ExtRole:  string(teleport.RoleNop),
			}}
		log.Debugf("[AUTH] session authenticated validate trusted cluster; token: %q", conn.User())
		return perms, nil
	default:
		return nil, trace.AccessDenied("unsupported auth method: '%v'", ab.Type)
	}
}

// authBucket uses password to transport app-specific user name and
// auth-type in addition to the password to support auth
type authBucket struct {
	User            string           `json:"user"`
	Type            string           `json:"type"`
	Pass            []byte           `json:"pass"`
	HotpToken       string           `json:"hotpToken"` // HotpToken is deprecated, use OTPToken.
	OTPToken        string           `json:"otp_token"`
	U2FSignResponse u2f.SignResponse `json:"u2fSignResponse"`
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

func NewWebPasswordAuth(user string, password []byte, otpToken string) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type:      AuthWebPassword,
		User:      user,
		Pass:      password,
		HotpToken: otpToken, // HotpToken is deprecated, used OTPToken.
		OTPToken:  otpToken,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewWebPasswordWithoutOTPAuth(user string, password []byte) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthWebPasswordWithoutOTP,
		User: user,
		Pass: password,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

// NewWebPasswordU2FSignAuth is for getting a U2F sign challenge
func NewWebPasswordU2FSignAuth(user string, password []byte) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthWebU2FSign,
		User: user,
		Pass: password,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

// NewWebU2FSignResponseAuth is for signing in with a U2F sign response
func NewWebU2FSignResponseAuth(user string, u2fSignResponse *u2f.SignResponse) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type:            AuthWebU2F,
		User:            user,
		U2FSignResponse: *u2fSignResponse,
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

func NewValidateTrustedClusterAuth(token string) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthValidateTrustedCluster,
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

// TunDisableRefresh will disable refreshing the list of auth servers. This is
// required when requesting user certificates because we only allow a single
// HTTP request to be made over the tunnel. This is because each request does
// keyAuth, and for situations like password+otp where the OTP token is invalid
// after the first use, that means all other requests would fail.
func TunDisableRefresh() TunClientOption {
	return func(t *TunClient) {
		t.disableRefresh = true
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
	throttler, err := ttlmap.NewMap(16)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc := &TunClient{
		purpose:           purpose,
		user:              user,
		staticAuthServers: authServers,
		authMethods:       authMethods,
		closeC:            make(chan struct{}),
		throttler:         throttler,
	}
	for _, o := range opts {
		o(tc)
	}
	log.Debugf("NewTunClient(%v) with auth: %v", purpose, authServers)

	clt, err := NewClient("http://stub:0", tc.Dial)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.Client = *clt

	// use local information about auth servers if it's available
	if tc.addrStorage != nil {
		cachedAuthServers, err := tc.addrStorage.GetAddresses()
		if err != nil {
			if !trace.IsNotFound(err) {
				log.Warnf("unable to load the auth server cache: %s", err.Error())
			}
		} else {
			tc.setAuthServers(cachedAuthServers)
		}
	}
	return tc, nil
}

func (c *TunClient) throttleAuthServer(addr string) {
	c.Lock()
	defer c.Unlock()
	c.throttler.Set(addr, "ok", int(defaults.DefaultThrottleTimeout/time.Second))
}

func (c *TunClient) isAuthServerThrottled(addr string) bool {
	c.Lock()
	defer c.Unlock()
	_, ok := c.throttler.Get(addr)
	return ok
}

func (c *TunClient) String() string {
	return fmt.Sprintf("TunClient[%s]", c.purpose)
}

// Close releases all the resources allocated for this client
func (c *TunClient) Close() error {
	if c != nil {
		log.Debugf("%v.Close()", c)
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
	const dialRetryTimes = 3

	return func() (conn net.Conn, err error) {
		for attempt := 0; attempt < dialRetryTimes; attempt++ {
			conn, err = c.Dial(addrNetwork, "accesspoint:0")
			if err == nil {
				return conn, nil
			}
			time.Sleep(4 * time.Duration(attempt) * dialRetryInterval)
		}
		log.Errorf("%v: ", err)
		return nil, trace.Wrap(err)
	}
}

// GetAgent creates an SSH key agent (similar object to what CLI uses), this
// key agent fetches user keys directly from the auth server using a custom channel
// created via "ReqWebSessionAgent" reguest
func (c *TunClient) GetAgent() (AgentCloser, error) {
	client, err := c.getClient() // we need an established connection first
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// create a special SSH channel into the auth server, which will be used to
	// serve user keys to a web-based terminal client (which will be using the
	// returned SSH agent)
	ch, _, err := client.OpenChannel(ReqWebSessionAgent, nil)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "failed to connect to remote API")
	}
	ta := &tunAgent{client: client}
	ta.Agent = agent.NewClient(ch)
	return ta, nil
}

// Dial dials to Auth server's HTTP API over SSH tunnel.
func (c *TunClient) Dial(network, address string) (net.Conn, error) {
	log.Debugf("TunClient[%s].Dial()", c.purpose)

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
	if c.disableRefresh == false {
		if c.refreshTicker == nil {
			c.refreshTicker = time.NewTicker(defaults.AuthServersRefreshPeriod)
			go c.authServersSyncLoop()
		}
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
	log.Debugf("%v: authServersSyncLoop() started", c)
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
			log.Debugf("%v: authServersSyncLoop() exited", c)
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
		serverAddr, err := utils.ParseAddr(server.GetAddr())
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
		return nil, trace.ConnectionProblem(nil, "all auth servers are offline")
	}
	log.Debugf("%v.authServers: %v", c, authServers)

	// try to connect to the 1st one who will pick up:
	for _, authServer := range authServers {
		if c.isAuthServerThrottled(authServer.String()) {
			continue
		}
		client, err = c.dialAuthServer(authServer)
		if err == nil {
			return client, nil
		}
		// if it's an auth failure, fail right away because all auth servers are backed
		// by the same data store and result does not depend on which auth server you hit.
		if trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		log.Errorf("%v.getClient() error while connecting to auth server %v: %v: throttling", c, authServer, err)
		c.throttleAuthServer(authServer.String())
	}
	return nil, trace.ConnectionProblem(nil, "all auth servers are offline")
}

func (c *TunClient) dialAuthServer(authServer utils.NetAddr) (sshClient *ssh.Client, err error) {
	config := &ssh.ClientConfig{
		User:    c.user,
		Auth:    c.authMethods,
		Timeout: defaults.DefaultDialTimeout,
	}
	const dialRetryTimes = 1
	for attempt := 0; attempt < dialRetryTimes; attempt++ {
		log.Debugf("%v.Dial(to=%v, attempt=%d)", c, authServer.Addr, attempt+1)
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

// tunAgent is an SSH key agent (as defined by golang lib) implemented on top
// of the tun client. It allows secure retreival of user credentials from the
// auth server API.
type tunAgent struct {
	agent.Agent
	client *ssh.Client
}

func (ta *tunAgent) Close() error {
	log.Debugf("tunAgent.Close")
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
	ExtWebU2F      = "web-u2f@teleport"
	ExtToken       = "provision@teleport"
	ExtHost        = "host@teleport"
	ExtRole        = "role@teleport"
	ExtOrigin      = "origin@teleport"

	AuthWebPassword            = "password"
	AuthWebPasswordWithoutOTP  = "password-without-otp"
	AuthWebU2FSign             = "u2f-sign"
	AuthWebU2F                 = "u2f"
	AuthWebSession             = "session"
	AuthToken                  = "provision-token"
	AuthSignupToken            = "signup-token"
	AuthValidateTrustedCluster = "trusted-cluster"
)

// AccessPointDialer dials to auth access point  remote HTTP api
type AccessPointDialer func() (net.Conn, error)
