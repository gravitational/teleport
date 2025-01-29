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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// runPlatformUserProcess creates a network stack for VNet and runs it in the
// background. To do this, it also needs to launch an admin process in the
// background. It returns a [ProcessManager] which controls the lifecycle of
// both background tasks.
func runPlatformUserProcess(ctx context.Context, cfg *UserProcessConfig) (pm *ProcessManager, nsi NetworkStackInfo, err error) {
	// Make sure to close the process manager if returning a non-nil error.
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()

	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return nil, nsi, trace.Wrap(err)
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})

	// Create the socket that's used to receive the TUN device from the admin process.
	socket, socketPath, err := createSocket()
	if err != nil {
		return nil, nsi, trace.Wrap(err)
	}
	log.DebugContext(ctx, "Created unix socket for admin process", "socket", socketPath)

	pm, processCtx := newProcessManager()
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
			HomePath:   cfg.HomePath,
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

	// It should be more than waitingForEnablementTimeout in the vnet/daemon package
	// so that the user sees the error about the background item first.
	const receiveTunTimeout = time.Minute
	receiveTunCtx, cancel := context.WithTimeoutCause(ctx, receiveTunTimeout,
		errors.New("admin process did not send back TUN device within timeout"))
	defer cancel()

	select {
	case <-receiveTunCtx.Done():
		return nil, nsi, trace.Wrap(context.Cause(receiveTunCtx))
	case <-processCtx.Done():
		return nil, nsi, trace.Wrap(context.Cause(processCtx))
	case err := <-recvTUNErr:
		if err != nil {
			if processCtx.Err() != nil {
				// Both errors being present means that VNet failed to receive a TUN device because of a
				// problem with the admin process.
				// Returning error from processCtx will be more informative to the user, e.g., the error
				// will say "password prompt closed by user" instead of "read from closed socket".
				log.DebugContext(ctx, "Error from recvTUNErr ignored in favor of processCtx.Err", "error", err)
				return nil, nsi, trace.Wrap(context.Cause(processCtx))
			}
			return nil, nsi, trace.Wrap(err, "receiving TUN device from admin process")
		}
	}

	tunDeviceName, err := tun.Name()
	if err != nil {
		return nil, nsi, trace.Wrap(err)
	}

	clock := clockwork.NewRealClock()
	appProvider := newLocalAppProvider(cfg.ClientApplication, clock)
	appResolver := newTCPAppResolver(appProvider, clock)
	ns, err := newNetworkStack(&networkStackConfig{
		tunDevice:          tun,
		ipv6Prefix:         ipv6Prefix,
		dnsIPv6:            dnsIPv6,
		tcpHandlerResolver: appResolver,
	})
	if err != nil {
		return nil, nsi, trace.Wrap(err)
	}

	pm.AddCriticalBackgroundTask("network stack", func() error {
		return trace.Wrap(ns.run(processCtx))
	})

	nsi = NetworkStackInfo{
		IfaceName: tunDeviceName,
	}

	return pm, nsi, nil
}
