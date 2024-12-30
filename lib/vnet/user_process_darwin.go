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
	"time"

	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
)

// UserProcessConfig provides the necessary configuration to run VNet.
type UserProcessConfig struct {
	// AppProvider is a required field providing an interface implementation for [AppProvider].
	AppProvider AppProvider
	// ClusterConfigCache is an optional field providing [ClusterConfigCache]. If empty, a new cache
	// will be created.
	ClusterConfigCache *ClusterConfigCache
	// HomePath is the tsh home used for Teleport clients created by VNet. Resolved using the same
	// rules as HomeDir in tsh.
	HomePath string
}

func (c *UserProcessConfig) CheckAndSetDefaults() error {
	if c.AppProvider == nil {
		return trace.BadParameter("missing AppProvider")
	}
	if c.HomePath == "" {
		c.HomePath = profile.FullProfilePath(os.Getenv(types.HomeEnvVar))
	}
	return nil
}

// RunUserProcess creates a network stack for VNet and runs it in the background. To do
// this, it also needs to launch an admin process in the background. It returns
// a [ProcessManager] which controls the lifecycle of both background tasks.
//
// The caller is expected to call Close on the process manager to close the
// network stack, clean up any resources used by it and terminate the admin
// process.
//
// ctx is used to wait for setup steps that happen before RunUserProcess hands out the
// control to the process manager. If ctx gets canceled during RunUserProcess, the process
// manager gets closed along with its background tasks.
func RunUserProcess(ctx context.Context, config *UserProcessConfig) (pm *ProcessManager, err error) {
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()

	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})

	pm, processCtx := newProcessManager()

	// Create the socket that's used to receive the TUN device from the admin process.
	socket, socketPath, err := createSocket()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.DebugContext(ctx, "Created unix socket for admin process", "socket", socketPath)
	pm.AddCriticalBackgroundTask("socket closer", func() error {
		// Keep the socket open until the process context is canceled.
		// Closing the socket signals the admin process to terminate.
		<-processCtx.Done()
		return trace.NewAggregate(processCtx.Err(), socket.Close())
	})

	pm.AddCriticalBackgroundTask("admin process", func() error {
		adminConfig := AdminProcessConfig{
			SocketPath: socketPath,
			IPv6Prefix: ipv6Prefix.String(),
			DNSAddr:    dnsIPv6.String(),
			HomePath:   config.HomePath,
		}
		return trace.Wrap(execAdminProcess(processCtx, adminConfig))
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
		return nil, trace.Wrap(context.Cause(receiveTunCtx))
	case <-processCtx.Done():
		return nil, trace.Wrap(context.Cause(processCtx))
	case err := <-recvTUNErr:
		if err != nil {
			if processCtx.Err() != nil {
				// Both errors being present means that VNet failed to receive a TUN device because of a
				// problem with the admin process.
				// Returning error from processCtx will be more informative to the user, e.g., the error
				// will say "password prompt closed by user" instead of "read from closed socket".
				log.DebugContext(ctx, "Error from recvTUNErr ignored in favor of processCtx.Err", "error", err)
				return nil, trace.Wrap(context.Cause(processCtx))
			}
			return nil, trace.Wrap(err, "receiving TUN device from admin process")
		}
	}

	appResolver, err := newTCPAppResolver(config.AppProvider,
		WithClusterConfigCache(config.ClusterConfigCache))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ns, err := newNetworkStack(&networkStackConfig{
		tunDevice:          tun,
		ipv6Prefix:         ipv6Prefix,
		dnsIPv6:            dnsIPv6,
		tcpHandlerResolver: appResolver,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pm.AddCriticalBackgroundTask("network stack", func() error {
		return trace.Wrap(ns.run(processCtx))
	})

	return pm, nil
}
