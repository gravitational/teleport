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

package common

import (
	"github.com/alecthomas/kingpin/v2"

	"github.com/gravitational/teleport"
)

type vnetCommands struct {
	vnet       *vnetCommand
	adminSetup *vnetAdminSetupCommand
}

func newVnetCommands(app *kingpin.Application) vnetCommands {
	return vnetCommands{
		vnet: newVnetCommand(app),
		// This is not a descendant of the vnet command because we want to refer to the admin setup
		// command using the same string both through app.Command and also exec.Command.
		adminSetup: newVnetAdminSetupCommand(app),
	}
}

type vnetCommand struct {
	*kingpin.CmdClause
	// customDNSZones is a comma-separated list of custom DNS zones.
	customDNSZones string
}

func newVnetCommandBase(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network emulator for HTTP and TCP apps."),
	}

	cmd.Flag("custom-dns-zones", "custom DNS zones (comma-separated)").StringVar(&cmd.customDNSZones)

	return cmd
}

type vnetAdminSetupCommand struct {
	*kingpin.CmdClause
	// socketPath is a path to a socket over which fd of the TUN device is exchanged.
	socketPath string
	// pidFilePath is a path to a PID file. Used by the privileged process to clean up DNS config when
	// the unprivileged process exits.
	pidFilePath string
	// customDNSZones is a comma-separated list of custom DNS zones.
	customDNSZones string
	// baseIPv6Address is the IPv6 prefix for the VNet.
	baseIPv6Address string
}

func newVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Helper to run the vnet setup as root.").Hidden(),
	}

	cmd.Flag("socket", "unix socket path").StringVar(&cmd.socketPath)
	cmd.Flag("pidfile", "pid file path").StringVar(&cmd.pidFilePath)
	cmd.Flag("custom-dns-zones", "custom DNS zones (comma-separated)").StringVar(&cmd.customDNSZones)
	cmd.Flag("ipv6-address", "IPv6 prefix for the VNet").StringVar(&cmd.baseIPv6Address)

	return cmd
}
