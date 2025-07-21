package web

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

// getAccessLists is the handler for GET /v1/enterprise/accesslist.
func (p *Plugin) getAccessLists(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListClient := clt.AccessListClient()

	// This will grab all access lists but in small chunks to not overload the grpc client.
	// The web UI won't require "paginating" because we don't expect access lists to get
	// in the thousands.
	var accessLists []*ui.AccessList
	var nextKey string
	for {
		var page []*accesslist.AccessList
		var err error

		page, nextKey, err = accessListClient.ListAccessLists(r.Context(), 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, accessList := range page {
			uiList := &ui.AccessList{
				AccessList:             accessList,
				MembersCount:           accessList.GetStatus().MemberCount,
				MemberListCount:        accessList.GetStatus().MemberListCount,
				CurrentUserAssignments: accessList.GetStatus().CurrentUserAssignments,
			}
			accessLists = append(accessLists, uiList)
		}

		if nextKey == "" {
			break
		}
	}

	return ui.AccessListsResponse{
		AccessLists: accessLists,
	}, nil
}

// getAccessList is the handler for GET /v1/enterprise/accesslist/:accessListId.
func (p *Plugin) getAccessList(_ http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListId := params.ByName("accessListId")
	accessListClient := clt.AccessListClient()

	accessList, err := accessListClient.GetAccessList(r.Context(), accessListId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	members, err := listAllMembers(r.Context(), accessListClient, accessListId)
	// If the user doesn't have access to the access list, we want to return the access list
	// without the members.
	if err != nil && !trace.IsAccessDenied(err) {
		return nil, trace.Wrap(err)
	}

	membersSpec := make([]accesslist.AccessListMemberSpec, 0, len(members))
	for _, member := range members {
		membersSpec = append(membersSpec, member.Spec)
	}

	inheritedGrants, err := accessListClient.GetInheritedGrants(r.Context(), accessListId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := ui.AccessListResponse{
		AccessList: &ui.AccessList{
			AccessList:             accessList,
			Members:                membersSpec,
			MembersCount:           accessList.GetStatus().MemberCount,
			MemberListCount:        accessList.GetStatus().MemberListCount,
			InheritedMemberGrants:  *inheritedGrants,
			CurrentUserAssignments: accessList.GetStatus().CurrentUserAssignments,
		},
	}

	return resp, nil
}

// listAllMembers is a helper function to list all members of an access list.
func listAllMembers(ctx context.Context, accessListClient services.AccessLists, accessListId string) ([]*accesslist.AccessListMember, error) {
	var pageToken string
	allMembers := make([]*accesslist.AccessListMember, 0)

	for {
		var members []*accesslist.AccessListMember
		var err error

		members, pageToken, err = accessListClient.ListAccessListMembers(ctx, accessListId, 0 /* default page size */, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allMembers = append(allMembers, members...)

		if pageToken == "" {
			break
		}
	}

	return allMembers, nil
}

// upsertAccessList is the handler for POST and PUT /v1/enterprise/accesslist.
func (p *Plugin) upsertAccessList(_ http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.UpsertAccessListRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	accessListId := params.ByName("accessListId")
	if accessListId == "" {
		// Assume we are creating instead.
		accessListId = uuid.New().String()
	}

	accessListClient := clt.AccessListClient()

	// Remove the MFA resp from the context before getting the access list.
	// Otherwise, it will be consumed before the Upsert which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	getAccessListCtx := mfa.ContextWithMFAResponse(r.Context(), nil)
	oldAccessList, err := accessListClient.GetAccessList(getAccessListCtx, accessListId)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	accessList, err := accesslist.NewAccessList(header.Metadata{Name: accessListId}, req.Spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure the old metadata is reflected in the new access list. This will preserve labels, expiration date,
	// description, etc.
	if oldAccessList != nil {
		accessList.Metadata = oldAccessList.Metadata
	}

	// Convert members
	members := make([]*accesslist.AccessListMember, 0, len(req.Members))
	for _, member := range req.Members {
		members = append(members, memberToAccessListMember(accessListId, member))
	}

	createdAccessList, updatedMembers, err := accessListClient.UpsertAccessListWithMembers(r.Context(), accessList, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberSpecs := make([]accesslist.AccessListMemberSpec, 0, len(updatedMembers))
	for _, member := range updatedMembers {
		memberSpecs = append(memberSpecs, member.Spec)
	}

	return ui.AccessListResponse{
		AccessList: &ui.AccessList{
			AccessList: createdAccessList,
			Members:    memberSpecs,
		},
	}, nil
}

// deleteAccessList is the handler for DELETE /v1/enterprise/accesslist/:accessListId.
func (p *Plugin) deleteAccessList(_ http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListId := params.ByName("accessListId")
	accessListClient := clt.AccessListClient()

	// First, delete all members.
	if err := accessListClient.DeleteAllAccessListMembersForAccessList(r.Context(), accessListId); err != nil {
		return nil, trace.Wrap(err)
	}

	// Then, delete the access list.
	if err := accessListClient.DeleteAccessList(r.Context(), accessListId); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

// addMembersToAccessList is the handler for POST /v1/enterprise/accesslist/:accessListId/members.
func (p *Plugin) addMembersToAccessList(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	var req ui.UpsertAccessListRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListId := params.ByName("accessListId")
	accessListClient := clt.AccessListClient()

	var addedMembers []accesslist.AccessListMemberSpec
	for _, member := range req.Members {
		alMember := memberToAccessListMember(accessListId, member)

		upsertMember, err := accessListClient.UpsertAccessListMember(r.Context(), alMember)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		addedMembers = append(addedMembers, upsertMember.Spec)
	}

	return ui.AddAccessListMemberResponse{
		Members: addedMembers,
	}, nil
}

func memberToAccessListMember(accessListName string, member accesslist.AccessListMemberSpec) *accesslist.AccessListMember {
	return &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Kind:    types.KindAccessListMember,
			Version: types.V3,
			Metadata: header.Metadata{
				Name: member.Name,
			},
		},
		Spec: accesslist.AccessListMemberSpec{
			AccessList:     accessListName,
			Name:           member.Name,
			Joined:         member.Joined,
			Expires:        member.Expires,
			Reason:         member.Reason,
			AddedBy:        member.AddedBy,
			MembershipKind: member.MembershipKind,
		},
	}
}

// reviewAccessList is the handler for POST /v1/enterprise/accesslist/:accessListId/review.
func (p *Plugin) reviewAccessList(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	var req ui.ReviewAccessListRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The following fields has to be filled, even though
	// they get set (replaced) in the back. These fillers are
	// required because the request converting from proto
	// does a check that these fields are not empty:
	//  - header.Metadata.Name
	//  - reviewSpec.Reviewers  // filled by client web UI
	//  - reviewSpec.ReviewDate // filled by client web UI
	review, err := accesslist.NewReview(header.Metadata{Name: uuid.New().String()}, req.ReviewSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, nextReviewDate, err := clt.AccessListClient().CreateAccessListReview(r.Context(), review)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui.ReviewAccessListResponse{
		NextAuditDate: nextReviewDate,
	}, nil
}

// listAccessListReviews is the handler for GET /enterprise/accesslist/:accessListId/reviews.
func (p *Plugin) listAccessListReviews(_ http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListId := params.ByName("accessListId")
	accessListClient := clt.AccessListClient()

	values := r.URL.Query()

	limit, err := web.QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startKey := values.Get("startKey")

	reviews, nextKey, err := accessListClient.ListAccessListReviews(r.Context(), accessListId, int(limit), startKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AccessListReviewsResponse{
		Reviews:  reviews,
		StartKey: nextKey,
	}, nil
}
