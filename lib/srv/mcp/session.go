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
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
)

// SessionCtx contains basic information of an MCP session.
type SessionCtx struct {
	// ClientConn is the incoming client connection.
	ClientConn net.Conn
	// AuthCtx is the authorization context.
	AuthCtx *authz.Context
	// App is the MCP server application being accessed.
	App types.Application
	// Identity is the user identity.
	Identity tlsca.Identity

	// sessionID is the session ID.
	sessionID session.ID
}

func (c *SessionCtx) checkAndSetDefaults() error {
	if c.ClientConn == nil {
		return trace.BadParameter("missing ClientConn")
	}
	if c.AuthCtx == nil {
		return trace.BadParameter("missing AuthCtx")
	}
	if c.App == nil {
		return trace.BadParameter("missing App")
	}
	if c.Identity.Username == "" {
		c.Identity = c.AuthCtx.Identity.GetIdentity()
	}
	if c.sessionID == "" {
		// Do not use web session ID from the app route.
		c.sessionID = session.NewID()
	}
	return nil
}
