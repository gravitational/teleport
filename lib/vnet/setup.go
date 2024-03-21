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
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"
)

func CreateAndSetupTUNDevice(ctx context.Context) (tun.Device, func(), error) {
	var (
		device  tun.Device
		name    string
		cleanup func()
		err     error
	)
	if os.Getuid() == 0 {
		device, name, cleanup, err = createAndSetupTUNDeviceAsRoot(ctx)
	} else {
		device, name, err = createAndSetupTUNDeviceWithoutRoot(ctx)
		cleanup = func() {}
	}
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	slog.Info("Created TUN device.", "name", name)
	return device, cleanup, nil
}

func createAndSetupTUNDeviceAsRoot(ctx context.Context) (tun.Device, string, func(), error) {
	tun, tunName, err := createTUNDevice()
	if err != nil {
		return nil, "", nil, trace.Wrap(err)
	}

	if err := setupHostIPRoutes(ctx, tunName); err != nil {
		return nil, "", nil, trace.Wrap(err, "setting up host IP routes")
	}

	cleanup, err := setupHostDNS(ctx)
	if err != nil {
		return nil, "", nil, trace.Wrap(err, "setting up host DNS configuration")
	}
	return tun, tunName, cleanup, nil
}

func createAndSetupTUNDeviceWithoutRoot(ctx context.Context) (tun.Device, string, error) {
	slog.Info("Spawning child process as root to create and setup TUN device")
	socket, socketPath, err := createUnixSocket()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	go func() {
		if err := runAdminSubcommand(ctx, socketPath); err != nil {
			slog.Error("Error running admin subcommand.", "error", err)
		}
	}()

	tunName, tunFd, err := recvTUNNameAndFd(ctx, socket)
	if err != nil {
		return nil, "", trace.Wrap(err, "receiving TUN name and file descriptor")
	}

	tunDevice, err := tun.CreateTUNFromFile(os.NewFile(tunFd, ""), 0)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating TUN device from file descriptor")
	}
	return tunDevice, tunName, nil
}

func createUnixSocket() (*net.UnixListener, string, error) {
	// Abuse CreateTemp to find an unused path.
	f, err := os.CreateTemp("", "vnet*.sock")
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	socketPath := f.Name()
	if err := f.Close(); err != nil {
		return nil, "", trace.Wrap(err)
	}
	if err := os.Remove(socketPath); err != nil {
		return nil, "", trace.Wrap(err)
	}
	socketAddr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	l, err := net.ListenUnix(socketAddr.Net, socketAddr)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating unix socket")
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		return nil, "", trace.Wrap(err, "setting permissions on unix socket")
	}
	return l, socketPath, nil
}

func sendTUNNameAndFd(socketPath, tunName string, fd uintptr) error {
	socketAddr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	conn, err := net.DialUnix(socketAddr.Net, nil /*laddr*/, socketAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = conn.SetDeadline(time.Now().Add(time.Minute))
	if err != nil {
		return trace.Wrap(err)
	}

	rights := unix.UnixRights(int(fd))
	if _, _, err := conn.WriteMsgUnix([]byte(tunName), rights, socketAddr); err != nil {
		return trace.Wrap(err, "writing to unix conn")
	}
	return nil
}

func recvTUNNameAndFd(ctx context.Context, socket *net.UnixListener) (string, uintptr, error) {
	err := socket.SetDeadline(time.Now().Add(time.Minute))
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
		<-ctx.Done()
		conn.Close()
	}()

	msg := make([]byte, 32)
	oob := make([]byte, unix.CmsgSpace(4)) // Fd is 4 bytes
	n, oobn, _, _, err := conn.ReadMsgUnix(msg, oob)
	if err != nil {
		return "", 0, trace.Wrap(err, "reading from unix conn")
	}
	if n == 0 {
		return "", 0, trace.Errorf("failed to read msg from unix conn")
	}
	if oobn != len(oob) {
		return "", 0, trace.Errorf("failed to read out-of-band data from unix conn")
	}
	tunName := string(msg[:n])

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

func runAdminSubcommand(ctx context.Context, socketPath string) error {
	pid := os.Getpid()
	pidFile, err := os.CreateTemp("", "vnet*.pid")
	if err != nil {
		return trace.Wrap(err, "creating PID file")
	}
	if _, err := pidFile.Write([]byte(strconv.Itoa(pid))); err != nil {
		pidFile.Close()
		return trace.Wrap(err, "writing to PID file")
	}
	if err := pidFile.Close(); err != nil {
		return trace.Wrap(err, "closing PID file")
	}

	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	prompt := "VNet wants to set up a virtual network device."
	appleScript := fmt.Sprintf(`
set executableName to "%s"
set socketPath to "%s"
set pidFile to "%s"
do shell script quoted form of executableName & " %s --socket " & quoted form of socketPath & " --pidfile " & quoted form of pidFile with prompt "%s" with administrator privileges`,
		executableName, socketPath, pidFile.Name(), teleport.VnetAdminSetupSubCommand, prompt)
	cmd := exec.CommandContext(ctx, "osascript", "-e", appleScript)
	stderr := new(strings.Builder)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			stderr := stderr.String()

			// When the user closes the prompt for administrator privileges, the -128 error is returned.
			// https://developer.apple.com/library/archive/documentation/AppleScript/Conceptual/AppleScriptLangGuide/reference/ASLR_error_codes.html#//apple_ref/doc/uid/TP40000983-CH220-SW2
			if strings.Contains(stderr, "-128") {
				return trace.Errorf("admin setup canceled by user")
			}

			return trace.Wrap(err, fmt.Sprintf("admin setup subcommand exited, stderr: %s", stderr))
		}
		return trace.Wrap(err, "running admin setup subcommand")
	}
	return nil
}

// AdminSubcommand is the tsh subcommand that should run as root that will
// create and setup a TUN device and pass the file descriptor for that device
// over the unix socket found at socketPath.
func AdminSubcommand(ctx context.Context, socketPath, pidFilePath string) error {
	ctx, err := withPidfileCancellation(ctx, pidFilePath)
	if err != nil {
		return trace.Wrap(err)
	}
	tun, tunName, cleanup, err := createAndSetupTUNDeviceAsRoot(ctx)
	if err != nil {
		return trace.Wrap(err, "doing admin setup")
	}
	defer cleanup()
	if err := sendTUNNameAndFd(socketPath, tunName, tun.File().Fd()); err != nil {
		return trace.Wrap(err)
	}
	// Defer cleanup until context is cancelled.
	<-ctx.Done()
	return trace.Wrap(ctx.Err())
}

func createTUNDevice() (tun.Device, string, error) {
	slog.Debug("Creating TUN device.")
	dev, err := tun.CreateTUN("utun", mtu)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating TUN device")
	}
	name, err := dev.Name()
	if err != nil {
		return nil, "", trace.Wrap(err, "getting TUN device name")
	}
	return dev, name, nil
}

func setupHostIPRoutes(ctx context.Context, tunName string) error {
	const (
		ip   = "100.64.0.1"
		mask = "100.64.0.0/10"
	)
	fmt.Println("Setting IP address for the TUN device:")
	cmd := exec.CommandContext(ctx, "ifconfig", tunName, ip, ip, "up")
	fmt.Println("\t", cmd.Path, strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "running ifconfig")
	}

	fmt.Println("Setting an IP route for the VNet:")
	cmd = exec.CommandContext(ctx, "route", "add", "-net", mask, "-interface", tunName)
	fmt.Println("\t", cmd.Path, strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "running route add")
	}
	return nil
}

func setupHostDNS(ctx context.Context) (func(), error) {
	// TODO: support multiple active profiles, custom DNS zones, and react to
	// changes.
	profileDir := profile.FullProfilePath(os.Getenv("TELEPORT_HOME"))
	currentProfile, err := profile.GetCurrentProfileName(profileDir)
	if err != nil {
		return nil, trace.Wrap(err, "getting current profile")
	}
	proxyAddress := currentProfile
	fileName := "/etc/resolver/" + proxyAddress
	contents := "nameserver " + defaultDNSAddress.String()
	if err := os.WriteFile(fileName, []byte(contents), 0644); err != nil {
		return nil, trace.Wrap(err, "writing DNS configuration file %s", fileName)
	}
	return func() {
		if err := os.Remove(fileName); err != nil {
			slog.Error("Failed to remove DNS configuration file.", "filename", fileName, "error", err)
		}
	}, nil

}
