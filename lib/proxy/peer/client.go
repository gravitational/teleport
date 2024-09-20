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
	"crypto/x509"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	peerv0 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/proxy/peer/v0"
	"github.com/gravitational/teleport/gen/proto/go/teleport/lib/proxy/peer/v0/peerv0connect"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessPoint is the subset of the auth cache consumed by the [Client].
type AccessPoint interface {
	types.Events
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
	Log logrus.FieldLogger
	// Clock is used to control connection monitoring ticker.
	Clock clockwork.Clock
	// ClusterName is the name of the cluster.
	ClusterName string

	QUICTransport *quic.Transport

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
		c.Log = logrus.StandardLogger()
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

// clientConn hold info about a dialed grpc connection
type clientConn struct {
	connCtx context.Context
	cancel  context.CancelFunc
	hc      *http.Client
	sc      peerv0connect.ProxyServiceClient

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
	defer c.hc.CloseIdleConnections()
	defer c.cancel()

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

func (c *clientConn) Close() error {
	c.cancel()
	c.hc.CloseIdleConnections()
	return nil
}

// Client is a peer proxy service client using grpc and tls.
type Client struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	config ClientConfig
	conns  map[string]*clientConn
}

// NewClient creats a new peer proxy client.
func NewClient(config ClientConfig) (*Client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(config.Context)

	c := &Client{
		config: config,
		ctx:    closeContext,
		cancel: cancel,
		conns:  make(map[string]*clientConn),
	}

	if c.config.sync != nil {
		go c.config.sync()
	} else {
		go c.sync()
	}

	return c, nil
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
	c.mu.RLock()

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
			c.config.Log.Debugf("Error dialing peer proxy %+v at %+v", id, proxy.GetPeerAddr())
			errs = append(errs, err)
			continue
		}

		toKeep[id] = conn
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

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
	stream, _, err := c.dial(proxyIDs, &peerv0.DialRequest{
		NodeId:     nodeID,
		TunnelType: string(tunnelType),
		Source: &peerv0.NetAddr{
			Address: src.String(),
			Network: src.Network(),
		},
		Destination: &peerv0.NetAddr{
			Address: dst.String(),
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

// clientFrameStream wraps a client side stream as a [streamutils.Source].
type clientFrameStream struct {
	stream interface {
		Send(*peerv0.DialNodeRequest) error
		Receive() (*peerv0.DialNodeResponse, error)
	}
	cancel context.CancelFunc
}

func (s *clientFrameStream) Send(p []byte) error {
	return trace.Wrap(s.stream.Send(&peerv0.DialNodeRequest{Message: &peerv0.DialNodeRequest_Data{
		Data: &peerv0.Data{Bytes: p},
	}}))
}

func (s *clientFrameStream) Recv() ([]byte, error) {
	frame, err := s.stream.Receive()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data := frame.GetData()
	if data == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return data.GetBytes(), nil
}

func (s *clientFrameStream) Close() error {
	s.cancel()
	return nil
}

// Shutdown gracefully shuts down all existing client connections.
func (c *Client) Shutdown(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

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

// Close closes all existing client connections.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.conns)
}

// dial opens a new stream to one of the supplied proxy ids.
// it tries to find an existing grpc.ClientConn or initializes a new rpc
// to one of the proxies otherwise.
// The boolean returned in the second argument is intended for testing purposes,
// to indicates whether the connection was cached or newly established.
func (c *Client) dial(proxyIDs []string, dialRequest *peerv0.DialRequest) (*clientFrameStream, bool, error) {
	conns, existing, err := c.getConnections(proxyIDs)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	var errs []error
	for _, conn := range conns {
		release := conn.maybeAcquire()
		if release == nil {
			errs = append(errs, trace.ConnectionProblem(nil, "error starting stream: connection is shutting down"))
			continue
		}

		ctx, cancel := context.WithCancel(conn.connCtx)
		context.AfterFunc(ctx, release)

		stream := conn.sc.DialNode(ctx)

		dialTimeout := time.AfterFunc(10*time.Second, cancel)
		err = stream.Send(&peerv0.DialNodeRequest{
			Message: &peerv0.DialNodeRequest_DialRequest{DialRequest: dialRequest},
		})
		if err != nil {
			cancel()
			errs = append(errs, trace.ConnectionProblem(err, "error sending dial frame: %v", err))
			continue
		}
		msg, err := stream.Receive()
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
		if !dialTimeout.Stop() {
			<-ctx.Done()
			errs = append(errs, trace.ConnectionProblem(err, "error receiving dial response: %v", err))
			continue
		}

		return &clientFrameStream{
			stream: stream,
			cancel: cancel,
		}, existing, nil
	}

	return nil, false, trace.NewAggregate(errs...)
}

// getConnections returns connections to the supplied proxy ids. it tries to
// find an existing [clientConn] or initializes a new one otherwise. The boolean
// returned in the second argument is intended for testing purposes, to
// indicates whether the connection was cached or newly established.
func (c *Client) getConnections(proxyIDs []string) (_ []*clientConn, existing bool, _ error) {
	if len(proxyIDs) == 0 {
		return nil, false, trace.BadParameter("failed to dial: no proxy ids given")
	}

	ids := make(map[string]struct{})
	var conns []*clientConn

	// look for existing matching connections.
	c.mu.RLock()
	for _, id := range proxyIDs {
		ids[id] = struct{}{}

		conn, ok := c.conns[id]
		if !ok {
			continue
		}

		conns = append(conns, conn)
	}
	c.mu.RUnlock()

	if len(conns) != 0 {
		c.config.connShuffler(conns)
		return conns, true, nil
	}

	// try to establish new connections otherwise.
	proxies, err := c.config.AuthClient.GetProxies()
	if err != nil {
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
			c.config.Log.Debugf("Error direct dialing peer proxy %+v at %+v", id, proxy.GetPeerAddr())
			errs = append(errs, err)
			continue
		}

		conns = append(conns, conn)
	}

	if len(conns) == 0 {
		return nil, false, trace.ConnectionProblem(trace.NewAggregate(errs...), "Error dialing all proxies")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, conn := range conns {
		c.conns[conn.id] = conn
	}

	c.config.connShuffler(conns)
	return conns, false, nil
}

// connect dials a new connection to proxyAddr.
func (c *Client) connect(peerID string, peerAddr string) (*clientConn, error) {
	tlsConfig := utils.TLSConfig(c.config.TLSCipherSuites)
	tlsConfig.ServerName = apiutils.EncodeClusterName(c.config.ClusterName)
	tlsConfig.NextProtos = []string{"h2"}

	getClientCertificate := c.config.GetTLSCertificate
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := getClientCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}

	expectedPeer := authclient.HostFQDN(peerID, c.config.ClusterName)
	tlsConfig.VerifyPeerCertificate = verifyPeerCertificateIsSpecificProxy(expectedPeer)

	dtlsConfig := tlsConfig.Clone()
	dtlsConfig.NextProtos = []string{"h3"}

	getRootCAs := c.config.GetTLSRoots
	ht := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			rootCAs, err := getRootCAs()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			tlsConfig := tlsConfig.Clone()
			tlsConfig.RootCAs = rootCAs

			nc, err := new(net.Dialer).DialContext(ctx, network, addr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			tc := tls.Client(nc, tlsConfig)
			if err := tc.HandshakeContext(ctx); err != nil {
				return nil, trace.Wrap(err)
			}
			return tc, nil
		},

		IdleConnTimeout: 5 * time.Minute,
		ReadIdleTimeout: time.Minute,
	}

	hc := &http.Client{
		Transport: ht,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	connCtx, cancel := context.WithCancel(c.config.Context)

	grpcOption := connect.WithGRPC()
	if quicTransport := c.config.QUICTransport; quicTransport != nil {
		grpcOption = connect.WithClientOptions(nil...)
		h3t := &http3.RoundTripper{
			EnableDatagrams:        true,
			MaxResponseHeaderBytes: math.MaxInt64,
			QUICConfig: &quic.Config{
				MaxIdleTimeout: 30 * time.Second,

				MaxIncomingStreams:    1 << 60,
				MaxIncomingUniStreams: 1 << 60,
				KeepAlivePeriod:       10 * time.Second,

				InitialPacketSize:       1200,
				DisablePathMTUDiscovery: true,

				EnableDatagrams: true,
			},
			Dial: func(ctx context.Context, addr string, _ *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
				c.config.Log.Errorf("== NEW DIAL IN HTTP3 ROUNDTRIPPER")
				rootCAs, err := getRootCAs()
				if err != nil {
					return nil, trace.Wrap(err)
				}

				tlsConfig := tlsConfig.Clone()
				tlsConfig.RootCAs = rootCAs
				tlsConfig.NextProtos = []string{"h3"}

				ap, err := netip.ParseAddrPort(addr)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				conn, err := quicTransport.DialEarly(ctx, net.UDPAddrFromAddrPort(ap), tlsConfig, cfg)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				context.AfterFunc(conn.Context(), func() {
					c.config.Log.Errorf("== CONNECTION LOST: %v", context.Cause(conn.Context()))
				})
				return conn, nil
			},
		}
		hc.Transport = h3t
		context.AfterFunc(connCtx, func() { _ = h3t.Close() })
	}

	clientOptions := connect.WithClientOptions(
		connect.WithAcceptCompression("gzip", nil, nil),
		connect.WithInterceptors(addVersionInterceptor{}, traceErrorsInterceptor{}),
		grpcOption,
	)
	sc := peerv0connect.NewProxyServiceClient(hc, "https://"+peerAddr, clientOptions)

	return &clientConn{
		connCtx: connCtx,
		cancel:  cancel,

		hc: hc,
		sc: sc,

		id:   peerID,
		addr: peerAddr,
	}, nil
}

func verifyPeerCertificateIsSpecificProxy(peer string) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) < 1 {
			return trace.AccessDenied("missing server certificate (this is a bug)")
		}

		serverCert := verifiedChains[0][0]
		serverIdentity, err := tlsca.FromSubject(serverCert.Subject, serverCert.NotAfter)
		if err != nil {
			return trace.Wrap(err)
		}

		if !slices.Contains(serverIdentity.Groups, string(types.RoleProxy)) {
			return trace.AccessDenied("expected Proxy client credentials")
		}

		if serverIdentity.Username != peer {
			return trace.AccessDenied("expected Proxy %v, got %q", peer, serverIdentity.Username)
		}

		return nil
	}
}
