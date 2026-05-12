package summarizer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestMakeEnhancedSummary_NewFieldsOnly(t *testing.T) {
	es := &summarizerv1.EnhancedSummary{
		ShortDescription: "did stuff",
		SessionEvents: []*summarizerv1.SessionEvent{{
			Category:  summarizerv1.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
			RiskLevel: summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
			RiskScore: 9,
			Details: &summarizerv1.SessionEvent_CommandEventDetails{
				CommandEventDetails: &summarizerv1.CommandEventDetails{
					Command: "rm -rf /",
				},
			},
		}},
		NeedsFurtherReviewReasons: []summarizerv1.NeedsReviewReason{
			summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
		},
		RiskScoreReasons: []*summarizerv1.RiskScoreReason{{
			Reason:      "destructive command",
			ScoreImpact: 50,
		}},
	}

	got := makeEnhancedSummary(es)
	require.Len(t, got.SessionEvents, 1)
	require.Equal(t, "rm -rf /", got.SessionEvents[0].CommandEventDetails.Command)
	require.Equal(t, []string{"too_large"}, got.NeedsFurtherReviewReasons)
	require.Equal(t, "too_large", got.NeedsFurtherReview, "deprecated mirror should be derived")
	require.Len(t, got.Commands, 1, "deprecated mirror should be derived")
	require.Equal(t, "rm -rf /", got.Commands[0].Command)
	require.Len(t, got.RiskScoreReasons, 1)
	require.Equal(t, int32(50), got.RiskScoreReasons[0].ScoreImpact)
}

func TestMakeEnhancedSummary_OldFieldsOnly_RollingUpgradeFallback(t *testing.T) {
	tooLarge := summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE
	es := &summarizerv1.EnhancedSummary{
		ShortDescription: "did stuff",
		//nolint:staticcheck // testing rolling-upgrade fallback path
		Commands: []*summarizerv1.CommandAnalysis{{
			Command:     "ls -la",
			Success:     true,
			Category:    summarizerv1.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
			RiskLevel:   summarizerv1.RiskLevel_RISK_LEVEL_LOW,
			RiskScore:   1,
			StartOffset: durationpb.New(0),
			EndOffset:   durationpb.New(0),
		}},
		//nolint:staticcheck // testing rolling-upgrade fallback path
		NeedsFurtherReview: &tooLarge,
	}

	got := makeEnhancedSummary(es)
	require.Len(t, got.SessionEvents, 1, "fallback should derive sessionEvents from proto.commands")
	require.NotNil(t, got.SessionEvents[0].CommandEventDetails)
	require.Equal(t, "ls -la", got.SessionEvents[0].CommandEventDetails.Command)
	require.True(t, got.SessionEvents[0].CommandEventDetails.Success)
	require.Equal(t, []string{"too_large"}, got.NeedsFurtherReviewReasons)
	require.Equal(t, "too_large", got.NeedsFurtherReview)
	require.Len(t, got.Commands, 1)
}

func TestMakeEnhancedSummary_BothFields_PrefersNew(t *testing.T) {
	tooLarge := summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE
	cmdAnalysisFailed := summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED
	es := &summarizerv1.EnhancedSummary{
		SessionEvents: []*summarizerv1.SessionEvent{{
			Details: &summarizerv1.SessionEvent_CommandEventDetails{
				CommandEventDetails: &summarizerv1.CommandEventDetails{Command: "from sessionEvents"},
			},
		}},
		//nolint:staticcheck // testing precedence
		Commands:                  []*summarizerv1.CommandAnalysis{{Command: "from commands"}},
		NeedsFurtherReviewReasons: []summarizerv1.NeedsReviewReason{cmdAnalysisFailed},
		//nolint:staticcheck // testing precedence
		NeedsFurtherReview: &tooLarge,
	}

	got := makeEnhancedSummary(es)
	require.Len(t, got.SessionEvents, 1)
	require.Equal(t, "from sessionEvents", got.SessionEvents[0].CommandEventDetails.Command,
		"new fields take precedence when both are set")
	require.Equal(t, []string{"command_analysis_failed"}, got.NeedsFurtherReviewReasons)
	require.Equal(t, "command_analysis_failed", got.NeedsFurtherReview)
}

func TestMakeEnhancedSummary_DesktopEvent_Preserved(t *testing.T) {
	es := &summarizerv1.EnhancedSummary{
		SessionEvents: []*summarizerv1.SessionEvent{{
			Details: &summarizerv1.SessionEvent_DesktopEventDetails{
				DesktopEventDetails: &summarizerv1.DesktopEventDetails{
					Applications:      []string{"chrome"},
					ActiveWindowTitle: "Gmail",
				},
			},
		}},
	}

	got := makeEnhancedSummary(es)
	require.Len(t, got.SessionEvents, 1)
	require.Nil(t, got.SessionEvents[0].CommandEventDetails)
	require.NotNil(t, got.SessionEvents[0].DesktopEventDetails)
	require.Equal(t, []string{"chrome"}, got.SessionEvents[0].DesktopEventDetails.Applications)
	require.Equal(t, "Gmail", got.SessionEvents[0].DesktopEventDetails.ActiveWindowTitle)
	require.Empty(t, got.Commands, "desktop events should not become deprecated commands")
}

func TestMakeEnhancedSummary_Nil(t *testing.T) {
	require.Nil(t, makeEnhancedSummary(nil))
}

func TestMakeSummary_PassthroughTopLevelFields(t *testing.T) {
	in := &summarizerv1.Summary{
		SessionId:    "abc",
		State:        summarizerv1.SummaryState_SUMMARY_STATE_SUCCESS,
		Content:      "free-form text",
		ErrorMessage: "",
	}
	got := MakeSummary(in)
	require.Equal(t, "abc", got.SessionID)
	require.Equal(t, "SUMMARY_STATE_SUCCESS", got.State)
	require.Equal(t, "free-form text", got.Content)
	require.Nil(t, got.EnhancedSummary)
}
