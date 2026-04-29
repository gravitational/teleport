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
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	filesftp "github.com/gravitational/teleport/lib/sshutils/sftp"
)

type beamsSCPCommand struct {
	*kingpin.CmdClause
	recursive bool
	quiet     bool
	src       string
	dest      string
	format    string

	// These helper functions can be overridden in tests.
	getBeamFn     func(context.Context, authclient.ClientI, string) (*beamsv1.Beam, error)
	sftpFn        func(context.Context, *client.TeleportClient, client.SFTPRequest) error
	withClusterFn func(context.Context, *client.TeleportClient, func(authclient.ClientI) error) error
}

func newBeamsSCPCommand(parent *kingpin.CmdClause) *beamsSCPCommand {
	cmd := &beamsSCPCommand{
		CmdClause: parent.Command("scp", "Copy files between a beam and the local filesystem.").Alias("cp"),
	}
	cmd.getBeamFn = getBeam
	cmd.sftpFn = func(ctx context.Context, tc *client.TeleportClient, req client.SFTPRequest) error {
		return trace.Wrap(tc.SFTP(ctx, req))
	}
	cmd.withClusterFn = cmd.withCluster
	cmd.Flag("recursive", "Recursive copy of subdirectories.").Short('r').BoolVar(&cmd.recursive)
	cmd.Flag("quiet", "Quiet mode.").Short('q').BoolVar(&cmd.quiet)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	cmd.Arg("src", "Source path to copy, in the form `BEAM_ID:PATH` or `LOCAL_PATH`.").Required().StringVar(&cmd.src)
	cmd.Arg("dest", "Destination path to copy, in the form `BEAM_ID:PATH` or `LOCAL_PATH`.").Required().StringVar(&cmd.dest)
	return cmd
}

func (c *beamsSCPCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

	err = c.withClusterFn(ctx, tc, func(cli authclient.ClientI) error {
		src, err := c.parseTarget(ctx, cli, c.src)
		if err != nil {
			return trace.Wrap(err)
		}
		dest, err := c.parseTarget(ctx, cli, c.dest)
		if err != nil {
			return trace.Wrap(err)
		}
		if src.beam == nil && dest.beam == nil {
			return trace.BadParameter("at least one of <src> <dest> must be in the form `BEAM_ID:PATH`")
		}

		req := client.SFTPRequest{
			Sources:     []string{src.toSFTP()},
			Destination: dest.toSFTP(),
			Recursive:   c.recursive,
		}
		if !c.quiet {
			req.ProgressWriter = cf.Stdout()
		}
		if err := c.sftpFn(ctx, tc, req); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(cf.Stdout(), "Copied successfully."); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return trace.Wrap(err)
}

func (c *beamsSCPCommand) withCluster(ctx context.Context, tc *client.TeleportClient, fn func(authclient.ClientI) error) error {
	return trace.Wrap(client.RetryWithRelogin(ctx, tc, func() error {
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

		return trace.Wrap(fn(rootClient))
	}))
}

type beamCopyTarget struct {
	path string
	beam *beamsv1.Beam
}

func (c *beamsSCPCommand) parseTarget(ctx context.Context, client authclient.ClientI, path string) (*beamCopyTarget, error) {
	// This is a path to a local file.
	if !filesftp.IsRemotePath(path) {
		return &beamCopyTarget{path: path}, nil
	}

	// Target is in the form `BEAM:PATH` (e.g. `quantum-leap:/etc/hosts`).
	before, after, _ := strings.Cut(path, ":")

	beam, err := c.getBeamFn(ctx, client, before)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if beam.GetStatus().GetNodeId() == "" {
		return nil, trace.Errorf("beam %q is not ready to accept SSH connections", beam.GetStatus().GetAlias())
	}

	return &beamCopyTarget{beam: beam, path: after}, nil
}

func (t *beamCopyTarget) toSFTP() string {
	if t.beam == nil {
		return t.path
	}
	return fmt.Sprintf(
		"%s@%s:%s",
		beamsLogin,
		t.beam.GetStatus().GetNodeId(),
		t.path,
	)
}
