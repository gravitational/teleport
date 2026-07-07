package sessionsearch

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	sessionsearchv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestMakeSessionSummary(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	end := time.Date(2026, 1, 2, 3, 10, 5, 0, time.UTC)
	traits, err := structpb.NewStruct(map[string]any{"login": "alice"})
	require.NoError(t, err)

	tests := []struct {
		name   string
		in     *sessionsearchv1.SessionSummary
		assert func(t *testing.T, got SessionSummary)
	}{
		{
			name: "all fields",
			in: sessionsearchv1.SessionSummary_builder{
				SessionId:        "sid-1",
				Kind:             "ssh",
				SessionStart:     timestamppb.New(start),
				SessionEnd:       timestamppb.New(end),
				Username:         "alice",
				UserTraits:       traits,
				UserRoles:        []string{"dev"},
				AccessRequestIds: []string{"req-1"},
				Participants:     []string{"bob"},
				ResourceKind:     "node",
				ResourceLabels:   map[string]string{"env": "prod"},
				ResourceId:       "res-1",
				ResourceName:     "host-1",
				Severity:         summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
				HostId:           "host-id-1",
				NeedsFurtherReviewReasons: []summarizerv1.NeedsReviewReason{
					summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
					summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_CLASSIFIER_MATCHED,
				},
				ResourceProperties: sessionsearchv1.ResourceProperties_builder{
					Ssh: sessionsearchv1.SSHProperties_builder{
						ServerHostname: proto.String("host-1"),
						ServerAddr:     proto.String("10.0.0.1:22"),
					}.Build(),
				}.Build(),
			}.Build(),
			assert: func(t *testing.T, got SessionSummary) {
				require.Equal(t, "sid-1", got.SessionID)
				require.Equal(t, "ssh", got.Kind)
				require.Equal(t, start, got.SessionStart)
				require.Equal(t, &end, got.SessionEnd)
				require.Equal(t, "alice", got.Username)
				require.Equal(t, map[string]any{"login": "alice"}, got.UserTraits)
				require.Equal(t, []string{"dev"}, got.UserRoles)
				require.Equal(t, []string{"req-1"}, got.AccessRequestIDs)
				require.Equal(t, []string{"bob"}, got.Participants)
				require.Equal(t, "node", got.ResourceKind)
				require.Equal(t, map[string]string{"env": "prod"}, got.ResourceLabels)
				require.Equal(t, "res-1", got.ResourceID)
				require.Equal(t, "host-1", got.ResourceName)
				require.Equal(t, "high", got.Severity)
				require.Equal(t, "host-id-1", got.HostID)
				require.Equal(t, []string{"too_large", "classifier_matched"}, got.NeedsFurtherReviewReasons)
				require.NotNil(t, got.ResourceProperties)
				require.NotNil(t, got.ResourceProperties.SSH)
				require.Equal(t, "host-1", got.ResourceProperties.SSH.ServerHostname)
				require.Equal(t, "10.0.0.1:22", got.ResourceProperties.SSH.ServerAddr)
			},
		},
		{
			name: "no optional fields",
			in: sessionsearchv1.SessionSummary_builder{
				SessionId:    "sid-2",
				SessionStart: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			}.Build(),
			assert: func(t *testing.T, got SessionSummary) {
				require.Equal(t, "sid-2", got.SessionID)
				require.Nil(t, got.SessionEnd)
				require.Nil(t, got.UserTraits)
				require.Nil(t, got.ResourceProperties)
				require.Nil(t, got.NeedsFurtherReviewReasons)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, MakeSessionSummary(tt.in))
		})
	}
}

func TestSeverity(t *testing.T) {
	t.Run("SeverityString", func(t *testing.T) {
		cases := map[summarizerv1.RiskLevel]string{
			summarizerv1.RiskLevel_RISK_LEVEL_LOW:         "low",
			summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM:      "medium",
			summarizerv1.RiskLevel_RISK_LEVEL_HIGH:        "high",
			summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL:    "critical",
			summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED: "",
		}
		for in, want := range cases {
			require.Equal(t, want, SeverityString(in))
		}
	})

	t.Run("SeverityFromString", func(t *testing.T) {
		cases := map[string]summarizerv1.RiskLevel{
			"low":      summarizerv1.RiskLevel_RISK_LEVEL_LOW,
			"critical": summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
			"":         summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED,
			"garbage":  summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED,
		}
		for in, want := range cases {
			require.Equal(t, want, SeverityFromString(in))
		}
	})

	t.Run("RoundTrip", func(t *testing.T) {
		for _, level := range []summarizerv1.RiskLevel{
			summarizerv1.RiskLevel_RISK_LEVEL_LOW,
			summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM,
			summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
			summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
		} {
			require.Equal(t, level, SeverityFromString(SeverityString(level)))
		}
	})
}

func TestNeedsReviewReason(t *testing.T) {
	t.Run("NeedsReviewReasonString", func(t *testing.T) {
		cases := map[summarizerv1.NeedsReviewReason]string{
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE:                        "too_large",
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED:          "command_analysis_failed",
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_FAILED_TO_FETCH_ACCESS_REQUEST:   "failed_to_fetch_access_request",
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_ACCESS_REQUEST_RESOURCE_MISMATCH: "access_request_resource_mismatch",
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_OUTPUT_NOT_FULLY_CAPTURED:        "output_not_fully_captured",
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_CLASSIFIER_MATCHED:               "classifier_matched",
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_UNSPECIFIED:                      "",
			// A value with no known token surfaces as an "unknown(<value>)"
			// placeholder rather than being dropped.
			summarizerv1.NeedsReviewReason(9999): "unknown(9999)",
		}
		for in, want := range cases {
			require.Equal(t, want, NeedsReviewReasonString(in))
		}
	})

	t.Run("NeedsReviewReasonFromString", func(t *testing.T) {
		cases := map[string]summarizerv1.NeedsReviewReason{
			"too_large":          summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
			"classifier_matched": summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_CLASSIFIER_MATCHED,
			"":                   summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_UNSPECIFIED,
			"garbage":            summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_UNSPECIFIED,
		}
		for in, want := range cases {
			require.Equal(t, want, NeedsReviewReasonFromString(in))
		}
	})

	// Exhaustive walks every value the proto enum defines so that adding a new
	// NeedsReviewReason without a corresponding web UI token fails this test:
	// such a value would fall through to the "unknown(<value>)" placeholder and
	// would not round-trip back to itself.
	t.Run("Exhaustive", func(t *testing.T) {
		for value, name := range summarizerv1.NeedsReviewReason_name {
			reason := summarizerv1.NeedsReviewReason(value)
			if reason == summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_UNSPECIFIED {
				continue
			}
			token := NeedsReviewReasonString(reason)
			require.False(t, strings.HasPrefix(token, "unknown("),
				"proto enum %s has no web UI token (got placeholder %q)", name, token)
			require.Equal(t, reason, NeedsReviewReasonFromString(token),
				"token %q does not round-trip back to proto enum %s", token, name)
		}
	})
}
