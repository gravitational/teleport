/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
	"github.com/mattn/go-shellwords"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/utils"
)

type ProxyServerConfig struct {
	Authorizer  authz.Authorizer
	AuthClient  authclient.ClientI
	AccessPoint authclient.ProxyAccessPoint
}

func (c *ProxyServerConfig) Check() error {
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	return nil
}

type ProxyServer struct {
	cfg        *ProxyServerConfig
	middleware *auth.Middleware
	logger     *slog.Logger
}

func NewProxyServer(ctx context.Context, cfg *ProxyServerConfig) (*ProxyServer, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := cfg.AccessPoint.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	middleware := &auth.Middleware{
		ClusterName: clusterName.GetClusterName(),
	}

	return &ProxyServer{
		cfg:        cfg,
		middleware: middleware,
		logger:     slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentProxy, "mcp")),
	}, nil
}

func (s *ProxyServer) HandleConnection(ctx context.Context, conn net.Conn) error {
	defer conn.Close()
	tlsConn, ok := conn.(utils.TLSConn)
	if !ok {
		return trace.BadParameter("expected *tls.Conn, got: %T", conn)
	}

	ctx, err := s.middleware.WrapContextWithUser(ctx, tlsConn)
	if err != nil {
		return trace.Wrap(err)
	}

	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO replace me with real impl
	cmdToRun := os.Getenv("TELEPORT_MCP_RUN_POSTGRES")
	s.logger.DebugContext(ctx, "=== MCP server authorized", "user", authCtx.User, "cmd", cmdToRun)
	if cmdToRun != "" {
		parts, err := shellwords.Parse(cmdToRun)
		if err != nil {
			return trace.BadParameter("cannot parse mcp.run: %v", err)
		}
		s.logger.DebugContext(ctx, "=== running tmp postgres server ", "command", parts)
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		cmd.Stdin = tlsConn
		cmd.Stdout = tlsConn
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			return trace.Wrap(err)
		}
		return cmd.Wait()
	} else {
		_, err := tlsConn.Write([]byte("hello teleport"))
		return trace.Wrap(err)
	}
}
