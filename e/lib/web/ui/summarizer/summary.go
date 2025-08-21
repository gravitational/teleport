package summarizer

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// Summary is a web API representation of a session recording summary.
type Summary struct {
	SessionID           string     `json:"sessionId"`
	State               string     `json:"state,omitempty"`
	InferenceStartedAt  time.Time  `json:"inferenceStartedAt,omitempty"`
	InferenceFinishedAt *time.Time `json:"inferenceFinishedAt,omitempty"`
	Content             string     `json:"content,omitempty"`
	ErrorMessage        string     `json:"errorMessage,omitempty"`
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
	}
}
