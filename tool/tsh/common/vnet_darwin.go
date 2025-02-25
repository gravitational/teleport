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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
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
	// socketPath is a path to a unix socket used for passing a TUN device from the admin process to
	// the unprivileged process.
	socketPath string
	// ipv6Prefix is the IPv6 prefix for the VNet.
	ipv6Prefix string
	// dnsAddr is the IP address for the VNet DNS server.
	dnsAddr string
	// egid of the user starting VNet. Unsafe for production use, as the egid comes from an unstrusted
	// source.
	egid int
	// euid of the user starting VNet. Unsafe for production use, as the euid comes from an unstrusted
	// source.
	euid int
}

func newPlatformVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Start the VNet admin subprocess.").Hidden(),
	}
	cmd.Flag("socket", "unix socket path").StringVar(&cmd.socketPath)
	cmd.Flag("ipv6-prefix", "IPv6 prefix for the VNet").StringVar(&cmd.ipv6Prefix)
	cmd.Flag("dns-addr", "VNet DNS address").StringVar(&cmd.dnsAddr)
	cmd.Flag("egid", "effective group ID of the user starting VNet").IntVar(&cmd.egid)
	cmd.Flag("euid", "effective user ID of the user starting VNet").IntVar(&cmd.euid)
	return cmd
}

func (c *vnetAdminSetupCommand) run(cf *CLIConf) error {
	homePath := os.Getenv(types.HomeEnvVar)
	if homePath == "" {
		// This runs as root so we need to be configured with the user's home path.
		return trace.BadParameter("%s must be set", types.HomeEnvVar)
	}
	config := daemon.Config{
		SocketPath: c.socketPath,
		IPv6Prefix: c.ipv6Prefix,
		DNSAddr:    c.dnsAddr,
		HomePath:   homePath,
		ClientCred: daemon.ClientCred{
			Valid: true,
			Egid:  c.egid,
			Euid:  c.euid,
		},
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

func runVnetDiagnostics(ctx context.Context, nsi vnet.NetworkStackInfo) error {
	fmt.Println("Running diagnostics.")
	routeConflictDiag, err := diag.NewRouteConflictDiag(&diag.RouteConflictConfig{
		VnetIfaceName: nsi.IfaceName,
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
