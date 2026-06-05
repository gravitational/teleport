package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/web/ui/summarizer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) registerSummarizerHandlers() {
	p.h.GET("/webapi/sites/:site/session-summaries/:session_id", p.h.WithClusterAuth(p.getSessionRecordingSummary))
}

// getSessionRecordingSummary retrieves a summary of a session recording.
func (h *Plugin) getSessionRecordingSummary(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	sessionId := p.ByName("session_id")
	if sessionId == "" {
		return nil, trace.BadParameter("session_id is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().GetSummary(
		r.Context(),
		summarizerv1.GetSummaryRequest_builder{SessionId: sessionId}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return summarizer.MakeSummary(response.GetSummary()), nil
}
