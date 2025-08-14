package web

import (
	"net/http"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// getSessionRecordingSummary retrieves a summary of a session recording.
func (h *Handler) getSessionRecordingSummary(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite,
) (any, error) {
	sessionId := p.ByName("session_id")
	if sessionId == "" {
		return nil, trace.BadParameter("session_id is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().GetSummary(
		r.Context(),
		&summarizerv1.GetSummaryRequest{SessionId: sessionId},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeSessionRecordingSummary(response.Summary), nil
}
