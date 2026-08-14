package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/web/ui/summarizer"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) registerSummarizerHandlers() {
	p.h.GET("/webapi/sites/:site/session-summaries/:session_id", p.h.WithClusterAuth(p.getSessionRecordingSummary))
	p.h.POST("/webapi/sites/:site/session-summaries/batch", p.h.WithClusterAuth(p.batchGetSessionSummaryMetadata))
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

// batchGetSessionSummaryMetadataRequest is a request to retrieve summary metadata for a batch of sessions.
type batchGetSessionSummaryMetadataRequest struct {
	// SessionIDs are the IDs of the sessions to fetch summary metadata for.
	SessionIDs []string `json:"sessionIds"`
}

// batchGetSessionSummaryMetadataResponse carries summary metadata keyed by session ID.
type batchGetSessionSummaryMetadataResponse struct {
	Summaries map[string]summarizer.SummaryMetadata `json:"summaries"`
}

// batchGetSessionSummaryMetadata retrieves lightweight summary metadata for a batch of sessions.
func (h *Plugin) batchGetSessionSummaryMetadata(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var req batchGetSessionSummaryMetadataRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().BatchGetSummaryMetadata(
		r.Context(),
		summarizerv1.BatchGetSummaryMetadataRequest_builder{SessionIds: req.SessionIDs}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	summaries := make(map[string]summarizer.SummaryMetadata, len(response.GetMetadata()))
	for _, md := range response.GetMetadata() {
		summaries[md.GetSessionId()] = summarizer.MakeSummaryMetadata(md)
	}

	return batchGetSessionSummaryMetadataResponse{Summaries: summaries}, nil
}
