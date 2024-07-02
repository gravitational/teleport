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

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// SetupAndRun creates a network stack for VNet and runs it in the background. To do this, it also
// needs to launch an admin process in the background. It returns [ProcessManager] which controls
// the lifecycle of both background tasks.
//
// The caller is expected to call Close on the process manager to close the network stack, clean
// up any resources used by it and terminate the admin process.
//
// ctx is used to wait for setup steps that happen before SetupAndRun hands out the control to the
// process manager. If ctx gets canceled during SetupAndRun, the process manager gets closed along
// with its background tasks.
func SetupAndRun(ctx context.Context, config *SetupAndRunConfig) (*ProcessManager, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

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

	// Create the socket that's used to receive the TUN device from the admin process.
	socket, socketPath, err := createUnixSocket()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Created unix socket for admin process", "socket", socketPath)
	pm.AddCriticalBackgroundTask("socket closer", func() error {
		// Keep the socket open until the process context is canceled.
		// Closing the socket signals the admin process to terminate.
		<-processCtx.Done()
		return trace.NewAggregate(processCtx.Err(), socket.Close())
	})

	pm.AddCriticalBackgroundTask("admin process", func() error {
		daemonConfig := daemon.Config{
			SocketPath: socketPath,
			IPv6Prefix: ipv6Prefix.String(),
			DNSAddr:    dnsIPv6.String(),
			HomePath:   config.HomePath,
		}
		if err := daemonConfig.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(execAdminProcess(processCtx, daemonConfig))
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
				// problem with the admin process.
				// Returning error from processCtx will be more informative to the user, e.g., the error
				// will say "password prompt closed by user" instead of "read from closed socket".
				slog.DebugContext(ctx, "Error from recvTUNErr ignored in favor of processCtx.Err", "error", err)
				return nil, trace.Wrap(context.Cause(processCtx))
			}
			return nil, trace.Wrap(err, "receiving TUN from admin process")
		}
	}

	appResolver, err := NewTCPAppResolver(config.AppProvider,
		WithClusterConfigCache(config.ClusterConfigCache))
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

// SetupAndRunConfig provides collaborators for the [SetupAndRun] function.
type SetupAndRunConfig struct {
	// AppProvider is a required field providing an interface implementation for [AppProvider].
	AppProvider AppProvider
	// ClusterConfigCache is an optional field providing [ClusterConfigCache]. If empty, a new cache
	// will be created.
	ClusterConfigCache *ClusterConfigCache
	// HomePath is the tsh home used for Teleport clients created by VNet. Resolved using the same
	// rules as HomeDir in tsh.
	HomePath string
}

func (c *SetupAndRunConfig) CheckAndSetDefaults() error {
	if c.AppProvider == nil {
		return trace.BadParameter("missing AppProvider")
	}

	if c.HomePath == "" {
		c.HomePath = profile.FullProfilePath(os.Getenv(types.HomeEnvVar))
	}

	return nil
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

// AdminSetup is supposed to be ran as root. It creates and setups a TUN device and passses the file
// descriptor for that device over the unix socket found at config.socketPath.
//
// It also handles host OS configuration that must run as root, and stays alive to keep the host configuration
// up to date. It will stay running until the socket at config.socketPath is deleted or until encountering an
// unrecoverable error.
func AdminSetup(ctx context.Context, config daemon.Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tunName, err := createAndSendTUNDevice(ctx, config.SocketPath)
	if err != nil {
		return trace.Wrap(err)
	}

	errCh := make(chan error)
	go func() {
		errCh <- trace.Wrap(osConfigurationLoop(ctx, tunName, config.IPv6Prefix, config.DNSAddr, config.HomePath))
	}()

	// Stay alive until we get an error on errCh, indicating that the osConfig loop exited.
	// If the socket is deleted, indicating that the parent process exited, cancel the context and then wait
	// for the osConfig loop to exit and send an err on errCh.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(config.SocketPath); err != nil {
				slog.DebugContext(ctx, "failed to stat socket path, assuming parent exited")
				cancel()
				return trace.Wrap(<-errCh)
			}
		case err := <-errCh:
			return trace.Wrap(err)
		}
	}
}

// createAndSendTUNDevice creates a virtual network TUN device and sends the open file descriptor on
// [socketPath]. It returns the name of the TUN device or an error.
func createAndSendTUNDevice(ctx context.Context, socketPath string) (string, error) {
	tun, tunName, err := createTUNDevice(ctx)
	if err != nil {
		return "", trace.Wrap(err, "creating TUN device")
	}

	defer func() {
		// We can safely close the TUN device in the admin process after it has been sent on the socket.
		if err := tun.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close TUN device.", "error", trace.Wrap(err))
		}
	}()

	if err := sendTUNNameAndFd(socketPath, tunName, tun.File()); err != nil {
		return "", trace.Wrap(err, "sending TUN over socket")
	}
	return tunName, nil
}

// osConfigurationLoop will keep running until [ctx] is canceled or an unrecoverable error is encountered, in
// order to keep the host OS configuration up to date.
func osConfigurationLoop(ctx context.Context, tunName, ipv6Prefix, dnsAddr, homePath string) error {
	osConfigurator, err := newOSConfigurator(tunName, ipv6Prefix, dnsAddr, homePath)
	if err != nil {
		return trace.Wrap(err, "creating OS configurator")
	}

	// Clean up any stale configuration left by a previous VNet instance that may have failed to clean up.
	// This is necessary in case any stale /etc/resolver/<proxy address> entries are still present, we need to
	// be able to reach the proxy in order to fetch the vnet_config.
	if err := osConfigurator.deconfigureOS(ctx); err != nil {
		return trace.Wrap(err, "cleaning up OS configuration on startup")
	}

	defer func() {
		// Shutting down, deconfigure OS. Pass context.Background because [ctx] has likely been canceled
		// already but we still need to clean up.
		if err := osConfigurator.deconfigureOS(context.Background()); err != nil {
			slog.ErrorContext(ctx, "Error deconfiguring host OS before shutting down.", "error", err)
		}
	}()

	if err := osConfigurator.updateOSConfiguration(ctx); err != nil {
		return trace.Wrap(err, "applying initial OS configuration")
	}

	// Re-configure the host OS every 10 seconds. This will pick up any newly logged-in clusters by
	// reading profiles from TELEPORT_HOME.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := osConfigurator.updateOSConfiguration(ctx); err != nil {
				return trace.Wrap(err, "updating OS configuration")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
