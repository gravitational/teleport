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

	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"
)

// runWindowsAdminProcess must run as administrator. It creates and sets up a TUN
// device, runs the VNet networking stack, and handles OS configuration. It will
// continue to run until [ctx] is canceled or encountering an unrecoverable
// error.
func runWindowsAdminProcess(ctx context.Context) error {
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
	// TODO(nklaassen): actually run VNet. For now, just stay alive until the
	// context is canceled.
	<-ctx.Done()
	return trace.Wrap(ctx.Err())
}
