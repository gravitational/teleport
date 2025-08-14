package ui

import (
	"time"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

type SessionRecordingSummary struct {
	SessionID           string     `json:"sessionId"`
	State               string     `json:"state,omitempty"`
	InferenceStartedAt  *time.Time `json:"inferenceStartedAt,omitempty"`
	InferenceFinishedAt *time.Time `json:"inferenceFinishedAt,omitempty"`
	Content             string     `json:"content,omitempty"`
	ErrorMessage        string     `json:"errorMessage,omitempty"`
}

// MakeSessionRecordingSummary converts a summary object into its Web API
// representation.
func MakeSessionRecordingSummary(summary *summarizerv1.Summary) SessionRecordingSummary {
	return SessionRecordingSummary{
		SessionID: summary.GetSessionId(),
		State:     summary.GetState().String(),
		// InferenceStartedAt:  summary.GetInferenceStartedAt().AsTime(),
		// InferenceFinishedAt: summary.GetInferenceFinishedAt().AsTime(),
		Content:      summary.GetContent(),
		ErrorMessage: summary.GetErrorMessage(),
	}
}
