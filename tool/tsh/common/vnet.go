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

	"github.com/gravitational/teleport/api/profile"
	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/diag"
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
	cmd.Flag("diag", "Run diagnostics after starting VNet.").Hidden().BoolVar(&cmd.runDiag)
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	clientApp, err := newVnetClientApplication(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	vnetProcess, err := vnet.RunUserProcess(cf.Context, clientApp)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("VNet is ready.")

	if c.runDiag {
		go func() {
			fmt.Println("Running diagnostics.")
			//nolint:staticcheck // SA4023. runVnetDiagnostics on unsupported platforms always returns err.
			if err := runVnetDiagnostics(cf.Context, vnetProcess); err != nil {
				logger.ErrorContext(cf.Context, "Ran into a problem while running diagnostics", "error", err)
				return
			}
			fmt.Println("Done running diagnostics.")
		}()
	}

	context.AfterFunc(cf.Context, vnetProcess.Close)
	return trace.Wrap(vnetProcess.Wait())
}

type vnetSSHAutoConfigCommand struct {
	*kingpin.CmdClause
}

func newVnetSSHAutoConfigCommand(app *kingpin.Application) *vnetSSHAutoConfigCommand {
	cmd := &vnetSSHAutoConfigCommand{
		CmdClause: app.Command("vnet-ssh-autoconfig", "Automatically include VNet's generated OpenSSH-compatible config file in ~/.ssh/config."),
	}
	return cmd
}

func (c *vnetSSHAutoConfigCommand) run(cf *CLIConf) error {
	err := vnet.AutoConfigureOpenSSH(cf.Context, profile.FullProfilePath(cf.HomePath))
	return trace.Wrap(err)
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

func newVnetInstallServiceCommand(app *kingpin.Application) vnetCLICommand {
	return newPlatformVnetInstallServiceCommand(app)
}

func newVnetUninstallServiceCommand(app *kingpin.Application) vnetCLICommand {
	return newPlatformVnetUninstallServiceCommand(app)
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

// runDNSDiag runs the DNS check and prints any issues.
func runDNSDiag(ctx context.Context, vnetProcess *vnet.UserProcess) error {
	nsi := vnetProcess.NetworkStackInfo()
	if nsi.Ipv6Prefix == "" {
		return nil
	}
	cfg, err := vnetProcess.GetUnifiedClusterConfigProvider().GetUnifiedClusterConfig(ctx)
	if err != nil {
		return trace.Wrap(err, "getting unified cluster config for DNS diag")
	}
	vnetDNSServer, err := diag.DNSServerForIPv6Prefix(nsi.Ipv6Prefix)
	if err != nil {
		return trace.Wrap(err, "computing VNet DNS server address for DNS diag")
	}
	dnsDiag, err := diag.NewDNSDiag(&diag.DNSConfig{
		DNSZones:      cfg.AllDNSZones(),
		VNetDNSServer: vnetDNSServer,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	report, err := dnsDiag.Run(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	printDNSReport(report.GetDnsReport())
	return nil
}

// printDNSReport prints only zones that exhibit a problem.
func printDNSReport(r *diagv1.DNSReport) {
	if r == nil {
		return
	}
	if !r.VnetDnsReachable {
		fmt.Printf("VNet DNS is unreachable: %s\n", r.VnetDnsUnreachableError)
		return
	}
	for _, zr := range r.ZoneResults {
		switch zr.Status {
		case diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK:
			// Working as expected; not printed.
		case diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED:
			fmt.Printf("DNS zone %q is hijacked: observed=%s, expected=%s\n", zr.Zone, zr.ObservedIp, zr.ExpectedIp)
		case diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED:
			fmt.Printf("DNS zone %q is not registered with the OS resolver\n", zr.Zone)
		case diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT:
			fmt.Printf("DNS zone %q timed out: %s\n", zr.Zone, zr.Error)
		case diagv1.DNSZoneStatus_DNS_ZONE_STATUS_RESOLVER_ERROR:
			fmt.Printf("DNS zone %q resolver error: %s\n", zr.Zone, zr.Error)
		default:
			fmt.Printf("DNS zone %q has unexpected status %s\n", zr.Zone, zr.Status)
		}
	}
}
