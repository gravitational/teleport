package web

import (
	"fmt"
	"mime"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/protobuf/encoding/protojson"
	googleproto "google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) getBillingSummaryInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetBillingSummaryInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getBillingInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetBillingInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetAccountUpgradeWindowStartHour(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) updateUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req cloudapi.UpdateAccountUpgradeWindowStartHourRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := client.UpdateAccountUpgradeWindowStartHour(r.Context(), &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	// emit audit event
	event := &apievents.UpgradeWindowStartUpdate{
		Metadata: apievents.Metadata{
			Type: events.UpgradeWindowStartUpdateEvent,
			Code: events.UpgradeWindowStartUpdatedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: ctx.GetUser(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: ctx.GetSessionID(),
		},
		UpgradeWindowStartMetadata: apievents.UpgradeWindowStartMetadata{
			UpgradeWindowStart: fmt.Sprintf("%02d:00:00", req.UpgradeWindowStartHour),
		},
	}

	if err := p.h.GetProxyClient().EmitAuditEvent(r.Context(), event); err != nil {
		p.Logger.WarnContext(r.Context(), "Failed to emit window upgrade start update event",
			"error", err,
			"user", event.UserMetadata.User,
			"upgrade_window_start", event.UpgradeWindowStartMetadata.UpgradeWindowStart,
		)
	}

	return web.OK(), nil
}

func (p *Plugin) surveyCompanyResponsesHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error) {
	res, err := client.GetSurveyCompany(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) surveyResultsHandler(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req cloudapi.SetSurveyResultsRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := client.SetSurveyResults(r.Context(), &cloudapi.SetSurveyResultsRequest{
		CompanyName:   req.CompanyName,
		EmployeeCount: req.EmployeeCount,
		Resources:     req.Resources,
		Role:          req.Role,
		Team:          req.Team,
		Username:      ctx.GetUser(),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return web.OK(), nil
}

// ReadProtoJSON reads a proto request and unmarshals it
// into passed interface{} obj
func (p *Plugin) readProtoJSON(r *http.Request, val googleproto.Message) error {
	// Check content type to mitigate CSRF attack.
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		p.Logger.WarnContext(r.Context(), "Error parsing media type for reading JSON", "error", err)
		return trace.BadParameter("invalid request")
	}

	if contentType != "application/json" {
		p.Logger.WarnContext(r.Context(), "Invalid HTTP request header content-type for reading JSON", "content_type", contentType)
		return trace.BadParameter("invalid request")
	}

	data, err := utils.ReadAtMost(r.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := protojson.Unmarshal(data, val); err != nil {
		return trace.BadParameter("request: %v", err.Error())
	}
	return nil
}
