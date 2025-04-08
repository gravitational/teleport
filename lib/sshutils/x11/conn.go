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

package x11

import (
	"errors"
	"io"
	"math"
	"net"
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

// XServerConn is a connection to an XServer through a direct or forwarded connection.
// It can either be a tcp conn, unix conn, or an X11 channel.
type XServerConn interface {
	io.ReadWriteCloser
	// must implement CloseWrite to prevent read loop from halting pre-maturely
	CloseWrite() error
}

// XServerListener is a proxy XServer listener used to forward XServer requests.
// to an actual XServer. The underlying listener may be a unix or tcp listener.
type XServerListener interface {
	// Accept waits for and returns the next connection to the listener.
	Accept() (XServerConn, error)

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error

	// Addr returns the listener's network address.
	Addr() net.Addr
}

type xserverUnixListener struct {
	*net.UnixListener
}

func (l *xserverUnixListener) Accept() (XServerConn, error) {
	return l.AcceptUnix()
}

type xserverTCPListener struct {
	*net.TCPListener
}

func (l *xserverTCPListener) Accept() (XServerConn, error) {
	return l.AcceptTCP()
}

// OpenNewXServerListener opens an XServerListener for the first available Display.
// displayOffset will determine what display number to start from when searching for
// an open display unix socket, and maxDisplays in optional limit for the number of
// display sockets which can be opened at once.
func OpenNewXServerListener(displayOffset int, maxDisplay int, screen uint32) (XServerListener, Display, error) {
	if displayOffset > maxDisplay {
		return nil, Display{}, trace.BadParameter("displayOffset (%d) cannot be larger than maxDisplay (%d)", displayOffset, maxDisplay)
	} else if maxDisplay > MaxDisplayNumber {
		return nil, Display{}, trace.BadParameter("maxDisplay (%d) cannot be larger than the max int32 (%d)", maxDisplay, math.MaxInt32)
	}

	// Create /tmp/.X11-unix if it doesn't exist (such as in CI)
	if err := os.Mkdir(x11SockDir(), 0o777|os.ModeSticky); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, Display{}, trace.Wrap(err)
	}

	for displayNumber := displayOffset; displayNumber <= maxDisplay; displayNumber++ {
		display := Display{DisplayNumber: displayNumber}
		if l, err := display.Listen(); err == nil {
			return l, display, nil
		} else if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, Display{}, trace.Wrap(err)
		}
	}

	return nil, Display{}, trace.LimitExceeded("No more X11 sockets are available")
}
