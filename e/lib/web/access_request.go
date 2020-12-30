package web

import (
	"context"
	"net/http"

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
	// If no specific roles were requested, then by default wild card is used which
	// the auth server automatically fills in with all roles the user is allowed to request.
	rolesRequested := []string{services.Wildcard}
	if len(request.Roles) != 0 {
		rolesRequested = request.Roles
	}

	req, err := services.NewAccessRequest(user, rolesRequested...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.SetRequestReason(request.Reason)

	if err := clt.CreateAccessRequest(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return getAccessRequest(ctx, clt, req.GetMetadata().Name, user)
}

func (p *Plugin) getAccessRequestHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	requestID := params.ByName("requestId")

	return getAccessRequest(r.Context(), clt, requestID, ctx.GetUser())
}

func getAccessRequest(ctx context.Context, clt accessRequestAPIGetter, requestID, user string) (*ui.AccessRequest, error) {
	if requestID == "" {
		return nil, trace.BadParameter("missing request id")
	}

	requestFilter := services.AccessRequestFilter{
		User: user,
		ID:   requestID,
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
	filter := services.AccessRequestFilter{
		User: query.Get("user"),
	}

	return getAccessRequests(r.Context(), clt, filter)
}

func getAccessRequests(ctx context.Context, clt accessRequestAPIGetter, filter services.AccessRequestFilter) ([]ui.AccessRequest, error) {
	reqs, err := clt.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiReqs := make([]ui.AccessRequest, 0, len(reqs))
	for _, req := range reqs {
		uiReq, err := ui.NewAccessRequest(req)
		if err != nil {
			log.Warnf("Failed to process access request: %v", err)
			continue
		}

		uiReqs = append(uiReqs, *uiReq)
	}

	return uiReqs, nil
}

type accessRequestAPIGetter interface {
	// CreateAccessRequest stores a new access request.
	CreateAccessRequest(ctx context.Context, req services.AccessRequest) error
	// GetAccessRequests gets all currently active access requests.
	GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error)
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
}
