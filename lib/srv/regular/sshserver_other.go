//go:build !darwin && !linux
// +build !darwin,!linux

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package regular

import (
	"net"
	"os"
	"runtime"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv"
)

func validateListenerSocket(scx *srv.ServerContext, controlConn *net.UnixConn, listenerFD *os.File) error {
	return trace.BadParameter("remote forwarding is not implemented for %s", runtime.GOOS)
}
