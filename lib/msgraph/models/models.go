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

package models

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// GroupMember represents Microsoft Graph API group members.
type GroupMember interface {
	GetID() *string
	isGroupMember()
}

// DirectoryObject is a core object structure of the
// Microsoft Graph API resource properties.
type DirectoryObject struct {
	ID          *string `json:"id,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
}

// Group defines Microsoft Graph API group object.
type Group struct {
	DirectoryObject
	// GroupTypes is a list of group type strings.
	GroupTypes []string `json:"groupTypes,omitempty"`
	// OnPremisesDomainName is the on-premises domain name of the group.
	OnPremisesDomainName *string `json:"onPremisesDomainName,omitempty"`
	// OnPremisesNetBiosName is the on-premises NetBIOS name of the group.
	OnPremisesNetBiosName *string `json:"onPremisesNetBiosName,omitempty"`
	// OnPremisesSamAccountName is the on-premises SAM account name of the group.
	OnPremisesSamAccountName *string `json:"onPremisesSamAccountName,omitempty"`
	// Owners is a list of users who are the owners of this group.
	Owners []*User `json:"owners,omitempty"`
}

// IsOffice365Group checks if the group is a Office 365 group.
func (g *Group) IsOffice365Group() bool {
	const office365Group = "Unified"
	return slices.Contains(g.GroupTypes, office365Group)
}

func (g *Group) isGroupMember() {}

// GetID returns ID of the group.
func (g *Group) GetID() *string { return g.ID }

// Group defines Microsoft Graph API user object.
type User struct {
	DirectoryObject

	Mail                     *string `json:"mail,omitempty"`
	OnPremisesSAMAccountName *string `json:"onPremisesSamAccountName,omitempty"`
	UserPrincipalName        *string `json:"userPrincipalName,omitempty"`
	Surname                  *string `json:"surname,omitempty"`
	GivenName                *string `json:"givenName,omitempty"`
}

func (g *User) isGroupMember() {}

// GetID returns ID of the user.
func (u *User) GetID() *string { return u.ID }

// Application defines Microsoft Graph API Application object.
type Application struct {
	DirectoryObject

	AppID                 *string         `json:"appId,omitempty"`
	IdentifierURIs        *[]string       `json:"identifierUris,omitempty"`
	Web                   *WebApplication `json:"web,omitempty"`
	GroupMembershipClaims *string         `json:"groupMembershipClaims,omitempty"`
	OptionalClaims        *OptionalClaims `json:"optionalClaims,omitempty"`
}

const (
	// OPTIONAL_CLAIM_GROUP_NAME is the group claim.
	OPTIONAL_CLAIM_GROUP_NAME = "groups"
)

const (
	// OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME is the sAMAccountName claim.
	OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME = "sam_account_name"
	// OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_DNS_DOMAIN_AND_SAM_ACCOUNT_NAME is the dnsDomainName\sAMAccountName claim.
	OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_DNS_DOMAIN_AND_SAM_ACCOUNT_NAME = "dns_domain_and_sam_account_name"
	// OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_NETBIOS_DOMAIN_AND_SAM_ACCOUNT_NAME is the netbiosDomainName\sAMAccountName claim.
	OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_NETBIOS_DOMAIN_AND_SAM_ACCOUNT_NAME = "netbios_domain_and_sam_account_name"
	// OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_EMIT_AS_ROLES is the emit_as_roles claim.
	OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_EMIT_AS_ROLES = "emit_as_roles"
	// OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_CLOUD_DISPLAYNAME is the cloud_displayname claim.
	OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_CLOUD_DISPLAYNAME = "cloud_displayname"
)

// OptionalClaim represents an optional claim in a token.
// https://learn.microsoft.com/en-us/entra/identity-platform/optional-claims?tabs=appui
type OptionalClaim struct {
	// AdditionalProperties is a list of additional properties.
	// Possible values:
	// - sam_account_name: sAMAccountName
	// - dns_domain_and_sam_account_name: dnsDomainName\sAMAccountName
	// - netbios_domain_and_sam_account_name: netbiosDomainName\sAMAccountName
	// - emit_as_roles
	// - cloud_displayname
	AdditionalProperties []string `json:"additionalProperties,omitempty"`
	Essential            *bool    `json:"essential,omitempty"`
	Name                 *string  `json:"name,omitempty"`
	Source               *string  `json:"source,omitempty"`
}

// OptionalClaims represents optional claims in an OIDC token.
type OptionalClaims struct {
	IDToken     []OptionalClaim `json:"idToken,omitempty"`
	AccessToken []OptionalClaim `json:"accessToken,omitempty"`
	SAML2Token  []OptionalClaim `json:"saml2Token,omitempty"`
}

// WebApplication defines Microsoft Graph API web application object
// used to configure SSO redirect URL for the Enterprise Application.
type WebApplication struct {
	RedirectURIs *[]string `json:"redirectUris,omitempty"`
}

// ServicePrincipal defines Microsoft Graph API service principal object.
type ServicePrincipal struct {
	DirectoryObject
	AppRoleAssignmentRequired          *bool      `json:"appRoleAssignmentRequired,omitempty"`
	PreferredSingleSignOnMode          *string    `json:"preferredSingleSignOnMode,omitempty"`
	PreferredTokenSigningKeyThumbprint *string    `json:"preferredTokenSigningKeyThumbprint,omitempty"`
	AppRoles                           []*AppRole `json:"appRoles,omitempty"`
}

// ApplicationServicePrincipal defines Microsoft Graph API
// application service principal associated with the Enterprise APplication.
type ApplicationServicePrincipal struct {
	Application      *Application      `json:"application,omitempty"`
	ServicePrincipal *ServicePrincipal `json:"servicePrincipal,omitempty"`
}

// FederatedIdentityCredential defines Microsoft Graph API
// federated identity credential type associated with the Enterprise
// Application.
type FederatedIdentityCredential struct {
	Audiences *[]string `json:"audiences,omitempty"`
	Issuer    *string   `json:"issuer,omitempty"`
	Name      *string   `json:"name,omitempty"`
	Subject   *string   `json:"subject,omitempty"`
}

// SelfSignedCertificate defines Microsoft Graph API self-signed
// token-signing certificate for the Service Principal.
type SelfSignedCertificate struct {
	Thumbprint *string `json:"thumbprint,omitempty"`
}

// AppRole defines Microsoft Graph API
// App Role object.
type AppRole struct {
	ID    *string `json:"id,omitempty"`
	Value *string `json:"value,omitempty"`
}

// AppRoleAssignment defines Microsoft Graph API
// App Role Assignment object.
type AppRoleAssignment struct {
	ID          *string `json:"id,omitempty"`
	AppRoleID   *string `json:"appRoleId,omitempty"`
	PrincipalID *string `json:"principalId,omitempty"`
	ResourceID  *string `json:"resourceId,omitempty"`
}

// DecodeGroupMember decodes raw group member response
// checks for supported group member type of user or group.
func DecodeGroupMember(msg json.RawMessage) (GroupMember, error) {
	var temp struct {
		Type string `json:"@odata.type"`
	}

	if err := utils.FastUnmarshal(msg, &temp); err != nil {
		return nil, trace.Wrap(err)
	}

	var err error
	var member GroupMember
	switch temp.Type {
	case "#microsoft.graph.user":
		var u *User
		err = utils.FastUnmarshal(msg, &u)
		member = u
	case "#microsoft.graph.group":
		var g *Group
		err = utils.FastUnmarshal(msg, &g)
		member = g
	default:
		// Return an error if we encounter a type we do not support.
		// The caller ignores the error and continues processing the next entry.
		err = &UnsupportedGroupMember{Type: temp.Type}
	}

	return member, trace.Wrap(err)
}

// UnsupportedGroupMember is an internal error to indicate that
// the `groupmembers` endpoint has returned a member of type that we do not support (yet).
type UnsupportedGroupMember struct {
	Type string
}

// Error returns friendly string for UnsupportedGroupMember error.
func (u *UnsupportedGroupMember) Error() string {
	return fmt.Sprintf("Unsupported group member: %q", u.Type)
}
