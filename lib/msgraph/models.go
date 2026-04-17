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

import "github.com/gravitational/teleport/lib/msgraph/models"

// TODO(sshah): Delete type alias below once teleport.e is updated
// to import new models package.

type GroupMember = models.GroupMember
type DirectoryObject = models.DirectoryObject
type Group = models.Group
type User = models.User
type Application = models.Application
type ServicePrincipal = models.ServicePrincipal
type ApplicationServicePrincipal = models.ApplicationServicePrincipal
type FederatedIdentityCredential = models.FederatedIdentityCredential
type SelfSignedCertificate = models.SelfSignedCertificate
type AppRole = models.AppRole
type AppRoleAssignment = models.AppRoleAssignment
type OptionalClaim = models.OptionalClaim
type OptionalClaims = models.OptionalClaims
type WebApplication = models.WebApplication

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
