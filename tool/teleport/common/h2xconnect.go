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
// The shim only fires for "teleport start" because that is the only
// subcommand that runs the proxy web listener. Every other invocation
// skips the re-exec so short-lived CLI subcommands do not pay an
// extra execve(2).
//
// An operator-supplied http2xconnect= value (including http2xconnect=0
// for incident rollback) is left intact. The shim only appends the
// default when no http2xconnect= entry is already present in GODEBUG.
//
// On a re-exec, syscall.Exec replaces the process with the same
// executable and arguments plus the updated GODEBUG. The replacement
// has the env var in place before net/http's init runs. On success
// syscall.Exec does not return; on failure the original process
// continues without the feature.
//
// Track https://github.com/golang/go/issues/71128 for the public API
// that will replace this shim.
func init() {
	const settingKey = "http2xconnect"
	const defaultValue = settingKey + "=1"

	if !needsHTTP2XConnect(os.Args) {
		return
	}

	existing := os.Getenv("GODEBUG")
	if godebugHasKey(existing, settingKey) {
		// Operator already set http2xconnect explicitly (=0 to disable,
		// =1 to confirm, or any other value). Honor it; no re-exec.
		return
	}

	updated := defaultValue
	if existing != "" {
		updated = existing + "," + defaultValue
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"teleport: cannot determine executable path for GODEBUG re-exec: %v\n", err)
		return
	}
	// Build a fresh env slice for the replacement process. Mutating the
	// current process's GODEBUG via os.Setenv would leak the change to
	// any child the original process spawns if the exec fails.
	env := append(os.Environ(), "GODEBUG="+updated)
	// Keep argv[0] consistent with the resolved executable path so the
	// replacement process's os.Args[0] and os.Executable() agree.
	argv := append([]string{exe}, os.Args[1:]...)
	if err := syscall.Exec(exe, argv, env); err != nil {
		fmt.Fprintf(os.Stderr,
			"teleport: GODEBUG re-exec failed: %v\n", err)
	}
}

// needsHTTP2XConnect reports whether the invocation will run the proxy
// web listener and therefore needs http2xconnect=1 in GODEBUG. Only
// "teleport start" runs the proxy. "teleport app start" and
// "teleport db start" start agents, not the proxy, so they fail the
// check on the first non-flag token.
//
// Leading top-level flags are skipped because kingpin accepts flags
// before the subcommand (e.g. "teleport --debug start ...").
func needsHTTP2XConnect(args []string) bool {
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "start"
	}
	return false
}

// godebugHasKey reports whether the comma-separated GODEBUG value
// already contains an entry with the given key, regardless of value.
// Honoring any explicit operator setting lets http2xconnect=0 disable
// the feature during incident rollback.
func godebugHasKey(godebug, key string) bool {
	for _, tok := range strings.Split(godebug, ",") {
		name, _, _ := strings.Cut(strings.TrimSpace(tok), "=")
		if name == key {
			return true
		}
	}
	return false
}
