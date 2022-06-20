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
	apiuri "github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

// DbcmdCLICommandProvider provides CLI commands for database gateways. It needs Storage to read
// fresh profile state from the disk.
type DbcmdCLICommandProvider struct {
	storage *Storage
}

func NewDbcmdCLICommandProvider(storage *Storage) DbcmdCLICommandProvider {
	return DbcmdCLICommandProvider{
		storage: storage,
	}
}

func (d DbcmdCLICommandProvider) GetCommand(gateway *gateway.Gateway) (*string, error) {
	cluster, err := d.resolveCluster(gateway.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName,
		Protocol:    gateway.Protocol,
		Username:    gateway.TargetUser,
		Database:    gateway.TargetSubresourceName,
	}

	cmd, err := dbcmd.NewCmdBuilder(cluster.clusterClient, &cluster.status, &routeToDb,
		// TODO(ravicious): Pass the root cluster name here. GetActualName returns leaf name for leaf
		// clusters.
		//
		// At this point it doesn't matter though because this argument is used only for
		// generating correct CA paths. We use dbcmd.WithNoTLS here which means that the CA paths aren't
		// included in the returned CLI command.
		cluster.GetActualName(),
		dbcmd.WithLogger(gateway.Log),
		dbcmd.WithLocalProxy(gateway.LocalAddress, gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithTolerateMissingCLIClient(),
	).GetConnectCommandNoAbsPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmdString := strings.TrimSpace(fmt.Sprintf("%s %s", strings.Join(cmd.Env, " "), cmd.String()))

	return &cmdString, nil
}

func (d DbcmdCLICommandProvider) resolveCluster(uri string) (*Cluster, error) {
	clusterURI, err := apiuri.ParseClusterURI(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := d.storage.GetByURI(clusterURI.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster, nil
}
