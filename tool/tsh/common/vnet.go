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
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet"
)

type vnetCLICommand interface {
	FullCommand() string
	run(cf *CLIConf) error
}

type vnetCommands struct {
	subcommands []vnetCLICommand
}

func (c *vnetCommands) tryRun(cf *CLIConf, command string) (bool, error) {
	for _, subcommand := range c.subcommands {
		if subcommand.FullCommand() == command {
			return true, trace.Wrap(subcommand.run(cf))
		}
	}
	return false, nil
}

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
	go func() {
		<-cf.Context.Done()
		processManager.Close()
	}()
	fmt.Println("VNet is ready.")
	return trace.Wrap(processManager.Wait())
}
