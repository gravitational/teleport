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
)

func (p *Plugin) createAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *accessRequestParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return createAccessRequest(r.Context(), clt, *req, ctx.GetUser())
}

func createAccessRequest(ctx context.Context, clt accessRequestAPIGetter, request accessRequestParameters, user string) (*ui.AccessRequest, error) {
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

	return getAccessRequest(ctx, clt, req.GetMetadata().Name)
}

func (p *Plugin) getAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")

	return getAccessRequest(r.Context(), clt, requestID)
}

func getAccessRequest(ctx context.Context, clt accessRequestAPIGetter, requestID string) (*ui.AccessRequest, error) {
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

	return ui.NewAccessRequest(reqs[0])
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

func (p *Plugin) reviewAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *accessRequestParameters
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return reviewAccessRequest(r.Context(), clt, *req)
}

func reviewAccessRequest(ctx context.Context, clt accessRequestAPIGetter, review accessRequestParameters) (*ui.AccessRequest, error) {
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

	return ui.NewAccessRequest(updatedRequest)
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
