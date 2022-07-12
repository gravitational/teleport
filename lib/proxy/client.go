// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	clientapi "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// ClientConfig configures a Client instance.
type ClientConfig struct {
	// Context is a signaling context
	Context context.Context
	// ID is the ID of this server proxy
	ID string
	// AuthClient is an auth client
	AuthClient auth.ClientI
	// AccessPoint is a caching auth client
	AccessPoint auth.ProxyAccessPoint
	// TLSConfig is the proxy client TLS configuration.
	TLSConfig *tls.Config
	// Log is the proxy client logger.
	Log logrus.FieldLogger
	// Clock is used to control connection monitoring ticker.
	Clock clockwork.Clock
	// GracefulShutdownTimout is used set the graceful shutdown
	// duration limit.
	GracefulShutdownTimeout time.Duration

	// getConfigForServer updates the client tls config.
	// configurable for testing purposes.
	getConfigForServer func() (*tls.Config, error)

	// sync runs proxy and connection syncing operations
	// configurable for testing purposes
	sync func()
}

// checkAndSetDefaults checks and sets default values
func (c *ClientConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.New()
	}

	c.Log = c.Log.WithField(
		trace.Component,
		teleport.Component(teleport.ComponentProxyPeer),
	)

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Context == nil {
		c.Context = context.Background()
	}

	if c.GracefulShutdownTimeout == 0 {
		c.GracefulShutdownTimeout = defaults.DefaultGracefulShutdownTimeout
	}

	if c.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}

	if c.AuthClient == nil {
		return trace.BadParameter("missing auth client")
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("missing access cache")
	}

	if c.TLSConfig == nil {
		return trace.BadParameter("missing tls config")
	}

	if len(c.TLSConfig.Certificates) == 0 {
		return trace.BadParameter("missing tls certificate")
	}

	if c.getConfigForServer == nil {
		c.getConfigForServer = getConfigForServer(c.TLSConfig, c.AccessPoint, c.Log)
	}

	return nil
}

// clientConn hold info about a dialed grpc connection
type clientConn struct {
	*grpc.ClientConn
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	id   string
	addr string
}

// Client is a peer proxy service client using grpc and tls.
type Client struct {
	sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	config   ClientConfig
	conns    map[string]*clientConn
	metrics  *clientMetrics
	reporter *reporter
}

// NewClient creats a new peer proxy client.
func NewClient(config ClientConfig) (*Client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	metrics, err := newClientMetrics()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reporter := newReporter(metrics)

	closeContext, cancel := context.WithCancel(config.Context)

	c := &Client{
		config:   config,
		ctx:      closeContext,
		cancel:   cancel,
		conns:    make(map[string]*clientConn),
		metrics:  metrics,
		reporter: reporter,
	}

	go c.monitor()

	if c.config.sync != nil {
		go c.config.sync()
	} else {
		go c.sync()
	}

	return c, nil
}

// monitor monitors the status of peer proxy grpc connections.
func (c *Client) monitor() {
	ticker := c.config.Clock.NewTicker(defaults.ResyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.Chan():
			c.RLock()
			c.reporter.resetConnections()
			for _, conn := range c.conns {
				switch conn.GetState() {
				case connectivity.Idle:
					c.reporter.incConnection(c.config.ID, conn.id, connectivity.Idle.String())
				case connectivity.Connecting:
					c.reporter.incConnection(c.config.ID, conn.id, connectivity.Connecting.String())
				case connectivity.Ready:
					c.reporter.incConnection(c.config.ID, conn.id, connectivity.Ready.String())
				case connectivity.TransientFailure:
					c.reporter.incConnection(c.config.ID, conn.id, connectivity.TransientFailure.String())
				case connectivity.Shutdown:
					c.reporter.incConnection(c.config.ID, conn.id, connectivity.Shutdown.String())
				}
			}
			c.RUnlock()
		}
	}
}

// sync runs the peer proxy watcher functionality.
func (c *Client) sync() {
	proxyWatcher, err := services.NewProxyWatcher(c.ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.Component(teleport.ComponentProxyPeer),
			Client:    c.config.AccessPoint,
			Log:       c.config.Log,
		},
		ProxyDiffer: func(old, new types.Server) bool {
			return old.GetPeerAddr() != new.GetPeerAddr()
		},
	})
	if err != nil {
		c.config.Log.Errorf("Error initializing proxy peer watcher: %+v.", err)
		return
	}
	defer proxyWatcher.Close()

	for {
		select {
		case <-c.ctx.Done():
			c.config.Log.Debug("Stopping peer proxy sync: context done.")
			return
		case <-proxyWatcher.Done():
			c.config.Log.Debug("Stopping peer proxy sync: proxy watcher done.")
			return
		case proxies := <-proxyWatcher.ProxiesC:
			if err := c.updateConnections(proxies); err != nil {
				c.config.Log.Errorf("Error syncing peer proxies: %+v.", err)
			}
		}
	}
}

func (c *Client) updateConnections(proxies []types.Server) error {
	c.RLock()

	toDial := make(map[string]types.Server)
	for _, proxy := range proxies {
		toDial[proxy.GetName()] = proxy
	}

	var toDelete []string
	toKeep := make(map[string]*clientConn)
	for id, conn := range c.conns {
		proxy, ok := toDial[id]

		// delete nonexistent connections
		if !ok {
			toDelete = append(toDelete, id)
			continue
		}

		// peer address changed
		if conn.addr != proxy.GetPeerAddr() {
			toDelete = append(toDelete, id)
			continue
		}

		toKeep[id] = conn
	}

	var errs []error
	for id, proxy := range toDial {
		// skips itself
		if id == c.config.ID {
			continue
		}

		// skip existing connections
		if _, ok := toKeep[id]; ok {
			continue
		}

		// establish new connections
		conn, err := c.connect(id, proxy.GetPeerAddr())
		if err != nil {
			c.metrics.reportTunnelError(errorProxyPeerTunnelDial)
			c.config.Log.Debugf("Error dialing peer proxy %+v at %+v", id, proxy.GetPeerAddr())
			errs = append(errs, err)
			continue
		}

		toKeep[id] = conn
	}
	c.RUnlock()

	c.Lock()
	defer c.Unlock()

	for _, id := range toDelete {
		if conn, ok := c.conns[id]; ok {
			go c.shutdownConn(conn)
		}
	}
	c.conns = toKeep

	return trace.NewAggregate(errs...)
}

// DialNode dials a node through a peer proxy.
func (c *Client) DialNode(
	proxyIDs []string,
	nodeID string,
	src net.Addr,
	dst net.Addr,
	tunnelType types.TunnelType,
) (net.Conn, error) {
	stream, _, err := c.dial(proxyIDs)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "error dialing peer proxies %s", proxyIDs)
	}

	// send dial request as the first frame
	if err = stream.Send(&clientapi.Frame{
		Message: &clientapi.Frame_DialRequest{
			DialRequest: &clientapi.DialRequest{
				NodeID:     nodeID,
				TunnelType: tunnelType,
				Source: &clientapi.NetAddr{
					Addr:    src.String(),
					Network: src.Network(),
				},
				Destination: &clientapi.NetAddr{
					Addr:    dst.String(),
					Network: dst.Network(),
				},
			},
		},
	}); err != nil {
		return nil, trace.ConnectionProblem(err, "error sending dial frame")
	}

	msg, err := stream.Recv()
	if err != nil {
		return nil, trace.ConnectionProblem(err, "error receiving dial response")
	}

	if msg.GetConnectionEstablished() == nil {
		err := stream.CloseSend()
		if err != nil {
			c.config.Log.Debugf("error closing stream: %w", err)
		}
		return nil, trace.ConnectionProblem(nil, "received malformed connection established frame")
	}

	return newStreamConn(stream, src, dst), nil
}

// Shutdown gracefully shuts down all existing client connections.
func (c *Client) Shutdown() {
	c.Lock()
	defer c.Unlock()

	var wg sync.WaitGroup
	for _, conn := range c.conns {
		wg.Add(1)
		go func(conn *clientConn) {
			defer wg.Done()

			timeoutCtx, cancel := context.WithTimeout(context.Background(), c.config.GracefulShutdownTimeout)
			defer cancel()

			go func() {
				if err := c.shutdownConn(conn); err != nil {
					c.config.Log.Infof("proxy peer connection %+v graceful shutdown error: %+v", conn.id, err)
				}
			}()

			select {
			case <-conn.ctx.Done():
			case <-timeoutCtx.Done():
				if err := c.stopConn(conn); err != nil {
					c.config.Log.Infof("proxy peer connection %+v close error: %+v", conn.id, err)
				}
			}
		}(conn)
	}
	wg.Wait()
	c.cancel()
}

// Stop closes all existing client connections.
func (c *Client) Stop() error {
	c.Lock()
	defer c.Unlock()

	var errs []error
	for _, conn := range c.conns {
		if err := c.stopConn(conn); err != nil {
			errs = append(errs, err)
		}
	}
	c.cancel()
	return trace.NewAggregate(errs...)
}

func (c *Client) GetConnectionsCount() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.conns)
}

// shutdownConn gracefully shuts down a clientConn
// by waiting for open streams to finish.
func (c *Client) shutdownConn(conn *clientConn) error {
	conn.wg.Wait() // wait for streams to gracefully end
	conn.cancel()
	return conn.Close()
}

// stopConn immediately closes a clientConn
func (c *Client) stopConn(conn *clientConn) error {
	conn.cancel()
	return conn.Close()
}

// dial opens a new stream to one of the supplied proxy ids.
// it tries to find an existing grpc.ClientConn or initializes a new rpc
// to one of the proxies otherwise.
// The boolean returned in the second argument is intended for testing purposes,
// to indicates whether the connection was cached or newly established.
func (c *Client) dial(proxyIDs []string) (clientapi.ProxyService_DialNodeClient, bool, error) {
	conns, existing, err := c.getConnections(proxyIDs)
	if err != nil {
		return nil, existing, trace.Wrap(err)
	}

	var errs []error
	for _, conn := range conns {
		stream, err := c.startStream(conn)
		if err != nil {
			c.metrics.reportTunnelError(errorProxyPeerTunnelRPC)
			c.config.Log.Debugf("Error opening tunnel rpc to proxy %+v at %+v", conn.id, conn.addr)
			errs = append(errs, err)
			continue
		}

		return stream, existing, nil
	}

	return nil, existing, trace.ConnectionProblem(trace.NewAggregate(errs...), "Error opening tunnel rpcs to all proxies")
}

// getConnections returns connections to the supplied proxy ids.
// it tries to find an existing grpc.ClientConn or initializes a new one
// otherwise.
// The boolean returned in the second argument is intended for testing purposes,
// to indicates whether the connection was cached or newly established.
func (c *Client) getConnections(proxyIDs []string) ([]*clientConn, bool, error) {
	if len(proxyIDs) == 0 {
		return nil, false, trace.BadParameter("failed to dial: no proxy ids given")
	}

	ids := make(map[string]struct{})
	var conns []*clientConn

	// look for existing matching connections.
	c.RLock()
	for _, id := range proxyIDs {
		ids[id] = struct{}{}

		conn, ok := c.conns[id]
		if !ok {
			continue
		}

		conns = append(conns, conn)
	}
	c.RUnlock()

	if len(conns) != 0 {
		return conns, true, nil
	}

	c.metrics.reportTunnelError(errorProxyPeerTunnelNotFound)

	// try to establish new connections otherwise.
	proxies, err := c.config.AuthClient.GetProxies()
	if err != nil {
		c.metrics.reportTunnelError(errorProxyPeerFetchProxies)
		return nil, false, trace.Wrap(err)
	}

	var errs []error
	for _, proxy := range proxies {
		id := proxy.GetName()
		if _, ok := ids[id]; !ok {
			continue
		}

		conn, err := c.connect(id, proxy.GetPeerAddr())
		if err != nil {
			c.metrics.reportTunnelError(errorProxyPeerTunnelDirectDial)
			c.config.Log.Debugf("Error direct dialing peer proxy %+v at %+v", id, proxy.GetPeerAddr())
			errs = append(errs, err)
			continue
		}

		conns = append(conns, conn)
	}

	if len(conns) == 0 {
		c.metrics.reportTunnelError(errorProxyPeerProxiesUnreachable)
		return nil, false, trace.ConnectionProblem(trace.NewAggregate(errs...), "Error dialing all proxies")
	}

	c.Lock()
	defer c.Unlock()

	for _, conn := range conns {
		c.conns[conn.id] = conn
	}

	return conns, false, nil
}

// connect dials a new connection to proxyAddr.
func (c *Client) connect(id string, proxyPeerAddr string) (*clientConn, error) {
	tlsConfig, err := c.config.getConfigForServer()
	if err != nil {
		return nil, trace.Wrap(err, "Error updating client tls config")
	}

	connCtx, cancel := context.WithCancel(c.ctx)
	wg := new(sync.WaitGroup)

	transportCreds := newProxyCredentials(credentials.NewTLS(tlsConfig))
	conn, err := grpc.DialContext(
		connCtx,
		proxyPeerAddr,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithStatsHandler(newStatsHandler(c.reporter)),
		grpc.WithChainStreamInterceptor(metadata.StreamClientInterceptor, utils.GRPCClientStreamErrorInterceptor, streamCounterInterceptor(wg)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                peerKeepAlive,
			Timeout:             peerTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err, "Error dialing proxy %+v", id)
	}

	return &clientConn{
		ClientConn: conn,
		ctx:        connCtx,
		cancel:     cancel,
		wg:         wg,
		id:         id,
		addr:       proxyPeerAddr,
	}, nil
}

// startStream opens a new stream to the provided connection.
func (c *Client) startStream(conn *clientConn) (clientapi.ProxyService_DialNodeClient, error) {
	client := clientapi.NewProxyServiceClient(conn.ClientConn)

	stream, err := client.DialNode(conn.ctx)
	if err != nil {
		return nil, trace.Wrap(err, "Error opening stream to proxy %+v", conn.id)
	}

	return stream, nil
}
