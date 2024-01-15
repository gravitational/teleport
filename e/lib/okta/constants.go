package okta

const (
	// oktaActive indicates an 'active' app or user status, implying that it
	// should be included in application listings.
	oktaActive = "ACTIVE"

	// oktaAdminConsole is the nme of the Okta Admin Console application.
	oktaAdminConsole = "Okta Admin Console"

	// oktaGroupNameEveryone identifies the default group containing every user
	// in an Okta system. It's always present.
	oktaGroupEveryone = "Everyone"

	// OktaCredPurposeLabel is a label that Okta uses to distinguish between
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
)
