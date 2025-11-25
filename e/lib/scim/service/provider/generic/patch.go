package generic

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/patch"
	libaccesslist "github.com/gravitational/teleport/lib/accesslists"
)

// PatchResource patches an existing SCIM group resource using SCIM PATCH operations as per RFC 7644 Section 3.5.2.
func (g groupHandler) PatchResource(ctx context.Context, req *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	resourceID := req.GetTarget().GetResourceId()
	acl, err := g.AccessListGetter.GetAccessList(ctx, resourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !accessListPredicate(acl) {
		return nil, trace.NotFound("access list %q not found", resourceID)
	}
	members, err := libaccesslist.GetMembersFor(ctx, resourceID, g.AccessListsService)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentResource, err := conv.AccessListToResource(acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := applyPatchOperations(currentResource, req)
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
	newACL, newMembers, err := g.AccessListsService.UpdateAccessListAndOverwriteMembers(ctx, acl, updatedMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.AccessListToResource(newACL, newMembers)
}

// applyPatchOperations applies SCIM PATCH operations to a resource.
// The resource is modified in place, with its Attributes field updated to reflect
// the patched state.
func applyPatchOperations(in *scimpb.Resource, req *scimpb.PatchSCIMResourceRequest) (*scimpb.Resource, error) {
	currentJSON, err := in.GetAttributes().MarshalJSON()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	patchJSON, err := req.GetPayload().MarshalJSON()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	patchedJSON, err := patch.Apply(currentJSON, patchJSON)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, ok := proto.Clone(in).(*scimpb.Resource)
	if !ok {
		return out, trace.BadParameter("could not clone resource")
	}
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(patchedJSON, out.Attributes); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}
