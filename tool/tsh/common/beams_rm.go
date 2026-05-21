/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
)

type beamsRMCommand struct {
	*kingpin.CmdClause
	name string
}

func newBeamsRMCommand(parent *kingpin.CmdClause) *beamsRMCommand {
	cmd := &beamsRMCommand{
		CmdClause: parent.Command("rm", "Delete a beam."),
	}
	cmd.Arg("name", "ID (or UUID) of the beam to delete.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsRMCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var beam *beamsv1.Beam
	err = client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		rootClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()

		// Read the beam to get its UUID name.
		beam, err = getBeam(ctx, rootClient, c.name)
		if err != nil {
			return trace.Wrap(err)
		}

		// Delete the beam.
		_, err = rootClient.
			BeamServiceClient().
			DeleteBeam(ctx, &beamsv1.DeleteBeamRequest{
				Name: beam.GetMetadata().GetName(),
			})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := fmt.Fprintf(
		cf.Stdout(),
		"Beam %q successfully deleted.\n",
		beam.GetStatus().GetAlias(),
	); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
