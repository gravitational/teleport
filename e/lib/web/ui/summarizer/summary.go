package summarizer

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// Summary is a web API representation of a session recording summary.
type Summary struct {
	SessionID           string           `json:"sessionId"`
	State               string           `json:"state,omitempty"`
	InferenceStartedAt  time.Time        `json:"inferenceStartedAt,omitempty"`
	InferenceFinishedAt *time.Time       `json:"inferenceFinishedAt,omitempty"`
	Content             string           `json:"content,omitempty"`
	ErrorMessage        string           `json:"errorMessage,omitempty"`
	EnhancedSummary     *EnhancedSummary `json:"enhancedSummary,omitempty"`
}

type EnhancedSummary struct {
	ShortDescription      string            `json:"shortDescription,omitempty"`
	DetailedDescription   string            `json:"detailedDescription,omitempty"`
	RiskLevel             string            `json:"riskLevel,omitempty"`
	SuspiciousActivities  []string          `json:"suspiciousActivities,omitempty"`
	CompromiseIndicators  bool              `json:"compromiseIndicators,omitempty"`
	NotableCommandIndexes []int32           `json:"notableCommandIndexes,omitempty"`
	Commands              []CommandAnalysis `json:"commands,omitempty"`
	NeedsFurtherReview    string            `json:"needsFurtherReview,omitempty"`
}

type CommandAnalysis struct {
	Command               string   `json:"command,omitempty"`
	Category              string   `json:"category,omitempty"`
	Success               bool     `json:"success,omitempty"`
	RiskLevel             string   `json:"riskLevel,omitempty"`
	RiskScore             int32    `json:"riskScore,omitempty"`
	ThreatCategory        string   `json:"threatCategory,omitempty"`
	TimelineTitle         string   `json:"timelineTitle,omitempty"`
	TimelineSubtitle      string   `json:"timelineSubtitle,omitempty"`
	ShortDescription      string   `json:"shortDescription,omitempty"`
	DetailedDescription   string   `json:"detailedDescription,omitempty"`
	ErrorMessages         []string `json:"errorMessages,omitempty"`
	SuspiciousFlags       []string `json:"suspiciousFlags,omitempty"`
	SensitiveItems        []string `json:"sensitiveItems,omitempty"`
	SuspiciousPatterns    []string `json:"suspiciousPatterns,omitempty"`
	IOCs                  []string `json:"iocs,omitempty"`
	MitreAttackIDs        []string `json:"mitreAttackIds,omitempty"`
	HasSensitiveData      bool     `json:"hasSensitiveData,omitempty"`
	PrivilegeEscalation   bool     `json:"privilegeEscalation,omitempty"`
	DataExfiltration      bool     `json:"dataExfiltration,omitempty"`
	Persistence           bool     `json:"persistence,omitempty"`
	StartOffset           int64    `json:"startOffset,omitempty"`
	EndOffset             int64    `json:"endOffset,omitempty"`
	InferenceErrorMessage string   `json:"inferenceErrorMessage,omitempty"`
}

func asOptionalTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// MakeSummary converts a summary object into its Web API representation.
func MakeSummary(summary *summarizerv1.Summary) Summary {
	return Summary{
		SessionID:           summary.GetSessionId(),
		State:               summary.GetState().String(),
		InferenceStartedAt:  summary.GetInferenceStartedAt().AsTime(),
		InferenceFinishedAt: asOptionalTime(summary.GetInferenceFinishedAt()),
		Content:             summary.GetContent(),
		ErrorMessage:        summary.GetErrorMessage(),
		EnhancedSummary:     makeEnhancedSummary(summary.GetEnhancedSummary()),
	}
}

func makeEnhancedSummary(es *summarizerv1.EnhancedSummary) *EnhancedSummary {
	if es == nil {
		return nil
	}
	return &EnhancedSummary{
		ShortDescription:      es.GetShortDescription(),
		DetailedDescription:   es.GetDetailedDescription(),
		RiskLevel:             es.GetRiskLevel().String(),
		SuspiciousActivities:  es.GetSuspiciousActivities(),
		CompromiseIndicators:  es.GetCompromiseIndicators(),
		NotableCommandIndexes: es.GetNotableCommandIndexes(),
		Commands:              makeCommandAnalyses(es.GetCommands()),
		NeedsFurtherReview:    makeNeedsFurtherReview(es.GetNeedsFurtherReview()),
	}
}

func makeNeedsFurtherReview(reason summarizerv1.NeedsReviewReason) string {
	switch reason {
	case summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_UNSPECIFIED:
		return ""
	case summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE:
		return "too_large"
	case summarizerv1.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED:
		return "command_analysis_failed"
	default:
		return "unknown"
	}
}

func makeCommandAnalyses(commands []*summarizerv1.CommandAnalysis) []CommandAnalysis {
	if commands == nil {
		return nil
	}
	result := make([]CommandAnalysis, len(commands))
	for i, cmd := range commands {
		result[i] = makeCommandAnalysis(cmd)
	}
	return result
}

func makeCommandAnalysis(cmd *summarizerv1.CommandAnalysis) CommandAnalysis {
	return CommandAnalysis{
		Command:               cmd.GetCommand(),
		Category:              cmd.GetCategory().String(),
		Success:               cmd.GetSuccess(),
		RiskLevel:             cmd.GetRiskLevel().String(),
		RiskScore:             cmd.GetRiskScore(),
		ThreatCategory:        cmd.GetThreatCategory().String(),
		TimelineTitle:         cmd.GetTimelineTitle(),
		TimelineSubtitle:      cmd.GetTimelineSubtitle(),
		ShortDescription:      cmd.GetShortDescription(),
		DetailedDescription:   cmd.GetDetailedDescription(),
		ErrorMessages:         cmd.GetErrorMessages(),
		SuspiciousFlags:       cmd.GetSuspiciousFlags(),
		SensitiveItems:        cmd.GetSensitiveItems(),
		SuspiciousPatterns:    cmd.GetSuspiciousPatterns(),
		IOCs:                  cmd.GetIocs(),
		MitreAttackIDs:        cmd.GetMitreAttackIds(),
		HasSensitiveData:      cmd.GetHasSensitiveData(),
		PrivilegeEscalation:   cmd.GetPrivilegeEscalation(),
		DataExfiltration:      cmd.GetDataExfiltration(),
		Persistence:           cmd.GetPersistence(),
		StartOffset:           cmd.GetStartOffset().AsDuration().Milliseconds(),
		EndOffset:             cmd.GetEndOffset().AsDuration().Milliseconds(),
		InferenceErrorMessage: cmd.GetInferenceErrorMessage(),
	}
}
