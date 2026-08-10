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
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
)

type beamsUnpublishCommand struct {
	*kingpin.CmdClause
	name string

	// These helper functions can be overridden in tests.
	getFn    func(context.Context, *client.TeleportClient, string) (*beamsv1.Beam, error)
	updateFn func(context.Context, *client.TeleportClient, *beamsv1.Beam) (*beamsv1.Beam, error)
}

func newBeamsUnpublishCommand(parent *kingpin.CmdClause) *beamsUnpublishCommand {
	cmd := &beamsUnpublishCommand{
		CmdClause: parent.Command("unpublish", "Unpublish a previously published service in a beam."),
	}
	cmd.getFn = cmd.getBeam
	cmd.updateFn = cmd.updateBeam
	cmd.Arg("name", "ID (or UUID) of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsUnpublishCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	beam, err := c.getFn(ctx, tc, c.name)
	if err != nil {
		return trace.Wrap(err)
	}

	if beam.Spec.Publish == nil {
		return trace.Errorf("Beam %q is not published.", beam.GetStatus().GetAlias())
	}

	// Blank out the `spec.publish` to trigger the deletion of the app.
	beam.Spec.Publish = nil

	updatedBeam, err := c.updateFn(ctx, tc, beam)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := fmt.Fprintf(
		cf.Stdout(),
		"Beam %q successfully unpublished.\n",
		updatedBeam.GetStatus().GetAlias(),
	); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *beamsUnpublishCommand) getBeam(ctx context.Context, tc *client.TeleportClient, name string) (*beamsv1.Beam, error) {
	var beam *beamsv1.Beam
	err := client.RetryWithRelogin(ctx, tc, func() error {
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

		beam, err = getBeam(ctx, rootClient, name)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beam, nil
}

func (c *beamsUnpublishCommand) updateBeam(ctx context.Context, tc *client.TeleportClient, beam *beamsv1.Beam) (*beamsv1.Beam, error) {
	var updatedBeam *beamsv1.Beam
	err := client.RetryWithRelogin(ctx, tc, func() error {
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

		rsp, err := rootClient.
			BeamServiceClient().
			UpdateBeam(ctx, &beamsv1.UpdateBeamRequest{Beam: beam})
		if err != nil {
			return trace.Wrap(err)
		}
		updatedBeam = rsp.GetBeam()
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updatedBeam, nil
}
