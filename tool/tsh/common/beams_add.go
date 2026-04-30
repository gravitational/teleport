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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/tool/common"
)

type beamsAddCommand struct {
	*kingpin.CmdClause
	console bool
	format  string
}

func newBeamsAddCommand(parent *kingpin.CmdClause) *beamsAddCommand {
	cmd := &beamsAddCommand{
		CmdClause: parent.Command("add", "Start a new beam, and optionally connect to it via SSH."),
	}
	cmd.Flag("console", "Connect to the beam via SSH after creation.").Default("true").BoolVar(&cmd.console)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

func (c *beamsAddCommand) run(cf *CLIConf) error {
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

		// Create the beam.
		rsp, err := rootClient.
			BeamServiceClient().
			CreateBeam(ctx, &beamsv1.CreateBeamRequest{
				Egress: beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED,
			})
		if err != nil {
			return trace.Wrap(err)
		}

		beam = rsp.GetBeam()
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// formatBeams uses the proxy address to derive the Publish URL but we given
	// the beam won't be published yet, there's no need to actually fetch it.
	const proxyAddr = ""

	switch strings.ToLower(c.format) {
	case teleport.JSON:
		return trace.Wrap(common.PrintJSONIndent(cf.Stdout(), formatBeam(beam, proxyAddr)))
	case teleport.YAML:
		return trace.Wrap(common.PrintYAML(cf.Stdout(), formatBeam(beam, proxyAddr)))
	default:
		if _, err := fmt.Fprintf(
			cf.Stdout(),
			"Beam %q created.\n",
			beam.GetStatus().GetAlias(),
		); err != nil {
			return trace.Wrap(err)
		}

		// Connect to the beam via SSH.
		if c.console {
			return trace.Wrap(sshBeam(cf, tc, beam, nil))
		}
	}

	return nil
}
