package common

import "github.com/gravitational/teleport/api/types"

const (
	// CredPurposeLabel is a label that Okta uses to distinguish between
	// creds stored in the static credentials. There will be credentials in the
	// wild that pre-date this label, and any such creds should be treated
	// as if they had `CredPurposeOktaAuth`.
	CredPurposeLabel = "okta/purpose"

	// CredPurposeOktaAuth indicates a token to be used by Teleport to
	// authenticate against Okta. The contents of the APIToken field is the
	// plaintext, user-supplied Okta API token.
	CredPurposeOktaAuth = "okta-auth"

	// CredPurposeSCIMToken indicates a token that should be used for
	// authenticating an inbound SCIM request. The contents of the APIToken
	// field will be a stringified bcrypt hash of the bearer token.
	CredPurposeSCIMToken = "scim-bearer-token"

	// CredPurposeOktaAPITokenWithSCIMOnlyIntegration is used when okta integration was enabled without
	// app groups sync. Due to backward compatibility when teleport was downgraded to version where the
	// AppGroupSyncDisabled flag is not supported we need to prevent plugin from starting.
	// This is done by distinguishing between OktaCredPurposeAuth and CredPurposeOktaAPITokenWithSCIMOnlyIntegration
	// that are only set when AppGroupSyncDisabled is set to true.
	CredPurposeOktaAPITokenWithSCIMOnlyIntegration = "okta-auth-scim-only"

	// CredPurposeOktaOauth is used to store the OAuth client ID for Okta.
	CredPurposeOktaOauth = "okta-oauth-client-id"

	// OktaSSOConnectorName is the name of the Okta SSO connector.
	OktaSSOConnectorName = "okta"

	// OktaSCIMTokenName is the name of the Okta SCIM token.
	OktaSCIMTokenName = types.PluginTypeOkta + "-scim-token"
)
