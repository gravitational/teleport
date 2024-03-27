/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reversetunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

// remoteSite is a remote site that established the inbound connection to
// the local reverse tunnel server, and now it can provide access to the
// cluster behind it.
type remoteSite struct {
	sync.RWMutex

	logger      *log.Entry
	domainName  string
	connections []*remoteConn
	lastUsed    int
	srv         *server

	// connInfo represents the connection to the remote cluster.
	connInfo types.TunnelConnection
	// lastConnInfo is the last connInfo.
	lastConnInfo types.TunnelConnection

	ctx    context.Context
	cancel context.CancelFunc
	clock  clockwork.Clock

	// certificateCache caches host certificates for the forwarding server.
	certificateCache *certificateCache

	// localClient provides access to the Auth Server API of the cluster
	// within which reversetunnelclient.Server is running.
	localClient auth.ClientI
	// remoteClient provides access to the Auth Server API of the remote cluster that
	// this site belongs to.
	remoteClient auth.ClientI
	// localAccessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	localAccessPoint auth.ProxyAccessPoint
	// remoteAccessPoint provides access to a cached subset of the Auth Server API of
	// the remote cluster this site belongs to.
	remoteAccessPoint auth.RemoteProxyAccessPoint

	// nodeWatcher provides access the node set for the remote site
	nodeWatcher *services.NodeWatcher

	// remoteCA is the last remote certificate authority recorded by the client.
	// It is used to detect CA rotation status changes. If the rotation
	// state has been changed, the tunnel will reconnect to re-create the client
	// with new settings.
	remoteCA types.CertAuthority

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration

	// proxySyncInterval defines the interval at which discovery requests are
	// sent to keep agents in sync
	proxySyncInterval time.Duration
}

func (s *remoteSite) getRemoteClient() (auth.ClientI, bool, error) {
	// check if all cert authorities are initiated and if everything is OK
	ca, err := s.srv.localAccessPoint.GetCertAuthority(s.ctx, types.CertAuthID{Type: types.HostCA, DomainName: s.domainName}, false)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	keys := ca.GetTrustedTLSKeyPairs()

	// The fact that cluster has keys to remote CA means that the key exchange
	// has completed.
	if len(keys) != 0 {
		s.logger.Debug("Using TLS client to remote cluster.")
		pool, err := services.CertPool(ca)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		tlsConfig := s.srv.ClientTLS.Clone()
		tlsConfig.RootCAs = pool
		// encode the name of this cluster to identify this cluster,
		// connecting to the remote one (it is used to find the right certificate
		// authority to verify)
		tlsConfig.ServerName = apiutils.EncodeClusterName(s.srv.ClusterName)
		clt, err := auth.NewClient(client.Config{
			Dialer: client.ContextDialerFunc(s.authServerContextDialer),
			Credentials: []client.Credentials{
				client.LoadTLS(tlsConfig),
			},
			CircuitBreakerConfig: s.srv.CircuitBreakerConfig,
		})
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		return clt, false, nil
	}

	return nil, false, trace.BadParameter("no TLS keys found")
}

func (s *remoteSite) authServerContextDialer(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := s.DialAuthServer(reversetunnelclient.DialParams{})
	return conn, err
}

// GetTunnelsCount always returns 0 for local cluster
func (s *remoteSite) GetTunnelsCount() int {
	return s.connectionCount()
}

func (s *remoteSite) CachingAccessPoint() (auth.RemoteProxyAccessPoint, error) {
	return s.remoteAccessPoint, nil
}

// NodeWatcher returns the services.NodeWatcher for the remote cluster.
func (s *remoteSite) NodeWatcher() (*services.NodeWatcher, error) {
	return s.nodeWatcher, nil
}

func (s *remoteSite) GetClient() (auth.ClientI, error) {
	return s.remoteClient, nil
}

func (s *remoteSite) String() string {
	return fmt.Sprintf("remoteSite(%v)", s.domainName)
}

func (s *remoteSite) connectionCount() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.connections)
}

func (s *remoteSite) HasValidConnections() bool {
	s.RLock()
	defer s.RUnlock()

	for _, conn := range s.connections {
		if !conn.isInvalid() {
			return true
		}
	}
	return false
}

// Close closes remote cluster connections
func (s *remoteSite) Close() error {
	s.Lock()
	defer s.Unlock()

	s.cancel()

	var errors []error
	for i := range s.connections {
		if err := s.connections[i].Close(); err != nil {
			errors = append(errors, err)
		}
	}
	s.connections = []*remoteConn{}
	if s.remoteAccessPoint != nil {
		if err := s.remoteAccessPoint.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	return trace.NewAggregate(errors...)
}

// IsClosed reports whether this remoteSite has been closed.
func (s *remoteSite) IsClosed() bool {
	return s.ctx.Err() != nil
}

// nextConn returns next connection that is ready
// and has not been marked as invalid
// it will close connections marked as invalid
func (s *remoteSite) nextConn() (*remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	s.removeInvalidConns()

	for i := 0; i < len(s.connections); i++ {
		s.lastUsed = (s.lastUsed + 1) % len(s.connections)
		remoteConn := s.connections[s.lastUsed]
		// connection could have been initiated, but agent
		// on the other side is not ready yet.
		// Proxy assumes that connection is ready to serve when
		// it has received a first heartbeat, otherwise
		// it could attempt to use it before the agent
		// had a chance to start handling connection requests,
		// what could lead to proxy marking the connection
		// as invalid without a good reason.
		if remoteConn.isReady() {
			return remoteConn, nil
		}
	}

	return nil, trace.NotFound("%v is offline: no active tunnels to %v found", s.GetName(), s.srv.ClusterName)
}

// removeInvalidConns removes connections marked as invalid,
// it should be called only under write lock
func (s *remoteSite) removeInvalidConns() {
	// for first pass, do nothing if no connections are marked
	count := 0
	for _, conn := range s.connections {
		if conn.isInvalid() {
			count++
		}
	}
	if count == 0 {
		return
	}
	s.lastUsed = 0
	conns := make([]*remoteConn, 0, len(s.connections)-count)
	for i := range s.connections {
		if !s.connections[i].isInvalid() {
			conns = append(conns, s.connections[i])
		} else {
			go func(conn *remoteConn) {
				if err := conn.Close(); err != nil {
					s.logger.WithError(err).Warn("Failed to close invalid connection")
				}
			}(s.connections[i])
		}
	}
	s.connections = conns
}

// addConn helper adds a new active remote cluster connection to the list
// of such connections
func (s *remoteSite) addConn(conn net.Conn, sconn ssh.Conn) (*remoteConn, error) {
	s.Lock()
	defer s.Unlock()

	rconn := newRemoteConn(&connConfig{
		conn:             conn,
		sconn:            sconn,
		tunnelType:       string(types.ProxyTunnel),
		proxyName:        s.connInfo.GetProxyName(),
		clusterName:      s.domainName,
		offlineThreshold: s.offlineThreshold,
	})

	s.connections = append(s.connections, rconn)
	s.lastUsed = 0
	return rconn, nil
}

func (s *remoteSite) adviseReconnect(ctx context.Context) {
	wg := &sync.WaitGroup{}

	s.RLock()
	for _, conn := range s.connections {
		s.logger.Debugf("Sending reconnect: %s", conn.nodeID)

		wg.Add(1)
		go func(conn *remoteConn) {
			if err := conn.adviseReconnect(); err != nil {
				s.logger.WithError(err).Warn("Failed to send reconnection advisory")
			}
			wg.Done()
		}(conn)
	}
	s.RUnlock()

	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()

	select {
	case <-ctx.Done():
	case <-wait:
	}
}

func (s *remoteSite) GetStatus() string {
	connInfo, err := s.getLastConnInfo()
	if err != nil {
		return teleport.RemoteClusterStatusOffline
	}
	return services.TunnelConnectionStatus(s.clock, connInfo, s.offlineThreshold)
}

func (s *remoteSite) copyConnInfo() types.TunnelConnection {
	s.RLock()
	defer s.RUnlock()
	return s.connInfo.Clone()
}

func (s *remoteSite) setLastConnInfo(connInfo types.TunnelConnection) {
	s.Lock()
	defer s.Unlock()
	s.lastConnInfo = connInfo.Clone()
}

func (s *remoteSite) getLastConnInfo() (types.TunnelConnection, error) {
	s.RLock()
	defer s.RUnlock()
	if s.lastConnInfo == nil {
		return nil, trace.NotFound("no last connection found")
	}
	return s.lastConnInfo.Clone(), nil
}

func (s *remoteSite) registerHeartbeat(t time.Time) {
	connInfo := s.copyConnInfo()
	connInfo.SetLastHeartbeat(t)
	connInfo.SetExpiry(s.clock.Now().Add(s.offlineThreshold))
	s.setLastConnInfo(connInfo)
	err := s.localAccessPoint.UpsertTunnelConnection(connInfo)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to register heartbeat")
	}
}

// deleteConnectionRecord deletes connection record to let know peer proxies
// that this node lost the connection and needs to be discovered
func (s *remoteSite) deleteConnectionRecord() {
	if err := s.localAccessPoint.DeleteTunnelConnection(s.connInfo.GetClusterName(), s.connInfo.GetName()); err != nil {
		s.logger.WithError(err).Warn("Failed to delete tunnel connection")
	}
}

// fanOutProxies is a non-blocking call that puts the new proxies
// list so that remote connection can notify the remote agent
// about the list update
func (s *remoteSite) fanOutProxies(proxies []types.Server) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range s.connections {
		conn.updateProxies(proxies)
	}
}

// handleHeartbeat receives heartbeat messages from the connected agent
// if the agent has missed several heartbeats in a row, Proxy marks
// the connection as invalid.
func (s *remoteSite) handleHeartbeat(conn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	logger := s.logger.WithFields(log.Fields{
		"serverID": conn.nodeID,
		"addr":     conn.conn.RemoteAddr().String(),
	})

	sshutils.DiscardChannelData(ch)
	if ch != nil {
		defer func() {
			if err := ch.Close(); err != nil {
				logger.Warnf("Failed to close heartbeat channel: %v", err)
			}
		}()
	}

	firstHeartbeat := true
	proxyResyncTicker := s.clock.NewTicker(s.proxySyncInterval)
	defer func() {
		proxyResyncTicker.Stop()
		logger.Info("Cluster connection closed.")

		if err := conn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
			logger.WithError(err).Warnf("Failed to close remote connection for remote site")
		}

		if err := s.srv.onSiteTunnelClose(s); err != nil {
			logger.WithError(err).Warn("Failed to close remote site")
		}
	}()

	offlineThresholdTimer := time.NewTimer(s.offlineThreshold)
	defer offlineThresholdTimer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			logger.Infof("closing")
			return
		case <-proxyResyncTicker.Chan():
			var req discoveryRequest
			req.SetProxies(s.srv.proxyWatcher.GetCurrent())

			if err := conn.sendDiscoveryRequest(req); err != nil {
				logger.WithError(err).Debug("Marking connection invalid on error")
				conn.markInvalid(err)
				return
			}
		case proxies := <-conn.newProxiesC:
			var req discoveryRequest
			req.SetProxies(proxies)

			if err := conn.sendDiscoveryRequest(req); err != nil {
				logger.WithError(err).Debug("Marking connection invalid on error")
				conn.markInvalid(err)
				return
			}
		case req := <-reqC:
			if req == nil {
				logger.Info("Cluster agent disconnected.")
				conn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
				if !s.HasValidConnections() {
					logger.Debug("Deleting connection record.")
					s.deleteConnectionRecord()
				}
				return
			}
			if firstHeartbeat {
				// as soon as the agent connects and sends a first heartbeat
				// send it the list of current proxies back
				current := s.srv.proxyWatcher.GetCurrent()
				if len(current) > 0 {
					conn.updateProxies(current)
				}
				firstHeartbeat = false
			}
			var timeSent time.Time
			var roundtrip time.Duration
			if req.Payload != nil {
				if err := timeSent.UnmarshalText(req.Payload); err == nil {
					roundtrip = s.srv.Clock.Now().Sub(timeSent)
				}
			}

			pinglog := logger
			if roundtrip != 0 {
				pinglog = pinglog.WithField("latency", roundtrip)
			}
			pinglog.Debugf("Ping <- %v", conn.conn.RemoteAddr())

			tm := s.clock.Now().UTC()
			conn.setLastHeartbeat(tm)
			go s.registerHeartbeat(tm)
		case t := <-offlineThresholdTimer.C:
			conn.markInvalid(trace.ConnectionProblem(nil, "no heartbeats for %v", s.offlineThreshold))

			// terminate and remove the connection after missing more than missedHeartBeatThreshold heartbeats if
			// the connection isn't still servicing any sessions
			hb := conn.getLastHeartbeat()
			if t.After(hb.Add(s.offlineThreshold * missedHeartBeatThreshold)) {
				count := conn.activeSessions()
				if count == 0 {
					logger.Errorf("Closing unhealthy and idle connection. Heartbeat last received at %s", hb)
					return
				}

				logger.Warnf("Deferring closure of unhealthy connection due to %d active connections", count)
			}

			offlineThresholdTimer.Reset(s.offlineThreshold)
			continue
		}

		if !offlineThresholdTimer.Stop() {
			<-offlineThresholdTimer.C
		}
		offlineThresholdTimer.Reset(s.offlineThreshold)
	}
}

func (s *remoteSite) GetName() string {
	return s.domainName
}

func (s *remoteSite) GetLastConnected() time.Time {
	connInfo, err := s.getLastConnInfo()
	if err != nil {
		return time.Time{}
	}
	return connInfo.GetLastHeartbeat()
}

func (s *remoteSite) compareAndSwapCertAuthority(ca types.CertAuthority) error {
	s.Lock()
	defer s.Unlock()

	if s.remoteCA == nil {
		s.remoteCA = ca
		return nil
	}

	rotation := s.remoteCA.GetRotation()
	if rotation.Matches(ca.GetRotation()) {
		s.remoteCA = ca
		return nil
	}
	s.remoteCA = ca
	return trace.CompareFailed("remote certificate authority rotation has been updated")
}

func (s *remoteSite) updateCertAuthorities(retry retryutils.Retry, remoteWatcher *services.CertAuthorityWatcher, remoteVersion string) {
	defer remoteWatcher.Close()

	for {
		err := s.watchCertAuthorities(remoteWatcher, remoteVersion)
		if err != nil {
			switch {
			case trace.IsNotFound(err):
				s.logger.Debug("Remote cluster does not support cert authorities rotation yet.")
			case trace.IsCompareFailed(err):
				s.logger.Info("Remote cluster has updated certificate authorities, going to force reconnect.")
				if err := s.srv.onSiteTunnelClose(&alwaysClose{RemoteSite: s}); err != nil {
					s.logger.WithError(err).Warn("Failed to close remote site")
				}
				return
			case trace.IsConnectionProblem(err):
				s.logger.Debug("Remote cluster is offline.")
			default:
				s.logger.Warnf("Could not perform cert authorities update: %v.", trace.DebugReport(err))
			}
		}

		startedWaiting := s.clock.Now()
		select {
		case t := <-retry.After():
			s.logger.Debugf("Initiating new cert authority watch after waiting %v.", t.Sub(startedWaiting))
			retry.Inc()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *remoteSite) watchCertAuthorities(remoteWatcher *services.CertAuthorityWatcher, remoteVersion string) error {
	filter := types.CertAuthorityFilter{
		types.HostCA:     s.srv.ClusterName,
		types.UserCA:     s.srv.ClusterName,
		types.DatabaseCA: s.srv.ClusterName,
		types.OpenSSHCA:  s.srv.ClusterName,
	}
	localWatch, err := s.srv.CertAuthorityWatcher.Subscribe(s.ctx, filter)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := localWatch.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close local ca watcher subscription.")
		}
	}()

	remoteWatch, err := remoteWatcher.Subscribe(
		s.ctx,
		types.CertAuthorityFilter{
			types.HostCA: s.domainName,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := remoteWatch.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close remote ca watcher subscription.")
		}
	}()

	localCAs := make(map[types.CertAuthType]types.CertAuthority, len(filter))
	for caType, clusterName := range filter {
		caID := types.CertAuthID{
			Type:       caType,
			DomainName: clusterName,
		}
		ca, err := s.localAccessPoint.GetCertAuthority(s.ctx, caID, false)
		if err != nil {
			return trace.Wrap(err, "failed to get local cert authority")
		}
		if err := s.remoteClient.RotateExternalCertAuthority(s.ctx, ca); err != nil {
			return trace.Wrap(err, "failed to push local cert authority")
		}
		s.logger.Debugf("Pushed local cert authority %v", caID.String())
		localCAs[caType] = ca
	}

	remoteCA, err := s.remoteAccessPoint.GetCertAuthority(s.ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: s.domainName,
	}, false)
	if err != nil {
		return trace.Wrap(err, "failed to get remote cert authority")
	}
	if remoteCA.GetName() != s.domainName || remoteCA.GetType() != types.HostCA {
		return trace.BadParameter("received wrong CA, expected remote host CA, got %v", remoteCA.GetID())
	}

	maybeUpsertRemoteCA := func(remoteCA types.CertAuthority) error {
		oldRemoteCA, err := s.localAccessPoint.GetCertAuthority(s.ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: remoteCA.GetClusterName(),
		}, false)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		// if CA is changed or does not exist, update backend
		if err != nil || !services.CertAuthoritiesEquivalent(oldRemoteCA, remoteCA) {
			s.logger.Debugf("Ingesting remote cert authority %v", remoteCA.GetID())
			if err := s.localClient.UpsertCertAuthority(s.ctx, remoteCA); err != nil {
				return trace.Wrap(err)
			}
		}

		// keep track of when the remoteSite needs to reconnect
		if err := s.compareAndSwapCertAuthority(remoteCA); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	if err := maybeUpsertRemoteCA(remoteCA); err != nil {
		return trace.Wrap(err)
	}

	s.logger.Debugf("Watching for cert authority changes.")
	for {
		select {
		case <-s.ctx.Done():
			s.logger.WithError(s.ctx.Err()).Debug("Context is closing.")
			return trace.Wrap(s.ctx.Err())
		case <-localWatch.Done():
			s.logger.Warn("Local CertAuthority watcher subscription has closed")
			return fmt.Errorf("local ca watcher for cluster %s has closed", s.srv.ClusterName)
		case <-remoteWatch.Done():
			s.logger.Warn("Remote CertAuthority watcher subscription has closed")
			return fmt.Errorf("remote ca watcher for cluster %s has closed", s.domainName)
		case evt := <-localWatch.Events():
			switch evt.Type {
			case types.OpPut:
				newCA, ok := evt.Resource.(types.CertAuthority)
				if !ok {
					continue
				}

				previousCA, ok := localCAs[newCA.GetType()]
				if ok && services.CertAuthoritiesEquivalent(previousCA, newCA) {
					continue
				}

				// clone to prevent a race with watcher filtering, as
				// RotateExternalCertAuthority (client side) will end up calling
				// CheckAndSetDefaults
				// TODO(espadolini): figure out who should be responsible for validating the CA *once*
				newCA = newCA.Clone()
				if err := s.remoteClient.RotateExternalCertAuthority(s.ctx, newCA); err != nil {
					log.WithError(err).Warn("Failed to rotate external ca")
					return trace.Wrap(err)
				}

				localCAs[newCA.GetType()] = newCA
			}
		case evt := <-remoteWatch.Events():
			switch evt.Type {
			case types.OpPut:
				remoteCA, ok := evt.Resource.(types.CertAuthority)
				if !ok {
					continue
				}

				// the CA might not be trusted but the watcher's fanout logic is
				// local, so this is ok
				if err := maybeUpsertRemoteCA(remoteCA); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
}

func (s *remoteSite) updateLocks(retry retryutils.Retry) {
	s.logger.Debugf("Watching for remote lock changes.")

	for {
		startedWaiting := s.clock.Now()
		select {
		case t := <-retry.After():
			s.logger.Debugf("Initiating new lock watch after waiting %v.", t.Sub(startedWaiting))
			retry.Inc()
		case <-s.ctx.Done():
			return
		}

		if err := s.watchLocks(); err != nil {
			switch {
			case trace.IsNotImplemented(err):
				s.logger.Debugf("Remote cluster %v does not support locks yet.", s.domainName)
			case trace.IsConnectionProblem(err):
				s.logger.Debugf("Remote cluster %v is offline.", s.domainName)
			default:
				s.logger.WithError(err).Warn("Could not update remote locks.")
			}
		}
	}
}

func (s *remoteSite) watchLocks() error {
	watcher, err := s.srv.LockWatcher.Subscribe(s.ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to subscribe to LockWatcher")
		return err
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			s.logger.WithError(err).Warn("Failed to close lock watcher subscription.")
		}
	}()

	for {
		select {
		case <-watcher.Done():
			s.logger.WithError(watcher.Error()).Warn("Lock watcher subscription has closed")
			return trace.Wrap(watcher.Error())
		case <-s.ctx.Done():
			s.logger.WithError(s.ctx.Err()).Debug("Context is closing.")
			return trace.Wrap(s.ctx.Err())
		case evt := <-watcher.Events():
			switch evt.Type {
			case types.OpPut, types.OpDelete:
				locks := s.srv.LockWatcher.GetCurrent()
				if err := s.remoteClient.ReplaceRemoteLocks(s.ctx, s.srv.ClusterName, locks); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
}

func (s *remoteSite) DialAuthServer(params reversetunnelclient.DialParams) (net.Conn, error) {
	conn, err := s.connThroughTunnel(&sshutils.DialReq{
		Address:       constants.RemoteAuthServer,
		ClientSrcAddr: stringOrEmpty(params.From),
		ClientDstAddr: stringOrEmpty(params.OriginalClientDstAddr),
	})
	return conn, err
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (s *remoteSite) Dial(params reversetunnelclient.DialParams) (net.Conn, error) {
	localRecCfg, err := s.localAccessPoint.GetSessionRecordingConfig(s.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if shouldDialAndForward(params, localRecCfg) {
		return s.dialAndForward(params)
	}

	if params.ConnType == types.NodeTunnel {
		// If the remote cluster is recording at the proxy we need to respect
		// that and forward and record the session. We will be connecting
		// to the node without connecting through the remote proxy, so the
		// session won't have a chance to get recorded at the remote proxy.
		remoteRecCfg, err := s.remoteAccessPoint.GetSessionRecordingConfig(s.ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if services.IsRecordAtProxy(remoteRecCfg.GetMode()) {
			return s.dialAndForward(params)
		}
	}

	// Attempt to perform a direct TCP dial.
	return s.DialTCP(params)
}

func (s *remoteSite) DialTCP(params reversetunnelclient.DialParams) (net.Conn, error) {
	s.logger.Debugf("Dialing from %v to %v.", params.From, params.To)

	conn, err := s.connThroughTunnel(&sshutils.DialReq{
		Address:         params.To.String(),
		ServerID:        params.ServerID,
		ConnType:        params.ConnType,
		ClientSrcAddr:   stringOrEmpty(params.From),
		ClientDstAddr:   stringOrEmpty(params.OriginalClientDstAddr),
		IsAgentlessNode: params.IsAgentlessNode,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *remoteSite) dialAndForward(params reversetunnelclient.DialParams) (_ net.Conn, retErr error) {
	if params.GetUserAgent == nil && !params.IsAgentlessNode {
		return nil, trace.BadParameter("user agent getter is required for teleport nodes")
	}
	s.logger.Debugf("Dialing and forwarding from %v to %v.", params.From, params.To)

	// request user agent connection if a SSH user agent is set
	var userAgent teleagent.Agent
	if params.GetUserAgent != nil {
		ua, err := params.GetUserAgent()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAgent = ua
		defer func() {
			if retErr != nil {
				retErr = trace.NewAggregate(retErr, userAgent.Close())
			}
		}()
	}

	// Get a host certificate for the forwarding node from the cache.
	hostCertificate, err := s.certificateCache.getHostCertificate(s.ctx, params.Address, params.Principals)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetConn, err := s.connThroughTunnel(&sshutils.DialReq{
		Address:         params.To.String(),
		ServerID:        params.ServerID,
		ConnType:        params.ConnType,
		ClientSrcAddr:   stringOrEmpty(params.From),
		ClientDstAddr:   stringOrEmpty(params.OriginalClientDstAddr),
		IsAgentlessNode: params.IsAgentlessNode,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a forwarding server that serves a single SSH connection on it. This
	// server does not need to close, it will close and release all resources
	// once conn is closed.
	serverConfig := forward.ServerConfig{
		LocalAuthClient:          s.localClient,
		TargetClusterAccessPoint: s.remoteAccessPoint,
		UserAgent:                userAgent,
		IsAgentlessNode:          params.IsAgentlessNode,
		AgentlessSigner:          params.AgentlessSigner,
		TargetConn:               targetConn,
		SrcAddr:                  params.From,
		DstAddr:                  params.To,
		HostCertificate:          hostCertificate,
		Ciphers:                  s.srv.Config.Ciphers,
		KEXAlgorithms:            s.srv.Config.KEXAlgorithms,
		MACAlgorithms:            s.srv.Config.MACAlgorithms,
		DataDir:                  s.srv.Config.DataDir,
		Address:                  params.Address,
		UseTunnel:                UseTunnel(s.logger, targetConn),
		FIPS:                     s.srv.FIPS,
		HostUUID:                 s.srv.ID,
		Emitter:                  s.srv.Config.Emitter,
		ParentContext:            s.srv.Context,
		LockWatcher:              s.srv.LockWatcher,
		TargetID:                 params.ServerID,
		TargetAddr:               params.To.String(),
		TargetHostname:           params.Address,
		TargetServer:             params.TargetServer,
		Clock:                    s.clock,
	}
	// Ensure the hostname is set correctly if we have details of the target
	if params.TargetServer != nil {
		serverConfig.TargetHostname = params.TargetServer.GetHostname()
	}
	remoteServer, err := forward.New(serverConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go remoteServer.Serve()

	// Return a connection to the forwarding server.
	conn, err := remoteServer.Dial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// UseTunnel makes a channel request asking for the type of connection. If
// the other side does not respond (older cluster) or takes to long to
// respond, be on the safe side and assume it's not a tunnel connection.
func UseTunnel(logger *log.Entry, c *sshutils.ChConn) bool {
	responseCh := make(chan bool, 1)

	go func() {
		ok, err := c.SendRequest(sshutils.ConnectionTypeRequest, true, nil)
		if err != nil {
			responseCh <- false
			return
		}
		responseCh <- ok
	}()

	select {
	case response := <-responseCh:
		return response
	case <-time.After(1 * time.Second):
		logger.Debugf("Timed out waiting for response: returning false.")
		return false
	}
}

func (s *remoteSite) connThroughTunnel(req *sshutils.DialReq) (*sshutils.ChConn, error) {
	s.logger.Debugf("Requesting connection to %v [%v] in remote cluster.", req.Address, req.ServerID)

	// Loop through existing remote connections and try and establish a
	// connection over the "reverse tunnel".
	var conn *sshutils.ChConn
	var err error
	for i := 0; i < s.connectionCount(); i++ {
		conn, err = s.chanTransportConn(req)
		if err == nil {
			return conn, nil
		}
		s.logger.WithError(err).Warn("Request for connection to remote site failed")
	}

	// Didn't connect and no error? This means we didn't have any connected
	// tunnels to try.
	if err == nil {
		// Return the appropriate message if the user is trying to connect to a
		// cluster or a node.
		message := fmt.Sprintf("cluster %v is offline", s.GetName())
		if req.Address != constants.RemoteAuthServer {
			message = fmt.Sprintf("node %v is offline", req.Address)
		}
		err = trace.ConnectionProblem(nil, message)
	}
	return nil, err
}

func (s *remoteSite) chanTransportConn(req *sshutils.DialReq) (*sshutils.ChConn, error) {
	rconn, err := s.nextConn()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, markInvalid, err := sshutils.ConnectProxyTransport(rconn.sconn, req, false)
	if err != nil {
		if markInvalid {
			rconn.markInvalid(err)
		}
		return nil, trace.Wrap(err)
	}

	return conn, nil
}
