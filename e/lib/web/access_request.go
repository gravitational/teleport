package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/web"
)

type getAccessRequestConfig struct {
	clusterClientProvider web.ClusterClientProvider
}

func defaultGetAccessRequestConfig() *getAccessRequestConfig {
	return &getAccessRequestConfig{}
}

type getAccessRequestOption func(cfg *getAccessRequestConfig)

func withClusterClientProvider(clusterClientProvider web.ClusterClientProvider) getAccessRequestOption {
	return func(cfg *getAccessRequestConfig) {
		cfg.clusterClientProvider = clusterClientProvider
	}
}

func (p *Plugin) createAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *accessRequestParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return createAccessRequest(r.Context(), clt, *req, ctx.GetUser(), withClusterClientProvider(clusterClientProvider))
}

func createAccessRequest(ctx context.Context, clt accessRequestAPIGetter, request accessRequestParameters, user string, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var err error
	var req types.AccessRequest

	resourceIDs := make([]types.ResourceID, 0, len(request.ResourceIDs))
	for _, resource := range request.ResourceIDs {
		resourceIDs = append(resourceIDs, types.ResourceID{
			ClusterName:     resource.ClusterName,
			Name:            resource.Name,
			Kind:            resource.Kind,
			SubResourceName: resource.SubResourceName,
		})
	}

	if len(resourceIDs) != 0 { // search based request
		req, err = services.NewAccessRequestWithResources(user, request.Roles, resourceIDs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else { // role based request
		// If no specific roles were requested, then by default wild card is used which
		// the auth server automatically fills in with all roles the user is allowed to request.
		rolesRequested := []string{types.Wildcard}
		if len(request.Roles) != 0 {
			rolesRequested = request.Roles
		}

		req, err = services.NewAccessRequest(user, rolesRequested...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	req.SetRequestReason(request.Reason)
	req.SetSuggestedReviewers(request.SuggestedReviewers)
	req.SetMaxDuration(request.MaxDuration)
	req.SetDryRun(request.DryRun)

	// If the request is a dry run, then we need to use the V2 API to get the
	// response with the resource details. Otherwise, we can use the V1 API
	// for backwards compatibility.
	if req.GetDryRun() {
		resp, err := clt.CreateAccessRequestV2(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		usResp, err := ui.NewAccessRequest(resp)
		return usResp, trace.Wrap(err)
	}

	req, err = clt.CreateAccessRequestV2(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getAccessRequest(ctx, clt, req.GetMetadata().Name, opts...)
}

func (p *Plugin) getResourceRequestRolesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceIds := r.URL.Query().Get("resourceIds")
	var req []ui.ResourceID
	err = json.Unmarshal([]byte(resourceIds), &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getResourceRequestRoles(r.Context(), clt, req, ctx.GetUser())
}

// getResourceRequestRoles returns the list of necessary roles to access a list of resources
// given their resource IDs.
func getResourceRequestRoles(ctx context.Context, clt auth.ClientI, req []ui.ResourceID, user string) ([]string, error) {
	// Creates new list of type types.ResourceID from the request of type ui.ResourceID.
	// This is done because the json field name for `ClusterName` is different in both.
	var resourceIDs []types.ResourceID
	for _, resourceID := range req {
		resourceIDs = append(resourceIDs, types.ResourceID{
			Name:            resourceID.Name,
			Kind:            resourceID.Kind,
			ClusterName:     resourceID.ClusterName,
			SubResourceName: resourceID.SubResourceName,
		})
	}

	accessCaps, err := clt.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{User: user, ResourceIDs: resourceIDs})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessCaps.ApplicableRolesForResources, nil
}

func (p *Plugin) getAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")

	return getAccessRequest(r.Context(), clt, requestID, withClusterClientProvider(clusterClientProvider))
}

func getAccessRequest(ctx context.Context, clt accessRequestAPIGetter, requestID string, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if requestID == "" {
		return nil, trace.BadParameter("missing request id")
	}

	requestFilter := types.AccessRequestFilter{
		ID: requestID,
	}

	reqs, err := clt.GetAccessRequests(ctx, requestFilter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(reqs) < 1 {
		return nil, trace.NotFound("access request %q not found", requestID)
	}
	req := reqs[0]

	resourceDetails, err := getResourceDetails(ctx, req, cfg)
	if err != nil {
		// This error is unexpected, but we don't want to break the API filling
		// in optional details
		logrus.WithError(err).Info("Unexpected error in getAccessRequest while fetching resource details")
		return ui.NewAccessRequest(req)
	}

	return ui.NewAccessRequest(req, ui.WithResourceDetails(resourceDetails))
}

// getResourceDetails returns a map of resource details keyed by the string
// form of the resourceID created by types.ResourceIDToString
func getResourceDetails(ctx context.Context, req types.AccessRequest, cfg *getAccessRequestConfig) (map[string]ui.ResourceDetails, error) {
	if cfg.clusterClientProvider == nil {
		// We have no way to get resource details, but this is not an error.
		// Some APIs (the list endpoint) do not need details. A nil map is a
		// valid result which will return empty details (the default value) for
		// all keys.
		return nil, nil
	}

	resourceIDsByCluster := services.GetResourceIDsByCluster(req)

	resourceDetails := make(map[string]ui.ResourceDetails)
	for clusterName, resourceIDs := range resourceIDsByCluster {
		clt, err := cfg.clusterClientProvider.UserClientForCluster(ctx, clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		details, err := services.GetResourceDetails(ctx, clusterName, clt, resourceIDs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for id, d := range details {
			resourceDetails[id] = ui.ResourceDetails{
				FriendlyName: d.FriendlyName,
			}
		}
	}

	return resourceDetails, nil
}

func (p *Plugin) getAccessRequestsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	filter := types.AccessRequestFilter{
		User: query.Get("user"),
	}

	return p.getAccessRequests(r.Context(), clt, filter, withClusterClientProvider(clusterClientProvider))
}

func (p *Plugin) getAccessRequests(ctx context.Context, clt accessRequestAPIGetter, filter types.AccessRequestFilter, opts ...getAccessRequestOption) ([]ui.AccessRequest, error) {
	reqs, err := clt.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	uiReqs := make([]ui.AccessRequest, 0, len(reqs))
	for _, req := range reqs {
		var opts []ui.NewAccessRequestOption
		resourceDetails, err := getResourceDetails(ctx, req, cfg)
		if err != nil {
			// This error is unexpected, but we don't want to break the API filling
			// in optional details
			logrus.WithError(err).Info("Unexpected error in getAccessRequest while fetching resource details")
		} else {
			opts = append(opts, ui.WithResourceDetails(resourceDetails))
		}

		uiReq, err := ui.NewAccessRequest(req, opts...)
		if err != nil {
			p.Log.Warnf("Failed to process access request: %v", err)
			continue
		}

		uiReqs = append(uiReqs, *uiReq)
	}

	return uiReqs, nil
}

func (p *Plugin) reviewAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *accessRequestParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return reviewAccessRequest(r.Context(), clt, *req, withClusterClientProvider(clusterClientProvider))
}

func reviewAccessRequest(ctx context.Context, clt accessRequestAPIGetter, review accessRequestParameters, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var reviewState types.RequestState
	if err := reviewState.Parse(review.State); err != nil {
		return nil, trace.Wrap(err)
	}

	if !reviewState.IsApproved() && !reviewState.IsDenied() {
		return nil, trace.BadParameter("access review state %q, is not a valid state", review.State)
	}

	reviewSubmission := types.AccessReviewSubmission{
		RequestID: review.ID,
		Review: types.AccessReview{
			Roles:         review.Roles,
			ProposedState: reviewState,
			Reason:        review.Reason,
			Created:       time.Now(),
		},
	}

	updatedRequest, err := clt.SubmitAccessReview(ctx, reviewSubmission)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceDetails, err := getResourceDetails(ctx, updatedRequest, cfg)
	if err != nil {
		// This error is unexpected, but we don't want to break the API filling
		// in optional details
		logrus.WithError(err).Info("Unexpected error in reviewAccessRequest while fetching resource details")
		return ui.NewAccessRequest(updatedRequest)
	}

	return ui.NewAccessRequest(updatedRequest, ui.WithResourceDetails(resourceDetails))
}

func (p *Plugin) deleteAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")
	if requestID == "" {
		return nil, trace.BadParameter("missing request id")
	}

	if err := clt.DeleteAccessRequest(r.Context(), requestID); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

type accessRequestAPIGetter interface {
	// CreateAccessRequestV2 stores a new access request and returns the created request.
	CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error)
	// GetAccessRequests gets all currently active access requests.
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	// SubmitAccessReview applies a review to a request and returns the post-application state.
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
}

type accessRequestParameters struct {
	// Reason is the AccessRequest request reason.
	// Used interchangeably between reason why request is made and resolved reason.
	Reason string `json:"reason"`
	// State is the AccessRequest state.
	State string `json:"state"`
	// ID is the request ID.
	ID string `json:"id"`
	// Roles is the list of roles.
	// Used interchangeably between roles requested by user and overriding roles.
	Roles []string `json:"roles"`
	// SuggestedReviewers is a suggested list of reviewers to review a request.
	SuggestedReviewers []string `json:"suggestedReviewers"`
	// ResourceID is a unique identifier for a teleport resource.
	ResourceIDs []ui.ResourceID `json:"resourceIds"`
	// MaxDuration is the maximum duration for which the request is valid.
	MaxDuration time.Time `json:"maxDuration,omitempty"`
	// DryRun is a flag that indicates whether the request is a dry run to check and set defaults,
	// and return before actually creating the request in the backend.
	DryRun bool `json:"dryRun,omitempty"`
}

// accessListSuggestionClient defines interfaces needed for suggesting access lists.
type accessListSuggestionClient interface {
	AccessListClient() services.AccessLists
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	GetUser(string, bool) (types.User, error)
	services.RoleGetter
	accessRequestAPIGetter
}

func (p *Plugin) getSuggestedAccessListsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")
	maxSuggestionsStr := params.ByName("maxSuggestions")
	if maxSuggestionsStr == "" {
		maxSuggestionsStr = "50"
	}

	maxSuggestions, err := strconv.Atoi(maxSuggestionsStr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getSuggestedAccessLists(r.Context(), clt, ctx.GetUser(), requestID, maxSuggestions)
}

// getSuggestedAccessLists returns a list of access lists that are suggested for a given request.
func getSuggestedAccessLists(ctx context.Context, clt accessListSuggestionClient, reviewerName string, requestID string, maxSuggestions int, opts ...getAccessRequestOption) (*ui.SuggestedAccessLists, error) {
	accessRequest, err := getAccessRequest(ctx, clt, requestID, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userName := accessRequest.User
	targetUser, err := clt.GetUser(userName, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists, err := clt.AccessListClient().GetAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Filter out access lists that the reviewer cannot modify using an in-place truncate.
	reviewer, err := clt.GetUser(reviewerName, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cursor := 0
	for _, accessList := range accessLists {
		err := shouldSuggestAccessList(ctx, clt, reviewer, accessList)
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

	ranked, err := scoreRelevance(ctx, clt, targetUser, accessRequest, accessLists[:cursor])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.SuggestedAccessLists{AccessLists: ranked[:min(maxSuggestions, len(ranked))]}, nil
}

func shouldSuggestAccessList(ctx context.Context, clt accessListSuggestionClient, reviewer types.User, accessList *accesslist.AccessList) error {
	// if owner, then can list and modify
	for _, owner := range accessList.GetOwners() {
		if owner.Name == reviewer.GetName() {
			return nil
		}
	}

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

func scoreRelevance(ctx context.Context, clt accessListSuggestionClient, targetUser types.User, request *ui.AccessRequest, lists []*accesslist.AccessList) ([]*accesslist.AccessList, error) {
	scores := make([]scoredAccessList, 0, len(lists))
	resources := make([]types.ResourceWithLabels, len(request.Resources))

	for i, uiResource := range request.Resources {
		resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType:        uiResource.ID.Kind,
			Namespace:           apidefaults.Namespace,
			Limit:               1,
			UseSearchAsRoles:    true,
			PredicateExpression: "name == " + uiResource.ID.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(resp.Resources) == 0 {
			return nil, trace.NotFound("resource %q not found", uiResource.ID)
		}

		resources[i] = resp.Resources[0]
	}

	for _, list := range lists {
		score, err := computeAccessListRelevancy(ctx, clt, targetUser, request.Roles, resources, list)
		if _, ok := err.(accessListIrrelevantError); ok {
			continue
		} else if err != nil {
			return nil, trace.Wrap(err)
		}

		scores = append(scores, scoredAccessList{
			list:  list,
			score: score,
		})
	}

	slices.SortFunc(scores, func(a, b scoredAccessList) int {
		switch {
		case a.score < b.score:
			return -1
		case a.score > b.score:
			return 1
		default:
			return 0
		}
	})

	for i, scoredList := range scores {
		lists[i] = scoredList.list
	}

	return lists[:len(scores)], nil
}

func computeAccessListRelevancy(ctx context.Context, clt accessListSuggestionClient, targetUser types.User, requestRoles []string, requestResources []types.ResourceWithLabels, list *accesslist.AccessList) (int, error) {
	const (
		roleNegativeWeight = -4
	)

	score := 0
	grantedRolesNames := list.GetGrants().Roles
	requirements := list.GetMembershipRequires()

	// Access list not assignable to the user are irrelevant.
	if !services.UserMeetsRequirements(tlsca.Identity{
		Groups: targetUser.GetRoles(),
		Traits: targetUser.GetTraits(),
	}, requirements) {
		return 0, accessListIrrelevantError{}
	}

	// Penalize access lists that provide access to roles that were not requested.
	for _, grantedRole := range grantedRolesNames {
		if !slices.Contains(requestRoles, grantedRole) {
			score += roleNegativeWeight
		}
	}

	// Access lists that don't provide access to the requested resources are irrelevant
	accessChecker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles: grantedRolesNames,
	}, "", clt)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	for _, resource := range requestResources {
		err := accessChecker.CheckAccess(resource, services.AccessState{MFAVerified: true})
		switch {
		case trace.IsAccessDenied(err):
			return 0, accessListIrrelevantError{}
		default:
			return 0, trace.Wrap(err)
		}
	}

	return 0, nil
}

type accessListIrrelevantError struct{}

func (accessListIrrelevantError) Error() string {
	return "access list is irrelevant"
}
