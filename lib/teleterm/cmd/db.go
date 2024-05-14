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
	"os/exec"

	"github.com/gravitational/trace"

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
func NewDBCLICommand(cluster *clusters.Cluster, gateway gateway.Gateway) (Cmds, error) {
	cmds, err := newDBCLICommandWithExecer(cluster, gateway, dbcmd.SystemExecer{})
	return cmds, trace.Wrap(err)
}

func newDBCLICommandWithExecer(cluster *clusters.Cluster, gateway gateway.Gateway, execer dbcmd.Execer) (Cmds, error) {
	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName(),
		Protocol:    gateway.Protocol(),
		Username:    gateway.TargetUser(),
		Database:    gateway.TargetSubresourceName(),
	}

	opts := []dbcmd.ConnectCommandFunc{
		dbcmd.WithLogger(gateway.Log()),
		dbcmd.WithLocalProxy(gateway.LocalAddress(), gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithTolerateMissingCLIClient(),
		dbcmd.WithExecer(execer),
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

	execCmd, err := clusters.NewDBCLICmdBuilder(cluster, routeToDb, opts...).GetConnectCommand()
	if err != nil {
		return Cmds{}, trace.Wrap(err)
	}

	previewCmd, err := clusters.NewDBCLICmdBuilder(cluster, routeToDb, previewOpts...).GetConnectCommand()
	if err != nil {
		return Cmds{}, trace.Wrap(err)
	}

	cmds := Cmds{
		Exec:    execCmd,
		Preview: previewCmd,
	}

	return cmds, nil
}
