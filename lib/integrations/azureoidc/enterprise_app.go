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

package azureoidc

import (
	"context"
	"log/slog"
	"net/url"
	"path"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/msgraph"
)

// nonGalleryAppTemplateID is a constant for the special application template ID in Microsoft Graph,
// equivalent to the "create your own application" option in Azure portal.
// Only non-gallery apps ("Create your own application" option in the UI) are allowed to use SAML SSO,
// hence we use this template.
// Ref: https://learn.microsoft.com/en-us/graph/api/applicationtemplate-instantiate
const nonGalleryAppTemplateID = "8adf8e6e-67b2-4cf2-a259-e3dc5476c621"

// A list of Microsoft Graph permissions ("app roles") for directory sync as performed by the Entra ID plugin.
// Ref: https://learn.microsoft.com/en-us/graph/permissions-reference
var appRoles = []string{
	// Application.Read.All
	"9a5d68dd-52b0-4cc2-bd40-abcf44ac3a30",
	// Directory.Read.All
	"7ab1d382-f21e-4acd-a863-ba3e13f7da61",
	// Policy.Read.All
	"246dd0d5-5bd0-4def-940b-0421030a5b68",
}

// SetupEnterpriseApp sets up an Enterprise Application in the Entra ID directory.
// The enterprise application:
//   - Provides Teleport with OIDC authentication to Azure
//   - Is given the permissions to access certain Microsoft Graph API endpoints for this tenant.
//   - Provides SSO to the Teleport cluster via SAML.
func SetupEnterpriseApp(ctx context.Context, proxyPublicAddr string, authConnectorName string, skipOIDCSetup bool) (string, string, error) {
	var appID, tenantID string

	tenantID, err := getTenantID()
	if err != nil {
		return appID, tenantID, trace.Wrap(err)
	}

	graphClient, err := createGraphClient()
	if err != nil {
		return appID, tenantID, trace.Wrap(err)
	}

	proxyURL, err := url.Parse(proxyPublicAddr)
	if err != nil {
		return appID, tenantID, trace.Wrap(err, "could not parse URL of the Proxy Service")
	}

	displayName := "Teleport" + " " + proxyURL.Hostname()

	appAndSP, err := graphClient.InstantiateApplicationTemplate(ctx, nonGalleryAppTemplateID, displayName)
	if err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to instantiate application template")
	}

	app := appAndSP.Application
	sp := appAndSP.ServicePrincipal
	appID = *app.AppID
	spID := *sp.ID

	msGraphResourceID, err := getMSGraphResourceID(ctx, graphClient)
	if err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to get MS Graph API resource ID")
	}

	r, err := retry()
	if err != nil {
		return appID, tenantID, trace.Wrap(err)
	}

	const maxRetries = 10
	for _, appRoleID := range appRoles {
		r.Reset()
		var err error

		assignment := &msgraph.AppRoleAssignment{}
		assignment.PrincipalID = &spID
		assignment.ResourceID = &msGraphResourceID
		assignment.AppRoleID = &appRoleID

		// There are  some eventual consistency shenanigans instantiating enteprise applications,
		// where assigning app roles may temporarily return "not found" for the newly-created App ID.
		// Retry a few times to remediate.
		for i := 0; i < maxRetries; i++ {
			slog.DebugContext(ctx, "assign app role", "role_id", appRoleID, "attempt", i)
			_, err = graphClient.GrantAppRoleToServicePrincipal(ctx, spID, assignment)
			if err != nil {
				r.Inc()
				<-r.After()
			} else {
				break
			}
		}
		if err != nil {
			return appID, tenantID, trace.Wrap(err, "failed to assign app role %s", appRoleID)
		}
	}

	// Skip OIDC setup if requested.
	// This is useful for clusters that can't use OIDC because they are not reachable from the public internet.
	if !skipOIDCSetup {
		if err := createFederatedAuthCredential(ctx, graphClient, *app.ID, proxyPublicAddr); err != nil {
			return appID, tenantID, trace.Wrap(err, "failed to create an OIDC federated auth credential")
		}
	}

	acsURL, err := url.Parse(proxyPublicAddr)
	if err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to parse proxy public address")
	}
	acsURL.Path = path.Join("/v1/webapi/saml/acs", authConnectorName)
	if err := setupSSO(ctx, graphClient, *app.ID, spID, acsURL.String()); err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to set up SSO for the enterprise app")
	}

	return appID, tenantID, nil
}

// createFederatedAuthCredential creates a new federated (OIDC) auth credential for the given Entra application.
func createFederatedAuthCredential(ctx context.Context, graphClient *msgraph.Client, appObjectID string, proxyPublicAddr string) error {
	credential := &msgraph.FederatedIdentityCredential{}
	name := "teleport-oidc"
	audiences := []string{azureDefaultJWTAudience}
	subject := azureSubject
	credential.Name = &name
	credential.Issuer = &proxyPublicAddr
	credential.Audiences = &audiences
	credential.Subject = &subject

	// ByApplicationID here means the object ID,
	// i.e. app.ID, not app.AppID.
	_, err := graphClient.CreateFederatedIdentityCredential(ctx, appObjectID, credential)

	return trace.Wrap(err)

}

// getMSGraphResourceID gets the resource ID for the Microsoft Graph app in the Entra directory.
func getMSGraphResourceID(ctx context.Context, graphClient *msgraph.Client) (string, error) {
	const displayName = "Microsoft Graph"

	spList, err := graphClient.GetServicePrincipalsByDisplayName(ctx, displayName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	switch len(spList) {
	case 0:
		return "", trace.NotFound("Microsoft Graph app not found in the tenant")
	case 1:
		return *spList[0].ID, nil
	default:
		return "", trace.BadParameter("Multiple service principals found for Microsoft Graph. This is not expected.")
	}

}

func retry() (*retryutils.RetryV2, error) {
	r, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  time.Second,
		Max:    10 * time.Second,
		Driver: retryutils.NewExponentialDriver(time.Second),
	})
	return r, trace.Wrap(err)
}
