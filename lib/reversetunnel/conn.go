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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
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
	*connConfig
	mu  sync.Mutex
	log *logrus.Entry

	// discoveryCh is the SSH channel over which discovery requests are sent.
	discoveryCh ssh.Channel

	// newProxiesC is a list used to nofity about new proxies
	newProxiesC chan []types.Server

	// invalid indicates the connection is invalid and connections can no longer
	// be made on it.
	invalid int32

	// lastError is the last error that occurred before this connection became
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
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "discovery",
		}),
		connConfig:  cfg,
		clock:       clockwork.NewRealClock(),
		newProxiesC: make(chan []types.Server, 100),
	}

	c.closeContext, c.closeCancel = context.WithCancel(context.Background())

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
	return sshutils.NewChConn(c.sconn, channel)
}

func (c *remoteConn) markInvalid(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	atomic.StoreInt32(&c.invalid, 1)
	c.lastError = err
	c.log.Debugf("Disconnecting connection to %v %v: %v.", c.clusterName, c.conn.RemoteAddr(), err)
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

// updateProxies is a non-blocking call that puts the new proxies
// list so that remote connection can notify the remote agent
// about the list update
func (c *remoteConn) updateProxies(proxies []types.Server) {
	select {
	case c.newProxiesC <- proxies:
	default:
		// Missing proxies update is no longer critical with more permissive
		// discovery protocol that tolerates conflicting, stale or missing updates
		c.log.Warnf("Discovery channel overflow at %v.", len(c.newProxiesC))
	}
}

// sendDiscoveryRequest sends a discovery request with up to date
// list of connected proxies
func (c *remoteConn) sendDiscoveryRequest(req discoveryRequest) error {
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

	// Log the discovery request being sent. Useful for debugging to know what
	// proxies the tunnel server thinks exist.
	names := make([]string, 0, len(req.Proxies))
	for _, proxy := range req.Proxies {
		names = append(names, proxy.GetName())
	}
	c.log.Debugf("Sending %v discovery request with proxies %q to %v.",
		req.Type, names, c.sconn.RemoteAddr())

	_, err = discoveryCh.SendRequest(chanDiscoveryReq, false, payload)
	if err != nil {
		c.markInvalid(err)
		return trace.Wrap(err)
	}

	return nil
}
