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

package clusters

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

// DbcmdCLICommandProvider provides CLI commands for database gateways. It needs Storage to read
// fresh profile state from the disk.
type DbcmdCLICommandProvider struct {
	storage StorageByResourceURI
	execer  dbcmd.Execer
}

type StorageByResourceURI interface {
	GetByResourceURI(string) (*Cluster, error)
}

func NewDbcmdCLICommandProvider(storage StorageByResourceURI, execer dbcmd.Execer) DbcmdCLICommandProvider {
	return DbcmdCLICommandProvider{
		storage: storage,
		execer:  execer,
	}
}

func (d DbcmdCLICommandProvider) GetCommand(gateway *gateway.Gateway) (string, error) {
	cluster, err := d.storage.GetByResourceURI(gateway.TargetURI())
	if err != nil {
		return "", trace.Wrap(err)
	}

	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName(),
		Protocol:    gateway.Protocol(),
		Username:    gateway.TargetUser(),
		Database:    gateway.TargetSubresourceName(),
	}

	cmd, err := dbcmd.NewCmdBuilder(cluster.clusterClient, &cluster.status, &routeToDb,
		// TODO(ravicious): Pass the root cluster name here. GetActualName returns leaf name for leaf
		// clusters.
		//
		// At this point it doesn't matter though because this argument is used only for
		// generating correct CA paths. We use dbcmd.WithNoTLS here which means that the CA paths aren't
		// included in the returned CLI command.
		cluster.GetActualName(),
		dbcmd.WithLogger(gateway.Log()),
		dbcmd.WithLocalProxy(gateway.LocalAddress(), gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithPrintFormat(),
		dbcmd.WithTolerateMissingCLIClient(),
		dbcmd.WithExecer(d.execer),
	).GetConnectCommandNoAbsPath()
	if err != nil {
		return "", trace.Wrap(err)
	}

	cmdString := strings.TrimSpace(fmt.Sprintf("%s %s", strings.Join(cmd.Env, " "), cmd.String()))

	return cmdString, nil
}
