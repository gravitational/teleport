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

type phaseRMCommand struct {
	*kingpin.CmdClause
	name string
}

func newPhaseRMCommand(parent *kingpin.CmdClause) *phaseRMCommand {
	cmd := &phaseRMCommand{CmdClause: parent.Command("rm", "Delete a Phase image.")}
	cmd.Arg("name", "Name of the Phase to delete.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *phaseRMCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()
		rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()
		_, err = rootClient.PhaseServiceClient().DeletePhase(cf.Context, beamsv1.DeletePhaseRequest_builder{Name: c.name}.Build())
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = fmt.Fprintf(cf.Stdout(), "Phase %q successfully deleted.\n", c.name)
	return trace.Wrap(err)
}
