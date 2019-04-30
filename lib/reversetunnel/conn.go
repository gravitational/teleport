/*
Copyright 2019 Gravitational, Inc.

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
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// remoteConn holds a connection to a remote host, either node or proxy.
type remoteConn struct {
	*connConfig
	mu  sync.Mutex
	log *logrus.Entry

	// discoveryCh is the channel over which discovery requests are sent.
	discoveryCh ssh.Channel

	// invalid indicates the connection is invalid and connections can no longer
	// be made on it.
	invalid int32

	// lastError is the last error that occured before this connection became
	// invalid.
	lastError error

	// Used to make sure calling Close on the connection multiple times is safe.
	closed int32

	// closeContext and closeCancel are used to signal to any waiting goroutines
	// that the remoteConn is now closed and to release any resources.
	closeContext context.Context
	closeCancel  context.CancelFunc

	// clock is used to control time in tests.
	clock clockwork.Clock

	// lastHeartbeat is the last time a heartbeat was received.
	lastHeartbeat int64
}

// connConfig is the configuration for the remoteConn.
type connConfig struct {
	// conn is the underlying net.Conn.
	conn net.Conn

	// sconn is the underlying SSH connection.
	sconn ssh.Conn

	// accessPoint provides access to the Auth Server API.
	accessPoint auth.AccessPoint

	// tunnelID is the tunnel ID. This is either the cluster name (trusted
	// clusters) or hostUUID.clusterName (nodes dialing back).
	tunnelID string

	// tunnelType is the type of tunnel connection, either proxy or node.
	tunnelType string

	// proxyName is the name of the proxy this remoteConn is located in.
	proxyName string

	// clusterName is the name of the cluster this tunnel is associated with.
	clusterName string
}

func newRemoteConn(cfg *connConfig) *remoteConn {
	c := &remoteConn{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "discovery",
		}),
		connConfig: cfg,
		clock:      clockwork.NewRealClock(),
	}

	c.closeContext, c.closeCancel = context.WithCancel(context.Background())

	// Continue sending periodic discovery requests over this connection so
	// that all local proxies can be discovered.
	go c.periodicSendDiscoveryRequests()

	return c
}

func (c *remoteConn) String() string {
	return fmt.Sprintf("remoteConn(remoteAddr=%v)", c.conn.RemoteAddr())
}

func (c *remoteConn) Close() error {
	defer c.closeCancel()

	// If the connection has already been closed, return right away.
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	// Close the discovery channel.
	if c.discoveryCh != nil {
		c.discoveryCh.Close()
		c.discoveryCh = nil
	}

	// Close the SSH connection which will close the underlying net.Conn as well.
	err := c.sconn.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil

}

// OpenChannel will open a SSH channel to the remote side.
func (c *remoteConn) OpenChannel(name string, data []byte) (ssh.Channel, error) {
	channel, _, err := c.sconn.OpenChannel(name, data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return channel, nil
}

// ChannelConn creates a net.Conn over a SSH channel.
func (c *remoteConn) ChannelConn(channel ssh.Channel) net.Conn {
	return utils.NewChConn(c.sconn, channel)
}

func (c *remoteConn) markInvalid(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	atomic.StoreInt32(&c.invalid, 1)
	c.lastError = err
	c.log.Errorf("Disconnecting connection to %v: %v.", c.conn.RemoteAddr(), err)
}

func (c *remoteConn) isInvalid() bool {
	return atomic.LoadInt32(&c.invalid) == 1
}

func (c *remoteConn) setLastHeartbeat(tm time.Time) {
	atomic.StoreInt64(&c.lastHeartbeat, tm.UnixNano())
}

// isReady returns true when connection is ready to be tried,
// it returns true when connection has received the first heartbeat
func (c *remoteConn) isReady() bool {
	return atomic.LoadInt64(&c.lastHeartbeat) != 0
}

func (c *remoteConn) openDiscoveryChannel() (ssh.Channel, error) {
	var err error

	if c.isInvalid() {
		return nil, trace.Wrap(c.lastError)
	}

	// If a discovery channel has already been opened, return it right away.
	if c.discoveryCh != nil {
		return c.discoveryCh, nil
	}

	c.discoveryCh, _, err = c.sconn.OpenChannel(chanDiscovery, nil)
	if err != nil {
		c.markInvalid(err)
		return nil, trace.Wrap(err)
	}
	return c.discoveryCh, nil
}

func (c *remoteConn) periodicSendDiscoveryRequests() {
	ticker := time.NewTicker(defaults.ReverseTunnelAgentHeartbeatPeriod)
	defer ticker.Stop()

	if err := c.findAndSend(); err != nil {
		c.log.Warnf("Failed to send discovery request: %v.", err)
	}

	for {
		select {
		case <-ticker.C:
			err := c.findAndSend()
			if err != nil {
				c.log.Warnf("Failed to send discovery request: %v.", err)
			}
		case <-c.closeContext.Done():
			return
		}
	}
}

// sendDiscovery requests sends special "Discovery Requests" back to the
// connected agent. Discovery request consists of the proxies that are part
// of the cluster, but did not receive the connection from the agent. Agent
// will act on a discovery request attempting to establish connection to the
// proxies that were not discovered.
func (c *remoteConn) findAndSend() error {
	// Find all proxies that don't have a connection to a remote agent. If all
	// proxies have connections, return right away.
	disconnectedProxies, err := c.findDisconnectedProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(disconnectedProxies) == 0 {
		return nil
	}

	c.log.Debugf("Proxy %v sending %v discovery request with tunnel ID: %v and disconnected proxies: %v.",
		c.proxyName, string(c.tunnelType), c.tunnelID, Proxies(disconnectedProxies))

	req := discoveryRequest{
		TunnelID: c.tunnelID,
		Type:     string(c.tunnelType),
		Proxies:  disconnectedProxies,
	}

	err = c.sendDiscoveryRequests(req)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// findDisconnectedProxies finds proxies that do not have inbound reverse tunnel
// connections.
func (c *remoteConn) findDisconnectedProxies() ([]services.Server, error) {
	// Find all proxies that have connection from the remote domain.
	conns, err := c.accessPoint.GetTunnelConnections(c.clusterName, services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connected := make(map[string]bool)
	for _, conn := range conns {
		if c.isOnline(conn) {
			connected[conn.GetProxyName()] = true
		}
	}

	// Build a list of local proxies that do not have a remote connection to them.
	var missing []services.Server
	proxies, err := c.accessPoint.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range proxies {
		proxy := proxies[i]

		// A proxy should never add itself to the list of missing proxies.
		if proxy.GetName() == c.proxyName {
			continue
		}

		if !connected[proxy.GetName()] {
			missing = append(missing, proxy)
		}
	}

	return missing, nil
}

// sendDiscoveryRequests sends a discovery request with missing proxies.
func (c *remoteConn) sendDiscoveryRequests(req discoveryRequest) error {
	discoveryCh, err := c.openDiscoveryChannel()
	if err != nil {
		return trace.Wrap(err)
	}

	// Marshal and send the request. If the connection failed, mark the
	// connection as invalid so it will be removed later.
	payload, err := marshalDiscoveryRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = discoveryCh.SendRequest(chanDiscoveryReq, false, payload)
	if err != nil {
		c.markInvalid(err)
		return trace.Wrap(err)
	}

	return nil
}

func (c *remoteConn) isOnline(conn services.TunnelConnection) bool {
	return services.TunnelConnectionStatus(c.clock, conn) == teleport.RemoteClusterStatusOnline
}
