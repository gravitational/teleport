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

	return createAccessRequest(r.Context(), clt, req.Reason, ctx.GetUser())
}

func createAccessRequest(ctx context.Context, clt accessRequestAPIGetter, reason, user string) (*ui.AccessRequest, error) {
	// Initial version of web UI does not ask user to specify what roles to request.
	// Wild card is used so that the auth server automatically fills in the request role
	// the user is allowed to request as defined in their rbac yaml.
	req, err := services.NewAccessRequest(user, []string{"*"}...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.SetRequestReason(reason)

	if err := clt.CreateAccessRequest(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.AccessRequest{
		ID:    req.GetMetadata().Name,
		State: req.GetState().String(),
	}, nil
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

	req := reqs[0]

	// Access Request state NONE is its empty value and the empty value
	// is treated internally as an error, so it should return as an error.
	if req.GetState().IsNone() {
		return nil, trace.AccessDenied("access request %q, state is set to none", requestID)
	}

	return &ui.AccessRequest{
		ID:     req.GetMetadata().Name,
		State:  req.GetState().String(),
		Reason: req.GetResolveReason(),
	}, nil
}

type accessRequestAPIGetter interface {
	// CreateAccessRequest stores a new access request.
	CreateAccessRequest(ctx context.Context, req services.AccessRequest) error
	// GetAccessRequests gets all currently active access requests.
	GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error)
}

type accessRequestParameters struct {
	// Reason is the AccessRequest request reason.
	Reason string `json:"reason"`
}
