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
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws"
)

// App describes an app resource.
type App struct {
	// URI is the app URI
	URI uri.ResourceURI
	// FQDN is the hostname under which the app is accessible within the root cluster.
	// It is included in this struct because the callsite which constructs FQDN must have access to
	// clusters.Cluster.
	FQDN string
	// AWSRoles is a list of AWS IAM roles for the application representing AWS console.
	AWSRoles aws.Roles

	App types.Application
}

// SAMLIdPServiceProvider describes a SAML IdP resource.
type SAMLIdPServiceProvider struct {
	// URI is the app URI
	URI uri.ResourceURI

	Provider types.SAMLIdPServiceProvider
}

func GetApp(ctx context.Context, authClient authclient.ClientI, appName string) (types.Application, error) {
	var app types.Application
	err := AddMetadataToRetryableError(ctx, func() error {
		apps, err := apiclient.GetAllResources[types.AppServer](ctx, authClient, &proto.ListResourcesRequest{
			Namespace:           apidefaults.Namespace,
			ResourceType:        types.KindAppServer,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, appName),
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if len(apps) == 0 {
			return trace.NotFound("app %q not found", appName)
		}

		app = apps[0].GetApp()
		return nil
	})

	return app, trace.Wrap(err)
}

// ReissueAppCert issue new certificates for the app and saves them to disk.
func (c *Cluster) ReissueAppCert(ctx context.Context, clusterClient *client.ClusterClient, routeToApp proto.RouteToApp) (tls.Certificate, error) {
	// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
	err := clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		AccessRequests: c.status.ActiveRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	keyRing, _, err := clusterClient.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		RouteToApp:     routeToApp,
		AccessRequests: c.status.ActiveRequests,
		RequesterName:  proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
		TTL:            c.clusterClient.KeyTTL,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	appCert, err := keyRing.AppTLSCert(routeToApp.Name)
	return appCert, trace.Wrap(err)
}

// AssembleAppFQDN is a wrapper on top of [utils.AssembleAppFQDN] which encapsulates translation
// between lib/teleterm and lib/web terminology.
//
// It assumes that app was fetched from c, as there's no way to check that in runtime.
func (c *Cluster) AssembleAppFQDN(app types.Application) string {
	// "local" in the context of the Web UI means "belonging to the cluster of this proxy service".
	// If you're looking at leaf resources in the Web UI, you're doing this through the Web UI of the
	// root cluster, so "local cluster" in this case is the root cluster.
	//
	// In case of lib/teleterm, clusters.Cluster can represent either a root cluster or a leaf
	// cluster. Variables prefixed with "local" are set to values associated with the root cluster.
	//
	// ProfileName is the same as the proxy hostname, as it's the name that tsh uses to store files
	// associated with the profile in ~/tsh. Technically, ProfileName is not necessarily the same as
	// the cluster name. However, localClusterName is used by utils.AssembleAppFQDN merely to
	// differentiate between leaf and root cluster apps.
	localClusterName := c.ProfileName
	localProxyDNSName := c.GetProxyHostname()
	// Since utils.AssembleAppFQDN uses localClusterName and appClusterName to differentiate between
	// root and local apps, appClusterName is set to ProfileName so that appClusterName equals
	// localClusterName for root cluster apps.
	appClusterName := c.ProfileName

	leafClusterName := c.URI.GetLeafClusterName()
	if leafClusterName != "" {
		appClusterName = leafClusterName
	}

	return utils.AssembleAppFQDN(localClusterName, localProxyDNSName, appClusterName, app)
}

// GetAWSRoles returns a list of allowed AWS role ARNs user can assume,
// associated with the app's AWS account ID.
func (c *Cluster) GetAWSRoles(app types.Application) aws.Roles {
	if app.IsAWSConsole() {
		return aws.FilterAWSRoles(c.GetAWSRolesARNs(), app.GetAWSAccountID())
	}
	return aws.Roles{}
}

// ValidateTargetPort parses rawTargetPort to uint32 and checks if it's included in TCP ports of app.
// It also returns an error if app doesn't have any TCP ports defined.
func ValidateTargetPort(app types.Application, rawTargetPort string) (uint32, error) {
	if rawTargetPort == "" {
		return 0, nil
	}

	targetPort, err := parseTargetPort(rawTargetPort)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	tcpPorts := app.GetTCPPorts()
	if len(tcpPorts) == 0 {
		return 0, trace.BadParameter("cannot specify target port %d because app %s does not provide access to multiple ports",
			targetPort, app.GetName())
	}

	if !tcpPorts.Contains(int(targetPort)) {
		return 0, trace.BadParameter("port %d is not included in target ports of app %s",
			targetPort, app.GetName())
	}

	return targetPort, nil
}
