// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	quicpeeringv1a "github.com/gravitational/teleport/gen/proto/go/teleport/quicpeering/v1alpha"
	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
	"github.com/gravitational/teleport/lib/proxy/peer/internal"
	"github.com/gravitational/teleport/lib/utils"
)

// ServerConfig holds the parameters for [NewServer].
type ServerConfig struct {
	Log *slog.Logger
	// Dialer is the dialer used to open connections to agents on behalf of the
	// peer proxies. Required.
	Dialer peerdial.Dialer

	// GetCertificate should return the server certificate at time of use. It
	// should be a certificate with the Proxy host role. Required.
	GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// GetClientCAs should return the certificate pool that should be used to
	// validate the client certificates of peer proxies; i.e., a pool containing
	// the trusted signers for the certificate authority of the local cluster.
	// Required.
	GetClientCAs func(*tls.ClientHelloInfo) (*x509.CertPool, error)
}

func (c *ServerConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.Default()
	}
	c.Log = c.Log.With(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentProxy, "qpeer"),
	)

	if c.Dialer == nil {
		return trace.BadParameter("missing Dialer")
	}

	if c.GetCertificate == nil {
		return trace.BadParameter("missing GetCertificate")
	}
	if c.GetClientCAs == nil {
		return trace.BadParameter("missing GetClientCAs")
	}

	return nil
}

// Server is a proxy peering server that uses the QUIC protocol.
type Server struct {
	log        *slog.Logger
	dialer     peerdial.Dialer
	tlsConfig  *tls.Config
	quicConfig *quic.Config

	replayStore replayStore

	// runCtx is a context that gets canceled when all connections should be
	// ungracefully terminated.
	runCtx context.Context
	// runCancel cancels runCtx.
	runCancel context.CancelFunc
	// serveCtx is a context that gets canceled when all listeners should stop
	// accepting new connections.
	serveCtx context.Context
	// serveCancel cancels serveCtx.
	serveCancel context.CancelFunc

	// mu protects everything further in the struct.
	mu sync.Mutex
	// closed is set at the beginning of shutdown. When set, nothing is allowed
	// to increase the waitgroup count past 0.
	closed bool
	// wg counts any active listener and connection. Should only be increased
	// from 0 while holding mu, if the closed flag is not set. Should only be
	// waited after setting the closed flag.
	wg sync.WaitGroup
}

// NewServer returns a [Server] with the given config.
func NewServer(cfg ServerConfig) (*Server, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// crypto/tls doesn't allow us to configure TLS 1.3 ciphersuites, and the
	// only other effect of [utils.TLSConfig] is to require at least TLS 1.2,
	// but QUIC requires at least TLS 1.3 anyway
	tlsConfig := &tls.Config{
		GetCertificate:        cfg.GetCertificate,
		VerifyPeerCertificate: internal.VerifyPeerCertificateIsProxy,
		NextProtos:            []string{nextProto},
		ClientAuth:            tls.RequireAndVerifyClientCert,
		MinVersion:            tls.VersionTLS13,
	}

	getClientCAs := cfg.GetClientCAs
	tlsConfig.GetConfigForClient = func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		clientCAs, err := getClientCAs(chi)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		utils.RefreshTLSConfigTickets(tlsConfig)
		c := tlsConfig.Clone()
		c.ClientCAs = clientCAs
		return c, nil
	}

	quicConfig := &quic.Config{
		MaxStreamReceiveWindow:     maxReceiveWindow,
		MaxConnectionReceiveWindow: maxReceiveWindow,

		MaxIncomingStreams:    maxIncomingStreams,
		MaxIncomingUniStreams: -1,

		MaxIdleTimeout:  maxIdleTimeout,
		KeepAlivePeriod: keepAlivePeriod,

		Allow0RTT: true,
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	serveCtx, serveCancel := context.WithCancel(runCtx)

	return &Server{
		log:        cfg.Log,
		dialer:     cfg.Dialer,
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,

		runCtx:      runCtx,
		runCancel:   runCancel,
		serveCtx:    serveCtx,
		serveCancel: serveCancel,
	}, nil
}

// Serve opens a listener and serves incoming connection. Returns after calling
// Close or Shutdown.
func (s *Server) Serve(transport *quic.Transport) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return trace.Wrap(quic.ErrServerClosed)
	}
	s.wg.Add(1)
	defer s.wg.Done()
	s.mu.Unlock()

	lis, err := transport.ListenEarly(s.tlsConfig, s.quicConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer lis.Close()
	defer context.AfterFunc(s.serveCtx, func() { _ = lis.Close() })()

	for {
		// the listener will be closed when serveCtx is done, but Accept will
		// return any queued connection before erroring out with a
		// [quic.ErrServerClosed]
		c, err := lis.Accept(context.Background())
		if err != nil {
			return trace.Wrap(err)
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(c)
		}()
	}
}

func (s *Server) handleConn(conn quic.EarlyConnection) {
	log := s.log.With(
		"remote_addr", conn.RemoteAddr().String(),
		"internal_id", uuid.NewString(),
	)
	state := conn.ConnectionState()
	log.InfoContext(conn.Context(),
		"handling new peer connection",
		"gso", state.GSO,
		"used_0rtt", state.Used0RTT,
	)
	defer func() {
		log.DebugContext(conn.Context(),
			"peer connection closed",
			"error", ignoreCodeZero(context.Cause(conn.Context())),
		)
	}()

	defer conn.CloseWithError(noApplicationErrorCode, "")
	defer context.AfterFunc(s.runCtx, func() { _ = conn.CloseWithError(noApplicationErrorCode, "") })()

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		// TODO(espadolini): stop accepting new streams once s.serveCtx is
		// canceled, once quic-go gains the ability to change the amount of
		// available streams during a connection (so we can set it to 0)
		st, err := conn.AcceptStream(context.Background())
		if err != nil {
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handleStream(st, conn, log)
		}()
	}
}

func (s *Server) handleStream(stream quic.Stream, conn quic.EarlyConnection, log *slog.Logger) {
	log = log.With("stream_id", stream.StreamID())
	log.DebugContext(conn.Context(), "handling stream")
	defer log.DebugContext(conn.Context(), "done handling stream")

	var closedStream bool
	defer func() {
		stream.CancelRead(genericStreamErrorCode)
		if !closedStream {
			stream.CancelWrite(genericStreamErrorCode)
		}
	}()

	sendErr := func(toSend error) {
		stream.CancelRead(noStreamErrorCode)
		errBuf, err := marshalSized(&quicpeeringv1a.DialResponse{
			Status: status.Convert(trail.ToGRPC(toSend)).Proto(),
		})
		if err != nil {
			return
		}
		if len(errBuf)-4 > maxMessageSize {
			log.WarnContext(conn.Context(), "refusing to send oversized error message (this is a bug)")
			return
		}
		stream.SetWriteDeadline(time.Now().Add(errorResponseTimeout))
		if _, err := stream.Write(errBuf); err != nil {
			return
		}
		_ = stream.Close()
		closedStream = true
	}

	stream.SetReadDeadline(time.Now().Add(requestTimeout))
	var reqLen uint32
	if err := binary.Read(stream, binary.LittleEndian, &reqLen); err != nil {
		log.DebugContext(conn.Context(), "failed to read request size", "error", err)
		return
	}
	if reqLen >= maxMessageSize {
		log.WarnContext(conn.Context(), "received oversized request", "request_len", reqLen)
		return
	}
	reqBuf := make([]byte, reqLen)
	if _, err := io.ReadFull(stream, reqBuf); err != nil {
		log.DebugContext(conn.Context(), "failed to read request", "error", err)
		return
	}
	stream.SetReadDeadline(time.Time{})

	req := new(quicpeeringv1a.DialRequest)
	if err := proto.Unmarshal(reqBuf, req); err != nil {
		log.WarnContext(conn.Context(), "failed to unmarshal request", "error", err)
		return
	}

	if req.GetPing() {
		log.DebugContext(conn.Context(),
			"received ping request",
			"nonce", req.GetNonce(),
			"request_timestamp", req.GetTimestamp().AsTime(),
		)
		select {
		case <-conn.HandshakeComplete():
		case <-conn.Context().Done():
			log.DebugContext(conn.Context(),
				"handshake failure or connection loss after receiving ping request",
				"error", context.Cause(conn.Context()),
			)
			return
		}
		stream.CancelRead(noStreamErrorCode)
		if _, err := stream.Write([]byte(dialResponseOK)); err != nil {
			return
		}
		_ = stream.Close()
		closedStream = true
		return
	}

	if requestTimestamp := req.GetTimestamp().AsTime(); time.Since(requestTimestamp).Abs() > timestampGraceWindow {
		log.WarnContext(conn.Context(),
			"dial request has out of sync timestamp, 0-RTT performance will be impacted",
			"request_timestamp", requestTimestamp,
		)
		select {
		case <-conn.HandshakeComplete():
		case <-conn.Context().Done():
			// logging this at warn level because it should be very atypical to
			// begin with, and it might be a symptom of a malicious actor
			// interfering with the connection
			log.WarnContext(conn.Context(),
				"handshake failure or connection loss after receiving dial request with out of sync timestamp",
				"error", context.Cause(conn.Context()),
			)
			return
		}
	}

	// a replayed request is always wrong even after a full handshake, the
	// replay might've happened before the legitimate request
	if !s.replayStore.add(req.GetNonce(), time.Now()) {
		log.ErrorContext(conn.Context(), "request is reusing a nonce, rejecting", "nonce", req.GetNonce())
		sendErr(trace.BadParameter("reused or invalid nonce"))
		return
	}

	_, clusterName, ok := strings.Cut(req.GetTargetHostId(), ".")
	if !ok {
		sendErr(trace.BadParameter("server_id %q is missing cluster information", req.GetTargetHostId()))
		return
	}

	nodeConn, err := s.dialer.Dial(clusterName, peerdial.DialParams{
		From: &utils.NetAddr{
			Addr:        req.GetSource().GetAddr(),
			AddrNetwork: req.GetSource().GetNetwork(),
		},
		To: &utils.NetAddr{
			Addr:        req.GetDestination().GetAddr(),
			AddrNetwork: req.GetDestination().GetNetwork(),
		},
		ServerID: req.GetTargetHostId(),
		ConnType: types.TunnelType(req.GetConnectionType()),
	})
	if err != nil {
		sendErr(err)
		return
	}

	var eg errgroup.Group
	eg.Go(func() error {
		defer nodeConn.Close()
		if _, err := stream.Write([]byte(dialResponseOK)); err != nil {
			return trace.Wrap(err)
		}
		if _, err := ignoreCodeZero2(io.Copy(stream, nodeConn)); err != nil && !utils.IsOKNetworkError(err) {
			return trace.Wrap(err)
		}
		_ = stream.Close()
		closedStream = true
		return nil
	})
	eg.Go(func() error {
		defer nodeConn.Close()
		defer stream.CancelRead(genericStreamErrorCode)

		// wait for the handshake before forwarding application data from the
		// client; the client shouldn't be sending application data as 0-RTT
		// anyway, but just in case
		select {
		case <-conn.HandshakeComplete():
		case <-conn.Context().Done():
			return trace.Wrap(context.Cause(conn.Context()))
		}
		if _, err := ignoreCodeZero2(io.Copy(nodeConn, stream)); err != nil && !utils.IsOKNetworkError(err) {
			return trace.Wrap(err)
		}
		stream.CancelRead(noStreamErrorCode)
		return nil
	})
	err = eg.Wait()
	log.DebugContext(conn.Context(), "done forwarding data", "error", err)
}

// dialResponseOK is the length-prefixed encoding of a DialResponse message that
// signifies a successful dial (see TestDialResponseOKEncoding).
const dialResponseOK = "\x00\x00\x00\x00"

// Close stops listening for incoming connections and ungracefully terminates
// all the existing ones.
func (s *Server) Close() error {
	s.runCancel()
	s.Shutdown(context.Background())
	return nil
}

// Shutdown stops listening for incoming connections and waits until the
// existing ones are closed or until the context expires. If the context
// expires, running connections are ungracefully terminated.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	defer s.runCancel()
	defer context.AfterFunc(ctx, s.runCancel)()
	s.serveCancel()
	s.wg.Wait()
	return nil
}

// replayStore will keep track of nonces for at least as much time as
// [quicNoncePersistence]. Nonces are added to a "current" set until the oldest
// item in it is older than the period, at which point the set is moved into a
// "previous" slot. After the next "current" set ages out, the previous set is
// cleared. This saves us from having to keep track of individual expiration
// times.
type replayStore struct {
	mu sync.Mutex

	currentTime time.Time
	currentSet  map[uint64]struct{}
	previousSet map[uint64]struct{}
}

func (r *replayStore) add(nonce uint64, now time.Time) (added bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if now.Sub(r.currentTime) > noncePersistence {
		r.currentTime = now
		r.previousSet, r.currentSet = r.currentSet, r.previousSet
		clear(r.currentSet)
	}
	if _, ok := r.previousSet[nonce]; ok {
		return false
	}
	if _, ok := r.currentSet[nonce]; ok {
		return false
	}
	if r.currentSet == nil {
		r.currentSet = make(map[uint64]struct{})
	}
	r.currentSet[nonce] = struct{}{}
	return true
}
