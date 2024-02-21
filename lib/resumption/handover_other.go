//go:build !unix

// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package resumption

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

func (r *SSHServerWrapper) listenHandover(token resumptionToken) (net.Listener, error) {
	return nil, trace.NotImplemented("handover is not implemented for the current platform")
}

func (r *SSHServerWrapper) dialHandover(token resumptionToken) (net.Conn, error) {
	return nil, trace.NotFound("handover is not implemented for the current platform")
}

// HandoverCleanup does nothing, because on this platform we don't support
// hand-over sockets, so there can't be anything to clean up.
func (r *SSHServerWrapper) HandoverCleanup(context.Context) error {
	return nil
}
