/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accessrequest

import (
	"context"
	"maps"
	"slices"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type roleGetter interface {
	GetRole(ctx context.Context, name string) (types.Role, error)
}

type AccessListSuggestionClient interface {
	GetUser(userName string, withSecrets bool) (types.User, error)
	roleGetter

	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

type AccessListGetter interface {
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
}

type AccessListLister interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
}

// GetSuggestedAccessLists returns a list of access lists that are suggested for a given request.
func GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt AccessListSuggestionClient,
	accessListGetter AccessListGetter, requestID string,
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
	reviewer, err := clt.GetUser(identity.Username, false)
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
		err := canModifyAccessList(clt, identity, reviewer, accessList)
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
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ranked, nil
}

func canModifyAccessList(clt roleGetter, reviewerIdentity *tlsca.Identity, reviewer types.User, accessList *accesslist.AccessList) error {
	// if owner, then can list and modify
	err := services.IsAccessListOwner(*reviewerIdentity, accessList)
	switch {
	case err == nil:
		return nil
	case !trace.IsAccessDenied(err):
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
	authErrCreate := accessChecker.CheckAccessToRule(&services.Context{User: reviewer, Resource: accessList}, apidefaults.Namespace, types.KindAccessList, types.VerbCreate, true)
	authErrUpdate := accessChecker.CheckAccessToRule(&services.Context{User: reviewer, Resource: accessList}, apidefaults.Namespace, types.KindAccessList, types.VerbUpdate, true)
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

func isValidSuggestion(ctx context.Context, clt roleGetter, requester types.User,
	requestedResources []types.ResourceWithLabels, list *accesslist.AccessList,
) (bool, error) {
	requirements := list.GetMembershipRequires()

	// Access lists not assignable to the user are irrelevant.
	if !services.UserMeetsRequirements(tlsca.Identity{
		Groups: requester.GetRoles(),
		Traits: requester.GetTraits(),
	}, requirements) {
		return false, nil
	}

	// TODO(jakule/mdwn): This can be unified with userloginstate.Generator.addAccessListsToState().
	// Clone the requester's roles and traits and add the access list's roles and traits.
	// We need them to check if the additional roles and traits provide access to the requested resources.
	allRoles := append(slices.Clone(requester.GetRoles()), list.GetGrants().Roles...)
	allTraits := map[string][]string{}
	maps.Copy(allTraits, requester.GetTraits())
	maps.Copy(allTraits, list.GetGrants().Traits)

	// Access lists that don't provide access to the requested resources are irrelevant
	accessChecker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles:  allRoles,
		Traits: allTraits,
	}, "", clt)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, resource := range requestedResources {
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

// GenerateAccessRequestPromotions returns a list of access lists that are suggested for a given access request.
func GenerateAccessRequestPromotions(ctx context.Context, resourceGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	if len(accessRequest.GetRequestedResourceIDs()) == 0 {
		// Suggestions are only available for resource-based access requests.
		return types.NewAccessRequestAllowedPromotions(nil), nil
	}

	requester, err := resourceGetter.GetUser(accessRequest.GetUser(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, err := services.GetResourcesByResourceIDs(ctx, resourceGetter, accessRequest.GetRequestedResourceIDs())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedPromotions := types.NewAccessRequestAllowedPromotions(nil)

	if err := forEachAccessList(ctx, resourceGetter, func(accessList *accesslist.AccessList) error {
		valid, err := isValidSuggestion(ctx, resourceGetter, requester, resources, accessList)
		if err != nil {
			log.Tracef("failed to validate access list suggestion: %v", err)
			return nil
		}

		if !valid {
			return nil
		}

		allowedPromotions.Promotions = append(allowedPromotions.Promotions, &types.AccessRequestAllowedPromotion{
			AccessListName: accessList.GetName(),
		})

		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return allowedPromotions, nil
}

func forEachAccessList(ctx context.Context, accessListGetter AccessListLister, process func(accessList *accesslist.AccessList) error) error {
	var nextToken string

	for {
		accessLists, token, err := accessListGetter.ListAccessLists(ctx, 0 /* default value */, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, accessList := range accessLists {
			if err := process(accessList); err != nil {
				return trace.Wrap(err)
			}
		}

		if token == "" {
			break
		}

		nextToken = token
	}

	return nil
}
