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
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type ClientConfig struct {
	Log *slog.Logger

	GetCertificate func() (*tls.Certificate, error)
	GetPool        func() (*x509.CertPool, error)
	Ciphersuites   []uint16

	TunnelType types.TunnelType
	RelayAddr  string

	HandleConnection func(net.Conn)
	RelayInfoSetter  SetRelayInfoFunc
}

func NewClient(cfg ClientConfig) (*Client, error) {
	c := &Client{
		log: cfg.Log,

		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,

		tunnelType: cfg.TunnelType,
		relayAddr:  cfg.RelayAddr,

		handleConnection: cfg.HandleConnection,
		relayInfoSetter:  cfg.RelayInfoSetter,

		running: make(chan struct{}),
	}

	return c, nil
}

type Client struct {
	log *slog.Logger

	getCertificate func() (*tls.Certificate, error)
	getPool        func() (*x509.CertPool, error)
	ciphersuites   []uint16

	tunnelType types.TunnelType
	relayAddr  string

	handleConnection func(net.Conn)
	relayInfoSetter  func(relayGroup string, relayIDs []string)

	wg sync.WaitGroup

	mu      sync.Mutex
	started bool
	running chan struct{}

	relayGroup            string
	targetConnectionCount int

	// activeConnections is keyed by relay username (i.e. host ID dot
	// clustername).
	activeConnections map[string]clientConn
}

type clientConn interface {
	Close() error

	ServerIsTerminating() bool
}

func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return trace.AlreadyExists("relay tunnel client already started")
	}
	c.started = true

	if c.running == nil {
		return nil
	}

	c.wg.Add(1)
	go c.dialLoopGrouped()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.discoverLoop()
	}()

	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.running == nil {
		c.mu.Unlock()
		return os.ErrClosed
	}
	close(c.running)
	c.running = nil
	activeConnections := maps.Clone(c.activeConnections)
	c.mu.Unlock()
	for _, cc := range activeConnections {
		_ = cc.Close()
	}
	c.wg.Wait()
	return nil
}

func (c *Client) pushRelayInfoLocked() {
	c.relayInfoSetter(c.relayGroup, slices.Sorted(maps.Keys(c.activeConnections)))
}

func (c *Client) getRunning() chan struct{} {
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	return running
}

func (c *Client) discoverLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		running := c.getRunning()
		if running == nil {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
		case <-running:
			cancel()
		}
	}()

	var errDelay time.Duration
	for {
		if ctx.Err() != nil {
			c.log.InfoContext(context.Background(), "Exiting from discover loop due to requested termination")
			return
		}

		if err := c.discoverOnce(ctx); err != nil {
			c.log.WarnContext(context.Background(), "Failed to discover relay", "error", err)
			errDelay = min(max(2*time.Second, errDelay*2), 30*time.Second)
			select {
			case <-time.After(errDelay):
			case <-ctx.Done():
				return
			}
			continue
		}

		c.log.DebugContext(ctx, "Relay discovery successful, waiting")
		errDelay = 0
		select {
		case <-time.After(5 * time.Minute):
		case <-ctx.Done():
			return
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

func (c *Client) dialLoopGrouped() {
	defer c.wg.Done()

	var delay time.Duration
	for {
		running := c.getRunning()
		if running == nil {
			c.log.InfoContext(context.Background(), "Exiting from dial loop due to requested termination")
			return
		}

		// TODO(espadolini): figure out a nice way to wait for "events" rather
		// than just checking repeatedly
		if !c.shouldDial() {
			c.log.LogAttrs(context.Background(), logutils.TraceLevel, "Not attempting new relay tunnel connection")
			select {
			case <-running:
				c.log.InfoContext(context.Background(), "Exiting from dial loop due to requested termination")
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		if err := c.dialRelayGrouped(); err != nil {
			level := slog.LevelWarn
			if trace.IsAlreadyExists(err) {
				level = slog.LevelDebug
			}

			c.log.Log(context.Background(), level, "Failed to dial relay tunnel", "error", err)
			delay = min(max(5*time.Second, delay*2), time.Minute)
			select {
			case <-running:
				c.log.InfoContext(context.Background(), "Exiting from dial loop due to requested termination")
				return
			case <-time.After(delay):
			}
			continue
		}

		delay = 0
	}
}

func (c *Client) dialRelayGrouped() error {
	log := c.log.With("connection_id", uuid.NewString())
	log.DebugContext(context.Background(), "Attempting new relay tunnel connection")

	cert, err := c.getCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	pool, err := c.getPool()
	if err != nil {
		return trace.Wrap(err)
	}

	helloCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

			if !slices.Contains(id.Groups, string(types.RoleRelay)) &&
				!slices.Contains(id.SystemRoles, string(types.RoleRelay)) {
				return trace.BadParameter("dialed server is not a relay (roles %+q, system roles %+q)", id.Groups, id.SystemRoles)
			}

			serverHostID, _, _ := strings.Cut(id.Username, ".")
			c.mu.Lock()
			_, alreadyConnected := c.activeConnections[serverHostID]
			c.mu.Unlock()
			if alreadyConnected {
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

	yamuxConfig := &yamux.Config{
		AcceptBacklog: 128,

		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,

		MaxStreamWindowSize: 256 * 1024,

		StreamCloseTimeout: time.Minute,
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

	c.updateDiscovery(helloCtx, serverHello.GetRelayGroup(), serverHello.GetTargetConnectionCount())

	serverHostID, _, _ := strings.Cut(serverID.Username, ".")
	c.mu.Lock()
	if _, ok := c.activeConnections[serverHostID]; ok {
		c.mu.Unlock()
		_ = controlStream.Close()
		_ = session.Close()
		return trace.AlreadyExists("relay %+q already claimed (after tunnel handshake)", serverHostID)
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
		log.InfoContext(context.Background(), "running relay tunnel client connection")
		defer c.log.InfoContext(context.Background(), "done with relay tunnel client connection")
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
			controlMsg := new(relaytunnelv1alpha.ServerControl)
			if err := readProto(controlStream, controlMsg); err != nil {
				return
			}

			// we only set the value, we don't reset it
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

	// this horrible hack is needed because a few places (including x/crypto/ssh
	// of all things, which was followed by the connection resumption handler)
	// expect TCP addresses to be [*net.TCPAddr]s and will type assert on that
	// rather than dealing in generic [net.Addr]s
	if ap, err := netip.ParseAddrPort(src.String()); err == nil {
		ap = netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
		src = net.TCPAddrFromAddrPort(ap)
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
