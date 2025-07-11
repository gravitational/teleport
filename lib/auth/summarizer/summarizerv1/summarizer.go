package summarizerv1

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// Summarizer summarizes session recordings using language model inference.
type Summarizer interface {
	// Summarize summarizes a session recording with a given ID. The
	// sessionEndEvent is optional, but should be specified if possible, as it
	// lets us skip reading the session stream just to find the end event.
	Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf) error
}

// SummarizerWrapper is a wrapper around the SummarizerService interface. Its
// purpose is to allow substituting the wrapped service after a dependent
// service has been configured with the wrapper as the service implementation.
type SummarizerWrapper struct {
	Summarizer
}

func NewSummarizerWrapper() *SummarizerWrapper {
	return &SummarizerWrapper{
		Summarizer: &UnimplementedSummarizer{},
	}
}

type UnimplementedSummarizer struct{}

func (s *UnimplementedSummarizer) Summarize(
	ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf,
) error {
	return requireEnterprise()
}

func requireEnterprise() error {
	return trace.AccessDenied(
		"session recording summarization is only available with an enterprise license that supports Teleport Identity Security")
}
