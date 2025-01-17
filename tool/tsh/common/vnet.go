// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet"
)

type vnetCLICommand interface {
	// FullCommand matches the signature of kingpin.CmdClause.FullCommand, which
	// most commands should embed.
	FullCommand() string
	// run should be called iff FullCommand() matches the CLI parameters.
	run(cf *CLIConf) error
}

// vnetCommand implements the `tsh vnet` command to run VNet.
type vnetCommand struct {
	*kingpin.CmdClause
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access."),
	}
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	appProvider, err := newVnetAppProvider(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	processManager, err := vnet.RunUserProcess(cf.Context, &vnet.UserProcessConfig{AppProvider: appProvider})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("VNet is ready.")
	context.AfterFunc(cf.Context, processManager.Close)
	return trace.Wrap(processManager.Wait())
}

func newVnetAdminSetupCommand(app *kingpin.Application) vnetCLICommand {
	return newPlatformVnetAdminSetupCommand(app)
}

func newVnetDaemonCommand(app *kingpin.Application) vnetCLICommand {
	return newPlatformVnetDaemonCommand(app)
}

func newVnetServiceCommand(app *kingpin.Application) vnetCLICommand {
	return newPlatformVnetServiceCommand(app)
}

// vnetCommandNotSupported implements vnetCLICommand, it is returned when a specific
// command is not implemented for a certain platform or environment.
type vnetCommandNotSupported struct{}

func (vnetCommandNotSupported) FullCommand() string {
	return ""
}
func (vnetCommandNotSupported) run(*CLIConf) error {
	panic("vnetCommandNotSupported.run should never be called, this is a bug")
}
