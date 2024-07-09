package services

import (
	"context"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
)

type PageToken string

const (
	EndOfList PageToken = ""
)

type ProvisioningStates interface {
	GetUserProvisioningState(context.Context, string) (*provisioningv1.UserState, error)
	ListUserProvisioningStates(context.Context, PageToken) ([]*provisioningv1.UserState, PageToken, error)
	CreateUserProvisioningState(context.Context, *provisioningv1.UserState) (*provisioningv1.UserState, error)
	UpdateUserProvisioningState(context.Context, *provisioningv1.UserState) (*provisioningv1.UserState, error)
	DeleteUserProvisioningState(context.Context, string) error
	DeleteAllUserProvisioningStates(context.Context) error
}

// MarshalProvisioningUserState marshals the User State object into a JSON byte array.
func MarshalProvisioningUserState(object *provisioningv1.UserState, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalProvisioningUserState un-marshals the User State  object from a JSON byte array.
func UnmarshalProvisioningUserState(data []byte, opts ...MarshalOption) (*provisioningv1.UserState, error) {
	return UnmarshalProtoResource[*provisioningv1.UserState](data, opts...)
}
