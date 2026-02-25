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
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	reversetunnelv1 "github.com/gravitational/teleport/gen/proto/go/teleport/reversetunnel/v1"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
)

// newAgent dials the proxy at cfg.addr, performs the AgentHello/ProxyHello
// handshake, calls cfg.lease.Claim with the proxy's reported ID, and returns a
// live Agent. It blocks until the handshake is complete or an error occurs. On
// error the lease is released.
//
// After a successful return the caller must eventually call Agent.Stop() or
// wait for Agent.Done() to close.
func newAgent(ctx context.Context, cfg agentConfig, log *slog.Logger, getCertificate func() (*tls.Certificate, error), getPool func() (*x509.CertPool, error), ciphersuites []uint16) (Agent, error) {
	helloCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cert, err := getCertificate()
	if err != nil {
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}
	pool, err := getPool()
	if err != nil {
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	host, _, err := net.SplitHostPort(cfg.addr)
	if err != nil {
		host = cfg.addr
	}

	tlsCfg := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		// The proxy address is typically a direct IP; we skip standard DNS-name
		// SAN verification and instead check that the server cert carries the
		// Proxy role, matching the pattern used in lib/relaytunnel.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol != yamuxTunnelALPN {
				return trace.NotImplemented("server did not negotiate %q ALPN", yamuxTunnelALPN)
			}
			opts := x509.VerifyOptions{
				Roots:     pool,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			if len(cs.PeerCertificates) > 1 {
				opts.Intermediates = x509.NewCertPool()
				for _, c := range cs.PeerCertificates[1:] {
					opts.Intermediates.AddCert(c)
				}
			}
			if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
				return trace.Wrap(err)
			}
			return nil
		},
		NextProtos: []string{yamuxTunnelALPN},
		ServerName: host,

		CipherSuites: ciphersuites,
		MinVersion:   tls.VersionTLS12,
	}

	nc, err := new(net.Dialer).DialContext(helloCtx, "tcp", cfg.addr)
	if err != nil {
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	tc := tls.Client(nc, tlsCfg)
	if err := tc.HandshakeContext(helloCtx); err != nil {
		_ = tc.Close()
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	session, err := yamux.Client(tc, yamuxConfig(log))
	if err != nil {
		_ = tc.Close()
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	controlStream, err := session.OpenStream()
	if err != nil {
		_ = session.Close()
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	helloDeadline, _ := helloCtx.Deadline()
	controlStream.SetDeadline(helloDeadline)

	// Build the service list for AgentHello.
	services := make([]string, 0, len(cfg.services))
	for _, svc := range cfg.services {
		services = append(services, string(svc))
	}

	if err := writeProto(controlStream, &reversetunnelv1.AgentHello{
		HostId:      cfg.hostID,
		Services:    services,
		ClusterName: cfg.clusterName,
		Version:     cfg.version,
		Scope:       cfg.scope,
	}); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	proxyHello := new(reversetunnelv1.ProxyHello)
	if err := readProto(controlStream, proxyHello); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	if err := trail.FromGRPC(grpcstatus.FromProto(proxyHello.GetStatus()).Err()); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		cfg.lease.Release()
		return nil, trace.Wrap(err)
	}

	proxyID := proxyHello.GetProxyId()

	// Claim the tracker lease. If the proxy is already claimed we bail out
	// so the pool can connect elsewhere.
	if !cfg.lease.Claim(proxyID) {
		_ = controlStream.Close()
		_ = session.Close()
		cfg.lease.Release()
		return nil, trace.AlreadyExists("proxy %q already claimed", proxyID)
	}

	// Feed the initial gossip payload into the tracker.
	cfg.tracker.TrackExpected(protoEntriesToTrack(proxyHello.GetProxies())...)

	controlStream.SetDeadline(time.Time{})

	done := make(chan struct{})
	a := &agent{
		log:     log,
		cfg:     cfg,
		session: session,
		proxyID: proxyID,
		done:    done,
	}

	go a.run(controlStream)

	return a, nil
}

// agent implements the Agent interface for a single live proxy connection.
type agent struct {
	log     *slog.Logger
	cfg     agentConfig
	session *yamux.Session
	proxyID string
	done    chan struct{}

	terminating atomic.Bool
}

// GetProxyID implements [Agent].
func (a *agent) GetProxyID() string { return a.proxyID }

// Done implements [Agent].
func (a *agent) Done() <-chan struct{} { return a.done }

// Stop implements [Agent].
func (a *agent) Stop() error {
	return trace.Wrap(a.session.Close())
}

// IsTerminating implements [Agent].
func (a *agent) IsTerminating() bool { return a.terminating.Load() }

// run drives the control stream loop (reading ProxyControl messages, sending
// periodic AgentControl heartbeats) and the dial-accept loop (accepting proxy-
// initiated yamux streams and routing them to service handlers). Closes done
// when it returns.
func (a *agent) run(controlStream *yamux.Stream) {
	defer close(a.done)
	defer a.cfg.lease.Release()

	var wg sync.WaitGroup

	// Control stream reader: process ProxyControl messages from the proxy.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer a.session.Close()
		defer controlStream.Close()
		a.readControlStream(controlStream)
	}()

	// Heartbeat writer: send AgentControl heartbeats on the control stream.
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.writeHeartbeats(controlStream)
	}()

	// Dial acceptor: accept proxy-initiated streams and dispatch to handlers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer a.session.Close()
		a.acceptDialStreams()
	}()

	wg.Wait()
}

// readControlStream reads ProxyControl messages until the stream or session
// closes. It updates the terminating flag and forwards gossip updates to the
// tracker.
func (a *agent) readControlStream(controlStream *yamux.Stream) {
	for {
		msg := new(reversetunnelv1.ProxyControl)
		if err := readProto(controlStream, msg); err != nil {
			if !errors.Is(err, io.EOF) {
				a.log.WarnContext(context.Background(), "Error reading ProxyControl", "error", err)
			}
			return
		}
		if msg.GetTerminating() {
			a.terminating.Store(true)
		}
		if len(msg.GetProxies()) > 0 {
			a.cfg.tracker.TrackExpected(protoEntriesToTrack(msg.GetProxies())...)
		}
	}
}

// writeHeartbeats sends AgentControl messages on the control stream at a fixed
// interval until the session closes.
func (a *agent) writeHeartbeats(controlStream *yamux.Stream) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.session.CloseChan():
			return
		case <-ticker.C:
			hb := &reversetunnelv1.AgentControl{
				HeartbeatUnixMs: time.Now().UnixMilli(),
			}
			if err := writeProto(controlStream, hb); err != nil {
				a.log.WarnContext(context.Background(), "Error writing AgentControl heartbeat", "error", err)
				return
			}
		}
	}
}

// heartbeatInterval is how often the agent sends AgentControl heartbeats.
const heartbeatInterval = 15 * time.Second

// acceptDialStreams accepts proxy-initiated yamux streams and dispatches each
// one to the appropriate local service handler in its own goroutine.
func (a *agent) acceptDialStreams() {
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		stream, err := a.session.AcceptStream()
		if err != nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.log.DebugContext(context.Background(), "Accepted dial stream", "stream_id", stream.StreamID())
			defer a.log.DebugContext(context.Background(), "Done with dial stream", "stream_id", stream.StreamID())
			a.handleDialStream(stream)
		}()
	}
}

// handleDialStream reads a DialRequest, selects the matching handler, sends a
// DialResponse, and then passes the stream to the handler.
func (a *agent) handleDialStream(stream *yamux.Stream) {
	defer stream.Close()

	dialReq := new(reversetunnelv1.DialRequest)
	if err := readProto(stream, dialReq); err != nil {
		a.log.WarnContext(context.Background(), "Error reading DialRequest", "error", err)
		return
	}

	serviceType := types.TunnelType(dialReq.GetServiceType())
	handler, ok := a.cfg.handlers[serviceType]
	if !ok {
		err := trace.NotFound("no handler registered for service type %q", serviceType)
		_ = writeProto(stream, &reversetunnelv1.DialResponse{
			Status: errToStatusProto(err),
		})
		return
	}

	if err := writeProto(stream, &reversetunnelv1.DialResponse{}); err != nil {
		a.log.WarnContext(context.Background(), "Error writing DialResponse", "error", err)
		return
	}

	src := tcpAddrFromProto(dialReq.GetSource())
	dst := addrFromProto(dialReq.GetDestination())

	nc := &yamuxStreamConn{
		Stream:     stream,
		localAddr:  dst,
		remoteAddr: src,
	}
	handler.HandleConnection(nc)
}

// protoEntriesToTrack converts a slice of protobuf ProxyEntry messages into
// the track.Proxy structs consumed by the tracker.
func protoEntriesToTrack(entries []*reversetunnelv1.ProxyEntry) []track.Proxy {
	out := make([]track.Proxy, 0, len(entries))
	for _, e := range entries {
		out = append(out, track.Proxy{
			Name:       e.GetName(),
			Group:      e.GetGroupId(),
			Generation: e.GetGeneration(),
		})
	}
	return out
}
