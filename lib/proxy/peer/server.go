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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"math"
	"net"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
	"github.com/gravitational/teleport/lib/proxy/peer/internal"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	peerKeepAlive = time.Second * 10
	peerTimeout   = time.Second * 20
)

// ServerConfig configures a Server instance.
type ServerConfig struct {
	Log    *slog.Logger
	Dialer peerdial.Dialer

	CipherSuites   []uint16
	GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	GetClientCAs   func(*tls.ClientHelloInfo) (*x509.CertPool, error)

	// service is a custom ProxyServiceServer
	// configurable for testing purposes.
	service proto.ProxyServiceServer
}

// checkAndSetDefaults checks and sets default values
func (c *ServerConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.Default()
	}
	c.Log = c.Log.With(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentProxy, "peer"),
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

	if c.service == nil {
		c.service = &proxyService{
			dialer: c.Dialer,
			log:    c.Log,
		}
	}

	return nil
}

// Server is a proxy service server using grpc and tls.
type Server struct {
	log    *slog.Logger
	dialer peerdial.Dialer
	server *grpc.Server
}

// NewServer creates a new proxy server instance.
func NewServer(cfg ServerConfig) (*Server, error) {
	err := cfg.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	metrics, err := newServerMetrics()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reporter := newReporter(metrics)

	tlsConfig := utils.TLSConfig(cfg.CipherSuites)
	tlsConfig.NextProtos = []string{"h2"}
	tlsConfig.GetCertificate = cfg.GetCertificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.VerifyPeerCertificate = internal.VerifyPeerCertificateIsProxy

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

	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.StatsHandler(newStatsHandler(reporter)),
		grpc.ChainStreamInterceptor(metadata.StreamServerInterceptor, interceptors.GRPCServerStreamErrorInterceptor),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    peerKeepAlive,
			Timeout: peerTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             peerKeepAlive,
			PermitWithoutStream: true,
		}),

		// the proxy peering server uses transport authentication to verify that
		// the client is another Teleport proxy, and the proxy peering service
		// is intended for mass connection routing (spawning an unbounded amount
		// of streams of unbounded duration), so adding a limit on concurrent
		// streams (for example to prevent CVE-2023-44487, see
		// https://github.com/grpc/grpc-go/pull/6703 ) is unnecessary and
		// counterproductive to the functionality of proxy peering
		grpc.MaxConcurrentStreams(math.MaxUint32),
	)

	proto.RegisterProxyServiceServer(server, cfg.service)

	return &Server{
		log:    cfg.Log,
		dialer: cfg.Dialer,
		server: server,
	}, nil
}

// Serve starts the proxy server.
func (s *Server) Serve(l net.Listener) error {
	if err := s.server.Serve(l); err != nil {
		if errors.Is(err, grpc.ErrServerStopped) ||
			utils.IsUseOfClosedNetworkError(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return nil
}

// Close closes the proxy server immediately.
func (s *Server) Close() error {
	s.server.Stop()
	return nil
}

// Shutdown does a graceful shutdown of the proxy server.
func (s *Server) Shutdown() error {
	s.server.GracefulStop()
	return nil
}
