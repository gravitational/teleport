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
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	quicpeeringv1a "github.com/gravitational/teleport/gen/proto/go/teleport/quicpeering/v1alpha"
	"github.com/gravitational/teleport/lib/proxy/peer/internal"
	"github.com/gravitational/teleport/lib/utils"
)

// ClientConnConfig contains the parameters to create a [ClientConn].
type ClientConnConfig struct {
	// PeerAddr is the public address of the proxy peering listener of the peer
	// proxy that the [ClientConn] will be connecting to.
	PeerAddr string

	// LocalID is the host ID of the current instance.
	LocalID string
	// ClusterName is the name of the Teleport cluster.
	ClusterName string

	// PeerID is the expected host ID of the peer proxy. Enforced at connection
	// time.
	PeerID string
	// PeerHost is the hostname of the peer proxy. Only used for metrics.
	PeerHost string
	// PeerGroup is the peer group ID advertised by the peer proxy, if any. Only
	// used for metrics.
	PeerGroup string

	Log *slog.Logger

	// GetTLSCertificate returns the client TLS certificate to use when
	// connecting to the peer proxy.
	GetTLSCertificate utils.GetCertificateFunc
	// GetTLSRoots returns a certificate pool used to validate TLS connections
	// to the peer proxy.
	GetTLSRoots utils.GetRootsFunc

	// Transport will be used to dial the peer proxy.
	Transport *quic.Transport
}

// NewClientConn opens an [internal.ClientConn] to a peer proxy using a QUIC
// transport.
func NewClientConn(config ClientConnConfig) (*ClientConn, error) {
	log := config.Log.With(
		"peer_id", config.PeerID,
		"peer_addr", config.PeerAddr,
	)
	log.DebugContext(context.Background(), "setting up QUIC client conn")

	udpAddr, err := net.ResolveUDPAddr("udp", config.PeerAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	getCertificate := config.GetTLSCertificate
	// crypto/tls doesn't allow us to configure TLS 1.3 ciphersuites, and the
	// only other effect of [utils.TLSConfig] is to require at least TLS 1.2,
	// but QUIC requires at least TLS 1.3 anyway
	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := getCertificate()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return cert, nil
		},
		VerifyPeerCertificate: internal.VerifyPeerCertificateIsSpecificProxy(config.PeerID + "." + config.ClusterName),
		NextProtos:            []string{nextProto},
		ServerName:            apiutils.EncodeClusterName(config.ClusterName),
		ClientSessionCache:    tls.NewLRUClientSessionCache(0),
		MinVersion:            tls.VersionTLS13,
	}

	quicConfig := &quic.Config{
		MaxStreamReceiveWindow:     maxReceiveWindow,
		MaxConnectionReceiveWindow: maxReceiveWindow,

		MaxIncomingStreams:    -1,
		MaxIncomingUniStreams: -1,

		MaxIdleTimeout:  maxIdleTimeout,
		KeepAlivePeriod: keepAlivePeriod,

		TokenStore: quic.NewLRUTokenStore(10, 10),
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	cc := &ClientConn{
		id:   config.PeerID,
		addr: udpAddr,

		log: log,

		transport: config.Transport,

		tlsConfig:  tlsConfig,
		getRootCAs: config.GetTLSRoots,
		quicConfig: quicConfig,

		runCtx:    runCtx,
		runCancel: runCancel,
	}

	pings, pingFailures := internal.ClientPingsMetrics(internal.ClientPingsMetricsParams{
		LocalID:   config.LocalID,
		PeerID:    config.PeerID,
		PeerHost:  config.PeerHost,
		PeerGroup: config.PeerGroup,
	})
	go internal.RunClientPing(runCtx, cc, pings, pingFailures)

	return cc, nil
}

// ClientConn is an [internal.ClientConn] that uses QUIC as its transport.
type ClientConn struct {
	id   string
	addr *net.UDPAddr

	log *slog.Logger

	transport *quic.Transport

	tlsConfig  *tls.Config
	getRootCAs utils.GetRootsFunc
	quicConfig *quic.Config

	// runCtx is canceled to signal that all connections should be closed
	// (ungracefully).
	runCtx context.Context
	// runCancel cancels runCtx.
	runCancel context.CancelFunc

	// mu guards the closed flag, waiting on the wg WaitGroup, and adding to the
	// WaitGroup when it's potentially zero.
	mu sync.Mutex
	// closed is set at the beginning of a shutdown to signify that this client
	// conn should not be opening any new connections.
	closed bool
	// wg counts the active connections. Must only be waited after setting the
	// closed flag. Must only be potentially increased from 0 while the closed
	// flag is not set.
	wg sync.WaitGroup
}

var _ internal.ClientConn = (*ClientConn)(nil)

// PeerID implements [internal.ClientConn].
func (c *ClientConn) PeerID() string {
	return c.id
}

// PeerAddr implements [internal.ClientConn].
func (c *ClientConn) PeerAddr() string {
	return c.addr.String()
}

// Close implements [internal.ClientConn].
func (c *ClientConn) Close() error {
	c.mu.Lock()
	// it's fine to double Close (or mix Close and Shutdown concurrently), the
	// logic is idempotent
	c.closed = true
	c.mu.Unlock()
	c.runCancel()
	c.wg.Wait()
	return nil
}

// Shutdown implements [internal.ClientConn].
func (c *ClientConn) Shutdown(ctx context.Context) {
	c.mu.Lock()
	// it's fine to double Shutdown (or mix Close and Shutdown concurrently),
	// the logic is idempotent
	c.closed = true
	c.mu.Unlock()
	defer c.runCancel()
	defer context.AfterFunc(ctx, c.runCancel)()
	c.wg.Wait()
}

// Dial implements [internal.ClientConn].
func (c *ClientConn) Dial(nodeID string, src net.Addr, dst net.Addr, tunnelType types.TunnelType, permit []byte) (_ net.Conn, err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, trace.ConnectionProblem(net.ErrClosed, "client is shut down")
	}
	c.wg.Add(1)
	defer c.wg.Done()
	c.mu.Unlock()

	var nonce uint64
	if err := binary.Read(rand.Reader, binary.NativeEndian, &nonce); err != nil {
		return nil, trace.Wrap(err)
	}

	log := c.log.With("conn_nonce", nonce)

	req := &quicpeeringv1a.DialRequest{
		TargetHostId:   nodeID,
		ConnectionType: string(tunnelType),
		Source: &quicpeeringv1a.Addr{
			Network: src.Network(),
			Addr:    src.String(),
		},
		Destination: &quicpeeringv1a.Addr{
			Network: dst.Network(),
			Addr:    dst.String(),
		},
		Timestamp: timestamppb.Now(),
		Nonce:     nonce,
		Permit:    permit,
	}
	sizedReqBuf, err := marshalSized(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sizedReqBuf)-4 > maxMessageSize {
		log.WarnContext(context.Background(), "refusing to send oversized dial request (this is a bug)")
		return nil, trace.LimitExceeded("oversized dial request")
	}

	rootCAs, err := c.getRootCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	utils.RefreshTLSConfigTickets(c.tlsConfig)
	tlsConfig := c.tlsConfig.Clone()
	tlsConfig.RootCAs = rootCAs

	deadline := time.Now().Add(dialTimeout)
	dialCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	log.DebugContext(dialCtx, "dialing peer proxy")
	earlyConn, err := c.transport.DialEarly(dialCtx, c.addr, tlsConfig, c.quicConfig)
	if err != nil {
		if errors.Is(err, internal.WrongProxyError{}) {
			internal.LogDuplicatePeer(dialCtx, log, slog.LevelError)
		}
		return nil, trace.Wrap(err)
	}

	var conn quic.Connection = earlyConn
	defer func() {
		if err == nil {
			return
		}
		conn.CloseWithError(genericApplicationErrorCode, "")
	}()

	log.DebugContext(conn.Context(),
		"opened connection",
		"gso", conn.ConnectionState().GSO,
	)

	respBuf, stream, err := sendUnary(deadline, sizedReqBuf, conn)
	if err != nil {
		if !errors.Is(err, quic.Err0RTTRejected) {
			return nil, trace.Wrap(err)
		}

		log.InfoContext(dialCtx, "0-RTT attempt rejected, retrying with a full handshake")
		nextConn, err := earlyConn.NextConnection(dialCtx)
		if err != nil {
			if errors.Is(err, internal.WrongProxyError{}) {
				// if we are hitting a QUIC-aware load balancer(?) it's possible
				// to reach an unexpected peer proxy after a failed 0-RTT
				// (failed because we got sent to the "wrong" peer)
				internal.LogDuplicatePeer(dialCtx, log, slog.LevelError)
			}
			return nil, trace.Wrap(err)
		}
		conn, earlyConn = nextConn, nil
		// NextConnection can return a valid, closed connection and no error; if
		// the connection is not closed then we completed a full handshake
		if conn.Context().Err() != nil {
			return nil, trace.Wrap(context.Cause(conn.Context()))
		}
		log.DebugContext(conn.Context(), "full handshake completed after 0-RTT rejection")
		respBuf, stream, err = sendUnary(deadline, sizedReqBuf, conn)
		if err != nil {
			log.DebugContext(conn.Context(),
				"failed to exchange dial request after 0-RTT rejection and handshake",
				"error", err,
			)
			return nil, trace.Wrap(err)
		}
	}

	log.DebugContext(conn.Context(),
		"exchanged dial request and response",
		"used_0rtt", conn.ConnectionState().Used0RTT,
	)

	resp := new(quicpeeringv1a.DialResponse)
	if err := proto.Unmarshal(respBuf, resp); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := trail.FromGRPC(status.FromProto(resp.GetStatus()).Err()); err != nil {
		return nil, trace.Wrap(err)
	}

	if earlyConn != nil {
		// avoid sending connection data as part of the early data; I'm not sure
		// if the client is guaranteed to be protected against replays
		// immediately after successfully receiving server data, but if it is
		// then it means that the handshake is already complete and this will
		// not block anyway
		select {
		case <-earlyConn.HandshakeComplete():
		case <-earlyConn.Context().Done():
			err := context.Cause(earlyConn.Context())
			log.DebugContext(conn.Context(),
				"failed to complete handshake after exchanging 0-RTT dial request",
				"error", err,
			)
			return nil, trace.Wrap(err)
		}
	}

	sc := &streamConn{
		st:   stream,
		conn: conn,

		src: src,
		dst: dst,
	}

	detach := context.AfterFunc(c.runCtx, func() { _ = sc.Close() })
	// conn.Context() is canceled when the connection is closed; wg is currently
	// at least at 1 because we add one count for the duration of this function,
	// so we're always allowed to add another one here
	c.wg.Add(1)
	context.AfterFunc(conn.Context(), func() {
		err := ignoreCodeZero(context.Cause(conn.Context()))
		log.DebugContext(conn.Context(), "connection closed", "error", err)
		c.wg.Done()
		// remove the connection from the runCtx cancellation tree
		detach()
	})

	return sc, nil
}

// Ping implements [internal.ClientConn].
func (c *ClientConn) Ping(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return trace.ConnectionProblem(net.ErrClosed, "client is shut down")
	}
	c.wg.Add(1)
	defer c.wg.Done()
	c.mu.Unlock()

	var nonce uint64
	if err := binary.Read(rand.Reader, binary.NativeEndian, &nonce); err != nil {
		return trace.Wrap(err)
	}

	log := c.log.With("ping_nonce", nonce)

	req := &quicpeeringv1a.DialRequest{
		Timestamp: timestamppb.Now(),
		Nonce:     nonce,
		Ping:      true,
	}
	sizedReqBuf, err := marshalSized(req)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(sizedReqBuf)-4 > maxMessageSize {
		log.WarnContext(context.Background(), "refusing to send oversized ping request (this is a bug)")
		return trace.LimitExceeded("oversized ping request")
	}

	rootCAs, err := c.getRootCAs()
	if err != nil {
		return trace.Wrap(err)
	}
	utils.RefreshTLSConfigTickets(c.tlsConfig)
	tlsConfig := c.tlsConfig.Clone()
	tlsConfig.RootCAs = rootCAs

	deadline := time.Now().Add(dialTimeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	log.DebugContext(ctx, "dialing peer proxy for pinging")
	conn, err := c.transport.Dial(ctx, c.addr, tlsConfig, c.quicConfig)
	if err != nil {
		if errors.Is(err, internal.WrongProxyError{}) {
			internal.LogDuplicatePeer(ctx, log, slog.LevelError)
		}
		return trace.Wrap(err)
	}
	defer conn.CloseWithError(noApplicationErrorCode, "")
	defer context.AfterFunc(ctx, func() { conn.CloseWithError(noApplicationErrorCode, "") })()

	log.DebugContext(conn.Context(),
		"opened connection",
		"gso", conn.ConnectionState().GSO,
	)

	respBuf, stream, err := sendUnary(deadline, sizedReqBuf, conn)
	if err != nil {
		return trace.Wrap(err)
	}
	stream.CancelRead(noStreamErrorCode)
	_ = stream.Close()

	log.DebugContext(conn.Context(),
		"exchanged ping request and response",
	)

	resp := new(quicpeeringv1a.DialResponse)
	if err := proto.Unmarshal(respBuf, resp); err != nil {
		return trace.Wrap(err)
	}
	if err := trail.FromGRPC(status.FromProto(resp.GetStatus()).Err()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// sendUnary opens a stream, sends the given request buffer and then reads a
// response buffer. Request and response are length-prefixed by a 32 bit little
// endian integer, but the buffer size is also limited by [quicMaxMessageSize].
// The given request buffer should already be length-prefixed.
func sendUnary(deadline time.Time, sizedReqBuf []byte, conn quic.Connection) (_ []byte, _ quic.Stream, err error) {
	stream, err := conn.OpenStream()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer func() {
		if err == nil {
			return
		}
		stream.CancelRead(genericStreamErrorCode)
		stream.CancelWrite(genericStreamErrorCode)
	}()

	stream.SetDeadline(deadline)
	if _, err := stream.Write(sizedReqBuf); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var respSize uint32
	if err := binary.Read(stream, binary.LittleEndian, &respSize); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if respSize > maxMessageSize {
		return nil, nil, trace.LimitExceeded("oversized response message")
	}
	respBuf := make([]byte, respSize)
	if _, err := io.ReadFull(stream, respBuf); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	stream.SetDeadline(time.Time{})
	return respBuf, stream, nil
}

// streamConn is a [net.Conn] using a single [quic.Stream] in a dedicated
// [quic.Connection].
type streamConn struct {
	st   quic.Stream
	conn quic.Connection

	src net.Addr
	dst net.Addr
}

var _ net.Conn = (*streamConn)(nil)

// Read implements [net.Conn].
func (c *streamConn) Read(b []byte) (n int, err error) {
	return c.st.Read(b)
}

// Write implements [net.Conn].
func (c *streamConn) Write(b []byte) (n int, err error) {
	return c.st.Write(b)
}

// Close implements [net.Conn].
func (c *streamConn) Close() error {
	// closing the connection will also close the stream
	return trace.Wrap(c.conn.CloseWithError(noApplicationErrorCode, ""))
}

// SetDeadline implements [net.Conn].
func (c *streamConn) SetDeadline(t time.Time) error {
	return c.st.SetDeadline(t)
}

// SetReadDeadline implements [net.Conn].
func (c *streamConn) SetReadDeadline(t time.Time) error {
	return c.st.SetReadDeadline(t)
}

// SetWriteDeadline implements [net.Conn].
func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return c.st.SetWriteDeadline(t)
}

// LocalAddr implements [net.Conn].
func (c *streamConn) LocalAddr() net.Addr {
	return c.src
}

// RemoteAddr implements [net.Conn].
func (c *streamConn) RemoteAddr() net.Addr {
	return c.dst
}
