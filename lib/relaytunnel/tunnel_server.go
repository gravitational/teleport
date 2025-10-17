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
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/tlsca"
)

// ServerConfig contains the parameters for [NewServer].
type ServerConfig struct {
	Log *slog.Logger

	GetCertificate func(ctx context.Context) (*tls.Certificate, error)
	GetPool        func(ctx context.Context) (*x509.CertPool, error)
	Ciphersuites   []uint16

	RelayGroup            string
	TargetConnectionCount int32
}

// NewServer creates a [Server] with a given configuration.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Log == nil {
		return nil, trace.BadParameter("missing Log")
	}
	if cfg.GetCertificate == nil {
		return nil, trace.BadParameter("missing GetCertificate")
	}
	if cfg.GetPool == nil {
		return nil, trace.BadParameter("missing GetPool")
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	return &Server{
		log: cfg.Log,

		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,

		relayGroup:            cfg.RelayGroup,
		targetConnectionCount: cfg.TargetConnectionCount,

		ctx:       ctx,
		ctxCancel: ctxCancel,
	}, nil
}

// Server is a relay tunnel server
type Server struct {
	log *slog.Logger

	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16

	relayGroup            string
	targetConnectionCount int32

	mu sync.Mutex

	// ctx has to synchronize with wg, so it should be a context that cannot be
	// closed externally.
	ctx       context.Context
	ctxCancel context.CancelFunc

	wg sync.WaitGroup

	// conns holds client connections per host ID and tunnel type. The
	// connection received the latest is always the one that will be used for
	// dialing, so order matters.
	conns map[connKey][]serverConn
}

// GRPCServerCredentials returns some gRPC [credentials.TransportCredentials]
// that will pass connections with a negotiated protocol of "h2" to the gRPC
// server, while dispatching tunnel connections to the [Server].
func (s *Server) GRPCServerCredentials() credentials.TransportCredentials {
	return &grpcServerCredentials{
		tunnelServer: s,

		getCertificate: s.getCertificate,
		getPool:        s.getPool,
		ciphersuites:   s.ciphersuites,
	}
}

type grpcServerCredentials struct {
	tunnelServer *Server

	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16
}

// ClientHandshake implements [credentials.TransportCredentials].
func (*grpcServerCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	_ = rawConn.Close()
	return nil, nil, trace.NotImplemented("these transport credentials can only be used as a server")
}

// OverrideServerName implements implements [credentials.TransportCredentials].
func (*grpcServerCredentials) OverrideServerName(string) error {
	return nil
}

// Clone implements implements [credentials.TransportCredentials].
func (s *grpcServerCredentials) Clone() credentials.TransportCredentials {
	// s is immutable so there's no need to copy anything
	return s
}

// Info implements implements [credentials.TransportCredentials].
func (s *grpcServerCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
	}
}

// ServerHandshake implements implements [credentials.TransportCredentials].
func (s *grpcServerCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cert, err := s.getCertificate(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}
	pool, err := s.getPool(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	var clientID *tlsca.Identity
	tlsConfig := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				// client tried to connect with no ALPN (or with http/1.1 in its
				// protocol list because of an undocumented behavior of the
				// crypto/tls server handshake)
				return trace.NotImplemented("missing ALPN in TLS ClientHello")
			}
			if len(cs.VerifiedChains) < 1 {
				return trace.AccessDenied("missing or invalid client certificate")
			}

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
		NextProtos: []string{yamuxTunnelALPN, "h2"},

		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,

		InsecureSkipVerify: false,

		MinVersion:             tls.VersionTLS12,
		CipherSuites:           s.ciphersuites,
		SessionTicketsDisabled: true,
	}

	tlsConn := tls.Server(rawConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		_ = tlsConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	cs := tlsConn.ConnectionState()
	if cs.NegotiatedProtocol == yamuxTunnelALPN {
		go s.tunnelServer.handleYamuxTunnel(tlsConn, clientID)
		return nil, nil, credentials.ErrConnDispatched
	}
	tlsInfo := credentials.TLSInfo{
		State: cs,
		CommonAuthInfo: credentials.CommonAuthInfo{
			SecurityLevel: credentials.PrivacyAndIntegrity,
		},
	}

	return tlsConn, tlsInfo, nil
}

func (s *Server) SetTerminating() {
	// we have to acquire the lock while canceling the context because we
	// shouldn't be adding to the connection map if we're terminating
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setTerminatingLocked()
}

func (s *Server) setTerminatingLocked() {
	s.ctxCancel()
}

// Shutdown will mark the server as terminating and then wait until either all
// connections are gone or until the context expires (at which point all
// connections are terminated).
func (s *Server) Shutdown(ctx context.Context) {
	s.SetTerminating()

	defer context.AfterFunc(ctx, func() { _ = s.Close() })()

	// we can wait after ensuring that the server is terminating, because we
	// only Add if the server is not terminating
	s.wg.Wait()
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.setTerminatingLocked()
	// we might have a lot of connections, so we take away the map and close
	// them serially while the lock is not held
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	for _, connSlice := range conns {
		for _, conn := range connSlice {
			_ = conn.Close()
		}
	}

	// we can wait after ensuring that the server is terminating, because we
	// only Add if the server is not terminating
	s.wg.Wait()
	return nil
}

func (s *Server) handleYamuxTunnel(c io.ReadWriteCloser, clientID *tlsca.Identity) error {
	// this is a copy of the default config as returned by [yamux.DefaultConfig]
	// at the time of writing with a slightly tighter timing for
	// StreamOpenTimeout (because we expect the dialing stream open requests to
	// be handled very promptly by the client) and our logger adapter
	cfg := &yamux.Config{
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
		Logger:    (*yamuxLogger)(s.log),
	}

	session, err := yamux.Server(c, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer session.Close()

	const helloTimeout = 30 * time.Second
	helloDeadline := time.Now().Add(helloTimeout)
	helloCtx, cancel := context.WithDeadline(context.Background(), helloDeadline)
	defer cancel()

	controlStream, err := session.AcceptStreamWithContext(helloCtx)
	if err != nil {
		return err
	}
	defer controlStream.Close()

	controlStream.SetDeadline(helloDeadline)

	clientHello := new(relaytunnelv1alpha.ClientHello)
	if err := readProto(controlStream, clientHello); err != nil {
		return trace.Wrap(err)
	}

	tunnelType := types.TunnelType(clientHello.GetTunnelType())
	helloErr := func() error {
		var requiredRole types.SystemRole
		switch tunnelType {
		case types.NodeTunnel:
			requiredRole = types.RoleNode
		default:
			return trace.BadParameter("unsupported tunnel type %q", tunnelType)
		}
		if !slices.Contains(clientID.Groups, string(requiredRole)) && !slices.Contains(clientID.SystemRoles, string(requiredRole)) {
			return trace.AccessDenied("required role %q not in client identity", requiredRole)
		}
		return nil
	}()
	if helloErr != nil {
		_ = writeProto(controlStream, &relaytunnelv1alpha.ServerHello{
			Status: status.Convert(trail.ToGRPC(helloErr)).Proto(),
		})
		return trace.Wrap(helloErr)
	}

	sc := &yamuxServerConn{
		session: session,
	}
	if !s.addConn(clientID.Username, tunnelType, sc) {
		// this can happen if a connection gets routed to our listener after we
		// have advertised termination but before the load balancer has stopped
		// sending connections our way
		err := &trace.ConnectionProblemError{Message: "server is shutting down"}
		_ = writeProto(controlStream, &relaytunnelv1alpha.ServerHello{
			Status: status.Convert(trail.ToGRPC(err)).Proto(),
		})
		return trace.Wrap(err)
	}
	s.log.InfoContext(context.Background(), "new client connected", "client_id", clientID.Username, "tunnel_type", tunnelType)
	defer s.log.InfoContext(context.Background(), "client disconnected", "client_id", clientID.Username, "tunnel_type", tunnelType)
	defer s.removeConn(clientID.Username, tunnelType, sc)

	controlStream.SetDeadline(time.Time{})

	if err := writeProto(controlStream, &relaytunnelv1alpha.ServerHello{
		Status: nil, // i.e. status.Convert(error(nil)).Proto()

		RelayGroup:            s.relayGroup,
		TargetConnectionCount: s.targetConnectionCount,
	}); err != nil {
		return trace.Wrap(err)
	}

	go func() {
		// we unblock if the session gets closed rather than the control
		// stream but currently they have the same lifetime and the session
		// has a convenient channel to wait on
		select {
		case <-session.CloseChan():
		case <-s.ctx.Done():
		}

		// currently the only message we have to send and we only send it once
		_ = writeProto(controlStream, &relaytunnelv1alpha.ServerControl{
			Terminating: true,
		})
	}()

	for {
		// TODO(espadolini): add a way to reuse buffers and allocated messages
		// for the control stream messages
		controlMsg := new(relaytunnelv1alpha.ClientControl)
		if err := readProto(controlStream, controlMsg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *Server) Dial(ctx context.Context, hostID string, tunnelType types.TunnelType, src, dst net.Addr) (net.Conn, error) {
	var sc serverConn
	s.mu.Lock()
	scs := s.conns[connKey{
		hostID:     hostID,
		tunnelType: tunnelType,
	}]
	if len(scs) > 0 {
		// new tunnels are appended to the store so by always taking the last we
		// are prioritizing the last tunnel that connected to us, following the
		// same logic as reversetunnel.localCluster.getRemoteConn
		//
		// TODO(espadolini): think about passing a nonce and a generation
		// counter that gets bumped whenever we reload Teleport so newer
		// executions of the same instance are guaranteed to be connected last;
		// this applies to the reverse tunnel too
		sc = scs[len(scs)-1]
	}
	s.mu.Unlock()

	if sc == nil {
		return nil, trace.NotFound("dial target not found among connected tunnels")
	}

	rwc, err := sc.dial(ctx, src, dst)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rwc, nil
}

type connKey struct {
	hostID     string
	tunnelType types.TunnelType
}

func (s *Server) addConn(hostID string, tunnelType types.TunnelType, conn serverConn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx.Err() != nil {
		return false
	}

	s.wg.Add(1)

	if s.conns == nil {
		s.conns = make(map[connKey][]serverConn)
	}

	ck := connKey{
		hostID:     hostID,
		tunnelType: tunnelType,
	}
	s.conns[ck] = append(s.conns[ck], conn)
	return true
}

func (s *Server) removeConn(hostID string, tunnelType types.TunnelType, conn serverConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.wg.Done()

	ck := connKey{
		hostID:     hostID,
		tunnelType: tunnelType,
	}
	conns := s.conns[ck]
	idx := slices.Index(conns, conn)
	if idx < 0 {
		return
	}
	s.conns[ck] = slices.Delete(conns, idx, idx+1)
}

type serverConn interface {
	io.Closer

	dial(ctx context.Context, src, dst net.Addr) (net.Conn, error)
}

type yamuxServerConn struct {
	session *yamux.Session
}

// Close implements [serverConn].
func (c *yamuxServerConn) Close() error {
	return trace.Wrap(c.session.Close())
}

var _ serverConn = (*yamuxServerConn)(nil)

// dial implements [serverConn].
func (c *yamuxServerConn) dial(ctx context.Context, src net.Addr, dst net.Addr) (net.Conn, error) {
	stream, err := c.session.OpenStream()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// non-destructively stop the dial request-response when the dial context is
	// canceled
	explode := make(chan struct{})
	defuse := context.AfterFunc(ctx, func() {
		defer close(explode)
		stream.SetDeadline(time.Unix(1, 0))
	})
	defer defuse()

	req := &relaytunnelv1alpha.DialRequest{
		Source:      addrToProto(src),
		Destination: addrToProto(dst),
	}
	if err := writeProto(stream, req); err != nil {
		defuse()
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	resp := new(relaytunnelv1alpha.DialResponse)
	if err := readProto(stream, resp); err != nil {
		defuse()
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	if defuse() {
		close(explode)
	}

	if err := status.FromProto(resp.GetStatus()).Err(); err != nil {
		_ = stream.Close()
		return nil, trail.FromGRPC(err)
	}

	<-explode
	stream.SetDeadline(time.Time{})

	nc := &yamuxStreamConn{
		Stream: stream,

		// on this side of the connection we are the source and the peer is the
		// destination, the tunnel client will do the opposite
		localAddr:  src,
		remoteAddr: dst,
	}

	return nc, nil
}
