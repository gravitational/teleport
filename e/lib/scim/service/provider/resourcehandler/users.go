package resourcehandler

import (
	"context"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/lister"
)

// ProviderUser is an interface that defines the methods required for a SCIM
// provider to manage users.
type ProviderUser interface {
	// UserPredicate returns true if the user is managed by this IdP plugin.
	UserPredicate(context.Context, types.User) bool
	// UserToResource converts a Teleport user to a SCIM resource.
	UserToResource(context.Context, types.User) (*scimpb.Resource, error)
	// ResourceToUser converts a SCIM resource to a Teleport user.
	ResourceToUser(context.Context, *scimpb.Resource) (types.User, error)
	// OnUpdatingUser is called before updating a user.
	OnUpdatingUser(context.Context, types.User, *scimpb.Resource) (types.User, bool, error)
	// OnCreatedUser is called after creating a user.
	OnCreatedUser(ctx context.Context, user types.User, resource *scimpb.Resource) error
}

// UserHandler is a struct that implements the ProviderUser interface and
// provides methods to manage users in a SCIM provider.
type UserHandler struct {
	common.Config
	ProviderUser
}

// CreateResource handles the creation of a new SCIM resource.
func (h *UserHandler) CreateResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	newUser, err := h.ResourceToUser(ctx, req.GetResource())
	if err != nil {
		return nil, trace.Wrap(err, "converting Teleport user")
	}

	createdUser, err := h.UsersService.CreateUser(ctx, newUser)
	if err != nil {
		return nil, trace.Wrap(err, "creating Teleport user")
	}
	if err := h.OnCreatedUser(ctx, createdUser, req.GetResource()); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := h.UserToResource(ctx, createdUser)
	if err != nil {
		return nil, trace.Wrap(err, "formatting created user")
	}

	return result, nil
}

// UpdateResource handles the update of an existing SCIM resource.
func (h *UserHandler) UpdateResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	user, err := h.UsersService.GetUser(ctx, req.GetResource().GetId(), false)
	if err != nil {
		return nil, trace.NotFound("%s", req.GetResource().GetId())
	}

	if user.GetRevision() != conv.ResourceVersion(req.GetResource()) {
		return nil, trace.CompareFailed("invalid revision: %q != %q", user.GetRevision(), req.GetResource().GetMeta().GetVersion())
	}

	updatedUser, saveUpdatedUser, err := h.OnUpdatingUser(ctx, user, req.GetResource())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if saveUpdatedUser {
		updatedUser, err = h.UsersService.UpdateUser(ctx, updatedUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	result, err := h.UserToResource(ctx, updatedUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return result, err
}

// GetResource handles an individual resource query from the server
func (h *UserHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	user, err := h.UsersService.GetUser(ctx, req.GetTarget().GetResourceId(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if this user does not belong to this IDP plugin, then they don't exist as
	// far as this request is concerned.
	if !h.UserPredicate(ctx, user) {
		return nil, trace.NotFound("%s", req.GetTarget().GetResourceId())
	}

	userResource, err := h.UserToResource(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "converting user %s to SCIM resource", user.GetName())
	}

	return userResource, nil
}

// ListResources handles a request to list SCIM resources. It supports
func (h *UserHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	l := lister.UserLister{
		Config: h.Config,
		Predicate: func(ctx context.Context, user types.User) bool {
			return h.UserPredicate(ctx, user)
		},
		UserToResource: func(user types.User) (*scimpb.Resource, error) {
			out, err := h.UserToResource(ctx, user)
			return out, trace.Wrap(err)
		},
	}
	out, err := l.ListResources(ctx, req)
	return out, trace.Wrap(err)
}

// DeleteResource handles a request to delete a SCIM resource.
func (h *UserHandler) DeleteResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) error {
	return trace.NotImplemented("delete")
}
