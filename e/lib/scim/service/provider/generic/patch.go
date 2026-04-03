package generic

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/patch"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	libaccesslist "github.com/gravitational/teleport/lib/accesslists"
)

// PatchResource patches an existing SCIM group resource using SCIM PATCH operations as per RFC 7644 Section 3.5.2.
func (g groupHandler) PatchResource(ctx context.Context, req *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	resourceID := req.GetTarget().GetResourceId()
	acl, err := g.Backend.GetAccessList(ctx, resourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !accessListPredicate(acl) {
		return nil, trace.NotFound("access list %q not found", resourceID)
	}
	members, err := libaccesslist.GetMembersFor(ctx, resourceID, g.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentResource, err := conv.AccessListToResource(acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := applyPatchOperations(ctx, g.Logger, currentResource, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updatedACL, updatedMembers, err := conv.AccessListFromResource(
		updated,
		conv.WithClock(g.Clock),
		conv.WithMemberAddedBy("SCIM"),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if updatedACL.GetRevision() != "" {
		acl.SetRevision(updatedACL.GetRevision())
	}
	// Access List have only title field and membership that can be updated via SCIM flow
	// Other field are immutable via SCIM
	acl.Spec.Title = updatedACL.Spec.Title

	// Apply the updates via CAS operation UpdateAccessListAndOverwriteMembers where the access list revision is checked
	// to avoid lost updates due to concurrent modifications.
	// In current state the PATCH operation relies on distributes lock to avoid concurrent modifications via SCIM API,
	// But the access list could be modified via other means (e.g. via CLI or UI).
	newACL, newMembers, err := g.UpdateAccessListAndOverwriteMembers(ctx, acl, updatedMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.AccessListToResource(newACL, newMembers)
}

// PatchResource patches an existing SCIM user resource using SCIM PATCH operations as per RFC 7644 Section 3.5.2.
func (h *userHandler) PatchResource(ctx context.Context, req *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	userID := req.GetTarget().GetResourceId()

	existingUser, err := h.GetUser(ctx, userID, false /* with secrets*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !hasSCIMOrigin(existingUser) {
		return nil, trace.NotFound("user %q not found", userID)
	}

	currentResource, err := conv.UserToResource(existingUser, conv.WithExternalIDFunc(userExternalID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := applyPatchOperations(ctx, h.Logger, currentResource, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The SCIM PATCH operation should not try to change the userName attribute
	// From Teleport side the userName is the unique identifier of the user
	// and the SCIM flow should not try to change it.
	// The scim client should respect the SCIM User schema where the userName is marked as immutable.
	// Refer to https://learn.microsoft.com/en-us/answers/questions/5560718/scim-validator-fails-on-username-update-test-is-us
	// for more details about this behavior.
	if err := h.checkForUserNameChange(ctx, existingUser, updated); err != nil {
		return nil, trace.Wrap(err)
	}

	updatedSCIMUser, err := conv.UserFromResource(updated,
		conv.WithUserOptionClock(h.Clock),
		conv.WithLabels(existingUser.GetAllLabels()),
		conv.WithConnectorRef(existingUser.GetCreatedBy().Connector),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if updatedSCIMUser.GetRevision() == "" {
		updatedSCIMUser.SetRevision(existingUser.GetRevision())
	}
	// Use CAS operation to avoid lost updates due to concurrent modifications.
	updatedUser, err := h.UpdateUser(ctx, updatedSCIMUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.UserToResource(updatedUser, conv.WithExternalIDFunc(userExternalID))
}

func (h *userHandler) checkForUserNameChange(ctx context.Context, existingUser types.User, updatedResource *scimpb.Resource) error {
	userNameAttr, ok := updatedResource.Attributes.GetFields()[common.UsernameAttribute]
	if !ok {
		return nil
	}
	if existingUser.GetName() == userNameAttr.GetStringValue() {
		return nil
	}

	h.Logger.With(
		slog.String("user_id", existingUser.GetName()),
		slog.String("existing_username", existingUser.GetName()),
		slog.String("updated_username", userNameAttr.GetStringValue()),
	).InfoContext(ctx, "SCIM PATCH request userName attribute change was rejected.")

	return trace.BadParameter("updating userName is not allowed. Teleport SCIM Schema defines userName as immutable.")
}

// applyPatchOperations applies SCIM PATCH operations to a resource.
// The resource is modified in place, with its Attributes field updated to reflect
// the patched state.
func applyPatchOperations(ctx context.Context, log *slog.Logger, in *scimpb.Resource, req *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	currentJSON, err := in.GetAttributes().MarshalJSON()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	patchJSON, err := req.GetPayload().MarshalJSON()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log = log.With(
		slog.String("op", "SCIMHandler/applyPatchOperations"),
		slog.Group("input",
			slog.String("resource_id", in.GetId()),
			slog.Any("current_attributes", json.RawMessage(currentJSON)),
			slog.Any("patch_payload", json.RawMessage(patchJSON)),
		),
	)

	patchedJSON, err := patch.Apply(currentJSON, patchJSON)
	if err != nil {
		log.ErrorContext(ctx, "SCIM Patch failed", "error", err)
		return nil, trace.Wrap(err)
	}
	out, ok := proto.Clone(in).(*scimpb.Resource)
	if !ok {
		return out, trace.BadParameter("could not clone resource")
	}
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(patchedJSON, out.Attributes); err != nil {
		return nil, trace.Wrap(err)
	}

	log.DebugContext(ctx, "SCIM PATCH applied successfully",
		slog.Group("output",
			slog.Any("patched_attributes", json.RawMessage(patchedJSON)),
		),
	)
	return out, nil
}
