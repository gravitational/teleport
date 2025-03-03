/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package cmd

import (
	"context"
	"os/exec"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Cmds represents a single command in two variants – one that can be used to spawn a process and
// one that can be copied and pasted into a terminal.
type Cmds struct {
	// Exec is the command that should be used when directly executing a command for the given
	// gateway.
	Exec *exec.Cmd
	// Preview is the command that should be used to display the command in the UI. Typically this
	// means that Preview includes quotes around special characters, so that the command gets executed
	// properly when the user copies and then pastes it into a terminal.
	Preview *exec.Cmd
}

// NewDBCLICommand creates CLI commands for database gateway.
func NewDBCLICommand(ctx context.Context, cluster *clusters.Cluster, gateway gateway.Gateway, authClient authclient.ClientI) (Cmds, error) {
	cmds, err := newDBCLICommandWithExecer(ctx, cluster, gateway, dbcmd.SystemExecer{}, authClient)
	return cmds, trace.Wrap(err)
}

func newDBCLICommandWithExecer(ctx context.Context, cluster *clusters.Cluster, gateway gateway.Gateway, execer dbcmd.Execer, authClient authclient.ClientI) (Cmds, error) {
	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName(),
		Protocol:    gateway.Protocol(),
		Username:    gateway.TargetUser(),
		Database:    gateway.TargetSubresourceName(),
	}

	var getDatabaseOnce sync.Once
	var getDatabaseError error
	var database types.Database

	opts := []dbcmd.ConnectCommandFunc{
		dbcmd.WithLogger(gateway.Log()),
		dbcmd.WithLocalProxy(gateway.LocalAddress(), gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithTolerateMissingCLIClient(),
		dbcmd.WithExecer(execer),
		dbcmd.WithOracleOpts(true /* can use TCP */, true /* has TCP servers */),
		dbcmd.WithGetDatabaseFunc(func(ctx context.Context, _ *client.TeleportClient, _ string) (types.Database, error) {
			getDatabaseOnce.Do(func() {
				database, getDatabaseError = cluster.GetDatabase(ctx, authClient, gateway.TargetURI())
			})
			return database, trace.Wrap(getDatabaseError)
		}),
	}

	switch gateway.Protocol() {
	case defaults.ProtocolDynamoDB, defaults.ProtocolSpanner:
		// DynamoDB doesn't support non-print-format use.
		// Spanner does, but it's not supported in Teleterm yet.
		// TODO(gavin): get the database GCP metadata to enable spanner-cli in
		// Teleterm.
		opts = append(opts, dbcmd.WithPrintFormat())
	}

	previewOpts := append(opts, dbcmd.WithPrintFormat())

	execCmd, err := clusters.NewDBCLICmdBuilder(cluster, routeToDb, opts...).GetConnectCommand(ctx)
	if err != nil {
		return Cmds{}, trace.Wrap(err)
	}

	previewCmd, err := clusters.NewDBCLICmdBuilder(cluster, routeToDb, previewOpts...).GetConnectCommand(ctx)
	if err != nil {
		return Cmds{}, trace.Wrap(err)
	}

	cmds := Cmds{
		Exec:    execCmd,
		Preview: previewCmd,
	}

	return cmds, nil
}
