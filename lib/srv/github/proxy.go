/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package github

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/trace"
)

// ConnMonitor monitors authorized connections and terminates them when
// session controls dictate so.
type ConnMonitor interface {
	MonitorConn(ctx context.Context, authzCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error)
}

// ProxyServerConfig is the proxy configuration.
type ProxyServerConfig struct {
	// TLSConfig is the proxy server TLS configuration.
	TLSConfig *tls.Config
	// Limiter is the connection/rate limiter.
	Limiter *limiter.Limiter
	// Log is slog.Logger
	Log *slog.Logger
	// IngressReporter reports new and active connections.
	IngressReporter *ingress.Reporter
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *ProxyServerConfig) CheckAndSetDefaults() error {
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLSConfig")
	}
	if c.Limiter == nil {
		// Empty config means no connection limit.
		connLimiter, err := limiter.NewLimiter(limiter.Config{})
		if err != nil {
			return trace.Wrap(err)
		}

		c.Limiter = connLimiter
	}
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, "github:proxy")
	}
	return nil
}

type ProxyServer struct {
	cfg ProxyServerConfig
}

func NewProxyServer(ctx context.Context, config ProxyServerConfig) (*ProxyServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	server := &ProxyServer{
		cfg: config,
	}
	return server, nil
}

func (s *ProxyServer) HandleConnection(ctx context.Context, connection net.Conn) error {
	if s.cfg.IngressReporter != nil {
		s.cfg.IngressReporter.ConnectionAccepted(ingress.DatabaseTLS, conn)
		defer s.cfg.IngressReporter.ConnectionClosed(ingress.DatabaseTLS, conn)
	}
	s.cfg.Log.DebugContext(ctx, "Proxying incoming github connection.")
}
