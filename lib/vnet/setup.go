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
	"os"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/tun"
)

// SetupAndRun creates a network stack for VNet and runs it in the background. To do this, it also
// needs to launch an admin subcommand in the background. It returns [ProcessManager] which controls
// the lifecycle of both background tasks.
//
// The caller is expected to call Close on the process manager to close the network stack, clean
// up any resources used by it and terminate the admin subcommand.
//
// ctx is used to wait for setup steps that happen before SetupAndRun hands out the control to the
// process manager. If ctx gets canceled during SetupAndRun, the process manager gets closed along
// with its background tasks.
func SetupAndRun(ctx context.Context, appProvider AppProvider) (*ProcessManager, error) {
	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})

	pm, processCtx := newProcessManager()
	success := false
	defer func() {
		if !success {
			// Closes the socket and background tasks.
			pm.Close()
		}
	}()

	// Create the socket that's used to receive the TUN device from the admin subcommand.
	socket, socketPath, err := createUnixSocket()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Created unix socket for admin subcommand", "socket", socketPath)
	pm.AddCriticalBackgroundTask("socket closer", func() error {
		// Keep the socket open until the process context is canceled.
		// Closing the socket signals the admin subcommand to terminate.
		<-processCtx.Done()
		return trace.NewAggregate(processCtx.Err(), socket.Close())
	})

	pm.AddCriticalBackgroundTask("admin subcommand", func() error {
		return trace.Wrap(execAdminSubcommand(processCtx, socketPath, ipv6Prefix.String(), dnsIPv6.String()))
	})

	recvTUNErr := make(chan error, 1)
	var tun tun.Device
	go func() {
		// Unblocks after receiving a TUN device or when the context gets canceled (and thus socket gets
		// closed).
		tunDevice, err := receiveTUNDevice(socket)
		tun = tunDevice
		recvTUNErr <- err
	}()

	select {
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	case <-processCtx.Done():
		return nil, trace.Wrap(context.Cause(processCtx))
	case err := <-recvTUNErr:
		if err != nil {
			if processCtx.Err() != nil {
				// Both errors being present means that VNet failed to receive a TUN device because of a
				// problem with the admin subcommand.
				// Returning error from processCtx will be more informative to the user, e.g., the error
				// will say "password prompt closed by user" instead of "read from closed socket".
				slog.DebugContext(ctx, "Error from recvTUNErr ignored in favor of processCtx.Err", "error", err)
				return nil, trace.Wrap(context.Cause(processCtx))
			}
			return nil, trace.Wrap(err, "receiving TUN from admin subcommand")
		}
	}

	appResolver, err := NewTCPAppResolver(appProvider)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ns, err := newNetworkStack(&Config{
		TUNDevice:          tun,
		IPv6Prefix:         ipv6Prefix,
		DNSIPv6:            dnsIPv6,
		TCPHandlerResolver: appResolver,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pm.AddCriticalBackgroundTask("network stack", func() error {
		return trace.Wrap(ns.Run(processCtx))
	})

	success = true
	return pm, nil
}

func newProcessManager() (*ProcessManager, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	return &ProcessManager{
		g:      g,
		cancel: cancel,
	}, ctx
}

// ProcessManager handles background tasks needed to run VNet.
// Its semantics are similar to an error group with a context, but it cancels the context whenever
// any task returns prematurely, that is, a task exits while the context was not canceled.
type ProcessManager struct {
	g      *errgroup.Group
	cancel context.CancelFunc
}

// AddCriticalBackgroundTask adds a function to the error group. [task] is expected to block until
// the context returned by [newProcessManager] gets canceled. The context gets canceled either by
// calling Close on [ProcessManager] or if any task returns.
func (pm *ProcessManager) AddCriticalBackgroundTask(name string, task func() error) {
	pm.g.Go(func() error {
		err := task()
		if err == nil {
			// Make sure to always return an error so that the errgroup context is canceled.
			err = fmt.Errorf("critical task %q exited prematurely", name)
		}
		return trace.Wrap(err)
	})
}

// Wait blocks and waits for the background tasks to finish, which typically happens when another
// goroutine calls Close on the process manager.
func (pm *ProcessManager) Wait() error {
	return trace.Wrap(pm.g.Wait())
}

// Close stops any active background tasks by canceling the underlying context.
func (pm *ProcessManager) Close() {
	pm.cancel()
}

// AdminSubcommand is the tsh subcommand that should run as root that will create and setup a TUN device and
// pass the file descriptor for that device over the unix socket found at socketPath.
//
// It also handles host OS configuration that must run as root, and stays alive to keep the host configuration
// up to date. It will stay running until the socket at [socketPath] is deleting or encountering an
// unrecoverable error.
func AdminSubcommand(ctx context.Context, socketPath, ipv6Prefix, dnsAddr string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tunCh, errCh := createAndSetupTUNDeviceAsRoot(ctx, ipv6Prefix, dnsAddr)
	var tun tun.Device
	select {
	case tun = <-tunCh:
	case err := <-errCh:
		return trace.Wrap(err, "performing admin setup")
	}
	tunName, err := tun.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN name")
	}
	if err := sendTUNNameAndFd(socketPath, tunName, tun.File().Fd()); err != nil {
		return trace.Wrap(err, "sending TUN over socket")
	}

	// Stay alive until we get an error on errCh, indicating that the osConfig loop exited.
	// If the socket is deleted, indicating that the parent process exited, cancel the context and then wait
	// for the osConfig loop to exit and send an err on errCh.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(socketPath); err != nil {
				slog.DebugContext(ctx, "failed to stat socket path, assuming parent exited")
				cancel()
				return trace.Wrap(<-errCh)
			}
		case err = <-errCh:
			return trace.Wrap(err)
		}
	}
}

// createAndSetupTUNDeviceAsRoot creates a virtual network device and configures the host OS to use that device for
// VNet connections.
//
// After the TUN device is created, it will be sent on the result channel. Any error will be sent on the err
// channel. Always select on both the result channel and the err channel when waiting for a result.
//
// This will keep running until [ctx] is canceled or an unrecoverable error is encountered, in order to keep
// the host OS configuration up to date.
func createAndSetupTUNDeviceAsRoot(ctx context.Context, ipv6Prefix, dnsAddr string) (<-chan tun.Device, <-chan error) {
	tunCh := make(chan tun.Device, 1)
	errCh := make(chan error, 2)

	tun, tunName, err := createTUNDevice(ctx)
	if err != nil {
		errCh <- trace.Wrap(err, "creating TUN device")
		return tunCh, errCh
	}
	tunCh <- tun

	osConfigurator, err := newOSConfigurator(tunName, ipv6Prefix, dnsAddr)
	if err != nil {
		errCh <- trace.Wrap(err, "creating OS configurator")
		return tunCh, errCh
	}

	go func() {
		defer func() {
			// Shutting down, deconfigure OS.
			errCh <- trace.Wrap(osConfigurator.deconfigureOS())
		}()

		if err := osConfigurator.updateOSConfiguration(ctx); err != nil {
			errCh <- trace.Wrap(err, "applying initial OS configuration")
			return
		}

		// Re-configure the host OS every 10 seconds. This will pick up any newly logged-in clusters by
		// reading profiles from TELEPORT_HOME.
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := osConfigurator.updateOSConfiguration(ctx); err != nil {
					errCh <- trace.Wrap(err, "updating OS configuration")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return tunCh, errCh
}

func createTUNDevice(ctx context.Context) (tun.Device, string, error) {
	slog.DebugContext(ctx, "Creating TUN device.")
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
