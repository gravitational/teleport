package common

import "github.com/gravitational/teleport/api/types"

const (
	// CredPurposeOktaOauth is used to store the OAuth client ID for Okta.
	CredPurposeOktaOauth = "okta-oauth-client-id"

	// OktaSSOConnectorName is the name of the Okta SSO connector.
	OktaSSOConnectorName = "okta"

	// OktaSSOConnectorDisplay is the display name of the Okta SSO connector.
	OktaSSOConnectorDisplay = "Okta"

	// OktaSCIMTokenName is the name of the Okta SCIM token.
	OktaSCIMTokenName = types.PluginTypeOkta + "-scim-token"
)
