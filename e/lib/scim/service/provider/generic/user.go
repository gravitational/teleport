package generic

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	typescommon "github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/plugins/pluginsv1"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/lister"
)

type userHandler struct {
	common.Config
	Plugin *types.PluginV1
	common.NotImplementedHandler
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

	connInfo, err := h.getConnectorInfo()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	scimUser, err := conv.UserFromResource(req.GetResource(),
		conv.WithUserOptionClock(h.Clock),
		conv.WithLabels(additionalLabels),
		conv.WithConnectorRef(
			&types.ConnectorRef{
				ID:       connInfo.Name,
				Type:     connInfo.Type,
				Identity: req.GetResource().GetExternalId(),
			}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportUser, err := h.createOrUpdateUser(ctx, scimUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.UserToResource(teleportUser,
		conv.WithExternalIDFunc(userExternalID))
}

func (h *userHandler) createOrUpdateUser(ctx context.Context, scimUser types.User) (types.User, error) {
	user, err := h.CreateUser(ctx, scimUser)
	switch {
	case err == nil:
		return user, nil
	case trace.IsAlreadyExists(err):
		// In some cases, a user may already exist in the backend due to having logged in via an SSO connector.
		// We want to upgrade ephemeral users to be managed via SCIM if they match the SCIM SSO connector.
		// This approach ensures that the CreateUser operation does not fail and disrupt the provisioning flow.
		currentUser, err := h.GetUser(ctx, scimUser.GetName(), false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		connInfo, err := h.getConnectorInfo()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !userCreatedByConnector(currentUser, connInfo) {
			return nil, trace.AlreadyExists(
				"a user with the username %q already exists in Teleport and is not managed by the same %q SSO connector selected for the SCIM integration",
				scimUser.GetName(), connInfo.Name)
		}

		scimUser.SetRevision(currentUser.GetRevision())
		user, err = h.UpdateUser(ctx, scimUser)
		if err != nil {
			if trace.IsNotFound(err) || trace.IsCompareFailed(err) {
				// It's possible that after the GetUser call, the user was deleted manually or expired due to TTL (ephemeral user).
				// In that case, the provisioning flow should be retried.
				return nil, trace.Wrap(err, "encountered a transient error while provisioning user %q, please retry", scimUser.GetName())
			}
			return nil, trace.Wrap(err)
		}
		return user, nil
	default:
		return nil, trace.Wrap(err)
	}
}

func userCreatedByConnector(user types.User, connectorInfo *types.PluginSCIMSettings_ConnectorInfo) bool {
	if user.GetCreatedBy().Connector == nil {
		return false
	}
	userConnector := user.GetCreatedBy().Connector
	// Connector ID is not unique, we need to check both ID and Type since SCIM plugin supports both SAML and OIDC.
	return userConnector.ID == connectorInfo.Name && userConnector.Type == connectorInfo.Type
}

// ListResources lists all SCIM user resources.
func (h *userHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	l := lister.UserLister{
		Config: h.Config,
		Predicate: func(_ context.Context, user types.User) bool {
			return hasSCIMOrigin(user)
		},
		UserToResource: func(user types.User) (*scimpb.Resource, error) {
			groups, err := h.getAccessListsForUserFromCache(ctx, user.GetName())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conv.UserToResource(user,
				conv.WithExternalIDFunc(userExternalID),
				conv.WithGroupsAttr(groups),
			)
		},
	}

	return l.ListResources(ctx, req)
}

// GetResource retrieves a specific SCIM user resource by its ID.
func (h *userHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	userID := req.GetTarget().GetResourceId()

	teleportUser, err := h.GetUser(ctx, userID, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !hasSCIMOrigin(teleportUser) {
		return nil, trace.AccessDenied("user %q is not SCIM managed", userID)
	}

	// NOTE that groupsNames are obtained from cache.
	// This is acceptable since group membership is eventually where the "groups" user attribute is
	// don't need to be strongly consistent like user.revision.
	groupNames, err := h.getAccessListsForUserFromCache(ctx, teleportUser.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.UserToResource(
		teleportUser,
		conv.WithExternalIDFunc(userExternalID),
		conv.WithGroupsAttr(groupNames),
	)
}

// UpdateResource updates an existing SCIM user resource.
func (h *userHandler) UpdateResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	userID := req.GetTarget().GetResourceId()

	existingUser, err := h.GetUser(ctx, userID, false /* with secrets*/)
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

	updatedUser, err := h.UpdateUser(ctx, updatedSCIMUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.UserToResource(updatedUser, conv.WithExternalIDFunc(userExternalID))
}

// DeleteResource deletes a SCIM user resource by its ID.
func (h *userHandler) DeleteResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) error {
	userID := req.GetTarget().GetResourceId()

	teleportUser, err := h.GetUser(ctx, userID, false)
	if err != nil {
		return trace.Wrap(err)
	}

	if !hasSCIMOrigin(teleportUser) {
		return trace.NotFound("user %q is not SCIM managed", userID)
	}

	return trace.Wrap(h.DeleteUser(ctx, teleportUser.GetName()))
}

// getAccessListsForUserFromCache returns all access list names a given user is a member of.
func (h *userHandler) getAccessListsForUserFromCache(ctx context.Context, userID string) ([]string, error) {
	var groups []string

	for acl, err := range clientutils.Resources(ctx, h.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		_, err := h.GetAccessListMember(ctx, acl.GetName(), userID)
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

func (h *userHandler) getConnectorInfo() (*types.PluginSCIMSettings_ConnectorInfo, error) {
	connInfo, err := pluginsv1.GetSCIMPluginConnectorInfo(h.Plugin.Spec.GetScim())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connInfo, nil
}
