package services

import (
	"context"
	"maps"
	"slices"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/utils/pagination"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// IdentityCenterAccount wraps a raw identity center record in a new type to
// allow it to implement some custom interfaces. Account is a reference type
// that wraps a pointer to the underlying account record. Copies of an
// IdentityCenterAccount will point to the same record.
type IdentityCenterAccount struct {
	*identitycenterv1.Account
}

// CloneResource creates a deep copy of the underlying account resource
func (a IdentityCenterAccount) CloneResource() IdentityCenterAccount {
	var expiry *timestamppb.Timestamp
	if a.Metadata.Expires != nil {
		expiry = &timestamppb.Timestamp{
			Seconds: a.Metadata.Expires.Seconds,
			Nanos:   a.Metadata.Expires.Nanos,
		}
	}

	return IdentityCenterAccount{
		Account: &identitycenterv1.Account{
			Kind:    a.Kind,
			SubKind: a.SubKind,
			Version: a.Version,
			Metadata: &headerv1.Metadata{
				Name:        a.Metadata.Name,
				Namespace:   a.Metadata.Namespace,
				Description: a.Metadata.Description,
				Labels:      maps.Clone(a.Metadata.Labels),
				Expires:     expiry,
				Revision:    a.Metadata.Revision,
			},
			Spec: &identitycenterv1.AccountSpec{
				Id:             a.Spec.Id,
				Arn:            a.Spec.Arn,
				Name:           a.Spec.Name,
				Description:    a.Spec.Description,
				PermissionSets: slices.Clone(a.Spec.PermissionSets),
				StartUrl:       a.Spec.StartUrl,
			},
		},
	}
}

type IdentityCenterAccountID string

type IdentityCenterAccountGetter interface {
	ListIdentityCenterAccounts(context.Context, pagination.PageRequestToken) ([]IdentityCenterAccount, pagination.NextPageToken, error)
}

type IdentityCenterAccounts interface {
	IdentityCenterAccountGetter

	CreateIdentityCenterAccount(context.Context, *identitycenterv1.Account) (IdentityCenterAccount, error)
	GetIdentityCenterAccount(context.Context, IdentityCenterAccountID) (IdentityCenterAccount, error)
	UpdateIdentityCenterAccount(context.Context, *identitycenterv1.Account) (IdentityCenterAccount, error)
	DeleteIdentityCenterAccount(context.Context, IdentityCenterAccountID) error
	DeleteAllIdentityCenterAccounts(context.Context) error
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
	DeleteAllPrincipalAssignments(context.Context) error
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
