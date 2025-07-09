package generic

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	typescommon "github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/lister"
)

type userHandler struct {
	common.Config
	Plugin *types.PluginV1
}

func userExternalID(u types.User) string {
	id, _ := u.GetLabel(common.ExternalIDLabel)
	return id
}

// CreateResource handles the creation of a new SCIM user resource.
func (h *userHandler) CreateResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	additionalLabels := map[string]string{
		common.ExternalIDLabel: req.GetResource().GetExternalId(),
		types.OriginLabel:      typescommon.OriginSCIM,
	}

	scimUser, err := conv.UserFromResource(req.GetResource(),
		conv.WithUserOptionClock(h.Clock),
		conv.WithLabels(additionalLabels),
		conv.WithConnectorRef(
			&types.ConnectorRef{
				ID:       h.Plugin.Spec.GetScim().SamlConnectorName,
				Type:     constants.SAML,
				Identity: req.GetResource().GetExternalId(),
			}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportUser, err := h.UsersService.CreateUser(ctx, scimUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.UserToResource(teleportUser, conv.WithExternalIDFunc(userExternalID))
}

// ListResources lists all SCIM user resources.
func (h *userHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	l := lister.UserLister{
		Config: h.Config,
		Predicate: func(_ context.Context, user types.User) bool {
			return hasSCIMOrigin(user)
		},
		UserToResource: func(user types.User) (*scimpb.Resource, error) {
			groups, err := h.getAccessListsForUser(ctx, user.GetName())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			attr := map[string]any{
				"groups": conv.ToSCIMGroups(groups),
			}
			return conv.UserToResource(user,
				conv.WithExternalIDFunc(userExternalID),
				conv.WithAttributes(attr),
			)
		},
	}

	return l.ListResources(ctx, req)
}

// GetResource retrieves a specific SCIM user resource by its ID.
func (h *userHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	userID := req.GetTarget().GetResourceId()

	teleportUser, err := h.UsersService.GetUser(ctx, userID, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !hasSCIMOrigin(teleportUser) {
		return nil, trace.AccessDenied("user %q is not SCIM managed", userID)
	}

	groupNames, err := h.getAccessListsForUser(ctx, teleportUser.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attr := map[string]any{
		"groups": conv.ToSCIMGroups(groupNames),
	}

	return conv.UserToResource(
		teleportUser,
		conv.WithExternalIDFunc(userExternalID),
		conv.WithAttributes(attr),
	)
}

// UpdateResource updates an existing SCIM user resource.
func (h *userHandler) UpdateResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	userID := req.GetTarget().GetResourceId()

	existingUser, err := h.UsersService.GetUser(ctx, userID, false /* with secrets*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !hasSCIMOrigin(existingUser) {
		return nil, trace.NotFound("user %q is not SCIM managed", userID)
	}
	additionalLabels := map[string]string{
		common.ExternalIDLabel: req.GetResource().GetExternalId(),
		types.OriginLabel:      typescommon.OriginSCIM,
	}

	updatedSCIMUser, err := conv.UserFromResource(req.GetResource(),
		conv.WithUserOptionClock(h.Clock),
		conv.WithLabels(additionalLabels),
		conv.WithConnectorRef(existingUser.GetCreatedBy().Connector),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use existing revision if not set
	if updatedSCIMUser.GetRevision() == "" {
		updatedSCIMUser.SetRevision(existingUser.GetRevision())
	}

	updatedUser, err := h.UsersService.UpdateUser(ctx, updatedSCIMUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.UserToResource(updatedUser, conv.WithExternalIDFunc(userExternalID))
}

// DeleteResource deletes a SCIM user resource by its ID.
func (h *userHandler) DeleteResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) error {
	userID := req.GetTarget().GetResourceId()

	teleportUser, err := h.UsersService.GetUser(ctx, userID, false)
	if err != nil {
		return trace.Wrap(err)
	}

	if !hasSCIMOrigin(teleportUser) {
		return trace.NotFound("user %q is not SCIM managed", userID)
	}

	return trace.Wrap(h.UsersService.DeleteUser(ctx, teleportUser.GetName()))
}

// getAccessListsForUser returns all access list names a given user is a member of.
func (h *userHandler) getAccessListsForUser(ctx context.Context, userID string) ([]string, error) {
	var groups []string

	for acl, err := range clientutils.Resources(ctx, h.AccessListsService.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		_, err := h.AccessListsService.GetAccessListMember(ctx, acl.GetName(), userID)
		if trace.IsNotFound(err) {
			continue
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}

		groups = append(groups, acl.GetName())
	}

	slices.Sort(groups)
	return groups, nil
}

// GetPlugin returns the plugin associated with the user handler.
func (h *userHandler) GetPlugin() *types.PluginV1 {
	return h.Plugin
}
