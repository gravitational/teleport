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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet"
)

func newPlatformVnetAdminSetupCommand(app *kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}

// vnetServiceCommand runs the VNet service.
type vnetServiceCommand struct {
	*kingpin.CmdClause
	cfg vnet.LinuxAdminProcessConfig
}

func (c *vnetServiceCommand) run(clf *CLIConf) error {
	return trace.Wrap(vnet.RunLinuxAdminProcess(clf.Context, c.cfg))
}

func newPlatformVnetServiceCommand(app *kingpin.Application) *vnetServiceCommand {
	cmd := &vnetServiceCommand{
		CmdClause: app.Command("vnet-service", "Start the VNet admin subprocess.").Hidden(),
	}
	cmd.Flag("addr", "client application service address").Required().StringVar(&cmd.cfg.ClientApplicationServiceAddr)
	cmd.Flag("cred-path", "path to TLS credentials for connecting to client application").Required().StringVar(&cmd.cfg.ServiceCredentialPath)
	return cmd
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
	return trace.NotImplemented("diagnostics not implemented")
}
