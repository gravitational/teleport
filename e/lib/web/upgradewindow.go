package web

import (
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

type updateUpgradeWindowStartHourReq struct {
	UpgradeWindowStartHour int64 `json:"upgradeWindowStart"`
}

type getUpgradeWindowStartHourRes struct {
	UpgradeWindowStartHour int64 `json:"upgradeWindowStart"`
}

func (p *Plugin) getUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetAccountUpgradeWindowStartHour(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return getUpgradeWindowStartHourRes{
		UpgradeWindowStartHour: res.UpgradeWindowStartHour,
	}, nil
}

func (p *Plugin) updateUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req updateUpgradeWindowStartHourReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := client.UpdateAccountUpgradeWindowStartHour(r.Context(), &cloudapi.UpdateAccountUpgradeWindowStartHourRequest{
		UpgradeWindowStartHour: req.UpgradeWindowStartHour,
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
			UpgradeWindowStart: fmt.Sprintf("%02d:00:00", req.UpgradeWindowStartHour),
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
