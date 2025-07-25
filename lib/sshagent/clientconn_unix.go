//go:build !windows

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

package sshagent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// DialSystemAgent connects to the SSH agent advertised by SSH_AUTH_SOCK.
func DialSystemAgent() (io.ReadWriteCloser, error) {
	socketPath := os.Getenv(teleport.SSHAuthSock)
	logger := slog.With(teleport.ComponentKey, teleport.ComponentKeyAgent)
	logger.DebugContext(context.Background(), "Connecting to the system agent", "socket_path", socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

func isClosedConnectionError(err error) bool {
	return errors.Is(err, io.EOF)
}
