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
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// WorkloadIdentityCommand is a group of commands pertaining to Teleport
// Workload Identity.
type WorkloadIdentityCommand struct {
	format               string
	workloadIdentityName string

	listCmd *kingpin.CmdClause
	rmCmd   *kingpin.CmdClause

	stdout io.Writer
}

// Initialize sets up the "tctl workload-identity" command.
func (c *WorkloadIdentityCommand) Initialize(
	app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config,
) {
	// TODO(noah): Remove the hidden flag once base functionality is released.
	cmd := app.Command(
		"workload-identity",
		"Manage Teleport Workload Identity.",
	).Hidden()

	c.listCmd = cmd.Command(
		"ls",
		"List workload identity configurations.",
	)
	c.listCmd.
		Flag(
			"format",
			"Output format, 'text' or 'json'",
		).
		Hidden().
		Default(teleport.Text).
		EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.rmCmd = cmd.Command(
		"rm",
		"Delete a workload identity configuration.",
	)
	c.rmCmd.
		Arg("name", "Name of the workload identity configuration to delete.").
		Required().
		StringVar(&c.workloadIdentityName)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun attempts to run subcommands.
func (c *WorkloadIdentityCommand) TryRun(
	ctx context.Context, cmd string, clientFunc commonclient.InitFunc,
) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.listCmd.FullCommand():
		commandFunc = c.ListWorkloadIdentities
	case c.rmCmd.FullCommand():
		commandFunc = c.DeleteWorkloadIdentity
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

func (c *WorkloadIdentityCommand) DeleteWorkloadIdentity(
	ctx context.Context,
	client *authclient.Client,
) error {
	workloadIdentityClient := client.WorkloadIdentityResourceServiceClient()
	_, err := workloadIdentityClient.DeleteWorkloadIdentity(
		ctx, &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
			Name: c.workloadIdentityName,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(
		c.stdout,
		"Workload Identity %q deleted successfully.\n",
		c.workloadIdentityName,
	)

	return nil
}

// ListWorkloadIdentities writes a listing of the WorkloadIdentity resources
func (c *WorkloadIdentityCommand) ListWorkloadIdentities(
	ctx context.Context, client *authclient.Client,
) error {
	workloadIdentityClient := client.WorkloadIdentityResourceServiceClient()
	var workloadIdentities []*workloadidentityv1pb.WorkloadIdentity
	req := &workloadidentityv1pb.ListWorkloadIdentitiesRequest{}
	for {
		resp, err := workloadIdentityClient.ListWorkloadIdentities(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		workloadIdentities = append(
			workloadIdentities, resp.WorkloadIdentities...,
		)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	if c.format == teleport.Text {
		if len(workloadIdentities) == 0 {
			fmt.Fprintln(c.stdout, "No workload identities configured")
			return nil
		}
		t := asciitable.MakeTable([]string{"Name", "SPIFFE ID"})
		for _, u := range workloadIdentities {
			t.AddRow([]string{
				u.GetMetadata().GetName(), u.GetSpec().GetSpiffe().GetId(),
			})
		}
		fmt.Fprintln(c.stdout, t.AsBuffer().String())
	} else {
		err := utils.WriteJSONArray(c.stdout, workloadIdentities)
		if err != nil {
			return trace.Wrap(err, "failed to marshal workload identities")
		}
	}
	return nil
}
