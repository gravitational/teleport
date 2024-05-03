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
	"log/slog"
	"os"

	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"
)

// Run is a blocking call to create and start Teleport VNet.
func Run(ctx context.Context) error {
	ipv6Prefix, err := IPv6Prefix()
	if err != nil {
		return trace.Wrap(err)
	}

	tun, err := CreateAndSetupTUNDevice(ctx, ipv6Prefix.String())
	if err != nil {
		return trace.Wrap(err)
	}

	manager, err := NewManager(&Config{
		TUNDevice:  tun,
		IPv6Prefix: ipv6Prefix,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(manager.Run(ctx))
}

// AdminSubcommand is the tsh subcommand that should run as root that will
// create and setup a TUN device and pass the file descriptor for that device
// over the unix socket found at socketPath.
func AdminSubcommand(ctx context.Context, socketPath, ipv6Prefix string) error {
	tun, tunName, err := createAndSetupTUNDeviceAsRoot(ctx, ipv6Prefix)
	if err != nil {
		return trace.Wrap(err, "performing admin setup")
	}
	if err := sendTUNNameAndFd(socketPath, tunName, tun.File().Fd()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateAndSetupTUNDevice returns a virtual network device and configures the host OS to use that device for
// VNet connections.
func CreateAndSetupTUNDevice(ctx context.Context, ipv6Prefix string) (tun.Device, error) {
	var (
		device tun.Device
		name   string
		err    error
	)
	if os.Getuid() == 0 {
		// We can get here if the user runs `tsh vnet` as root, but it is not in the expected path when
		// started as a regular user. Typically we expect `tsh vnet` to be run as a non-root user, and for
		// AdminSubcommand to directly call createAndSetupTUNDeviceAsRoot.
		device, name, err = createAndSetupTUNDeviceAsRoot(ctx, ipv6Prefix)
	} else {
		device, name, err = createAndSetupTUNDeviceWithoutRoot(ctx, ipv6Prefix)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog.InfoContext(ctx, "Created TUN device.", "device", name)
	return device, nil
}

func createAndSetupTUNDeviceAsRoot(ctx context.Context, ipv6Prefix string) (tun.Device, string, error) {
	tun, tunName, err := createTUNDevice(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	tunIPv6 := ipv6Prefix + "1"
	cfg := osConfig{
		tunName: tunName,
		tunIPv6: tunIPv6,
	}
	if err := configureOS(ctx, &cfg); err != nil {
		return nil, "", trace.Wrap(err, "configuring OS")
	}

	return tun, tunName, nil
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

type osConfig struct {
	tunName string
	tunIPv6 string
}
