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

package clusters

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

type CreateGatewayParams struct {
	// TargetURI is the cluster resource URI
	TargetURI uri.ResourceURI
	// TargetUser is the target user name
	TargetUser string
	// TargetSubresourceName points at a subresource of the remote resource, for example a database
	// name on a database server.
	TargetSubresourceName string
	// LocalPort is the gateway local port
	LocalPort            string
	TCPPortAllocator     gateway.TCPPortAllocator
	OnExpiredCert        gateway.OnExpiredCertFunc
	KubeconfigsDir       string
	MFAPromptConstructor func(cfg *libmfa.PromptConfig) mfa.Prompt
	ClusterClient        *client.ClusterClient
}

// CreateGateway creates a gateway
func (c *Cluster) CreateGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	c.clusterClient.MFAPromptConstructor = params.MFAPromptConstructor

	switch {
	case params.TargetURI.IsDB():
		gateway, err := c.createDBGateway(ctx, params)
		return gateway, trace.Wrap(err)

	case params.TargetURI.IsKube():
		gateway, err := c.createKubeGateway(ctx, params)
		return gateway, trace.Wrap(err)

	case params.TargetURI.IsApp():
		gateway, err := c.createAppGateway(ctx, params)
		return gateway, trace.Wrap(err)

	default:
		return nil, trace.NotImplemented("gateway not supported for %v", params.TargetURI)
	}
}

func (c *Cluster) createDBGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	db, err := c.GetDatabase(ctx, params.ClusterClient.AuthClient, params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToDatabase := tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    params.TargetUser,
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		return trace.Wrap(c.reissueDBCerts(ctx, params.ClusterClient, routeToDatabase))
	})
	if err != nil {
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
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Log:                           c.Log,
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

func (c *Cluster) createKubeGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	kube := params.TargetURI.GetKubeName()

	// Check if this kube exists and the user has access to it.
	if _, err := c.getKube(ctx, params.ClusterClient.AuthClient, kube); err != nil {
		return nil, trace.Wrap(err)
	}

	var cert tls.Certificate
	var err error

	if err := AddMetadataToRetryableError(ctx, func() error {
		cert, err = c.reissueKubeCert(ctx, params.ClusterClient, kube)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(ravicious): Support TargetUser (--as), TargetGroups (--as-groups), TargetSubresourceName (--kube-namespace).
	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetName:                    kube,
		Cert:                          cert,
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Log:                           c.Log,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
		ClusterName:                   c.Name,
		Username:                      c.status.Username,
		KubeconfigsDir:                params.KubeconfigsDir,
	})
	return gw, trace.Wrap(err)
}

func (c *Cluster) createAppGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	appName := params.TargetURI.GetAppName()

	app, err := c.getApp(ctx, params.ClusterClient.AuthClient, appName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cert tls.Certificate

	if err := AddMetadataToRetryableError(ctx, func() error {
		cert, err = c.reissueAppCert(ctx, params.ClusterClient, app)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	gw, err := gateway.New(gateway.Config{
		LocalPort:                     params.LocalPort,
		TargetURI:                     params.TargetURI,
		TargetName:                    appName,
		Cert:                          cert,
		Protocol:                      app.GetProtocol(),
		Insecure:                      c.clusterClient.InsecureSkipVerify,
		WebProxyAddr:                  c.clusterClient.WebProxyAddr,
		Log:                           c.Log,
		TCPPortAllocator:              params.TCPPortAllocator,
		OnExpiredCert:                 params.OnExpiredCert,
		Clock:                         c.clock,
		TLSRoutingConnUpgradeRequired: c.clusterClient.TLSRoutingConnUpgradeRequired,
		RootClusterCACertPoolFunc:     c.clusterClient.RootClusterCACertPool,
		ClusterName:                   c.Name,
		Username:                      c.status.Username,
	})
	return gw, trace.Wrap(err)
}

// ReissueGatewayCerts reissues certificate for the provided gateway.
//
// At the moment, kube gateways reload their certs in memory while db gateways use the old approach
// of saving a cert to disk and only then loading it to memory.
// TODO(ravicious): Refactor db gateways to reload cert in memory and support MFA.
func (c *Cluster) ReissueGatewayCerts(ctx context.Context, clusterClient *client.ClusterClient, g gateway.Gateway) (tls.Certificate, error) {
	switch {
	case g.TargetURI().IsDB():
		db, err := gateway.AsDatabase(g)
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
		err = c.reissueDBCerts(ctx, clusterClient, db.RouteToDatabase())
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}

		// db gateways still store certs on disk, so they need to load it after reissue.
		// Unlike other gateway types, the custom middleware for db proxies does not set the cert on the
		// local proxy.
		err = g.ReloadCert()
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}

		// Return an empty cert even if there is no error. DB gateways do not utilize certs returned
		// from ReissueGatewayCerts, at least not until we add support for MFA to them.
		return tls.Certificate{}, nil
	case g.TargetURI().IsKube():
		cert, err := c.reissueKubeCert(ctx, clusterClient, g.TargetName())
		return cert, trace.Wrap(err)
	case g.TargetURI().IsApp():
		appName := g.TargetURI().GetAppName()
		app, err := c.getApp(ctx, clusterClient.AuthClient, appName)
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}

		// The cert is returned from this function and finally set on LocalProxy by the middleware.
		cert, err := c.reissueAppCert(ctx, clusterClient, app)
		return cert, trace.Wrap(err)
	default:
		return tls.Certificate{}, trace.NotImplemented("ReissueGatewayCerts does not support this gateway kind %v", g.TargetURI().String())
	}
}
