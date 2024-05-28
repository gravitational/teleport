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
	"errors"
	"math"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	peerKeepAlive = time.Second * 10
	peerTimeout   = time.Second * 20
)

// ServerConfig configures a Server instance.
type ServerConfig struct {
	AccessCache   authclient.CAGetter
	Listener      net.Listener
	TLSConfig     *tls.Config
	ClusterDialer ClusterDialer
	Log           logrus.FieldLogger
	ClusterName   string

	// getConfigForClient gets the client tls config.
	// configurable for testing purposes.
	getConfigForClient func(*tls.ClientHelloInfo) (*tls.Config, error)

	// service is a custom ProxyServiceServer
	// configurable for testing purposes.
	service proto.ProxyServiceServer
}

// checkAndSetDefaults checks and sets default values
func (c *ServerConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.New()
	}
	c.Log = c.Log.WithField(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentProxy, "peer"),
	)

	if c.AccessCache == nil {
		return trace.BadParameter("missing access cache")
	}

	if c.Listener == nil {
		return trace.BadParameter("missing listener")
	}

	if c.ClusterDialer == nil {
		return trace.BadParameter("missing cluster dialer server")
	}

	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}

	if c.TLSConfig == nil {
		return trace.BadParameter("missing tls config")
	}

	if len(c.TLSConfig.Certificates) == 0 {
		return trace.BadParameter("missing tls certificate")
	}

	c.TLSConfig = c.TLSConfig.Clone()
	c.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert

	if c.getConfigForClient == nil {
		c.getConfigForClient = getConfigForClient(c.TLSConfig, c.AccessCache, c.Log, c.ClusterName)
	}
	c.TLSConfig.GetConfigForClient = c.getConfigForClient

	if c.service == nil {
		c.service = &proxyService{
			c.ClusterDialer,
			c.Log,
		}
	}

	return nil
}

// Server is a proxy service server using grpc and tls.
type Server struct {
	config ServerConfig
	server *grpc.Server
}

// NewServer creates a new proxy server instance.
func NewServer(config ServerConfig) (*Server, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	metrics, err := newServerMetrics()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reporter := newReporter(metrics)

	server := grpc.NewServer(
		grpc.Creds(newServerCredentials(credentials.NewTLS(config.TLSConfig))),
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

	proto.RegisterProxyServiceServer(server, config.service)

	return &Server{
		config: config,
		server: server,
	}, nil
}

// Serve starts the proxy server.
func (s *Server) Serve() error {
	if err := s.server.Serve(s.config.Listener); err != nil {
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
