package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	recordingmetadatav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	sessionsearchv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/web/ui/sessionsearch"
	"github.com/gravitational/teleport/lib/web"
)

func TestSearchRequestToProto(t *testing.T) {
	tests := []struct {
		name    string
		req     searchRequest
		wantErr bool
		assert  func(t *testing.T, got *sessionsearchv1.SearchSessionSummariesRequest)
	}{
		{
			name: "all fields",
			req: searchRequest{
				StartTime:                 "2026-01-01T00:00:00Z",
				EndTime:                   "2026-01-02T00:00:00Z",
				Kinds:                     []string{"ssh"},
				Username:                  "alice",
				UserRoles:                 []string{"dev"},
				AccessRequestIDs:          []string{"req-1"},
				ResourceKind:              "node",
				ResourceName:              "host-1",
				ResourceLabels:            map[string]string{"env": "prod"},
				Severity:                  "high",
				NeedsFurtherReviewReasons: []string{"too_large", "garbage", "classifier_matched"},
				SearchQueries:             []string{"rm -rf"},
				MaxResults:                50,
				BatchToken:                "tok",
				SearchMode:                "keyword",
			},
			assert: func(t *testing.T, got *sessionsearchv1.SearchSessionSummariesRequest) {
				require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), got.GetStartTime().AsTime())
				require.Equal(t, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), got.GetEndTime().AsTime())
				require.Equal(t, []string{"ssh"}, got.GetKinds())
				require.Equal(t, "alice", got.GetUsername())
				require.Equal(t, []string{"dev"}, got.GetUserRoles())
				require.Equal(t, []string{"req-1"}, got.GetAccessRequestIds())
				require.Equal(t, "node", got.GetResourceKind())
				require.Equal(t, "host-1", got.GetResourceName())
				require.Equal(t, map[string]string{"env": "prod"}, got.GetResourceLabels())
				require.Equal(t, summarizerv1.RiskLevel_RISK_LEVEL_HIGH, got.GetSeverity())
				// Unknown tokens ("garbage") are dropped.
				require.Equal(t, []summarizerv1.NeedsReviewReason{
					summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
					summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_CLASSIFIER_MATCHED,
				}, got.GetFilterNeedsFurtherReviewReasons())
				require.Equal(t, []string{"rm -rf"}, got.GetSearchQueries())
				require.Equal(t, uint32(50), got.GetMaxResults())
				require.Equal(t, "tok", got.GetBatchToken())
				require.Equal(t, sessionsearchv1.SearchMode_SEARCH_MODE_KEYWORD_ONLY, got.GetSearchMode())
			},
		},
		{
			name: "optional fields unset",
			req: searchRequest{
				StartTime: "2026-01-01T00:00:00Z",
				EndTime:   "2026-01-02T00:00:00Z",
			},
			assert: func(t *testing.T, got *sessionsearchv1.SearchSessionSummariesRequest) {
				require.Empty(t, got.GetUsername())
				require.Empty(t, got.GetResourceKind())
				require.Empty(t, got.GetResourceName())
				require.Equal(t, summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED, got.GetSeverity())
				require.Empty(t, got.GetFilterNeedsFurtherReviewReasons())
				require.Equal(t, sessionsearchv1.SearchMode_SEARCH_MODE_UNSPECIFIED, got.GetSearchMode())
			},
		},
		{
			name:    "invalid start time",
			req:     searchRequest{StartTime: "nope", EndTime: "2026-01-02T00:00:00Z"},
			wantErr: true,
		},
		{
			name:    "missing end time",
			req:     searchRequest{StartTime: "2026-01-01T00:00:00Z", EndTime: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.req.toProto()
			if tt.wantErr {
				require.True(t, trace.IsBadParameter(err))
				return
			}
			require.NoError(t, err)
			tt.assert(t, got)
		})
	}
}

func TestStringMappings(t *testing.T) {
	t.Run("availabilityString", func(t *testing.T) {
		cases := map[sessionsearchv1.SessionSearchAvailability]string{
			sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_AVAILABLE:             "available",
			sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED:       "unsupported",
			sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE:   "text_search_unavailable",
			sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE: "semantic_search_unavailable",
			sessionsearchv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED:           "unknown",
		}
		for in, want := range cases {
			require.Equal(t, want, availabilityString(in))
		}
	})

	t.Run("searchModeFromString", func(t *testing.T) {
		cases := map[string]sessionsearchv1.SearchMode{
			"hybrid":   sessionsearchv1.SearchMode_SEARCH_MODE_HYBRID,
			"keyword":  sessionsearchv1.SearchMode_SEARCH_MODE_KEYWORD_ONLY,
			"semantic": sessionsearchv1.SearchMode_SEARCH_MODE_EMBEDDING_ONLY,
			"":         sessionsearchv1.SearchMode_SEARCH_MODE_UNSPECIFIED,
			"garbage":  sessionsearchv1.SearchMode_SEARCH_MODE_UNSPECIFIED,
		}
		for in, want := range cases {
			require.Equal(t, want, searchModeFromString(in))
		}
	})
}

// fakeSearchStream is a mock SearchSessionSummaries server stream. It yields its
// messages in order, then recvErr (if set) or io.EOF.
type fakeSearchStream struct {
	grpc.ServerStreamingClient[sessionsearchv1.SearchSessionSummariesResponse]
	msgs    []*sessionsearchv1.SearchSessionSummariesResponse
	idx     int
	recvErr error
}

func (s *fakeSearchStream) Recv() (*sessionsearchv1.SearchSessionSummariesResponse, error) {
	if s.idx < len(s.msgs) {
		m := s.msgs[s.idx]
		s.idx++
		return m, nil
	}
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	return nil, io.EOF
}

// fakeSessionSearchClient is a mock SessionSearchServiceClient. It records the
// request it received and returns the configured stream (or error).
type fakeSessionSearchClient struct {
	sessionsearchv1.SessionSearchServiceClient
	stream    grpc.ServerStreamingClient[sessionsearchv1.SearchSessionSummariesResponse]
	streamErr error
	gotReq    *sessionsearchv1.SearchSessionSummariesRequest
}

func (c *fakeSessionSearchClient) SearchSessionSummaries(ctx context.Context, in *sessionsearchv1.SearchSessionSummariesRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[sessionsearchv1.SearchSessionSummariesResponse], error) {
	c.gotReq = in
	return c.stream, c.streamErr
}

// fakeRecordingClient is a mock RecordingMetadataServiceClient. It returns a
// configured thumbnail per session, or an error per session.
type fakeRecordingClient struct {
	recordingmetadatav1.RecordingMetadataServiceClient
	thumbnails map[string]*recordingmetadatav1.SessionRecordingThumbnail
	errs       map[string]error
}

func (c *fakeRecordingClient) GetThumbnail(ctx context.Context, in *recordingmetadatav1.GetThumbnailRequest, opts ...grpc.CallOption) (*recordingmetadatav1.GetThumbnailResponse, error) {
	sid := in.GetSessionId()
	if err := c.errs[sid]; err != nil {
		return nil, err
	}
	return recordingmetadatav1.GetThumbnailResponse_builder{
		Thumbnail: c.thumbnails[sid],
	}.Build(), nil
}

// testEnvelope mirrors sessionSearchMessage for decoding on the client side,
// keeping the payload raw so each test can decode it into the expected type.
type testEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// runStreamSessionSearch spins up a real WebSocket server running
// streamSessionSearch with the given mocked clients, sends request as the first
// frame, and returns every envelope the server emits before closing.
func runStreamSessionSearch(t *testing.T, search sessionsearchv1.SessionSearchServiceClient, recording recordingmetadatav1.RecordingMetadataServiceClient, request any) []testEnvelope {
	t.Helper()

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		streamSessionSearch(r.Context(), ws, search, recording)
	}))
	t.Cleanup(srv.Close)

	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		t.Cleanup(func() { _ = resp.Body.Close() })
	}
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, conn.WriteJSON(request))

	var msgs []testEnvelope
	for {
		var env testEnvelope
		if err := conn.ReadJSON(&env); err != nil {
			break
		}
		msgs = append(msgs, env)
	}
	return msgs
}

func summaryResponse(s *sessionsearchv1.SessionSummary) *sessionsearchv1.SearchSessionSummariesResponse {
	return sessionsearchv1.SearchSessionSummariesResponse_builder{Summary: s}.Build()
}

func batchCompleteResponse(hasMore bool, token string) *sessionsearchv1.SearchSessionSummariesResponse {
	return sessionsearchv1.SearchSessionSummariesResponse_builder{
		BatchComplete: sessionsearchv1.SearchSessionSummariesResponse_BatchComplete_builder{
			HasMore:        hasMore,
			NextBatchToken: token,
		}.Build(),
	}.Build()
}

func TestStreamSessionSearch(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		search    *fakeSessionSearchClient
		recording *fakeRecordingClient
		req       searchRequest
		assert    func(t *testing.T, search *fakeSessionSearchClient, msgs []testEnvelope)
	}{
		{
			name: "streams summaries and thumbnails",
			search: &fakeSessionSearchClient{
				stream: &fakeSearchStream{msgs: []*sessionsearchv1.SearchSessionSummariesResponse{
					summaryResponse(sessionsearchv1.SessionSummary_builder{
						SessionId:    "sid-1",
						Kind:         "ssh",
						SessionStart: timestamppb.New(start),
						Username:     "alice",
						Severity:     summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
					}.Build()),
					summaryResponse(sessionsearchv1.SessionSummary_builder{
						SessionId:    "sid-2",
						Kind:         "k8s",
						SessionStart: timestamppb.New(start),
						Username:     "bob",
					}.Build()),
					batchCompleteResponse(true, "next-token"),
				}},
			},
			recording: &fakeRecordingClient{
				thumbnails: map[string]*recordingmetadatav1.SessionRecordingThumbnail{
					"sid-1": recordingmetadatav1.SessionRecordingThumbnail_builder{
						Svg:  []byte("<svg/>"),
						Cols: 80,
						Rows: 24,
					}.Build(),
				},
				// sid-2 has no thumbnail and returns NotFound.
				errs: map[string]error{
					"sid-2": trace.NotFound("no thumbnail for sid-2"),
				},
			},
			req: searchRequest{
				StartTime:  start.Format(time.RFC3339),
				EndTime:    end.Format(time.RFC3339),
				Kinds:      []string{"ssh", "k8s"},
				Username:   "alice",
				Severity:   "high",
				SearchMode: "keyword",
				MaxResults: 25,
			},
			assert: func(t *testing.T, search *fakeSessionSearchClient, msgs []testEnvelope) {
				require.NotNil(t, search.gotReq)
				require.Equal(t, []string{"ssh", "k8s"}, search.gotReq.GetKinds())
				require.Equal(t, "alice", search.gotReq.GetUsername())
				require.Equal(t, summarizerv1.RiskLevel_RISK_LEVEL_HIGH, search.gotReq.GetSeverity())
				require.Equal(t, sessionsearchv1.SearchMode_SEARCH_MODE_KEYWORD_ONLY, search.gotReq.GetSearchMode())
				require.Equal(t, uint32(25), search.gotReq.GetMaxResults())
				require.Equal(t, start, search.gotReq.GetStartTime().AsTime())
				require.Equal(t, end, search.gotReq.GetEndTime().AsTime())

				var (
					summaries  []sessionsearch.SessionSummary
					thumbsByID = map[string]*web.SessionRecordingThumbnailResponse{}
					gotThumb   = map[string]bool{}
					batch      batchCompleteData
					errEnv     []sessionSearchErrorData
				)
				for _, env := range msgs {
					switch env.Type {
					case string(sessionSummaryMessageType):
						var s sessionsearch.SessionSummary
						require.NoError(t, json.Unmarshal(env.Data, &s))
						summaries = append(summaries, s)
					case string(sessionThumbnailMessageType):
						var td thumbnailData
						require.NoError(t, json.Unmarshal(env.Data, &td))
						thumbsByID[td.SessionID] = td.Thumbnail
						gotThumb[td.SessionID] = true
					case string(sessionBatchCompleteMessageType):
						require.NoError(t, json.Unmarshal(env.Data, &batch))
					case string(sessionSearchErrorMessageType):
						var e sessionSearchErrorData
						require.NoError(t, json.Unmarshal(env.Data, &e))
						errEnv = append(errEnv, e)
					}
				}

				require.Empty(t, errEnv, "no error envelopes expected")

				// Summaries are streamed in order with UI-friendly fields.
				require.Len(t, summaries, 2)
				require.Equal(t, "sid-1", summaries[0].SessionID)
				require.Equal(t, "high", summaries[0].Severity)
				require.Equal(t, "alice", summaries[0].Username)
				require.Equal(t, "sid-2", summaries[1].SessionID)
				require.Empty(t, summaries[1].Severity)

				// Each session gets a thumbnail envelope: sid-1 with content, sid-2 a null marker.
				require.True(t, gotThumb["sid-1"])
				require.True(t, gotThumb["sid-2"])
				require.NotNil(t, thumbsByID["sid-1"])
				require.Equal(t, "<svg/>", thumbsByID["sid-1"].Svg)
				require.Equal(t, int32(80), thumbsByID["sid-1"].Cols)
				require.Nil(t, thumbsByID["sid-2"], "missing thumbnail must be a null marker")

				// batch_complete is strictly the last frame and carries pagination state.
				require.NotEmpty(t, msgs)
				require.Equal(t, string(sessionBatchCompleteMessageType), msgs[len(msgs)-1].Type)
				require.True(t, batch.HasMore)
				require.Equal(t, "next-token", batch.NextBatchToken)
			},
		},
		{
			name:      "stream open error",
			search:    &fakeSessionSearchClient{streamErr: trace.AccessDenied("denied")},
			recording: &fakeRecordingClient{},
			req: searchRequest{
				StartTime: "2026-01-01T00:00:00Z",
				EndTime:   "2026-01-02T00:00:00Z",
			},
			assert: func(t *testing.T, search *fakeSessionSearchClient, msgs []testEnvelope) {
				require.Len(t, msgs, 1)
				require.Equal(t, string(sessionSearchErrorMessageType), msgs[0].Type)
				var e sessionSearchErrorData
				require.NoError(t, json.Unmarshal(msgs[0].Data, &e))
				require.Contains(t, e.Error, "denied")
			},
		},
		{
			name: "mid-stream error",
			search: &fakeSessionSearchClient{
				stream: &fakeSearchStream{
					msgs: []*sessionsearchv1.SearchSessionSummariesResponse{
						summaryResponse(sessionsearchv1.SessionSummary_builder{
							SessionId:    "sid-1",
							SessionStart: timestamppb.New(start),
						}.Build()),
					},
					recvErr: trace.ConnectionProblem(io.ErrUnexpectedEOF, "stream broke"),
				},
			},
			recording: &fakeRecordingClient{},
			req: searchRequest{
				StartTime: "2026-01-01T00:00:00Z",
				EndTime:   "2026-01-02T00:00:00Z",
			},
			assert: func(t *testing.T, search *fakeSessionSearchClient, msgs []testEnvelope) {
				var sawError, sawBatch bool
				for _, env := range msgs {
					switch env.Type {
					case string(sessionSearchErrorMessageType):
						sawError = true
					case string(sessionBatchCompleteMessageType):
						sawBatch = true
					}
				}
				require.True(t, sawError, "mid-stream error must produce an error envelope")
				require.False(t, sawBatch, "no batch_complete after a mid-stream error")
			},
		},
		{
			name:      "invalid request",
			search:    &fakeSessionSearchClient{},
			recording: &fakeRecordingClient{},
			// Invalid startTime fails toProto before any stream is opened.
			req: searchRequest{StartTime: "not-a-time", EndTime: "2026-01-02T00:00:00Z"},
			assert: func(t *testing.T, search *fakeSessionSearchClient, msgs []testEnvelope) {
				require.Len(t, msgs, 1)
				require.Equal(t, string(sessionSearchErrorMessageType), msgs[0].Type)
				require.Nil(t, search.gotReq, "stream must not be opened for an invalid request")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := runStreamSessionSearch(t, tt.search, tt.recording, tt.req)
			tt.assert(t, tt.search, msgs)
		})
	}
}
