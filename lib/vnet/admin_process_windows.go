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
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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

const (
	tunInterfaceName = "TeleportVNet"
)

type windowsAdminProcessConfig struct {
	// clientApplicationServiceAddr is the local TCP address of the client
	// application gRPC service.
	clientApplicationServiceAddr string
	// serviceCredentialPath is the path where credentials for IPC with the
	// client application are found.
	serviceCredentialPath string
	// userSID is the SID of the user running the client application.
	userSID string
}

func (c *windowsAdminProcessConfig) check() error {
	if c.clientApplicationServiceAddr == "" {
		return trace.BadParameter("clientApplicationServiceAddr is required")
	}
	if c.serviceCredentialPath == "" {
		return trace.BadParameter("serviceCredentialPath is required")
	}
	if c.userSID == "" {
		return trace.BadParameter("userSID is required")
	}
	return nil
}

// runWindowsAdminProcess must run as administrator. It creates and sets up a TUN
// device, runs the VNet networking stack, and handles OS configuration. It will
// continue to run until [ctx] is canceled or encountering an unrecoverable
// error.
func runWindowsAdminProcess(ctx context.Context, cfg *windowsAdminProcessConfig) error {
	log.InfoContext(ctx, "Running VNet admin process")
	if err := cfg.check(); err != nil {
		return trace.Wrap(err)
	}

	serviceCreds, err := readCredentials(cfg.serviceCredentialPath)
	if err != nil {
		return trace.Wrap(err, "reading service IPC credentials")
	}
	clt, err := newClientApplicationServiceClient(ctx, serviceCreds, cfg.clientApplicationServiceAddr)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.close()

	if err := authenticateUserProcess(ctx, clt, cfg.userSID); err != nil {
		return trace.Wrap(err, "authenticating user process")
	}

	device, err := tun.CreateTUN(tunInterfaceName, mtu)
	if err != nil {
		return trace.Wrap(err, "creating TUN device")
	}
	defer device.Close()
	tunName, err := device.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN device name")
	}
	log.InfoContext(ctx, "Created TUN interface", "tun", tunName)

	networkStackConfig, err := newWindowsNetworkStackConfig(device, clt)
	if err != nil {
		return trace.Wrap(err, "creating network stack config")
	}
	networkStack, err := newNetworkStack(networkStackConfig)
	if err != nil {
		return trace.Wrap(err, "creating network stack")
	}

	osConfigProvider, err := newRemoteOSConfigProvider(
		clt,
		tunName,
		networkStackConfig.ipv6Prefix.String(),
		networkStackConfig.dnsIPv6.String(),
	)
	if err != nil {
		return trace.Wrap(err, "creating OS config provider")
	}
	osConfigurator := newOSConfigurator(osConfigProvider)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := networkStack.run(ctx); err != nil {
			return trace.Wrap(err, "running network stack")
		}
		return errors.New("network stack terminated")
	})
	g.Go(func() error {
		if err := osConfigurator.runOSConfigurationLoop(ctx); err != nil {
			return trace.Wrap(err, "running OS configuration loop")
		}
		return errors.New("OS configuration loop terminated")
	})
	g.Go(func() error {
		tick := time.Tick(time.Second)
		for {
			select {
			case <-tick:
				if err := clt.Ping(ctx); err != nil {
					return trace.Wrap(err, "failed to ping client application, it may have exited, shutting down")
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	return trace.Wrap(g.Wait(), "running VNet admin process")
}

func newWindowsNetworkStackConfig(tun tunDevice, clt *clientApplicationServiceClient) (*networkStackConfig, error) {
	appProvider := newRemoteAppProvider(clt)
	appResolver := newTCPAppResolver(appProvider, clockwork.NewRealClock())
	ipv6Prefix, err := newIPv6Prefix()
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

func authenticateUserProcess(ctx context.Context, clt *clientApplicationServiceClient, userSID string) error {
	pipe, err := createNamedPipe(ctx, userSID)
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
		if err := validateClientExe(ctx, pipe); err != nil {
			return trace.Wrap(err, "validating user process exe")
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateClientExe(ctx context.Context, p *winpipe) error {
	thisExePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path for this service")
	}
	clientExePath, err := p.waitForClient(ctx)
	if err != nil {
		return trace.Wrap(err, "waiting for client to connect to named pipe")
	}
	log.DebugContext(ctx, "Got pipe connection from client", "exe", clientExePath)
	if err := compareFiles(thisExePath, clientExePath); err != nil {
		return trace.AccessDenied(
			"remote process is not running the same executable as this service, remote process exe: %s, this process exe: %s",
			clientExePath, thisExePath)
	}
	return nil
}

func compareFiles(p1, p2 string) error {
	h1, err := hashFile(p1)
	if err != nil {
		return trace.Wrap(err)
	}
	h2, err := hashFile(p2)
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal(h1, h2) {
		return trace.CompareFailed("files %s and %s are not equal", p1, p2)
	}
	return nil
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err, "opening %s", path)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, trace.Wrap(err, "hashing %s", path)
	}
	return h.Sum(nil), nil
}

type winpipe struct {
	pipeHandle  windows.Handle
	eventHandle windows.Handle
	name        string
}

// createNamedPipe creates a new Windows named pipe, connects to it as a server
// and sets up the pipe to receive client connections.
func createNamedPipe(ctx context.Context, userSID string) (*winpipe, error) {
	pipeName := `\\.\pipe\` + uuid.NewString()
	pipePath, err := syscall.UTF16PtrFromString(pipeName)
	if err != nil {
		return nil, trace.Wrap(err, "converting string to UTF16")
	}
	// https://learn.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-definition-language
	// This discretionary ACL grants GA (Generic All) pipe access to the user.
	sdString := fmt.Sprintf("D:(A;;GA;;;%s)", userSID)
	sd, err := windows.SecurityDescriptorFromString(sdString)
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
		1,    // maxInstances
		1024, // outSize
		1024, // inSize
		0,    // defaultTimeOut
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

// waitForClient waits for a client to connect to the pipe, and returns the exe
// path of the client process.
func (p *winpipe) waitForClient(ctx context.Context) (string, error) {
	evt, err := windows.WaitForSingleObject(p.eventHandle, 500 /*milliseconds*/)
	if err != nil {
		return "", trace.Wrap(err, "waiting for connection on named pipe")
	}
	if evt != windows.WAIT_OBJECT_0 {
		return "", trace.Errorf("failed to wait for connection on named pipe, error code: %d", evt)
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(p.pipeHandle, &pid); err != nil {
		return "", trace.Wrap(err, "getting named pipe client process ID")
	}
	processHandle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false, // inheritHandle
		pid,
	)
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
