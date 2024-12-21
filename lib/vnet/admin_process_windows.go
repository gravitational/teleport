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

	// TODO(nklaassen): delete these, the admin process will decide them, they
	// don't need to be passed from the user process. Keeping them until I
	// remove the references from osconfig.go.
	IPv6Prefix string
	DNSAddr    string
	HomePath   string
}

func (c *AdminProcessConfig) CheckAndSetDefaults() error {
	if c.NamedPipe == "" {
		return trace.BadParameter("missing pipe path")
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
	log.InfoContext(ctx, "Successfully connected to user process over named pipe", "pipe", pipePath)

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

	// TODO(nklaassen): actually run VNet. For now, stay alive as long as we can
	// dial the pipe.
	for {
		select {
		case <-time.After(time.Second):
			conn, err := winio.DialPipe(pipePath, &dialTimeout)
			if err != nil {
				return trace.Wrap(err, "failed to dial pipe, assuming user process has terminated")
			}
			conn.Close()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

var (
	// Satisfy unused linter.
	// TODO(nklaassen): run os configuration loop in admin process.
	_ = osConfigurationLoop
)
