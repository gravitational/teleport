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
	"crypto/tls"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

// ReissueCertFunc is a callback function for Cluster to actually do the issue
// of user certificates with TeleportClient.
type ReissueCertFunc func(context.Context) error

// GatewayCertReissuer defines an interface of a helper that manages the
// process of reissuing certificates.
type GatewayCertReissuer interface {
	ReissueCert(ctx context.Context, gateway *gateway.Gateway, doReissueCert ReissueCertFunc) error
}

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
	CertReissuer       GatewayCertReissuer
}

// CreateGateway creates a gateway
func (c *Cluster) CreateGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	if params.CLICommandProvider == nil {
		params.CLICommandProvider = NewDbcmdCLICommandProvider(c, dbcmd.SystemExecer{})
	}

	db, err := c.GetDatabase(ctx, params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToDatabase := tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    params.TargetUser,
	}

	tlsCert, err := c.reissueAndLoadDBCert(ctx, routeToDatabase)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetUser:                    params.TargetUser,
		TargetName:                    db.GetName(),
		TargetSubresourceName:         params.TargetSubresourceName,
		Cert:                          tlsCert,
		Protocol:                      db.GetProtocol(),
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Log:                           c.Log,
		CLICommandProvider:            params.CLICommandProvider,
		TCPPortAllocator:              gateway.NetTCPPortAllocator{},
		ReissueCert:                   c.makeGatewayReissueDBCertFunc(params.CertReissuer, routeToDatabase),
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return gw, nil
}

// makeGatewayReissueDBCertFunc creates a gateway.ReissueCertFunc that reissues
// the database certificate using provided GatewayCertReissuer, then loads the
// certificate.
func (c *Cluster) makeGatewayReissueDBCertFunc(certReissuer GatewayCertReissuer, routeToDatabase tlsca.RouteToDatabase) gateway.ReissueCertFunc {
	return func(ctx context.Context, gateway *gateway.Gateway) (tls.Certificate, error) {
		err := certReissuer.ReissueCert(ctx, gateway, func(ctx context.Context) error {
			return trace.Wrap(c.reissueDBCerts(ctx, routeToDatabase))
		})
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}

		tlsCert, err := c.loadDBCert(routeToDatabase)
		return tlsCert, trace.Wrap(err)
	}
}
