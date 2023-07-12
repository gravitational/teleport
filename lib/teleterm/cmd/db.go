// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os/exec"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

// DbcmdCLICommandProvider provides CLI commands for database gateways. It needs Storage to read
// fresh profile state from the disk.
type DbcmdCLICommandProvider struct {
	storage StorageByResourceURI
	execer  dbcmd.Execer
}

func NewDbcmdCLICommandProvider(storage StorageByResourceURI, execer dbcmd.Execer) DbcmdCLICommandProvider {
	return DbcmdCLICommandProvider{
		storage: storage,
		execer:  execer,
	}
}

func (d DbcmdCLICommandProvider) GetCommand(gateway gateway.GatewayReader) (*exec.Cmd, error) {
	cluster, err := d.storage.GetByResourceURI(gateway.TargetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName(),
		Protocol:    gateway.Protocol(),
		Username:    gateway.TargetUser(),
		Database:    gateway.TargetSubresourceName(),
	}

	cmd, err := clusters.NewDbcmdCLICmdBuilder(cluster, routeToDb,
		dbcmd.WithLogger(gateway.Log()),
		dbcmd.WithLocalProxy(gateway.LocalAddress(), gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithPrintFormat(),
		dbcmd.WithTolerateMissingCLIClient(),
		dbcmd.WithExecer(d.execer),
	).GetConnectCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}
