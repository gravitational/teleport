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

	"github.com/gravitational/trace"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"
)

// setupSSO sets up SAML based SSO to Teleport for the given application (service principal).
func setupSSO(ctx context.Context, graphClient *msgraphsdk.GraphServiceClient, appObjectID string, spID string, acsURL string) error {
	spPatch := models.NewServicePrincipal()
	// Set service principal to prefer SAML sign on
	preferredSingleSignOnMode := "saml"
	spPatch.SetPreferredSingleSignOnMode(&preferredSingleSignOnMode)
	// Do not require explicit assignment of the app to use SSO.
	// This is per our manual set-up recommendations, see https://goteleport.com/docs/access-controls/sso/azuread/ .
	appRoleAssignmentRequired := false
	spPatch.SetAppRoleAssignmentRequired(&appRoleAssignmentRequired)

	_, err := graphClient.ServicePrincipals().
		ByServicePrincipalId(spID).
		Patch(ctx, spPatch, nil)

	if err != nil {
		return trace.Wrap(err, "failed to enable SSO for service principal")
	}

	// Add SAML urls
	app := models.NewApplication()
	app.SetIdentifierUris([]string{acsURL})
	webApp := models.NewWebApplication()
	webApp.SetRedirectUris([]string{acsURL})
	app.SetWeb(webApp)

	_, err = graphClient.Applications().
		ByApplicationId(appObjectID).
		Patch(ctx, app, nil)

	if err != nil {
		return trace.Wrap(err, "failed to set SAML URIs")
	}

	// Add a SAML signing certificate
	certRequest := serviceprincipals.NewItemAddTokenSigningCertificatePostRequestBody()
	// Display name is required to start with `CN=`.
	// Ref: https://learn.microsoft.com/en-us/graph/api/serviceprincipal-addtokensigningcertificate
	displayName := "CN=azure-sso"
	certRequest.SetDisplayName(&displayName)

	cert, err := graphClient.ServicePrincipals().
		ByServicePrincipalId(spID).
		AddTokenSigningCertificate().
		Post(ctx, certRequest, nil)

	if err != nil {
		trace.Wrap(err, "failed to create a signing certificate")
	}

	// Set the preferred SAML signing key
	spPatch = models.NewServicePrincipal()
	spPatch.SetPreferredTokenSigningKeyThumbprint(cert.GetThumbprint())

	_, err = graphClient.ServicePrincipals().
		ByServicePrincipalId(spID).
		Patch(ctx, spPatch, nil)

	if err != nil {
		return trace.Wrap(err, "failed to set SAML signing key")
	}

	return nil
}
