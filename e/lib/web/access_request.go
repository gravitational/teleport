package web

import (
	"context"
	"net/http"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
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
			ClusterName: resource.ClusterName,
			Name:        resource.Name,
			Kind:        resource.Kind,
		})
	}

	if len(resourceIDs) != 0 { // search based request
		req, err = services.NewAccessRequestWithResources(user, nil, resourceIDs)
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

	if err := clt.CreateAccessRequest(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return getAccessRequest(ctx, clt, req.GetMetadata().Name, opts...)
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

	resourceIDsByCluster := make(map[string][]types.ResourceID)
	for _, resourceID := range req.GetRequestedResourceIDs() {
		if resourceID.Kind != types.KindNode {
			// The only detail we want, for now, is the server hostname, so we
			// can skip all other resource kinds as a minor optimization.
			continue
		}
		resourceIDsByCluster[resourceID.ClusterName] = append(resourceIDsByCluster[resourceID.ClusterName], resourceID)
	}

	resourceDetails := make(map[string]ui.ResourceDetails)
	for clusterName, resourceIDs := range resourceIDsByCluster {
		clt, err := cfg.clusterClientProvider.UserClientForCluster(clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources, err := services.GetResourcesByResourceIDs(ctx, clt, resourceIDs)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, resource := range resources {
			hostname := ""
			if r, ok := resource.(interface{ GetHostname() string }); ok {
				hostname = r.GetHostname()
			} else {
				// The only detail we want, for now, is the server hostname.
				continue
			}

			id := types.ResourceID{
				ClusterName: clusterName,
				Kind:        resource.GetKind(),
				Name:        resource.GetName(),
			}
			key := types.ResourceIDToString(id)
			resourceDetails[key] = ui.ResourceDetails{
				Hostname: hostname,
			}
		}
	}

	return resourceDetails, nil
}

func (p *Plugin) getAccessRequestsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	filter := types.AccessRequestFilter{
		User: query.Get("user"),
	}

	return p.getAccessRequests(r.Context(), clt, filter)
}

func (p *Plugin) getAccessRequests(ctx context.Context, clt accessRequestAPIGetter, filter types.AccessRequestFilter) ([]ui.AccessRequest, error) {
	reqs, err := clt.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiReqs := make([]ui.AccessRequest, 0, len(reqs))
	for _, req := range reqs {
		uiReq, err := ui.NewAccessRequest(req)
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
	// CreateAccessRequest stores a new access request.
	CreateAccessRequest(ctx context.Context, req types.AccessRequest) error
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
}
