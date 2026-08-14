package web

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth/crownjewel"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

type createCrownJewelRequest struct {
	TeleportMatcher *crownjewelv1.TeleportMatcher `json:"teleport_matcher"`
	AwsMatcher      *crownjewelv1.AWSMatcher      `json:"aws_matcher"`
	Query           string                        `json:"query"`
	Description     string                        `json:"description"`
}

func (c *createCrownJewelRequest) CheckAndSetDefaults() error {
	if c.TeleportMatcher == nil && c.AwsMatcher == nil && c.Query == "" {
		return trace.BadParameter("at least one matcher must be set")
	}
	if len(c.Description) > 255 {
		return trace.BadParameter("description must be less than 255 characters")
	}
	return nil
}

type listCrownJewelsResponse struct {
	CrownJewels []*ui.CrownJewel `json:"crown_jewels"`
	NextToken   string           `json:"next_token"`
}

func (p *Plugin) getCrownJewel(_ http.ResponseWriter, r *http.Request, params httprouter.Params, webCtx *web.SessionContext) (any, error) {
	authClient, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	crownJewelName := params.ByName("name")

	ctx := r.Context()
	crownJewel, err :=
		authClient.CrownJewelServiceClient().GetCrownJewel(ctx, crownJewelName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ToCrownJewel(crownJewel), nil
}

func (*Plugin) listCrownJewels(_ http.ResponseWriter, r *http.Request, p httprouter.Params, webCtx *web.SessionContext) (any, error) {
	authClient, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx := r.Context()

	limit := int64(0) /* default limit */
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, err = strconv.ParseInt(l, 10, 64)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if limit < 0 {
			return nil, trace.BadParameter("limit must be greater than or equal to 0")
		}
		if limit > 1000 {
			return nil, trace.BadParameter("limit must be less than or equal to 1000")
		}
	}

	lastToken := ""
	if t := r.URL.Query().Get("next_token"); t != "" {
		token, err := base64.RawURLEncoding.DecodeString(t)
		if err != nil {
			return nil, trace.Wrap(err, "invalid next token")
		}
		lastToken = string(token)
	}

	crownJewels := make([]*crownjewelv1.CrownJewel, 0, limit)
	for i := int64(0); i < limit || limit == 0; {
		batch, nextToken, err := authClient.CrownJewelServiceClient().ListCrownJewels(ctx, limit, lastToken)
		if err != nil {
			return nil, trace.Wrap(err, "unable to get access lists")
		}
		crownJewels = append(crownJewels, batch...)

		lastToken = nextToken
		if nextToken == "" || len(batch) == 0 {
			break
		}
		i += int64(len(batch))
	}

	// Convert the next token to base64
	lastTokenB64 := base64.RawURLEncoding.EncodeToString([]byte(lastToken))

	return &listCrownJewelsResponse{
		CrownJewels: ui.ToCrownJewels(crownJewels),
		NextToken:   lastTokenB64,
	}, nil
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

	spec := crownjewelv1.CrownJewelSpec_builder{
		Query: req.Query,
	}.Build()

	if req.TeleportMatcher != nil {
		spec.SetTeleportMatchers(append(spec.GetTeleportMatchers(), req.TeleportMatcher))
	}

	if req.AwsMatcher != nil {
		spec.SetAwsMatchers(append(spec.GetAwsMatchers(), req.AwsMatcher))
	}

	resource, err := crownjewel.NewCrownJewel(resourceID, spec)
	if err != nil {
		return nil, trace.BadParameter("incorrect crown jewel resource: %v", err)
	}

	if req.Description != "" {
		resource.GetMetadata().SetDescription(req.Description)
	}

	ctx := r.Context()
	p.Logger.DebugContext(ctx, "Creating crown jewel", "id", resourceID)

	resp, err := authClient.CrownJewelServiceClient().CreateCrownJewel(ctx, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ToCrownJewel(resp), nil
}

func (p *Plugin) deleteCrownJewel(_ http.ResponseWriter, r *http.Request, params httprouter.Params, webCtx *web.SessionContext) (any, error) {
	ctx := r.Context()
	authClient, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	crownJewelName := params.ByName("name")
	p.Logger.DebugContext(ctx, "Deleting crown jewel", "name", crownJewelName)

	err = authClient.CrownJewelServiceClient().DeleteCrownJewel(ctx, crownJewelName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}
