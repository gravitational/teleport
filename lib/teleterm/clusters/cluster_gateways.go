/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clusters

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

type CreateGatewayParams struct {
	// TargetURI is the cluster resource URI
	TargetURI string
	// TargetUser is the target user name
	TargetUser string
	// TargetSubresourceName points at a subresource of the remote resource, for example a database
	// name on a database server.
	TargetSubresourceName string
	// LocalPort is the gateway local port
	LocalPort string
}

// CreateGateway creates a gateway
func (c *Cluster) CreateGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	db, err := c.GetDatabase(ctx, params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := c.ReissueDBCerts(ctx, params.TargetUser, params.TargetSubresourceName, db); err != nil {
		return nil, trace.Wrap(err)
	}

	gw, err := gateway.New(gateway.Config{
		LocalPort:             params.LocalPort,
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetName:            db.GetName(),
		TargetSubresourceName: params.TargetSubresourceName,
		Protocol:              db.GetProtocol(),
		KeyPath:               c.status.KeyPath(),
		CertPath:              c.status.DatabaseCertPathForCluster(c.clusterClient.SiteName, db.GetName()),
		Insecure:              c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:          c.clusterClient.WebProxyAddr,
		Log:                   c.Log.WithField("gateway", params.TargetURI),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cliCommand, err := buildCLICommand(c, gw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gw.CLICommand = fmt.Sprintf("%s %s", strings.Join(cliCommand.Env, " "), cliCommand.String())

	return gw, nil
}

func buildCLICommand(c *Cluster, gw *gateway.Gateway) (*exec.Cmd, error) {
	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gw.TargetName,
		Protocol:    gw.Protocol,
		Username:    gw.TargetUser,
		Database:    gw.TargetSubresourceName,
	}

	cmd, err := dbcmd.NewCmdBuilder(c.clusterClient, &c.status, &routeToDb,
		// TODO(ravicious): Pass the root cluster name here. GetActualName returns leaf name for leaf
		// clusters.
		//
		// At this point it doesn't matter though, because this argument is used only for
		// generating correct CA paths. But we use dbcmd.WithNoTLS here, which doesn't include CA paths
		// in the returned CLI command.
		c.GetActualName(),
		dbcmd.WithLogger(gw.Log),
		dbcmd.WithLocalProxy(gw.LocalAddress, gw.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithTolerateMissingCLIClient(),
	).GetConnectCommandNoAbsPath()

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}
