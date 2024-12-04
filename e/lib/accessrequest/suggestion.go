package accessrequest

import (
	"context"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/accessrequest"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type userDataGetter interface {
	modules.RoleGetter
	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
}

type AccessListLister interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
}

// GetSuggestedAccessLists returns a list of access lists that are suggested for a given request.
func GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt modules.AccessListSuggestionClient,
	accessListGetter modules.AccessListAndMembersGetter, requestID string,
) ([]*accesslist.AccessList, error) {
	accessRequests, err := clt.GetAccessRequests(ctx, types.AccessRequestFilter{ID: requestID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(accessRequests) != 1 {
		return nil, trace.NotFound("access request %q not found", requestID)
	}
	accessRequest := accessRequests[0]

	// Fetch the allowed promotions for the access request and convert them
	// into suggestions that the current reviewer can approve.
	allowedPromotions, err := clt.GetAccessRequestAllowedPromotions(ctx, accessRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Filter out access lists that the reviewer cannot modify using in-place truncate.
	reviewer, err := clt.GetUser(ctx, identity.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslist.AccessList, 0, len(allowedPromotions.Promotions))
	for _, promotion := range allowedPromotions.Promotions {
		accList, err := accessListGetter.GetAccessList(ctx, promotion.AccessListName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		accessLists = append(accessLists, accList)
	}

	cursor := 0
	for _, accessList := range accessLists {
		// Reviewer must be the owner of the access list to be able to approve it.
		err := canModifyAccessList(ctx, clt, reviewer, accessList, accessListGetter)
		switch {
		case err == nil:
			// write the access list to it's potentially new position
			accessLists[cursor] = accessList
			cursor++
		case trace.IsAccessDenied(err):
			// do nothing, we'll truncate out the denied lists later
		default:
			// unexpected error, abort
			return nil, trace.Wrap(err)
		}
	}

	ranked := ScoreRelevance(accessRequest, accessLists[:cursor])

	return ranked, nil
}

func canModifyAccessList(ctx context.Context, clt modules.RoleGetter, reviewer types.User, accessList *accesslist.AccessList, accessListGetter modules.AccessListAndMembersGetter) error {
	// if owner, then can list and modify
	ownershipType, err := accesslists.IsAccessListOwner(ctx, reviewer, accessList, accessListGetter, nil, clockwork.NewRealClock())
	// Owner is inherited or explicit
	if ownershipType != accesslists.MembershipOrOwnershipTypeNone {
		return nil
	}
	if !trace.IsAccessDenied(err) {
		return trace.Wrap(err)
	}

	// If the reviewer is not an owner, check if they have access to modify the access list.
	accessChecker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles:  reviewer.GetRoles(),
		Traits: reviewer.GetTraits(),
	}, "", clt)
	if err != nil {
		return trace.Wrap(err)
	}

	// check if the reviewer can modify this access list
	authErrCreate := accessChecker.CheckAccessToRule(&services.Context{User: reviewer, Resource: accessList}, apidefaults.Namespace, types.KindAccessList, types.VerbCreate)
	authErrUpdate := accessChecker.CheckAccessToRule(&services.Context{User: reviewer, Resource: accessList}, apidefaults.Namespace, types.KindAccessList, types.VerbUpdate)
	authErr := trace.NewAggregate(authErrCreate, authErrUpdate)
	switch {
	case authErr == nil:
		return nil
	case trace.IsAccessDenied(authErr):
		return trace.AccessDenied("access denied to modify access list")
	default:
		return trace.Wrap(authErr)
	}
}

type scoredAccessList struct {
	list *accesslist.AccessList
	// score is the score of the access list. Higher scores are more relevant.
	// The score can be any valid integer.
	score int
}

// ScoreRelevance returns a list of access lists sorted by their relevance to the given access request.
func ScoreRelevance(request types.AccessRequest, lists []*accesslist.AccessList) []*accesslist.AccessList {
	scores := make([]scoredAccessList, 0, len(lists))

	for _, list := range lists {
		score := computeAccessListRelevancy(request.GetRoles(), list)

		scores = append(scores, scoredAccessList{
			list:  list,
			score: score,
		})
	}

	slices.SortFunc(scores, func(a, b scoredAccessList) int {
		switch {
		case a.score < b.score:
			return 1
		case a.score > b.score:
			return -1
		default:
			return 0
		}
	})

	for i, scoredList := range scores {
		lists[i] = scoredList.list
	}

	return lists[:len(scores)]
}

type suggestionValidator struct {
	dataGetter         userDataGetter
	requester          types.User
	requestedResources []types.ResourceWithLabels

	clock clockwork.Clock
}

func (v *suggestionValidator) isValidSuggestion(ctx context.Context, list *accesslist.AccessList) (bool, error) {
	// If the user is already a member, or doesn't meet the requirements to be assigned to the access list,
	// then the access list is not a valid suggestion.
	membershipType, err := accesslists.IsAccessListMember(ctx, v.requester, list, v.dataGetter, nil, v.clock)
	if err != nil && !trace.IsAccessDenied(err) {
		return false, trace.Wrap(err)
	}
	// If the user is not a member, then the access list may be a valid suggestion.
	if membershipType != accesslists.MembershipOrOwnershipTypeNone {
		return false, nil
	}
	// Access lists not assignable to the user are irrelevant.
	if !accesslists.UserMeetsRequirements(v.requester, list.GetMembershipRequires()) {
		return false, nil
	}

	// Access lists that don't provide access to the requested resources are irrelevant
	accessChecker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles:  list.GetGrants().Roles,
		Traits: map[string][]string(list.GetGrants().Traits),
	}, "", v.dataGetter)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, resource := range v.requestedResources {
		select {
		case <-ctx.Done():
			return false, trace.Wrap(ctx.Err())
		default:
		}

		err := accessChecker.CheckAccess(resource, services.AccessState{MFAVerified: true})
		switch {
		case trace.IsAccessDenied(err):
			return false, nil
		case err != nil:
			return false, trace.Wrap(err)
		}
	}

	return true, nil
}

func computeAccessListRelevancy(requestRoles []string, list *accesslist.AccessList) int {
	const (
		roleNegativeWeight = -4
		rolePositiveWeight = 2
	)

	score := 0
	grantedRolesNames := list.GetGrants().Roles

	for _, grantedRole := range grantedRolesNames {
		if !slices.Contains(requestRoles, grantedRole) {
			// Penalize access lists that provide access to roles that were not requested.
			score += roleNegativeWeight
		} else {
			// Reward access lists that provide access to roles that were requested.
			score += rolePositiveWeight
		}
	}

	return score
}

// GenerateAccessRequestPromotions returns a list of Access Lists that are suggested for a given
// Access Request. The list is always empty for role-based requests. An Access List is included in
// the returned list if it allows all requested resources and the user is not a member of the
// Access List.
func GenerateAccessRequestPromotions(ctx context.Context, resourceGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	if len(accessRequest.GetRequestedResourceIDs()) == 0 {
		// Suggestions are only available for resource-based access requests.
		return types.NewAccessRequestAllowedPromotions(nil), nil
	}

	requester, err := resourceGetter.GetUser(ctx, accessRequest.GetUser(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, err := accessrequest.GetResourcesByResourceIDs(ctx, resourceGetter, accessRequest.GetRequestedResourceIDs())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedPromotions := types.NewAccessRequestAllowedPromotions(nil)

	validator := suggestionValidator{
		dataGetter:         resourceGetter,
		requester:          requester,
		requestedResources: resources,
		clock:              clockwork.NewRealClock(),
	}

	allAccessLists, err := resourceGetter.GetAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, accessList := range allAccessLists {
		valid, err := validator.isValidSuggestion(ctx, accessList)
		if err != nil {
			slog.Log(ctx, logutils.TraceLevel, "failed to validate access list suggestion", "error", err)
			continue
		}

		if !valid {
			continue
		}

		allowedPromotions.Promotions = append(allowedPromotions.Promotions, &types.AccessRequestAllowedPromotion{
			AccessListName: accessList.GetName(),
		})
	}

	return allowedPromotions, nil
}
