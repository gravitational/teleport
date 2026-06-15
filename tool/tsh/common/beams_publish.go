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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/tool/common"
)

type beamsPublishCommand struct {
	*kingpin.CmdClause
	name   string
	tcp    bool
	format string

	// These helper functions can be overridden in tests.
	getFn       func(context.Context, *client.TeleportClient, string) (*beamsv1.Beam, error)
	updateFn    func(context.Context, *client.TeleportClient, *beamsv1.Beam) (*beamsv1.Beam, error)
	proxyAddrFn func(*CLIConf) (string, error)
}

func newBeamsPublishCommand(parent *kingpin.CmdClause) *beamsPublishCommand {
	cmd := &beamsPublishCommand{
		CmdClause: parent.Command("publish", "Publish an HTTP or TCP service running in a beam."),
	}
	cmd.getFn = cmd.getBeam
	cmd.updateFn = cmd.updateBeam
	cmd.proxyAddrFn = cmd.proxyAddr
	cmd.Flag("tcp", "Publish as a TCP app instead of an HTTP app.").BoolVar(&cmd.tcp)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("name", "ID (or UUID) of the beam to target.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsPublishCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	beam, err := c.getFn(ctx, tc, c.name)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set the `spec.publish` to trigger the creation of an app.
	beam.Spec.Publish = &beamsv1.PublishSpec{
		Port:     8080,
		Protocol: beamsv1.Protocol_PROTOCOL_HTTP,
	}
	if c.tcp {
		beam.Spec.Publish.Protocol = beamsv1.Protocol_PROTOCOL_TCP
	}

	updatedBeam, err := c.updateFn(ctx, tc, beam)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyAddr, err := c.proxyAddrFn(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.print(cf, updatedBeam, proxyAddr))
}

func (c *beamsPublishCommand) getBeam(ctx context.Context, tc *client.TeleportClient, name string) (*beamsv1.Beam, error) {
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

func (c *beamsPublishCommand) updateBeam(ctx context.Context, tc *client.TeleportClient, beam *beamsv1.Beam) (*beamsv1.Beam, error) {
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

func (c *beamsPublishCommand) proxyAddr(cf *CLIConf) (string, error) {
	_, proxyAddr, err := fetchProxyVersion(cf)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return proxyAddr, nil
}

func (c *beamsPublishCommand) print(cf *CLIConf, updatedBeam *beamsv1.Beam, proxyAddr string) error {
	switch strings.ToLower(c.format) {
	case teleport.JSON:
		return trace.Wrap(common.PrintJSONIndent(cf.Stdout(), formatBeam(updatedBeam, proxyAddr)))
	case teleport.YAML:
		return trace.Wrap(common.PrintYAML(cf.Stdout(), formatBeam(updatedBeam, proxyAddr)))
	default:
		if _, err := fmt.Fprintf(
			cf.Stdout(),
			"Beam %q successfully published.\n\n",
			updatedBeam.GetStatus().GetAlias(),
		); err != nil {
			return trace.Wrap(err)
		}

		switch updatedBeam.GetSpec().GetPublish().GetProtocol() {
		case beamsv1.Protocol_PROTOCOL_HTTP:
			if _, err := fmt.Fprintf(
				cf.Stdout(),
				"URL: %s\n",
				beamPublishURL(updatedBeam, proxyAddr),
			); err != nil {
				return trace.Wrap(err)
			}
		case beamsv1.Protocol_PROTOCOL_TCP:
			if _, err := fmt.Fprintf(
				cf.Stdout(),
				"Connect to your TCP application via VNet at:\n  %s\n\n",
				beamPublishURL(updatedBeam, proxyAddr),
			); err != nil {
				return trace.Wrap(err)
			}
			if _, err := fmt.Fprintf(
				cf.Stdout(),
				"Or start a local tunnel to the application with:\n  tsh proxy app %s\n",
				updatedBeam.GetStatus().GetAppName(),
			); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}
}
