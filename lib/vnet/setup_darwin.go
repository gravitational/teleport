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

//go:build darwin
// +build darwin

package vnet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
)

// createAndSetupTUNDeviceWithoutRoot creates a virtual network device and configures the host OS to use that
// device for VNet connections. It will spawn a root process to handle the TUN creation and host
// configuration.
//
// After the TUN device is created, it will be sent on the result channel. Any error will be sent on the err
// channel. Always select on both the result channel and the err channel when waiting for a result.
//
// This will keep running until [ctx] is canceled or an unrecoverable error is encountered, in order to keep
// the host OS configuration up to date.
func createAndSetupTUNDeviceWithoutRoot(ctx context.Context, ipv6Prefix, dnsAddr string) (<-chan tun.Device, <-chan error) {
	tunCh := make(chan tun.Device, 1)
	errCh := make(chan error, 1)

	slog.InfoContext(ctx, "Spawning child process as root to create and setup TUN device")
	socket, socketPath, err := createUnixSocket()
	if err != nil {
		errCh <- trace.Wrap(err, "creating unix socket")
		return tunCh, errCh
	}
	slog.DebugContext(ctx, "Created unix socket for admin subcommand", "socket", socketPath)

	socketCtx, cancelSocketCtx := context.WithCancel(ctx)
	// Make sure all goroutines complete before sending an err on the error chan, to be sure they all have a
	// chance to clean up before the process terminates.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Requirements:
		// - must close the socket concurrently with recvTUNNameAndFd if ctx is canceled to unblock
		//   a stuck AcceptUnix (can't defer).
		// - must close the socket exactly once before letting the process terminate.
		<-socketCtx.Done()
		// When the socket gets closed, the admin process that's on the other end notices that and shuts
		// down as well.
		return trace.Wrap(socket.Close())
	})
	g.Go(func() error {
		// If the admin subcommand exits before ctx gets canceled, make sure that the goroutine which
		// closes the socket terminates so that g.Wait() gets unblocked. Without the admin subcommand,
		// there's no one to consume the socket anyway.
		//
		// This can happen when the socket is removed while the unprivileged process is still running.
		// The admin process sees that the socket was removed, so it quits because it assumes that the
		// unprivileged process has exited too.
		defer cancelSocketCtx()

		// Once the user gets through the osascript password dialog, ctx has no control over the admin
		// subcommand anyway – an unprivileged process cannot kill a privileged process. Nevertheless,
		// ctx needs to be passed anyway so that the password dialog gets closed if the context gets
		// canceled while the dialog is shown.
		return trace.Wrap(execAdminSubcommand(ctx, socketPath, ipv6Prefix, dnsAddr))
	})
	g.Go(func() error {
		tunName, tunFd, err := recvTUNNameAndFd(ctx, socket)
		if err != nil {
			return trace.Wrap(err, "receiving TUN name and file descriptor")
		}
		tunDevice, err := tun.CreateTUNFromFile(os.NewFile(tunFd, tunName), 0)
		if err != nil {
			return trace.Wrap(err, "creating TUN device from file descriptor")
		}
		tunCh <- tunDevice
		return nil
	})
	go func() { errCh <- g.Wait() }()

	return tunCh, errCh
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
		`with prompt "VNet wants to set up a virtual network device" with administrator privileges`,
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
				slog.DebugContext(ctx, "osascript exiting due to canceled context", "stderr", stderr)
				return nil
			}

			stderrDesc := ""
			if stderr != "" {
				stderrDesc = fmt.Sprintf(", stderr: %s", stderr)
			}
			return trace.Wrap(exitError, "osascript exited%s", stderrDesc)
		}

		return trace.Wrap(err)
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
func sendTUNNameAndFd(socketPath, tunName string, fd uintptr) error {
	socketAddr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	conn, err := net.DialUnix(socketAddr.Net, nil /*laddr*/, socketAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return trace.Wrap(err)
	}

	// Write the device name as the main message and pass the file desciptor as out-of-band data.
	rights := unix.UnixRights(int(fd))
	if _, _, err := conn.WriteMsgUnix([]byte(tunName), rights, socketAddr); err != nil {
		return trace.Wrap(err, "writing to unix conn")
	}
	return nil
}

// recvTUNNameAndFd receives the name of a TUN device and its open file descriptor over a unix socket, meant
// for passing the TUN from the root process which must create it to the user process.
func recvTUNNameAndFd(ctx context.Context, socket *net.UnixListener) (string, uintptr, error) {
	var conn *net.UnixConn
	errC := make(chan error)
	go func() {
		connection, err := socket.AcceptUnix()
		conn = connection
		errC <- err
	}()

	select {
	case <-ctx.Done():
		return "", 0, trace.Wrap(ctx.Err())
	case err := <-errC:
		if err != nil {
			return "", 0, trace.Wrap(err, "accepting connection on unix socket")
		}
	}

	// Close the connection early to unblock reads if the context is canceled.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

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
