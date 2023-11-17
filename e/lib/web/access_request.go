package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	apiaccessrequest "github.com/gravitational/teleport/api/accessrequest"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
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

type accessRequestGetCreator interface {
	accessRequestGetter
	CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error)
}

func createAccessRequest(ctx context.Context, clt accessRequestGetCreator, request accessRequestParameters, user string, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
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
	req.SetExpiry(request.RequestTTL)
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
func getBulkResourceDetails(ctx context.Context, reqs []types.AccessRequest, cfg *getAccessRequestConfig) (map[string]ui.ResourceDetails, error) {
	if cfg.clusterClientProvider == nil {
		// We have no way to get resource details, but this is not an error.
		// Some APIs may not need details. A nil map is a valid result
		// which will return empty details (the default value) for all keys.
		return nil, nil
	}

	// allIDs aggregates all resource IDs (this step is mostly only useful for deduplication).
	allIDs := make(map[string]types.ResourceID)
	for _, req := range reqs {
		for _, id := range req.GetRequestedResourceIDs() {
			allIDs[types.ResourceIDToString(id)] = id
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
		return nil, trace.Wrap(err)
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

type accessRequestGetter interface {
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

func (p *Plugin) getAccessRequests(ctx context.Context, clt accessRequestGetter, filter types.AccessRequestFilter, opts ...getAccessRequestOption) ([]ui.AccessRequest, error) {
	reqs, err := clt.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := defaultGetAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	details, err := getBulkResourceDetails(ctx, reqs, cfg)
	if err != nil {
		logrus.WithError(err).Warn("Failed to load resource details for access requests.")
	}

	uiReqs := make([]ui.AccessRequest, 0, len(reqs))
	for _, req := range reqs {
		uiReq, err := ui.NewAccessRequest(req, ui.WithResourceDetails(details))
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

type accessReviewSubmitter interface {
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
}

func reviewAccessRequest(ctx context.Context, clt accessReviewSubmitter, review accessRequestParameters, opts ...getAccessRequestOption) (*ui.AccessRequest, error) {
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
	// RequestTTL is the expiration time of the request (how long it will await
	// approval).
	RequestTTL time.Time `json:"requestTTL,omitempty"`
	// DryRun is a flag that indicates whether the request is a dry run to check and set defaults,
	// and return before actually creating the request in the backend.
	DryRun bool `json:"dryRun,omitempty"`
	// PromotedAccessListTitle is the title of the access list that this request
	// was promoted to. Used by WebUI to display the title of the access list.
	// This field is only populated when the request is in the PROMOTED state.
	PromotedAccessListTitle string `json:"promotedAccessListTitle,omitempty"`
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
