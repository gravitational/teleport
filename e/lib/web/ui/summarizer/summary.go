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
	ShortDescription          string            `json:"shortDescription,omitempty"`
	DetailedDescription       string            `json:"detailedDescription,omitempty"`
	RiskLevel                 string            `json:"riskLevel,omitempty"`
	SuspiciousActivities      []string          `json:"suspiciousActivities,omitempty"`
	CompromiseIndicators      bool              `json:"compromiseIndicators,omitempty"`
	NotableCommandIndexes     []int32           `json:"notableCommandIndexes,omitempty"`
	SessionEvents             []SessionEvent    `json:"sessionEvents,omitempty"`
	NeedsFurtherReviewReasons []string          `json:"needsFurtherReviewReasons,omitempty"`
	RiskScoreReasons          []RiskScoreReason `json:"riskScoreReasons,omitempty"`
	// Commands is the deprecated form of SessionEvents, retained for older
	// frontends that still consume it. Derived from SessionEvents entries that
	// carry CommandEventDetails.
	Commands []CommandAnalysis `json:"commands,omitempty"`
	// NeedsFurtherReview is the deprecated single-reason form, retained for
	// older frontends that still consume it. Derived from the first entry of
	// NeedsFurtherReviewReasons.
	NeedsFurtherReview string `json:"needsFurtherReview,omitempty"`
}

type SessionEvent struct {
	Category              string               `json:"category,omitempty"`
	RiskLevel             string               `json:"riskLevel,omitempty"`
	RiskScore             int32                `json:"riskScore,omitempty"`
	ThreatCategory        string               `json:"threatCategory,omitempty"`
	TimelineTitle         string               `json:"timelineTitle,omitempty"`
	TimelineSubtitle      string               `json:"timelineSubtitle,omitempty"`
	ShortDescription      string               `json:"shortDescription,omitempty"`
	DetailedDescription   string               `json:"detailedDescription,omitempty"`
	SuspiciousFlags       []string             `json:"suspiciousFlags,omitempty"`
	SensitiveItems        []string             `json:"sensitiveItems,omitempty"`
	SuspiciousPatterns    []string             `json:"suspiciousPatterns,omitempty"`
	IOCs                  []string             `json:"iocs,omitempty"`
	MitreAttackIDs        []string             `json:"mitreAttackIds,omitempty"`
	HasSensitiveData      bool                 `json:"hasSensitiveData,omitempty"`
	PrivilegeEscalation   bool                 `json:"privilegeEscalation,omitempty"`
	DataExfiltration      bool                 `json:"dataExfiltration,omitempty"`
	Persistence           bool                 `json:"persistence,omitempty"`
	StartOffset           int64                `json:"startOffset,omitempty"`
	EndOffset             int64                `json:"endOffset,omitempty"`
	InferenceErrorMessage string               `json:"inferenceErrorMessage,omitempty"`
	CommandEventDetails   *CommandEventDetails `json:"commandEventDetails,omitempty"`
	DesktopEventDetails   *DesktopEventDetails `json:"desktopEventDetails,omitempty"`
}

type CommandEventDetails struct {
	Command       string   `json:"command,omitempty"`
	Success       bool     `json:"success,omitempty"`
	ErrorMessages []string `json:"errorMessages,omitempty"`
}

type DesktopEventDetails struct {
	Applications      []string `json:"applications,omitempty"`
	VisibleURLs       []string `json:"visibleUrls,omitempty"`
	VisibleFilePaths  []string `json:"visibleFilePaths,omitempty"`
	ActiveWindowTitle string   `json:"activeWindowTitle,omitempty"`
}

type RiskScoreReason struct {
	Reason      string `json:"reason,omitempty"`
	ScoreImpact int32  `json:"scoreImpact,omitempty"`
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
	out := &EnhancedSummary{
		ShortDescription:          es.GetShortDescription(),
		DetailedDescription:       es.GetDetailedDescription(),
		RiskLevel:                 es.GetRiskLevel().String(),
		SuspiciousActivities:      es.GetSuspiciousActivities(),
		CompromiseIndicators:      es.GetCompromiseIndicators(),
		NotableCommandIndexes:     es.GetNotableCommandIndexes(),
		SessionEvents:             makeSessionEvents(es.GetSessionEvents()),
		NeedsFurtherReviewReasons: makeNeedsFurtherReviewReasons(es.GetNeedsFurtherReviewReasons()),
		RiskScoreReasons:          makeRiskScoreReasons(es.GetRiskScoreReasons()),
	}

	// Old-auth fallback: a v19+ proxy talking to a pre-v19 auth (rolling
	// upgrade) gets a response with only the deprecated fields populated, since
	// that auth never ran the version-aware adjuster. Surface the deprecated
	// data as the new fields so the JSON shape is consistent regardless of
	// which auth answered.
	//nolint:staticcheck // deprecated field read for cross-version compatibility
	deprecatedCommands := es.GetCommands()
	if len(out.SessionEvents) == 0 && len(deprecatedCommands) > 0 {
		out.SessionEvents = sessionEventsFromProtoCommands(deprecatedCommands)
	}
	//nolint:staticcheck // deprecated field read for cross-version compatibility
	deprecatedReason := es.NeedsFurtherReview
	if len(out.NeedsFurtherReviewReasons) == 0 && deprecatedReason != nil {
		out.NeedsFurtherReviewReasons = makeNeedsFurtherReviewReasons([]summarizerv1.NeedsReviewReason{*deprecatedReason})
	}

	// Mirror the new fields into the deprecated JSON keys for older frontends.
	out.Commands = commandsFromSessionEvents(out.SessionEvents)
	if len(out.NeedsFurtherReviewReasons) > 0 {
		out.NeedsFurtherReview = out.NeedsFurtherReviewReasons[0]
	}
	return out
}

func makeNeedsFurtherReviewReason(reason summarizerv1.NeedsReviewReason) string {
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

func makeNeedsFurtherReviewReasons(reasons []summarizerv1.NeedsReviewReason) []string {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		s := makeNeedsFurtherReviewReason(r)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func makeRiskScoreReasons(reasons []*summarizerv1.RiskScoreReason) []RiskScoreReason {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]RiskScoreReason, len(reasons))
	for i, r := range reasons {
		out[i] = RiskScoreReason{
			Reason:      r.GetReason(),
			ScoreImpact: r.GetScoreImpact(),
		}
	}
	return out
}

func makeSessionEvents(events []*summarizerv1.SessionEvent) []SessionEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]SessionEvent, len(events))
	for i, e := range events {
		out[i] = SessionEvent{
			Category:              e.GetCategory().String(),
			RiskLevel:             e.GetRiskLevel().String(),
			RiskScore:             e.GetRiskScore(),
			ThreatCategory:        e.GetThreatCategory().String(),
			TimelineTitle:         e.GetTimelineTitle(),
			TimelineSubtitle:      e.GetTimelineSubtitle(),
			ShortDescription:      e.GetShortDescription(),
			DetailedDescription:   e.GetDetailedDescription(),
			SuspiciousFlags:       e.GetSuspiciousFlags(),
			SensitiveItems:        e.GetSensitiveItems(),
			SuspiciousPatterns:    e.GetSuspiciousPatterns(),
			IOCs:                  e.GetIocs(),
			MitreAttackIDs:        e.GetMitreAttackIds(),
			HasSensitiveData:      e.GetHasSensitiveData(),
			PrivilegeEscalation:   e.GetPrivilegeEscalation(),
			DataExfiltration:      e.GetDataExfiltration(),
			Persistence:           e.GetPersistence(),
			StartOffset:           e.GetStartOffset().AsDuration().Milliseconds(),
			EndOffset:             e.GetEndOffset().AsDuration().Milliseconds(),
			InferenceErrorMessage: e.GetInferenceErrorMessage(),
		}
		if cmd := e.GetCommandEventDetails(); cmd != nil {
			out[i].CommandEventDetails = &CommandEventDetails{
				Command:       cmd.GetCommand(),
				Success:       cmd.GetSuccess(),
				ErrorMessages: cmd.GetErrorMessages(),
			}
		}
		if dt := e.GetDesktopEventDetails(); dt != nil {
			out[i].DesktopEventDetails = &DesktopEventDetails{
				Applications:      dt.GetApplications(),
				VisibleURLs:       dt.GetVisibleUrls(),
				VisibleFilePaths:  dt.GetVisibleFilePaths(),
				ActiveWindowTitle: dt.GetActiveWindowTitle(),
			}
		}
	}
	return out
}

func sessionEventsFromProtoCommands(commands []*summarizerv1.CommandAnalysis) []SessionEvent {
	if len(commands) == 0 {
		return nil
	}
	out := make([]SessionEvent, len(commands))
	for i, c := range commands {
		out[i] = SessionEvent{
			Category:              c.GetCategory().String(),
			RiskLevel:             c.GetRiskLevel().String(),
			RiskScore:             c.GetRiskScore(),
			ThreatCategory:        c.GetThreatCategory().String(),
			TimelineTitle:         c.GetTimelineTitle(),
			TimelineSubtitle:      c.GetTimelineSubtitle(),
			ShortDescription:      c.GetShortDescription(),
			DetailedDescription:   c.GetDetailedDescription(),
			SuspiciousFlags:       c.GetSuspiciousFlags(),
			SensitiveItems:        c.GetSensitiveItems(),
			SuspiciousPatterns:    c.GetSuspiciousPatterns(),
			IOCs:                  c.GetIocs(),
			MitreAttackIDs:        c.GetMitreAttackIds(),
			HasSensitiveData:      c.GetHasSensitiveData(),
			PrivilegeEscalation:   c.GetPrivilegeEscalation(),
			DataExfiltration:      c.GetDataExfiltration(),
			Persistence:           c.GetPersistence(),
			StartOffset:           c.GetStartOffset().AsDuration().Milliseconds(),
			EndOffset:             c.GetEndOffset().AsDuration().Milliseconds(),
			InferenceErrorMessage: c.GetInferenceErrorMessage(),
			CommandEventDetails: &CommandEventDetails{
				Command:       c.GetCommand(),
				Success:       c.GetSuccess(),
				ErrorMessages: c.GetErrorMessages(),
			},
		}
	}
	return out
}

func commandsFromSessionEvents(events []SessionEvent) []CommandAnalysis {
	if len(events) == 0 {
		return nil
	}
	out := make([]CommandAnalysis, 0, len(events))
	for _, e := range events {
		if e.CommandEventDetails == nil {
			continue
		}
		out = append(out, CommandAnalysis{
			Command:               e.CommandEventDetails.Command,
			Success:               e.CommandEventDetails.Success,
			ErrorMessages:         e.CommandEventDetails.ErrorMessages,
			Category:              e.Category,
			RiskLevel:             e.RiskLevel,
			RiskScore:             e.RiskScore,
			ThreatCategory:        e.ThreatCategory,
			TimelineTitle:         e.TimelineTitle,
			TimelineSubtitle:      e.TimelineSubtitle,
			ShortDescription:      e.ShortDescription,
			DetailedDescription:   e.DetailedDescription,
			SuspiciousFlags:       e.SuspiciousFlags,
			SensitiveItems:        e.SensitiveItems,
			SuspiciousPatterns:    e.SuspiciousPatterns,
			IOCs:                  e.IOCs,
			MitreAttackIDs:        e.MitreAttackIDs,
			HasSensitiveData:      e.HasSensitiveData,
			PrivilegeEscalation:   e.PrivilegeEscalation,
			DataExfiltration:      e.DataExfiltration,
			Persistence:           e.Persistence,
			StartOffset:           e.StartOffset,
			EndOffset:             e.EndOffset,
			InferenceErrorMessage: e.InferenceErrorMessage,
		})
	}
	return out
}
