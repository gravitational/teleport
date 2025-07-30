package ui

import (
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

type SessionRecordingSummary struct {
	Content string `json:"content"`
}

func MakeSessionRecordingSummary(summary *summarizerv1.Summary) SessionRecordingSummary {
	return SessionRecordingSummary{
		Content: summary.GetContent(),
	}
}
