package services

import (
	"context"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type IdentityCenterAccountID string

type IdentityCenterAccounts interface {
	ListIdentityCenterAccounts(context.Context, pagination.PageRequestToken) ([]*identitycenterv1.Account, pagination.NextPageToken, error)
	CreateIdentityCenterAccount(context.Context, *identitycenterv1.Account) (*identitycenterv1.Account, error)
	GetIdentityCenterAccount(context.Context, IdentityCenterAccountID) (*identitycenterv1.Account, error)
	UpdateIdentityCenterAccount(context.Context, *identitycenterv1.Account) (*identitycenterv1.Account, error)
	DeleteIdentityCenterAccount(context.Context, IdentityCenterAccountID) error
}

// MarshalIdentityCenterAccount marshals the account object into a JSON byte array.
func MarshalIdentityCenterAccount(object *identitycenterv1.Account, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalIdentityCenterAccount un-marshals an account object from a JSON byte array.
func UnmarshalIdentityCenterAccount(data []byte, opts ...MarshalOption) (*identitycenterv1.Account, error) {
	return UnmarshalProtoResource[*identitycenterv1.Account](data, opts...)
}

type PrincipalAssignmentID string

type IdentityCenterPrincipalAssignments interface {
	ListPrincipalAssignments(context.Context, pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error)
	CreatePrincipalAssignment(context.Context, *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error)
	GetPrincipalAssignment(context.Context, PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error)
	UpdatePrincipalAssignment(context.Context, *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error)
	DeletePrincipalAssignment(context.Context, PrincipalAssignmentID) error
}

// MarshalPrincipalAssignment marshals the assignment state object into a JSON byte array.
func MarshalPrincipalAssignment(object *identitycenterv1.PrincipalAssignment, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalPrincipalAssignment un-marshals the User State  object from a JSON byte array.
func UnmarshalPrincipalAssignment(data []byte, opts ...MarshalOption) (*identitycenterv1.PrincipalAssignment, error) {
	return UnmarshalProtoResource[*identitycenterv1.PrincipalAssignment](data, opts...)
}

type PermissionSetID string

type IdentityCenterPermissionSets interface {
	ListPermissionSets(context.Context, pagination.PageRequestToken) ([]*identitycenterv1.PermissionSet, pagination.NextPageToken, error)
	CreatePermissionSet(context.Context, *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error)
	GetPermissionSet(context.Context, PermissionSetID) (*identitycenterv1.PermissionSet, error)
	UpdatePermissionSet(context.Context, *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error)
	DeletePermissionSet(context.Context, PermissionSetID) error
}

// MarshalPermissionSet marshals the assignment state object into a JSON byte array.
func MarshalPermissionSet(object *identitycenterv1.PermissionSet, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalPermissionSet un-marshals the User State  object from a JSON byte array.
func UnmarshalPermissionSet(data []byte, opts ...MarshalOption) (*identitycenterv1.PermissionSet, error) {
	return UnmarshalProtoResource[*identitycenterv1.PermissionSet](data, opts...)
}

// IdentityCenter combines all the resource managers used by the Identity Center plugin
type IdentityCenter interface {
	IdentityCenterAccounts
	IdentityCenterPermissionSets
	IdentityCenterPrincipalAssignments
}
