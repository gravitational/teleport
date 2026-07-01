package report

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

func TestBuildSummary(t *testing.T) {
	inputs := []HostInput{
		{Hostname: "h1", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictAuto, Status: classify.StatusSatisfied, Attention: classify.AttentionNone}},
		{Hostname: "h2", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictAuto, Status: classify.StatusSatisfied, Attention: classify.AttentionNone}},
		{Hostname: "h3", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictPrereq, Status: classify.StatusBlocked, Attention: classify.AttentionIaCOnetime}, RemediationID: "rem-1"},
		{Hostname: "h4", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictPrereq, Status: classify.StatusBlocked, Attention: classify.AttentionIaCOnetime}, RemediationID: "rem-1"},
		{Hostname: "h5", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictManual, Status: classify.StatusBlocked, Attention: classify.AttentionManual}},
	}
	meta := ReportMeta{RunID: "test-run"}
	rpt := Build(inputs, meta)
	require.Equal(t, 5, rpt.Summary.Total)
	require.Equal(t, 2, rpt.Summary.ByVerdict[classify.VerdictAuto])
	require.Equal(t, 2, rpt.Summary.ByVerdict[classify.VerdictPrereq])
	require.Equal(t, 1, rpt.Summary.ByVerdict[classify.VerdictManual])
	require.Equal(t, 3, rpt.Summary.Blocked)
	require.Equal(t, 2, rpt.Summary.ReadyToEnroll)
	require.Equal(t, 2, rpt.Summary.Attention.AutomaticHosts)
	require.Equal(t, 1, rpt.Summary.Attention.IaCActions) // 1 distinct remediation
	require.Equal(t, 2, rpt.Summary.Attention.IaCHostsCovered)
	require.Equal(t, 1, rpt.Summary.Attention.ManualHosts)
}

func TestBuildAttentionCountsDistinctRemediations(t *testing.T) {
	inputs := []HostInput{
		{Hostname: "h1", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictPrereq, Status: classify.StatusBlocked, Attention: classify.AttentionIaCOnetime}, RemediationID: "rem-1"},
		{Hostname: "h2", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictPrereq, Status: classify.StatusBlocked, Attention: classify.AttentionIaCOnetime}, RemediationID: "rem-1"},
		{Hostname: "h3", ClassifyOutput: classify.ClassifyOutput{Verdict: classify.VerdictPrereq, Status: classify.StatusBlocked, Attention: classify.AttentionIaCOnetime}, RemediationID: "rem-2"},
	}
	rpt := Build(inputs, ReportMeta{})
	// 2 distinct IaC actions cover 3 hosts
	require.Equal(t, 2, rpt.Summary.Attention.IaCActions)
	require.Equal(t, 3, rpt.Summary.Attention.IaCHostsCovered)
}
