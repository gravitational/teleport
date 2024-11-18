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
	targetPort := int(identity.RouteToApp.TargetPort)

	var dialTarget string
	switch {
	// Regular TCP app with port in URI in app spec.
	case len(app.GetTCPPorts()) < 1:
		if err := ensureZeroTargetPortOrEqualToPortFromURI(addr, targetPort); err != nil {
			return trace.Wrap(err, "comparing target port against port from URI of app %q", app.GetName())
		}

		dialTarget = addr.String()
	// Multi-port TCP app but target port was not provided.
	case targetPort == 0:
		// If the client didn't supply a target port, use the first port found in TCP ports. This is to
		// provide backwards compatibility.
		//
		// In theory, this behavior could be removed in the future if we guarantee that all clients
		// always send a target port when connecting to multi-port apps, but no such effort was
		// undertaken so far.
		firstPort := int(app.GetTCPPorts()[0].Port)
		dialTarget = net.JoinHostPort(addr.Host(), strconv.Itoa(firstPort))
	// Multi-port TCP app with target port specified in cert.
	default:
		isTargetPortInTCPPorts := slices.ContainsFunc(app.GetTCPPorts(), func(portRange *apitypes.PortRange) bool {
			return netutils.IsPortInRange(int(portRange.Port), int(portRange.EndPort), targetPort)
		})

		if !isTargetPortInTCPPorts {
			// This is not treated as an access denied error since there's no RBAC on TCP ports.
			return trace.BadParameter("port %d is not in TCP ports of app %q", targetPort, app.GetName())
		}

		dialTarget = net.JoinHostPort(addr.Host(), strconv.Itoa(targetPort))
	}

	serverConn, err := dialer.DialContext(ctx, addr.AddrNetwork, dialTarget)
	if err != nil {
		return trace.Wrap(err)
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

// ensureZeroTargetPortOrEqualToPortFromURI handles an esoteric edge case where a connection to a
// single-port TCP app was made with a cert that includes TargetPort meant for multi-port apps.
//
// This can happen when the cert was generated before the app spec was changed in a way that
// transitioned the app from multi-port to single-port. It can also happen due to a programmer error
// where TargetPort is provided despite the app being single-port.
func ensureZeroTargetPortOrEqualToPortFromURI(addr *utils.NetAddr, targetPort int) error {
	if targetPort == 0 {
		return nil
	}

	addrPort := addr.Port(0)
	if addrPort == 0 {
		return trace.Errorf("missing or invalid port number in URI %q", addr.String())
	}

	if targetPort != addrPort {
		return trace.BadParameter("target port is not equal to port number from URI (%d vs %d)", targetPort, addrPort)
	}

	return nil
}
