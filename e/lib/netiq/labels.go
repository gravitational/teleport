package netiq

const (
	// NetIQOrgURLLabel is the label for which NetIQ instance object belongs to.
	NetIQOrgURLLabel = "netiq/org"
	// CredPurposeLabel is the label for the purpose of the credential.
	CredPurposeLabel = "netiq/purpose"
	// CredPurposeNetIQOauth is the purpose for the NetIQ OAuth client ID and secret.
	CredPurposeNetIQOauth = "oauth-client-id-secret"
	// CredPurposeNetIQAuth is the purpose for the NetIQ Identity Vault user and password.
	CredPurposeNetIQAuth = "netiq-auth"
)
