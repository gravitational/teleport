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
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/client"
)

type beamsSSHCommand struct {
	*kingpin.CmdClause
	name string
}

func newBeamsSSHCommand(parent *kingpin.CmdClause) *beamsSSHCommand {
	cmd := &beamsSSHCommand{
		CmdClause: parent.Command("ssh", "Start an interactive shell in a beam, via SSH.").Alias("console"),
	}
	cmd.Arg("name", "ID (or UUID) of the beam to connect to.").Required().StringVar(&cmd.name)
	return cmd
}

func (c *beamsSSHCommand) run(cf *CLIConf) error {
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
	return trace.Wrap(sshBeam(cf, tc, beam, nil))
}

func sshBeam(cf *CLIConf, tc *client.TeleportClient, beam *beamsv1.Beam, command []string) error {
	// TODO(boxofrad): Right now, we expect the CreateBeam call to block until
	// it's ready to accept SSH connections - which can lead to timeouts when
	// we need to spin up more nodes. We should make this non-blocking and have
	// the retry loop re-read the beam.
	if beam.GetStatus().GetNodeId() == "" {
		return trace.Errorf("beam %q is not ready to accept SSH connections", beam.GetStatus().GetAlias())
	}

	target := fmt.Sprintf("%s:0", beam.GetStatus().GetNodeId())
	tc.HostLogin = beamsLogin
	tc.Stdin = cf.Stdin()

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  100 * time.Millisecond,
		Step:   100 * time.Millisecond,
		Max:    time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var lastErr error
	for i := range 10 {
		lastErr = trace.Wrap(tc.SSH(cf.Context, command, client.WithHostAddress(target)))
		if lastErr == nil {
			return nil
		}
		logger.DebugContext(cf.Context, "Connect to beam with retry", "attempt", i+1, "error", lastErr)

		switch {
		case trace.IsNotFound(lastErr):
			// Cache may not have receive the node write.
		case trace.IsConnectionProblem(lastErr):
			// Beam network may not be ready yet.
		default:
			return trace.Wrap(convertSSHExitCode(tc, lastErr))
		}

		select {
		case <-cf.Context.Done():
		case <-retry.After():
			retry.Inc()
		}
	}
	return lastErr
}
