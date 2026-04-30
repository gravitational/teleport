package common

import (
	"os"
	"strings"
	"syscall"
)

func init() {
	// Enable WebSocket over HTTP/2 (RFC 8441) for app and database access features.
	//
	// Go's net/http h2_bundle.go reads GODEBUG directly via os.Getenv during its own
	// init() to set http2disableExtendedConnectProtocol. Since stdlib init() always
	// runs before user package init(), os.Setenv cannot influence it from here.
	//
	// go:linkname to net/http.http2disableExtendedConnectProtocol is also blocked:
	// Go 1.23+ requires the destination symbol to carry a //go:linkname opt-in comment,
	// which h2_bundle.go does not have.
	//
	// Instead, if GODEBUG does not already contain http2xconnect=1, we add it and
	// re-exec the current process via syscall.Exec. The replacement process has the
	// env var in place before net/http's init() runs, so h2_bundle.go sets
	// http2disableExtendedConnectProtocol=false and the h2 server advertises
	// SETTINGS_ENABLE_CONNECT_PROTOCOL=1 to HTTP/2 clients (RFC 8441).
	//
	// Revert once the Go team exposes an official API:
	//   - https://github.com/golang/go/issues/53208
	//   - https://github.com/golang/go/issues/72071
	if strings.Contains(os.Getenv("GODEBUG"), "http2xconnect=1") {
		return
	}

	existing := os.Getenv("GODEBUG")
	updated := "http2xconnect=1"
	if existing != "" {
		updated = existing + "," + updated
	}
	if err := os.Setenv("GODEBUG", updated); err != nil {
		panic("teleport: failed to set GODEBUG for http2xconnect: " + err.Error())
	}

	exe, err := os.Executable()
	if err != nil {
		panic("teleport: rexec: " + err.Error())
	}
	// syscall.Exec replaces the current process image; does not return on success.
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		panic("teleport: rexec: " + err.Error())
	}
}
