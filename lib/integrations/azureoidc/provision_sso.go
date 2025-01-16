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

	"github.com/gravitational/teleport/lib/msgraph"
)

// setupSSO sets up SAML based SSO to Teleport for the given application (service principal).
func setupSSO(ctx context.Context, graphClient *msgraph.Client, appObjectID string, spID string, acsURL string) error {
	spPatch := &msgraph.ServicePrincipal{}
	// Set service principal to prefer SAML sign on
	preferredSingleSignOnMode := "saml"
	spPatch.PreferredSingleSignOnMode = &preferredSingleSignOnMode
	// Do not require explicit assignment of the app to use SSO.
	// This is per our manual set-up recommendations, see
	// https://goteleport.com/docs/admin-guides/access-controls/sso/azuread/ .
	appRoleAssignmentRequired := false
	spPatch.AppRoleAssignmentRequired = &appRoleAssignmentRequired

	err := graphClient.UpdateServicePrincipal(ctx, spID, spPatch)

	if err != nil {
		return trace.Wrap(err, "failed to enable SSO for service principal")
	}

	// Add SAML urls
	app := &msgraph.Application{}
	uris := []string{acsURL}
	app.IdentifierURIs = &uris
	webApp := &msgraph.WebApplication{}
	webApp.RedirectURIs = &uris
	app.Web = webApp
	securityGroups := new(string)
	*securityGroups = "SecurityGroup"
	app.GroupMembershipClaims = securityGroups

	claimName := "groups"
	optionalClaim := []msgraph.OptionalClaim{
		{
			Name: &claimName,
		},
	}
	app.OptionalClaims = &msgraph.OptionalClaims{
		IDToken:     optionalClaim,
		SAML2Token:  optionalClaim,
		AccessToken: optionalClaim,
	}

	err = graphClient.UpdateApplication(ctx, appObjectID, app)

	if err != nil {
		return trace.Wrap(err, "failed to set SAML URIs")
	}

	// Add a SAML signing certificate
	// Display name is required to start with `CN=`.
	// Ref: https://learn.microsoft.com/en-us/graph/api/serviceprincipal-addtokensigningcertificate
	const displayName = "CN=azure-sso"
	cert, err := graphClient.CreateServicePrincipalTokenSigningCertificate(ctx, spID, displayName)

	if err != nil {
		trace.Wrap(err, "failed to create a signing certificate")
	}

	// Set the preferred SAML signing key
	spPatch = &msgraph.ServicePrincipal{}
	spPatch.PreferredTokenSigningKeyThumbprint = cert.Thumbprint

	err = graphClient.UpdateServicePrincipal(ctx, spID, spPatch)
	if err != nil {
		return trace.Wrap(err, "failed to set SAML signing key")
	}

	return nil
}
