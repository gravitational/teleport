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
	"slices"
	"strconv"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apitypes "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	netutils "github.com/gravitational/teleport/api/utils/net"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type tcpServer struct {
	emitter apievents.Emitter
	hostID  string
	log     *slog.Logger
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

	var serverConn net.Conn
	if len(app.GetTCPPorts()) > 0 {
		// Multi-port TCP app.
		targetPort := int(identity.RouteToApp.TargetPort)
		serverConn, err = dialMultiPortTCPApp(ctx, dialer, addr, targetPort, app)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// Regular TCP app. addr includes port number.
		serverConn, err = dialer.DialContext(ctx, addr.AddrNetwork, addr.String())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  s.emitter,
		Recorder: events.WithNoOpPreparer(events.NewDiscardRecorder()),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := audit.OnSessionStart(ctx, s.hostID, identity, app); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		// The connection context may be closed once the connection is closed.
		ctx := context.Background()
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

// dialMultiPortTCPApp assumes that app has TCP ports specifies and dials targetPort if it's found
// in TCP ports.
//
// If the client did not include targetPort (it's equal to zero), it dials the first port found in
// TCP ports.
func dialMultiPortTCPApp(ctx context.Context, dialer net.Dialer, appAddr *utils.NetAddr, targetPort int, app apitypes.Application) (net.Conn, error) {
	// If the client didn't supply a target port, use the first port found in TCP ports. This is to
	// provide backwards compatibility.
	//
	// In theory, this behavior could be removed in the future if we guarantee that all clients always
	// send a target port when connecting to multi-port apps, but no such effort was undertaken so far.
	if targetPort == 0 {
		firstPort := int(app.GetTCPPorts()[0].Port)
		appAddrWithFirstPort := net.JoinHostPort(appAddr.Host(), strconv.Itoa(firstPort))

		serverConn, err := dialer.DialContext(ctx, appAddr.AddrNetwork, appAddrWithFirstPort)
		return serverConn, trace.Wrap(err)
	}

	isTargetPortInTCPPorts := slices.ContainsFunc(app.GetTCPPorts(), func(portRange *apitypes.PortRange) bool {
		return netutils.IsPortInRange(int(portRange.Port), int(portRange.EndPort), targetPort)
	})

	if !isTargetPortInTCPPorts {
		// This is not treated as an access denied error since there's no RBAC on TCP ports.
		return nil, trace.BadParameter("port %d is not in TCP ports of app %q", targetPort, app.GetName())
	}

	appAddrWithTargetPort := net.JoinHostPort(appAddr.Host(), strconv.Itoa(targetPort))
	serverConn, err := dialer.DialContext(ctx, appAddr.AddrNetwork, appAddrWithTargetPort)
	return serverConn, trace.Wrap(err)
}
