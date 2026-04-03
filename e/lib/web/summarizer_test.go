package web

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/session"
)

func makeSessionEndEvent(t *testing.T, sid string) *structpb.Struct {
	s := eventstest.GenerateTestSession(eventstest.SessionParams{
		SessionID:   sid,
		PrintEvents: 0,
	})
	sef, err := events.ToEventFields(s[len(s)-1])
	require.NoError(t, err)
	eventStruct, err := structpb.NewStruct(sef)
	require.NoError(t, err)
	return eventStruct
}

func uploadSummary(t *testing.T, s *webSuite, summary *summarizerv1.Summary) {
	summaryJson, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary)
	require.NoError(t, err)
	_, err = s.testAuthServer.AuthServer.UploadHandler.UploadSummary(
		t.Context(), session.ID(summary.SessionId), bytes.NewReader(summaryJson),
	)
	require.NoError(t, err)
}

func TestGetRecordingSummary(t *testing.T) {
	ctx := t.Context()
	s := newWebSuite(t,
		withUploadHandler(eventstest.NewMemoryUploader()),
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Policy: {Enabled: true},
					// TODO(emargetis): update after https://github.com/gravitational/teleport/pull/63117 merges
					// ensures that /e does not break when new Access Graph entitlement is added to Teleport
					"AccessGraph": {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo", withExtraRules(types.Rule{
		Resources: []string{types.KindSession},
		Verbs:     []string{types.VerbRead},
	}))
	clusterName := s.testAuthServer.ClusterName()

	// Upload an example successful summary.
	successfulSessionID := "d2f22f16-d3bb-4b0b-bcef-00daa17190d9"
	uploadSummary(t, s, &summarizerv1.Summary{
		SessionId:           successfulSessionID,
		State:               summarizerv1.SummaryState_SUMMARY_STATE_SUCCESS,
		InferenceStartedAt:  timestamppb.New(time.Date(2025, 8, 1, 11, 12, 0, 0, time.UTC)),
		InferenceFinishedAt: timestamppb.New(time.Date(2025, 8, 1, 11, 12, 30, 0, time.UTC)),
		Content:             "Thank you for a very enjoyable game.",
		ModelName:           "HAL 9000",
		SessionEndEvent:     makeSessionEndEvent(t, successfulSessionID),
	})

	// Upload an example pending summary.
	pendingSessionID := "3c8f9e27-831d-4d0c-af3c-41893a863df0"
	uploadSummary(t, s, &summarizerv1.Summary{
		SessionId:           pendingSessionID,
		State:               summarizerv1.SummaryState_SUMMARY_STATE_PENDING,
		InferenceStartedAt:  timestamppb.New(time.Date(2025, 8, 2, 11, 12, 0, 0, time.UTC)),
		InferenceFinishedAt: nil,
		ModelName:           "HAL 9000",
		SessionEndEvent:     makeSessionEndEvent(t, pendingSessionID),
	})

	// Upload an example failed summary.
	failedSessionID := "1c64b043-ca5d-4de3-b289-0a24c5fbcb16"
	uploadSummary(t, s, &summarizerv1.Summary{
		SessionId:           failedSessionID,
		State:               summarizerv1.SummaryState_SUMMARY_STATE_ERROR,
		InferenceStartedAt:  timestamppb.New(time.Date(2025, 8, 3, 11, 12, 0, 0, time.UTC)),
		InferenceFinishedAt: timestamppb.New(time.Date(2025, 8, 3, 11, 12, 30, 0, time.UTC)),
		ErrorMessage:        "I'm sorry, Dave. I'm afraid I can't do that.",
		ModelName:           "HAL 9000",
		SessionEndEvent:     makeSessionEndEvent(t, pendingSessionID),
	})

	cases := []struct {
		name      string
		sessionID string
		expected  map[string]any
		errorIs   func(err error) bool
	}{
		{
			name:      "existing successful summary",
			sessionID: successfulSessionID,
			expected: map[string]any{
				"sessionId":           successfulSessionID,
				"state":               "SUMMARY_STATE_SUCCESS",
				"inferenceStartedAt":  "2025-08-01T11:12:00Z",
				"inferenceFinishedAt": "2025-08-01T11:12:30Z",
				"content":             "Thank you for a very enjoyable game.",
			},
		},
		{
			name:      "existing pending summary",
			sessionID: pendingSessionID,
			expected: map[string]any{
				"sessionId":          pendingSessionID,
				"state":              "SUMMARY_STATE_PENDING",
				"inferenceStartedAt": "2025-08-02T11:12:00Z",
			},
		},
		{
			name:      "existing failed summary",
			sessionID: failedSessionID,
			expected: map[string]any{
				"sessionId":           failedSessionID,
				"state":               "SUMMARY_STATE_ERROR",
				"inferenceStartedAt":  "2025-08-03T11:12:00Z",
				"inferenceFinishedAt": "2025-08-03T11:12:30Z",
				"errorMessage":        "I'm sorry, Dave. I'm afraid I can't do that.",
			},
		},
		{
			name:      "nonexistent summary",
			sessionID: "6fa2b7f1-b977-417f-bb2c-af255391cf5e",
			errorIs:   trace.IsNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "session-summaries", tc.sessionID)
			resp, err := webPack.clt.Get(ctx, endpoint, nil)

			if tc.errorIs != nil {
				require.Error(t, err)
				assert.True(t, tc.errorIs(err))
			} else {
				require.NoError(t, err)
			}

			if tc.expected != nil {
				var got map[string]any
				err = json.Unmarshal(resp.Bytes(), &got)
				require.NoError(t, err)
				assert.Empty(t, cmp.Diff(tc.expected, got))
			}
		})
	}
}
