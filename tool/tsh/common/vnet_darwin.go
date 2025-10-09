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
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/daemon"
	"github.com/gravitational/teleport/lib/vnet/diag"
)

// vnetAdminSetupCommand is the fallback command ran as root when tsh wasn't compiled with the
// vnetdaemon build tag. This is typically the case when running tsh in development where it's not
// signed and bundled in tsh.app.
//
// This command expects TELEPORT_HOME to be set to the tsh home of the user who wants to run VNet.
type vnetAdminSetupCommand struct {
	*kingpin.CmdClause
	// addr is the local TCP address of the client application gRPC service.
	addr string
	// credPath is the path where credentials for IPC with the client
	// application are found.
	credPath string
}

func newPlatformVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Start the VNet admin subprocess.").Hidden(),
	}
	cmd.Flag("addr", "client application service address").Required().StringVar(&cmd.addr)
	cmd.Flag("cred-path", "path to TLS credentials for connecting to client application").Required().StringVar(&cmd.credPath)
	return cmd
}

func (c *vnetAdminSetupCommand) run(cf *CLIConf) error {
	config := daemon.Config{
		ClientApplicationServiceAddr: c.addr,
		ServiceCredentialPath:        c.credPath,
	}
	return trace.Wrap(vnet.RunDarwinAdminProcess(cf.Context, config))
}

// The vnet-service command is only supported on windows.
func newPlatformVnetServiceCommand(app *kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}

// The vnet-install-service command is only supported on windows.
func newPlatformVnetInstallServiceCommand(app *kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}

// The vnet-uninstall-service command is only supported on windows.
func newPlatformVnetUninstallServiceCommand(app *kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}

func runVnetDiagnostics(ctx context.Context, nsi *vnetv1.NetworkStackInfo) error {
	routeConflictDiag, err := diag.NewRouteConflictDiag(&diag.RouteConflictConfig{
		VnetIfaceName: nsi.InterfaceName,
		Routing:       &diag.DarwinRouting{},
		Interfaces:    &diag.NetInterfaces{},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	rcs, err := routeConflictDiag.Run(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, rc := range rcs.GetRouteConflictReport().RouteConflicts {
		fmt.Printf("Found a conflicting route: %+v\n", rc)
	}

	return nil
}
