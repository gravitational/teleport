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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/state"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// server is a "reverse tunnel server". it exposes the cluster capabilities
// (like access to a cluster's auth) to remote trusted clients
// (also known as 'reverse tunnel agents'.
type server struct {
	sync.RWMutex

	// localAuth points to the cluster's auth server API
	localAuth       auth.AccessPoint
	hostCertChecker ssh.CertChecker
	userCertChecker ssh.CertChecker

	// srv is the "base class" i.e. the underlying SSH server
	srv     *sshutils.Server
	limiter *limiter.Limiter

	// remoteSites is the list of conencted remote clusters
	remoteSites []*remoteSite

	// localSites is the list of local (our own cluster) tunnel clients,
	// usually each of them is a local proxy.
	localSites []*localSite

	// newAccessPoint returns new caching access point
	newAccessPoint state.NewCachingAccessPoint
}

// ServerOption sets reverse tunnel server options
type ServerOption func(s *server) error

// DirectSite instructs server to proxy access to this site not using
// reverse tunnel
func DirectSite(domainName string, clt auth.ClientI) ServerOption {
	return func(s *server) error {
		site, err := newlocalSite(s, domainName, clt)
		if err != nil {
			return trace.Wrap(err)
		}
		s.localSites = append(s.localSites, site)
		return nil
	}
}

// SetLimiter sets rate limiter for reverse tunnel
func SetLimiter(limiter *limiter.Limiter) ServerOption {
	return func(s *server) error {
		s.limiter = limiter
		return nil
	}
}

// NewServer creates and returns a reverse tunnel server which is fully
// initialized but hasn't been started yet
func NewServer(addr utils.NetAddr, hostSigners []ssh.Signer,
	authAPI auth.AccessPoint, fn state.NewCachingAccessPoint, opts ...ServerOption) (Server, error) {

	srv := &server{
		localSites:     []*localSite{},
		remoteSites:    []*remoteSite{},
		localAuth:      authAPI,
		newAccessPoint: fn,
	}
	var err error
	srv.limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, o := range opts {
		if err := o(srv); err != nil {
			return nil, trace.Wrap(err)
		}
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

func (s *server) Start() error {
	return s.srv.Start()
}

func (s *server) Close() error {
	return s.srv.Close()
}

func (s *server) HandleNewChan(conn net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	// apply read/write timeouts to the server connection
	conn = utils.ObeyIdleTimeout(conn,
		defaults.ReverseTunnelAgentHeartbeatPeriod*10,
		"reverse tunnel server")

	ct := nch.ChannelType()
	if ct != chanHeartbeat {
		msg := fmt.Sprintf("reversetunnel received unknown channel request %v from %v",
			nch.ChannelType(), sconn)
		// if someone is trying to open a new SSH session by talking to a reverse tunnel,
		// they're most likely using the wrong port number. Lets give them the explicit hint:
		if ct == "session" {
			msg = "Cannot open new SSH session on reverse tunnel. Are you connecting to the right port?"
		}
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
		if ca.GetClusterName() != domainName {
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

func (s *server) upsertSite(conn net.Conn, sshConn *ssh.ServerConn) (*remoteSite, *remoteConn, error) {
	domainName := sshConn.Permissions.Extensions[extAuthority]
	if strings.TrimSpace(domainName) == "" {
		return nil, nil, trace.BadParameter("Cannot create reverse tunnel: empty domain name")
	}

	s.Lock()
	defer s.Unlock()

	var site *remoteSite
	for _, st := range s.remoteSites {
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
		s.remoteSites = append(s.remoteSites, site)
	}
	log.Infof("[TUNNEL] site %v connected from %v. sites: %d",
		domainName, conn.RemoteAddr(), len(s.remoteSites))
	return site, remoteConn, nil
}

func (s *server) GetSites() []RemoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]RemoteSite, 0, len(s.remoteSites)+len(s.localSites))
	for i := range s.localSites {
		out = append(out, s.localSites[i])
	}
	for i := range s.remoteSites {
		out = append(out, s.remoteSites[i])
	}
	return out
}

func (s *server) GetSite(domainName string) (RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	for i := range s.remoteSites {
		if s.remoteSites[i].domainName == domainName {
			return s.remoteSites[i], nil
		}
	}
	for i := range s.localSites {
		if s.localSites[i].domainName == domainName {
			return s.localSites[i], nil
		}
	}
	return nil, trace.NotFound("site '%v' not found", domainName)
}

func (s *server) RemoveSite(domainName string) error {
	s.Lock()
	defer s.Unlock()
	for i := range s.remoteSites {
		if s.remoteSites[i].domainName == domainName {
			s.remoteSites = append(s.remoteSites[:i], s.remoteSites[i+1:]...)
			return nil
		}
	}
	for i := range s.localSites {
		if s.localSites[i].domainName == domainName {
			s.localSites = append(s.localSites[:i], s.localSites[i+1:]...)
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

// newRemoteSite helper creates and initializes 'remoteSite' instance
func newRemoteSite(srv *server, domainName string) (*remoteSite, error) {
	remoteSite := &remoteSite{
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

	accessPoint, err := srv.newAccessPoint(clt, []string{"reverse", domainName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	remoteSite.accessPoint = accessPoint

	return remoteSite, nil
}

const (
	extHost         = "host@teleport"
	extCertType     = "certtype@teleport"
	extAuthority    = "auth@teleport"
	extCertTypeHost = "host"
	extCertTypeUser = "user"
)
