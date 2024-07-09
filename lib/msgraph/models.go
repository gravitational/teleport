package msgraph

type GroupMember interface {
	isGroupMember()
}

type DirectoryObject struct {
	ID          *string `json:"id,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
}

type Group struct {
	DirectoryObject
}

func (g *Group) isGroupMember() {}

type User struct {
	DirectoryObject

	Mail                     *string `json:"mail,omitempty"`
	OnPremisesSAMAccountName *string `json:"onPremisesSamAccountName,omitempty"`
	UserPrincipalName        *string `json:"userPrincipalName,omitempty"`
}

func (g *User) isGroupMember() {}

type Application struct {
	DirectoryObject

	AppID          *string         `json:"appId,omitempty"`
	IdentifierURIs *[]string       `json:"identifierUris,omitempty"`
	Web            *WebApplication `json:"web,omitempty"`
}

type WebApplication struct {
	RedirectURIs *[]string `json:"redirectUris,omitempty"`
}

type ServicePrincipal struct {
	DirectoryObject
	AppRoleAssignmentRequired          *bool   `json:"appRoleAssignmentRequired,omitempty"`
	PreferredSingleSignOnMode          *string `json:"preferredSingleSignOnMode,omitempty"`
	PreferredTokenSigningKeyThumbprint *string `json:"preferredTokenSigningKeyThumbprint,omitempty"`
}

type ApplicationServicePrincipal struct {
	Application      *Application      `json:"application,omitempty"`
	ServicePrincipal *ServicePrincipal `json:"servicePrincipal,omitempty"`
}

type FederatedIdentityCredential struct {
	Audiences *[]string `json:"audiences,omitempty"`
	Issuer    *string   `json:"issuer,omitempty"`
	Name      *string   `json:"name,omitempty"`
	Subject   *string   `json:"subject,omitempty"`
}

type SelfSignedCertificate struct {
	Thumbprint *string `json:"thumbprint,omitempty"`
}
