package summarizerv1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestAdjustEnhancedSummaryForClient_OldClient_NewData(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "18.5.0"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Empty(t, es.SessionEvents, "new field should be stripped for old client")
	require.Empty(t, es.NeedsFurtherReviewReasons, "new field should be stripped for old client")

	//nolint:staticcheck // verifying downgrade populated deprecated field
	cmds := es.GetCommands()
	require.Len(t, cmds, 1)
	require.Equal(t, "rm -rf /", cmds[0].GetCommand())
	require.False(t, cmds[0].GetSuccess())
	require.Equal(t, []string{"permission denied"}, cmds[0].GetErrorMessages())
	require.Equal(t, pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION, cmds[0].GetCategory())
	require.Equal(t, pb.RiskLevel_RISK_LEVEL_HIGH, cmds[0].GetRiskLevel())
	require.Equal(t, int32(9), cmds[0].GetRiskScore())
	require.Equal(t, []string{"T1059"}, cmds[0].GetMitreAttackIds())

	//nolint:staticcheck // verifying downgrade populated deprecated field
	require.NotNil(t, es.NeedsFurtherReview)
	//nolint:staticcheck // verifying downgrade populated deprecated field
	require.Equal(t, pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE, *es.NeedsFurtherReview)
}

func TestAdjustEnhancedSummaryForClient_OldClient_OldData(t *testing.T) {
	summary := oldDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "18.5.0"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Empty(t, es.SessionEvents)
	require.Empty(t, es.NeedsFurtherReviewReasons)

	//nolint:staticcheck // deprecated field is what old clients consume
	require.Len(t, es.GetCommands(), 1)
	//nolint:staticcheck // deprecated field is what old clients consume
	require.Equal(t, "ls -la", es.GetCommands()[0].GetCommand())
	//nolint:staticcheck // deprecated field is what old clients consume
	require.NotNil(t, es.NeedsFurtherReview)
}

func TestAdjustEnhancedSummaryForClient_NewClient_OldData(t *testing.T) {
	summary := oldDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.0.0"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.SessionEvents, 1)
	ev := es.SessionEvents[0]
	require.Equal(t, "ls -la", ev.GetCommandEventDetails().GetCommand())
	require.True(t, ev.GetCommandEventDetails().GetSuccess())
	require.Equal(t, []string{"x"}, ev.GetCommandEventDetails().GetErrorMessages())
	require.Equal(t, pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION, ev.GetCategory())
	require.Equal(t, pb.RiskLevel_RISK_LEVEL_LOW, ev.GetRiskLevel())
	require.Equal(t, []string{"T1083"}, ev.GetMitreAttackIds())

	require.Equal(t,
		[]pb.NeedsReviewReason{pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE},
		es.NeedsFurtherReviewReasons,
	)
}

func TestAdjustEnhancedSummaryForClient_PrereleaseClient_TreatedAsRelease(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.0.0-prealpha.2"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.SessionEvents, 1, "prerelease of release threshold must not trigger downgrade")
	require.NotEmpty(t, es.NeedsFurtherReviewReasons)
}

func TestAdjustEnhancedSummaryForClient_NewClient_NewData(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.2.3"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.SessionEvents, 1)
	require.Equal(t, "rm -rf /", es.SessionEvents[0].GetCommandEventDetails().GetCommand())
	require.Equal(t,
		[]pb.NeedsReviewReason{
			pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
			pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED,
		},
		es.NeedsFurtherReviewReasons,
	)
}

func TestAdjustEnhancedSummaryForClient_NoClientVersion_TreatedAsNew(t *testing.T) {
	summary := oldDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, ""), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.SessionEvents, 1)
	require.Equal(t, "ls -la", es.SessionEvents[0].GetCommandEventDetails().GetCommand())
}

func TestAdjustEnhancedSummaryForClient_InvalidClientVersion_Errors(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "not-a-semver"), summary)
	require.Error(t, err)
}

func TestAdjustEnhancedSummaryForClient_NilEnhancedSummary_NoOp(t *testing.T) {
	summary := &pb.Summary{}
	require.NoError(t, adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "18.5.0"), summary))
	require.NoError(t, adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.0.0"), summary))
}

func TestAdjustEnhancedSummaryForClient_OldClient_DesktopEvent_Skipped(t *testing.T) {
	summary := &pb.Summary{
		EnhancedSummary: &pb.EnhancedSummary{
			SessionEvents: []*pb.SessionEvent{
				{
					Category: pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
					Details: &pb.SessionEvent_CommandEventDetails{
						CommandEventDetails: &pb.CommandEventDetails{Command: "whoami"},
					},
				},
				{
					Category: pb.CommandCategory_COMMAND_CATEGORY_OTHER,
					Details: &pb.SessionEvent_DesktopEventDetails{
						DesktopEventDetails: &pb.DesktopEventDetails{Applications: []string{"chrome"}},
					},
				},
			},
		},
	}
	require.NoError(t, adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "18.5.0"), summary))

	es := summary.GetEnhancedSummary()
	//nolint:staticcheck // verifying downgrade populated deprecated field
	cmds := es.GetCommands()
	require.Len(t, cmds, 1, "desktop event should be dropped on downgrade")
	require.Equal(t, "whoami", cmds[0].GetCommand())
}

func ctxWithClientVersion(t *testing.T, version string) context.Context {
	t.Helper()
	if version == "" {
		return t.Context()
	}
	return metadata.NewIncomingContext(
		t.Context(),
		metadata.New(map[string]string{"version": version}),
	)
}

func newDataSummary() *pb.Summary {
	return &pb.Summary{
		EnhancedSummary: &pb.EnhancedSummary{
			SessionEvents: []*pb.SessionEvent{{
				Category:       pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
				RiskLevel:      pb.RiskLevel_RISK_LEVEL_HIGH,
				RiskScore:      9,
				MitreAttackIds: []string{"T1059"},
				Details: &pb.SessionEvent_CommandEventDetails{
					CommandEventDetails: &pb.CommandEventDetails{
						Command:       "rm -rf /",
						Success:       false,
						ErrorMessages: []string{"permission denied"},
					},
				},
			}},
			NeedsFurtherReviewReasons: []pb.NeedsReviewReason{
				pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
				pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED,
			},
		},
	}
}

func oldDataSummary() *pb.Summary {
	tooLarge := pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE
	return &pb.Summary{
		EnhancedSummary: &pb.EnhancedSummary{
			//nolint:staticcheck // testing deprecated field upgrade path
			Commands: []*pb.CommandAnalysis{{
				Command:        "ls -la",
				Success:        true,
				Category:       pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
				RiskLevel:      pb.RiskLevel_RISK_LEVEL_LOW,
				RiskScore:      1,
				MitreAttackIds: []string{"T1083"},
				ErrorMessages:  []string{"x"},
			}},
			//nolint:staticcheck // testing deprecated field upgrade path
			NeedsFurtherReview: &tooLarge,
		},
	}
}
