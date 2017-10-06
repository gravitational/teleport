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
	"context"
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
	"github.com/gravitational/teleport/lib/state"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// server is a "reverse tunnel server". it exposes the cluster capabilities
// (like access to a cluster's auth) to remote trusted clients
// (also known as 'reverse tunnel agents'.
type server struct {
	sync.RWMutex
	Config

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

	// clusterPeers is a map of clusters connected to peer proxies
	// via reverse tunnels
	clusterPeers map[string]*clusterPeers

	// newAccessPoint returns new caching access point
	newAccessPoint state.NewCachingAccessPoint

	// cancel function will cancel the
	cancel context.CancelFunc

	// ctx is a context used for signalling and broadcast
	ctx context.Context
}

// DirectCluster is used to access cluster directly
type DirectCluster struct {
	// Name is a cluster name
	Name string
	// Client is a client to the cluster
	Client auth.ClientI
}

// Config is a reverse tunnel server configuration
type Config struct {
	// ID is the ID of this server proxy
	ID string
	// ListenAddr is a listening address for reverse tunnel server
	ListenAddr utils.NetAddr
	// HostSigners is a list of host signers
	HostSigners []ssh.Signer
	// HostKeyCallback
	// Limiter is optional request limiter
	Limiter *limiter.Limiter
	// AccessPoint is access point
	AccessPoint auth.AccessPoint
	// NewCachingAccessPoint returns new caching access points
	// per remote cluster
	NewCachingAccessPoint state.NewCachingAccessPoint
	// DirectClusters is a list of clusters accessed directly
	DirectClusters []DirectCluster
	// Context is a signalling context
	Context context.Context
}

// CheckAndSetDefaults checks parameters and sets default values
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if cfg.ListenAddr.IsEmpty() {
		return trace.BadParameter("missing parameter ListenAddr")
	}
	if cfg.Context == nil {
		cfg.Context = context.TODO()
	}
	if cfg.Limiter == nil {
		var err error
		cfg.Limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewServer creates and returns a reverse tunnel server which is fully
// initialized but hasn't been started yet
func NewServer(cfg Config) (Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	srv := &server{
		Config:         cfg,
		localSites:     []*localSite{},
		remoteSites:    []*remoteSite{},
		localAuth:      cfg.AccessPoint,
		newAccessPoint: cfg.NewCachingAccessPoint,
		limiter:        cfg.Limiter,
		ctx:            ctx,
		cancel:         cancel,
		clusterPeers:   make(map[string]*clusterPeers),
	}

	for _, clusterInfo := range cfg.DirectClusters {
		cluster, err := newlocalSite(srv, clusterInfo.Name, clusterInfo.Client)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		srv.localSites = append(srv.localSites, cluster)
	}

	var err error
	s, err := sshutils.NewServer(
		teleport.ComponentReverseTunnel,
		cfg.ListenAddr,
		srv,
		cfg.HostSigners,
		sshutils.AuthMethods{
			PublicKey: srv.keyAuth,
		},
		sshutils.SetLimiter(cfg.Limiter),
	)
	if err != nil {
		return nil, err
	}
	srv.hostCertChecker = ssh.CertChecker{IsAuthority: srv.isHostAuthority}
	srv.userCertChecker = ssh.CertChecker{IsAuthority: srv.isUserAuthority}
	srv.srv = s
	go srv.periodicFetchClusterPeers()
	return srv, nil
}

func (s *server) periodicFetchClusterPeers() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()
	if err := s.fetchClusterPeers(); err != nil {
		log.Warningf("[TUNNEL] failed to fetch cluster peers: %v", err)
	}
	for {
		select {
		case <-s.ctx.Done():
			log.Debugf("[TUNNEL] closing")
			return
		case <-ticker.C:
			err := s.fetchClusterPeers()
			if err != nil {
				log.Warningf("[TUNNEL] failed to fetch cluster peers: %v", err)
			}
		}
	}
}

func (s *server) fetchClusterPeers() error {
	conns, err := s.AccessPoint.GetAllTunnelConnections()
	if err != nil {
		return trace.Wrap(err)
	}
	newConns := make(map[string]services.TunnelConnection)
	for i := range conns {
		newConn := conns[i]
		// filter out peer records for our own proxy
		if newConn.GetProxyName() == s.ID {
			continue
		}
		newConns[newConn.GetName()] = newConn
	}
	existingConns := s.existingConns()
	connsToAdd, connsToUpdate, connsToRemove := s.diffConns(newConns, existingConns)
	s.removeClusterPeers(connsToRemove)
	s.updateClusterPeers(connsToUpdate)
	return s.addClusterPeers(connsToAdd)
}

func (s *server) addClusterPeers(conns map[string]services.TunnelConnection) error {
	for key := range conns {
		connInfo := conns[key]
		peer, err := newClusterPeer(s, connInfo)
		if err != nil {
			return trace.Wrap(err)
		}
		s.addClusterPeer(peer)
	}
	return nil
}

func (s *server) updateClusterPeers(conns map[string]services.TunnelConnection) {
	for key := range conns {
		connInfo := conns[key]
		s.updateClusterPeer(connInfo)
	}
}

func (s *server) addClusterPeer(peer *clusterPeer) {
	s.Lock()
	defer s.Unlock()
	clusterName := peer.connInfo.GetClusterName()
	peers, ok := s.clusterPeers[clusterName]
	if !ok {
		peers = newClusterPeers(clusterName)
		s.clusterPeers[clusterName] = peers
	}
	peers.addPeer(peer)
}

func (s *server) updateClusterPeer(conn services.TunnelConnection) bool {
	s.Lock()
	defer s.Unlock()
	clusterName := conn.GetClusterName()
	peers, ok := s.clusterPeers[clusterName]
	if !ok {
		return false
	}
	return peers.updatePeer(conn)
}

func (s *server) removeClusterPeers(conns []services.TunnelConnection) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range conns {
		peers, ok := s.clusterPeers[conn.GetClusterName()]
		if !ok {
			log.Warningf("[TUNNEL] failed to remove cluster peer, not found peers for %v", conn)
			continue
		}
		peers.removePeer(conn)
		log.Debugf("[TUNNEL] removed cluster peer %v", conn)
	}
}

func (s *server) existingConns() map[string]services.TunnelConnection {
	s.RLock()
	defer s.RUnlock()
	conns := make(map[string]services.TunnelConnection)
	for _, peers := range s.clusterPeers {
		for _, cluster := range peers.peers {
			conns[cluster.connInfo.GetName()] = cluster.connInfo
		}
	}
	return conns
}

func (s *server) diffConns(newConns, existingConns map[string]services.TunnelConnection) (map[string]services.TunnelConnection, map[string]services.TunnelConnection, []services.TunnelConnection) {
	connsToAdd := make(map[string]services.TunnelConnection)
	connsToUpdate := make(map[string]services.TunnelConnection)
	var connsToRemove []services.TunnelConnection

	for existingKey := range existingConns {
		conn := existingConns[existingKey]
		if _, ok := newConns[existingKey]; !ok { // tunnel was removed
			connsToRemove = append(connsToRemove, conn)
		}
	}

	for newKey := range newConns {
		conn := newConns[newKey]
		if _, ok := existingConns[newKey]; !ok { // tunnel was added
			connsToAdd[newKey] = conn
		} else {
			connsToUpdate[newKey] = conn
		}
	}

	return connsToAdd, connsToUpdate, connsToRemove
}

func (s *server) Wait() {
	s.srv.Wait()
}

func (s *server) Start() error {
	return s.srv.Start()
}

func (s *server) Close() error {
	s.cancel()
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
	log.Infof("[TUNNEL] cluster %v connected from %v. clusters: %d",
		domainName, conn.RemoteAddr(), len(s.remoteSites))
	// treat first connection as a registered heartbeat,
	// otherwise the connection information will appear after initial
	// heartbeat delay
	go site.registerHeartbeat(time.Now())
	return site, remoteConn, nil
}

func (s *server) GetSites() []RemoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]RemoteSite, 0, len(s.remoteSites)+len(s.localSites)+len(s.clusterPeers))
	for i := range s.localSites {
		out = append(out, s.localSites[i])
	}
	haveLocalConnection := make(map[string]bool)
	for i := range s.remoteSites {
		site := s.remoteSites[i]
		haveLocalConnection[site.GetName()] = true
		out = append(out, site)
	}
	for i := range s.clusterPeers {
		cluster := s.clusterPeers[i]
		if _, ok := haveLocalConnection[cluster.GetName()]; !ok {
			out = append(out, cluster)
		}
	}
	return out
}

func (s *server) GetSite(name string) (RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	for i := range s.remoteSites {
		if s.remoteSites[i].GetName() == name {
			return s.remoteSites[i], nil
		}
	}
	for i := range s.localSites {
		if s.localSites[i].GetName() == name {
			return s.localSites[i], nil
		}
	}
	for i := range s.clusterPeers {
		if s.clusterPeers[i].GetName() == name {
			return s.clusterPeers[i], nil
		}
	}
	return nil, trace.NotFound("cluster %q is not found", name)
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
	return trace.NotFound("cluster %q is not found", domainName)
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
	connInfo, err := services.NewTunnelConnection(
		fmt.Sprintf("%v-%v", srv.ID, domainName),
		services.TunnelConnectionSpecV2{
			ClusterName:   domainName,
			ProxyName:     srv.ID,
			LastHeartbeat: time.Now().UTC(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	remoteSite := &remoteSite{
		srv:        srv,
		domainName: domainName,
		connInfo:   connInfo,
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
