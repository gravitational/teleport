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

package app

import (
	"context"
	"log/slog"
	"net"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type tcpServer struct {
	newAudit func(ctx context.Context, sessionID string) (common.Audit, error)
	hostID   string
	log      *slog.Logger
}

// handleConnection handles connection from a TCP application.
func (s *tcpServer) handleConnection(ctx context.Context, clientConn net.Conn, identity *tlsca.Identity, app apitypes.Application) error {
	addr, err := utils.ParseAddr(app.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}
	if addr.AddrNetwork != "tcp" {
		return trace.BadParameter(`unexpected app %q address network, expected "tcp": %+v`, app.GetName(), addr)
	}
	dialer := net.Dialer{
		Timeout: apidefaults.DefaultIOTimeout,
	}
	serverConn, err := dialer.DialContext(ctx, addr.AddrNetwork, addr.String())
	if err != nil {
		return trace.Wrap(err)
	}

	audit, err := s.newAudit(ctx, identity.RouteToApp.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := audit.OnSessionStart(ctx, s.hostID, identity, app); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := audit.OnSessionEnd(ctx, s.hostID, identity, app); err != nil {
			s.log.WarnContext(ctx, "Failed to emit session end event for app.", "app", app.GetName(), "error", err)
		}
	}()
	err = utils.ProxyConn(ctx, clientConn, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
