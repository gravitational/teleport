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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
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
	LocalPort          string
	CLICommandProvider gateway.CLICommandProvider
	TCPPortAllocator   gateway.TCPPortAllocator
	OnExpiredCert      gateway.OnExpiredCertFunc
}

// CreateGateway creates a gateway
func (c *Cluster) CreateGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	switch {
	case uri.IsDB(params.TargetURI):
		gw, err := c.createDatabaseGateway(ctx, params)
		return gw, trace.Wrap(err)

	case uri.IsKube(params.TargetURI):
		gw, err := c.createKubeGateway(ctx, params)
		return gw, trace.Wrap(err)

	default:
		return nil, trace.NotImplemented("gateway not supported for %v", params.TargetURI)
	}
}

func (c *Cluster) createDatabaseGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	db, err := c.GetDatabase(ctx, params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToDatabase := tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    params.TargetUser,
	}

	if err := c.ReissueDBCerts(ctx, routeToDatabase); err != nil {
		return nil, trace.Wrap(err)
	}

	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetUser:                    params.TargetUser,
		TargetName:                    db.GetName(),
		TargetSubresourceName:         params.TargetSubresourceName,
		Protocol:                      db.GetProtocol(),
		KeyPath:                       c.status.KeyPath(),
		CertPath:                      c.status.DatabaseCertPathForCluster(c.clusterClient.SiteName, db.GetName()),
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		ClusterName:                   c.Name,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Log:                           c.Log,
		CLICommandProvider:            params.CLICommandProvider,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return gw, nil
}

func (c *Cluster) createKubeGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	kubeCluster := uri.New(params.TargetURI).GetKubeName()
	if kubeCluster == "" {
		return nil, trace.BadParameter("invalid TargetURI %v for Kube gateway", params.TargetURI)
	}

	if err := c.ReissueKubeCerts(ctx, kubeCluster); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO support TargetUser (--as), TargetGroups (--as-groups), TargetSubresourceName (--kube-namespace)
	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetName:                    kubeCluster,
		KeyPath:                       c.status.KeyPath(),
		CertPath:                      c.status.KubeCertPath(kubeCluster),
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		ClusterName:                   c.Name,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Log:                           c.Log,
		CLICommandProvider:            params.CLICommandProvider,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
	})
	return gw, trace.Wrap(err)
}
