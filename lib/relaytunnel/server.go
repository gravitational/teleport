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
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
)

const maxMessageSize = 128 * 1024

type ServerConfig struct {
	Log *slog.Logger

	GetCertificate func(ctx context.Context) (*tls.Certificate, error)
	GetPool        func(ctx context.Context) (*x509.CertPool, error)
	Ciphersuites   []uint16

	RelayGroup            string
	TunnelPublicAddr      string
	TargetConnectionCount int32
}

func NewServer(cfg ServerConfig) (*Server, error) {
	return &Server{
		log: cfg.Log,

		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,

		relayGroup:            cfg.RelayGroup,
		tunnelPublicAddr:      cfg.TunnelPublicAddr,
		targetConnectionCount: cfg.TargetConnectionCount,

		terminatingC: make(chan struct{}),
	}, nil
}

type Server struct {
	log *slog.Logger

	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16

	relayGroup            string
	tunnelPublicAddr      string
	targetConnectionCount int32

	mu sync.Mutex

	wg           sync.WaitGroup
	terminatingC chan struct{}

	tlsListeners map[net.Listener]struct{}

	// conns holds client connections.
	conns map[connKey][]serverConn
}

func (s *Server) SetTerminating() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setTerminatingLocked()
}

func (s *Server) setTerminatingLocked() {
	if s.terminatingC == nil {
		return
	}
	close(s.terminatingC)
	s.terminatingC = nil
}

func (s *Server) Shutdown(ctx context.Context) {
	s.mu.Lock()
	s.setTerminatingLocked()
	tlsListeners := s.tlsListeners
	s.tlsListeners = nil
	s.mu.Unlock()

	for l := range tlsListeners {
		_ = l.Close()
	}

	defer context.AfterFunc(ctx, func() {
		s.mu.Lock()
		conns := s.conns
		s.conns = nil
		s.mu.Unlock()

		for _, connSlice := range conns {
			for _, conn := range connSlice {
				conn.close()
			}
		}
	})()

	s.wg.Wait()
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.setTerminatingLocked()
	tlsListeners := s.tlsListeners
	s.tlsListeners = nil
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	for l := range tlsListeners {
		_ = l.Close()
	}
	for _, connSlice := range conns {
		for _, conn := range connSlice {
			conn.close()
		}
	}

	s.wg.Wait()
	return nil
}

func (s *Server) ServeTLSTunnelListener(l net.Listener) error {
	defer l.Close()

	s.mu.Lock()
	if s.terminatingC == nil {
		s.mu.Unlock()
		return trace.Errorf("server is already terminating")
	}

	s.wg.Add(1)
	defer s.wg.Done()

	if s.tlsListeners == nil {
		s.tlsListeners = make(map[net.Listener]struct{})
	}
	s.tlsListeners[l] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.tlsListeners, l)
		s.mu.Unlock()
	}()

	var tempDelay time.Duration
	for {
		c, err := l.Accept()
		if err != nil {
			s.mu.Lock()
			terminatingC := s.terminatingC
			s.mu.Unlock()
			if terminatingC == nil {
				return nil
			}
			if tempErr := *new(interface{ Temporary() bool }); errors.As(err, &tempErr) && tempErr.Temporary() {
				tempDelay = max(5*time.Millisecond, min(2*tempDelay, time.Second))
				select {
				case <-time.After(tempDelay):
				case <-terminatingC:
					return nil
				}
				continue
			}
			return trace.Wrap(err)
		}
		tempDelay = 0

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleTLSTunnel(c)
		}()
	}
}

func (s *Server) handleTLSTunnel(nc net.Conn) {
	handshakeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cert, err := s.getCertificate(handshakeCtx)
	if err != nil {
		_ = nc.Close()
		return
	}
	pool, err := s.getPool(handshakeCtx)
	if err != nil {
		_ = nc.Close()
		return
	}

	var clientID *tlsca.Identity
	tlsConfig := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				return trace.NotImplemented("relay tunnel protocol not supported")
			}
			if len(cs.VerifiedChains) < 1 {
				return trace.AccessDenied("missing or invalid client certificate")
			}

			id, err := tlsca.FromSubject(cs.VerifiedChains[0][0].Subject, cs.VerifiedChains[0][0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}
			clientID = id

			return nil
		},
		NextProtos: []string{yamuxTunnelALPN},

		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,

		InsecureSkipVerify: false,

		MinVersion:             tls.VersionTLS12,
		CipherSuites:           s.ciphersuites,
		SessionTicketsDisabled: true,
	}

	tc := tls.Server(nc, tlsConfig)
	defer tc.Close()

	if err := tc.HandshakeContext(handshakeCtx); err != nil {
		return
	}

	// the only possible value for tc.ConnectionState().NegotiatedProtocol is yamuxTunnelALPN
	s.handleYamuxTunnel(tc, clientID)
}

func (s *Server) handleYamuxTunnel(c io.ReadWriteCloser, clientID *tlsca.Identity) error {
	cfg := &yamux.Config{
		AcceptBacklog: 128,

		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,

		MaxStreamWindowSize: 256 * 1024,

		StreamCloseTimeout: time.Minute,
		StreamOpenTimeout:  30 * time.Second,

		LogOutput: nil,
		Logger:    (*yamuxLogger)(slog.Default()),
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
	roleErr := func() error {
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
	if roleErr != nil {
		_ = writeProto(controlStream, &relaytunnelv1alpha.ServerHello{
			Status: status.Convert(trail.ToGRPC(roleErr)).Proto(),
		})
		return trace.Wrap(roleErr)
	}

	sc := &yamuxServerConn{
		session: session,
	}
	if !s.addConn(clientID.Username, tunnelType, sc) {
		err := trace.Errorf("server is shutting down")
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
		TunnelPublicAddr:      s.tunnelPublicAddr,
		TargetConnectionCount: s.targetConnectionCount,
	}); err != nil {
		return trace.Wrap(err)
	}

	go func() {
		s.mu.Lock()
		terminatingC := s.terminatingC
		s.mu.Unlock()
		if terminatingC != nil {
			// we unblock if the session gets closed rather than the control
			// stream but currently they have the same lifetime and the session
			// has a convenient channel to wait on
			select {
			case <-session.CloseChan():
				return
			case <-terminatingC:
			}
		}

		// currently the only message we have to send and we only send it once
		_ = writeProto(controlStream, &relaytunnelv1alpha.ServerControl{
			Terminating: true,
		})
	}()

	for {
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

	if s.terminatingC == nil {
		return false
	}

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

type connDial struct {
	dialRequest reversetunnelclient.DialParams
}

type serverConn interface {
	dial(ctx context.Context, src, dst net.Addr) (net.Conn, error)

	close()
}

type yamuxServerConn struct {
	session *yamux.Session
}

// close implements serverConn.
func (c *yamuxServerConn) close() {
	_ = c.session.Close()
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

	return stream, nil
}
