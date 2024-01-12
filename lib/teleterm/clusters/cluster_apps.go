// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clusters

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// App describes an app resource.
type App struct {
	// URI is the app URI
	URI uri.ResourceURI

	App types.Application
}

// SAMLIdPServiceProvider describes a SAML IdP resource.
type SAMLIdPServiceProvider struct {
	// URI is the app URI
	URI uri.ResourceURI

	Provider types.SAMLIdPServiceProvider
}

// GetApps returns a paginated apps list
func (c *Cluster) GetApps(ctx context.Context, r *api.GetAppsRequest) (*GetAppsResponse, error) {
	var (
		page        apiclient.ResourcePage[types.AppServerOrSAMLIdPServiceProvider]
		authClient  auth.ClientI
		proxyClient *client.ProxyClient
		err         error
	)

	req := &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindAppOrSAMLIdPServiceProvider,
		Limit:               r.Limit,
		SortBy:              types.GetSortByFromString(r.SortBy),
		StartKey:            r.StartKey,
		PredicateExpression: r.Query,
		SearchKeywords:      client.ParseSearchKeywords(r.Search, ' '),
		UseSearchAsRoles:    r.SearchAsRoles == "yes",
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		page, err = apiclient.GetResourcePage[types.AppServerOrSAMLIdPServiceProvider](ctx, authClient, req)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]AppServerOrSAMLIdPServiceProvider, 0, len(page.Resources))
	for _, appServerOrProvider := range page.Resources {
		if appServerOrProvider.IsAppServer() {
			app := appServerOrProvider.GetAppServer().GetApp()
			results = append(results, AppServerOrSAMLIdPServiceProvider{App: &App{
				URI: c.URI.AppendApp(app.GetName()),
				App: app,
			}})
		} else {
			provider := appServerOrProvider.GetSAMLIdPServiceProvider()
			results = append(results, AppServerOrSAMLIdPServiceProvider{SAMLIdPServiceProvider: &SAMLIdPServiceProvider{
				URI:      c.URI.AppendApp(provider.GetName()),
				Provider: provider,
			}})
		}
	}

	return &GetAppsResponse{
		Apps:       results,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

type AppServerOrSAMLIdPServiceProvider struct {
	App                    *App
	SAMLIdPServiceProvider *SAMLIdPServiceProvider
}

type GetAppsResponse struct {
	Apps []AppServerOrSAMLIdPServiceProvider
	// StartKey is the next key to use as a starting point.
	StartKey string
	// // TotalCount is the total number of resources available as a whole.
	TotalCount int
}

func (c *Cluster) getApp(ctx context.Context, appName string) (types.Application, error) {
	var app types.Application
	err := AddMetadataToRetryableError(ctx, func() error {
		apps, err := c.clusterClient.ListApps(ctx, &proto.ListResourcesRequest{
			Namespace:           c.clusterClient.Namespace,
			ResourceType:        types.KindAppServer,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, appName),
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if len(apps) == 0 {
			return trace.NotFound("app %q not found", appName)
		}

		app = apps[0]
		return nil
	})

	return app, trace.Wrap(err)
}

// reissueAppCert issue new certificates for the app and saves them to disk.
func (c *Cluster) reissueAppCert(ctx context.Context, app types.Application) (tls.Certificate, error) {
	if app.IsAWSConsole() || app.IsGCP() || app.IsAzureCloud() {
		return tls.Certificate{}, trace.BadParameter("cloud applications are not supported")
	}
	// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
	err := c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		AccessRequests: c.status.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	defer proxyClient.Close()

	request := types.CreateAppSessionRequest{
		Username:          c.status.Username,
		PublicAddr:        app.GetPublicAddr(),
		ClusterName:       c.clusterClient.SiteName,
		AWSRoleARN:        "",
		AzureIdentity:     "",
		GCPServiceAccount: "",
	}

	ws, err := proxyClient.CreateAppSession(ctx, request)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	err = proxyClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		RouteToApp: proto.RouteToApp{
			Name:              app.GetName(),
			SessionID:         ws.GetName(),
			PublicAddr:        app.GetPublicAddr(),
			ClusterName:       c.clusterClient.SiteName,
			AWSRoleARN:        "",
			AzureIdentity:     "",
			GCPServiceAccount: "",
		},
		AccessRequests: c.status.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	key, err := c.clusterClient.LocalAgent().GetKey(c.clusterClient.SiteName, libclient.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, ok := key.AppTLSCerts[app.GetName()]
	if !ok {
		return tls.Certificate{}, trace.NotFound("the user is not logged in into the application %v", app.GetName())
	}

	tlsCert, err := key.TLSCertificate(cert)
	return tlsCert, trace.Wrap(err)
}
