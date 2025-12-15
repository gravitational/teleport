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

package relaypeer

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
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaypeeringv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaypeering/v1alpha"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type dialFunc = func(ctx context.Context, dialTarget string, tunnelType types.TunnelType, src, dst net.Addr) (net.Conn, error)

// ServerConfig contains parameters for [NewServer].
type ServerConfig struct {
	Log *slog.Logger

	GetCertificate func(ctx context.Context) (*tls.Certificate, error)
	GetPool        func(ctx context.Context) (*x509.CertPool, error)
	Ciphersuites   []uint16

	// LocalDial should dial the given target host (in "<host ID>.<cluster
	// name>" form) for a given tunnel type, returning a connection with the
	// given source (remote) address and destination (local) address.
	LocalDial dialFunc
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
	if cfg.LocalDial == nil {
		return nil, trace.BadParameter("missing LocalDial")
	}
	ctx, ctxCancel := context.WithCancel(context.Background())
	return &Server{
		log: cfg.Log,

		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,

		localDial: cfg.LocalDial,

		ctx:       ctx,
		ctxCancel: ctxCancel,
	}, nil
}

// Server manages listeners and accepts connections for the relay peering dial
// protocol, used by other relays in the same relay group to bounce connections
// for which the local relay hopefully has a tunnel but the peer relay does not.
// It implements the server side of the relay peering dial protocol. The
// relay_server heartbeat for the local relay should be advertising a peer
// address that lets the other relays reach the listener (or listeners) of this
// server.
type Server struct {
	log *slog.Logger

	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16

	localDial dialFunc

	mu sync.Mutex

	wg sync.WaitGroup

	// ctx should only be canceled while holding mu to synchronize adding
	// connections and listeners to the maps and closing them, as well as adding
	// tasks to wg, so it should not be externally cancelable.
	ctx       context.Context
	ctxCancel context.CancelFunc

	tlsListeners map[net.Listener]struct{}
	conns        map[io.Closer]struct{}
}

func (s *Server) ServeTLSListener(l net.Listener) error {
	defer l.Close()

	s.mu.Lock()
	if s.ctx.Err() != nil {
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
		if s.ctx.Err() != nil {
			s.log.DebugContext(s.ctx, "Exiting due to requested termination")
			return nil
		}

		c, err := l.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				continue
			}

			if tempErr := *new(interface{ Temporary() bool }); errors.As(err, &tempErr) && tempErr.Temporary() {
				tempDelay = max(5*time.Millisecond, min(2*tempDelay, time.Second))
				select {
				case <-time.After(tempDelay):
				case <-s.ctx.Done():
				}
				continue
			}
			return trace.Wrap(err)
		}
		tempDelay = 0

		s.mu.Lock()
		if s.ctx.Err() != nil {
			// a connection sneaked by right before we closed the listener
			s.mu.Unlock()
			_ = c.Close()
			continue
		}
		if s.conns == nil {
			s.conns = make(map[io.Closer]struct{})
		}
		s.conns[c] = struct{}{}
		s.mu.Unlock()

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.conns, c)
				s.mu.Unlock()
			}()
			err := s.handleTLSConnection(c)
			s.log.DebugContext(context.Background(), "Finished handling peer connection", "error", err)
		}()
	}
}

func (s *Server) Close() error {
	s.mu.Lock()
	tlsListeners := s.tlsListeners
	s.tlsListeners = nil
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	for l := range tlsListeners {
		_ = l.Close()
	}
	for c := range conns {
		_ = c.Close()
	}

	s.wg.Wait()
	return nil
}

func (s *Server) handleTLSConnection(nc net.Conn) error {
	handshakeDeadline := time.Now().Add(30 * time.Second)
	handshakeCtx, cancel := context.WithDeadline(context.Background(), handshakeDeadline)
	defer cancel()

	cert, err := s.getCertificate(handshakeCtx)
	if err != nil {
		_ = nc.Close()
		return trace.Wrap(err)
	}
	pool, err := s.getPool(handshakeCtx)
	if err != nil {
		_ = nc.Close()
		return trace.Wrap(err)
	}

	tlsConfig := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				return trace.NotImplemented("relay peering protocol not supported")
			}
			if len(cs.VerifiedChains) < 1 {
				return trace.AccessDenied("missing or invalid client certificate")
			}

			id, err := tlsca.FromSubject(cs.VerifiedChains[0][0].Subject, cs.VerifiedChains[0][0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}

			if !slices.Contains(id.Groups, string(types.RoleRelay)) &&
				!slices.Contains(id.SystemRoles, string(types.RoleRelay)) {
				return trace.BadParameter("client is not a relay (roles %+q, system roles %+q)", id.Groups, id.SystemRoles)
			}

			return nil
		},
		NextProtos: []string{simpleALPN},

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
		return trace.Wrap(err)
	}

	// the only possible negotiated protocol is here is the only supported one,
	// "teleport-relaypeer"

	tc.SetDeadline(handshakeDeadline)

	req := new(relaypeeringv1alpha.DialRequest)
	if err := readProto(tc, req); err != nil {
		return trace.Wrap(err)
	}

	lc, err := s.localDial(
		handshakeCtx,
		req.GetTargetHostId(),
		types.TunnelType(req.GetConnectionType()),
		addrFromProto(req.GetSource()),
		addrFromProto(req.GetDestination()),
	)
	if err != nil {
		_ = writeProto(tc, &relaypeeringv1alpha.DialResponse{
			Status: status.Convert(trail.ToGRPC(err)).Proto(),
		})
		return trace.Wrap(err)
	}
	defer lc.Close()

	if err := writeProto(tc, &relaypeeringv1alpha.DialResponse{
		Status: nil, // i.e. status.Convert(error(nil)).Proto()
	}); err != nil {
		return trace.Wrap(err)
	}

	tc.SetDeadline(time.Time{})

	return utils.ProxyConn(context.Background(), lc, tc)
}
