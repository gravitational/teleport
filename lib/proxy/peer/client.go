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

package peer

import (
	"context"
	"crypto/tls"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/gravitational/teleport"
	clientapi "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
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
	// ClusterName is the name of the cluster.
	ClusterName string

	// getConfigForServer updates the client tls config.
	// configurable for testing purposes.
	getConfigForServer func() (*tls.Config, error)

	// connShuffler determines the order client connections will be used.
	connShuffler connShuffler

	// sync runs proxy and connection syncing operations
	// configurable for testing purposes
	sync func()
}

// connShuffler shuffles the order of client connections.
type connShuffler func([]*clientConn)

// randomConnShuffler returns a conn shuffler that randomizes the order of connections.
func randomConnShuffler() connShuffler {
	return func(conns []*clientConn) {
		rand.Shuffle(len(conns), func(i, j int) {
			conns[i], conns[j] = conns[j], conns[i]
		})
	}
}

// noopConnShutffler returns a conn shuffler that keeps the original connection ordering.
func noopConnShuffler() connShuffler {
	return func([]*clientConn) {}
}

// checkAndSetDefaults checks and sets default values
func (c *ClientConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.New()
	}

	c.Log = c.Log.WithField(
		teleport.ComponentKey,
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

	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}

	if c.TLSConfig == nil {
		return trace.BadParameter("missing tls config")
	}

	if len(c.TLSConfig.Certificates) == 0 {
		return trace.BadParameter("missing tls certificate")
	}

	if c.connShuffler == nil {
		c.connShuffler = randomConnShuffler()
	}

	if c.getConfigForServer == nil {
		c.getConfigForServer = getConfigForServer(c.TLSConfig, c.AccessPoint, c.Log, c.ClusterName)
	}

	return nil
}

// clientConn hold info about a dialed grpc connection
type clientConn struct {
	*grpc.ClientConn

	id   string
	addr string

	// if closing is set, count is not allowed to increase from zero; upon
	// reaching zero, cond should be broadcast
	mu      sync.Mutex
	cond    sync.Cond
	closing bool
	count   int
}

func (c *clientConn) maybeAcquire() (release func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closing && c.count < 1 {
		return nil
	}
	c.count++

	return sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.count--
		if c.count == 0 {
			c.cond.Broadcast()
		}
	})
}

// Shutdown closes the clientConn after all connections through it are closed,
// or after the context is done.
func (c *clientConn) Shutdown(ctx context.Context) {
	defer c.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.closing = true
	if c.count == 0 {
		return
	}

	if c.cond.L == nil {
		c.cond.L = &c.mu
	}
	defer context.AfterFunc(ctx, c.cond.Broadcast)()
	for c.count > 0 && ctx.Err() == nil {
		c.cond.Wait()
	}
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
			go conn.Shutdown(c.ctx)
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
	stream, _, err := c.dial(proxyIDs, &clientapi.DialRequest{
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
	})
	if err != nil {
		return nil, trace.ConnectionProblem(err, "error dialing peer proxies %s: %v", proxyIDs, err)
	}

	streamRW, err := streamutils.NewReadWriter(stream)
	if err != nil {
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	return streamutils.NewConn(streamRW, src, dst), nil
}

// stream is the common subset of the [clientapi.ProxyService_DialNodeClient] and
// [clientapi.ProxyService_DialNodeServer] interfaces.
type stream interface {
	Send(*clientapi.Frame) error
	Recv() (*clientapi.Frame, error)
}

// frameStream implements [streamutils.Source].
type frameStream struct {
	stream stream
	cancel context.CancelFunc
}

func (s frameStream) Send(p []byte) error {
	return trace.Wrap(s.stream.Send(&clientapi.Frame{Message: &clientapi.Frame_Data{Data: &clientapi.Data{Bytes: p}}}))
}

func (s frameStream) Recv() ([]byte, error) {
	frame, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if frame.GetData() == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return frame.GetData().Bytes, nil
}

func (s frameStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// Shutdown gracefully shuts down all existing client connections.
func (c *Client) Shutdown(ctx context.Context) {
	c.Lock()
	defer c.Unlock()

	var wg sync.WaitGroup
	for _, conn := range c.conns {
		wg.Add(1)
		go func(conn *clientConn) {
			defer wg.Done()
			conn.Shutdown(ctx)
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
		if err := conn.Close(); err != nil {
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

// dial opens a new stream to one of the supplied proxy ids.
// it tries to find an existing grpc.ClientConn or initializes a new rpc
// to one of the proxies otherwise.
// The boolean returned in the second argument is intended for testing purposes,
// to indicates whether the connection was cached or newly established.
func (c *Client) dial(proxyIDs []string, dialRequest *clientapi.DialRequest) (frameStream, bool, error) {
	conns, existing, err := c.getConnections(proxyIDs)
	if err != nil {
		return frameStream{}, existing, trace.Wrap(err)
	}

	var errs []error
	for _, conn := range conns {
		release := conn.maybeAcquire()
		if release == nil {
			c.metrics.reportTunnelError(errorProxyPeerTunnelRPC)
			errs = append(errs, trace.ConnectionProblem(nil, "error starting stream: connection is shutting down"))
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		context.AfterFunc(ctx, release)

		stream, err := clientapi.NewProxyServiceClient(conn.ClientConn).DialNode(ctx)
		if err != nil {
			cancel()
			c.metrics.reportTunnelError(errorProxyPeerTunnelRPC)
			c.config.Log.Debugf("Error opening tunnel rpc to proxy %+v at %+v", conn.id, conn.addr)
			errs = append(errs, trace.ConnectionProblem(err, "error starting stream: %v", err))
			continue
		}

		err = stream.Send(&clientapi.Frame{
			Message: &clientapi.Frame_DialRequest{
				DialRequest: dialRequest,
			},
		})
		if err != nil {
			cancel()
			errs = append(errs, trace.ConnectionProblem(err, "error sending dial frame: %v", err))
			continue
		}
		msg, err := stream.Recv()
		if err != nil {
			cancel()
			errs = append(errs, trace.ConnectionProblem(err, "error receiving dial response: %v", err))
			continue
		}
		if msg.GetConnectionEstablished() == nil {
			cancel()
			errs = append(errs, trace.ConnectionProblem(nil, "received malformed connection established frame"))
			continue
		}

		return frameStream{
			stream: stream,
			cancel: cancel,
		}, existing, nil
	}

	return frameStream{}, existing, trace.NewAggregate(errs...)
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
		c.config.connShuffler(conns)
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

	c.config.connShuffler(conns)
	return conns, false, nil
}

// connect dials a new connection to proxyAddr.
func (c *Client) connect(peerID string, peerAddr string) (*clientConn, error) {
	tlsConfig, err := c.config.getConfigForServer()
	if err != nil {
		return nil, trace.Wrap(err, "Error updating client tls config")
	}

	expectedPeer := auth.HostFQDN(peerID, c.config.ClusterName)

	conn, err := grpc.Dial(
		peerAddr,
		grpc.WithTransportCredentials(newClientCredentials(expectedPeer, peerAddr, c.config.Log, credentials.NewTLS(tlsConfig))),
		grpc.WithStatsHandler(newStatsHandler(c.reporter)),
		grpc.WithChainStreamInterceptor(metadata.StreamClientInterceptor, interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                peerKeepAlive,
			Timeout:             peerTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		return nil, trace.Wrap(err, "Error dialing proxy %q", peerID)
	}

	return &clientConn{
		ClientConn: conn,

		id:   peerID,
		addr: peerAddr,
	}, nil
}
