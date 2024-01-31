package scim

import (
	"context"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
)

// providerShim is an abstraction over the identity-provider-specific
// requirements (e.g. different methods identifying Teleport users as belonging
// to a specific upstream IDP, or different ways of authenticating requests from
// different client implementations.
//
// Shims are created on a per-request basis amd will only ever be interacted
// with from one request handler.
type providerShim interface {
	userPredicate(context.Context, types.User) bool
	userToResource(context.Context, types.User) (*scimpb.Resource, error)
	resourceToUser(context.Context, *scimpb.Resource) (types.User, error)
	updateUser(context.Context, types.User, *scimpb.Resource) (*scimpb.Resource, error)
	authorizeRequest(context.Context, string) error
}

// shimFactory is a function for creating new shim instances from a given plugin
// instance.
type shimFactory func(context.Context, types.Plugin, *Service) (providerShim, error)
