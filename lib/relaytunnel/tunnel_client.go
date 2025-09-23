// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package relaytunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ClientConfig contains the parameters for [NewClient].
type ClientConfig struct {
	Log *slog.Logger

	GetCertificate func() (*tls.Certificate, error)
	GetPool        func() (*x509.CertPool, error)
	Ciphersuites   []uint16

	TunnelType types.TunnelType
	RelayAddr  string

	// HandleConnection will be called with every new connection, and should
	// block until it's done using the connection.
	HandleConnection func(net.Conn)

	// RelayInfoSetter will be called every time the relay group name is
	// discovered (or changes, which shouldn't happen) and whenever the list of
	// connected relay host IDs changes. It's always called with an owned,
	// sorted list of host IDs.
	RelayInfoSetter SetRelayInfoFunc
}

// NewClient creates a [Client] with a given configuration.
func NewClient(cfg ClientConfig) (*Client, error) {
	running, cancelRunning := context.WithCancel(context.Background())

	c := &Client{
		log: cfg.Log,

		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,

		tunnelType: cfg.TunnelType,
		relayAddr:  cfg.RelayAddr,

		handleConnection: cfg.HandleConnection,
		relayInfoSetter:  cfg.RelayInfoSetter,

		ctx:       running,
		cancelCtx: cancelRunning,
	}

	return c, nil
}

// Client is a relay tunnel client which will periodically discover the relay
// server configuration and keep an appropriate amount of open tunnel connection
// to distinct, non-terminating relay servers.
type Client struct {
	log *slog.Logger

	getCertificate func() (*tls.Certificate, error)
	getPool        func() (*x509.CertPool, error)
	ciphersuites   []uint16

	tunnelType types.TunnelType
	relayAddr  string

	handleConnection func(net.Conn)
	relayInfoSetter  func(relayGroup string, relayIDs []string)

	// mu protects the rest of the struct. Canceling the context and adding to
	// the wait group without guards should only happen while holding the mutex.
	mu sync.Mutex

	wg sync.WaitGroup

	// ctx has to synchronize with wg, so it should be a context that cannot
	// be closed externally.
	ctx       context.Context
	cancelCtx context.CancelFunc

	// started is used to prevent double calls to Start, but should not be used
	// for other synchronization since it's never reset.
	started bool

	relayGroup            string
	targetConnectionCount int

	// activeConnections is keyed by relay username (i.e. host ID dot
	// clustername). Closing a connection should eventually delete it from the
	// map and release all of its tasks in wg.
	activeConnections map[string]clientConn
}

// clientConn is used to keep track of open connections to relay servers.
type clientConn interface {
	io.Closer

	// ServerIsTerminating should return true if the relay server at the remote
	// end of the connection has announced that it's shutting down. It should
	// not block or perform I/O, so the intended implementation is loading an
	// atomic boolean.
	ServerIsTerminating() bool
}

// Start will start an unstarted client, or return an error. Starting a closed
// unstarted client will succeed and do nothing.
func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return trace.AlreadyExists("relay tunnel client already started")
	}
	c.started = true

	if c.ctx.Err() != nil {
		return nil
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.dialLoopGrouped()
	}()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.discoverLoop()
	}()

	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.ctx.Err() != nil {
		c.mu.Unlock()
		return os.ErrClosed
	}
	c.cancelCtx()

	for _, cc := range c.activeConnections {
		// Close might block and we're not expecting so many connections that we
		// need to limit parallelism
		go cc.Close()
	}
	c.mu.Unlock()

	// this will wait for all client conns to exit, as well as the discovery and
	// dial loops
	c.wg.Wait()

	return nil
}

func (c *Client) pushRelayInfoLocked() {
	c.relayInfoSetter(c.relayGroup, slices.Sorted(maps.Keys(c.activeConnections)))
}

func (c *Client) discoverLoop() {
	var discoveryErrorDelay time.Duration
	for {
		if c.ctx.Err() != nil {
			c.log.InfoContext(c.ctx, "Exiting from discover loop due to requested termination")
			return
		}

		if err := c.discoverOnce(c.ctx); err != nil {
			if c.ctx.Err() != nil {
				continue
			}

			c.log.WarnContext(c.ctx, "Failed to discover relay", "error", err)
			discoveryErrorDelay = min(max(5*time.Second, discoveryErrorDelay*2), retryutils.SeventhJitter(30*time.Second))
			select {
			case <-time.After(discoveryErrorDelay):
			case <-c.ctx.Done():
			}
			continue
		}

		discoveryErrorDelay = 0
		c.log.DebugContext(c.ctx, "Relay discovery successful, waiting")

		select {
		case <-time.After(retryutils.SeventhJitter(5 * time.Minute)):
		case <-c.ctx.Done():
		}
	}
}

func (c *Client) discoverOnce(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := discover(ctx, DiscoverParams{
		GetCertificate: c.getCertificate,
		GetPool:        c.getPool,
		Ciphersuites:   c.ciphersuites,
		Target:         c.relayAddr,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	c.updateDiscovery(ctx, resp.GetRelayGroup(), resp.GetTargetConnectionCount())

	return nil
}

func (c *Client) updateDiscovery(ctx context.Context, relayGroup string, connectionCount int32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if relayGroup != "" {
		if c.relayGroup != "" && c.relayGroup != relayGroup {
			c.log.WarnContext(ctx, "Relay group has changed", "old_relay_group", c.relayGroup, "new_relay_group", relayGroup)
		}
		c.relayGroup = relayGroup
	}
	if connectionCount > 0 {
		c.targetConnectionCount = int(connectionCount)
	}

	c.pushRelayInfoLocked()
}

// dialLoopGrouped runs the logic to open new connections. Should only be called
// while holding a task in the [Client.wg] wait group.
func (c *Client) dialLoopGrouped() {
	var dialErrorDelay time.Duration
	for {
		if c.ctx.Err() != nil {
			c.log.InfoContext(c.ctx, "Exiting from dial loop due to requested termination")
			return
		}

		// TODO(espadolini): figure out a nice way to wait for "events" rather
		// than just checking repeatedly
		if !c.shouldDial() {
			c.log.LogAttrs(c.ctx, logutils.TraceLevel, "Not attempting new relay tunnel connection")
			select {
			case <-c.ctx.Done():
			case <-time.After(2 * time.Second):
			}
			continue
		}

		// this ID is only used to tie logs of the same connection together
		log := c.log.With("connection_id", uuid.NewString())
		if err := c.dialOnceGrouped(log); err != nil {
			level := slog.LevelWarn
			if trace.IsAlreadyExists(err) {
				level = slog.LevelDebug
			}

			log.Log(c.ctx, level, "Failed to dial relay tunnel", "error", err)
			dialErrorDelay = min(max(2*time.Second, dialErrorDelay*2), retryutils.SeventhJitter(time.Minute))
			select {
			case <-c.ctx.Done():
			case <-time.After(dialErrorDelay):
			}
			continue
		}

		dialErrorDelay = 0
	}
}

// dialOnceGrouped will dial the relay address and return success after the
// connection is established and stored in [Client.activeConnections]. Should
// only be called while holding a task in the [Client.wg] wait group, as it will
// need to spawn more goroutines tied to the waitgroup.
func (c *Client) dialOnceGrouped(log *slog.Logger) error {
	log.DebugContext(c.ctx, "Attempting new relay tunnel connection")

	cert, err := c.getCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	pool, err := c.getPool()
	if err != nil {
		return trace.Wrap(err)
	}

	helloCtx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	nc, err := new(net.Dialer).DialContext(helloCtx, "tcp", c.relayAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	serverName, _, err := net.SplitHostPort(c.relayAddr)
	if err != nil {
		serverName = c.relayAddr
	}

	var serverID *tlsca.Identity
	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return cert, nil
		},

		// we're likely going to connect to a self-determined IP address, and we
		// are not really going to get any benefit from allowing any
		// self-determined address in IP SANs at join time just so we can verify
		// them (or, even worse, requiring some general hostname based on the
		// hostname), so instead we disregard the normal verification (which
		// would enforce that the address is in the server cert's SANs) and we
		// check that the server cert is valid for the Relay role; as a bonus,
		// we get to be good clients and pass the appropriate SNI in our TLS
		// ClientHello
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				return trace.NotImplemented("relay tunnel protocol not supported")
			}

			opts := x509.VerifyOptions{
				DNSName: "",

				Roots:         pool,
				Intermediates: nil,

				KeyUsages: []x509.ExtKeyUsage{
					x509.ExtKeyUsageServerAuth,
				},
			}
			if len(cs.PeerCertificates) > 1 {
				opts.Intermediates = x509.NewCertPool()
				for _, cert := range cs.PeerCertificates[1:] {
					opts.Intermediates.AddCert(cert)
				}
			}
			if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
				return trace.Wrap(err)
			}

			id, err := tlsca.FromSubject(cs.PeerCertificates[0].Subject, cs.PeerCertificates[0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}
			serverID = id

			// multiple system roles (as used by the Instance cert) are
			// currently only in SystemRoles (with Groups only containing the
			// Instance role) for the sake of backwards compatibility, but
			// there's no drawback to checking both
			if !slices.Contains(id.Groups, string(types.RoleRelay)) &&
				!slices.Contains(id.SystemRoles, string(types.RoleRelay)) {
				return trace.BadParameter("dialed server is not a relay (roles %+q, system roles %+q)", id.Groups, id.SystemRoles)
			}

			serverHostID, _, _ := strings.Cut(id.Username, ".")
			c.mu.Lock()
			_, alreadyConnected := c.activeConnections[serverHostID]
			c.mu.Unlock()
			if alreadyConnected {
				// by aborting the handshake here we reduce the cpu load on both
				// client and server
				return trace.AlreadyExists("relay %+q already claimed", serverHostID)
			}

			return nil
		},

		NextProtos: []string{yamuxTunnelALPN},
		ServerName: serverName,

		CipherSuites: c.ciphersuites,
		MinVersion:   tls.VersionTLS12,
	}

	tc := tls.Client(nc, tlsConfig)
	if err := tc.HandshakeContext(helloCtx); err != nil {
		return trace.Wrap(err)
	}

	// this is a copy of the default config as returned by [yamux.DefaultConfig]
	// at the time of writing with a slightly tighter timing for
	// StreamOpenTimeout (because we expect the control stream open request to
	// be handled very promptly by the server) and our logger adapter
	yamuxConfig := &yamux.Config{
		AcceptBacklog: 128,

		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,

		// the window size defines a maximum throughput limit per stream based
		// on the RTT but the relay is intended for use in low latency
		// environments (it's the whole point of it) so unless we find a reason
		// we will just stick with the default for now; the values can differ
		// between client and server since they take effect on the receive
		// direction of the stream
		MaxStreamWindowSize: 256 * 1024,

		StreamCloseTimeout: 5 * time.Minute,
		StreamOpenTimeout:  30 * time.Second,

		LogOutput: nil,
		Logger:    (*yamuxLogger)(log),
	}

	session, err := yamux.Client(tc, yamuxConfig)
	if err != nil {
		_ = tc.Close()
		return err
	}

	controlStream, err := session.OpenStream()
	if err != nil {
		_ = session.Close()
		return trace.Wrap(err)
	}
	helloDeadline, _ := helloCtx.Deadline()
	controlStream.SetDeadline(helloDeadline)

	if err := writeProto(controlStream, &relaytunnelv1alpha.ClientHello{
		TunnelType: string(c.tunnelType),
	}); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		return trace.Wrap(err)
	}

	serverHello := new(relaytunnelv1alpha.ServerHello)
	if err := readProto(controlStream, serverHello); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		return trace.Wrap(err)
	}

	controlStream.SetDeadline(time.Time{})

	if err := trail.FromGRPC(status.FromProto(serverHello.GetStatus()).Err()); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		return trace.Wrap(err)
	}

	serverHostID, _, _ := strings.Cut(serverID.Username, ".")
	log.DebugContext(c.ctx, "Successfully connected", "server_id", serverHostID)

	c.updateDiscovery(helloCtx, serverHello.GetRelayGroup(), serverHello.GetTargetConnectionCount())

	c.mu.Lock()
	if _, ok := c.activeConnections[serverHostID]; ok {
		c.mu.Unlock()
		_ = controlStream.Close()
		_ = session.Close()
		return trace.AlreadyExists("relay %+q already claimed after tunnel handshake (this is a bug)", serverHostID)
	}
	cc := &yamuxClientConn{
		session: session,
	}
	if c.activeConnections == nil {
		c.activeConnections = make(map[string]clientConn)
	}
	c.activeConnections[serverHostID] = cc
	c.pushRelayInfoLocked()
	c.mu.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		defer func() {
			c.mu.Lock()
			delete(c.activeConnections, serverHostID)
			c.pushRelayInfoLocked()
			c.mu.Unlock()
		}()

		log := c.log.With("server_id", serverHostID)
		log.InfoContext(c.ctx, "running relay tunnel client connection")
		defer c.log.InfoContext(c.ctx, "done with relay tunnel client connection")
		cc.run(controlStream, c.handleConnection, log)
	}()

	return nil
}

type yamuxClientConn struct {
	session *yamux.Session

	terminating atomic.Bool
}

func (c *yamuxClientConn) run(controlStream *yamux.Stream, handleConnection func(net.Conn), log *slog.Logger) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer c.session.Close()
		defer controlStream.Close()
		for {
			// TODO(espadolini): add a way to reuse buffers and allocated
			// messages for the control stream messages
			controlMsg := new(relaytunnelv1alpha.ServerControl)
			if err := readProto(controlStream, controlMsg); err != nil {
				return
			}

			// we only set the value, we never reset it
			if controlMsg.GetTerminating() {
				c.terminating.Store(true)
			}

			log.DebugContext(context.Background(), "Received control message from server", "terminating", controlMsg.GetTerminating())
		}
	}()
	go func() {
		defer wg.Done()
		defer c.session.Close()
		for {
			stream, err := c.session.AcceptStream()
			if err != nil {
				return
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				log.DebugContext(context.Background(), "Serving tunneled connection", "stream_id", stream.StreamID())
				defer log.DebugContext(context.Background(), "Done serving tunneled connection", "stream_id", stream.StreamID())
				c.handleStream(stream, handleConnection)
			}()
		}
	}()
	wg.Wait()
}

func (c *yamuxClientConn) handleStream(stream *yamux.Stream, handleConnection func(net.Conn)) {
	defer stream.Close()

	dialReq := new(relaytunnelv1alpha.DialRequest)
	if err := readProto(stream, dialReq); err != nil {
		return
	}
	src := addrFromProto(dialReq.Source)
	dst := addrFromProto(dialReq.Destination)
	if src == nil || dst == nil {
		err := trace.BadParameter("missing source or destination address")
		_ = writeProto(stream, &relaytunnelv1alpha.DialResponse{
			Status: status.Convert(trail.ToGRPC(err)).Proto(),
		})
		return
	}

	if err := writeProto(stream, &relaytunnelv1alpha.DialResponse{
		Status: nil, // i.e. status.Convert(error(nil)).Proto()
	}); err != nil {
		return
	}

	// this is needed because a few places (including x/crypto/ssh of all
	// things, which was then followed by the connection resumption handler)
	// expect TCP addresses to be [*net.TCPAddr]s and will type assert on that
	// rather than dealing in generic [net.Addr]s
	if src.Network() == "tcp" {
		ap, err := netip.ParseAddrPort(src.String())
		if err == nil {
			ap = netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
			src = net.TCPAddrFromAddrPort(ap)
		}
	}

	nc := utils.NewConnWithAddr(stream, dst, src)
	handleConnection(nc)
}

// Close implements [clientConn].
func (c *yamuxClientConn) Close() error {
	return c.session.Close()
}

// ServerIsTerminating implements [clientConn].
func (c *yamuxClientConn) ServerIsTerminating() bool {
	return c.terminating.Load()
}

func (c *Client) shouldDial() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	var thrivingConnections int
	for _, cc := range c.activeConnections {
		if !cc.ServerIsTerminating() {
			thrivingConnections++
		}
	}

	return thrivingConnections < c.targetConnectionCount
}
