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
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"
)

type AdminProcessConfig struct {
	// NamedPipe is the name of a pipe used for IPC between the user process and
	// the admin service.
	NamedPipe string
	// IPv6Prefix is the IPv6 prefix for the VNet.
	IPv6Prefix string
	// DNSAddr is the IP address for the VNet DNS server.
	DNSAddr string
	// HomePath points to TELEPORT_HOME that will be used by the admin process.
	HomePath string
}

func (c *AdminProcessConfig) CheckAndSetDefaults() error {
	switch {
	case c.NamedPipe == "":
		return trace.BadParameter("missing socket path")
	case c.IPv6Prefix == "":
		return trace.BadParameter("missing IPv6 prefix")
	case c.DNSAddr == "":
		return trace.BadParameter("missing DNS address")
	case c.HomePath == "":
		return trace.BadParameter("missing home path")
	}
	return nil
}

// RunAdminProcess must run as administrator. It creates and sets up a TUN
// device and runs the VNet networking stack.
//
// It also handles host OS configuration, OS configuration is updated every [osConfigurationInterval].
//
// The admin process will stay running until the socket at config.socketPath is
// deleted or until encountering an unrecoverable error.
func RunAdminProcess(ctx context.Context, cfg AdminProcessConfig) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "checking admin process config")
	}
	log.InfoContext(ctx, "Running VNet admin process", "cfg", cfg)

	dialTimeout := 200 * time.Millisecond
	conn, err := winio.DialPipe(pipePath, &dialTimeout)
	if err != nil {
		return trace.Wrap(err, "dialing named pipe %s", pipePath)
	}
	conn.Close()

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

	// TODO(nklaassen): actually run the networking stack and OS configuration.
	// For now, stay alive as long as we can dial the pipe.
	for {
		select {
		case <-time.After(time.Second):
			conn, err := winio.DialPipe(pipePath, &dialTimeout)
			if err != nil {
				return trace.Wrap(err)
			}
			conn.Close()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
