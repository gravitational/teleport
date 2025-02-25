// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//go:build !vnetdaemon
// +build !vnetdaemon

package vnet

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// execAdminProcess is called from the normal user process to execute
// "tsh vnet-admin-setup" as root via an osascript wrapper.
func execAdminProcess(ctx context.Context, config daemon.Config) error {
	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	if homePath := os.Getenv(types.HomeEnvVar); homePath == "" {
		// Explicitly set TELEPORT_HOME if not already set.
		os.Setenv(types.HomeEnvVar, config.HomePath)
	}

	appleScript := fmt.Sprintf(`
set executableName to "%s"
set socketPath to "%s"
set ipv6Prefix to "%s"
set dnsAddr to "%s"
do shell script quoted form of executableName & `+
		`" %s -d --socket " & quoted form of socketPath & `+
		`" --ipv6-prefix " & quoted form of ipv6Prefix & `+
		`" --dns-addr " & quoted form of dnsAddr & `+
		`" --egid %d --euid %d" & `+
		`" >/var/log/vnet.log 2>&1" `+
		`with prompt "Teleport VNet wants to set up a virtual network device." with administrator privileges`,
		executableName, config.SocketPath, config.IPv6Prefix, config.DNSAddr, teleport.VnetAdminSetupSubCommand,
		os.Getegid(), os.Geteuid())

	// The context we pass here has effect only on the password prompt being shown. Once osascript spawns the
	// privileged process, canceling the context (and thus killing osascript) has no effect on the privileged
	// process.
	cmd := exec.CommandContext(ctx, "osascript", "-e", appleScript)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr := stderr.String()

			// When the user closes the prompt for administrator privileges, the -128 error is returned.
			// https://developer.apple.com/library/archive/documentation/AppleScript/Conceptual/AppleScriptLangGuide/reference/ASLR_error_codes.html#//apple_ref/doc/uid/TP40000983-CH220-SW2
			if strings.Contains(stderr, "-128") {
				return trace.Errorf("password prompt closed by user")
			}

			if errors.Is(ctx.Err(), context.Canceled) {
				// osascript exiting due to canceled context.
				return ctx.Err()
			}

			stderrDesc := ""
			if stderr != "" {
				stderrDesc = fmt.Sprintf(", stderr: %s", stderr)
			}
			return trace.Wrap(exitError, "osascript exited%s", stderrDesc)
		}

		return trace.Wrap(err)
	}

	if ctx.Err() == nil {
		// The admin subcommand is expected to run until VNet gets stopped (in other words, until ctx
		// gets canceled).
		//
		// If it exits with no error _before_ ctx is canceled, then it most likely means that the socket
		// was unexpectedly removed. When the socket gets removed, the admin subcommand assumes that the
		// unprivileged process (executing this code here) has quit and thus it should quit as well. But
		// we know that it's not the case, so in this scenario we return an error instead.
		//
		// If we don't return an error here, then other code won't be properly notified about the fact
		// that the admin process has quit.
		return trace.Errorf("admin subcommand exited prematurely with no error (likely because socket was removed)")
	}

	return nil
}
