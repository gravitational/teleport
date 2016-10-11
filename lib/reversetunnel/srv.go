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

package reversetunnel

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/mailgun/oxy/forward"
	"golang.org/x/crypto/ssh"
)

// RemoteSite represents remote teleport site that can be accessed via
// teleport tunnel or directly by proxy
type RemoteSite interface {
	// ConnectToServer allows to SSH into remote teleport server
	ConnectToServer(addr, user string, auth []ssh.AuthMethod) (*ssh.Client, error)
	// DialServer dials teleport server and returns connection
	DialServer(addr string) (net.Conn, error)
	// Dial dials any address withing reach of remote site's servers
	Dial(network, addr string) (net.Conn, error)
	// GetLastConnected returns last time the remote site was seen connected
	GetLastConnected() time.Time
	// GetName returns site name (identified by authority domain's name)
	GetName() string
	// GetStatus returns status of this site (either offline or connected)
	GetStatus() string
	// GetClient returns client connected to remote auth server
	GetClient() (auth.ClientI, error)
}

// Server represents server connected to one or many remote sites
type Server interface {
	// GetSites returns a list of connected remote sites
	GetSites() []RemoteSite
	// GetSite returns remote site this node belongs to
	GetSite(domainName string) (RemoteSite, error)
	// RemoveSite removes the site with the specified name from the list of connected sites
	RemoveSite(domainName string) error
	// Start starts server
	Start() error
	// CLose closes server's socket
	Close() error
	// Wait waits for server to close all outstanding operations
	Wait()
}

type server struct {
	sync.RWMutex

	localAuth       auth.ClientI
	hostCertChecker ssh.CertChecker
	userCertChecker ssh.CertChecker
	l               net.Listener
	srv             *sshutils.Server
	timeout         time.Duration
	limiter         *limiter.Limiter

	tunnelSites []*tunnelSite
	directSites []*directSite
}

// ServerOption sets reverse tunnel server options
type ServerOption func(s *server)

// ServerTimeout sets server timeout for read and write operations
func ServerTimeout(duration time.Duration) ServerOption {
	return func(s *server) {
		s.timeout = duration
	}
}

// DirectSite instructs server to proxy access to this site not using
// reverse tunnel
func DirectSite(domainName string, clt auth.ClientI) ServerOption {
	return func(s *server) {
		s.directSites = append(s.directSites, newDirectSite(domainName, clt))
	}
}

func SetLimiter(limiter *limiter.Limiter) ServerOption {
	return func(s *server) {
		s.limiter = limiter
	}
}

// NewServer returns an unstarted server
func NewServer(addr utils.NetAddr, hostSigners []ssh.Signer,
	clt auth.ClientI, opts ...ServerOption) (Server, error) {

	srv := &server{
		directSites: []*directSite{},
		tunnelSites: []*tunnelSite{},
		localAuth:   clt,
	}
	var err error
	srv.limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, o := range opts {
		o(srv)
	}
	if srv.timeout == 0 {
		srv.timeout = teleport.DefaultTimeout
	}

	s, err := sshutils.NewServer(
		teleport.ComponentReverseTunnel,
		addr,
		srv,
		hostSigners,
		sshutils.AuthMethods{
			PublicKey: srv.keyAuth,
		},
		sshutils.SetLimiter(srv.limiter),
	)
	if err != nil {
		return nil, err
	}
	srv.hostCertChecker = ssh.CertChecker{IsAuthority: srv.isHostAuthority}
	srv.userCertChecker = ssh.CertChecker{IsAuthority: srv.isUserAuthority}
	srv.srv = s
	return srv, nil
}

func (s *server) Wait() {
	s.srv.Wait()
}

func (s *server) Addr() string {
	return s.srv.Addr()
}

func (s *server) Start() error {
	return s.srv.Start()
}

func (s *server) Close() error {
	return s.srv.Close()
}

func (s *server) HandleNewChan(conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	if nch.ChannelType() != chanHeartbeat {
		msg := fmt.Sprintf("reversetunnel received unknown channel request %v from %v",
			nch.ChannelType(), sconn)
		log.Warningf(msg)
		nch.Reject(ssh.ConnectionFailed, msg)
		return
	}
	log.Debugf("[TUNNEL] new tunnel from %s", sconn.RemoteAddr())
	if sconn.Permissions.Extensions[extCertType] != extCertTypeHost {
		log.Error(trace.BadParameter("can't retrieve certificate type in certType"))
		return
	}
	// add the incoming site (cluster) to the list of active connections:
	site, remoteConn, err := s.upsertSite(conn, sconn)
	if err != nil {
		log.Error(trace.Wrap(err))
		nch.Reject(ssh.ConnectionFailed, "failed to accept incoming cluster connection")
		return
	}
	// accept the request and start the heartbeat on it:
	ch, req, err := nch.Accept()
	if err != nil {
		log.Error(trace.Wrap(err))
		sconn.Close()
		return
	}
	go site.handleHeartbeat(remoteConn, ch, req)
}

// isHostAuthority is called during checking the client key, to see if the signing
// key is the real host CA authority key.
func (s *server) isHostAuthority(auth ssh.PublicKey) bool {
	keys, err := s.getTrustedCAKeys(services.HostCA)
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

// isUserAuthority is called during checking the client key, to see if the signing
// key is the real user CA authority key.
func (s *server) isUserAuthority(auth ssh.PublicKey) bool {
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

func (s *server) getTrustedCAKeys(CertType services.CertAuthType) ([]ssh.PublicKey, error) {
	cas, err := s.localAuth.GetCertAuthorities(CertType, false)
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

func (s *server) checkTrustedKey(CertType services.CertAuthType, domainName string, key ssh.PublicKey) error {
	cas, err := s.localAuth.GetCertAuthorities(CertType, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, ca := range cas {
		if ca.DomainName != domainName {
			continue
		}
		checkers, err := ca.Checkers()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(key, checker) {
				return nil
			}
		}
	}
	return trace.NotFound("authority domain %v not found or has no mathching keys", domainName)
}

func (s *server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	logger := log.WithFields(log.Fields{
		"remote": conn.RemoteAddr(),
		"user":   conn.User(),
	})

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		logger.Warningf("server doesn't support provided key type")
		return nil, trace.BadParameter("server doesn't support provided key type")
	}

	switch cert.CertType {
	case ssh.HostCert:
		authDomain, ok := cert.Extensions[utils.CertExtensionAuthority]
		if !ok || authDomain == "" {
			err := trace.BadParameter("missing authority domainName parameter")
			logger.Warningf("failed authenticate host, err: %v", err)
			return nil, err
		}
		err := s.hostCertChecker.CheckHostKey(conn.User(), conn.RemoteAddr(), key)
		if err != nil {
			logger.Warningf("failed authenticate host, err: %v", err)
			return nil, trace.Wrap(err)
		}
		if err := s.hostCertChecker.CheckCert(conn.User(), cert); err != nil {
			logger.Warningf("failed to authenticate host err: %v", err)
			return nil, trace.Wrap(err)
		}
		// this fixes possible injection attack
		// when we have 2 trusted remote sites, and one can simply
		// pose as another. so we have to check that authority
		// matches by some other way (in absence of x509 chains)
		if err := s.checkTrustedKey(services.HostCA, authDomain, cert.SignatureKey); err != nil {
			logger.Warningf("this claims to be signed as authDomain %v, but no matching signing keys found")
			return nil, trace.Wrap(err)
		}
		return &ssh.Permissions{
			Extensions: map[string]string{
				extHost:      conn.User(),
				extCertType:  extCertTypeHost,
				extAuthority: authDomain,
			},
		}, nil
	case ssh.UserCert:
		_, err := s.userCertChecker.Authenticate(conn, key)
		if err != nil {
			logger.Warningf("failed to authenticate user, err: %v", err)
			return nil, err
		}

		if err := s.userCertChecker.CheckCert(conn.User(), cert); err != nil {
			logger.Warningf("failed to authenticate user err: %v", err)
			return nil, trace.Wrap(err)
		}

		return &ssh.Permissions{
			Extensions: map[string]string{
				extHost:     conn.User(),
				extCertType: extCertTypeUser,
			},
		}, nil
	default:
		return nil, trace.BadParameter("unsupported cert type: %v", cert.CertType)
	}
}

func (s *server) upsertSite(conn net.Conn, sshConn *ssh.ServerConn) (*tunnelSite, *remoteConn, error) {
	domainName := sshConn.Permissions.Extensions[extAuthority]
	if strings.TrimSpace(domainName) == "" {
		return nil, nil, trace.BadParameter("Cannot create reverse tunnel: empty domain name")
	}

	s.Lock()
	defer s.Unlock()

	var site *tunnelSite
	for _, st := range s.tunnelSites {
		if st.domainName == domainName {
			site = st
			break
		}
	}
	var err error
	var remoteConn *remoteConn
	if site != nil {
		if remoteConn, err = site.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		site, err = newRemoteSite(s, domainName)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if remoteConn, err = site.addConn(conn, sshConn); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		s.tunnelSites = append(s.tunnelSites, site)
	}
	log.Infof("[TUNNEL] site %v connected from %v. sites: %d",
		domainName, conn.RemoteAddr(), len(s.tunnelSites))
	return site, remoteConn, nil
}

func (s *server) GetSites() []RemoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]RemoteSite, 0, len(s.tunnelSites)+len(s.directSites))
	for i := range s.directSites {
		out = append(out, s.directSites[i])
	}
	for i := range s.tunnelSites {
		out = append(out, s.tunnelSites[i])
	}
	return out
}

func (s *server) GetSite(domainName string) (RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	for i := range s.tunnelSites {
		if s.tunnelSites[i].domainName == domainName {
			return s.tunnelSites[i], nil
		}
	}
	for i := range s.directSites {
		if s.directSites[i].domainName == domainName {
			return s.directSites[i], nil
		}
	}
	return nil, trace.NotFound("site '%v' not found", domainName)
}

func (s *server) RemoveSite(domainName string) error {
	s.Lock()
	defer s.Unlock()
	for i := range s.tunnelSites {
		if s.tunnelSites[i].domainName == domainName {
			s.tunnelSites = append(s.tunnelSites[:i], s.tunnelSites[i+1:]...)
			return nil
		}
	}
	for i := range s.directSites {
		if s.directSites[i].domainName == domainName {
			s.directSites = append(s.directSites[:i], s.directSites[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("site '%v' not found", domainName)
}

type remoteConn struct {
	sshConn ssh.Conn
	conn    net.Conn
	invalid int32
	log     *log.Entry
	counter int32
}

func (rc *remoteConn) String() string {
	return fmt.Sprintf("remoteConn(remoteAddr=%v)", rc.conn.RemoteAddr())
}

func (rc *remoteConn) Close() error {
	return rc.sshConn.Close()
}

func (rc *remoteConn) markInvalid(err error) {
	atomic.StoreInt32(&rc.invalid, 1)
}

func (rc *remoteConn) isInvalid() bool {
	return atomic.LoadInt32(&rc.invalid) == 1
}

func newRemoteConn(log *log.Entry, conn net.Conn, sshConn ssh.Conn) (*remoteConn, error) {
	return &remoteConn{
		sshConn: sshConn,
		conn:    conn,
		log:     log,
	}, nil
}

func newRemoteSite(srv *server, domainName string) (*tunnelSite, error) {
	remoteSite := &tunnelSite{
		srv:        srv,
		domainName: domainName,
		log: log.WithFields(log.Fields{
			teleport.Component: teleport.ComponentReverseTunnel,
			teleport.ComponentFields: map[string]string{
				"domainName": domainName,
				"side":       "server",
			},
		}),
	}
	// transport uses connection do dial out to the remote address
	remoteSite.transport = &http.Transport{
		Dial: remoteSite.dialAccessPoint,
	}
	clt, err := auth.NewClient("http://stub:0", remoteSite.dialAccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite.clt = clt
	return remoteSite, nil
}

// tunnelSite is a site accessed via SSH reverse tunnel that established
// between proxy and remote site
type tunnelSite struct {
	sync.Mutex

	log         *log.Entry
	domainName  string
	connections []*remoteConn
	lastUsed    int
	lastActive  time.Time
	srv         *server

	transport *http.Transport
	clt       *auth.Client
}

func (s *tunnelSite) GetClient() (auth.ClientI, error) {
	return s.clt, nil
}

func (s *tunnelSite) String() string {
	return fmt.Sprintf("remoteSite(%v)", s.domainName)
}

func (s *tunnelSite) nextConn() (*remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	for {
		if len(s.connections) == 0 {
			return nil, trace.NotFound("no active tunnels to cluster %v", s.GetName())
		}
		s.lastUsed = (s.lastUsed + 1) % len(s.connections)
		remoteConn := s.connections[s.lastUsed]
		if !remoteConn.isInvalid() {
			return remoteConn, nil
		}
		s.connections = append(s.connections[:s.lastUsed], s.connections[s.lastUsed+1:]...)
		s.lastUsed = 0
		go remoteConn.Close()
	}
}

func (s *tunnelSite) addConn(conn net.Conn, sshConn ssh.Conn) (*remoteConn, error) {
	remoteConn, err := newRemoteConn(s.log, conn, sshConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.Lock()
	defer s.Unlock()
	s.connections = append(s.connections, remoteConn)
	s.lastUsed = 0
	return remoteConn, nil
}

func (s *tunnelSite) GetStatus() string {
	s.Lock()
	defer s.Unlock()
	diff := time.Now().Sub(s.lastActive)
	if diff > 2*defaults.ReverseTunnelAgentHeartbeatPeriod {
		return RemoteSiteStatusOffline
	}
	return RemoteSiteStatusOnline
}

func (s *tunnelSite) setLastActive(t time.Time) {
	s.Lock()
	defer s.Unlock()
	s.lastActive = t
}

func (s *tunnelSite) handleHeartbeat(conn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	go func() {
		for {
			select {
			case req := <-reqC:
				if req == nil {
					s.log.Infof("[TUNNEL] site disconnected: %v", s.domainName)
					conn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
					return
				}
				log.Debugf("[TUNNEL] ping from \"%s\" %s", s.domainName, conn.conn.RemoteAddr())
				s.setLastActive(time.Now())
			case <-time.After(3 * defaults.ReverseTunnelAgentHeartbeatPeriod):
				conn.markInvalid(trace.ConnectionProblem(nil, "agent missed 3 heartbeats"))
				conn.sshConn.Close()
			}
		}
	}()
}

func (s *tunnelSite) GetName() string {
	return s.domainName
}

func (s *tunnelSite) GetLastConnected() time.Time {
	s.Lock()
	defer s.Unlock()
	return s.lastActive
}

func (s *tunnelSite) timeout() time.Duration {
	return s.srv.timeout
}

func (s *tunnelSite) ConnectToServer(server, user string, auth []ssh.AuthMethod) (*ssh.Client, error) {
	s.log.Infof("[TUNNEL] connect(server=%v, user=%v)", server, user)
	remoteConn, err := s.nextConn()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ch, _, err := remoteConn.sshConn.OpenChannel(chanTransport, nil)
	if err != nil {
		remoteConn.markInvalid(err)
		return nil, trace.Wrap(err)
	}
	// ask remote channel to dial
	dialed, err := ch.SendRequest(chanTransportDialReq, true, []byte(server))
	if err != nil {
		remoteConn.markInvalid(err)
		return nil, trace.Wrap(err)
	}
	if !dialed {
		return nil, trace.Errorf("remote server %v is not available", server)
	}
	transportConn := utils.NewChConn(remoteConn.sshConn, ch)
	conn, chans, reqs, err := ssh.NewClientConn(
		transportConn, server,
		&ssh.ClientConfig{
			User: user,
			Auth: auth,
		})
	if err != nil {
		s.log.Errorf("[TUNNEL] connect(server=%v): %v", server, err)
		return nil, trace.Wrap(err)
	}
	return ssh.NewClient(conn, chans, reqs), nil
}

// dialAccessPoint establishes a connection from the proxy (reverse tunnel server)
// back into the client using previously established tunnel.
func (s *tunnelSite) dialAccessPoint(network, addr string) (net.Conn, error) {
	s.log.Infof("[TUNNEL] dial to site '%s'", s.GetName())

	try := func() (net.Conn, error) {
		remoteConn, err := s.nextConn()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ch, _, err := remoteConn.sshConn.OpenChannel(chanAccessPoint, nil)
		if err != nil {
			remoteConn.markInvalid(err)
			s.log.Errorf("[TUNNEL] disconnecting site '%s' on %v. Err: %v",
				s.GetName(),
				remoteConn.conn.RemoteAddr(),
				err)
			return nil, trace.Wrap(err)
		}
		s.log.Infof("[TUNNEL] success dialing to site '%s'", s.GetName())
		return utils.NewChConn(remoteConn.sshConn, ch), nil
	}

	for {
		conn, err := try()
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		return conn, nil
	}
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (s *tunnelSite) Dial(network string, addr string) (net.Conn, error) {
	s.log.Infof("[TUNNEL] dialing %v@%v through the tunnel", addr, s.domainName)

	try := func() (net.Conn, error) {
		remoteConn, err := s.nextConn()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var ch ssh.Channel
		ch, _, err = remoteConn.sshConn.OpenChannel(chanTransport, nil)
		if err != nil {
			remoteConn.markInvalid(err)
			return nil, trace.Wrap(err)
		}
		// we're creating a new SSH connection inside reverse SSH connection
		// as a new SSH channel:
		var dialed bool
		dialed, err = ch.SendRequest(chanTransportDialReq, true, []byte(addr))
		if err != nil {
			remoteConn.markInvalid(err)
			return nil, trace.Wrap(err)
		}
		if !dialed {
			remoteConn.markInvalid(err)
			return nil, trace.ConnectionProblem(
				nil, "remote server %v is not available", addr)
		}
		return utils.NewChConn(remoteConn.sshConn, ch), nil
	}

	for {
		conn, err := try()
		if err != nil {
			s.log.Errorf("[TUNNEL] Dial(addr=%v) failed: %v", addr, err)
			// we interpret it as a "out of connections and will try again"
			if trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		return conn, nil
	}
}

func (s *tunnelSite) DialServer(addr string) (net.Conn, error) {
	s.log.Infof("[TUNNEL] DialServer(addr=%v)", addr)
	clt, err := s.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var knownServers []services.Server
	for i := 0; i < 10; i++ {
		knownServers, err = clt.GetNodes()
		if err != nil {
			log.Errorf("[TUNNEL] failed to get servers: %v", err)
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	server, err := findServer(addr, knownServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.Dial("tcp", server.Addr)
}

func (s *tunnelSite) handleAuthProxy(w http.ResponseWriter, r *http.Request) {
	s.log.Infof("[TUNNEL] handleAuthProxy()")

	fwd, err := forward.New(forward.RoundTripper(s.transport), forward.Logger(s.log))
	if err != nil {
		roundtrip.ReplyJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	r.URL.Scheme = "http"
	r.URL.Host = "stub"
	fwd.ServeHTTP(w, r)
}

const (
	extHost         = "host@teleport"
	extCertType     = "certtype@teleport"
	extAuthority    = "auth@teleport"
	extCertTypeHost = "host"
	extCertTypeUser = "user"
)
