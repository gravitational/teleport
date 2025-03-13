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
	"log/slog"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/quic-go/quic-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/gravitational/teleport"
	clientapi "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/proxy/peer/internal"
	peerquic "github.com/gravitational/teleport/lib/proxy/peer/quic"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessPoint is the subset of the auth cache consumed by the [Client].
type AccessPoint interface {
	types.Events
	services.ProxyGetter
}

// ClientConfig configures a Client instance.
type ClientConfig struct {
	// Context is a signaling context
	Context context.Context
	// ID is the ID of this server proxy
	ID string
	// AuthClient is an auth client
	AuthClient authclient.ClientI
	// AccessPoint is a caching auth client
	AccessPoint AccessPoint
	// GetTLSCertificate returns a the client TLS certificate to use when
	// connecting to other proxies.
	GetTLSCertificate utils.GetCertificateFunc
	// GetTLSRoots returns a certificate pool used to validate TLS connections
	// to other proxies.
	GetTLSRoots utils.GetRootsFunc
	// TLSCipherSuites optionally contains a list of TLS ciphersuites to use.
	TLSCipherSuites []uint16
	// Log is the proxy client logger.
	Log *slog.Logger
	// Clock is used to control connection monitoring ticker.
	Clock clockwork.Clock
	// GracefulShutdownTimout is used set the graceful shutdown
	// duration limit.
	GracefulShutdownTimeout time.Duration
	// ClusterName is the name of the cluster.
	ClusterName string
	// QUICTransport, if set, will be used to dial peer proxies that advertise
	// support for peering connections over QUIC.
	QUICTransport *quic.Transport

	// connShuffler determines the order client connections will be used.
	connShuffler connShuffler

	// sync runs proxy and connection syncing operations
	// configurable for testing purposes
	sync func()
}

// connShuffler shuffles the order of client connections.
type connShuffler func([]internal.ClientConn)

// randomConnShuffler returns a conn shuffler that randomizes the order of connections.
func randomConnShuffler() connShuffler {
	return func(conns []internal.ClientConn) {
		rand.Shuffle(len(conns), func(i, j int) {
			conns[i], conns[j] = conns[j], conns[i]
		})
	}
}

// noopConnShutffler returns a conn shuffler that keeps the original connection ordering.
func noopConnShuffler() connShuffler {
	return func([]internal.ClientConn) {}
}

// checkAndSetDefaults checks and sets default values
func (c *ClientConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.Default()
	}

	c.Log = c.Log.With(
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

	if c.GetTLSCertificate == nil {
		return trace.BadParameter("missing tls certificate getter")
	}
	if c.GetTLSRoots == nil {
		return trace.BadParameter("missing tls roots getter")
	}

	if c.connShuffler == nil {
		c.connShuffler = randomConnShuffler()
	}

	return nil
}

// grpcClientConn manages client connections to a specific peer proxy over gRPC.
type grpcClientConn struct {
	cc      *grpc.ClientConn
	metrics *clientMetrics

	id    string
	addr  string
	host  string
	group string

	// if closing is set, count is not allowed to increase from zero; upon
	// reaching zero, cond should be broadcast
	mu      sync.Mutex
	cond    sync.Cond
	closing bool
	count   int

	pingCancel context.CancelFunc
}

var _ internal.ClientConn = (*grpcClientConn)(nil)

// PeerID implements [internal.ClientConn].
func (c *grpcClientConn) PeerID() string { return c.id }

// PeerAddr implements [internal.ClientConn].
func (c *grpcClientConn) PeerAddr() string { return c.addr }

// maybeAcquire returns a non-nil release func if the grpcClientConn is
// currently allowed to open connections; i.e., if it hasn't fully shut down.
func (c *grpcClientConn) maybeAcquire() (release func()) {
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

// Shutdown implements [internal.ClientConn].
func (c *grpcClientConn) Shutdown(ctx context.Context) {
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

// Close implements [internal.ClientConn].
func (c *grpcClientConn) Close() error {
	c.pingCancel()
	return c.cc.Close()
}

// Ping implements [internal.ClientConn].
func (c *grpcClientConn) Ping(ctx context.Context) error {
	release := c.maybeAcquire()
	if release == nil {
		return trace.ConnectionProblem(nil, "error starting stream: connection is shutting down")
	}
	defer release()

	_, err := clientapi.NewProxyServiceClient(c.cc).Ping(ctx, new(clientapi.ProxyServicePingRequest))
	if trace.IsNotImplemented(err) {
		err = nil
	}
	return trace.Wrap(err)
}

// Dial implements [internal.ClientConn].
func (c *grpcClientConn) Dial(
	nodeID string,
	src net.Addr,
	dst net.Addr,
	tunnelType types.TunnelType,
	permit []byte,
) (net.Conn, error) {
	release := c.maybeAcquire()
	if release == nil {
		c.metrics.reportTunnelError(errorProxyPeerTunnelRPC)
		return nil, trace.ConnectionProblem(nil, "error starting stream: connection is shutting down")
	}

	ctx, cancel := context.WithCancel(context.Background())
	context.AfterFunc(ctx, release)

	stream, err := clientapi.NewProxyServiceClient(c.cc).DialNode(ctx)
	if err != nil {
		cancel()
		c.metrics.reportTunnelError(errorProxyPeerTunnelRPC)
		return nil, trace.ConnectionProblem(err, "error starting stream: %v", err)
	}

	err = stream.Send(&clientapi.Frame{
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
				Permit: permit,
			},
		},
	})
	if err != nil {
		cancel()
		return nil, trace.ConnectionProblem(err, "error sending dial frame: %v", err)
	}
	msg, err := stream.Recv()
	if err != nil {
		cancel()
		return nil, trace.ConnectionProblem(err, "error receiving dial response: %v", err)
	}
	if msg.GetConnectionEstablished() == nil {
		cancel()
		return nil, trace.ConnectionProblem(nil, "received malformed connection established frame")
	}

	source := &frameStream{
		stream: stream,
		cancel: cancel,
	}

	streamRW, err := streamutils.NewReadWriter(source)
	if err != nil {
		_ = source.Close()
		return nil, trace.Wrap(err)
	}

	return streamutils.NewConn(streamRW, src, dst), nil
}

// Client manages connections to known peer proxies and allows to open
// connections to agents through them.
type Client struct {
	sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	config   ClientConfig
	conns    map[string]internal.ClientConn
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
		conns:    make(map[string]internal.ClientConn),
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
				switch conn := conn.(type) {
				case *grpcClientConn:
					switch conn.cc.GetState() {
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
			Logger:    c.config.Log,
		},
		ProxyGetter: c.config.AccessPoint,
		ProxyDiffer: func(old, new types.Server) bool {
			return old.GetPeerAddr() != new.GetPeerAddr()
		},
	})
	if err != nil {
		c.config.Log.ErrorContext(c.ctx, "error initializing proxy peer watcher", "error", err)
		return
	}
	defer proxyWatcher.Close()

	for {
		select {
		case <-c.ctx.Done():
			c.config.Log.DebugContext(c.ctx, "stopping peer proxy sync: context done")
			return
		case <-proxyWatcher.Done():
			c.config.Log.DebugContext(c.ctx, "stopping peer proxy sync: proxy watcher done")
			return
		case proxies := <-proxyWatcher.ResourcesC:
			if err := c.updateConnections(proxies); err != nil {
				c.config.Log.ErrorContext(c.ctx, "error syncing peer proxies", "error", err)
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
	toKeep := make(map[string]internal.ClientConn)
	for id, conn := range c.conns {
		proxy, ok := toDial[id]

		// delete nonexistent connections
		if !ok {
			toDelete = append(toDelete, id)
			continue
		}

		// peer address changed
		if conn.PeerAddr() != proxy.GetPeerAddr() {
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
		supportsQUIC, _ := proxy.GetLabel(types.UnstableProxyPeerQUICLabel)
		proxyGroup, _ := proxy.GetLabel(types.ProxyGroupIDLabel)
		conn, err := c.connect(connectParams{
			peerID:       id,
			peerAddr:     proxy.GetPeerAddr(),
			peerHost:     proxy.GetHostname(),
			peerGroup:    proxyGroup,
			supportsQUIC: supportsQUIC == "yes",
		})
		if err != nil {
			c.metrics.reportTunnelError(errorProxyPeerTunnelDial)
			c.config.Log.DebugContext(c.ctx, "error dialing peer proxy", "peer_id", id, "peer_addr", proxy.GetPeerAddr())
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
		go func(conn internal.ClientConn) {
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

// DialNode dials a node through a peer proxy.
func (c *Client) DialNode(
	proxyIDs []string,
	nodeID string,
	src net.Addr,
	dst net.Addr,
	tunnelType types.TunnelType,
	permit []byte,
) (net.Conn, error) {
	conn, _, err := c.dial(
		proxyIDs,
		nodeID,
		src,
		dst,
		tunnelType,
		permit,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// dial opens a new connection through one of the given proxy ids. It tries to
// find an existing [clientConn] or initializes new clientConns to the given
// proxies otherwise. The boolean returned in the second argument is intended
// for testing purposes, to indicates whether the connection used an existing
// clientConn or a newly established one.
func (c *Client) dial(
	proxyIDs []string,
	nodeID string,
	src net.Addr,
	dst net.Addr,
	tunnelType types.TunnelType,
	permit []byte,
) (net.Conn, bool, error) {
	conns, existing, err := c.getConnections(proxyIDs)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	var errs []error
	for _, clientConn := range conns {
		conn, err := clientConn.Dial(nodeID, src, dst, tunnelType, permit)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		return conn, existing, nil
	}

	return nil, existing, trace.NewAggregate(errs...)
}

// getConnections returns connections to the supplied proxy ids.
// it tries to find an existing grpc.ClientConn or initializes a new one
// otherwise.
// The boolean returned in the second argument is intended for testing purposes,
// to indicates whether the connection was cached or newly established.
func (c *Client) getConnections(proxyIDs []string) ([]internal.ClientConn, bool, error) {
	if len(proxyIDs) == 0 {
		return nil, false, trace.BadParameter("failed to dial: no proxy ids given")
	}

	ids := make(map[string]struct{})
	var conns []internal.ClientConn

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

		supportsQUIC, _ := proxy.GetLabel(types.UnstableProxyPeerQUICLabel)
		proxyGroup, _ := proxy.GetLabel(types.ProxyGroupIDLabel)
		conn, err := c.connect(connectParams{
			peerID:       id,
			peerAddr:     proxy.GetPeerAddr(),
			peerHost:     proxy.GetHostname(),
			peerGroup:    proxyGroup,
			supportsQUIC: supportsQUIC == "yes",
		})
		if err != nil {
			c.metrics.reportTunnelError(errorProxyPeerTunnelDirectDial)
			c.config.Log.DebugContext(c.ctx, "error direct dialing peer proxy", "peer_id", id, "peer_addr", proxy.GetPeerAddr())
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
		c.conns[conn.PeerID()] = conn
	}

	c.config.connShuffler(conns)
	return conns, false, nil
}

type connectParams struct {
	peerID       string
	peerAddr     string
	peerHost     string
	peerGroup    string
	supportsQUIC bool
}

// connect dials a new connection to a peer proxy with the given ID and address.
func (c *Client) connect(params connectParams) (internal.ClientConn, error) {
	if params.supportsQUIC && c.config.QUICTransport != nil {
		conn, err := peerquic.NewClientConn(peerquic.ClientConnConfig{
			PeerAddr: params.peerAddr,

			LocalID:     c.config.ID,
			ClusterName: c.config.ClusterName,

			PeerID:    params.peerID,
			PeerHost:  params.peerHost,
			PeerGroup: params.peerGroup,

			Log: c.config.Log,

			GetTLSCertificate: c.config.GetTLSCertificate,
			GetTLSRoots:       c.config.GetTLSRoots,

			Transport: c.config.QUICTransport,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
	tlsConfig := utils.TLSConfig(c.config.TLSCipherSuites)
	tlsConfig.ServerName = apiutils.EncodeClusterName(c.config.ClusterName)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := c.config.GetTLSCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(c.config.GetTLSRoots)

	expectedPeer := authclient.HostFQDN(params.peerID, c.config.ClusterName)

	conn, err := grpc.Dial(
		params.peerAddr,
		grpc.WithTransportCredentials(newClientCredentials(expectedPeer, params.peerAddr, c.config.Log, credentials.NewTLS(tlsConfig))),
		grpc.WithStatsHandler(newStatsHandler(c.reporter)),
		grpc.WithChainStreamInterceptor(metadata.StreamClientInterceptor, interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                peerKeepAlive,
			Timeout:             peerTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin": {}}]}`),
	)
	if err != nil {
		return nil, trace.Wrap(err, "Error dialing proxy %q", params.peerID)
	}

	pingCtx, pingCancel := context.WithCancel(context.Background())
	cc := &grpcClientConn{
		cc:      conn,
		metrics: c.metrics,

		id:    params.peerID,
		addr:  params.peerAddr,
		host:  params.peerHost,
		group: params.peerGroup,

		pingCancel: pingCancel,
	}

	pings, pingFailures := internal.ClientPingsMetrics(internal.ClientPingsMetricsParams{
		LocalID:   c.config.ID,
		PeerID:    params.peerID,
		PeerHost:  params.peerHost,
		PeerGroup: params.peerGroup,
	})
	go internal.RunClientPing(pingCtx, cc, pings, pingFailures)

	return cc, nil
}
