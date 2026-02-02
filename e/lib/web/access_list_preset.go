package web

import (
	"context"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/e/lib/accesslist/preset"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

// createAccessListWithPreset is the handler for POST /enterprise/accesslistpreset.
// It creates a new access list with preset configuration.
func (p *Plugin) createAccessListWithPreset(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	var req ui.AccessListWithPresetRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	aclClient := getAccessListServiceClient(sctx)

	resp, err := aclClient.CreateAccessListWithPreset(r.Context(), &accesslistv1.CreateAccessListWithPresetRequest{
		PresetType: req.PresetType,
		AccessList: conv.ToProto(req.AccessList.AccessList),
		Roles:      req.AccessRoles,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	acl, err := conv.FromProto(resp.AccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	members := make([]*accesslist.AccessListMember, 0, len(req.AccessList.Members))
	for _, member := range req.AccessList.Members {
		members = append(members, memberToAccessListMember(acl.GetName(), member))
	}
	acl, updatedMembers, err := upsertAccessListWithMembers(r.Context(), sctx, acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, member := range updatedMembers {
		enrichAccessListMemberTitle(r.Context(), clt.AccessListClient(), member)
	}
	for i := range acl.Spec.Owners {
		enrichAccessListOwnerTitle(r.Context(), clt.AccessListClient(), &acl.Spec.Owners[i])
	}

	return &ui.AccessListWithPresetResponse{
		AccessList: &ui.AccessList{
			AccessList: acl,
			Members:    membersToMembersSpec(updatedMembers),
		},
		AccessRoles: resp.GetRoles(),
	}, nil
}

// updateAccessListWithPreset is the handler for PUT /enterprise/accesslistpreset/:accessListId.
// It updates an existing access list that was created with a preset.
func (p *Plugin) updateAccessListWithPreset(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	accessListID, err := extractAccessListID(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.AccessListWithPresetRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if accessListID != req.AccessList.Metadata.Name {
		return nil, trace.BadParameter("accessList.metadata.name must match the accessListName from request")
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	aclClient := getAccessListServiceClient(sctx)
	resp, err := aclClient.UpdateAccessListWithPreset(r.Context(), &accesslistv1.UpdateAccessListWithPresetRequest{
		AccessList: conv.ToProto(req.AccessList.AccessList),
		Roles:      req.AccessRoles,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	acl, err := conv.FromProto(resp.AccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members := make([]*accesslist.AccessListMember, 0, len(req.AccessList.Members))
	for _, member := range req.AccessList.Members {
		members = append(members, memberToAccessListMember(accessListID, member))
	}
	acl, updatedMembers, err := upsertAccessListWithMembers(r.Context(), sctx, acl, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, member := range updatedMembers {
		enrichAccessListMemberTitle(r.Context(), clt.AccessListClient(), member)
	}
	for i := range acl.Spec.Owners {
		enrichAccessListOwnerTitle(r.Context(), clt.AccessListClient(), &acl.Spec.Owners[i])
	}

	return &ui.AccessListWithPresetResponse{
		AccessList: &ui.AccessList{
			AccessList: acl,
			Members:    membersToMembersSpec(updatedMembers),
		},
		AccessRoles: resp.GetRoles(),
	}, nil
}

func membersToMembersSpec(members []*accesslist.AccessListMember) []accesslist.AccessListMemberSpec {
	specs := make([]accesslist.AccessListMemberSpec, 0, len(members))
	for _, member := range members {
		specs = append(specs, member.Spec)
	}
	return specs
}

// deleteAccessListWithPreset is the handler for DELETE /enterprise/accesslistpreset/:accessListId
// It deletes an access list that was created with a preset, including all associated preset roles.
func (p *Plugin) deleteAccessListWithPreset(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	accessListID, err := extractAccessListID(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	acl, err := client.AccessListClient().GetAccessList(r.Context(), accessListID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := client.AccessListClient().DeleteAccessList(r.Context(), acl.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ui.DeleteAccessListWithPresetResponse{
		Roles: preset.ExtractRolesFromLabels(acl.GetAllLabels()),
	}, nil
}

// extractAccessListID extracts and validates the accessListId parameter.
func extractAccessListID(params httprouter.Params) (string, error) {
	accessListID := params.ByName("accessListId")
	if accessListID == "" {
		return "", trace.BadParameter("accessListID is required")
	}
	return accessListID, nil
}

// getAccessListServiceClient creates a new AccessListServiceClient from the session context.
func getAccessListServiceClient(sctx *web.SessionContext) accesslistv1.AccessListServiceClient {
	return accesslistv1.NewAccessListServiceClient(sctx.GetClientConnection())
}

// upsertAccessListWithMembers upserts an access list along with its members.
func upsertAccessListWithMembers(ctx context.Context, sctx *web.SessionContext, acl *accesslist.AccessList, members []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	client, err := sctx.GetClient()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return client.AccessListClient().UpsertAccessListWithMembers(ctx, acl, members)
}

// enrichAccessListMemberTitle enriches the member's title if the membership kind is MembershipKindList.
func enrichAccessListMemberTitle(ctx context.Context, client services.AccessLists, member *accesslist.AccessListMember) {
	if member.Spec.MembershipKind == accesslist.MembershipKindList {
		if al, err := getAccessListNoMFACtx(ctx, client, member.GetName()); err == nil {
			member.Spec.Title = al.Spec.Title
		}
	}
}

// enrichAccessListOwnerTitle enriches the owner's title if the membership kind is MembershipKindList.
func enrichAccessListOwnerTitle(ctx context.Context, client services.AccessLists, owner *accesslist.Owner) {
	if owner.MembershipKind == accesslist.MembershipKindList {
		if al, err := getAccessListNoMFACtx(ctx, client, owner.Name); err == nil {
			owner.Title = al.Spec.Title
		}
	}
}
