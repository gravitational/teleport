package report

import (
	"time"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

// HostInput is everything the report builder needs per host.
type HostInput struct {
	HostUUID      string
	Hostname      string
	Mapping       string
	JoinMethod    string
	Services      []string
	ConfigDiff    string
	RewriteMode   string
	RemediationID string
	ClassifyOutput classify.ClassifyOutput
}

// ReportMeta provides run-level metadata.
type ReportMeta struct {
	RunID         string
	Source        ClusterIdentity
	Target        ClusterIdentity
	ScopesEnabled bool
	Scopable      Capability
	Mappings      []MappingSummary
	Warnings      []string
}

// Build constructs the full report from classified host inputs. Pure, no I/O.
func Build(inputs []HostInput, meta ReportMeta) *Report {
	rpt := &Report{
		RunID:         meta.RunID,
		GeneratedAt:   time.Now().UTC(),
		Source:        meta.Source,
		Target:        meta.Target,
		ScopesEnabled: meta.ScopesEnabled,
		Scopable:      meta.Scopable,
		Mappings:      meta.Mappings,
		Warnings:      meta.Warnings,
		Summary: Summary{
			ByVerdict: make(map[classify.Verdict]int),
		},
	}

	iacRemediations := make(map[string]bool)
	pipelineRemediations := make(map[string]bool)

	for _, input := range inputs {
		co := input.ClassifyOutput
		host := HostReport{
			HostUUID:         input.HostUUID,
			Hostname:         input.Hostname,
			Mapping:          input.Mapping,
			Verdict:          co.Verdict,
			Status:           co.Status,
			Attention:        co.Attention,
			Reason:           co.Reason,
			JoinMethod:       input.JoinMethod,
			Services:         input.Services,
			StrippedServices: co.StrippedServices,
			MarkerTrust:      co.MarkerTrust,
			ConfigDiff:       input.ConfigDiff,
			RewriteMode:      input.RewriteMode,
			RemediationRef:   input.RemediationID,
		}
		rpt.Hosts = append(rpt.Hosts, host)
		rpt.Summary.Total++
		rpt.Summary.ByVerdict[co.Verdict]++

		if input.Mapping == "(orphan)" {
			rpt.Summary.Orphans++
		}
		if co.Status == classify.StatusBlocked {
			rpt.Summary.Blocked++
		}
		if co.Attention == classify.AttentionNone {
			rpt.Summary.ReadyToEnroll++
			rpt.Summary.Attention.AutomaticHosts++
		}
		if co.Attention == classify.AttentionManual {
			rpt.Summary.Attention.ManualHosts++
		}
		if co.Attention == classify.AttentionIaCOnetime {
			rpt.Summary.Attention.IaCHostsCovered++
			if input.RemediationID != "" {
				iacRemediations[input.RemediationID] = true
			}
		}
		if co.Attention == classify.AttentionPipeline {
			rpt.Summary.Attention.PipelineHostsCovered++
			if input.RemediationID != "" {
				pipelineRemediations[input.RemediationID] = true
			}
		}
	}

	rpt.Summary.Attention.IaCActions = len(iacRemediations)
	rpt.Summary.Attention.PipelineActions = len(pipelineRemediations)

	return rpt
}
