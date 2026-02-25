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

package reversetunnelv3

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/credentials"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	reversetunnelv1 "github.com/gravitational/teleport/gen/proto/go/teleport/reversetunnel/v1"
	"github.com/gravitational/teleport/lib/tlsca"
)

// ServerConfig contains the parameters for [NewServer].
type ServerConfig struct {
	Log *slog.Logger

	// ProxyID is the UUID of this proxy instance, sent to agents in ProxyHello.
	ProxyID string

	// GetCertificate returns the TLS certificate to present to agents.
	GetCertificate func(ctx context.Context) (*tls.Certificate, error)

	// GetPool returns the certificate pool used to verify agent certificates.
	GetPool func(ctx context.Context) (*x509.CertPool, error)

	Ciphersuites []uint16

	// GetProxies returns the current gossip proxy list to include in ProxyHello
	// and periodic ProxyControl messages.
	GetProxies func() []types.Server
}

// NewServer creates a [TunnelServer] with the given configuration.
func NewServer(cfg ServerConfig) (*TunnelServer, error) {
	if cfg.Log == nil {
		return nil, trace.BadParameter("missing Log")
	}
	if cfg.ProxyID == "" {
		return nil, trace.BadParameter("missing ProxyID")
	}
	if cfg.GetCertificate == nil {
		return nil, trace.BadParameter("missing GetCertificate")
	}
	if cfg.GetPool == nil {
		return nil, trace.BadParameter("missing GetPool")
	}
	if cfg.GetProxies == nil {
		return nil, trace.BadParameter("missing GetProxies")
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	return &TunnelServer{
		log:            cfg.Log,
		proxyID:        cfg.ProxyID,
		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,
		getProxies:     cfg.GetProxies,
		ctx:            ctx,
		ctxCancel:      ctxCancel,
	}, nil
}

// TunnelServer is the proxy-side component of the reversetunnelv3 protocol. It
// accepts inbound TLS connections from agents, performs the AgentHello/ProxyHello
// handshake, and routes proxy-initiated dial streams to the correct agent.
type TunnelServer struct {
	log *slog.Logger

	proxyID        string
	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16
	getProxies     func() []types.Server

	mu sync.Mutex
	// conns maps connKey{hostID, scope} to a list of active agentConns. The
	// most-recently-connected entry is always appended last and preferred for
	// dialing.
	conns map[connKey][]*agentConn

	// ctx is cancelled when the server begins shutting down. wg tracks all
	// goroutines spawned by handleAgentConn so Shutdown can wait for them.
	ctx       context.Context
	ctxCancel context.CancelFunc
	wg        sync.WaitGroup
}

// connKey identifies a single agent (host) within the server's connection
// table. The scope field supports scoped resource certificates.
type connKey struct {
	hostID string
	scope  string
}

// GRPCServerCredentials returns gRPC TransportCredentials that dispatch
// connections negotiating yamuxTunnelALPN to the TunnelServer while forwarding
// connections negotiating "h2" to the gRPC server.
func (s *TunnelServer) GRPCServerCredentials() credentials.TransportCredentials {
	return &grpcServerCredentials{
		tunnelServer:   s,
		getCertificate: s.getCertificate,
		getPool:        s.getPool,
		ciphersuites:   s.ciphersuites,
	}
}

// grpcServerCredentials implements credentials.TransportCredentials. It
// performs TLS negotiation and dispatches tunnel connections out-of-band so
// the gRPC server never sees them.
type grpcServerCredentials struct {
	tunnelServer   *TunnelServer
	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16
}

// ClientHandshake implements [credentials.TransportCredentials].
func (*grpcServerCredentials) ClientHandshake(_ context.Context, _ string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	_ = rawConn.Close()
	return nil, nil, trace.NotImplemented("grpcServerCredentials can only be used as a server")
}

// OverrideServerName implements [credentials.TransportCredentials].
func (*grpcServerCredentials) OverrideServerName(string) error { return nil }

// Clone implements [credentials.TransportCredentials].
func (c *grpcServerCredentials) Clone() credentials.TransportCredentials { return c }

// Info implements [credentials.TransportCredentials].
func (*grpcServerCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "tls", SecurityVersion: "1.2"}
}

// ServerHandshake implements [credentials.TransportCredentials].
func (c *grpcServerCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cert, err := c.getCertificate(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}
	pool, err := c.getPool(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	var clientID *tlsca.Identity
	tlsCfg := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				return trace.NotImplemented("missing ALPN in TLS ClientHello")
			}
			if len(cs.VerifiedChains) < 1 {
				return trace.AccessDenied("missing or invalid client certificate")
			}
			// For h2 (gRPC) we do not need the TLS identity here.
			if cs.NegotiatedProtocol == "h2" {
				return nil
			}
			id, err := tlsca.FromSubject(cs.VerifiedChains[0][0].Subject, cs.VerifiedChains[0][0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}
			clientID = id
			return nil
		},
		NextProtos:             []string{yamuxTunnelALPN, "h2"},
		ClientAuth:             tls.RequireAndVerifyClientCert,
		ClientCAs:              pool,
		InsecureSkipVerify:     false,
		MinVersion:             tls.VersionTLS12,
		CipherSuites:           c.ciphersuites,
		SessionTicketsDisabled: true,
	}

	tlsConn := tls.Server(rawConn, tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		_ = tlsConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	cs := tlsConn.ConnectionState()
	if cs.NegotiatedProtocol == yamuxTunnelALPN {
		go c.tunnelServer.handleAgentConn(tlsConn, clientID)
		return nil, nil, credentials.ErrConnDispatched
	}

	return tlsConn, credentials.TLSInfo{
		State: cs,
		CommonAuthInfo: credentials.CommonAuthInfo{
			SecurityLevel: credentials.PrivacyAndIntegrity,
		},
	}, nil
}

// SetTerminating marks the server as shutting down. Newly arriving agent
// connections will be rejected and all existing agents will receive a
// ProxyControl{terminating: true} message.
func (s *TunnelServer) SetTerminating() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctxCancel()
}

// Shutdown marks the server as terminating and waits for all active agent
// connections to close, or until ctx expires (in which case all connections
// are force-closed).
func (s *TunnelServer) Shutdown(ctx context.Context) {
	s.SetTerminating()
	defer context.AfterFunc(ctx, func() { _ = s.Close() })()
	s.wg.Wait()
}

// Close immediately terminates all connections and waits for cleanup.
func (s *TunnelServer) Close() error {
	s.mu.Lock()
	s.ctxCancel()
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	for _, cs := range conns {
		for _, c := range cs {
			_ = c.session.Close()
		}
	}
	s.wg.Wait()
	return nil
}

// Dial opens a new stream to the agent identified by hostID and scope for the
// given serviceType, exchanging a DialRequest/DialResponse before returning
// the raw stream as a net.Conn. Returns trace.NotFound if no matching agent
// is connected.
func (s *TunnelServer) Dial(ctx context.Context, hostID string, scope string, serviceType types.TunnelType, src, dst net.Addr) (net.Conn, error) {
	var ac *agentConn
	s.mu.Lock()
	acs := s.conns[connKey{hostID: hostID, scope: scope}]
	if len(acs) > 0 {
		// Prefer the most recently connected agent (last element).
		ac = acs[len(acs)-1]
	}
	s.mu.Unlock()

	if ac == nil {
		return nil, trace.NotFound("no connected agent for host %q scope %q", hostID, scope)
	}

	return ac.dial(ctx, serviceType, src, dst)
}

// handleAgentConn runs the full lifecycle of a single agent connection: yamux
// session setup, AgentHello/ProxyHello exchange, registration, heartbeat loop,
// and cleanup on disconnect.
func (s *TunnelServer) handleAgentConn(c io.ReadWriteCloser, clientID *tlsca.Identity) {
	session, err := yamux.Server(c, yamuxConfig(s.log))
	if err != nil {
		s.log.ErrorContext(context.Background(), "Failed to create yamux session", "error", err)
		_ = c.Close()
		return
	}
	defer session.Close()

	const helloTimeout = 30 * time.Second
	helloDeadline := time.Now().Add(helloTimeout)
	helloCtx, cancel := context.WithDeadline(context.Background(), helloDeadline)
	defer cancel()

	controlStream, err := session.AcceptStreamWithContext(helloCtx)
	if err != nil {
		s.log.ErrorContext(context.Background(), "Failed to accept control stream", "error", err)
		return
	}
	defer controlStream.Close()

	controlStream.SetDeadline(helloDeadline)

	hello := new(reversetunnelv1.AgentHello)
	if err := readProto(controlStream, hello); err != nil {
		s.log.ErrorContext(context.Background(), "Failed to read AgentHello", "error", err)
		return
	}

	services := make([]types.TunnelType, 0, len(hello.GetServices()))
	for _, svc := range hello.GetServices() {
		services = append(services, types.TunnelType(svc))
	}

	// Validate that every advertised service is authorised by the agent's cert.
	if helloErr := validateServices(clientID, services); helloErr != nil {
		_ = writeProto(controlStream, &reversetunnelv1.ProxyHello{
			Status: errToStatusProto(helloErr),
		})
		return
	}

	ac := &agentConn{
		session:     session,
		hostID:      hello.GetHostId(),
		scope:       hello.GetScope(),
		clusterName: hello.GetClusterName(),
		services:    services,
	}

	if !s.addConn(ac) {
		shutdownErr := &trace.ConnectionProblemError{Message: "server is shutting down"}
		_ = writeProto(controlStream, &reversetunnelv1.ProxyHello{
			Status: errToStatusProto(shutdownErr),
		})
		return
	}
	s.log.InfoContext(context.Background(), "Agent connected",
		"host_id", ac.hostID, "scope", ac.scope, "services", services)
	defer s.log.InfoContext(context.Background(), "Agent disconnected",
		"host_id", ac.hostID, "scope", ac.scope)
	defer s.removeConn(ac)

	// Send ProxyHello with the initial gossip payload.
	controlStream.SetDeadline(time.Time{})
	if err := writeProto(controlStream, &reversetunnelv1.ProxyHello{
		ProxyId: s.proxyID,
		Proxies: s.currentProxyEntries(),
	}); err != nil {
		s.log.ErrorContext(context.Background(), "Failed to send ProxyHello", "error", err)
		return
	}

	// Background goroutine: push ProxyControl messages (termination signal,
	// periodic gossip) to the agent.
	go func() {
		ticker := time.NewTicker(proxySyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-session.CloseChan():
				return
			case <-s.ctx.Done():
				_ = writeProto(controlStream, &reversetunnelv1.ProxyControl{
					Terminating: true,
					Proxies:     s.currentProxyEntries(),
				})
				return
			case <-ticker.C:
				_ = writeProto(controlStream, &reversetunnelv1.ProxyControl{
					Proxies: s.currentProxyEntries(),
				})
			}
		}
	}()

	// Foreground: drain AgentControl heartbeats until the agent disconnects.
	for {
		hb := new(reversetunnelv1.AgentControl)
		if err := readProto(controlStream, hb); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			s.log.WarnContext(context.Background(), "Error reading AgentControl", "error", err)
			break
		}
	}
}

// proxySyncInterval is how often the proxy pushes a fresh gossip proxy list
// to each connected agent.
const proxySyncInterval = 30 * time.Second

// currentProxyEntries converts the result of GetProxies() into the wire
// representation used in ProxyHello and ProxyControl.
func (s *TunnelServer) currentProxyEntries() []*reversetunnelv1.ProxyEntry {
	proxies := s.getProxies()
	entries := make([]*reversetunnelv1.ProxyEntry, 0, len(proxies))
	for _, p := range proxies {
		entry := &reversetunnelv1.ProxyEntry{
			Name: p.GetName(),
		}
		labels := p.GetStaticLabels()
		entry.GroupId = labels["teleport.internal/proxygroup-id"]
		if genStr, ok := labels["teleport.internal/proxygroup-gen"]; ok {
			if gen, err := strconv.ParseUint(genStr, 10, 64); err == nil {
				entry.Generation = gen
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// addConn registers ac in the connection table. Returns false if the server
// is already shutting down.
func (s *TunnelServer) addConn(ac *agentConn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx.Err() != nil {
		return false
	}

	s.wg.Add(1)

	if s.conns == nil {
		s.conns = make(map[connKey][]*agentConn)
	}

	ck := connKey{hostID: ac.hostID, scope: ac.scope}
	s.conns[ck] = append(s.conns[ck], ac)
	return true
}

// removeConn unregisters ac from the connection table and signals the
// WaitGroup.
func (s *TunnelServer) removeConn(ac *agentConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.wg.Done()

	ck := connKey{hostID: ac.hostID, scope: ac.scope}
	conns := s.conns[ck]
	idx := slices.Index(conns, ac)
	if idx < 0 {
		return
	}
	s.conns[ck] = slices.Delete(conns, idx, idx+1)
}

// agentConn represents a single live yamux session from an agent to this proxy.
// It is created by handleAgentConn after a successful handshake.
type agentConn struct {
	session     *yamux.Session
	hostID      string
	scope       string
	clusterName string
	services    []types.TunnelType
}

// dial opens a new yamux stream on the agent's session, sends a DialRequest,
// waits for a DialResponse, and returns the stream as a net.Conn.
func (ac *agentConn) dial(ctx context.Context, serviceType types.TunnelType, src, dst net.Addr) (net.Conn, error) {
	stream, err := ac.session.OpenStream()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use context cancellation to abort the request-response non-destructively.
	explode := make(chan struct{})
	defuse := context.AfterFunc(ctx, func() {
		defer close(explode)
		stream.SetDeadline(time.Unix(1, 0))
	})
	defer defuse()

	req := &reversetunnelv1.DialRequest{
		ServiceType: string(serviceType),
		Source:      addrToProto(src),
		Destination: addrToProto(dst),
	}
	if err := writeProto(stream, req); err != nil {
		defuse()
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	resp := new(reversetunnelv1.DialResponse)
	if err := readProto(stream, resp); err != nil {
		defuse()
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	if defuse() {
		close(explode)
	}

	if err := trail.FromGRPC(grpcstatus.FromProto(resp.GetStatus()).Err()); err != nil {
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	<-explode
	stream.SetDeadline(time.Time{})

	return &yamuxStreamConn{
		Stream:     stream,
		localAddr:  src,
		remoteAddr: dst,
	}, nil
}

// errToStatusProto converts an error to the google.rpc.Status proto message
// embedded in ProxyHello and DialResponse. A nil error produces nil (OK).
func errToStatusProto(err error) *statuspb.Status {
	if err == nil {
		return nil
	}
	return grpcstatus.Convert(trail.ToGRPC(err)).Proto()
}
