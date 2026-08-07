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
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
)

type beamsExecCommand struct {
	*kingpin.CmdClause
	name    string
	command []string
}

func newBeamsExecCommand(parent *kingpin.CmdClause) *beamsExecCommand {
	cmd := &beamsExecCommand{
		CmdClause: parent.Command("exec", "Run a command in a beam, via SSH."),
	}
	cmd.Arg("name", "ID (or UUID) of the beam to target.").Required().StringVar(&cmd.name)
	cmd.Arg("command", "Command to execute in the instance.").Required().StringsVar(&cmd.command)
	return cmd
}

func (c *beamsExecCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

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

		beam, err = getBeam(ctx, rootClient, c.name)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(sshBeam(cf, tc, beam, c.command))
}
