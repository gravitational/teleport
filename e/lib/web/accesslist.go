package web

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

type readOnlyAccessListError struct {
	accessListTitle string
	accessListType  accesslist.Type
}

func newReadOnlyAccessListError(accessListTitle string, accessListType accesslist.Type) *readOnlyAccessListError {
	return &readOnlyAccessListError{
		accessListTitle: accessListTitle,
		accessListType:  accessListType,
	}
}

func (e *readOnlyAccessListError) Error() string {
	return fmt.Sprintf(
		`Access list %[1]q is of type %[2]q and cannot be created or modified via web UI. Non-reviewable access lists (i.e., access_list with spec.type "scim" or "static") are currently modifiable only with Terraform and tctl.`,
		e.accessListTitle, e.accessListType,
	)
}

func (e *readOnlyAccessListError) Unwrap() error {
	return &trace.BadParameterError{
		Message: e.Error(),
	}
}

// listAccessLists is the handler for GET /v2/enterprise/accesslists with filtering and sorting support.
func (p *Plugin) listAccessLists(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()

	startKey := values.Get("startKey")

	limit, err := web.QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	searchFilter := values.Get("search")

	owners := slices.DeleteFunc(values["owners"], func(owner string) bool {
		return owner == ""
	})

	// default to title:asc
	sortBy := types.SortBy{
		Field:  "title",
		IsDesc: false,
	}
	sortParam := values.Get("sort")
	if sortParam != "" {
		sortBy = types.GetSortByFromString(values.Get("sort"))
	}

	req := &accesslistv1.ListAccessListsV2Request{
		PageToken: startKey,
		SortBy:    &sortBy,
		PageSize:  limit,
		Filter: &accesslistv1.AccessListsFilter{
			Search: searchFilter,
			Owners: owners,
			Origin: values.Get("origin"),
		},
	}
	accessListClient := clt.AccessListClient()

	page, nextKey, err := accessListClient.ListAccessListsV2(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*ui.AccessList, 0, len(page))

	for _, accessList := range page {
		uiList := &ui.AccessList{
			AccessList:             accessList,
			MembersCount:           accessList.GetStatus().MemberCount,
			MemberListCount:        accessList.GetStatus().MemberListCount,
			CurrentUserAssignments: accessList.GetStatus().CurrentUserAssignments,
		}
		accessLists = append(accessLists, uiList)
	}

	return ui.AccessListsResponse{
		AccessLists: accessLists,
		StartKey:    nextKey,
	}, nil
}

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

	for i, owner := range accessList.Spec.Owners {
		if owner.MembershipKind == accesslist.MembershipKindList {
			if al, err := accessListClient.GetAccessList(r.Context(), owner.Name); err == nil {
				accessList.Spec.Owners[i].Title = al.Spec.Title
			}
		}
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

func fillMemberListTitles(ctx context.Context, accessListClient services.AccessLists, members []*accesslist.AccessListMember) {
	for i, member := range members {
		switch member.Spec.MembershipKind {
		case accesslist.MembershipKindList:
			// we could get a not found error or an rbac error. if we dont get an error, get the title, otherwise ignore
			if fetchedAccessList, err := accessListClient.GetAccessList(ctx, member.GetName()); err == nil {
				members[i].Spec.Title = fetchedAccessList.Spec.Title
			}
		}
	}
}

// listAllMembers is a helper function to list all members of an access list.
func listAllMembers(ctx context.Context, accessListClient services.AccessLists, accessListId string) ([]*accesslist.AccessListMember, error) {
	var pageToken string
	var allMembers []*accesslist.AccessListMember

	for {
		var members []*accesslist.AccessListMember
		var err error

		members, pageToken, err = accessListClient.ListAccessListMembers(ctx, accessListId, 0 /* default page size */, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fillMemberListTitles(ctx, accessListClient, members)
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

	// Note, only the access list type in the request is being checked. That's sufficient
	// because access list types are immutable and backed will refuse to change the access
	// list type.
	if isUIReadOnlyAccessListType(req.Type) {
		return nil, trace.Wrap(newReadOnlyAccessListError(req.Title, req.Type))
	}

	accessListID := params.ByName("accessListId")
	if accessListID == "" {
		// Assume we are creating instead.
		accessListID = uuid.New().String()
	}

	accessListClient := clt.AccessListClient()

	oldAccessList, err := getAccessListNoMFACtx(r.Context(), accessListClient, accessListID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	accessList, err := accesslist.NewAccessList(header.Metadata{Name: accessListID}, req.Spec)
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
		members = append(members, memberToAccessListMember(accessListID, member))
	}

	createdAccessList, updatedMembers, err := accessListClient.UpsertAccessListWithMembers(r.Context(), accessList, members)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberSpecs := make([]accesslist.AccessListMemberSpec, 0, len(updatedMembers))
	for _, member := range updatedMembers {
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			if al, err := getAccessListNoMFACtx(r.Context(), accessListClient, member.GetName()); err == nil {
				member.Spec.Title = al.Spec.Title
			}
		}
		memberSpecs = append(memberSpecs, member.Spec)
	}

	for i := range createdAccessList.Spec.Owners {
		owner := &createdAccessList.Spec.Owners[i]
		if owner.MembershipKind == accesslist.MembershipKindList {
			if al, err := getAccessListNoMFACtx(r.Context(), accessListClient, owner.Name); err == nil {
				owner.Title = al.Spec.Title
			}
		}
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

	accessListID := params.ByName("accessListId")
	accessListClient := clt.AccessListClient()

	existingAccessList, err := getAccessListNoMFACtx(r.Context(), accessListClient, accessListID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if existingAccessList != nil && isUIReadOnlyAccessListType(existingAccessList.Spec.Type) {
		return nil, trace.Wrap(newReadOnlyAccessListError(existingAccessList.Spec.Title, existingAccessList.Spec.Type))
	}

	if err := accessListClient.DeleteAccessList(r.Context(), accessListID); err != nil {
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

func getAccessListNoMFACtx(ctx context.Context, clt services.AccessLists, name string) (*accesslist.AccessList, error) {
	// Remove the MFA resp from the context before getting the access list.
	// Otherwise, it will be consumed before the operation which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	noMFACtx := mfa.ContextWithMFAResponse(ctx, nil)
	accessList, err := clt.GetAccessList(noMFACtx, name)
	return accessList, trace.Wrap(err)
}

// listUserAccessLists is the handler for GET /enterprise/users/:username/accesslists.
func (p *Plugin) listUserAccessLists(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext) (any, error) {
	username := params.ByName("username")
	if username == "" {
		return nil, trace.BadParameter("missing username")
	}

	values := r.URL.Query()

	startKey := values.Get("startKey")

	limit, err := web.QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	aclClient := getAccessListServiceClient(sctx)

	resp, err := aclClient.ListUserAccessLists(r.Context(), &accesslistv1.ListUserAccessListsRequest{
		Username:  username,
		PageSize:  limit,
		PageToken: startKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*ui.AccessList, 0, len(resp.AccessLists))
	for _, protoAcl := range resp.AccessLists {
		accessList, err := conv.FromProto(protoAcl, conv.WithOwnersIneligibleStatusField(protoAcl.GetSpec().GetOwners()))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiList := &ui.AccessList{
			AccessList:             accessList,
			MembersCount:           accessList.GetStatus().MemberCount,
			MemberListCount:        accessList.GetStatus().MemberListCount,
			CurrentUserAssignments: accessList.GetStatus().CurrentUserAssignments,
			UserAssignments:        accessList.GetStatus().UserAssignments,
		}
		accessLists = append(accessLists, uiList)
	}

	return ui.AccessListsResponse{
		AccessLists: accessLists,
		StartKey:    resp.NextPageToken,
		TotalCount:  resp.TotalCount,
	}, nil
}

// isUIReadOnlyAccessListType returns true if the AccessList type is static. Those access lists are
// supposed to be managed only by the IaC tools. It may change in the future.
func isUIReadOnlyAccessListType(typ accesslist.Type) bool {
	return typ == accesslist.Static
}
