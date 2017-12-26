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
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/forward"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	oxyforward "github.com/mailgun/oxy/forward"
	log "github.com/sirupsen/logrus"
)

// remoteSite is a remote site that established the inbound connecton to
// the local reverse tunnel server, and now it can provide access to the
// cluster behind it.
type remoteSite struct {
	sync.RWMutex

	*log.Entry
	domainName  string
	connections []*remoteConn
	lastUsed    int
	lastActive  time.Time
	srv         *server
	transport   *http.Transport
	connInfo    services.TunnelConnection
	ctx         context.Context
	clock       clockwork.Clock

	// certificateCache caches host certificates for the forwarding server.
	certificateCache *certificateCache

	// localClient provides access to the Auth Server API of the cluster
	// within which reversetunnel.Server is running.
	localClient auth.ClientI
	// remoteClient provides access to the Auth Server API of the remote cluster that
	// this site belongs to.
	remoteClient auth.ClientI
	// localAccessPoint provides access to a cached subset of the Auth Server API of
	// the local cluster.
	localAccessPoint auth.AccessPoint
	// remoteAccessPoint provides access to a cached subset of the Auth Server API of
	// the remote cluster this site belongs to.
	remoteAccessPoint auth.AccessPoint
}

func (s *remoteSite) getRemoteClient() (auth.ClientI, bool, error) {
	// check if all cert authorities are initiated and if everything is OK
	ca, err := s.srv.localAccessPoint.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: s.domainName}, false)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	keys := ca.GetTLSKeyPairs()
	// the fact that cluster has keys to remote CA means that the key exchange has completed
	if len(keys) != 0 {
		s.Debugf("Using TLS client to remote cluster.")
		pool, err := services.CertPool(ca)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		tlsConfig := s.srv.ClientTLS.Clone()
		tlsConfig.RootCAs = pool
		clt, err := auth.NewTLSClientWithDialer(s.authServerContextDialer, tlsConfig)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		return clt, false, nil
	}
	// create legacy client that will continue to perform certificate
	// exchange attempts
	s.Debugf("Created legacy SSH client to remote cluster.")
	clt, err := auth.NewClient("http://stub:0", s.dialAccessPoint)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	return clt, true, nil
}

func (s *remoteSite) authServerContextDialer(ctx context.Context, network, address string) (net.Conn, error) {
	return s.DialAuthServer()
}

func (s *remoteSite) CachingAccessPoint() (auth.AccessPoint, error) {
	return s.remoteAccessPoint, nil
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

func (s *remoteSite) hasValidConnections() bool {
	s.RLock()
	defer s.RUnlock()

	for _, conn := range s.connections {
		if !conn.isInvalid() {
			return true
		}
	}
	return false
}

// Clos closes remote cluster connections
func (s *remoteSite) Close() error {
	s.Lock()
	defer s.Unlock()

	for i := range s.connections {
		s.connections[i].Close()
	}
	s.connections = []*remoteConn{}
	return nil
}

func (s *remoteSite) nextConn() (*remoteConn, error) {
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

// addConn helper adds a new active remote cluster connection to the list
// of such connections
func (s *remoteSite) addConn(conn net.Conn, sshConn ssh.Conn) (*remoteConn, error) {
	rc := &remoteConn{
		sshConn: sshConn,
		conn:    conn,
		log:     s.Entry,
	}

	s.Lock()
	defer s.Unlock()

	s.connections = append(s.connections, rc)
	s.lastUsed = 0
	return rc, nil
}

func (s *remoteSite) getLatestTunnelConnection() (services.TunnelConnection, error) {
	conns, err := s.localAccessPoint.GetTunnelConnections(s.domainName)
	if err != nil {
		s.Warningf("failed to fetch tunnel statuses: %v", err)
		return nil, trace.Wrap(err)
	}
	var lastConn services.TunnelConnection
	for i := range conns {
		conn := conns[i]
		if lastConn == nil || conn.GetLastHeartbeat().After(lastConn.GetLastHeartbeat()) {
			lastConn = conn
		}
	}
	if lastConn == nil {
		return nil, trace.NotFound("no connections found")
	}
	return lastConn, nil
}

func (s *remoteSite) GetStatus() string {
	connInfo, err := s.getLatestTunnelConnection()
	if err != nil {
		return RemoteSiteStatusOffline
	}
	if s.isOnline(connInfo) {
		return RemoteSiteStatusOnline
	}
	return RemoteSiteStatusOffline
}

func (s *remoteSite) copyConnInfo() services.TunnelConnection {
	s.RLock()
	defer s.RUnlock()
	return s.connInfo.Clone()
}

func (s *remoteSite) registerHeartbeat(t time.Time) {
	connInfo := s.copyConnInfo()
	connInfo.SetLastHeartbeat(t)
	connInfo.SetExpiry(s.clock.Now().Add(defaults.ReverseTunnelOfflineThreshold))
	err := s.localAccessPoint.UpsertTunnelConnection(connInfo)
	if err != nil {
		s.Warningf("failed to register heartbeat: %v", err)
	}
}

// deleteConnectionRecord deletes connection record to let know peer proxies
// that this node lost the connection and needs to be discovered
func (s *remoteSite) deleteConnectionRecord() {
	s.localAccessPoint.DeleteTunnelConnection(s.connInfo.GetClusterName(), s.connInfo.GetName())
}

// handleHearbeat receives heartbeat messages from the connected agent
// if the agent has missed several heartbeats in a row, Proxy marks
// the connection as invalid.
func (s *remoteSite) handleHeartbeat(conn *remoteConn, ch ssh.Channel, reqC <-chan *ssh.Request) {
	defer func() {
		s.Infof("cluster connection closed")
		conn.Close()
	}()
	for {
		select {
		case <-s.ctx.Done():
			s.Infof("closing")
			return
		case req := <-reqC:
			if req == nil {
				s.Infof("cluster agent disconnected")
				conn.markInvalid(trace.ConnectionProblem(nil, "agent disconnected"))
				if !s.hasValidConnections() {
					s.Debugf("deleting connection record")
					s.deleteConnectionRecord()
				}
				return
			}
			var timeSent time.Time
			var roundtrip time.Duration
			if req.Payload != nil {
				if err := timeSent.UnmarshalText(req.Payload); err == nil {
					roundtrip = s.srv.Clock.Now().Sub(timeSent)
				}
			}
			if roundtrip != 0 {
				s.WithFields(log.Fields{"latency": roundtrip}).Debugf("ping <- %v", conn.conn.RemoteAddr())
			} else {
				s.Debugf("ping <- %v", conn.conn.RemoteAddr())
			}
			go s.registerHeartbeat(time.Now())
		// since we block on select, time.After is re-created everytime we process a request.
		case <-time.After(defaults.ReverseTunnelOfflineThreshold):
			conn.markInvalid(trace.ConnectionProblem(nil, "no heartbeats for %v", defaults.ReverseTunnelOfflineThreshold))
		}
	}
}

func (s *remoteSite) GetName() string {
	return s.domainName
}

func (s *remoteSite) GetLastConnected() time.Time {
	connInfo, err := s.getLatestTunnelConnection()
	if err != nil {
		return time.Time{}
	}
	return connInfo.GetLastHeartbeat()
}

func (s *remoteSite) periodicSendDiscoveryRequests() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()
	if err := s.sendDiscoveryRequest(); err != nil {
		s.Warningf("failed to send discovery: %v", err)
	}
	for {
		select {
		case <-s.ctx.Done():
			s.Debugf("closing")
			return
		case <-ticker.C:
			err := s.sendDiscoveryRequest()
			if err != nil {
				s.Warningf("could not send discovery request: %v", trace.DebugReport(err))
			}
		}
	}
}

// DELETE IN: 2.6.0
// attemptCertExchange tries to exchange TLS certificates with remote
// clusters that have upgraded to 2.5.0
func (s *remoteSite) attemptCertExchange() error {
	// this logic is explicitly using the local non cached
	// client as it has to have write access to the auth server
	localCA, err := s.localClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.srv.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	re, err := s.remoteClient.ExchangeCerts(auth.ExchangeCertsRequest{
		PublicKey: localCA.GetCheckingKeys()[0],
		TLSCert:   localCA.GetTLSKeyPairs()[0].Cert,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	remoteCA, err := s.localClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: s.domainName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.localClient.ExchangeCerts(auth.ExchangeCertsRequest{
		PublicKey: remoteCA.GetCheckingKeys()[0],
		TLSCert:   re.TLSCert,
	})
	return err
}

// DELETE IN: 2.6.0
// This logic is only relevant for upgrades from 2.5.0 to 2.6.0
func (s *remoteSite) periodicAttemptCertExchange() {
	ticker := time.NewTicker(defaults.NetworkBackoffDuration)
	defer ticker.Stop()
	if err := s.attemptCertExchange(); err != nil {
		s.Warningf("Attempt at cert exchange failed: %v.", err)
	} else {
		s.Debugf("Certificate exchange has completed, going to force reconnect.")
		s.srv.RemoveSite(s.domainName)
		s.Close()
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			s.Debugf("Context is closing.")
			return
		case <-ticker.C:
			err := s.attemptCertExchange()
			if err != nil {
				s.Warningf("Could not perform certificate exchange: %v.", trace.DebugReport(err))
			} else {
				s.Debugf("Certificate exchange has completed, going to force reconnect.")
				s.srv.RemoveSite(s.domainName)
				s.Close()
				return
			}
		}
	}
}

func (s *remoteSite) isOnline(conn services.TunnelConnection) bool {
	diff := s.clock.Now().Sub(conn.GetLastHeartbeat())
	return diff < defaults.ReverseTunnelOfflineThreshold
}

// findDisconnectedProxies finds proxies that do not have inbound reverse tunnel
// connections
func (s *remoteSite) findDisconnectedProxies() ([]services.Server, error) {
	connInfo := s.copyConnInfo()
	conns, err := s.localAccessPoint.GetTunnelConnections(s.domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connected := make(map[string]bool)
	for _, conn := range conns {
		if s.isOnline(conn) {
			connected[conn.GetProxyName()] = true
		}
	}
	proxies, err := s.localAccessPoint.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var missing []services.Server
	for i := range proxies {
		proxy := proxies[i]
		// do not add this proxy to the list of disconnected proxies
		if !connected[proxy.GetName()] && proxy.GetName() != connInfo.GetProxyName() {
			missing = append(missing, proxy)
		}
	}
	return missing, nil
}

// sendDiscovery requests sends special "Discovery requests"
// back to the connected agent.
// Discovery request consists of the proxies that are part
// of the cluster, but did not receive the connection from the agent.
// Agent will act on a discovery request attempting
// to establish connection to the proxies that were not discovered.
// See package documentation for more details.
func (s *remoteSite) sendDiscoveryRequest() error {
	disconnectedProxies, err := s.findDisconnectedProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(disconnectedProxies) == 0 {
		return nil
	}
	clusterName, err := s.localAccessPoint.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}
	connInfo := s.copyConnInfo()
	s.Debugf("proxy %q is going to request discovery for: %q", connInfo.GetProxyName(), Proxies(disconnectedProxies))
	req := discoveryRequest{
		ClusterName: clusterName,
		Proxies:     disconnectedProxies,
	}
	payload, err := marshalDiscoveryRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}
	send := func() error {
		remoteConn, err := s.nextConn()
		if err != nil {
			return trace.Wrap(err)
		}
		discoveryC, err := remoteConn.openDiscoveryChannel()
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = discoveryC.SendRequest("discovery", false, payload)
		if err != nil {
			remoteConn.markInvalid(err)
			s.Errorf("disconnecting cluster on %v, err: %v",
				remoteConn.conn.RemoteAddr(),
				err)
			return trace.Wrap(err)
		}
		return nil
	}

	// loop over existing connections (reverse tunnels) and try to send discovery
	// requests to the remote cluster
	for i := 0; i < s.connectionCount(); i++ {
		err := send()
		if err != nil {
			s.Warningf("%v", err)
		}
	}
	return nil
}

// dialAccessPoint establishes a connection from the proxy (reverse tunnel server)
// back into the client using previously established tunnel.
func (s *remoteSite) dialAccessPoint(network, addr string) (net.Conn, error) {
	try := func() (net.Conn, error) {
		remoteConn, err := s.nextConn()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ch, _, err := remoteConn.sshConn.OpenChannel(chanAccessPoint, nil)
		if err != nil {
			remoteConn.markInvalid(err)
			s.Errorf("disconnecting cluster on %v, err: %v",
				remoteConn.conn.RemoteAddr(),
				err)
			return nil, trace.Wrap(err)
		}
		s.Debugf("success dialing to cluster")
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

func (s *remoteSite) DialAuthServer() (conn net.Conn, err error) {
	return s.connThroughTunnel(chanTransportDialReq, RemoteAuthServer)
}

// Dial is used to connect a requesting client (say, tsh) to an SSH server
// located in a remote connected site, the connection goes through the
// reverse proxy tunnel.
func (s *remoteSite) Dial(from net.Addr, to net.Addr, userAgent agent.Agent) (net.Conn, error) {
	clusterConfig, err := s.localAccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if the proxy is in recording mode use the agent to dial and build a
	// in-memory forwarding server
	if clusterConfig.GetSessionRecording() == services.RecordAtProxy {
		if userAgent == nil {
			return nil, trace.BadParameter("user agent missing")
		}
		return s.dialWithAgent(from, to, userAgent)
	}
	return s.dial(from, to)
}

func (s *remoteSite) dial(from, to net.Addr) (net.Conn, error) {
	s.Debugf("Dialing from %v to %v", from, to)

	conn, err := s.connThroughTunnel(chanTransportDialReq, to.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *remoteSite) dialWithAgent(from, to net.Addr, userAgent agent.Agent) (net.Conn, error) {
	s.Debugf("Dialing with an agent from %v to %v", from, to)

	// get a host certificate for the forwarding node from the cache
	hostCertificate, err := s.certificateCache.GetHostCertificate(to.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetConn, err := s.connThroughTunnel(chanTransportDialReq, to.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create a forwarding server that serves a single ssh connection on it. we
	// don't need to close this server it will close and release all resources
	// once conn is closed.
	//
	// note, a localClient is passed to the forwarding server, that's to make
	// sure that the session gets recorded in the local cluster instead of the
	// remote cluster.
	serverConfig := forward.ServerConfig{
		ID:              s.srv.Config.ID,
		AuthClient:      s.localClient,
		UserAgent:       userAgent,
		TargetConn:      targetConn,
		SrcAddr:         from,
		DstAddr:         to,
		HostCertificate: hostCertificate,
		Ciphers:         s.srv.Config.Ciphers,
		KEXAlgorithms:   s.srv.Config.KEXAlgorithms,
		MACAlgorithms:   s.srv.Config.MACAlgorithms,
	}
	remoteServer, err := forward.New(serverConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go remoteServer.Serve()

	// return a connection to the forwarding server
	conn, err := remoteServer.Dial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func (s *remoteSite) connThroughTunnel(transportType string, data string) (conn net.Conn, err error) {
	var stop bool

	s.Debugf("Requesting %v connection to remote site with payload: %v", transportType, data)

	// loop through existing TCP/IP connections (reverse tunnels) and try
	// to establish an inbound connection-over-ssh-channel to the remote
	// cluster (AKA "remotetunnel agent"):
	for i := 0; i < s.connectionCount() && !stop; i++ {
		conn, stop, err = s.chanTransportConn(transportType, data)
		if err == nil {
			return conn, nil
		}
		s.Warnf("Request for %v connection to remote site failed: %v", transportType, err)
	}
	// didn't connect and no error? this means we didn't have any connected
	// tunnels to try
	if err == nil {
		err = trace.ConnectionProblem(nil, "%v is offline", s.GetName())
	}
	return nil, err
}

func (s *remoteSite) chanTransportConn(transportType string, addr string) (net.Conn, bool, error) {
	var stop bool

	remoteConn, err := s.nextConn()
	if err != nil {
		return nil, stop, trace.Wrap(err)
	}
	var ch ssh.Channel
	ch, _, err = remoteConn.sshConn.OpenChannel(chanTransport, nil)
	if err != nil {
		remoteConn.markInvalid(err)
		return nil, stop, trace.Wrap(err)
	}
	// send a special SSH out-of-band request called "teleport-transport"
	// the agent on the other side will create a new TCP/IP connection to
	// 'addr' on its network and will start proxying that connection over
	// this SSH channel:
	var dialed bool
	dialed, err = ch.SendRequest(transportType, true, []byte(addr))
	if err != nil {
		return nil, stop, trace.Wrap(err)
	}
	stop = true
	if !dialed {
		defer ch.Close()
		// pull the error message from the tunnel client (remote cluster)
		// passed to us via stderr:
		errMessage, _ := ioutil.ReadAll(ch.Stderr())
		if errMessage == nil {
			errMessage = []byte("failed connecting to " + addr)
		}
		return nil, stop, trace.Errorf(strings.TrimSpace(string(errMessage)))
	}
	return utils.NewChConn(remoteConn.sshConn, ch), stop, nil
}

func (s *remoteSite) handleAuthProxy(w http.ResponseWriter, r *http.Request) {
	s.Debugf("handleAuthProxy()")

	fwd, err := oxyforward.New(oxyforward.RoundTripper(s.transport), oxyforward.Logger(s.Entry))
	if err != nil {
		roundtrip.ReplyJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	r.URL.Scheme = "http"
	r.URL.Host = "stub"
	fwd.ServeHTTP(w, r)
}
