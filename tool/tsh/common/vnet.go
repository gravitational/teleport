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
	"time"

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
	// runDiag determines whether to run diagnostics after VNet starts or not. Intended as a "feature
	// flag" before we start running diagnostics on each start of VNet.
	runDiag bool
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access."),
	}
	cmd.Flag("diag", "Run diagnostics after starting VNet").Hidden().BoolVar(&cmd.runDiag)
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	clientApp, err := newVnetClientApplication(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	processManager, nsi, err := vnet.RunUserProcess(cf.Context, &vnet.UserProcessConfig{
		ClientApplication: clientApp,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("VNet is ready.")

	if c.runDiag {
		go func() {
			timeout := 2 * time.Second
			fmt.Printf("Running diagnostics in %s.\n", timeout)
			select {
			case <-cf.Context.Done():
				return
			case <-time.After(timeout):
			}
			// Sleep is needed to give the admin process time to actually set up the routes.
			// TODO(ravicious): Figure out how to guarantee that routes are set up without sleeping.
			//nolint:staticcheck // SA4023. runVnetDiagnostics on unsupported platforms always returns err.
			if err := runVnetDiagnostics(cf.Context, nsi); err != nil {
				logger.ErrorContext(cf.Context, "Ran into a problem while running diagnostics", "error", err)
				return
			}
			fmt.Println("Done running diagnostics.")
		}()
	}

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
