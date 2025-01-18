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
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/tun"
)

type windowsAdminProcessConfig struct {
	clientApplicationServiceAddr string
}

// runWindowsAdminProcess must run as administrator. It creates and sets up a TUN
// device, runs the VNet networking stack, and handles OS configuration. It will
// continue to run until [ctx] is canceled or encountering an unrecoverable
// error.
func runWindowsAdminProcess(ctx context.Context, cfg *windowsAdminProcessConfig) error {
	pm, ctx := newProcessManager()
	log.InfoContext(ctx, "Running VNet admin process")

	device, err := tun.CreateTUN("TeleportVNet", mtu)
	if err != nil {
		return trace.Wrap(err, "creating TUN device")
	}
	defer device.Close()
	tunName, err := device.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN device name")
	}
	log.InfoContext(ctx, "Created TUN interface", "tun", tunName)

	clt, err := newClientApplicationServiceClient(ctx, cfg.clientApplicationServiceAddr)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.Close()

	if err := authenticateUserProcess(ctx, clt); err != nil {
		log.ErrorContext(ctx, "Failed to authenticate user process", "error", err)
		return trace.Wrap(err, "authenticating user process")
	}

	networkStackConfig, err := newWindowsNetworkStackConfig(device, clt)
	if err != nil {
		return trace.Wrap(err, "creating network stack config")
	}
	networkStack, err := newNetworkStack(networkStackConfig)
	if err != nil {
		return trace.Wrap(err, "creating network stack")
	}

	pm.AddCriticalBackgroundTask("network stack", func() error {
		return trace.Wrap(networkStack.run(ctx), "running network stack")
	})
	pm.AddCriticalBackgroundTask("user process ping", func() error {
		for {
			select {
			case <-time.After(time.Second):
				if err := clt.Ping(ctx); err != nil {
					return trace.Wrap(err, "pinging user process")
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	// TODO(nklaassen): run OS configuration loop.
	return trace.Wrap(pm.Wait())
}

func newWindowsNetworkStackConfig(tun tunDevice, clt *clientApplicationServiceClient) (*networkStackConfig, error) {
	appProvider := newRemoteAppProvider(clt)
	appResolver := newTCPAppResolver(appProvider, clockwork.NewRealClock())
	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err, "creating new IPv6 prefix")
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})
	return &networkStackConfig{
		tunDevice:          tun,
		ipv6Prefix:         ipv6Prefix,
		dnsIPv6:            dnsIPv6,
		tcpHandlerResolver: appResolver,
	}, nil
}

func authenticateUserProcess(ctx context.Context, clt *clientApplicationServiceClient) error {
	pipe, err := createNamedPipe(ctx)
	if err != nil {
		return trace.Wrap(err, "creating named pipe")
	}
	defer pipe.Close()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := clt.AuthenticateProcess(ctx, pipe.name); err != nil {
			return trace.Wrap(err, "authenticating user process")
		}
		return nil
	})
	g.Go(func() error {
		if err := pipe.validateClientExe(ctx); err != nil {
			return trace.Wrap(err, "validating user process exe")
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type winpipe struct {
	pipeHandle  windows.Handle
	eventHandle windows.Handle
	name        string
}

func createNamedPipe(ctx context.Context) (*winpipe, error) {
	pipeName := `\\.\pipe\` + uuid.NewString()
	pipePath, err := syscall.UTF16PtrFromString(pipeName)
	if err != nil {
		return nil, trace.Wrap(err, "converting string to UTF16")
	}
	// This allows pipe access to everyone
	// TODO(nklaassen): restrict access to only the calling user.
	sddl := "D:P(A;;GA;;;WD)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, trace.Wrap(err, "creating security descriptor from string")
	}
	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}
	pipeHandle, err := windows.CreateNamedPipe(
		pipePath,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
		windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		1024,
		1024,
		0,
		&sa,
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating named pipe")
	}
	log.DebugContext(ctx, "Created named pipe", "name", pipeName)
	eventHandle, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, trace.Wrap(err, "creating Windows event handle")
	}
	overlapped := &windows.Overlapped{HEvent: eventHandle}
	if err := windows.ConnectNamedPipe(pipeHandle, overlapped); err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil, trace.Wrap(err, "connecting to named pipe")
	}
	return &winpipe{
		pipeHandle:  pipeHandle,
		eventHandle: overlapped.HEvent,
		name:        pipeName,
	}, nil
}

func (p *winpipe) validateClientExe(ctx context.Context) error {
	if err := p.waitForClient(ctx); err != nil {
		return trace.Wrap(err, "waiting for client to connect to named pipe")
	}
	clientExePath, err := p.clientExePath(ctx)
	if err != nil {
		return trace.Wrap(err, "getting pipe client exe path")
	}
	log.DebugContext(ctx, "Got pipe connection from client", "exe", clientExePath)
	thisExePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path for this service")
	}
	if thisExePath != clientExePath {
		return trace.AccessDenied("remote process is not running the same executable as this service, remote process exe: %s, this process exe: %s",
			clientExePath, thisExePath)
	}
	// TODO(nklaassen): validate exe is signed, or consider if this is
	// unnecessary as long as the two exes are identical.
	return nil
}

func (p *winpipe) waitForClient(ctx context.Context) error {
	evt, err := windows.WaitForSingleObject(p.eventHandle, 500 /*milliseconds*/)
	if err != nil {
		return trace.Wrap(err, "waiting for connection on named pipe")
	}
	if evt != windows.WAIT_OBJECT_0 {
		return trace.Errorf("failed to wait for connection on named pipe, error code: %d", evt)
	}
	return nil
}

func (p *winpipe) clientExePath(ctx context.Context) (string, error) {
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(p.pipeHandle, &pid); err != nil {
		return "", trace.Wrap(err, "getting named pipe client process ID")
	}
	processHandle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", trace.Wrap(err, "opening client process")
	}
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(processHandle, 0, &buf[0], &size); err != nil {
		return "", trace.Wrap(err, "querying pipe client process image name")
	}
	return windows.UTF16PtrToString(&buf[0]), nil
}

func (p *winpipe) Close() error {
	return trace.NewAggregate(
		trace.Wrap(windows.CloseHandle(p.pipeHandle), "closing pipe handle"),
		trace.Wrap(windows.CloseHandle(p.eventHandle), "closing pipe event handle"),
	)
}

// connectToPipe connects to a Windows named pipe, then immediately closes the
// connection. This is used for process authentication.
func connectToPipe(pipePath string) error {
	pipePathPtr, err := syscall.UTF16PtrFromString(pipePath)
	if err != nil {
		return trace.Wrap(err, "converting string to UTF16")
	}
	handle, err := windows.CreateFile(
		pipePathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,   // ShareMode
		nil, // SecurityAttributes
		windows.OPEN_EXISTING,
		0, // FlagsAndAttributes
		0, // TemplateFile
	)
	if err != nil {
		return trace.Wrap(err, "opening named pipe")
	}
	if err := windows.CloseHandle(handle); err != nil {
		return trace.Wrap(err, "closing named pipe")
	}
	return nil
}
