//go:build !windows

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// init enables HTTP/2 extended CONNECT (RFC 8441) so that browsers can
// open WebSockets over an HTTP/2 connection to the proxy's web listener.
//
// Go's net/http and golang.org/x/net/http2 gate extended CONNECT support
// on the GODEBUG variable http2xconnect=1. The variable is read once at
// stdlib init, before any user init function runs, so setting it from a
// regular init() is too late. The only ways to influence it are to set
// it in the environment of the parent process or to re-exec.
//
// This init prepends http2xconnect=1 to GODEBUG when missing, then
// re-execs the current binary via syscall.Exec. The replacement process
// has the env var in place before net/http's init runs. On success
// syscall.Exec does not return; on failure the original process
// continues without the feature.
//
// Track https://github.com/golang/go/issues/71128 for the public API
// that will replace this shim.
func init() {
	const want = "http2xconnect=1"

	existing := os.Getenv("GODEBUG")
	if godebugContains(existing, want) {
		return
	}

	updated := want
	if existing != "" {
		updated = existing + "," + want
	}
	if err := os.Setenv("GODEBUG", updated); err != nil {
		fmt.Fprintf(os.Stderr,
			"teleport: failed to set GODEBUG=%s: %v\n", want, err)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"teleport: cannot determine executable path for GODEBUG re-exec: %v\n", err)
		return
	}
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr,
			"teleport: GODEBUG re-exec failed: %v\n", err)
	}
}

// godebugContains reports whether the comma-separated GODEBUG value
// already includes the given setting (exact match on a token).
func godebugContains(godebug, setting string) bool {
	for _, tok := range strings.Split(godebug, ",") {
		if strings.TrimSpace(tok) == setting {
			return true
		}
	}
	return false
}
