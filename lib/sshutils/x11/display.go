// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package x11

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
)

const (
	// DefaultDisplayOffset is the default display offset when
	// searching for an X11 Server reverse tunnel port.
	DefaultDisplayOffset = 10
	// DisplayEnv is an environment variable used to determine what
	// local display should be connected to during X11 forwarding.
	DisplayEnv = "DISPLAY"

	// x11BasePort is the base port used for XServer tcp addresses.
	// e.g. DISPLAY=localhost:10 -> net.Dial("tcp", "localhost:6010")
	// Used by some XServer clients, such as openSSH and MobaXTerm.
	x11BasePort = 6000
	// x11SocketDir is the name of the directory where X11 unix sockets are kept.
	x11SocketDir = ".X11-unix"
)

// Display is an XServer display.
type Display struct {
	// HostName is the the display's hostname. For tcp display sockets, this will be
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
	tcpSock, tcpErr := d.tcpSocket()
	if tcpErr == nil {
		return net.DialTCP("tcp", nil, tcpSock)
	}

	unixSock, unixErr := d.unixSocket()
	if unixErr == nil {
		return net.DialUnix("unix", nil, unixSock)
	}

	return nil, trace.NewAggregate(tcpErr, unixErr)
}

// Listen opens an XServer listener
func (d *Display) Listen() (XServerListener, error) {
	unixSock, unixErr := d.unixSocket()
	if unixErr == nil {
		l, err := net.ListenUnix("unix", unixSock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &xserverUnixListener{l}, nil
	}

	tcpSock, tcpErr := d.tcpSocket()
	if tcpErr == nil {
		l, err := net.ListenTCP("tcp", tcpSock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &xserverTCPListener{l}, nil
	}

	return nil, trace.NewAggregate(tcpErr, unixErr)
}

// xserverUnixSocket returns the display's associated unix socket.
func (d *Display) unixSocket() (*net.UnixAddr, error) {
	// For x11 unix domain sockets, the hostname must be "unix" or empty. In these cases
	// we return the actual unix socket for the display "/tmp/.X11-unix/X<display_number>"
	if d.HostName == "unix" || d.HostName == "" {
		sockName := filepath.Join(os.TempDir(), x11SocketDir, fmt.Sprintf("X%d", d.DisplayNumber))
		return net.ResolveUnixAddr("unix", sockName)
	}
	return nil, trace.BadParameter("display is not a unix socket")
}

// xserverTCPSocket returns the display's associated tcp socket.
// e.g. "hostname:<6000+display_number>"
func (d *Display) tcpSocket() (*net.TCPAddr, error) {
	if d.HostName == "" {
		return nil, trace.BadParameter("hostname can't be empty for an XServer tcp socket")
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
//in one of the following formats - hostname:d[.s], unix:d[.s], :d[.s], ::d[.s].
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
