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

import "context"

type Client interface {
	// IterateUsers lists all users in the Entra ID directory using pagination.
	// `f` will be called for each object in the result set.
	// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
	// Ref: [https://learn.microsoft.com/en-us/graph/api/user-list].
	IterateUsers(ctx context.Context, f func(*User) bool) error
	// IterateGroups lists all groups in the Entra ID directory using pagination.
	// `f` will be called for each object in the result set.
	// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
	// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list].
	IterateGroups(ctx context.Context, f func(*Group) bool) error
	// IterateGroupMembers lists all members for the given Entra ID group using pagination.
	// `f` will be called for each object in the result set.
	// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
	// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list-members].
	IterateGroupMembers(ctx context.Context, groupID string, f func(GroupMember) bool) error
	// IterateApplications lists all applications in the Entra ID directory using pagination.
	// `f` will be called for each object in the result set.
	// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
	// Ref: [https://learn.microsoft.com/en-us/graph/api/application-list].
	IterateApplications(ctx context.Context, f func(*Application) bool) error

	// CreateFederatedIdentityCredential creates a new FederatedCredential.
	// Ref: [https://learn.microsoft.com/en-us/graph/api/application-post-federatedidentitycredentials].
	CreateFederatedIdentityCredential(ctx context.Context, appObjectID string, cred *FederatedIdentityCredential) (*FederatedIdentityCredential, error)
	// CreateServicePrincipalTokenSigningCertificate generates a new token signing certificate for the given service principal.
	// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-addtokensigningcertificate].
	CreateServicePrincipalTokenSigningCertificate(ctx context.Context, spID string, displayName string) (*SelfSignedCertificate, error)
	// GetServicePrincipalByAppId returns the service principal associated with the given application.
	// Note that appID here is the app the application "client ID" ([Application.AppID]), not "object ID" ([Application.ID]).
	// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-get].
	GetServicePrincipalsByDisplayName(ctx context.Context, displayName string) ([]*ServicePrincipal, error)
	// GetServicePrincipalsByDisplayName returns the service principals that have the given display name.
	// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list].
	GetServicePrincipalByAppId(ctx context.Context, appID string) (*ServicePrincipal, error)
	// GrantAppRoleToServicePrincipal grants the given app role to the specified Service Principal.
	// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-post-approleassignedto]
	GrantAppRoleToServicePrincipal(ctx context.Context, spID string, assignment *AppRoleAssignment) (*AppRoleAssignment, error)
	// InstantiateApplicationTemplate instantiates an application from the Entra application Gallery,
	// creating a pair of [Application] and [ServicePrincipal].
	// Ref: [https://learn.microsoft.com/en-us/graph/api/applicationtemplate-instantiate].
	InstantiateApplicationTemplate(ctx context.Context, appTemplateID string, displayName string) (*ApplicationServicePrincipal, error)
	// UpdateApplication issues a partial update for an [Application].
	// Note that appID here is the app the application  "object ID" ([Application.ID]), not "client ID" ([Application.AppID]).
	// Ref: [https://learn.microsoft.com/en-us/graph/api/application-update].
	UpdateApplication(ctx context.Context, appObjectID string, app *Application) error
	// UpdateServicePrincipal issues a partial update for a [ServicePrincipal].
	// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-update].
	UpdateServicePrincipal(ctx context.Context, spID string, sp *ServicePrincipal) error
}
