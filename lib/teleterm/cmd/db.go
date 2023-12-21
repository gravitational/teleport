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
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

// NewDBCLICommand creates CLI commands for database gateway.
func NewDBCLICommand(cluster *clusters.Cluster, gateway gateway.Gateway) (*exec.Cmd, error) {
	cmd, err := newDBCLICommandWithExecer(cluster, gateway, dbcmd.SystemExecer{})
	return cmd, trace.Wrap(err)
}

func newDBCLICommandWithExecer(cluster *clusters.Cluster, gateway gateway.Gateway, execer dbcmd.Execer) (*exec.Cmd, error) {
	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName(),
		Protocol:    gateway.Protocol(),
		Username:    gateway.TargetUser(),
		Database:    gateway.TargetSubresourceName(),
	}

	cmd, err := clusters.NewDBCLICmdBuilder(cluster, routeToDb,
		dbcmd.WithLogger(gateway.Log()),
		dbcmd.WithLocalProxy(gateway.LocalAddress(), gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithPrintFormat(),
		dbcmd.WithTolerateMissingCLIClient(),
		dbcmd.WithExecer(execer),
	).GetConnectCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}
