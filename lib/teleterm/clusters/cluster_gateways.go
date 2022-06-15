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

	"github.com/gravitational/teleport/lib/teleterm/gateway"

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

	if err := c.ReissueDBCerts(ctx, params.TargetUser, db); err != nil {
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

	cliCommand, err := c.cliCommandProvider.GetCommand(c, gw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gw.CLICommand = *cliCommand

	return gw, nil
}

// SetGatewayTargetSubresourceName sets the target subresource name and updates the CLI command, as
// changes to the target subresource name might impact the command.
func (c *Cluster) SetGatewayTargetSubresourceName(gateway *gateway.Gateway, targetSubresourceName string) error {
	gateway.TargetSubresourceName = targetSubresourceName

	cliCommand, err := c.cliCommandProvider.GetCommand(c, gateway)
	if err != nil {
		return trace.Wrap(err)
	}

	gateway.CLICommand = *cliCommand

	return nil
}
