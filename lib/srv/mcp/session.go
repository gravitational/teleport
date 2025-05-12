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
	"log/slog"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type sessionCtx struct {
	parentCtx    context.Context
	clientConn   net.Conn
	authCtx      *authz.Context
	identity     tlsca.Identity
	app          types.Application
	serverID     string
	log          *slog.Logger
	emitter      apievents.Emitter
	allowedTools services.EnumerationResult
	idTracker    *idTracker
}

func (c *sessionCtx) checkAccessToTool(toolName string) error {
	if c.allowedTools.WildcardDenied() {
		return trace.AccessDenied("access to MCP tool %q is denied", toolName)
	}

	if result, err := utils.SliceMatchesRegex(toolName, c.allowedTools.Denied()); err != nil {
		return trace.Wrap(err)
	} else if result {
		return trace.AccessDenied("access to MCP tool %q is denied", toolName)
	}

	if c.allowedTools.WildcardAllowed() {
		return nil
	}

	if result, err := utils.SliceMatchesRegex(toolName, c.allowedTools.Allowed()); err != nil {
		return trace.Wrap(err)
	} else if !result {
		return trace.AccessDenied("access to MCP tool %q is denied", toolName)
	}
	return nil
}
