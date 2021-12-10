package x11

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// displayFromEnv retrieves the set $DISPLAY form env
func displayFromEnv() (string, error) {
	display := os.Getenv(DisplayEnv)
	if display == "" {
		return "", trace.BadParameter("$DISPLAY is not set")
	}
	return display, nil
}

// parsesDisplay parses the given display value and returns the host,
// display number, and screen number, or a parsing error. display
// should be in the format "hostname:displayNumber.screenNumber".
func parseDisplay(display string) (string, int, int, error) {
	splitHost := strings.Split(display, ":")
	host := splitHost[0]
	if len(splitHost) < 2 {
		return host, 0, 0, nil
	}

	splitDisplayNumber := strings.Split(splitHost[1], ".")
	displayNumber, err := strconv.Atoi(splitDisplayNumber[0])
	if err != nil {
		return "", 0, 0, trace.Wrap(err)
	}
	if len(splitDisplayNumber) < 2 {
		return host, displayNumber, 0, nil
	}

	screenNumber, err := strconv.Atoi(splitDisplayNumber[1])
	if err != nil {
		return "", 0, 0, trace.Wrap(err)
	}

	return host, displayNumber, screenNumber, nil
}

// dialDisplay dials the set $DISPLAY via unix socket for local
// displays or tcp for remote displays.
func dialDisplay() (net.Conn, error) {
	display, err := displayFromEnv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostname, displayNumber, _, err := parseDisplay(display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// display is a unix socket, dial the default x11 unix socket for the display number
	if hostname == "unix" || hostname == "" {
		sock := filepath.Join(os.TempDir(), x11UnixSocket, fmt.Sprintf("X%d", displayNumber))
		return net.Dial("unix", sock)
	}

	// dial generic display
	return net.Dial("tcp", display)
}
