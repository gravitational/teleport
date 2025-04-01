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

package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os/exec"

	"github.com/gravitational/trace"

	apitypes "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type mcpServer struct {
	emitter apievents.Emitter
	hostID  string
	log     *slog.Logger
}

// handleConnection handles connection from an MCP application.
func (s *mcpServer) handleConnection(ctx context.Context, clientConn net.Conn, identity *tlsca.Identity, app apitypes.Application) error {
	s.log.DebugContext(ctx, "Running mcp", "app", app.GetName(),
		"cmd", app.GetMCPCommand(), "args", app.GetMCPArgs())

	// TODO hijack the input/output and parse with SDK?
	cmd := exec.CommandContext(ctx, app.GetMCPCommand(), app.GetMCPArgs()...)
	cmd.Stdin = io.TeeReader(clientConn, &dumpWriter{
		ctx:    ctx,
		logger: s.log.With("stdio", "in"),
	})
	cmd.Stdout = io.MultiWriter(utils.NewSyncWriter(clientConn), &dumpWriter{
		ctx:    ctx,
		logger: s.log.With("stdio", "out"),
	})
	cmd.Stderr = &dumpWriter{
		ctx:    ctx,
		logger: s.log.With("stdio", "err"),
	}
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return cmd.Wait()
}

type dumpWriter struct {
	ctx    context.Context
	logger *slog.Logger
}

func (d *dumpWriter) Write(p []byte) (int, error) {
	d.logger.DebugContext(d.ctx, "=== dump", "data", string(p))
	return len(p), nil
}
