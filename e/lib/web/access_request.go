package web

import (
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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
	if len(req.ResourceAccessIDs) > 0 && !componentfeatures.InAllSets(componentfeatures.FeatureResourceConstraintsV1, clusterAuthProxyServerFeatures) {
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
				ClusterName: rid.ClusterName,
				Name:        rid.Name,
				Kind:        rid.Kind,
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

	return getResourceRequestRoles(r.Context(), clt, req, ctx.GetUser(), ctx, clusterClientProvider)
}

// repackUIResourceIDs repacks the supplied list of [ui.ResourceID] values into a
// list of [types.ResourceID]. This is necessary due to wire format differences
// in the two resource ID types.
func repackUIResourceIDs(req []ui.ResourceID) []types.ResourceID {
	resourceIDs := make([]types.ResourceID, 0, len(req))
	for _, resourceID := range req {
		resourceIDs = append(resourceIDs, types.ResourceID{
			Name:            resourceID.Name,
			Kind:            resourceID.Kind,
			ClusterName:     resourceID.ClusterName,
			SubResourceName: resourceID.SubResourceName,
		})
	}
	return resourceIDs
}

// getResourceRequestRoles returns the list of necessary roles to access a list of resources
// given their resource IDs.
func getResourceRequestRoles(ctx context.Context, clt authclient.ClientI, req []ui.ResourceID, user string, sessionContext *web.SessionContext, clusterClientProvider web.ClusterClientProvider) ([]string, error) {
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
	for _, rID := range req {
		if cluster != "" && cluster != rID.ClusterName {
			return nil, trace.BadParameter("all requested resources must be from the same cluster")
		}
		cluster = rID.ClusterName
	}
	targetCluster := cmp.Or(cluster, localClusterName.GetClusterName())

	resourceIDs := repackUIResourceIDs(req)

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
			User:          user,
			ResourceIDs:   resourceIDs,
			SearchAsRoles: searchAsRoles,
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

	accessCaps, err := clt.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{User: user, ResourceIDs: resourceIDs})
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

	// TODO(kiosion): Handle generating long-term resource groupings similar to getResourceDetails.
	resourceDetails, err := getResourceDetails(ctx, req, cfg)
	if err != nil {
		// This error is unexpected, but we don't want to break the API filling
		// in optional details
		slog.InfoContext(ctx, "Unexpected error in getAccessRequest while fetching resource details", "error", err)
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
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error)
	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
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

	uiReqs := make([]ui.AccessRequest, 0, len(resp.AccessRequests))
	for _, req := range resp.AccessRequests {
		uiReq, err := ui.NewAccessRequest(req, ui.WithResourceDetails(details))
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

	resourceDetails, err := getResourceDetails(ctx, updatedRequest, cfg)
	if err != nil {
		// This error is unexpected, but we don't want to break the API filling
		// in optional details
		slog.InfoContext(ctx, "Unexpected error in reviewAccessRequest while fetching resource details", "error", err)
		return ui.NewAccessRequest(updatedRequest)
	}

	return ui.NewAccessRequest(updatedRequest, ui.WithResourceDetails(resourceDetails))
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
		&accesslistv1.AccessRequestPromoteRequest{
			RequestId:      requestID,
			AccessListName: req.AccessListName,
			Reason:         req.Reason,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ar, err := ui.NewAccessRequest(resp.AccessRequest)
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
