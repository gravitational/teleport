//go:build darwin
// +build darwin

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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/vnet"
)

type vnetCommand struct {
	*kingpin.CmdClause
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		// TODO(nklaassen): unhide this when ready to ship.
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access.").Hidden(),
	}
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	return trace.Wrap(vnet.Run(cf.Context))
}

type vnetAdminSetupCommand struct {
	*kingpin.CmdClause
	// ipv6Prefix is the IPv6 prefix for the VNet.
	ipv6Prefix string
	// socketPath is a path to a unix socket used for communication with the parent process.
	socketPath string
}

func newVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Start the VNet admin subprocess.").Hidden(),
	}
	cmd.Flag("ipv6-prefix", "IPv6 prefix for the VNet").StringVar(&cmd.ipv6Prefix)
	cmd.Flag("socket", "unix socket path").StringVar(&cmd.socketPath)
	return cmd
}

func (c *vnetAdminSetupCommand) run(cf *CLIConf) error {
	return trace.Wrap(vnet.AdminSubcommand(cf.Context, c.socketPath, c.ipv6Prefix))
}
