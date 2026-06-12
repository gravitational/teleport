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
	"math"
	"net"
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

const (
	// DefaultDisplayOffset is the default display offset when
	// searching for an open XServer unix socket.
	DefaultDisplayOffset = 10
	// DefaultMaxDisplays is the default maximum number of displays
	// supported when searching for an open XServer unix socket.
	DefaultMaxDisplays = 1000
	// MaxDisplay is the theoretical max display value which
	// X Clients and serverwill be able to parse into a unix socket.
	MaxDisplayNumber = math.MaxInt32
)

// OpenNewXServerListener opens an XServerListener for the first available Display.
// displayOffset will determine what display number to start from when searching for
// an open display unix socket, and maxDisplays in optional limit for the number of
// display sockets which can be opened at once.
func OpenNewXServerListener(displayOffset int, maxDisplay int, screen uint32) (net.Listener, Display, error) {
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
		display := Display{DisplayNumber: displayNumber, ScreenNumber: int(screen)}
		if l, err := display.Listen(); err == nil {
			return l, display, nil
		} else if !errors.Is(err, syscall.EADDRINUSE) && !errors.Is(err, syscall.EACCES) {
			return nil, Display{}, trace.Wrap(err)
		}
	}

	return nil, Display{}, trace.LimitExceeded("No more X11 sockets are available")
}
