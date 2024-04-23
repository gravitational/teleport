package scim

import (
	"context"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

// providerShim is an abstraction over the identity-provider-specific
// requirements (e.g. different methods identifying Teleport users as belonging
// to a specific upstream IDP, or different ways of authenticating requests from
// different client implementations.
//
// Shims are created on a per-request basis amd will only ever be interacted
// with from one request handler.
type providerShim interface {
	//authorizeRequest grants or denies the request based on the contents of the
	// supplied authorization header string.
	authorizeRequest(context.Context, string) error

	// accessListPredicate checks if the access list is "owned" by this provider
	accessListPredicate(context.Context, *accesslist.AccessList) bool

	// userPredicate checks if the given user is "owned" by this provider
	userPredicate(context.Context, types.User) bool

	// userToResource formats the supplied Teleport user as a SCIM resource
	userToResource(context.Context, types.User) (*scimpb.Resource, error)

	// resourceToUser constructs an in-memory Teleport user from the supplied
	// SCIM resource
	resourceToUser(context.Context, *scimpb.Resource) (types.User, error)

	// onCreatingAccessList is called immediately before the SCIM service creates
	// an access list in order to give the shim a chance to modify the supplied
	// resources before they are actually created.
	onCreatingAccessList(context.Context, *accesslist.AccessList) error

	// onCreatingAccessList is called before the SCIM service creates an access
	// list member in order to give the shim a chance to modify the member
	// record before it is presented to the AccessList service
	onCreatingAccessListMember(context.Context, *accesslist.AccessListMember) error

	// onCreatingUser is called by the User handler immediately prior to
	// creating a cluster user in order to give the shim a chance to take
	// any action it deems necessary.
	onCreatingUser(context.Context, types.User, *scimpb.Resource) error

	// onCreatedUser is called by the User handler immediately after creating
	// the user cluster user to give the shim a chance to take any action it
	// deems necessary. Any changes made to the supplied resource will be sent
	// back to the client.
	onCreatedUser(context.Context, types.User, *scimpb.Resource) error

	// onUpdatingUser is called immediately prior to the user handler updating a
	// Teleport user. The event handler should update the supplied user with data
	// from resource and return the resulting user.
	//
	// The event handler should also return `true` if the user handler
	// should continue to update the cluster with the returned user, or
	// `false` if the user handler should take no further action
	// and just return the updated user to the client as-is.
	onUpdatingUser(context.Context, types.User, *scimpb.Resource) (types.User, bool, error)

	getResourceLabels() map[string]string
}

// shimFactory is a function for creating new shim instances from a given plugin
// instance.
type shimFactory func(context.Context, types.Plugin, *Service) (providerShim, error)
