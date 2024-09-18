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

package peer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	peerv0c "github.com/gravitational/teleport/gen/proto/go/teleport/lib/proxy/peer/v0/peerv0connect"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ServerConfig configures a Server instance.
type ServerConfig struct {
	Log           logrus.FieldLogger
	ClusterDialer ClusterDialer

	CipherSuites   []uint16
	GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	GetClientCAs   func(*tls.ClientHelloInfo) (*x509.CertPool, error)

	// service is a custom ProxyServiceHandler configurable for testing
	// purposes.
	service peerv0c.ProxyServiceHandler
}

// checkAndSetDefaults checks and sets default values
func (c *ServerConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.StandardLogger()
	}
	c.Log = c.Log.WithField(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentProxy, "peer"),
	)

	if c.ClusterDialer == nil {
		return trace.BadParameter("missing cluster dialer server")
	}

	if c.GetCertificate == nil {
		return trace.BadParameter("missing GetCertificate")
	}
	if c.GetClientCAs == nil {
		return trace.BadParameter("missing GetClientCAs")
	}

	if c.service == nil {
		c.service = &proxyService{
			clusterDialer: c.ClusterDialer,
			log:           c.Log,
		}
	}

	return nil
}

// Server is a proxy service server using grpc and tls.
type Server struct {
	log logrus.FieldLogger

	clusterDialer ClusterDialer

	tlsConfig *tls.Config
	server    *http.Server
}

// NewServer creates a new proxy server instance.
func NewServer(cfg ServerConfig) (*Server, error) {
	err := cfg.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	handlerOptions := connect.WithHandlerOptions(
		connect.WithCompression("gzip", nil, nil),
		connect.WithInterceptors(addVersionInterceptor{}, traceErrorsInterceptor{}),
	)

	mux := http.NewServeMux()
	mux.Handle(peerv0c.NewProxyServiceHandler(cfg.service, handlerOptions))

	tlsConfig := utils.TLSConfig(cfg.CipherSuites)
	tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	tlsConfig.GetCertificate = cfg.GetCertificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.VerifyPeerCertificate = verifyPeerCertificateIsProxy

	getClientCAs := cfg.GetClientCAs
	tlsConfig.GetConfigForClient = func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		clientCAs, err := getClientCAs(chi)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c := tlsConfig.Clone()
		c.ClientCAs = clientCAs
		return c, nil
	}

	server := &http.Server{
		Handler: mux,

		ReadHeaderTimeout: time.Minute,
		IdleTimeout:       5 * time.Minute,
	}

	if err := http2.ConfigureServer(server, &http2.Server{
		MaxConcurrentStreams: math.MaxUint32,
		IdleTimeout:          30 * time.Minute,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Server{
		log: cfg.Log,

		clusterDialer: cfg.ClusterDialer,

		tlsConfig: tlsConfig,
		server:    server,
	}, nil
}

func verifyPeerCertificateIsProxy(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(verifiedChains) < 1 {
		return trace.AccessDenied("missing client certificate (this is a bug)")
	}

	clientCert := verifiedChains[0][0]
	clientIdentity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	if !slices.Contains(clientIdentity.Groups, string(types.RoleProxy)) {
		return trace.AccessDenied("expected Proxy client credentials")
	}
	return nil
}

// Serve starts the proxy server.
func (s *Server) Serve(l net.Listener) error {
	err := s.server.Serve(tls.NewListener(l, s.tlsConfig))
	if errors.Is(err, http.ErrServerClosed) ||
		utils.IsUseOfClosedNetworkError(err) {
		return nil
	}
	return trace.Wrap(err)
}

// Close closes the proxy server immediately.
func (s *Server) Close() error {
	_ = s.server.Close()
	return nil
}

// Shutdown does a graceful shutdown of the proxy server.
func (s *Server) Shutdown(ctx context.Context) error {
	_ = s.server.Shutdown(ctx)
	return nil
}

type addVersionInterceptor struct{}

var _ connect.Interceptor = addVersionInterceptor{}

// WrapStreamingClient implements [connect.Interceptor].
func (addVersionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, s connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, s)
		conn.RequestHeader().Set(metadata.VersionKey, api.Version)
		return conn
	}
}

// WrapStreamingHandler implements [connect.Interceptor].
func (addVersionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		conn.ResponseHeader().Set(metadata.VersionKey, api.Version)
		return next(ctx, conn)
	}
}

// WrapUnary implements [connect.Interceptor].
func (addVersionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set(metadata.VersionKey, api.Version)
		return next(ctx, req)
	}
}

type traceErrorsInterceptor struct{}

var _ connect.Interceptor = traceErrorsInterceptor{}

type traceErrorsStreamingClientConn struct {
	connect.StreamingClientConn
}

func (c *traceErrorsStreamingClientConn) Send(msg any) error {
	return fromConnectRPC(c.StreamingClientConn.Send(msg))
}

func (c *traceErrorsStreamingClientConn) CloseRequest() error {
	return fromConnectRPC(c.StreamingClientConn.CloseRequest())
}

func (c *traceErrorsStreamingClientConn) Receive(msg any) error {
	return fromConnectRPC(c.StreamingClientConn.Receive(msg))
}

func (c *traceErrorsStreamingClientConn) CloseResponse() error {
	return fromConnectRPC(c.StreamingClientConn.CloseResponse())
}

// WrapStreamingClient implements connect.Interceptor.
func (traceErrorsInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, s connect.Spec) connect.StreamingClientConn {
		return &traceErrorsStreamingClientConn{
			StreamingClientConn: next(ctx, s),
		}
	}
}

type traceErrorsStreamingHandlerConn struct {
	connect.StreamingHandlerConn
}

func (c *traceErrorsStreamingHandlerConn) Send(msg any) error {
	return fromConnectRPC(c.StreamingHandlerConn.Send(msg))
}

func (c *traceErrorsStreamingHandlerConn) Receive(msg any) error {
	return fromConnectRPC(c.StreamingHandlerConn.Receive(msg))
}

// WrapStreamingHandler implements connect.Interceptor.
func (traceErrorsInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return toConnectRPC(next(ctx, &traceErrorsStreamingHandlerConn{
			StreamingHandlerConn: conn,
		}))
	}
}

// WrapUnary implements connect.Interceptor.
func (traceErrorsInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		err = fromConnectRPC(err)
		return resp, err
	}
}

func toConnectRPC(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return err
	}
	err = trail.ToGRPC(err)
	return connect.NewError(connect.Code(status.Code(err)), err)
}

func fromConnectRPC(err error) error {
	if err == nil {
		return nil
	}
	if cErr := (*connect.Error)(nil); errors.As(err, &cErr) {
		return trail.FromGRPC(status.Error(codes.Code(cErr.Code()), cErr.Message()))
	}
	return err
}
