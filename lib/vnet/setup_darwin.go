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
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport"
)

const (
	tunHandoverTimeout = time.Minute
)

func createAndSetupTUNDeviceWithoutRoot(ctx context.Context, ipv6Prefix string) (tun.Device, string, error) {
	slog.InfoContext(ctx, "Spawning child process as root to create and setup TUN device")
	socket, socketPath, err := createUnixSocket()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	adminCommandErr := make(chan error, 1)
	go func() {
		adminCommandErr <- trace.Wrap(execAdminSubcommand(ctx, socketPath, ipv6Prefix))
	}()

	recvTunErr := make(chan error, 1)
	var tunName string
	var tunFd uintptr
	go func() {
		tunName, tunFd, err = recvTUNNameAndFd(ctx, socket)
		recvTunErr <- trace.Wrap(err, "receiving TUN name and file descriptor")
	}()

loop:
	for {
		select {
		case err := <-adminCommandErr:
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
		case err := <-recvTunErr:
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			break loop
		}
	}

	tunDevice, err := tun.CreateTUNFromFile(os.NewFile(tunFd, ""), 0)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating TUN device from file descriptor")
	}

	return tunDevice, tunName, nil
}

func execAdminSubcommand(ctx context.Context, socketPath, ipv6Prefix string) error {
	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	appleScript := fmt.Sprintf(`
set executableName to "%s"
set socketPath to "%s"
set ipv6Prefix to "%s"
do shell script quoted form of executableName & `+
		`" %s --socket " & quoted form of socketPath & `+
		`" --ipv6-prefix " & quoted form of ipv6Prefix `+
		`with prompt "VNet wants to set up a virtual network device" with administrator privileges`,
		executableName, socketPath, ipv6Prefix, teleport.VnetAdminSetupSubCommand)

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
			return trace.Wrap(exitError, "admin subcommand exited, stderr: %s", stderr)
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
	defer conn.Close()

	err = conn.SetDeadline(time.Now().Add(tunHandoverTimeout))
	if err != nil {
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
	ctx, cancel := context.WithTimeout(ctx, tunHandoverTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	err := socket.SetDeadline(deadline)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}
	go func() {
		<-ctx.Done()
		socket.Close()
	}()

	conn, err := socket.AcceptUnix()
	if err != nil {
		return "", 0, trace.Wrap(err)
	}
	go func() {
		// Close the connection early to unblock reads if the context is canceled.
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

func configureOS(ctx context.Context, cfg *osConfig) error {
	if cfg.tunIPv6 != "" && cfg.tunName != "" {
		slog.InfoContext(ctx, "Setting IPv6 address for the TUN device.", "device", cfg.tunName, "address", cfg.tunIPv6)
		cmd := exec.CommandContext(ctx, "ifconfig", cfg.tunName, "inet6", cfg.tunIPv6, "prefixlen", "64")
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "running %v", cmd.Args)
		}

		slog.InfoContext(ctx, "Setting an IPv6 route for the VNet.")
		cmd = exec.CommandContext(ctx, "route", "add", "-inet6", cfg.tunIPv6, "-prefixlen", "64", "-interface", cfg.tunName)
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "running %v", cmd.Args)
		}
	}
	return nil
}
