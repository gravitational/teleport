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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
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
	err = withRootCluster(ctx, tc, func(root authclient.ClientI, _ string) error {
		beam, err = getBeam(ctx, root, c.name)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(sshBeam(cf, tc, beam, nil))
}

func sshBeam(cf *CLIConf, tc *client.TeleportClient, beam *beamsv1.Beam, command []string) error {
	return sshBeamWithRetry(cf, tc, beam, command, true)
}

// Log readers disable connection retries because a lost exec connection may
// already have streamed data or acknowledged collection. Replaying is unsafe.
func sshBeamWithRetry(cf *CLIConf, tc *client.TeleportClient, beam *beamsv1.Beam, command []string, retryConnection bool) error {
	nodeID, err := beamNodeID(beam)
	if err != nil {
		return trace.Wrap(err)
	}

	target := fmt.Sprintf("%s:0", nodeID)
	tc.HostLogin = types.BeamsLogin
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
			if !retryConnection {
				return lastErr
			}
			// Beam network may not be ready yet.
		default:
			return trace.Wrap(convertSSHExitCode(tc, lastErr))
		}

		select {
		case <-cf.Context.Done():
			return trace.Wrap(cf.Context.Err())
		case <-retry.After():
			retry.Inc()
		}
	}
	return lastErr
}
