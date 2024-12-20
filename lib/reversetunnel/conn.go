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
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// connKey is a key used to identify tunnel connections. It contains the UUID
// of the process as well as the type of tunnel. For example, this allows a
// single process to connect multiple reverse tunnels to a proxy, like SSH IoT
// and applications.
type connKey struct {
	// uuid is the host UUID of the process.
	uuid string
	// connType is the type of tunnel, for example: node or application.
	connType types.TunnelType
}

// remoteConn holds a connection to a remote host, either node or proxy.
type remoteConn struct {
	// lastHeartbeat is the last time a heartbeat was received.
	// intentionally placed first to ensure 64-bit alignment
	lastHeartbeat atomic.Int64

	*connConfig
	mu     sync.Mutex
	logger *slog.Logger

	// discoveryCh is the SSH channel over which discovery requests are sent.
	discoveryCh ssh.Channel

	// newProxiesC is a list used to nofity about new proxies
	newProxiesC chan []types.Server

	// invalid indicates the connection is invalid and connections can no longer
	// be made on it.
	invalid atomic.Bool

	// lastError is the last error that occurred before this connection became
	// invalid.
	lastError error

	// Used to make sure calling Close on the connection multiple times is safe.
	closed atomic.Bool

	// clock is used to control time in tests.
	clock clockwork.Clock

	// sessions counts the number of active sessions being serviced by this connection
	sessions atomic.Int64
}

// connConfig is the configuration for the remoteConn.
type connConfig struct {
	// conn is the underlying net.Conn.
	conn net.Conn

	// sconn is the underlying SSH connection.
	sconn ssh.Conn

	// tunnelType is the type of tunnel connection, either proxy or node.
	tunnelType string

	// proxyName is the name of the proxy this remoteConn is located in.
	proxyName string

	// clusterName is the name of the cluster this tunnel is associated with.
	clusterName string

	// nodeID is used when tunnelType is node and is set
	// to the node UUID dialing back
	nodeID string

	// offlineThreshold is how long to wait for a keep alive message before
	// marking a reverse tunnel connection as invalid.
	offlineThreshold time.Duration
}

func newRemoteConn(cfg *connConfig) *remoteConn {
	c := &remoteConn{
		logger:      slog.With(teleport.ComponentKey, "discovery"),
		connConfig:  cfg,
		clock:       clockwork.NewRealClock(),
		newProxiesC: make(chan []types.Server, 100),
	}

	return c
}

func (c *remoteConn) String() string {
	return fmt.Sprintf("remoteConn(remoteAddr=%v)", c.conn.RemoteAddr())
}

func (c *remoteConn) Close() error {
	// If the connection has already been closed, return right away.
	if c.closed.Swap(true) {
		return nil
	}

	var errs []error
	// Close the discovery channel.
	if c.discoveryCh != nil {
		errs = append(errs, c.discoveryCh.Close())
		c.discoveryCh = nil
	}

	// Close the SSH connection which will close the underlying net.Conn as well.
	err := c.sconn.Close()
	if err != nil {
		errs = append(errs, err)
	}

	return trace.NewAggregate(errs...)

}

// OpenChannel will open a SSH channel to the remote side.
func (c *remoteConn) OpenChannel(name string, data []byte) (ssh.Channel, error) {
	channel, reqC, err := c.sconn.OpenChannel(name, data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go ssh.DiscardRequests(reqC)

	return channel, nil
}

// ChannelConn creates a net.Conn over a SSH channel.
func (c *remoteConn) ChannelConn(channel ssh.Channel) net.Conn {
	return sshutils.NewChConn(c.sconn, channel)
}

func (c *remoteConn) incrementActiveSessions() {
	c.sessions.Add(1)
}

func (c *remoteConn) decrementActiveSessions() {
	c.sessions.Add(-1)
}

func (c *remoteConn) activeSessions() int64 {
	return c.sessions.Load()
}

func (c *remoteConn) markInvalid(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastError = err
	c.invalid.Store(true)
	c.logger.WarnContext(context.Background(), "Unhealthy reverse tunnel connection",
		"cluster", c.clusterName,
		"remote_addr", logutils.StringerAttr(c.conn.RemoteAddr()),
		"error", err,
	)
}

func (c *remoteConn) markValid() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastError = nil
	c.invalid.Store(false)
}

func (c *remoteConn) isInvalid() bool {
	return c.invalid.Load()
}

// isOffline determines if the remoteConn has missed
// enough heartbeats to be considered offline. Any active connections
// still being serviced by the connection will cause a true return
// even if the threshold has been exceeded.
func (c *remoteConn) isOffline(now time.Time, threshold time.Duration) bool {
	hb := c.getLastHeartbeat()
	count := c.activeSessions()
	return now.After(hb.Add(threshold)) && count == 0
}

func (c *remoteConn) setLastHeartbeat(tm time.Time) {
	c.lastHeartbeat.Store(tm.UnixNano())
}

func (c *remoteConn) getLastHeartbeat() time.Time {
	hb := c.lastHeartbeat.Load()
	if hb == 0 {
		return time.Time{}
	}

	return time.Unix(0, hb)
}

// isReady returns true when connection is ready to be tried,
// it returns true when connection has received the first heartbeat
func (c *remoteConn) isReady() bool {
	return c.lastHeartbeat.Load() != 0
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

	discoveryCh, reqC, err := c.sconn.OpenChannel(chanDiscovery, nil)
	if err != nil {
		c.markInvalid(err)
		return nil, trace.Wrap(err)
	}
	go ssh.DiscardRequests(reqC)
	c.discoveryCh = discoveryCh
	return c.discoveryCh, nil
}

// updateProxies is a non-blocking call that puts the new proxies
// list so that remote connection can notify the remote agent
// about the list update
func (c *remoteConn) updateProxies(proxies []types.Server) {
	select {
	case c.newProxiesC <- proxies:
	default:
		// Missing proxies update is no longer critical with more permissive
		// discovery protocol that tolerates conflicting, stale or missing updates
		c.logger.WarnContext(context.Background(), "Discovery channel overflow", "new_proxy_count", len(c.newProxiesC))
	}
}

func (c *remoteConn) adviseReconnect() error {
	_, _, err := c.sconn.SendRequest(reconnectRequest, true, nil)
	return trace.Wrap(err)
}

// sendDiscoveryRequest sends a discovery request with up to date
// list of connected proxies
func (c *remoteConn) sendDiscoveryRequest(ctx context.Context, req discoveryRequest) error {
	discoveryCh, err := c.openDiscoveryChannel()
	if err != nil {
		return trace.Wrap(err)
	}

	// Marshal and send the request. If the connection failed, mark the
	// connection as invalid so it will be removed later.
	payload, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// Log the discovery request being sent. Useful for debugging to know what
	// proxies the tunnel server thinks exist.
	c.logger.DebugContext(ctx, "Sending discovery request",
		"proxies", req.ProxyNames(),
		"target_addr", logutils.StringerAttr(c.sconn.RemoteAddr()),
	)

	if _, err := discoveryCh.SendRequest(chanDiscoveryReq, false, payload); err != nil {
		c.markInvalid(err)
		return trace.Wrap(err)
	}

	return nil
}
