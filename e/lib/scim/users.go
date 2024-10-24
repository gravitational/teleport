package scim

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/scim2/filter-parser/v2"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
)

const (
	usernameAttribute = "userName"
)

type userHandler struct {
	users  UsersService
	logger *slog.Logger
}

var _ resourceHandler = (*userHandler)(nil)

func (uh *userHandler) create(ctx context.Context, shim providerShim, r *scimpb.Resource) (*scimpb.Resource, error) {
	newUser, err := shim.resourceToUser(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err, "converting Teleport user")
	}

	if err := shim.onCreatingUser(ctx, newUser, r); err != nil {
		return nil, trace.Wrap(err)
	}

	createdUser, err := uh.users.CreateUser(ctx, newUser)
	if err != nil {
		return nil, trace.Wrap(err, "creating Teleport user")
	}

	if err := shim.onCreatedUser(ctx, createdUser, r); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := shim.userToResource(ctx, createdUser)
	if err != nil {
		return nil, trace.Wrap(err, "formatting created user")
	}

	return result, nil
}

func (uh *userHandler) update(ctx context.Context, shim providerShim, r *scimpb.Resource) (*scimpb.Resource, error) {
	user, err := uh.users.GetUser(ctx, r.Id, false)
	if err != nil {
		return nil, trace.NotFound(r.Id)
	}

	if user.GetRevision() != r.Meta.Version {
		return nil, trace.CompareFailed("invalid revision: %q != %q", user.GetRevision(), r.Meta.Version)
	}

	updatedUser, saveUpdatedUser, err := shim.onUpdatingUser(ctx, user, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if saveUpdatedUser {
		updatedUser, err = uh.users.UpdateUser(ctx, updatedUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	result, err := shim.userToResource(ctx, updatedUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return result, err
}

// get handles an individual resource query from the server
func (uh *userHandler) get(ctx context.Context, shim providerShim, resourceID string) (*scimpb.Resource, error) {
	user, err := uh.users.GetUser(ctx, resourceID, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if this user does not belong to this IDP plugin, then they don't exist as
	// far as this request is concerned.
	if !shim.userPredicate(ctx, user) {
		return nil, trace.NotFound(resourceID)
	}

	userResource, err := shim.userToResource(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "converting user %s to SCIM resource", user.GetName())
	}

	return userResource, nil
}

// list handles a bulk listing query from the client
func (uh *userHandler) list(ctx context.Context, shim providerShim, filter filter.Expression, requestedPage *scimpb.Page) (*scimpb.ResourceList, error) {
	const pageSize = 100
	index := 0
	totalCount := 0
	outputResources := []*scimpb.Resource{}

	req := userspb.ListUsersRequest{
		PageSize: pageSize,
	}

	for {
		rsp, err := uh.users.ListUsers(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, user := range rsp.Users {
			if !shim.userPredicate(ctx, user) {
				continue
			}

			filterAttribs := map[string]string{usernameAttribute: user.GetName()}
			if err := evaluateFilter(filter, filterAttribs); err != nil {
				continue
			}

			index++
			if index < int(requestedPage.StartIndex) {
				continue
			}

			if len(outputResources) < int(requestedPage.Count) {
				userResource, err := shim.userToResource(ctx, user)
				if err != nil {
					uh.logger.ErrorContext(ctx, "converting user to SCIM resource",
						"user", user.GetName(),
						"error", err,
					)
					continue
				}

				outputResources = append(outputResources, userResource)
			}

			totalCount++
		}

		req.PageToken = rsp.NextPageToken
		if req.PageToken == "" {
			break
		}
	}

	output := &scimpb.ResourceList{
		TotalResults: int32(totalCount),
		StartIndex:   int32(requestedPage.StartIndex),
		ItemsPerPage: int32(requestedPage.Count),
		Resources:    outputResources,
	}

	return output, nil
}

func (uh *userHandler) delete(ctx context.Context, shim providerShim, id string) error {
	return trace.NotImplemented("delete")
}
