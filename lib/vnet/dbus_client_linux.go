// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"
)

// startService is called from the normal user process to start
// the privileged VNet daemon. It connects to the system D-Bus
// and calls the corresponding Start method, exposed on the VNet
// D-Bus interface, passing the client service unix socket path.
func startService(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()

	// basically this corresponds to calling something like
	// `busctl --system call org.teleport.vnet1 /org/teleport/vnet1 org.teleport.vnet1.Daemon Start s "<socketPath>"`
	// each D-Bus service owns a well-known name you refer to, then you specify an
	// object path. object path is for granularity (a service can expose
	// multiple objects, but it rarely used, so in our case it is the same as the name but
	// slash-separated). then you call a method on a specific interface. the interface
	// is implemented by some object, for vnet it’s the dbusDaemon struct. the D-Bus
	// interface exposes the same methods as dbusDaemon, so we can call them over
	// D-Bus.
	obj := conn.Object(vnetDBusServiceName, dbus.ObjectPath(vnetDBusObjectPath))
	call := obj.CallWithContext(ctx, vnetDBusStartMethod, 0, cfg.ClientApplicationServiceSocketPath)
	if call.Err != nil {
		return trace.Wrap(call.Err, "calling D-Bus Start")
	}
	return nil
}

// stopService is called from the normal user process to stop
// the privileged VNet daemon. It connects to the system D-Bus
// and calls the corresponding Stop method, exposed on the VNet
// D-Bus interface.
func stopService(ctx context.Context) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()

	obj := conn.Object(vnetDBusServiceName, dbus.ObjectPath(vnetDBusObjectPath))
	call := obj.CallWithContext(ctx, vnetDBusStopMethod, 0)
	if call.Err != nil {
		return trace.Wrap(call.Err, "calling D-Bus Stop")
	}
	return nil
}
