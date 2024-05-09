package web

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/lib/auth/crownjewel"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

type createCrownJewelRequest struct {
	TeleportMatcher *crownjewelv1.TeleportMatcher `json:"teleport_matcher"`
	AwsMatcher      *crownjewelv1.AWSMatcher      `json:"aws_matcher"`
}

func (c *createCrownJewelRequest) CheckAndSetDefaults() error {
	if c.TeleportMatcher == nil && c.AwsMatcher == nil {
		return trace.BadParameter("either teleport_matcher or aws_matcher must be set")
	}
	return nil
}

func (p *Plugin) markCrownJewel(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, webCtx *web.SessionContext) (any, error) {
	var req createCrownJewelRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.BadParameter("invalid request: %v", err)
	}

	authClient, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceID := uuid.New().String()

	spec := &crownjewelv1.CrownJewelSpec{}

	if req.TeleportMatcher != nil {
		spec.TeleportMatchers = append(spec.TeleportMatchers, req.TeleportMatcher)
	}

	if req.AwsMatcher != nil {
		spec.AwsMatchers = append(spec.AwsMatchers, req.AwsMatcher)
	}

	resource, err := crownjewel.NewCrownJewel(resourceID, spec)
	if err != nil {
		return nil, trace.BadParameter("incorrect crown jewel resource: %v", err)
	}

	p.Log.Debug("Creating crown jewel", "id", resourceID)
	ctx := r.Context()
	resp, err := authClient.CrownJewelServiceClient().CreateCrownJewel(ctx, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.Log.Debug("Crown jewel created", "resp", resp)

	return `{"status": "ok"}`, nil
}

func (p *Plugin) deleteCrownJewel(_ http.ResponseWriter, r *http.Request, params httprouter.Params, webCtx *web.SessionContext) (any, error) {
	authClient, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	crownJewelName := params.ByName("name")
	p.Log.Debug("Deleting crown jewel", "name", crownJewelName)
	ctx := r.Context()
	err = authClient.CrownJewelServiceClient().DeleteCrownJewel(ctx, crownJewelName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.Log.Debug("Crown jewel deleted", "name", crownJewelName)

	return nil, nil
}
