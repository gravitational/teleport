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

	"github.com/google/uuid"
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
	sessionID := uuid.New().String()

	log := s.log.With("session", sessionID)

	log.DebugContext(ctx, "Running mcp",
		"app", app.GetName(),
		"cmd", app.GetMCPCommand(),
		"args", app.GetMCPArgs(),
	)

	mkWriter := func(handleName string, emitEvents bool) *dumpWriter {
		if emitEvents {
			return newDumpWriter(ctx, handleName, s.emitter, log, identity, sessionID)
		}
		return newDumpWriter(ctx, handleName, nil, log, identity, sessionID)
	}

	cmd := exec.CommandContext(ctx, app.GetMCPCommand(), app.GetMCPArgs()...)
	cmd.Stdin = io.TeeReader(clientConn, mkWriter("in", true))
	cmd.Stdout = io.MultiWriter(utils.NewSyncWriter(clientConn), mkWriter("out", false))
	cmd.Stderr = mkWriter("err", false)
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return cmd.Wait()
}

func newDumpWriter(ctx context.Context, handleName string, emitter apievents.Emitter, log *slog.Logger, identity *tlsca.Identity, sessionID string) *dumpWriter {
	return &dumpWriter{
		ctx:       ctx,
		logger:    log.With("stdio", handleName),
		emitter:   emitter,
		identity:  identity,
		sessionID: sessionID,
	}
}

type dumpWriter struct {
	ctx       context.Context
	logger    *slog.Logger
	identity  *tlsca.Identity
	emitter   apievents.Emitter
	sessionID string
}

func (d *dumpWriter) emitAuditEvent(msg string) {
	if d.emitter == nil {
		return
	}

	event, emit, err := mcpMessageToEvent(msg)
	if err != nil {
		d.logger.WarnContext(d.ctx, "Failed to parse RPC message", "error", err)
		return
	}
	if !emit {
		return
	}
	d.logger.InfoContext(d.ctx, "event", "val", event)

	if err := d.emitter.EmitAuditEvent(d.ctx, event); err != nil {
		d.logger.WarnContext(d.ctx, "Failed to emit MCP call event.", "error", err)
	}
}

func (d *dumpWriter) Write(p []byte) (int, error) {
	d.emitAuditEvent(string(p))
	d.logger.DebugContext(d.ctx, "=== dump", "data", string(p))
	return len(p), nil
}
