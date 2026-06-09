package summarizerv1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestAdjustEnhancedSummaryForClient_OldClient_NewData(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "18.5.0"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Empty(t, es.GetSessionEvents(), "new field should be stripped for old client")
	require.Empty(t, es.GetNeedsFurtherReviewReasons(), "new field should be stripped for old client")

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
	require.NotNil(t, proto.ValueOrNil(es.HasNeedsFurtherReview(), es.GetNeedsFurtherReview))
	//nolint:staticcheck // verifying downgrade populated deprecated field
	require.Equal(t, pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE, es.GetNeedsFurtherReview())
}

func TestAdjustEnhancedSummaryForClient_OldClient_OldData(t *testing.T) {
	summary := oldDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "18.5.0"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Empty(t, es.GetSessionEvents())
	require.Empty(t, es.GetNeedsFurtherReviewReasons())

	//nolint:staticcheck // deprecated field is what old clients consume
	require.Len(t, es.GetCommands(), 1)
	//nolint:staticcheck // deprecated field is what old clients consume
	require.Equal(t, "ls -la", es.GetCommands()[0].GetCommand())
	//nolint:staticcheck // deprecated field is what old clients consume
	require.NotNil(t, proto.ValueOrNil(es.HasNeedsFurtherReview(), es.GetNeedsFurtherReview))
}

func TestAdjustEnhancedSummaryForClient_NewClient_OldData(t *testing.T) {
	summary := oldDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.0.0"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.GetSessionEvents(), 1)
	ev := es.GetSessionEvents()[0]
	require.Equal(t, "ls -la", ev.GetCommandEventDetails().GetCommand())
	require.True(t, ev.GetCommandEventDetails().GetSuccess())
	require.Equal(t, []string{"x"}, ev.GetCommandEventDetails().GetErrorMessages())
	require.Equal(t, pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION, ev.GetCategory())
	require.Equal(t, pb.RiskLevel_RISK_LEVEL_LOW, ev.GetRiskLevel())
	require.Equal(t, []string{"T1083"}, ev.GetMitreAttackIds())

	require.Equal(t,
		[]pb.NeedsReviewReason{pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE},
		es.GetNeedsFurtherReviewReasons(),
	)
}

func TestAdjustEnhancedSummaryForClient_PrereleaseClient_TreatedAsRelease(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.0.0-prealpha.2"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.GetSessionEvents(), 1, "prerelease of release threshold must not trigger downgrade")
	require.NotEmpty(t, es.GetNeedsFurtherReviewReasons())
}

func TestAdjustEnhancedSummaryForClient_NewClient_NewData(t *testing.T) {
	summary := newDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, "19.2.3"), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.GetSessionEvents(), 1)
	require.Equal(t, "rm -rf /", es.GetSessionEvents()[0].GetCommandEventDetails().GetCommand())
	require.Equal(t,
		[]pb.NeedsReviewReason{
			pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
			pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED,
		},
		es.GetNeedsFurtherReviewReasons(),
	)
}

func TestAdjustEnhancedSummaryForClient_NoClientVersion_TreatedAsNew(t *testing.T) {
	summary := oldDataSummary()
	err := adjustEnhancedSummaryForClient(ctxWithClientVersion(t, ""), summary)
	require.NoError(t, err)

	es := summary.GetEnhancedSummary()
	require.Len(t, es.GetSessionEvents(), 1)
	require.Equal(t, "ls -la", es.GetSessionEvents()[0].GetCommandEventDetails().GetCommand())
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
	summary := pb.Summary_builder{
		EnhancedSummary: pb.EnhancedSummary_builder{
			SessionEvents: []*pb.SessionEvent{
				pb.SessionEvent_builder{
					Category:            pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
					CommandEventDetails: pb.CommandEventDetails_builder{Command: "whoami"}.Build(),
				}.Build(),
				pb.SessionEvent_builder{
					Category:            pb.CommandCategory_COMMAND_CATEGORY_OTHER,
					DesktopEventDetails: pb.DesktopEventDetails_builder{Applications: []string{"chrome"}}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()
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
	return pb.Summary_builder{
		EnhancedSummary: pb.EnhancedSummary_builder{
			SessionEvents: []*pb.SessionEvent{pb.SessionEvent_builder{
				Category:       pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
				RiskLevel:      pb.RiskLevel_RISK_LEVEL_HIGH,
				RiskScore:      9,
				MitreAttackIds: []string{"T1059"},
				CommandEventDetails: pb.CommandEventDetails_builder{
					Command:       "rm -rf /",
					Success:       false,
					ErrorMessages: []string{"permission denied"},
				}.Build(),
			}.Build()},
			NeedsFurtherReviewReasons: []pb.NeedsReviewReason{
				pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
				pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED,
			},
		}.Build(),
	}.Build()
}

func oldDataSummary() *pb.Summary {
	tooLarge := pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE
	return pb.Summary_builder{
		EnhancedSummary: pb.EnhancedSummary_builder{
			//nolint:staticcheck // testing deprecated field upgrade path
			Commands: []*pb.CommandAnalysis{pb.CommandAnalysis_builder{
				Command:        "ls -la",
				Success:        true,
				Category:       pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION,
				RiskLevel:      pb.RiskLevel_RISK_LEVEL_LOW,
				RiskScore:      1,
				MitreAttackIds: []string{"T1083"},
				ErrorMessages:  []string{"x"},
			}.Build()},
			//nolint:staticcheck // testing deprecated field upgrade path
			NeedsFurtherReview: &tooLarge,
		}.Build(),
	}.Build()
}
