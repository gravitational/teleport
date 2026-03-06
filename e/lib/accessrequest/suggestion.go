package accessrequest

import (
	"context"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/accessrequest"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/parse"
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

// GenerateLongTermResourceGrouping analyzes how resources can be grouped into access lists
// and returns information about optimal groupings for long-term access. This helps users
// understand which resources can be requested together for long-term access.
func GenerateLongTermResourceGrouping(ctx context.Context, clt modules.AccessResourcesGetter, request types.AccessRequest) (*types.LongTermResourceGrouping, error) {
	var resourceIDs []types.ResourceID

	suggestion := &types.LongTermResourceGrouping{
		AccessListToResources: make(map[string]types.ResourceIDList),
		RecommendedAccessList: "",
		CanProceed:            true,
		ValidationMessage:     "",
	}

	// Filter out any 'namespace' or 'windows_desktop' resource IDs, they are not supported.
	// TODO(kiosion): These should be supported by `ListResources`, see #58184
	for _, rid := range request.GetRequestedResourceIDs() {
		switch rid.Kind {
		case types.KindWindowsDesktop, types.KindNamespace:
			continue
		default:
			resourceIDs = append(resourceIDs, rid)
		}
	}

	if len(resourceIDs) == 0 {
		suggestion.CanProceed = false
		suggestion.ValidationMessage = "No resources are available for long-term access"
		return suggestion, nil
	}

	requester, err := clt.GetUser(ctx, request.GetUser(), false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We don't allow long-term requests where the resources are in different clusters.
	// This is to match the current behavior of request Promotions, which are not available
	// for resources in different clusters.
	if err := validateResourcesAreFromSameCluster(resourceIDs); err != nil {
		suggestion.CanProceed = false
		suggestion.ValidationMessage = err.Error()
		return suggestion, nil
	}

	// Get the actual resources from the backend to ensure we're using verified data
	resources, err := accessrequest.GetResourcesByResourceIDs(ctx, clt, resourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListToResources := make(map[string]types.ResourceIDList)
	var recommendedList string

	analysisInput := accessListAnalysisInput{
		Clock:       clockwork.NewRealClock(),
		Clt:         clt,
		Requester:   requester,
		ResourceIDs: resourceIDs,
		Resources:   resources,
	}

	allAccessLists, err := clt.GetAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// For each access list, analyze which resources would be covered
	maxResources := 0
	for _, accessList := range allAccessLists {
		analysisInput.AccessList = accessList
		covered, err := analyzeAccessListForLongTermAccess(ctx, analysisInput)
		if err != nil {
			slog.Log(ctx, slog.LevelDebug, "failed to analyze access list for long-term access", "error", err)
			continue
		}
		if len(covered) == 0 {
			continue
		}

		accessListToResources[accessList.GetName()] = types.ResourceIDList{ResourceIds: covered}

		// Update the recommended list if this one covers more resources
		if len(covered) > maxResources {
			maxResources = len(covered)
			recommendedList = accessList.GetName()
			// Elif this access list covers the same number of resources,
			// and we don't have a recommended list yet, choose this one
		} else if len(covered) == maxResources && recommendedList == "" {
			recommendedList = accessList.GetName()
		}
	}

	suggestion.AccessListToResources = accessListToResources
	suggestion.RecommendedAccessList = recommendedList

	// If no access lists are available for any of these resources
	if len(accessListToResources) == 0 || recommendedList == "" || len(accessListToResources[recommendedList].ResourceIds) == 0 {
		suggestion.CanProceed = false
		suggestion.ValidationMessage = "Long-term access is not available for any selected resources"
		return suggestion, nil
	}

	// If any resources are uncovered
	if uncovered := findUncoveredResources(request, accessListToResources); len(uncovered) > 0 {
		suggestion.CanProceed = false
		suggestion.ValidationMessage = "Long-term access is not available for some selected resources"
		return suggestion, nil
	}

	// If any resources are somehow covered, but not by the recommended list's grouping
	if conflicting := findConflictingResources(request, accessListToResources, recommendedList); len(conflicting) > 0 {
		suggestion.CanProceed = false
		suggestion.ValidationMessage = "Selected resources cannot be grouped for long-term access"
	}

	return suggestion, nil
}

// findConflictingResources checks which resources are not covered by the recommended access list's resource grouping.
func findConflictingResources(request types.AccessRequest, accessListToResources map[string]types.ResourceIDList, recommendedList string) (conflicting []types.ResourceID) {
	if len(accessListToResources) == 0 {
		return conflicting
	}
	optimalSet := buildResourceIDSet(accessListToResources[recommendedList].ResourceIds)
	for _, r := range request.GetAllRequestedResourceIDs() {
		rid := r.GetResourceID()
		k := types.ResourceIDToString(rid)
		if _, ok := optimalSet[k]; !ok {
			conflicting = append(conflicting, rid)
		}
	}
	return conflicting
}

// findUncoveredResources returns requested resources that aren't covered by any access list.
func findUncoveredResources(
	request types.AccessRequest,
	accessListToResources map[string]types.ResourceIDList,
) (uncovered []types.ResourceID) {
	capCovered := 0
	for _, rs := range accessListToResources {
		capCovered += len(rs.ResourceIds)
	}
	covered := make(map[string]struct{}, capCovered)
	for _, rs := range accessListToResources {
		for _, r := range rs.ResourceIds {
			covered[types.ResourceIDToString(r)] = struct{}{}
		}
	}

	req := request.GetAllRequestedResourceIDs()
	seen := make(map[string]struct{}, len(req))
	for _, r := range req {
		rid := r.GetResourceID()
		k := types.ResourceIDToString(rid)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if _, ok := covered[k]; !ok {
			uncovered = append(uncovered, rid)
		}
	}
	return uncovered
}

type accessListAnalysisInput struct {
	Clock       clockwork.Clock
	Clt         modules.AccessResourcesGetter
	Requester   types.User
	ResourceIDs []types.ResourceID
	Resources   []types.ResourceWithLabels
	AccessList  *accesslist.AccessList
}

// analyzeAccessListForLongTermAccess checks requester membership and requirements for the access list,
// then checks which resources the access list would cover if the requester were to be assigned to it.
func analyzeAccessListForLongTermAccess(ctx context.Context, in accessListAnalysisInput) (covered []types.ResourceID, err error) {
	canUse, err := validateCanUseAccessList(ctx, in.AccessList, in.Requester, in.Clt, in.Clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !canUse {
		return nil, nil
	}

	// Create an access checker for this access list's grants (incl. inherited)
	grants, err := getInheritedGrants(ctx, in.AccessList, in.Clt)
	if err != nil {
		return nil, trace.Wrap(err, "getting inherited grants for access list")
	}

	checker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles:  grants.Roles,
		Traits: map[string][]string(grants.Traits),
	}, "", in.Clt)
	if err != nil {
		return nil, trace.Wrap(err, "initializing accessChecker for access list")
	}

	// Check which resources this access list would grant access to
	for _, res := range in.Resources {
		if err := checker.CheckAccess(res, services.AccessState{MFAVerified: true}); err != nil {
			continue
		}
		for _, rid := range in.ResourceIDs {
			// TODO(kiosion): Should be some helper for this check; single 'source-of-truth' for ResourceID->Resource mapping
			if rid.Name == res.GetName() && rid.Kind == res.GetKind() {
				covered = append(covered, rid)
				break
			}
		}
	}

	if len(covered) == 0 {
		return nil, nil
	}

	return covered, nil
}

// validateCanUseAccessList checks if the requester can be assigned as a member of the access list.
func validateCanUseAccessList(ctx context.Context, al *accesslist.AccessList, user types.User, clt modules.AccessResourcesGetter, clock clockwork.Clock) (bool, error) {
	// Check if the user is already a member or doesn't meet requirements
	_, err := accesslists.IsAccessListMember(ctx, user, al, clt, nil, clock)
	// IsAccessListMember returns AccessDenied if the user is not a valid OR current member.
	// Here, "not a member" is the precondition to consider assignment, so we proceed on AccessDenied.
	if err != nil && !trace.IsAccessDenied(err) {
		return false, trace.Wrap(err, "checking access list membership")
	}

	// Ensure the user meets the requirements, including any inherited requires
	requires, err := accesslists.GetInheritedMembershipRequires(ctx, al, clt)
	if err != nil {
		return false, trace.Wrap(err, "getting inherited membershipRequires for access list")
	}

	if !accesslists.UserMeetsRequirements(user, *requires) {
		return false, nil
	}

	return true, nil
}

// validateResourcesAreFromSameCluster checks that all provided resource IDs are from the same cluster.
func validateResourcesAreFromSameCluster(resourceIDs []types.ResourceID) error {
	if len(resourceIDs) == 0 {
		return trace.BadParameter("No resources provided for long-term access suggestion")
	}
	firstClusterName := resourceIDs[0].ClusterName
	for _, rid := range resourceIDs[1:] {
		if rid.ClusterName != "" && rid.ClusterName != firstClusterName {
			return trace.BadParameter("Long-term access is not available for resources in different clusters")
		}
	}
	return nil
}

// getInheritedGrants combines an access list's own grants with those inherited from ancestor lists it has membership in.
func getInheritedGrants(ctx context.Context, a *accesslist.AccessList, clt modules.AccessResourcesGetter) (*accesslist.Grants, error) {
	inherited, err := accesslists.GetInheritedGrants(ctx, a, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ownGrants := a.GetGrants()
	combinedRoles := append(ownGrants.Roles, inherited.Roles...)
	slices.Sort(combinedRoles)
	combinedRoles = slices.Compact(combinedRoles)
	combinedTraits := mergeTraits(ownGrants.Traits, inherited.Traits)

	return &accesslist.Grants{
		Roles:  combinedRoles,
		Traits: combinedTraits,
	}, nil
}

func buildResourceIDSet(resources []types.ResourceID) map[string]struct{} {
	set := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		set[types.ResourceIDToString(r)] = struct{}{}
	}
	return set
}

func mergeTraits(t1, t2 map[string][]string) map[string][]string {
	out := make(map[string][]string)
	for _, traits := range []map[string][]string{t1, t2} {
		for key, values := range traits {
			out[key] = append(out[key], values...)
		}
	}
	for key, values := range out {
		slices.Sort(values)
		out[key] = slices.Compact(values)
	}
	return out
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
	if ownershipType != accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED {
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
	requestedRoles     []string
	requestKind        types.AccessRequestKind

	clock clockwork.Clock
}

// isListOwnerValidSuggestedReviewer returns true if the owner of the access list is a valid suggestion
// via either membership or promotion.
func (v *suggestionValidator) isListOwnerValidSuggestedReviewer(ctx context.Context, list *accesslist.AccessList) bool {
	validForCurrentMember, err := v.isValidForCurrentMember(ctx, list)
	if err != nil {
		slog.Log(ctx, logutils.TraceLevel, "Failed to validate access list membership", "error", err)
	}
	if validForCurrentMember {
		return true
	}

	validPromotionSuggestion, err := v.isValidPromotionSuggestion(ctx, list)
	if err != nil {
		slog.Log(ctx, logutils.TraceLevel, "Failed to validate access list promotion", "error", err)
		return false
	}

	return validPromotionSuggestion
}

// isValidForCurrentMember returns true when the requester is a member of the given access list, where member grants allow requesting
// and owner grants allow reviewing.
func (v *suggestionValidator) isValidForCurrentMember(ctx context.Context, list *accesslist.AccessList) (bool, error) {
	_, err := accesslists.IsAccessListMember(ctx, v.requester, list, v.dataGetter, nil, v.clock)
	if trace.IsAccessDenied(err) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}

	return v.memberGrantsAllowRequesting(ctx, list) && v.ownerGrantsAllowReviewing(ctx, list), nil
}

func (v *suggestionValidator) memberGrantsAllowRequesting(ctx context.Context, list *accesslist.AccessList) bool {
	if v.requestKind.IsLongTerm() {
		return slices.ContainsFunc(list.GetGrants().Roles, func(role string) bool {
			return slices.Contains(v.requestedRoles, role)
		})
	}

	// Deny conditions not checked. If a role denies requesting then the user wouldn't even be able to make the request.
	for _, roleName := range list.GetGrants().Roles {
		role, err := v.dataGetter.GetRole(ctx, roleName)
		if err != nil {
			slog.DebugContext(ctx, "Failed to get role from grants role name", "role_name", roleName, "error", err)
		} else if anyRoleMatches(ctx, role.GetAccessRequestConditions(types.Allow).Roles, v.requestedRoles) {
			return true
		}
	}

	return false
}

func (v *suggestionValidator) ownerGrantsAllowReviewing(ctx context.Context, list *accesslist.AccessList) bool {
	allowed := false

	for _, roleName := range list.GetOwnerGrants().Roles {
		role, err := v.dataGetter.GetRole(ctx, roleName)
		if err != nil {
			slog.DebugContext(ctx, "Failed to get role from owner grants role name", "role_name", roleName, "error", err)
			continue
		}
		if anyRoleMatches(ctx, role.GetAccessReviewConditions(types.Deny).Roles, v.requestedRoles) {
			return false
		}
		if anyRoleMatches(ctx, role.GetAccessReviewConditions(types.Allow).Roles, v.requestedRoles) {
			allowed = true
		}
	}

	return allowed
}

func (v *suggestionValidator) isValidPromotionSuggestion(ctx context.Context, list *accesslist.AccessList) (bool, error) {
	// Promotions are only available for resource-based access requests.
	if len(v.requestedResources) == 0 {
		return false, nil
	}
	// If the user is already a member, or doesn't meet the requirements to be assigned to the access list,
	// then the access list is not a valid suggestion.
	//
	// TODO(smallinsky) Switch to GetHierarchyForUser when it will be supported in v18
	members, err := accesslists.GetMembersFor(ctx, list.GetName(), v.dataGetter)
	if err != nil {
		return false, trace.Wrap(err, "getting accesslist members")
	}
	for _, member := range members {
		if member.GetName() == v.requester.GetName() {
			return false, nil
		}
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
	if len(accessRequest.GetAllRequestedResourceIDs()) == 0 {
		// Promotions are only available for resource-based access requests.
		// Return early and avoid constructing the validator if we already know it's invalid.
		return &types.AccessRequestAllowedPromotions{}, nil
	}

	validator, err := newSuggestionValidator(ctx, resourceGetter, accessRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedPromotions := &types.AccessRequestAllowedPromotions{}

	for accessList, err := range clientutils.Resources(ctx, resourceGetter.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		validPromotionSuggestion, err := validator.isValidPromotionSuggestion(ctx, accessList)
		if err != nil {
			slog.Log(ctx, logutils.TraceLevel, "Failed to validate access list promotion", "error", err)
			continue
		}

		if validPromotionSuggestion {
			allowedPromotions.Promotions = append(allowedPromotions.Promotions, &types.AccessRequestAllowedPromotion{
				AccessListName: accessList.GetName(),
			})
		}
	}

	return allowedPromotions, nil
}

// GenerateAccessRequestSuggestedReviewers returns a list of suggested reviewers for an access request.
// Suggested reviewers are owners of access lists determined via two paths:
//
// Promotion: the requester is not a member of the access list, but the list grants access to the requested resources and its owners have review permissions for the requested roles.
// Only applicable to resource-based requests.
//
// Membership: the requester is a current member of the access list, and:
//
// - For short-term requests: the list grants a role that allows requesting the requested role, and its owners have review permissions for those roles.
//
// - For long-term requests: the list directly grants one of the requested roles, and its owners have review permissions for those roles.
func GenerateAccessRequestSuggestedReviewers(ctx context.Context, resourceGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) ([]string, error) {
	validator, err := newSuggestionValidator(ctx, resourceGetter, accessRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var suggestedReviewers []string

	for accessList, err := range clientutils.Resources(ctx, resourceGetter.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if validator.isListOwnerValidSuggestedReviewer(ctx, accessList) {
			allOwners, err := resourceGetter.GetAccessListOwners(ctx, accessList.GetName())
			if err != nil {
				slog.WarnContext(ctx, "Failed to get nested access list owners, skipping access list", "error", err, "access_list", accessList.GetName())
				// Continue with remaining lists so we add all we can.
				continue
			}

			for _, owner := range allOwners {
				suggestedReviewers = append(suggestedReviewers, owner.Name)
			}
		}
	}

	return suggestedReviewers, nil
}

func newSuggestionValidator(ctx context.Context, resourceGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) (*suggestionValidator, error) {
	requester, err := resourceGetter.GetUser(ctx, accessRequest.GetUser(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We can reduce ResourceAccessIDs here to just ResourceIDs as Constraints are not
	// applicable for constraining membership in an Access List.
	allResourceIDs := types.RiskyExtractResourceIDs(accessRequest.GetAllRequestedResourceIDs())

	resources, err := accessrequest.GetResourcesByResourceIDs(ctx, resourceGetter, allResourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &suggestionValidator{
		dataGetter:         resourceGetter,
		requester:          requester,
		requestedResources: resources,
		requestedRoles:     accessRequest.GetRoles(),
		requestKind:        accessRequest.GetRequestKind(),
		clock:              clockwork.NewRealClock(),
	}, nil
}

func anyRoleMatches(ctx context.Context, patterns, targets []string) bool {
	for _, pattern := range patterns {
		m, err := parse.NewMatcher(pattern)
		if err != nil {
			slog.DebugContext(ctx, "Failed to parse matcher expression", "pattern", pattern, "error", err)
		} else if slices.ContainsFunc(targets, m.Match) {
			return true
		}
	}

	return false
}
