/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
)

// StatusCommand implements `tctl token` group of commands.
type StatusCommand struct {
	config *service.Config

	// CLI clauses (subcommands)
	status *kingpin.CmdClause
}

// Initialize allows StatusCommand to plug itself into the CLI parser.
func (c *StatusCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	c.status = app.Command("status", "Report cluster status.")
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *StatusCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.status.FullCommand():
		err = c.Status(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

type caFetchError struct {
	caType  types.CertAuthType
	message string
}

// Status is called to execute "status" CLI command.
func (c *StatusCommand) Status(ctx context.Context, client auth.ClientI) error {
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
