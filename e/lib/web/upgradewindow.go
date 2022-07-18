package web

import (
	"net/http"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
)

type updateUpgradeWindowStartReq struct {
	UpgradeWindowStart string `json:"upgradeWindowStart"`
}

type getUpgradeWindowStartRes struct {
	UpgradeWindowStart string `json:"upgradeWindowStart"`
}

func (p *Plugin) getUpgradeWindowStartHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetAccountUpgradeWindowStart(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return getUpgradeWindowStartRes{
		UpgradeWindowStart: res.UpgradeWindowStart,
	}, nil
}

func (p *Plugin) updateUpgradeWindowStartHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req updateUpgradeWindowStartReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := client.UpdateAccountUpgradeWindowStart(r.Context(), &cloudapi.UpdateAccountUpgradeWindowStartRequest{
		UpgradeWindowStart: req.UpgradeWindowStart,
	})
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
			UpgradeWindowStart: req.UpgradeWindowStart,
		},
	}

	if err := p.h.GetProxyClient().EmitAuditEvent(r.Context(), event); err != nil {
		p.Log.WithError(err).WithFields(logrus.Fields{
			"user":                 event.UserMetadata.User,
			"upgrade_window_start": event.UpgradeWindowStartMetadata.UpgradeWindowStart,
		}).Warn("Failed to emit window upgrade start update event.")
	}

	return web.OK(), nil
}
