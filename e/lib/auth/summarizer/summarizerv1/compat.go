package summarizerv1

import (
	"context"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/lib/utils"
)

// minClientVersionForNewSummaryFields is the minimum client version that
// understands the SessionEvents and NeedsFurtherReviewReasons fields on
// EnhancedSummary. Older clients receive the deprecated Commands and
// NeedsFurtherReview fields.
//
// TODO(ryanclark): DELETE IN v21.0.0.
var minClientVersionForNewSummaryFields = semver.Version{Major: 19, Minor: 0, Patch: 0}

// adjustEnhancedSummaryForClient rewrites the EnhancedSummary on a Summary
// based on the calling client's reported version: older clients get the
// deprecated fields populated and the new fields stripped, newer clients get
// the new fields populated when only the deprecated form is on disk.
//
// The write path in schema/proto.go also persists both the new and deprecated
// fields, so a pre-v19 auth serving a recording written by a v19+ auth (during
// a rolling upgrade or rollback) still has data to return without running this
// adjuster.
func adjustEnhancedSummaryForClient(ctx context.Context, summary *pb.Summary) error {
	es := summary.GetEnhancedSummary()
	if es == nil {
		return nil
	}

	clientVersionString, ok := metadata.ClientVersionFromContext(ctx)
	if !ok {
		upgradeEnhancedSummary(es)
		return nil
	}

	supported, err := utils.MinVerWithoutPreRelease(clientVersionString, minClientVersionForNewSummaryFields.String())
	if err != nil {
		return trace.BadParameter("unrecognized client version: %s is not a valid semver", clientVersionString)
	}

	if !supported {
		downgradeEnhancedSummary(es)
		return nil
	}

	upgradeEnhancedSummary(es)
	return nil
}

func upgradeEnhancedSummary(es *pb.EnhancedSummary) {
	//nolint:staticcheck // deprecated field read for backwards compatibility
	if len(es.GetSessionEvents()) == 0 && len(es.GetCommands()) > 0 {
		//nolint:staticcheck // deprecated field read for backwards compatibility
		es.SetSessionEvents(commandsToSessionEvents(es.GetCommands()))
	}
	//nolint:staticcheck // deprecated field read for backwards compatibility
	if len(es.GetNeedsFurtherReviewReasons()) == 0 && es.HasNeedsFurtherReview() {
		//nolint:staticcheck // deprecated field read for backwards compatibility
		es.SetNeedsFurtherReviewReasons([]pb.NeedsReviewReason{es.GetNeedsFurtherReview()})
	}
}

func downgradeEnhancedSummary(es *pb.EnhancedSummary) {
	//nolint:staticcheck // deprecated field write for backwards compatibility
	if len(es.GetCommands()) == 0 && len(es.GetSessionEvents()) > 0 {
		//nolint:staticcheck // deprecated field write for backwards compatibility
		es.SetCommands(sessionEventsToCommands(es.GetSessionEvents()))
	}
	//nolint:staticcheck // deprecated field write for backwards compatibility
	if !es.HasNeedsFurtherReview() && len(es.GetNeedsFurtherReviewReasons()) > 0 {
		first := es.GetNeedsFurtherReviewReasons()[0]
		//nolint:staticcheck // deprecated field write for backwards compatibility
		es.SetNeedsFurtherReview(first)
	}
	es.SetSessionEvents(nil)
	es.SetNeedsFurtherReviewReasons(nil)
}

func sessionEventsToCommands(events []*pb.SessionEvent) []*pb.CommandAnalysis {
	out := make([]*pb.CommandAnalysis, 0, len(events))
	for _, e := range events {
		details := e.GetCommandEventDetails()
		if details == nil {
			continue
		}
		out = append(out, pb.CommandAnalysis_builder{
			Command:               details.GetCommand(),
			Success:               details.GetSuccess(),
			ErrorMessages:         details.GetErrorMessages(),
			Category:              e.GetCategory(),
			RiskLevel:             e.GetRiskLevel(),
			RiskScore:             e.GetRiskScore(),
			ThreatCategory:        e.GetThreatCategory(),
			TimelineTitle:         e.GetTimelineTitle(),
			TimelineSubtitle:      e.GetTimelineSubtitle(),
			ShortDescription:      e.GetShortDescription(),
			DetailedDescription:   e.GetDetailedDescription(),
			SuspiciousFlags:       e.GetSuspiciousFlags(),
			SensitiveItems:        e.GetSensitiveItems(),
			SuspiciousPatterns:    e.GetSuspiciousPatterns(),
			Iocs:                  e.GetIocs(),
			MitreAttackIds:        e.GetMitreAttackIds(),
			HasSensitiveData:      e.GetHasSensitiveData(),
			PrivilegeEscalation:   e.GetPrivilegeEscalation(),
			DataExfiltration:      e.GetDataExfiltration(),
			Persistence:           e.GetPersistence(),
			StartOffset:           e.GetStartOffset(),
			EndOffset:             e.GetEndOffset(),
			InferenceErrorMessage: e.GetInferenceErrorMessage(),
		}.Build())
	}
	return out
}

func commandsToSessionEvents(commands []*pb.CommandAnalysis) []*pb.SessionEvent {
	out := make([]*pb.SessionEvent, len(commands))
	for i, c := range commands {
		out[i] = pb.SessionEvent_builder{
			Category:              c.GetCategory(),
			RiskLevel:             c.GetRiskLevel(),
			RiskScore:             c.GetRiskScore(),
			ThreatCategory:        c.GetThreatCategory(),
			TimelineTitle:         c.GetTimelineTitle(),
			TimelineSubtitle:      c.GetTimelineSubtitle(),
			ShortDescription:      c.GetShortDescription(),
			DetailedDescription:   c.GetDetailedDescription(),
			SuspiciousFlags:       c.GetSuspiciousFlags(),
			SensitiveItems:        c.GetSensitiveItems(),
			SuspiciousPatterns:    c.GetSuspiciousPatterns(),
			Iocs:                  c.GetIocs(),
			MitreAttackIds:        c.GetMitreAttackIds(),
			HasSensitiveData:      c.GetHasSensitiveData(),
			PrivilegeEscalation:   c.GetPrivilegeEscalation(),
			DataExfiltration:      c.GetDataExfiltration(),
			Persistence:           c.GetPersistence(),
			StartOffset:           c.GetStartOffset(),
			EndOffset:             c.GetEndOffset(),
			InferenceErrorMessage: c.GetInferenceErrorMessage(),
			CommandEventDetails: pb.CommandEventDetails_builder{
				Command:       c.GetCommand(),
				Success:       c.GetSuccess(),
				ErrorMessages: c.GetErrorMessages(),
			}.Build(),
		}.Build()
	}
	return out
}
