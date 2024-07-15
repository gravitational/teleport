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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applicationtemplates"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"

	"github.com/gravitational/teleport/api/utils/retryutils"
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
func SetupEnterpriseApp(ctx context.Context, proxyPublicAddr string, authConnectorName string) (string, string, error) {
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

	instantiateRequest := applicationtemplates.NewItemInstantiatePostRequestBody()
	instantiateRequest.SetDisplayName(&displayName)
	appAndSP, err := graphClient.ApplicationTemplates().
		ByApplicationTemplateId(nonGalleryAppTemplateID).
		Instantiate().
		Post(ctx, instantiateRequest, nil)

	if err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to instantiate application template")
	}

	app := appAndSP.GetApplication()
	sp := appAndSP.GetServicePrincipal()
	appID = *app.GetAppId()
	spID := *sp.GetId()

	msGraphResourceID, err := getMSGraphResourceID(ctx, graphClient)
	if err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to get MS Graph API resource ID")
	}

	msGraphResourceUUID := uuid.MustParse(msGraphResourceID)

	r, err := retry()
	if err != nil {
		return appID, tenantID, trace.Wrap(err)
	}

	const maxRetries = 10
	for _, appRoleID := range appRoles {
		r.Reset()
		var err error

		assignment := models.NewAppRoleAssignment()
		spUUID := uuid.MustParse(spID)
		assignment.SetPrincipalId(&spUUID)
		assignment.SetResourceId(&msGraphResourceUUID)
		appRoleUUID := uuid.MustParse(appRoleID)
		assignment.SetAppRoleId(&appRoleUUID)

		// There are  some eventual consistency shenanigans instantiating enteprise applications,
		// where assigning app roles may temporarily return "not found" for the newly-created App ID.
		// Retry a few times to remediate.
		for i := 0; i < maxRetries; i++ {
			slog.DebugContext(ctx, "assign app role", "role_id", appRoleID, "attempt", i)
			_, err = graphClient.ServicePrincipals().
				ByServicePrincipalId(spID).
				AppRoleAssignments().
				Post(ctx, assignment, nil)
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

	if err := createFederatedAuthCredential(ctx, graphClient, *app.GetId(), proxyPublicAddr); err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to create an OIDC federated auth credential")
	}

	acsURL, err := url.Parse(proxyPublicAddr)
	if err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to parse proxy public address")
	}
	acsURL.Path = path.Join("/v1/webapi/saml/acs", authConnectorName)
	if err := setupSSO(ctx, graphClient, *app.GetId(), spID, acsURL.String()); err != nil {
		return appID, tenantID, trace.Wrap(err, "failed to set up SSO for the enterprise app")
	}

	return appID, tenantID, nil
}

// createFederatedAuthCredential creates a new federated (OIDC) auth credential for the given Entra application.
func createFederatedAuthCredential(ctx context.Context, graphClient *msgraphsdk.GraphServiceClient, appObjectID string, proxyPublicAddr string) error {
	credential := models.NewFederatedIdentityCredential()
	name := "teleport-oidc"
	audiences := []string{azureDefaultJWTAudience}
	subject := azureSubject
	credential.SetName(&name)
	credential.SetIssuer(&proxyPublicAddr)
	credential.SetAudiences(audiences)
	credential.SetSubject(&subject)

	// ByApplicationID here means the object ID,
	// i.e. app.GetId(), not app.GetAppId().
	_, err := graphClient.Applications().ByApplicationId(appObjectID).
		FederatedIdentityCredentials().Post(ctx, credential, nil)

	return trace.Wrap(err)

}

// getMSGraphResourceID gets the resource ID for the Microsoft Graph app in the Entra directory.
func getMSGraphResourceID(ctx context.Context, graphClient *msgraphsdk.GraphServiceClient) (string, error) {
	requestFilter := "displayName eq 'Microsoft Graph'"

	requestParameters := &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
		Filter: &requestFilter,
	}
	configuration := &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	spResponse, err := graphClient.ServicePrincipals().Get(ctx, configuration)
	if err != nil {
		return "", trace.Wrap(err)
	}

	spList := spResponse.GetValue()
	switch len(spList) {
	case 0:
		return "", trace.NotFound("Microsoft Graph app not found in the tenant")
	case 1:
		return *spList[0].GetId(), nil
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
