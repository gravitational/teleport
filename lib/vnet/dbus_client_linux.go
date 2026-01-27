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

func startService(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()

	obj := conn.Object(vnetDBusServiceName, dbus.ObjectPath(vnetDBusObjectPath))
	call := obj.CallWithContext(ctx, vnetDBusStartMethod, 0, cfg.ClientApplicationServiceAddr, cfg.ServiceCredentialPath)
	if call.Err != nil {
		return trace.Wrap(call.Err, "calling D-Bus Start")
	}
	return nil
}

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
