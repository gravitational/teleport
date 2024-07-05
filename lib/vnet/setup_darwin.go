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

package vnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// receiveTUNDevice is a blocking call which waits for the admin subcommand to pass over the socket
// the name and fd of the TUN device.
func receiveTUNDevice(socket *net.UnixListener) (tun.Device, error) {
	tunName, tunFd, err := recvTUNNameAndFd(socket)
	if err != nil {
		return nil, trace.Wrap(err, "receiving TUN name and file descriptor")
	}

	tunDevice, err := tun.CreateTUNFromFile(os.NewFile(tunFd, tunName), 0)
	return tunDevice, trace.Wrap(err, "creating TUN device from file descriptor")
}

func execAdminProcess(ctx context.Context, socketPath, ipv6Prefix, dnsAddr string) error {
	// TODO(ravicious): Remove the feature env var after the daemon gets implemented.
	if os.Getenv("VNET_DAEMON") == "1" {
		// SMAppService that we use to manage the launch daemon works only with signed app bundles.
		isSigned, err := daemon.IsSigned(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if isSigned {
			return trace.Wrap(daemon.RegisterAndCall(ctx, socketPath, ipv6Prefix, dnsAddr))
		}
	}

	return trace.Wrap(execAdminSubcommand(ctx, socketPath, ipv6Prefix, dnsAddr))
}

func execAdminSubcommand(ctx context.Context, socketPath, ipv6Prefix, dnsAddr string) error {
	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	if homePath := os.Getenv(types.HomeEnvVar); homePath == "" {
		// Explicitly set TELEPORT_HOME if not already set.
		os.Setenv(types.HomeEnvVar, profile.FullProfilePath(""))
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
		`" >/var/log/vnet.log 2>&1" `+
		`with prompt "Teleport VNet wants to set up a virtual network device." with administrator privileges`,
		executableName, socketPath, ipv6Prefix, dnsAddr, teleport.VnetAdminSetupSubCommand)

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

func createUnixSocket() (*net.UnixListener, string, error) {
	socketPath := filepath.Join(os.TempDir(), "vnet"+uuid.NewString()+".sock")
	socketAddr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	l, err := net.ListenUnix(socketAddr.Net, socketAddr)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating unix socket")
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return nil, "", trace.Wrap(err, "setting permissions on unix socket")
	}
	return l, socketPath, nil
}

// sendTUNNameAndFd sends the name of the TUN device and its open file descriptor over a unix socket, meant
// for passing the TUN from the root process which must create it to the user process.
func sendTUNNameAndFd(socketPath, tunName string, tunFile *os.File) error {
	socketAddr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	conn, err := net.DialUnix(socketAddr.Net, nil /*laddr*/, socketAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return trace.Wrap(err)
	}

	// Write the device name as the main message and pass the file desciptor as out-of-band data.
	rights := unix.UnixRights(int(tunFile.Fd()))
	_, _, err = conn.WriteMsgUnix([]byte(tunName), rights, socketAddr)

	// Hint to the garbage collector not to call the finalizer on tunFile, which would close the file and
	// invalidate fd, until it has been written to the socket.
	runtime.KeepAlive(tunFile)

	return trace.Wrap(err, "writing to unix conn")
}

// recvTUNNameAndFd receives the name of a TUN device and its open file descriptor over a unix socket, meant
// for passing the TUN from the root process which must create it to the user process.
func recvTUNNameAndFd(socket *net.UnixListener) (string, uintptr, error) {
	conn, err := socket.AcceptUnix()
	if err != nil {
		return "", 0, trace.Wrap(err, "accepting connection on unix socket")
	}
	defer conn.Close()

	msg := make([]byte, 128)
	oob := make([]byte, unix.CmsgSpace(4)) // Fd is 4 bytes
	n, oobn, _, _, err := conn.ReadMsgUnix(msg, oob)
	if err != nil {
		return "", 0, trace.Wrap(err, "reading from unix conn")
	}

	// Parse the device name from the main message.
	if n == 0 {
		return "", 0, trace.Errorf("failed to read msg from unix conn")
	}
	if oobn != len(oob) {
		return "", 0, trace.Errorf("failed to read out-of-band data from unix conn")
	}
	tunName := string(msg[:n])

	// Parse the file descriptor from the out-of-band data.
	scm, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return "", 0, trace.Wrap(err, "parsing socket control message")
	}
	if len(scm) != 1 {
		return "", 0, trace.BadParameter("expect 1 socket control message, got %d", len(scm))
	}
	fds, err := unix.ParseUnixRights(&scm[0])
	if err != nil {
		return "", 0, trace.Wrap(err, "parsing file descriptors")
	}
	if len(fds) != 1 {
		return "", 0, trace.BadParameter("expected 1 file descriptor, got %d", len(fds))
	}
	fd := uintptr(fds[0])

	return tunName, fd, nil
}
