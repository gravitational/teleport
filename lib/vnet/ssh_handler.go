// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

// sshHandler handles incoming VNet SSH connections.
type sshHandler struct {
	cfg sshHandlerConfig
}

type sshHandlerConfig struct {
	sshProvider *sshProvider
	target      dialTarget
}

func newSSHHandler(cfg sshHandlerConfig) *sshHandler {
	return &sshHandler{
		cfg: cfg,
	}
}

// handleTCPConnector handles an incoming TCP connection from VNet and proxies
// the connection to a target SSH node.
func (h *sshHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	if localPort != 22 {
		return trace.BadParameter("SSH is only handled on port 22")
	}
	targetConn, err := h.cfg.sshProvider.dial(ctx, h.cfg.target)
	if err != nil {
		return trace.Wrap(err)
	}
	defer targetConn.Close()
	return trace.Wrap(h.handleTCPConnectorWithTargetConn(ctx, localPort, connector, targetConn))
}

// handleTCPConnectorWithTargetTCPConn handles an incoming TCP connection from
// VNet when a TCP connection to the target host has already been established.
func (h *sshHandler) handleTCPConnectorWithTargetConn(
	ctx context.Context,
	localPort uint16,
	connector func() (net.Conn, error),
	targetConn net.Conn,
) error {
	// For now we accept the incoming TCP conn to indicate that the node exists,
	// but SSH connection forwarding is not implemented yet so we immediately
	// close it.
	localConn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	localConn.Close()
	return trace.NotImplemented("VNet SSH connection forwarding is not yet implemented")
}
