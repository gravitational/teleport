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

// Package sshutils contains the implementations of the base SSH
// server used throughout Teleport.
package sshutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/utils"
)

var proxyConnectionLimitHitCount = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: teleport.MetricProxyConnectionLimitHit,
		Help: "Number of times the proxy connection limit was exceeded",
	},
)

// Server is a generic implementation of an SSH server. All Teleport
// services (auth, proxy, ssh) use this as a base to accept SSH connections.
type Server struct {
	sync.RWMutex

	logger *slog.Logger
	// component is a name of the facility which uses this server,
	// used for logging/debugging. typically it's "proxy" or "auth api", etc
	component string

	// addr is the address this server binds to and listens on
	addr utils.NetAddr

	// listener is usually the listening TCP/IP socket
	listener net.Listener

	newChanHandler NewChanHandler
	reqHandler     RequestHandler
	newConnHandler NewConnHandler
	getHostSigners GetHostSignersFunc

	cfg     ssh.ServerConfig
	limiter *limiter.Limiter

	closeContext context.Context
	closeFunc    context.CancelFunc

	// userConns tracks amount of current active connections with user certificates.
	userConns int32
	// shutdownPollPeriod sets polling period for shutdown
	shutdownPollPeriod time.Duration

	// insecureSkipHostValidation does not validate the host signers to make sure
	// they are a valid certificate. Used in tests.
	insecureSkipHostValidation bool

	// fips means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	fips bool

	// tracerProvider is used to create tracers capable
	// of starting spans.
	tracerProvider oteltrace.TracerProvider

	// clock is used to control time.
	clock clockwork.Clock

	clusterName string

	// ingressReporter reports new and active connections.
	ingressReporter *ingress.Reporter
	// ingressService the service name passed to the ingress reporter.
	ingressService string
}

const (
	// SSHVersionPrefix is the prefix of "server version" string which begins
	// every SSH handshake. It MUST start with "SSH-2.0" according to
	// https://tools.ietf.org/html/rfc4253#page-4
	SSHVersionPrefix = "SSH-2.0-Teleport"

	// MaxVersionStringBytes is the maximum number of bytes allowed for a
	// SSH version string
	// https://tools.ietf.org/html/rfc4253
	MaxVersionStringBytes = 255
)

// ServerOption is a functional argument for server
type ServerOption func(cfg *Server) error

// SetIngressReporter sets the reporter for reporting new and active connections.
func SetIngressReporter(service string, r *ingress.Reporter) ServerOption {
	return func(s *Server) error {
		s.ingressReporter = r
		s.ingressService = service
		return nil
	}
}

// SetLogger sets the logger for the server
func SetLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) error {
		s.logger = logger.With(teleport.ComponentKey, teleport.Component("ssh", s.component))
		return nil
	}
}

func SetLimiter(limiter *limiter.Limiter) ServerOption {
	return func(s *Server) error {
		s.limiter = limiter
		return nil
	}
}

// SetShutdownPollPeriod sets a polling period for graceful shutdowns of SSH servers
func SetShutdownPollPeriod(period time.Duration) ServerOption {
	return func(s *Server) error {
		s.shutdownPollPeriod = period
		return nil
	}
}

// SetInsecureSkipHostValidation does not validate the host signers to make sure
// they are a valid certificate. Used in tests.
func SetInsecureSkipHostValidation() ServerOption {
	return func(s *Server) error {
		s.insecureSkipHostValidation = true
		return nil
	}
}

// SetTracerProvider sets the tracer provider for the server.
func SetTracerProvider(provider oteltrace.TracerProvider) ServerOption {
	return func(s *Server) error {
		s.tracerProvider = provider
		return nil
	}
}

// SetClock sets the server's clock.
func SetClock(clock clockwork.Clock) ServerOption {
	return func(s *Server) error {
		s.clock = clock
		return nil
	}
}

func SetClusterName(clusterName string) ServerOption {
	return func(s *Server) error {
		s.clusterName = clusterName
		return nil
	}
}

func NewServer(
	component string,
	a utils.NetAddr,
	h NewChanHandler,
	getHostSigners GetHostSignersFunc,
	ah AuthMethods,
	opts ...ServerOption,
) (*Server, error) {
	err := metrics.RegisterPrometheusCollectors(proxyConnectionLimitHitCount)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(context.TODO())
	s := &Server{
		logger:         slog.With(teleport.ComponentKey, teleport.Component("ssh", component)),
		addr:           a,
		newChanHandler: h,
		getHostSigners: getHostSigners,
		component:      component,
		closeContext:   closeContext,
		closeFunc:      cancel,
	}
	s.limiter, err = limiter.NewLimiter(limiter.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, err
		}
	}
	if s.shutdownPollPeriod == 0 {
		s.shutdownPollPeriod = defaults.ShutdownPollPeriod
	}

	if s.tracerProvider == nil {
		s.tracerProvider = tracing.DefaultProvider()
	}

	err = s.checkArguments(a, h, getHostSigners, ah)
	if err != nil {
		return nil, err
	}

	s.cfg.PublicKeyCallback = ah.PublicKey
	s.cfg.PasswordCallback = ah.Password
	s.cfg.NoClientAuth = ah.NoClient

	if s.fips {
		s.cfg.PublicKeyAuthAlgorithms = defaults.FIPSPubKeyAuthAlgorithms
	}

	// Teleport servers need to identify as such to allow passing of the client
	// IP from the client to the proxy to the destination node.
	s.cfg.ServerVersion = SSHVersionPrefix

	return s, nil
}

func SetSSHConfig(cfg ssh.ServerConfig) ServerOption {
	return func(s *Server) error {
		s.cfg = cfg
		return nil
	}
}

func SetRequestHandler(req RequestHandler) ServerOption {
	return func(s *Server) error {
		s.reqHandler = req
		return nil
	}
}

func SetNewConnHandler(handler NewConnHandler) ServerOption {
	return func(s *Server) error {
		s.newConnHandler = handler
		return nil
	}
}

func SetCiphers(ciphers []string) ServerOption {
	return func(s *Server) error {
		s.logger.DebugContext(context.Background(), "Supported ciphers updated", "ciphers", ciphers)
		if ciphers != nil {
			s.cfg.Ciphers = ciphers
		}
		return nil
	}
}

func SetKEXAlgorithms(kexAlgorithms []string) ServerOption {
	return func(s *Server) error {
		s.logger.DebugContext(context.Background(), "Supported KEX algorithms updated", "kex_algorithms", kexAlgorithms)
		if kexAlgorithms != nil {
			s.cfg.KeyExchanges = kexAlgorithms
		}
		return nil
	}
}

func SetMACAlgorithms(macAlgorithms []string) ServerOption {
	return func(s *Server) error {
		s.logger.DebugContext(context.Background(), "Supported MAC algorithms updated", "mac_algorithms", macAlgorithms)
		if macAlgorithms != nil {
			s.cfg.MACs = macAlgorithms
		}
		return nil
	}
}

func SetFIPS(fips bool) ServerOption {
	return func(s *Server) error {
		s.fips = fips
		return nil
	}
}

func (s *Server) Addr() string {
	s.RLock()
	defer s.RUnlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Serve(listener net.Listener) error {
	if err := s.SetListener(listener); err != nil {
		return trace.Wrap(err)
	}
	s.acceptConnections()
	return nil
}

func (s *Server) Start() error {
	if s.listener == nil {
		listener, err := net.Listen(s.addr.AddrNetwork, s.addr.Addr)
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		listener, err = s.limiter.WrapListener(listener)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := s.SetListener(listener); err != nil {
			return trace.Wrap(err)
		}
	}
	s.logger.DebugContext(s.closeContext, "Starting server", "addr", s.listener.Addr().String())
	go s.acceptConnections()
	return nil
}

func (s *Server) SetListener(l net.Listener) error {
	s.Lock()
	defer s.Unlock()
	if s.listener != nil {
		return trace.BadParameter("listener is already set to %v", s.listener.Addr())
	}
	s.listener = l
	return nil
}

// Wait waits until server stops serving new connections
// on the listener socket
func (s *Server) Wait(ctx context.Context) {
	select {
	case <-s.closeContext.Done():
	case <-ctx.Done():
	}
}

// Shutdown initiates graceful shutdown - waiting until all active
// connections will get closed
func (s *Server) Shutdown(ctx context.Context) error {
	// close listener to stop receiving new connections
	err := s.Close()
	s.Wait(ctx)
	activeConnections := s.trackUserConnections(0)
	if activeConnections == 0 {
		return err
	}
	minReportInterval := 10 * s.shutdownPollPeriod
	maxReportInterval := 600 * s.shutdownPollPeriod
	s.logger.InfoContext(ctx, "Shutdown: waiting for active connections to finish", "active_connections", activeConnections)
	reportedConnections := activeConnections
	lastReport := time.Now()
	reportInterval := minReportInterval
	ticker := time.NewTicker(s.shutdownPollPeriod)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			activeConnections = s.trackUserConnections(0)
			if activeConnections == 0 {
				return err
			}
			if activeConnections != reportedConnections || now.Sub(lastReport) > reportInterval {
				s.logger.InfoContext(ctx, "Shutdown: waiting for active connections to finish", "active_connections", activeConnections)
				lastReport = now
				if activeConnections == reportedConnections {
					reportInterval = min(reportInterval*2, maxReportInterval)
				} else {
					reportInterval = minReportInterval
					reportedConnections = activeConnections
				}
			}
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "Context canceled wait, returning")
			return trace.ConnectionProblem(err, "context canceled")
		}
	}
}

// Close closes listening socket and stops accepting connections
func (s *Server) Close() error {
	s.Lock()
	defer s.Unlock()
	defer s.closeFunc()

	if s.listener != nil {
		err := s.listener.Close()
		if utils.IsUseOfClosedNetworkError(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	return nil
}

func (s *Server) acceptConnections() {
	defer s.closeFunc()
	logger := s.logger.With("listen_addr", s.Addr())
	logger.DebugContext(s.closeContext, "Listening for connections")
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if trace.IsLimitExceeded(err) {
				proxyConnectionLimitHitCount.Inc()
				logger.ErrorContext(s.closeContext, "connection limit exceeded", "error", err)
				continue
			}

			if utils.IsUseOfClosedNetworkError(err) {
				logger.DebugContext(s.closeContext, "Server has closed")
				return
			}
			select {
			case <-s.closeContext.Done():
				logger.DebugContext(s.closeContext, "Server has closed")
				return
			case <-time.After(5 * time.Second):
				logger.DebugContext(s.closeContext, "Applying backoff in response to network error", "error", err)
			}
		} else {
			go s.HandleConnection(conn)
		}
	}
}

func (s *Server) trackUserConnections(delta int32) int32 {
	return atomic.AddInt32(&s.userConns, delta)
}

// TrackUserConnection tracks a user connection that should prevent
// the server from being terminated if active. The returned function
// should be called when the connection is terminated.
func (s *Server) TrackUserConnection() (release func()) {
	s.trackUserConnections(1)

	return sync.OnceFunc(func() {
		s.trackUserConnections(-1)
	})
}

// ActiveConnections returns the number of connections that are
// being served.
func (s *Server) ActiveConnections() int32 {
	return atomic.LoadInt32(&s.userConns)
}

// HandleConnection is called every time an SSH server accepts a new
// connection from a client.
//
// this is the foundation of all SSH connections in Teleport (between clients
// and proxies, proxies and servers, servers and auth, etc), except for forwarding
// SSH proxy that used when "recording on proxy" is enabled.
func (s *Server) HandleConnection(conn net.Conn) {
	if s.ingressReporter != nil {
		s.ingressReporter.ConnectionAccepted(s.ingressService, conn)
		defer s.ingressReporter.ConnectionClosed(s.ingressService, conn)
	}

	hostSigners := s.getHostSigners()
	if err := s.validateHostSigners(hostSigners); err != nil {
		s.logger.ErrorContext(s.closeContext, "Error during server setup for a new SSH connection (this is a bug)",
			"error", err,
			"remote_addr", conn.RemoteAddr(),
		)
		conn.Close()
		return
	}

	cfg := s.cfg
	for _, signer := range hostSigners {
		cfg.AddHostKey(signer)
	}
	if v := serverVersionOverrideFromConn(conn); v != "" && v != cfg.ServerVersion {
		cfg.ServerVersion = v
	}

	// apply idle read/write timeout to this connection.
	conn = utils.ObeyIdleTimeout(conn, defaults.DefaultIdleConnectionDuration)
	// Wrap connection with a tracker used to monitor how much data was
	// transmitted and received over the connection.
	conn = utils.NewTrackingConn(conn)

	sconn, chans, reqs, err := ssh.NewServerConn(conn, &cfg)
	if err != nil {
		// Ignore EOF as these are triggered by loadbalancer health checks
		if !errors.Is(err, io.EOF) {
			s.logger.WarnContext(s.closeContext, "Error occurred in handshake for new SSH conn",
				"error", err,
				"remote_addr", conn.RemoteAddr(),
			)
		}
		conn.Close()
		return
	}

	if s.ingressReporter != nil {
		s.ingressReporter.ConnectionAuthenticated(s.ingressService, conn)
		defer s.ingressReporter.AuthenticatedConnectionClosed(s.ingressService, conn)
	}

	certType := "unknown"
	if sconn.Permissions != nil {
		certType = sconn.Permissions.Extensions[utils.ExtIntCertType]
	}

	if certType == utils.ExtIntCertTypeUser {
		s.trackUserConnections(1)
		defer s.trackUserConnections(-1)
	}

	user := sconn.User()
	if err := s.limiter.RegisterRequest(user); err != nil {
		s.logger.ErrorContext(s.closeContext, "user connection rate limit exceeded", "user", user, "error", err)
		sconn.Close()
		conn.Close()
		return
	}
	// Connection successfully initiated
	s.logger.DebugContext(s.closeContext, "handling incoming connection",
		"remote_addr", sconn.RemoteAddr(),
		"local_addr", sconn.LocalAddr(),
		"version", string(sconn.ClientVersion()),
		"cert_type", certType,
	)

	// will be called when the connection is closed
	connClosed := func() {
		s.logger.DebugContext(s.closeContext, "Closed connection", "remote_addr", sconn.RemoteAddr())
	}

	// The keepalive ticket will ensure that SSH keepalive requests are being sent
	// to the client at an interval much shorter than idle connection kill switch
	keepAliveTick := time.NewTicker(defaults.DefaultIdleConnectionDuration / 3)
	defer keepAliveTick.Stop()
	keepAlivePayload := [8]byte{0}

	// NOTE: we deliberately don't use s.closeContext here because the server's
	// closeContext field is used to trigger starvation on cancellation by halting
	// the acceptance of new connections; it is not intended to halt in-progress
	// connection handling, and is therefore orthogonal to the role of ConnectionContext.
	ctx, ccx := NewConnectionContext(context.Background(), conn, sconn, SetConnectionContextClock(s.clock))
	defer ccx.Close()

	if s.newConnHandler != nil {
		// if newConnHandler was set, then we have additional setup work
		// to do before we can begin serving normally.  Errors returned
		// from a NewConnHandler are rejections.
		ctx, err = s.newConnHandler.HandleNewConn(ctx, ccx)
		if err != nil {
			s.logger.WarnContext(ctx, "Dropping inbound ssh connection due to error", "error", err)
			// Immediately dropping the ssh connection results in an
			// EOF error for the client.  We therefore wait briefly
			// to see if the client opens a channel or sends any global
			// requests, which will give us the opportunity to respond
			// with a human-readable error.
			waitCtx, waitCancel := context.WithTimeout(s.closeContext, time.Second)
			defer waitCancel()
			for {
				select {
				case req := <-reqs:
					if req == nil {
						connClosed()
						break
					}
					// wait for a request that wants a reply to send the error
					if !req.WantReply {
						continue
					}

					if err := req.Reply(false, []byte(err.Error())); err != nil {
						s.logger.WarnContext(ctx, "failed to reply to request", "request_type", req.Type, "error", err)
					}
				case firstChan := <-chans:
					// channel was closed, terminate the connection
					if firstChan == nil {
						break
					}

					if err := firstChan.Reject(ssh.Prohibited, err.Error()); err != nil {
						s.logger.WarnContext(ctx, "failed to reject channel", "channel_type", firstChan.ChannelType(), "error", err)
					}
				case <-waitCtx.Done():
				}

				break
			}

			if err := sconn.Close(); err != nil && !utils.IsOKNetworkError(err) {
				s.logger.WarnContext(ctx, "failed to close ssh server connection", "error", err)
			}
			if err := conn.Close(); err != nil && !utils.IsOKNetworkError(err) {
				s.logger.WarnContext(ctx, "failed to close ssh client connection", "error", err)
			}
			return
		}
	}

	for {
		select {
		// handle out of band ssh requests
		case req := <-reqs:
			if req == nil {
				connClosed()
				return
			}
			s.logger.DebugContext(ctx, "Received out-of-band request", "request_type", req.Type)

			reqCtx := tracessh.ContextFromRequest(req)
			ctx, span := s.tracerProvider.Tracer("ssh").Start(
				oteltrace.ContextWithRemoteSpanContext(ctx, oteltrace.SpanContextFromContext(reqCtx)),
				fmt.Sprintf("ssh.GlobalRequest/%s", req.Type),
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					semconv.RPCServiceKey.String("ssh.Server"),
					semconv.RPCMethodKey.String("GlobalRequest"),
					semconv.RPCSystemKey.String("ssh"),
				),
			)

			if s.reqHandler != nil {
				go func(span oteltrace.Span) {
					defer span.End()
					s.reqHandler.HandleRequest(ctx, ccx, req)
				}(span)
			} else {
				span.End()
			}
			// handle channels:
		case nch := <-chans:
			if nch == nil {
				connClosed()
				return
			}

			chanCtx, nch := tracessh.ContextFromNewChannel(nch)
			ctx, span := s.tracerProvider.Tracer("ssh").Start(
				oteltrace.ContextWithRemoteSpanContext(ctx, oteltrace.SpanContextFromContext(chanCtx)),
				fmt.Sprintf("ssh.OpenChannel/%s", nch.ChannelType()),
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					semconv.RPCServiceKey.String("ssh.Server"),
					semconv.RPCMethodKey.String("OpenChannel"),
					semconv.RPCSystemKey.String("ssh"),
				),
			)

			go func(span oteltrace.Span) {
				defer span.End()
				s.newChanHandler.HandleNewChan(ctx, ccx, nch)
			}(span)
			// send keepalive pings to the clients
		case <-keepAliveTick.C:
			const wantReply = true
			_, _, err = sconn.SendRequest(teleport.KeepAliveReqType, wantReply, keepAlivePayload[:])
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed sending keepalive request", "error", err)
			}
		case <-ctx.Done():
			s.logger.DebugContext(ctx, "Connection context canceled", "remote_addr", conn.RemoteAddr(), "local_addr", conn.LocalAddr())
			return
		}
	}
}

func (s *Server) HandleStapledConnection(conn net.Conn, permit []byte) {
	// TODO: unmarshal and use the permit
	s.logger.ErrorContext(context.TODO(), "=== GOT STAPLED CONNECTION, PROCEEDING NORMALLY ===", "permit", permit)
	s.HandleConnection(conn)
}

type RequestHandler interface {
	HandleRequest(ctx context.Context, ccx *ConnectionContext, r *ssh.Request)
}

type NewChanHandler interface {
	HandleNewChan(context.Context, *ConnectionContext, ssh.NewChannel)
}

type NewChanHandlerFunc func(context.Context, *ConnectionContext, ssh.NewChannel)

func (f NewChanHandlerFunc) HandleNewChan(ctx context.Context, ccx *ConnectionContext, ch ssh.NewChannel) {
	f(ctx, ccx, ch)
}

// NewConnHandler is called once per incoming connection.
// Errors terminate the incoming connection.  The returned context
// must be the same as, or a child of, the passed in context.
type NewConnHandler interface {
	HandleNewConn(ctx context.Context, ccx *ConnectionContext) (context.Context, error)
}

// NewConnHandlerFunc wraps a function to satisfy NewConnHandler interface.
type NewConnHandlerFunc func(ctx context.Context, ccx *ConnectionContext) (context.Context, error)

func (f NewConnHandlerFunc) HandleNewConn(ctx context.Context, ccx *ConnectionContext) (context.Context, error) {
	return f(ctx, ccx)
}

type AuthMethods struct {
	PublicKey PublicKeyFunc
	Password  PasswordFunc
	NoClient  bool
}

// GetHostSignersFunc is an infallible function that returns host signers for
// use with a new SSH connection. It should not block, as it's called while the
// SSH client is already connected and waiting for the SSH handshake.
type GetHostSignersFunc = func() []ssh.Signer

// StaticHostSigners returns a [GetHostSignersFunc] that always returns the
// given host signers.
func StaticHostSigners(hostSigners ...ssh.Signer) GetHostSignersFunc {
	return func() []ssh.Signer {
		return hostSigners
	}
}

func (s *Server) checkArguments(a utils.NetAddr, h NewChanHandler, getHostSigners GetHostSignersFunc, ah AuthMethods) error {
	// If the server is not in tunnel mode, an address must be specified.
	if s.listener != nil {
		if a.Addr == "" || a.AddrNetwork == "" {
			return trace.BadParameter("addr: specify network and the address for listening socket")
		}
	}

	if h == nil {
		return trace.BadParameter("missing NewChanHandler")
	}
	if getHostSigners == nil {
		return trace.BadParameter("missing GetHostSignersFunc")
	}
	if err := s.validateHostSigners(getHostSigners()); err != nil {
		return trace.Wrap(err)
	}
	if ah.PublicKey == nil && ah.Password == nil && !ah.NoClient {
		return trace.BadParameter("need at least one auth method")
	}
	return nil
}

func (s *Server) validateHostSigners(hostSigners []ssh.Signer) error {
	for _, signer := range hostSigners {
		if signer == nil {
			return trace.BadParameter("host signer can not be nil")
		}
		if s.insecureSkipHostValidation {
			continue
		}
		if err := validateHostSigner(s.fips, signer); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// validateHostSigner make sure the signer is a valid certificate.
func validateHostSigner(fips bool, signer ssh.Signer) error {
	cert, ok := signer.PublicKey().(*ssh.Certificate)
	if !ok {
		return trace.BadParameter("only host certificates supported")
	}
	if len(cert.ValidPrincipals) == 0 {
		return trace.BadParameter("at least one valid principal is required in host certificate")
	}

	certChecker := sshutils.CertChecker{
		FIPS: fips,
	}
	err := certChecker.CheckCert(cert.ValidPrincipals[0], cert)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type (
	PublicKeyFunc func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error)
	PasswordFunc  func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error)
)

// ClusterDetails specifies information about a cluster
type ClusterDetails struct {
	RecordingProxy bool
	FIPSEnabled    bool
}

// SSHServerVersionOverrider returns a SSH server version string that should be
// used instead of the one from a static configuration (typically because the
// version was already sent and can't be un-sent). If SSHServerVersionOverride
// returns a blank string (which is an invalid version string, as version
// strings should start with "SSH-2.0-") then no override is specified. The
// string is intended to be passed as the [ssh.ServerConfig.ServerVersion], so
// it should not include a trailing CRLF pair ("\r\n").
type SSHServerVersionOverrider interface {
	SSHServerVersionOverride() string
}

func serverVersionOverrideFromConn(nc net.Conn) string {
	for nc != nil {
		if overrider, ok := nc.(SSHServerVersionOverrider); ok {
			if v := overrider.SSHServerVersionOverride(); v != "" {
				return v
			}
		}

		netConner, ok := nc.(interface{ NetConn() net.Conn })
		if !ok {
			break
		}
		nc = netConner.NetConn()
	}
	return ""
}
