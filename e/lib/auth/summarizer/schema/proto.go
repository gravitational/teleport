package schema

import (
	"google.golang.org/protobuf/types/known/durationpb"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func commandCategoryToProto(category string) summarizerv1pb.CommandCategory {
	switch category {
	case "file_operation":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_FILE_OPERATION
	case "network":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_NETWORK
	case "process":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_PROCESS
	case "system_config":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_SYSTEM_CONFIG
	case "data_access":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_DATA_ACCESS
	case "authentication":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_AUTHENTICATION
	case "other":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_OTHER
	default:
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_UNSPECIFIED
	}
}

func riskLevelToProto(level string) summarizerv1pb.RiskLevel {
	switch level {
	case "low":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_LOW
	case "medium":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_MEDIUM
	case "high":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH
	case "critical":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_CRITICAL
	default:
		return summarizerv1pb.RiskLevel_RISK_LEVEL_UNSPECIFIED
	}
}

func threatCategoryToProto(category string) summarizerv1pb.ThreatCategory {
	switch category {
	case "reconnaissance":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_RECONNAISSANCE
	case "execution":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_EXECUTION
	case "persistence":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_PERSISTENCE
	case "privilege_escalation":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_PRIVILEGE_ESCALATION
	case "defense_evasion":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_DEFENSE_EVASION
	case "credential_access":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_CREDENTIAL_ACCESS
	case "discovery":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_DISCOVERY
	case "lateral_movement":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_LATERAL_MOVEMENT
	case "collection":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_COLLECTION
	case "exfiltration":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_EXFILTRATION
	case "impact":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_IMPACT
	case "none":
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_NONE
	default:
		return summarizerv1pb.ThreatCategory_THREAT_CATEGORY_UNSPECIFIED
	}
}

func commandAnalysisToProto(cmd *CommandAnalysis) *summarizerv1pb.CommandAnalysis {
	if cmd == nil {
		return nil
	}

	return &summarizerv1pb.CommandAnalysis{
		Command:               cmd.Command,
		Category:              commandCategoryToProto(cmd.Category),
		Success:               cmd.Success,
		RiskLevel:             riskLevelToProto(cmd.RiskLevel),
		RiskScore:             int32(cmd.RiskScore),
		ThreatCategory:        threatCategoryToProto(cmd.ThreatCategory),
		TimelineTitle:         cmd.TimelineTitle,
		TimelineSubtitle:      cmd.TimelineSubtitle,
		ShortDescription:      cmd.ShortDescription,
		DetailedDescription:   cmd.Description,
		ErrorMessages:         cmd.ErrorMessages,
		SuspiciousFlags:       cmd.SuspiciousFlags,
		SensitiveItems:        cmd.SensitiveItems,
		SuspiciousPatterns:    cmd.SuspiciousPatterns,
		Iocs:                  cmd.IOCs,
		MitreAttackIds:        cmd.MitreAttackIDs,
		HasSensitiveData:      cmd.HasSensitiveData,
		PrivilegeEscalation:   cmd.PrivilegeEscalation,
		DataExfiltration:      cmd.DataExfiltration,
		Persistence:           cmd.Persistence,
		StartOffset:           durationpb.New(cmd.StartOffset),
		EndOffset:             durationpb.New(cmd.EndOffset),
		InferenceErrorMessage: cmd.InferenceErrorMessage,
	}
}

func commandAnalysesListToProto(commands []*CommandAnalysis) []*summarizerv1pb.CommandAnalysis {
	if commands == nil {
		return nil
	}

	result := make([]*summarizerv1pb.CommandAnalysis, len(commands))
	for i := range commands {
		result[i] = commandAnalysisToProto(commands[i])
	}
	return result
}

func notableCommandIndexesToProto(indexes []int) []int32 {
	if indexes == nil {
		return nil
	}

	result := make([]int32, len(indexes))
	for i, idx := range indexes {
		result[i] = int32(idx)
	}
	return result
}

func SessionAnalysisToProto(analysis *SessionAnalysis, commands []*CommandAnalysis) *summarizerv1pb.EnhancedSummary {
	es := &summarizerv1pb.EnhancedSummary{
		ShortDescription:      analysis.ShortDescription,
		DetailedDescription:   analysis.SessionDescription,
		RiskLevel:             riskLevelToProto(analysis.RiskLevel),
		SuspiciousActivities:  analysis.SuspiciousActivities,
		CompromiseIndicators:  analysis.CompromiseIndicators,
		NotableCommandIndexes: notableCommandIndexesToProto(analysis.NotableCommandIndexes),
		Commands:              commandAnalysesListToProto(commands),
	}

	if analysis.TooLarge {
		tooLarge := summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE
		es.NeedsFurtherReview = &tooLarge
	}

	if analysis.CommandAnalysisFailed && es.NeedsFurtherReview == nil {
		failed := summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED
		es.NeedsFurtherReview = &failed
	}

	return es
}
