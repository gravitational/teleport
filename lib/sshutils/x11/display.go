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
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"

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
	// DisplayEnv is an environment variable used to determine what
	// local display should be connected to during X11 forwarding.
	DisplayEnv = "DISPLAY"

	// x11BasePort is the base port used for XServer tcp addresses.
	// e.g. DISPLAY=localhost:10 -> net.Dial("tcp", "localhost:6010")
	// Used by some XServer clients, such as openSSH and MobaXTerm.
	x11BasePort = 6000
	// x11SocketDirName is the name of the directory where X11 unix sockets are kept.
	x11SocketDirName = ".X11-unix"
)

// Display is an XServer display.
type Display struct {
	// HostName is the display's hostname. For tcp display sockets, this will be
	// an ip address. For unix display sockets, this will be empty or "unix".
	HostName string `json:"hostname"`
	// DisplayNumber is a number representing an x display.
	DisplayNumber int `json:"display_number"`
	// ScreenNumber is a specific screen number of an x display.
	ScreenNumber int `json:"screen_number"`
}

// String returns the string representation of the display.
func (d *Display) String() string {
	return fmt.Sprintf("%s:%d.%d", d.HostName, d.DisplayNumber, d.ScreenNumber)
}

// Dial opens an XServer connection to the display
func (d *Display) Dial() (XServerConn, error) {
	var conn XServerConn

	unixSock, unixErr := d.unixSocket()
	if unixErr == nil {
		if conn, unixErr = net.DialUnix("unix", nil, unixSock); unixErr == nil {
			return conn, nil
		}
	}

	tcpSock, tcpErr := d.tcpSocket()
	if tcpErr == nil {
		if conn, tcpErr = net.DialTCP("tcp", nil, tcpSock); tcpErr == nil {
			return conn, nil
		}
	}

	return nil, trace.NewAggregate(unixErr, tcpErr)
}

// Listen opens an XServer listener. It will attempt to listen on the display
// address for both tcp and unix and return an aggregate error, unless one
// results in an addr in use error.
func (d *Display) Listen() (XServerListener, error) {
	unixSock, unixErr := d.unixSocket()
	if unixErr == nil {
		var l *net.UnixListener
		if l, unixErr = net.ListenUnix("unix", unixSock); unixErr == nil {
			return &xserverUnixListener{l}, nil
		} else if errors.Is(unixErr, syscall.EADDRINUSE) {
			return nil, trace.Wrap(unixErr)
		}
	}

	tcpSock, tcpErr := d.tcpSocket()
	if tcpErr == nil {
		var l *net.TCPListener
		if l, tcpErr = net.ListenTCP("tcp", tcpSock); tcpErr == nil {
			return &xserverTCPListener{l}, nil
		} else if errors.Is(tcpErr, syscall.EADDRINUSE) {
			return nil, trace.Wrap(tcpErr)
		}

	}

	return nil, trace.NewAggregate(unixErr, tcpErr)
}

// xserverUnixSocket returns the display's associated unix socket.
func (d *Display) unixSocket() (*net.UnixAddr, error) {
	// If hostname is "unix" or empty, then the actual unix socket
	// for the display is "/tmp/.X11-unix/X<display_number>"
	if d.HostName == "unix" || d.HostName == "" {
		sockName := filepath.Join(x11SockDir(), fmt.Sprintf("X%d", d.DisplayNumber))
		return net.ResolveUnixAddr("unix", sockName)
	}

	// It's possible that the display is actually the full path
	// to an open XServer socket, such as with xquartz on OSX:
	// "/private/tmp/com.apple.com/launchd.xxx/org.xquartz.com:0"
	if d.HostName[0] == '/' {
		sockName := d.String()
		if _, err := os.Stat(sockName); err == nil {
			return net.ResolveUnixAddr("unix", sockName)
		}

		// The socket might not include the screen number.
		sockName = fmt.Sprintf("%s:%d", d.HostName, d.DisplayNumber)
		if _, err := os.Stat(sockName); err == nil {
			return net.ResolveUnixAddr("unix", sockName)
		}
	}

	return nil, trace.BadParameter("display is not a unix socket")
}

// xserverTCPSocket returns the display's associated tcp socket.
// e.g. "hostname:<6000+display_number>"
func (d *Display) tcpSocket() (*net.TCPAddr, error) {
	if d.HostName == "" {
		return nil, trace.BadParameter("display is not a tcp socket, hostname can't be empty")
	}

	port := fmt.Sprint(d.DisplayNumber + x11BasePort)
	rawAddr := net.JoinHostPort(d.HostName, port)
	addr, err := net.ResolveTCPAddr("tcp", rawAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return addr, nil
}

// GetXDisplay retrieves and validates the local XServer display
// set in the environment variable $DISPLAY.
func GetXDisplay() (Display, error) {
	displayString := os.Getenv(DisplayEnv)
	if displayString == "" {
		return Display{}, trace.BadParameter("$DISPLAY not set")
	}

	display, err := ParseDisplay(displayString)
	if err != nil {
		return Display{}, trace.Wrap(err)
	}
	return display, nil
}

// ParseDisplay parses the given display value and returns the host,
// display number, and screen number, or a parsing error. display must be
// in one of the following formats - hostname:d[.s], unix:d[.s], :d[.s], ::d[.s].
func ParseDisplay(displayString string) (Display, error) {

	if displayString == "" {
		return Display{}, trace.BadParameter("display cannot be an empty string")
	}

	// check the display for illegal characters in case of code injection attempt
	allowedSpecialChars := ":/.-_" // chars used for hostname or display delimiters.
	for _, c := range displayString {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c) || strings.ContainsRune(allowedSpecialChars, c)) {
			return Display{}, trace.BadParameter("display contains invalid character %q", c)
		}
	}

	// Parse hostname.
	colonIdx := strings.LastIndex(displayString, ":")
	if colonIdx == -1 || len(displayString) == colonIdx+1 {
		return Display{}, trace.BadParameter("display value is missing display number")
	}

	var display Display
	if displayString[0] == ':' {
		display.HostName = ""
	} else {
		display.HostName = displayString[:colonIdx]
	}

	// Parse display number and screen number
	splitDot := strings.Split(displayString[colonIdx+1:], ".")
	displayNumber, err := strconv.ParseUint(splitDot[0], 10, 0)
	if err != nil {
		return Display{}, trace.Wrap(err)
	}

	display.DisplayNumber = int(displayNumber)
	if len(splitDot) < 2 {
		return display, nil
	}

	screenNumber, err := strconv.ParseUint(splitDot[1], 10, 0)
	if err != nil {
		return Display{}, trace.Wrap(err)
	}

	display.ScreenNumber = int(screenNumber)
	return display, nil
}

func x11SockDir() string {
	// We use "/tmp" instead of "os.TempDir" because X11
	// is not OS aware and uses "/tmp" regardless of OS.
	return filepath.Join("/tmp", x11SocketDirName)
}
