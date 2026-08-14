package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	recordingmetadatav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	sessionsearchv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/web/ui/sessionsearch"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

// searchRequest is the JSON request sent by the client as the first WebSocket
// frame of a session search. It mirrors sessionsearchv1.SearchSessionSummariesRequest.
type searchRequest struct {
	StartTime        string            `json:"startTime"`
	EndTime          string            `json:"endTime"`
	Kinds            []string          `json:"kinds,omitempty"`
	Username         string            `json:"username,omitempty"`
	UserRoles        []string          `json:"userRoles,omitempty"`
	AccessRequestIDs []string          `json:"accessRequestIds,omitempty"`
	ResourceKind     string            `json:"resourceKind,omitempty"`
	ResourceName     string            `json:"resourceName,omitempty"`
	ResourceLabels   map[string]string `json:"resourceLabels,omitempty"`
	Severity         string            `json:"severity,omitempty"`
	// NeedsFurtherReviewReasons restricts results to sessions flagged as needing
	// further review for at least one of the given reasons, as web UI tokens
	// (e.g. "too_large"). Empty or unknown tokens are ignored.
	NeedsFurtherReviewReasons []string `json:"needsFurtherReviewReasons,omitempty"`
	SearchQueries             []string `json:"searchQueries,omitempty"`
	MaxResults                uint32   `json:"maxResults,omitempty"`
	BatchToken                string   `json:"batchToken,omitempty"`
	SearchMode                string   `json:"searchMode,omitempty"`
}

// toProto converts the inbound JSON request into the gRPC request message.
func (req searchRequest) toProto() (*sessionsearchv1.SearchSessionSummariesRequest, error) {
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return nil, trace.BadParameter("invalid startTime, expected RFC3339: %v", err)
	}
	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return nil, trace.BadParameter("invalid endTime, expected RFC3339: %v", err)
	}

	b := sessionsearchv1.SearchSessionSummariesRequest_builder{
		StartTime:        timestamppb.New(start),
		EndTime:          timestamppb.New(end),
		Kinds:            req.Kinds,
		UserRoles:        req.UserRoles,
		AccessRequestIds: req.AccessRequestIDs,
		ResourceLabels:   req.ResourceLabels,
		SearchQueries:    req.SearchQueries,
		MaxResults:       req.MaxResults,
		BatchToken:       req.BatchToken,
		SearchMode:       searchModeFromString(req.SearchMode),
	}
	if req.Username != "" {
		username := req.Username
		b.Username = &username
	}
	if req.ResourceKind != "" {
		kind := req.ResourceKind
		b.ResourceKind = &kind
	}
	if req.ResourceName != "" {
		name := req.ResourceName
		b.ResourceName = &name
	}
	if sev := sessionsearch.SeverityFromString(req.Severity); sev != summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED {
		b.Severity = &sev
	}
	if reasons := needsFurtherReviewReasonsFromStrings(req.NeedsFurtherReviewReasons); len(reasons) > 0 {
		b.FilterNeedsFurtherReviewReasons = reasons
	}
	return b.Build(), nil
}

// needsFurtherReviewReasonsFromStrings converts web UI needs-further-review
// tokens into their proto enums, dropping any that are empty or unknown.
func needsFurtherReviewReasonsFromStrings(tokens []string) []summarizerv1.NeedsReviewReason {
	if len(tokens) == 0 {
		return nil
	}
	reasons := make([]summarizerv1.NeedsReviewReason, 0, len(tokens))
	for _, t := range tokens {
		if r := sessionsearch.NeedsReviewReasonFromString(t); r != summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_UNSPECIFIED {
			reasons = append(reasons, r)
		}
	}
	return reasons
}

// Search modes as accepted from the web UI. These are short, meaningful tokens
// rather than the raw proto enum names (e.g. "semantic" not
// "SEARCH_MODE_EMBEDDING_ONLY").
const (
	searchModeHybrid   = "hybrid"
	searchModeKeyword  = "keyword"
	searchModeSemantic = "semantic"
)

// searchModeFromString converts a web UI search-mode token into its proto enum.
// Empty or unknown tokens fall back to the server default (unspecified/hybrid).
func searchModeFromString(s string) sessionsearchv1.SearchMode {
	switch s {
	case searchModeHybrid:
		return sessionsearchv1.SearchMode_SEARCH_MODE_HYBRID
	case searchModeKeyword:
		return sessionsearchv1.SearchMode_SEARCH_MODE_KEYWORD_ONLY
	case searchModeSemantic:
		return sessionsearchv1.SearchMode_SEARCH_MODE_EMBEDDING_ONLY
	default:
		return sessionsearchv1.SearchMode_SEARCH_MODE_UNSPECIFIED
	}
}

// Session search availability states as exposed to the web UI. These describe
// the feature's status in UI terms rather than leaking the access graph's
// Postgres extension names.
const (
	// availabilityAvailable: session search is enabled and ready.
	availabilityAvailable = "available"
	// availabilityUnsupported: the access graph backing this cluster does not
	// implement session search (e.g. an older version).
	availabilityUnsupported = "unsupported"
	// availabilityTextSearchUnavailable: full-text search is unavailable
	// (the pg_trgm extension is missing in the access graph database).
	availabilityTextSearchUnavailable = "text_search_unavailable"
	// availabilitySemanticSearchUnavailable: semantic search is unavailable
	// (the pgvector extension is missing in the access graph database).
	availabilitySemanticSearchUnavailable = "semantic_search_unavailable"
	// availabilityUnknown: the availability could not be determined.
	availabilityUnknown = "unknown"
)

// availabilityString maps the gRPC availability enum to its web UI token.
func availabilityString(a sessionsearchv1.SessionSearchAvailability) string {
	switch a {
	case sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_AVAILABLE:
		return availabilityAvailable
	case sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED:
		return availabilityUnsupported
	case sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE:
		return availabilityTextSearchUnavailable
	case sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE:
		return availabilitySemanticSearchUnavailable
	default:
		return availabilityUnknown
	}
}

const (
	thumbnailFetchConcurrency = 8

	sessionSearchRequestReadLimit   = 64 * 1024
	sessionSearchRequestReadTimeout = 4 * time.Second
)

type sessionSearchMessageType string

const (
	sessionSummaryMessageType       sessionSearchMessageType = "summary"
	sessionThumbnailMessageType     sessionSearchMessageType = "thumbnail"
	sessionBatchCompleteMessageType sessionSearchMessageType = "batch_complete"
	sessionSearchErrorMessageType   sessionSearchMessageType = "error"
)

// sessionSearchMessage is the envelope for every frame sent over the search
// WebSocket. The frontend switches on Type.
type sessionSearchMessage struct {
	Type sessionSearchMessageType `json:"type"`
	Data any                      `json:"data"`
}

type thumbnailData struct {
	SessionID string                                 `json:"sessionId"`
	Thumbnail *web.SessionRecordingThumbnailResponse `json:"thumbnail"`
}

type batchCompleteData struct {
	HasMore        bool   `json:"hasMore"`
	NextBatchToken string `json:"nextBatchToken,omitempty"`
}

type sessionSearchErrorData struct {
	Error string `json:"error"`
}

type sessionSearchEnabledResponse struct {
	Availability string `json:"availability"`
}

func (p *Plugin) registerSessionSearchHandlers() {
	p.h.GET("/webapi/sites/:site/session-search/ws", p.h.WithClusterAuthWebSocket(p.searchSessionSummaries))
	p.h.GET("/webapi/sites/:site/session-search/enabled", p.h.WithClusterAuth(p.sessionSearchEnabled))
}

// searchSessionSummaries streams session summary search results over a
// WebSocket. The client sends a searchRequest as the first frame; the handler
// forwards each summary and (concurrently) its recording thumbnail as JSON
// envelopes, ending with a single batch_complete frame.
func (p *Plugin) searchSessionSummaries(
	w http.ResponseWriter, r *http.Request, params httprouter.Params,
	sctx *web.SessionContext, cluster reversetunnelclient.Cluster, ws *websocket.Conn,
) (any, error) {
	ctx := r.Context()

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		_ = ws.WriteJSON(sessionSearchMessage{
			Type: sessionSearchErrorMessageType,
			Data: sessionSearchErrorData{Error: err.Error()},
		})
		return nil, nil
	}

	streamSessionSearch(ctx, ws, clt.SessionSearchServiceClient(), clt.RecordingMetadataServiceClient())
	return nil, nil
}

// streamSessionSearch runs the session search WebSocket exchange against the
// given gRPC clients. It is separated from the handler so it can be tested with
// mocked clients over a real WebSocket connection.
func streamSessionSearch(
	ctx context.Context,
	ws *websocket.Conn,
	searchClient sessionsearchv1.SessionSearchServiceClient,
	recordingClient recordingmetadatav1.RecordingMetadataServiceClient,
) {
	var writeMu sync.Mutex
	send := func(msgType sessionSearchMessageType, data any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return ws.WriteJSON(sessionSearchMessage{Type: msgType, Data: data})
	}
	sendError := func(err error) {
		_ = send(sessionSearchErrorMessageType, sessionSearchErrorData{Error: err.Error()})
	}

	ws.SetReadLimit(sessionSearchRequestReadLimit)
	if err := ws.SetReadDeadline(time.Now().Add(sessionSearchRequestReadTimeout)); err != nil {
		sendError(trace.Wrap(err, "setting search request read deadline"))
		return
	}

	var req searchRequest
	if err := ws.ReadJSON(&req); err != nil {
		sendError(trace.Wrap(err, "reading search request"))
		return
	}
	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		sendError(trace.Wrap(err, "clearing search request read deadline"))
		return
	}

	protoReq, err := req.toProto()
	if err != nil {
		sendError(err)
		return
	}

	stream, err := searchClient.SearchSessionSummaries(ctx, protoReq)
	if err != nil {
		sendError(err)
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(thumbnailFetchConcurrency)

	var (
		hasMore        bool
		nextBatchToken string
	)

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = g.Wait()
			sendError(trace.Wrap(err))
			return
		}

		switch resp.WhichPayload() {
		case sessionsearchv1.SearchSessionSummariesResponse_Summary_case:
			summary := resp.GetSummary()
			if err := send(sessionSummaryMessageType, sessionsearch.MakeSessionSummary(summary)); err != nil {
				_ = g.Wait()
				return
			}
			sessionID := summary.GetSessionId()
			g.Go(func() error {
				sendThumbnail(gctx, recordingClient, send, sessionID)
				return nil
			})
		case sessionsearchv1.SearchSessionSummariesResponse_BatchComplete_case:
			bc := resp.GetBatchComplete()
			hasMore = bc.GetHasMore()
			nextBatchToken = bc.GetNextBatchToken()
		}
	}

	// Wait for all thumbnail fetches so batch_complete is the final frame.
	_ = g.Wait()

	_ = send(sessionBatchCompleteMessageType, batchCompleteData{
		HasMore:        hasMore,
		NextBatchToken: nextBatchToken,
	})
}

// sendThumbnail fetches the recording thumbnail for a session and sends a
// thumbnail envelope. On a missing or failed thumbnail it sends an empty
// marker (Thumbnail: nil) so the frontend knows there is no thumbnail rather
// than one still loading. A failure here never aborts the search stream.
func sendThumbnail(
	ctx context.Context,
	clt recordingmetadatav1.RecordingMetadataServiceClient,
	send func(sessionSearchMessageType, any) error,
	sessionID string,
) {
	data := thumbnailData{SessionID: sessionID}

	resp, err := clt.GetThumbnail(ctx, recordingmetadatav1.GetThumbnailRequest_builder{
		SessionId: sessionID,
	}.Build())
	switch {
	case err != nil:
		// A missing or failed thumbnail is non-fatal: fall through and send the
		// empty marker so the frontend stops waiting for this session.
	case resp.HasThumbnail():
		t := web.EncodeSessionRecordingThumbnail(resp.GetThumbnail())
		data.Thumbnail = &t
	}

	_ = send(sessionThumbnailMessageType, data)
}

// sessionSearchEnabled reports whether the session search feature is available.
func (p *Plugin) sessionSearchEnabled(
	w http.ResponseWriter, r *http.Request, params httprouter.Params,
	sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.SessionSearchServiceClient().IsEnabled(
		r.Context(), sessionsearchv1.IsEnabledRequest_builder{}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sessionSearchEnabledResponse{
		Availability: availabilityString(resp.GetAvailability()),
	}, nil
}
