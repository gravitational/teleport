/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// StatusCommand implements `tctl token` group of commands.
type StatusCommand struct {
	config *servicecfg.Config

	// CLI clauses (subcommands)
	status *kingpin.CmdClause
}

// Initialize allows StatusCommand to plug itself into the CLI parser.
func (c *StatusCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	c.status = app.Command("status", "Report cluster status.")
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *StatusCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.status.FullCommand():
		commandFunc = c.Status
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

type caFetchError struct {
	caType  types.CertAuthType
	message string
}

// Status is called to execute "status" CLI command.
func (c *StatusCommand) Status(ctx context.Context, client *authclient.Client) error {
	pingRsp, err := client.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	serverVersion := pingRsp.ServerVersion
	clusterName := pingRsp.ClusterName

	var (
		authorities     []types.CertAuthority
		authFetchErrors []caFetchError
	)

	for _, caType := range types.CertAuthTypes {
		ca, err := client.GetCertAuthorities(ctx, caType, false)
		if err != nil {
			// Collect all errors, so they can be displayed to the user.
			fetchError := caFetchError{
				caType:  caType,
				message: err.Error(),
			}
			authFetchErrors = append(authFetchErrors, fetchError)
		} else {
			authorities = append(authorities, ca...)
		}
	}

	// Calculate the CA pins for this cluster. The CA pins are used by the
	// client to verify the identity of the Auth Server.
	localCAResponse, err := client.GetClusterCACert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return trace.Wrap(err)
	}

	view := func() string {
		table := asciitable.MakeHeadlessTable(2)
		table.AddRow([]string{"Cluster", clusterName})
		table.AddRow([]string{"Version", serverVersion})
		for _, ca := range authorities {
			if ca.GetClusterName() != clusterName {
				continue
			}
			info := fmt.Sprintf("%v CA ", string(ca.GetType()))
			rotation := ca.GetRotation()
			standbyPhase := rotation.Phase == types.RotationPhaseStandby || rotation.Phase == ""
			if standbyPhase && len(ca.GetAdditionalTrustedKeys().SSH) > 0 {
				// There should never be AdditionalTrusted keys present during
				// the Standby phase unless an auth server has just started up
				// with a new HSM (or without an HSM and all other auth servers
				// have HSMs)
				fmt.Println("WARNING: One or more auth servers has a newly added or removed " +
					"HSM or KMS configured. You should not route traffic to that server until " +
					"a CA rotation has been completed.")
			}
			if c.config.Debug {
				table.AddRow([]string{
					info,
					fmt.Sprintf("%v, update_servers: %v, complete: %v",
						rotation.String(),
						rotation.Schedule.UpdateServers.Format(constants.HumanDateFormatSeconds),
						rotation.Schedule.Standby.Format(constants.HumanDateFormatSeconds),
					),
				})
			} else {
				table.AddRow([]string{info, rotation.String()})
			}

		}
		for _, ca := range authFetchErrors {
			info := fmt.Sprintf("%v CA ", string(ca.caType))
			table.AddRow([]string{info, ca.message})
		}
		for _, caPin := range caPins {
			table.AddRow([]string{"CA pin", caPin})
		}
		return table.AsBuffer().String()
	}
	fmt.Print(view())

	// in debug mode, output mode of remote certificate authorities
	if c.config.Debug {
		view := func() string {
			table := asciitable.MakeHeadlessTable(2)
			for _, ca := range authorities {
				if ca.GetClusterName() == clusterName {
					continue
				}
				info := fmt.Sprintf("Remote %v CA %q", string(ca.GetType()), ca.GetClusterName())
				rotation := ca.GetRotation()
				table.AddRow([]string{info, rotation.String()})
			}
			return "Remote clusters\n\n" + table.AsBuffer().String()
		}
		fmt.Print(view())
	}
	return nil
}
