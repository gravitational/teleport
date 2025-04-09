package client

import (
	"context"

	"github.com/gravitational/trace"
)

// Role represents a role in the Identity Vault.
type Role struct {
	// ID is the unique identifier of the role (e.g. "cn=Role1,cn=Roles,cn=Access,cn=IDVault").
	ID string `json:"id"`
	// Name is the name of the role (e.g. "Role1").
	Name string `json:"name"`
	// Description is the description of the role.
	Description string `json:"description"`
	// Categories is the list of categories the role belongs to.
	Categories []Category `json:"categories,omitempty"`
	// Level is the level of the role.
	Level int `json:"level"`
	// RoleLevel is the role level.
	RoleLevel struct {
		// Name is the name of the role level.
		Name string `json:"name"`
		// Level is the level of the role level.
		Level int `json:"level"`
		// Cn is the common name of the role level.
		Cn string `json:"cn"`
	} `json:"roleLevel"`
	// Entitlements is the list of entitlements associated with the role.
	Entitlements map[string]string
}

// ListRoles returns a list of all roles in the Identity Vault.
func (c *Client) ListRoles(ctx context.Context) ([]Role, error) {
	const rolesBase = "rest/catalog/roles/listV2"

	roles, err := listResponse(
		ctx,
		c,
		rolesBase,
		func(r listRolesResponse) ([]Role, error) {
			return r.Roles, nil
		},
		withQueryParams("q", "*"),
	)

	return roles, trace.Wrap(err)
}

type listRolesResponse struct {
	NextIndex int    `json:"nextIndex"`
	ArraySize int    `json:"arraySize"`
	Roles     []Role `json:"roles"`
	TotalSize int    `json:"totalSize"`
	Total     int    `json:"total"`
}

func (n listRolesResponse) GetNextIndex() int {
	return n.NextIndex
}

// RoleRef represents a role reference in the Identity Vault.
type RoleRef struct {
	// ID is the unique identifier of the role (e.g. "cn=Role1,cn=Roles,cn=Access,cn=IDVault").
	ID string `json:"id"`
	// Name is the name of the role (e.g. "Role1").
	Name string `json:"name"`
	// Description is the description of the role.
	Description string `json:"description"`
	// Requester is the requester of the role.
	Requester string `json:"requester"`
	// Level is the level of the role.
	Level int `json:"level"`
	// RequestDescription is the description of the request.
	RequestDescription string `json:"requestDescription"`
}

// ListRoleSubRoles returns a list of all sub role references in the Identity Vault.
func (c *Client) ListRoleSubRoles(ctx context.Context, roleID string) ([]RoleRef, error) {
	b, err := newPayloadRequestBody(roleID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const rolesBase = "rest/catalog/roles/subRoles/list"

	roleRef, err := listResponse(
		ctx,
		c,
		rolesBase,
		func(r listRoleRefResponse) ([]RoleRef, error) {
			return r.Roles, nil
		},
		withQueryParams("q", "*"),
		withPostRequest(b),
	)

	return roleRef, trace.Wrap(err)
}

// ListRoleParentRoles returns a list of all parent role references in the Identity Vault.
func (c *Client) ListRoleParentRoles(ctx context.Context, roleID string) ([]RoleRef, error) {
	b, err := newPayloadRequestBody(roleID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const rolesBase = "rest/catalog/roles/parentRoles/list"
	roleRef, err := listResponse(
		ctx,
		c,
		rolesBase,
		func(r listRoleRefResponse) ([]RoleRef, error) {
			return r.Roles, nil
		},
		withQueryParams("q", "*"),
		withPostRequest(b),
	)

	return roleRef, trace.Wrap(err)
}

type listRoleRefResponse struct {
	NextIndex int       `json:"nextIndex"`
	ArraySize int       `json:"arraySize"`
	Roles     []RoleRef `json:"roles"`
}

func (n listRoleRefResponse) GetNextIndex() int {
	return n.NextIndex
}

// RoleAssignmentStatus represents the status of a role assignment in the Identity Vault.
type RoleAssignmentStatus struct {
	// DN is the distinguished name of the role assignment.
	Dn string `json:"dn"`
	// RecipientType is the type of the recipient.
	// Can be "USER" or "GROUP".
	RecipientType string `json:"recipientType"`
	// RecipientTypeSubContainer is the sub container of the recipient type.
	RecipientTypeSubContainer string `json:"recipientTypeSubContainer"`
	// RecipientDn is the distinguished name of the recipient.
	RecipientDn string `json:"recipientDn"`
	// RecipientFullName is the full name of the recipient.
	RecipientFullName string `json:"recipientFullName"`
	// StatusCode is the status code of the role assignment.
	StatusCode string `json:"statusCode"`
	// StatusDisplay is the display of the status.
	StatusDisplay string `json:"statusDisplay"`
	// EffectiveDate is the effective date of the role assignment.
	EffectiveDate string `json:"effectiveDate"`
	// ExpiryDate is the expiry date of the role assignment.
	ExpiryDate string `json:"expiryDate"`
	// Description is the description of the role assignment.
	Description string `json:"description"`
	// Grant is a flag that determines whether the role assignment is granted.
	Grant bool `json:"grant"`
}

// ListRoleMembers returns a list of all members of the role with the given ID.
func (c *Client) ListRoleMembers(ctx context.Context, roleID string) ([]RoleAssignmentStatus, error) {
	b, err := newDNPayloadRequestBody(roleID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const rolesBase = "rest/catalog/roles/role/assignments/v2"
	roleMembers, err := listResponse(
		ctx,
		c,
		rolesBase,
		func(r listRoleMembersResponse) ([]RoleAssignmentStatus, error) {
			return r.AssignmentStatusList, nil
		},
		withQueryParams("q", "*"),
		withPostRequest(b),
	)

	return roleMembers, trace.Wrap(err)
}

type listRoleMembersResponse struct {
	AssignmentStatusList []RoleAssignmentStatus `json:"assignmentStatusList"`
	NextIndex            int                    `json:"nextIndex"`
	ArraySize            int                    `json:"arraySize"`
	Total                int                    `json:"total"`
	Hasmore              bool                   `json:"hasmore"`
}

func (n listRoleMembersResponse) GetNextIndex() int {
	return n.NextIndex
}

// ResourceRef represents a resource reference in the Identity Vault.
type ResourceRef struct {
	// ID is the unique identifier of the resource.
	ID string `json:"id"`
	// Name is the name of the resource.
	Name string `json:"name"`
	// Description is the description of the resource.
	Description string `json:"description"`
	// MappingDescription is the description of the mapping.
	MappingDescription string `json:"mappingDescription"`
	// Status is the display of the status.
	Status int `json:"status"`
	// EntityKey is the entity key of the resource.
	EntityKey string `json:"entityKey"`
	// EntitlementValues is the list of entitlement values associated with the resource.
	Entitlements []EntitlementValue `json:"entitlementValues"`
}

// EntitlementValue represents an entitlement value in the Identity Vault.
type EntitlementValue struct {
	// ID is the unique identifier of the resource.
	ID string `json:"id"`
	// Name is the name of the resource.
	Name string `json:"name"`
	// Value is the value of the entitlement.
	Value string `json:"value"`
}

// ListMappedResources returns a list of all mapped resources of the role with the given ID.
func (c *Client) ListMappedResources(ctx context.Context, roleID string) ([]ResourceRef, error) {
	b, err := newPayloadRequestBody(roleID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const rolesBase = "rest/catalog/roles/mappedResources/list"
	roleMembers, err := listResponse(
		ctx,
		c,
		rolesBase,
		func(r listMappedResourcesResponse) ([]ResourceRef, error) {
			return r.Resources, nil
		},
		withQueryParams("q", "*"),
		withPostRequest(b),
	)

	return roleMembers, trace.Wrap(err)
}

type listMappedResourcesResponse struct {
	NextIndex int           `json:"nextIndex"`
	ArraySize int           `json:"arraySize"`
	Resources []ResourceRef `json:"resources"`
}

func (n listMappedResourcesResponse) GetNextIndex() int {
	return n.NextIndex
}
