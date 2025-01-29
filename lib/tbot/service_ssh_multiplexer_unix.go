//go:build unix

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

package tbot

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/uds"
)

// ConnectToSSHMultiplex connects to the SSH multiplexer and sends the target
// to the multiplexer. It then returns the connection to the SSH multiplexer
// over stdout.
func ConnectToSSHMultiplex(ctx context.Context, socketPath string, target string, stdout *os.File) error {
	outConn, err := net.FileConn(stdout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer outConn.Close()
	outUnix, ok := outConn.(*net.UnixConn)
	if !ok {
		return trace.BadParameter("expected stdout to be %T, got %T", outUnix, outConn)
	}

	c, err := new(uds.Dialer).DialUnix(ctx, "unix", socketPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer c.Close()

	if _, err := fmt.Fprint(c, target, "\x00"); err != nil {
		return trace.Wrap(err)
	}

	rawC, err := c.SyscallConn()
	if err != nil {
		return trace.Wrap(err)
	}

	var innerErr error
	if controlErr := rawC.Control(func(fd uintptr) {
		// We have to write something in order to send a control message so
		// we just write NUL.
		_, _, innerErr = outUnix.WriteMsgUnix(
			[]byte{0},
			syscall.UnixRights(int(fd)),
			nil,
		)
	}); controlErr != nil {
		return trace.Wrap(controlErr)
	}
	if innerErr != nil {
		return trace.Wrap(err)
	}

	return nil
}
