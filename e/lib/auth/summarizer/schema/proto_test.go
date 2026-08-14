package schema

import (
	"testing"

	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema/schematypes"
)

func TestDesktopSessionAnalysisToProto(t *testing.T) {
	analysis := &schematypes.DesktopSessionAnalysis{
		ShortDescription:     "Suspicious credential access followed by data downloads",
		SessionDescription:   "The session showed credential manager access and bulk downloads.",
		SuspiciousActivities: []string{"opened credentials manager and copied entries"},
		CompromiseIndicators: true,
		RiskLevel:            "high",
		RiskScore:            72,
	}
	events := []schematypes.DesktopSessionEvent{
		{
			Category:          "data_access",
			RiskLevel:         "high",
			RiskScore:         68,
			TimelineTitle:     "Credential Manager Access",
			ShortDescription:  "Opened the Windows credential manager",
			ActiveWindowTitle: "Credential Manager",
			StartTime:         "1:05",
			EndTime:           "1:30",
		},
	}

	es := DesktopSessionAnalysisToProto(analysis, events)

	require.Equal(t, "Suspicious credential access followed by data downloads", es.GetShortDescription())
	require.Equal(t, summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH, es.GetRiskLevel())
	require.Equal(t, int32(72), es.GetRiskScore())

	require.Len(t, es.GetSessionEvents(), 1)
	event := es.GetSessionEvents()[0]
	require.Equal(t, int32(68), event.GetRiskScore())
	require.Equal(t, "Credential Manager Access", event.GetTimelineTitle())
	require.Equal(t, "Credential Manager", event.GetDesktopEventDetails().GetActiveWindowTitle())
}
