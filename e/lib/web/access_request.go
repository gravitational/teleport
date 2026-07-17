package web

import (
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"

	apiaccessrequest "github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client/proto"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/componentfeatures"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
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

func (p *Plugin) createAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *ui.AccessRequestParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterAuthProxyServerFeatures := componentfeatures.GetClusterAuthProxyServerFeatures(r.Context(), clt, p.Logger)
	hasConstraints := slices.ContainsFunc(req.ResourceAccessIDs, func(r ui.ResourceAccessID) bool {
		return r.Constraints != nil
	})
	if hasConstraints && !componentfeatures.InAllSets(componentfeatures.FeatureResourceConstraintsV1, clusterAuthProxyServerFeatures) {
		return nil, trace.BadParameter("constrained resources were specified in Access Request, but the cluster does not support Resource Constraints")
	}

	return createAccessRequest(r.Context(), clt, *req, ctx.GetUser(), withClusterClientProvider(clusterClientProvider))
}

type accessRequestGetCreator interface {
	accessRequestGetter
	CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error)
}

func createAccessRequest(ctx context.Context, clt accessRequestGetCreator, request ui.AccessRequestParameters, user string, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var err error
	var req types.AccessRequest

	resourceAccessIDs := make([]types.ResourceAccessID, 0, len(request.ResourceIDs)+len(request.ResourceAccessIDs))
	for _, rid := range request.ResourceIDs {
		resourceAccessIDs = append(resourceAccessIDs, types.ResourceAccessID{
			Id: types.ResourceID{
				ClusterName:     rid.ClusterName,
				Name:            rid.Name,
				Kind:            rid.Kind,
				SubResourceName: rid.SubResourceName,
			},
		})
	}
	for _, raid := range request.ResourceAccessIDs {
		rid, constraints := raid.ID, raid.Constraints
		resourceAccessIDs = append(resourceAccessIDs, types.ResourceAccessID{
			Id: types.ResourceID{
				ClusterName:     rid.ClusterName,
				Name:            rid.Name,
				Kind:            rid.Kind,
				SubResourceName: rid.SubResourceName,
			},
			Constraints: constraints,
		})
	}

	if len(resourceAccessIDs) != 0 { // search based request
		req, err = services.NewAccessRequestWithResources(user, request.Roles, resourceAccessIDs)
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
	req.SetDryRun(request.DryRun)
	req.SetRequestKind(request.RequestKind)
	req.SetExpiry(request.RequestTTL)
	req.SetMaxDuration(request.MaxDuration)

	if request.AssumeStartTime != nil {
		req.SetAssumeStartTime(*request.AssumeStartTime)
	}

	resp, err := clt.CreateAccessRequestV2(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetDryRun() {
		return ui.NewAccessRequest(resp)
	}

	uiResp, err := getAccessRequest(ctx, clt, resp.GetMetadata().Name, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(kiosion): Remove and rely on getAccessRequest to handle adding this info.
	if request.RequestKind.IsLongTerm() {
		if r, err := ui.NewAccessRequest(resp); err == nil && r.LongTermResourceGrouping != nil {
			uiResp.LongTermResourceGrouping = r.LongTermResourceGrouping
		}
	}

	return uiResp, nil
}

// getResourceRequestRolesHandle handles GET requests for resource request roles.
//
// Deprecated: Use getResourceRequestRolesV2Handle which supports ResourceAccessIDs with constraints.
func (p *Plugin) getResourceRequestRolesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
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
	resourceAccessIDs := sliceutils.Map(req, func(id ui.ResourceID) ui.ResourceAccessID { return ui.ResourceAccessID{ID: id} })

	return getResourceRequestRoles(r.Context(), clt, resourceAccessIDs, ctx.GetUser(), ctx, clusterClientProvider)
}

// resourceRequestRolesRequest is the request body for the POST /enterprise/resourcerequestroles endpoint.
type resourceRequestRolesRequest struct {
	// ResourceAccessIDs is the list of resources (with optional constraints) to find applicable roles for.
	ResourceAccessIDs []ui.ResourceAccessID `json:"resourceAccessIds"`
}

// getResourceRequestRolesV2Handle handles POST requests for resource request roles,
// accepting ResourceAccessIDs with optional constraints.
func (p *Plugin) getResourceRequestRolesV2Handle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req resourceRequestRolesRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return getResourceRequestRoles(r.Context(), clt, req.ResourceAccessIDs, ctx.GetUser(), ctx, clusterClientProvider)
}

// repackUIResourceAccessIDs repacks a given list of [ui.ResourceAccessID] values into a
// list of [types.ResourceAccessID].
func repackUIResourceAccessIDs(req []ui.ResourceAccessID) []types.ResourceAccessID {
	return sliceutils.Map(req, func(r ui.ResourceAccessID) types.ResourceAccessID {
		return types.ResourceAccessID{
			Id: types.ResourceID{
				Name:            r.ID.Name,
				Kind:            r.ID.Kind,
				SubResourceName: r.ID.SubResourceName,
				ClusterName:     r.ID.ClusterName,
			},
			Constraints: r.Constraints,
		}
	})
}

// getResourceRequestRoles returns the list of necessary roles to access a list of resources
// given their resource IDs.
func getResourceRequestRoles(ctx context.Context, clt authclient.ClientI, req []ui.ResourceAccessID, user string, sessionContext *web.SessionContext, clusterClientProvider web.ClusterClientProvider) ([]string, error) {
	if len(req) == 0 {
		return []string{}, nil
	}

	localClusterName, err := clt.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We assume that an Access Request may only ever request resources from a
	// single cluster. If this is ever becomes an invalid assumption, it should
	// be straightforward to group the requests by cluster and the map/reduce
	// the per-cluster resources
	var cluster string
	for _, raid := range req {
		if cluster != "" && cluster != raid.ID.ClusterName {
			return nil, trace.BadParameter("all requested resources must be from the same cluster")
		}
		cluster = raid.ID.ClusterName
	}
	targetCluster := cmp.Or(cluster, localClusterName.GetClusterName())

	resourceAccessIDs := repackUIResourceAccessIDs(req)
	// For backwards-compat with an old Auth that only reads ResourceIDs,
	// also send all resources as ResourceIDs. Updated Auth deduplicates
	// and prefers the ResourceAccessID version if present.
	unwrappedResourceIDs := types.RiskyExtractResourceIDs(resourceAccessIDs)

	if targetCluster != localClusterName.GetClusterName() {
		slog.DebugContext(ctx, "Delegating role selection to remote cluster", "remote_cluster", targetCluster)

		accessChecker, err := sessionContext.GetUserAccessChecker()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		searchAsRoles := accessChecker.GetAllowedSearchAsRoles()

		clusterClient, err := clusterClientProvider.UserClientForCluster(ctx, targetCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		accessCaps, err := clusterClient.GetRemoteAccessCapabilities(ctx, types.RemoteAccessCapabilitiesRequest{
			User:              user,
			SearchAsRoles:     searchAsRoles,
			ResourceIDs:       unwrappedResourceIDs,
			ResourceAccessIds: resourceAccessIDs,
		})
		switch {
		case err == nil:
			return accessCaps.ApplicableRolesForResources, nil
		case trace.IsNotImplemented(err):
			// Remote cluster does not support pruning roles. Fall back to the
			// local pruning algorithm
		default:
			return nil, trace.Wrap(err)
		}
	}

	accessCaps, err := clt.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{
		User:              user,
		ResourceIDs:       unwrappedResourceIDs,
		ResourceAccessIds: resourceAccessIDs,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessCaps.ApplicableRolesForResources, nil
}

func (p *Plugin) getAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")

	return getAccessRequest(r.Context(), clt, requestID, withClusterClientProvider(clusterClientProvider))
}

func getAccessRequest(ctx context.Context, clt accessRequestGetter, requestID string, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if requestID == "" {
		return nil, trace.BadParameter("missing request id")
	}

	resp, err := clt.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
		Filter: &types.AccessRequestFilter{
			ID: requestID,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(resp.AccessRequests) < 1 {
		return nil, trace.NotFound("access request %q not found", requestID)
	}
	req := resp.AccessRequests[0]
	userDisplays := userDisplaysFromProto(resp.UserDisplays)

	// TODO(kiosion): Handle generating long-term resource groupings similar to getResourceDetails.
	resourceDetails, err := getResourceDetails(ctx, req, cfg)
	if err != nil {
		// This error is unexpected, but we don't want to break the API filling
		// in optional details
		slog.InfoContext(ctx, "Unexpected error in getAccessRequest while fetching resource details", "error", err)
		return ui.NewAccessRequest(req, ui.WithUserDisplays(userDisplays))
	}

	return ui.NewAccessRequest(req, ui.WithResourceDetails(resourceDetails), ui.WithUserDisplays(userDisplays))
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

	resourceIDsByCluster := apiaccessrequest.GetResourceIDsByCluster(req)

	resourceDetails := make(map[string]ui.ResourceDetails)
	for clusterName, resourceIDs := range resourceIDsByCluster {
		clt, err := cfg.clusterClientProvider.UserClientForCluster(ctx, clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		details, err := apiaccessrequest.GetResourceDetails(ctx, clusterName, clt, resourceIDs)
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

// getBulkResourceDetails is equivalent to getResourceDetails except that it batch-processes sets of
// access requests.
func getBulkResourceDetails(ctx context.Context, reqs []*types.AccessRequestV3, cfg *getAccessRequestConfig) (map[string]ui.ResourceDetails, error) {
	if cfg.clusterClientProvider == nil {
		// We have no way to get resource details, but this is not an error.
		// Some APIs may not need details. A nil map is a valid result
		// which will return empty details (the default value) for all keys.
		return map[string]ui.ResourceDetails{}, nil
	}

	// allIDs aggregates all resource IDs (this step is mostly only useful for deduplication).
	allIDs := make(map[string]types.ResourceID)
	for _, req := range reqs {
		for _, w := range req.GetAllRequestedResourceIDs() {
			rid := w.GetResourceID()
			str := types.ResourceIDToString(rid)
			allIDs[str] = rid
		}
	}

	// sortedIDs aggregates ids sorted into a nested mapping of the form cluster -> type -> ids.
	sortedIDs := make(map[string]map[string][]types.ResourceID)
	for _, id := range allIDs {
		cluster := sortedIDs[id.ClusterName]
		if cluster == nil {
			cluster = make(map[string][]types.ResourceID)
			sortedIDs[id.ClusterName] = cluster
		}

		cluster[id.Kind] = append(cluster[id.Kind], id)
	}

	var mu sync.Mutex
	var eg errgroup.Group
	allDetails := make(map[string]ui.ResourceDetails, len(allIDs))

	for clusterName, idsByKind := range sortedIDs {
		for _, allIDsOfKind := range idsByKind {
			// split request IDs into chunks of 256 for cuncurrent resolution (larger chunk sizes than this
			// result in degraded performance, likely due to the complexity of the resulting predicate expression
			// becoming more harmful than the batching is beneficial).
			for _, resourceIDs := range splitChunks(allIDsOfKind, 256) {

				clusterName, resourceIDs := clusterName, resourceIDs

				eg.Go(func() error {
					clt, err := cfg.clusterClientProvider.UserClientForCluster(ctx, clusterName)
					if err != nil {
						return trace.Wrap(err)
					}

					details, err := apiaccessrequest.GetResourceDetails(ctx, clusterName, clt, resourceIDs)
					if err != nil {
						return trace.Wrap(err)
					}

					mu.Lock()
					defer mu.Unlock()
					for id, d := range details {
						allDetails[id] = ui.ResourceDetails{
							FriendlyName: d.FriendlyName,
						}
					}

					return nil
				})
			}
		}
	}

	if err := eg.Wait(); err != nil {
		return map[string]ui.ResourceDetails{}, trace.Wrap(err)
	}

	return allDetails, nil
}

// splitChunks is a helper for chunking a slice s into sub-slices of size n. If the length of s
// is not evenly divisble by n then the last slice will be shorter than the rest.
func splitChunks[T any](s []T, n int) [][]T {
	c := make([][]T, 0, (len(s)/n)+1)
	for i := 0; i < len(s); i += n {
		end := min(len(s), i+n)
		c = append(c, s[i:end])
	}
	return c
}

func getSortField(sortByString string) proto.AccessRequestSort {
	switch sortByString {
	case "created":
		return proto.AccessRequestSort_CREATED
	case "user":
		return proto.AccessRequestSort_USER
	case "state":
		return proto.AccessRequestSort_STATE
	default:
		return proto.AccessRequestSort_CREATED
	}
}

func getAccessRequestScope(scope string) types.AccessRequestScope {
	switch scope {
	case "my_requests":
		return types.AccessRequestScope_MY_REQUESTS
	case "needs_review":
		return types.AccessRequestScope_NEEDS_REVIEW
	case "reviewed":
		return types.AccessRequestScope_REVIEWED
	default:
		return types.AccessRequestScope_DEFAULT
	}
}

func (p *Plugin) getAccessRequestsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	filter := &types.AccessRequestFilter{
		User:           query.Get("user"),
		SearchKeywords: client.ParseSearchKeywords(query.Get("search"), ' '),
		Scope:          getAccessRequestScope(query.Get("scope")),
	}

	sortBy := types.GetSortByFromString(query.Get("sort"))

	req := &proto.ListAccessRequestsRequest{
		Filter:     filter,
		StartKey:   query.Get("startKey"),
		Sort:       getSortField(sortBy.Field),
		Descending: sortBy.IsDesc,
	}

	limit, err := web.QueryLimitAsInt32(query, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Limit = limit

	// if limit exists as a query parameter, this means its coming from a "new" webui
	// and can return the new paginated response
	// TODO (avatus) make prefixed versions instead of checking query params
	resp, err := p.getAccessRequests(r.Context(), clt, req, withClusterClientProvider(clusterClientProvider))
	if query.Get("limit") != "" {
		return resp, trace.Wrap(err)
	}
	// old api request must return requests only
	return resp.AccessRequests, trace.Wrap(err)
}

type accessRequestGetter interface {
	ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error)
}

type AccessRequestsPage struct {
	AccessRequests []ui.AccessRequest `json:"requests"`
	StartKey       string             `json:"startKey"`
}

func (p *Plugin) getAccessRequests(ctx context.Context, clt accessRequestGetter, req *proto.ListAccessRequestsRequest, opts ...getAccessRequestOption) (AccessRequestsPage, error) {
	resp, err := clt.ListAccessRequests(ctx, req)
	if err != nil {
		return AccessRequestsPage{}, trace.Wrap(err)
	}

	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	details, err := getBulkResourceDetails(ctx, resp.AccessRequests, cfg)
	if err != nil {
		slog.WarnContext(ctx, "Failed to load resource details for access requests", "error", err)
	}

	userDisplays := userDisplaysFromProto(resp.UserDisplays)
	uiReqs := make([]ui.AccessRequest, 0, len(resp.AccessRequests))
	for _, req := range resp.AccessRequests {
		uiReq, err := ui.NewAccessRequest(req, ui.WithResourceDetails(details), ui.WithUserDisplays(userDisplays))
		if err != nil {
			p.Logger.WarnContext(ctx, "Failed to process access request", "error", err)
			continue
		}
		uiReqs = append(uiReqs, *uiReq)
	}

	return AccessRequestsPage{
		AccessRequests: uiReqs,
		StartKey:       resp.NextKey,
	}, nil
}

func userDisplaysFromProto(displays map[string]*proto.UserDisplay) map[string]types.UserDisplay {
	if len(displays) == 0 {
		return nil
	}

	out := make(map[string]types.UserDisplay, len(displays))
	for username, display := range displays {
		if display == nil {
			out[username] = types.UserDisplay{}
			continue
		}
		out[username] = types.UserDisplay{
			Primary:   display.Primary,
			Secondary: display.Secondary,
		}
	}
	return out
}

func (p *Plugin) reviewAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *ui.AccessRequestParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return reviewAccessRequest(r.Context(), clt, *req, withClusterClientProvider(clusterClientProvider))
}

type accessReviewSubmitter interface {
	accessRequestGetter
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
}

func reviewAccessRequest(ctx context.Context, clt accessReviewSubmitter, review ui.AccessRequestParameters, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var reviewState types.RequestState
	if err := reviewState.Parse(review.State); err != nil {
		return nil, trace.Wrap(err)
	}

	if !reviewState.IsResolved() {
		return nil, trace.BadParameter("access review state %q, is not a valid state. The request has already been processed.", review.State)
	}

	var assumeStartTime *time.Time
	if review.AssumeStartTime != nil {
		assumeStartTime = review.AssumeStartTime
	}

	reviewSubmission := types.AccessReviewSubmission{
		RequestID: review.ID,
		Review: types.AccessReview{
			Roles:           review.Roles,
			ProposedState:   reviewState,
			Reason:          review.Reason,
			Created:         time.Now(),
			AssumeStartTime: assumeStartTime,
		},
	}

	updatedRequest, err := clt.SubmitAccessReview(ctx, reviewSubmission)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Refetch the reviewed request. getAccessRequest returns the complete
	// response, with both server-resolved user displays and resource details.
	// The review is already committed at this point, so a fetch failure must
	// not surface as an error, fall back to the submit response, which lacks
	// user displays.
	uiResp, err := getAccessRequest(ctx, clt, review.ID, opts...)
	if err != nil {
		slog.WarnContext(ctx, "Failed to fetch reviewed access request, returning the review response without user displays", "error", err)

		resourceDetails, err := getResourceDetails(ctx, updatedRequest, cfg)
		if err != nil {
			// This error is unexpected, but we don't want to break the API filling
			// in optional details
			slog.InfoContext(ctx, "Unexpected error in reviewAccessRequest while fetching resource details", "error", err)
			return ui.NewAccessRequest(updatedRequest)
		}
		return ui.NewAccessRequest(updatedRequest, ui.WithResourceDetails(resourceDetails))
	}

	return uiResp, nil
}

func (p *Plugin) deleteAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
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

type accessRequestPromoteParameters struct {
	// AccessListName is the name of the access list to promote the request to.
	AccessListName string `json:"accessListName"`
	// Reason is the reason for promoting the request.
	Reason string `json:"reason"`
}

func (a accessRequestPromoteParameters) CheckAndSetDefaults() error {
	if a.AccessListName == "" {
		return trace.BadParameter("missing access list name")
	}

	return nil
}

type accessRequestPromoteResponse struct {
	// AccessRequest is the access request that was promoted.
	AccessRequest *ui.AccessRequest `json:"accessRequest"`
}

func (p *Plugin) accessRequestPromoteHandle(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sessCtx *web.SessionContext, _ web.ClusterClientProvider) (any, error) {
	clt, err := sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")

	var req accessRequestPromoteParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.AccessListClient().AccessRequestPromote(r.Context(),
		accesslistv1.AccessRequestPromoteRequest_builder{
			RequestId:      requestID,
			AccessListName: req.AccessListName,
			Reason:         req.Reason,
		}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ar, err := ui.NewAccessRequest(resp.GetAccessRequest())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &accessRequestPromoteResponse{
		AccessRequest: ar,
	}, nil
}

func (p *Plugin) getSuggestedAccessListsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, clusterClientProvider web.ClusterClientProvider) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const defaultMaxSuggestions = 50

	requestID := params.ByName("requestId")
	maxSuggestionsStr := params.ByName("maxSuggestions")

	maxSuggestions := defaultMaxSuggestions
	if maxSuggestionsStr != "" {
		maxSuggestions, err = strconv.Atoi(maxSuggestionsStr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	identity, err := ctx.GetIdentity()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	suggestions, err := accessrequest.GetSuggestedAccessLists(r.Context(), identity, clt, clt.AccessListClient(), requestID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.SuggestedAccessLists{AccessLists: suggestions[:min(len(suggestions), maxSuggestions)]}, nil
}
