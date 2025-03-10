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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// remoteSite is a remote site that established the inbound connection to
// the local reverse tunnel server, and now it can provide access to the
// cluster behind it.
type remoteSite struct {
	sync.RWMutex

	logger      *slog.Logger
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
	localClient authclient.ClientI
	// remoteClient provides access to the Auth Server API of the remote cluster that
	// this site belongs to.
	remoteClient authclient.ClientI
	// localAccessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	localAccessPoint authclient.ProxyAccessPoint
	// remoteAccessPoint provides access to a cached subset of the Auth Server API of
	// the remote cluster this site belongs to.
	remoteAccessPoint authclient.RemoteProxyAccessPoint

	// nodeWatcher provides access the node set for the remote site
	nodeWatcher *services.GenericWatcher[types.Server, readonly.Server]

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

func (s *remoteSite) getRemoteClient() (authclient.ClientI, bool, error) {
	// check if all cert authorities are initiated and if everything is OK
	ca, err := s.srv.localAccessPoint.GetCertAuthority(s.ctx, types.CertAuthID{Type: types.HostCA, DomainName: s.domainName}, false)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if len(ca.GetTrustedTLSKeyPairs()) == 0 {
		return nil, false, trace.BadParameter("no TLS keys found")
	}
	// The fact that cluster has keys to remote CA means that the key exchange
	// has completed.

	s.logger.DebugContext(s.ctx, "Using TLS client to remote cluster")
	tlsConfig := utils.TLSConfig(s.srv.ClientTLSCipherSuites)
	// encode the name of this cluster to identify this cluster,
	// connecting to the remote one (it is used to find the right certificate
	// authority to verify)
	tlsConfig.ServerName = apiutils.EncodeClusterName(s.srv.ClusterName)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := s.srv.GetClientTLSCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(func() (*x509.CertPool, error) {
		pool, _, err := authclient.ClientCertPool(s.ctx, s.srv.localAccessPoint, s.domainName, types.HostCA)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return pool, nil
	})

	clt, err := authclient.NewClient(client.Config{
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

func (s *remoteSite) authServerContextDialer(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := s.DialAuthServer(reversetunnelclient.DialParams{})
	return conn, err
}

// GetTunnelsCount always returns 0 for local cluster
func (s *remoteSite) GetTunnelsCount() int {
	return s.connectionCount()
}

func (s *remoteSite) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return s.remoteAccessPoint, nil
}

// NodeWatcher returns the services.NodeWatcher for the remote cluster.
func (s *remoteSite) NodeWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return s.nodeWatcher, nil
}

// GitServerWatcher returns the Git server watcher for the remote cluster.
func (s *remoteSite) GitServerWatcher() (*services.GenericWatcher[types.Server, readonly.Server], error) {
	return nil, trace.NotImplemented("GitServerWatcher not implemented for remoteSite")
}

func (s *remoteSite) GetClient() (authclient.ClientI, error) {
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
					s.logger.WarnContext(s.ctx, "Failed to close invalid connection", "error", err)
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
		s.logger.DebugContext(ctx, "Sending reconnect to server", "server_id", conn.nodeID)

		wg.Add(1)
		go func(conn *remoteConn) {
			if err := conn.adviseReconnect(); err != nil {
				s.logger.WarnContext(ctx, "Failed to send reconnection advisory", "error", err)
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
		s.logger.WarnContext(s.ctx, "Failed to register heartbeat", "error", err)
	}
}

// deleteConnectionRecord deletes connection record to let know peer proxies
// that this node lost the connection and needs to be discovered
func (s *remoteSite) deleteConnectionRecord() {
	if err := s.localAccessPoint.DeleteTunnelConnection(s.connInfo.GetClusterName(), s.connInfo.GetName()); err != nil {
		s.logger.WarnContext(s.ctx, "Failed to delete tunnel connection", "error", err)
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
func (s *remoteSite) handleHeartbeat(ctx context.Context, conn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	logger := s.logger.With(
		"server_id", conn.nodeID,
		"addr", logutils.StringerAttr(conn.conn.RemoteAddr()),
	)

	sshutils.DiscardChannelData(ch)
	if ch != nil {
		defer func() {
			if err := ch.Close(); err != nil {
				logger.WarnContext(ctx, "Failed to close heartbeat channel", "error", err)
			}
		}()
	}

	firstHeartbeat := true
	proxyResyncTicker := s.clock.NewTicker(s.proxySyncInterval)
	defer func() {
		proxyResyncTicker.Stop()
		logger.InfoContext(ctx, "Cluster connection closed")

		if err := conn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
			logger.WarnContext(ctx, "Failed to close remote connection for remote site", "error", err)
		}

		if err := s.srv.onSiteTunnelClose(s); err != nil {
			logger.WarnContext(ctx, "Failed to close remote site", "error", err)
		}
	}()

	offlineThresholdTimer := time.NewTimer(s.offlineThreshold)
	defer offlineThresholdTimer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			logger.InfoContext(ctx, "closing")
			return
		case <-proxyResyncTicker.Chan():
			var req discoveryRequest
			proxies, err := s.srv.proxyWatcher.CurrentResources(s.srv.ctx)
			if err != nil {
				logger.WarnContext(ctx, "Failed to get proxy set", "error", err)
			}
			req.SetProxies(proxies)

			if err := conn.sendDiscoveryRequest(ctx, req); err != nil {
				logger.DebugContext(ctx, "Marking connection invalid on error", "error", err)
				conn.markInvalid(err)
				return
			}
		case proxies := <-conn.newProxiesC:
			var req discoveryRequest
			req.SetProxies(proxies)

			if err := conn.sendDiscoveryRequest(ctx, req); err != nil {
				logger.DebugContext(ctx, "Marking connection invalid on error", "error", err)
				conn.markInvalid(err)
				return
			}
		case req := <-reqC:
			if req == nil {
				logger.InfoContext(ctx, "Cluster agent disconnected")
				conn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
				if !s.HasValidConnections() {
					logger.DebugContext(ctx, "Deleting connection record")
					s.deleteConnectionRecord()
				}
				return
			}
			if firstHeartbeat {
				// as soon as the agent connects and sends a first heartbeat
				// send it the list of current proxies back
				proxies, err := s.srv.proxyWatcher.CurrentResources(ctx)
				if err != nil {
					logger.WarnContext(ctx, "Failed to get proxy set", "error", err)
				}
				if len(proxies) > 0 {
					conn.updateProxies(proxies)
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
				pinglog = pinglog.With("latency", roundtrip)
			}
			pinglog.DebugContext(ctx, "Received ping request", "remote_addr", logutils.StringerAttr(conn.conn.RemoteAddr()))

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
					logger.ErrorContext(ctx, "Closing unhealthy and idle connection", "last_heartbeat", hb)
					return
				}

				logger.WarnContext(ctx, "Deferring closure of unhealthy connection due to active connections", "active_conn_count", count)
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
				s.logger.DebugContext(s.ctx, "Remote cluster does not support cert authorities rotation yet")
			case trace.IsCompareFailed(err):
				s.logger.InfoContext(s.ctx, "Remote cluster has updated certificate authorities, going to force reconnect")
				if err := s.srv.onSiteTunnelClose(&alwaysClose{RemoteSite: s}); err != nil {
					s.logger.WarnContext(s.ctx, "Failed to close remote site", "error", err)
				}
				return
			case trace.IsConnectionProblem(err):
				s.logger.DebugContext(s.ctx, "Remote cluster is offline")
			default:
				s.logger.WarnContext(s.ctx, "Could not perform cert authorities update", "error", err)
			}
		}

		startedWaiting := s.clock.Now()
		select {
		case t := <-retry.After():
			s.logger.DebugContext(s.ctx, "Initiating new cert authority watch after applying backoff", "backoff_duration", t.Sub(startedWaiting))
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
			s.logger.WarnContext(s.ctx, "Failed to close local ca watcher subscription", "error", err)
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
			s.logger.WarnContext(s.ctx, "Failed to close remote ca watcher subscription", "error", err)
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
		s.logger.DebugContext(s.ctx, "Pushed local cert authority", "cert_authority", logutils.StringerAttr(caID))
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
			s.logger.DebugContext(s.ctx, "Ingesting remote cert authority", "cert_authority", logutils.StringerAttr(remoteCA.GetID()))
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

	s.logger.DebugContext(s.ctx, "Watching for cert authority changes")
	for {
		select {
		case <-s.ctx.Done():
			return trace.Wrap(s.ctx.Err())
		case <-localWatch.Done():
			s.logger.WarnContext(s.ctx, "Local CertAuthority watcher subscription has closed")
			return fmt.Errorf("local ca watcher for cluster %s has closed", s.srv.ClusterName)
		case <-remoteWatch.Done():
			s.logger.WarnContext(s.ctx, "Remote CertAuthority watcher subscription has closed")
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
					s.logger.WarnContext(s.ctx, "Failed to rotate external ca", "error", err)
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
	s.logger.DebugContext(s.ctx, "Watching for remote lock changes")

	for {
		startedWaiting := s.clock.Now()
		select {
		case t := <-retry.After():
			s.logger.DebugContext(s.ctx, "Initiating new lock watch after applying backoff", "backoff_duration", t.Sub(startedWaiting))
			retry.Inc()
		case <-s.ctx.Done():
			return
		}

		if err := s.watchLocks(); err != nil {
			switch {
			case trace.IsNotImplemented(err):
				s.logger.DebugContext(s.ctx, "Remote cluster does not support locks yet", "cluster", s.domainName)
			case trace.IsConnectionProblem(err):
				s.logger.DebugContext(s.ctx, "Remote cluster is offline", "cluster", s.domainName)
			default:
				s.logger.WarnContext(s.ctx, "Could not update remote locks", "error", err)
			}
		}
	}
}

func (s *remoteSite) watchLocks() error {
	watcher, err := s.srv.LockWatcher.Subscribe(s.ctx)
	if err != nil {
		s.logger.ErrorContext(s.ctx, "Failed to subscribe to LockWatcher", "error", err)
		return err
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			s.logger.WarnContext(s.ctx, "Failed to close lock watcher subscription", "error", err)
		}
	}()

	for {
		select {
		case <-watcher.Done():
			s.logger.WarnContext(s.ctx, "Lock watcher subscription has closed", "error", watcher.Error())
			return trace.Wrap(watcher.Error())
		case <-s.ctx.Done():
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
	if params.TargetServer == nil && params.ConnType == types.NodeTunnel {
		return nil, trace.BadParameter("target server is required for teleport nodes")
	}

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
	s.logger.DebugContext(s.ctx, "Initiating dial request",
		"source_addr", logutils.StringerAttr(params.From),
		"target_addr", logutils.StringerAttr(params.To),
	)

	isAgentless := params.ConnType == types.NodeTunnel && params.TargetServer != nil && params.TargetServer.IsOpenSSHNode()

	conn, err := s.connThroughTunnel(&sshutils.DialReq{
		Address:         params.To.String(),
		ServerID:        params.ServerID,
		ConnType:        params.ConnType,
		ClientSrcAddr:   stringOrEmpty(params.From),
		ClientDstAddr:   stringOrEmpty(params.OriginalClientDstAddr),
		IsAgentlessNode: isAgentless,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *remoteSite) dialAndForward(params reversetunnelclient.DialParams) (_ net.Conn, retErr error) {
	if params.GetUserAgent == nil && !params.TargetServer.IsOpenSSHNode() {
		return nil, trace.BadParameter("user agent getter is required for teleport nodes")
	}
	s.logger.DebugContext(s.ctx, "Initiating dial and forward request",
		"source_addr", logutils.StringerAttr(params.From),
		"target_addr", logutils.StringerAttr(params.To),
	)

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
		IsAgentlessNode: params.TargetServer.IsOpenSSHNode(),
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
		TargetAddr:               params.To.String(),
		TargetServer:             params.TargetServer,
		Clock:                    s.clock,
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
func UseTunnel(logger *slog.Logger, c *sshutils.ChConn) bool {
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
		logger.DebugContext(context.Background(), "Timed out waiting for response: returning false")
		return false
	}
}

func (s *remoteSite) connThroughTunnel(req *sshutils.DialReq) (*sshutils.ChConn, error) {
	s.logger.DebugContext(s.ctx, "Requesting connection in remote cluster.",
		"target_address", req.Address,
		"target_server_id", req.ServerID,
	)

	// Loop through existing remote connections and try and establish a
	// connection over the "reverse tunnel".
	var conn *sshutils.ChConn
	var err error
	for i := 0; i < s.connectionCount(); i++ {
		conn, err = s.chanTransportConn(req)
		if err == nil {
			return conn, nil
		}
		s.logger.WarnContext(s.ctx, "Request for connection to remote site failed", "error", err)
	}

	// Didn't connect and no error? This means we didn't have any connected
	// tunnels to try.
	if err == nil {
		// Return the appropriate message if the user is trying to connect to a
		// cluster or a node.
		if req.Address != constants.RemoteAuthServer {
			return nil, trace.ConnectionProblem(nil, "node %v is offline", req.Address)
		}
		return nil, trace.ConnectionProblem(nil, "cluster %v is offline", s.GetName())
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
