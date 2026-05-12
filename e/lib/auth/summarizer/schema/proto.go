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
	case "package_management":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_PACKAGE_MANAGEMENT
	case "container":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_CONTAINER
	case "source_control":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_SOURCE_CONTROL
	case "scheduling":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_SCHEDULING
	case "monitoring":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_MONITORING
	case "user_management":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_USER_MANAGEMENT
	case "transfer":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_TRANSFER
	case "development":
		return summarizerv1pb.CommandCategory_COMMAND_CATEGORY_DEVELOPMENT
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

func commandAnalysisToSessionEvent(cmd *CommandAnalysis) *summarizerv1pb.SessionEvent {
	if cmd == nil {
		return nil
	}

	return &summarizerv1pb.SessionEvent{
		Category:              commandCategoryToProto(cmd.Category),
		RiskLevel:             riskLevelToProto(cmd.RiskLevel),
		RiskScore:             int32(cmd.RiskScore),
		ThreatCategory:        threatCategoryToProto(cmd.ThreatCategory),
		TimelineTitle:         cmd.TimelineTitle,
		TimelineSubtitle:      cmd.TimelineSubtitle,
		ShortDescription:      cmd.ShortDescription,
		DetailedDescription:   cmd.Description,
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
		Details: &summarizerv1pb.SessionEvent_CommandEventDetails{
			CommandEventDetails: &summarizerv1pb.CommandEventDetails{
				Command:       cmd.Command,
				Success:       cmd.Success,
				ErrorMessages: cmd.ErrorMessages,
			},
		},
	}
}

func commandAnalysesListToSessionEvents(commands []*CommandAnalysis) []*summarizerv1pb.SessionEvent {
	if commands == nil {
		return nil
	}

	result := make([]*summarizerv1pb.SessionEvent, len(commands))
	for i := range commands {
		result[i] = commandAnalysisToSessionEvent(commands[i])
	}
	return result
}

// commandAnalysisToProto converts a CommandAnalysis to its deprecated proto form.
// Persisted alongside SessionEvents so a v18 auth serving a summary written by a
// v19 auth (rolling upgrade or rollback) still has data to return.
//
// TODO(ryanclark): DELETE IN v21.0.0.
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
		SessionEvents:         commandAnalysesListToSessionEvents(commands),
		// Persisted alongside SessionEvents so a pre-v19 auth serving a recording
		// written by a v19+ auth (rolling upgrade or rollback) still has data to
		// return. TODO(ryanclark): DELETE IN v21.0.0.
		Commands: commandAnalysesListToProto(commands),
	}

	if analysis.TooLarge {
		es.NeedsFurtherReviewReasons = append(es.NeedsFurtherReviewReasons,
			summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE)
	}

	if analysis.CommandAnalysisFailed {
		es.NeedsFurtherReviewReasons = append(es.NeedsFurtherReviewReasons,
			summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED)
	}

	// Mirror the first reason into the deprecated NeedsFurtherReview field so a
	// pre-v19 auth serving a recording written by a v19+ auth still has data to
	// return. TODO(ryanclark): DELETE IN v21.0.0.
	if len(es.NeedsFurtherReviewReasons) > 0 {
		first := es.NeedsFurtherReviewReasons[0]
		//nolint:staticcheck // deprecated field populated for cross-version compatibility
		es.NeedsFurtherReview = &first
	}

	return es
}
