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

package msgraph

import (
	"encoding/json"

	"github.com/gravitational/trace"
)

type GroupMember interface {
	GetID() *string
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
func (g *Group) GetID() *string { return g.ID }

type User struct {
	DirectoryObject

	Mail                     *string `json:"mail,omitempty"`
	OnPremisesSAMAccountName *string `json:"onPremisesSamAccountName,omitempty"`
	UserPrincipalName        *string `json:"userPrincipalName,omitempty"`
}

func (g *User) isGroupMember() {}
func (u *User) GetID() *string { return u.ID }

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

type AppRoleAssignment struct {
	ID          *string `json:"id,omitempty"`
	AppRoleID   *string `json:"appRoleId,omitempty"`
	PrincipalID *string `json:"principalId,omitempty"`
	ResourceID  *string `json:"resourceId,omitempty"`
}

func decodeGroupMember(msg json.RawMessage) (GroupMember, error) {
	var temp struct {
		Type string `json:"@odata.type"`
	}

	if err := json.Unmarshal(msg, &temp); err != nil {
		return nil, trace.Wrap(err)
	}

	var err error
	var member GroupMember
	switch temp.Type {
	case "#microsoft.graph.user":
		var u *User
		err = json.Unmarshal(msg, &u)
		member = u
	default:
		err = trace.BadParameter("unsupported group member type: %s", temp.Type)
	}

	return member, trace.Wrap(err)
}
