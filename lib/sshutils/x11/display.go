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

// parseDisplayNumber parses the displayNumber from the given display value
// which should be in the format - "hostname:displayNumber.screenNumber"
func parseDisplayNumber(display string) (int, error) {
	colonIdx := strings.LastIndex(display, ":")
	if colonIdx == -1 {
		return 0, nil
	}

	dotIdx := strings.LastIndex(display, ".")
	if dotIdx == -1 {
		dotIdx = len(display)
	}

	displayNumber, err := strconv.Atoi(display[colonIdx+1 : dotIdx])
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return displayNumber, nil
}

// parseDisplayHostname parses the hostname from the given display value
// which should be in the format - "hostname:displayNumber.screenNumber"
func parseDisplayHostname(display string) string {
	colonIdx := strings.LastIndex(display, ":")
	if colonIdx == -1 {
		return display
	}

	return display[:colonIdx]
}

// parseDisplayScreenNumber parses the screenNumber from the given display value
// which should be in the format - "hostname:displayNumber.screenNumber"
func parseDisplayScreenNumber(display string) (int, error) {
	dotIdx := strings.LastIndex(display, ".")
	if dotIdx == -1 {
		return 0, nil
	}

	screenNumber, err := strconv.Atoi(display[dotIdx+1:])
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return screenNumber, nil
}

// dialDisplay dials the set $DISPLAY via unix socket for local
// displays or tcp for remote displays.
func dialDisplay() (net.Conn, error) {
	display, err := displayFromEnv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// display is a unix socket, dial the default x11 unix socket for the display number
	if host := parseDisplayHostname(display); host == "unix" || host == "" {
		displayNumber, err := parseDisplayNumber(display)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sock := filepath.Join(os.TempDir(), x11UnixSocket, fmt.Sprintf("X%d", displayNumber))
		return net.Dial("unix", sock)
	}

	// dial generic display
	return net.Dial("tcp", display)
}
