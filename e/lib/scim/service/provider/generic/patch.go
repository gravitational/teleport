package generic

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	scimclient "github.com/gravitational/teleport/api/client/scim"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	apiutils "github.com/gravitational/teleport/api/utils"
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

	// Fast path: if the entire request is expressible as a single
	// self-describing membership add/remove operation, apply it directly
	// against the backend - O(1) - instead of fetching the group's full
	// membership to compute a diff. Falls through to the existing
	// full-state flow for anything else (multiple ops, bulk replace,
	// displayName changes, malformed payloads, etc.).
	if memberOps, ok := g.canFastPatch(ctx, req); ok {
		resp, err := g.fastPatchResource(ctx, acl, memberOps)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resp, nil
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

// disableSCIMFastPatchLabel opts an individual SCIM plugin out of the
// membership PATCH fast path, as an escape hatch for a specific plugin
// without having to disable the fast path cluster-wide - see
// isSCIMFastPatchEnabled.
const disableSCIMFastPatchLabel = types.TeleportNamespace + "/disable-scim-fast-patch"

// isSCIMFastPatchEnabled reports whether the membership PATCH fast path is
// enabled. It's on by default it can be turned off cluster-wide via
// TELEPORT_UNSTABLE_DISABLE_SCIM_FAST_PATCH, or for a specific plugin via
// the disableSCIMFastPatchLabel label.
func isSCIMFastPatchEnabled(plugin *types.PluginV1) bool {
	envDisabled, _ := apiutils.ParseBool(os.Getenv("TELEPORT_UNSTABLE_DISABLE_SCIM_FAST_PATCH"))
	labelDisabled, _ := apiutils.ParseBool(plugin.GetMetadata().Labels[disableSCIMFastPatchLabel])
	return !envDisabled && !labelDisabled
}

// canFastPatch is the prerequisite for the fast path: it reports whether
// req's payload is expressible as a single self-describing membership
// add/remove operation, which can be applied directly against the backend
// - O(1) - instead of fetching the group's full membership to compute a
// diff. Anything else (multiple ops, bulk replace, displayName changes,
// a malformed payload, etc.) isn't eligible and must fall through to the
// full-state flow, which marshals and applies the payload again itself.
//
// It also requires the fast path to have been enabled - see
// isSCIMFastPatchEnabled - and the client to have declared it understands
// scimclient.NoContentTrailer: during a rolling upgrade, an old client
// talking to an already-upgraded auth wouldn't know to look for the
// trailer, so it must keep going through the full-state flow and getting
// the old, fully-populated response.
func (g groupHandler) canFastPatch(ctx context.Context, req *scimpb.PatchSCIMResourceRequest) ([]patch.MemberOp, bool) {
	if !isSCIMFastPatchEnabled(g.Plugin) {
		return nil, false
	}
	if !scimclient.ClientSupportsNoContent(ctx) {
		return nil, false
	}
	patchJSON, err := req.GetPayload().MarshalJSON()
	if err != nil {
		return nil, false
	}
	memberOps, ok := patch.ExtractMemberOps(patchJSON)
	return memberOps, ok && len(memberOps) == 1
}

// fastPatchResource applies a single self-describing membership op
// directly against the backend and builds the response for it - see
// fastPatch and scimclient.NoContentTrailer.
func (g groupHandler) fastPatchResource(ctx context.Context, acl *accesslist.AccessList, memberOps []patch.MemberOp) (*scimpb.Resource, error) {
	if err := g.fastPatch(ctx, acl, memberOps); err != nil {
		return nil, trace.Wrap(err)
	}
	// Flag on the trailer that the fast path applied the patch directly
	// against the backend, so the client can translate this into a 204 HTTP code.
	// allowing to skip the full response body, which is expensive to build and marshal in case
	// of large amount of members in group.
	if err := grpc.SetTrailer(ctx, metadata.Pairs(scimclient.NoContentTrailer, "true")); err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.AccessListToResource(acl, nil)
}

// fastPatch applies single operation membership changes directly against
// the backend, without ever  need to fetch whole collection of members.
// The add/remove member is O(1) instead of O(N) UpsertAccessListWithMembers call.
// The Access List resource itself is untouched, since none of these
// operations can change its title.
func (g groupHandler) fastPatch(ctx context.Context, acl *accesslist.AccessList, ops []patch.MemberOp) error {
	for _, op := range ops {
		switch op.Action {
		case patch.MemberAdd:
			if err := g.addMember(ctx, acl.GetName(), op.Value); err != nil {
				return trace.Wrap(err)
			}
		case patch.MemberRemove:
			// When members is deleted the access list revision is not bumped up.
			// This is acceptable since Revision is used in Update flow where SCIM clients
			// either use Update or Patch approach.
			if err := g.Backend.DeleteAccessListMember(ctx, acl.GetName(), op.Value); err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		g.Logger.With(
			slog.String("op", "fastPatch"),
			slog.String("action", op.Action.String()),
			slog.String("access_list", acl.GetName()),
			slog.String("member", op.Value),
		).DebugContext(ctx, "SCIM fast patch applied.")
	}
	return nil
}

func (g groupHandler) addMember(ctx context.Context, accessListName, memberName string) error {
	member, err := accesslist.NewAccessListMember(
		header.Metadata{Name: memberName},
		accesslist.AccessListMemberSpec{
			AccessList:       accessListName,
			Name:             memberName,
			Joined:           g.Clock.Now(),
			AddedBy:          "SCIM",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(smallinsky): Ideally we should call CreateAccessListMember that
	// is not yet supported.
	// But in SCIM flow the client is fully owner of the resource.
	// If PATCH member is called multiple on the same member the flow
	// will just update the field. This is acceptable since only Joined filed.
	_, err = g.Backend.UpsertAccessListMember(ctx, member)
	return trace.Wrap(err)
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
	userNameAttr, ok := updatedResource.GetAttributes().GetFields()[common.UsernameAttribute]
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
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(patchedJSON, out.GetAttributes()); err != nil {
		return nil, trace.Wrap(err)
	}

	log.DebugContext(ctx, "SCIM PATCH applied successfully",
		slog.Group("output",
			slog.Any("patched_attributes", json.RawMessage(patchedJSON)),
		),
	)
	return out, nil
}
